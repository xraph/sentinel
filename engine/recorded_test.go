package engine_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/store/memory"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/target"
	"github.com/xraph/sentinel/testcase"
)

func recordedFixture(t *testing.T, opts ...engine.Option) (*engine.Engine, *memory.Store, context.Context, *engine.EvaluationPlan, engine.RecordedRunRequest) {
	t.Helper()
	s := memory.New()
	ctx := sentinel.WithApp(sentinel.WithTenant(context.Background(), "tenant-a"), "app-a")
	su := &suite.Suite{ID: id.NewSuiteID(), AppID: "app-a", Name: "recorded"}
	if err := s.CreateSuite(ctx, su); err != nil {
		t.Fatal(err)
	}
	tc := &testcase.Case{ID: id.NewCaseID(), SuiteID: su.ID, Name: "case", Input: "question", Expected: "answer", Scorers: []testcase.ScorerConfig{{Name: "exact"}}}
	if err := s.CreateCase(ctx, tc); err != nil {
		t.Fatal(err)
	}
	e, err := engine.New(append([]engine.Option{engine.WithStore(s)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := e.FreezeEvaluation(ctx, engine.FreezeRequest{SuiteID: su.ID})
	if err != nil {
		t.Fatal(err)
	}
	req := engine.RecordedRunRequest{PlanID: plan.ID, PlanDigest: plan.Digest, IdempotencyKey: "source-run/v1", Cases: []engine.RecordedCase{{CaseID: tc.ID, InputDigest: recorded.InputDigest(tc.Input), Outcome: recorded.Completed, Output: "answer", Duration: 17 * time.Second}}}
	return e, s, ctx, plan, req
}

func TestEvaluateRecordedNeverCallsTargetAndPreservesMetrics(t *testing.T) {
	var calls atomic.Int32
	e, s, ctx, plan, req := recordedFixture(t, engine.WithTarget("must-not-run", "", target.FromFunc("must-not-run", func(context.Context, string) (string, error) { calls.Add(1); return "unexpected", nil })))
	tc := plan.Cases[0].Case
	tc.Expected = "changed"
	if err := s.UpdateCase(ctx, &tc); err != nil {
		t.Fatal(err)
	}
	result, err := e.EvaluateRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || result.Results[0].Status != evalrun.StatusPass {
		t.Fatalf("target ran or frozen answer drifted: calls=%d %+v", calls.Load(), result)
	}
	if result.Results[0].LatencyMs != 17000 || result.Recorded.Request.Cases[0].Usage.CostKnown() {
		t.Fatalf("captured metrics replaced: %+v", result)
	}
	if err = s.DeleteSuite(ctx, plan.Suite.ID); err != nil {
		t.Fatal(err)
	}
	again, err := e.EvaluateRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if again.Run.ID != result.Run.ID || again.Results[0].ID != result.Results[0].ID {
		t.Fatal("retry produced new evaluation")
	}
}

func TestRecordedExecutionFailureDiffersFromScorerError(t *testing.T) {
	e, _, ctx, _, req := recordedFixture(t)
	req.Cases[0].Outcome = recorded.Incomplete
	req.Cases[0].ExecutionError = "step limit"
	result, err := e.EvaluateRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Results[0].Status != evalrun.StatusError || result.Recorded.Results[0].ExecutionError != "step limit" || result.Recorded.Results[0].ScoringError != "" || len(result.Results[0].ScorerResults) != 0 {
		t.Fatalf("execution failure misclassified: %+v", result.Recorded)
	}
}

type recordedScorerFunc struct {
	f func(context.Context, *scorer.Input) (*scorer.Output, error)
}

func (s recordedScorerFunc) Name() string { return "exact" }
func (s recordedScorerFunc) Score(ctx context.Context, in *scorer.Input) (*scorer.Output, error) {
	return s.f(ctx, in)
}

func TestRecordedRetryUsesSameResult(t *testing.T) {
	var calls atomic.Int32
	e, _, ctx, _, req := recordedFixture(t, engine.WithScorer(scorer.Descriptor{Name: "exact", Revision: "count/v1"}, func(map[string]any) (scorer.Scorer, error) {
		return recordedScorerFunc{f: func(context.Context, *scorer.Input) (*scorer.Output, error) {
			calls.Add(1)
			return &scorer.Output{Score: 1, Passed: true}, nil
		}}, nil
	}))
	first, err := e.EvaluateRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	e.Scorers().RegisterDescribed(scorer.Descriptor{Name: "exact", Revision: "changed/v2"}, func(map[string]any) (scorer.Scorer, error) {
		return nil, errors.New("must not resolve on completed retry")
	})
	again, err := e.EvaluateRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || first.Run.ID != again.Run.ID {
		t.Fatalf("completed retry rescored: %d", calls.Load())
	}
	req.Cases[0].Output = "conflict"
	if _, err = e.EvaluateRecorded(ctx, req); !errors.Is(err, recorded.ErrConflict) {
		t.Fatalf("conflicting retry accepted: %v", err)
	}
}

func TestRecordedCancellationKeepsPartialResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	e, _, scoped, _, req := recordedFixture(t, engine.WithScorer(scorer.Descriptor{Name: "exact", Revision: "cancel/v1"}, func(map[string]any) (scorer.Scorer, error) {
		return recordedScorerFunc{f: func(context.Context, *scorer.Input) (*scorer.Output, error) {
			calls.Add(1)
			cancel()
			return &scorer.Output{Score: 1, Passed: true}, nil
		}}, nil
	}))
	ctx = sentinel.WithApp(sentinel.WithTenant(ctx, sentinel.TenantFromContext(scoped)), sentinel.AppFromContext(scoped))
	second := req.Cases[0]
	second.Repetition = 1
	req.Cases = append(req.Cases, second)
	result, err := e.EvaluateRecorded(ctx, req)
	if !errors.Is(err, sentinel.ErrRunCancelled) || result == nil || len(result.Results) != 1 || calls.Load() != 1 {
		t.Fatalf("cancellation lost completed checkpoint: %+v %v calls=%d", result, err, calls.Load())
	}
}

func TestRecordedJudgeUsageIsSeparateFromExecution(t *testing.T) {
	e, _, ctx, _, req := recordedFixture(t, engine.WithScorer(scorer.Descriptor{Name: "exact", Revision: "metered/v1"}, func(map[string]any) (scorer.Scorer, error) {
		return recordedScorerFunc{f: func(context.Context, *scorer.Input) (*scorer.Output, error) {
			return &scorer.Output{Score: 1, Passed: true, Usage: &evalrun.Usage{Cost: .05, CostQuality: evalrun.Known, Currency: "USD", Tokens: 12, TokensQuality: evalrun.Known}}, nil
		}}, nil
	}))
	req.Cases[0].Usage = recorded.Usage{Cost: 1, CostQuality: recorded.Known, Currency: "USD", Tokens: 120, TokensQuality: recorded.Known}
	result, err := e.EvaluateRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Results[0].Cost != 1 || result.Results[0].TokensUsed != 120 || result.Recorded.Results[0].JudgeUsage.Cost != .05 || result.Recorded.Results[0].JudgeUsage.Tokens != 12 || !result.Recorded.Results[0].JudgeUsage.CostKnown() {
		t.Fatalf("judge and executor accounting mixed: %+v", result.Recorded)
	}
}

func TestRecordedUnknownCostCannotPassCostCriterion(t *testing.T) {
	e, s, ctx, plan, req := recordedFixture(t)
	tc := plan.Cases[0].Case
	tc.Scorers = append(tc.Scorers, testcase.ScorerConfig{Name: "cost", Config: map[string]any{"max_cost": 1.0}})
	if err := s.UpdateCase(ctx, &tc); err != nil {
		t.Fatal(err)
	}
	frozen, err := e.FreezeEvaluation(ctx, engine.FreezeRequest{SuiteID: plan.Suite.ID})
	if err != nil {
		t.Fatal(err)
	}
	req.PlanID = frozen.ID
	req.PlanDigest = frozen.Digest
	result, err := e.EvaluateRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Results[0].Status != evalrun.StatusError || result.Recorded.Results[0].ScoringError == "" {
		t.Fatalf("unknown cost passed as zero: %+v", result.Recorded)
	}
}

func TestRecordedScorerFailureRemainsSeparate(t *testing.T) {
	e, _, ctx, _, req := recordedFixture(t, engine.WithScorer(scorer.Descriptor{Name: "exact", Revision: "failed/v1"}, func(map[string]any) (scorer.Scorer, error) {
		return recordedScorerFunc{f: func(context.Context, *scorer.Input) (*scorer.Output, error) {
			return nil, errors.New("judge unavailable")
		}}, nil
	}))
	result, err := e.EvaluateRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Results[0].Status != evalrun.StatusError || result.Recorded.Results[0].ExecutionError != "" || result.Recorded.Results[0].ScoringError == "" {
		t.Fatalf("scorer failure mislabeled: %+v", result.Recorded)
	}
}
