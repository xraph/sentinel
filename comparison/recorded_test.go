package comparison_test

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/comparison"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/store/memory"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

type fixtureScorer struct{}

func (fixtureScorer) Name() string { return "quality" }
func (fixtureScorer) Score(_ context.Context, in *scorer.Input) (*scorer.Output, error) {
	score := .5
	pass := true
	if in.Actual == "varying" {
		var index int
		if _, err := fmt.Sscanf(in.Input, "question %d", &index); err != nil {
			return nil, err
		}
		score = .6 + float64(index)*.01
	}
	if in.Actual == "candidate" {
		score = .8
	}
	if in.Actual == "unsafe" {
		score = .99
		pass = false
	}
	return &scorer.Output{Score: score, Passed: pass, Dimension: "quality"}, nil
}

func recordedComparisonFixture(t *testing.T, cases, repetitions int, output string, missing bool, duplicateInputs ...bool) (context.Context, comparison.RecordedComparisonRequest) {
	t.Helper()
	ctx := sentinel.WithApp(context.Background(), "compare-app")
	s := memory.New()
	su := &suite.Suite{ID: id.NewSuiteID(), AppID: "compare-app", Name: "comparison"}
	if err := s.CreateSuite(ctx, su); err != nil {
		t.Fatal(err)
	}
	req := engine.RecordedRunRequest{IdempotencyKey: "baseline"}
	for index := range cases {
		input := fmt.Sprintf("question %d", index)
		if len(duplicateInputs) > 0 && duplicateInputs[0] {
			input = "same question"
		}
		tc := &testcase.Case{ID: id.NewCaseID(), SuiteID: su.ID, Input: input, Scorers: []testcase.ScorerConfig{{Name: "quality"}}}
		if len(duplicateInputs) > 1 && duplicateInputs[1] {
			tc.Context = map[string]any{"environment_snapshot": fmt.Sprintf("snapshot-%d", index)}
		}
		if err := s.CreateCase(ctx, tc); err != nil {
			t.Fatal(err)
		}
		for repetition := range repetitions {
			req.Cases = append(req.Cases, engine.RecordedCase{CaseID: tc.ID, Repetition: repetition, InputDigest: recorded.InputDigest(tc.Input), Outcome: recorded.Completed, Output: "baseline"})
		}
	}
	e, err := engine.New(engine.WithStore(s), engine.WithScorer(scorer.Descriptor{Name: "quality", Revision: "fixture/v1"}, func(map[string]any) (scorer.Scorer, error) { return fixtureScorer{}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.FreezeEvaluation(ctx, engine.FreezeRequest{SuiteID: su.ID})
	if err != nil {
		t.Fatal(err)
	}
	req.PlanID = p.ID
	req.PlanDigest = p.Digest
	baseline, err := e.EvaluateRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	req.IdempotencyKey = "candidate"
	for i := range req.Cases {
		req.Cases[i].Output = output
	}
	if missing {
		req.Cases = req.Cases[:len(req.Cases)-1]
	}
	candidate, err := e.EvaluateRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, comparison.RecordedComparisonRequest{Store: s, BaselineID: baseline.Recorded.ID, CandidateID: candidate.Recorded.ID, Seed: 42}
}

func TestRecordedComparisonPairsCasesAndUsesDeterministicBootstrap(t *testing.T) {
	ctx, req := recordedComparisonFixture(t, 20, 3, "candidate", false)
	first, err := comparison.CompareRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := comparison.CompareRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("bootstrap changed with identical seed and evidence")
	}
	if !first.Conclusive || first.CaseCount != 20 || first.PairCount != 60 || first.BootstrapSamples != 2000 || first.MeanDelta == nil || math.Abs(*first.MeanDelta-.3) > 1e-12 || first.Interval == nil || math.Abs(first.Interval.Lower-.3) > 1e-12 {
		t.Fatalf("invalid paired estimate: %+v", first)
	}
	if first.CostComparable || first.MeanCostDelta != nil {
		t.Fatal("historical unknown cost became comparable zero")
	}
}

func TestRecordedComparisonRepetitionsDoNotInflateSampleSize(t *testing.T) {
	ctx, req := recordedComparisonFixture(t, 2, 20, "candidate", false)
	result, err := comparison.CompareRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Conclusive || result.CaseCount != 2 || len(result.IncompletenessReasons) == 0 {
		t.Fatalf("repetitions inflated evidence: %+v", result)
	}
}

func TestRecordedComparisonRefusesMissingTrial(t *testing.T) {
	ctx, req := recordedComparisonFixture(t, 20, 3, "candidate", true)
	if _, err := comparison.CompareRecorded(ctx, req); err == nil {
		t.Fatal("missing case/repetition accepted")
	}
}

func TestRecordedComparisonKeepsRequiredFailureDespiteHighAverage(t *testing.T) {
	ctx, req := recordedComparisonFixture(t, 20, 1, "unsafe", false)
	result, err := comparison.CompareRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.MeanDelta == nil || *result.MeanDelta <= 0 || result.RequiredCriteriaPassed || len(result.RequiredFailures) != 20 {
		t.Fatalf("high average hid required failures: %+v", result)
	}
}

func TestRecordedComparisonRefusesDuplicateInputsUnderNewCaseIDs(t *testing.T) {
	ctx, req := recordedComparisonFixture(t, 20, 1, "candidate", false, true)
	if _, err := comparison.CompareRecorded(ctx, req); err == nil {
		t.Fatal("duplicate inputs inflated independent evidence")
	}
}

type wrongRecordedReference struct {
	comparison.RecordedStore
	replacement id.ID
}

func (s wrongRecordedReference) GetRecordedJob(ctx context.Context, _ id.ID) (*recorded.Job, error) {
	return s.RecordedStore.GetRecordedJob(ctx, s.replacement)
}

func TestRecordedComparisonRejectsWrongReturnedReference(t *testing.T) {
	ctx, req := recordedComparisonFixture(t, 20, 1, "candidate", false)
	req.Store = wrongRecordedReference{RecordedStore: req.Store, replacement: req.CandidateID}
	if _, err := comparison.CompareRecorded(ctx, req); err == nil {
		t.Fatal("store returned another job and comparison accepted it")
	}
}

func TestRecordedBootstrapUsesSeedForNonuniformEvidence(t *testing.T) {
	ctx, req := recordedComparisonFixture(t, 20, 3, "varying", false)
	first, err := comparison.CompareRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := comparison.CompareRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, again) {
		t.Fatal("same seed changed nonuniform interval")
	}
	req.Seed++
	other, err := comparison.CompareRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Interval == nil || first.Interval.Lower >= *first.MeanDelta || first.Interval.Upper <= *first.MeanDelta || reflect.DeepEqual(first.Interval, other.Interval) {
		t.Fatalf("bootstrap ignored paired variation or seed: %+v %+v", first.Interval, other.Interval)
	}
}

func TestRecordedComparisonDistinguishesFrozenTaskContexts(t *testing.T) {
	ctx, req := recordedComparisonFixture(t, 20, 1, "candidate", false, true, true)
	result, err := comparison.CompareRecorded(ctx, req)
	if err != nil || !result.Conclusive || result.CaseCount != 20 {
		t.Fatalf("distinct frozen environments collapsed: %+v %v", result, err)
	}
}
