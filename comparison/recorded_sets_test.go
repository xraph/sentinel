package comparison_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/comparison"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/store/memory"
)

// splitRecordedJobs scores the same captures through real, independently stored jobs.
func splitRecordedJobs(ctx context.Context, t *testing.T, req comparison.RecordedComparisonRequest, queued bool) comparison.RecordedSetComparisonRequest {
	t.Helper()
	s, ok := req.Store.(*memory.Store)
	if !ok {
		t.Fatal("fixture requires memory store")
	}
	e, err := engine.New(engine.WithStore(s), engine.WithScorer(scorer.Descriptor{Name: "quality", Revision: "fixture/v1"}, func(map[string]any) (scorer.Scorer, error) { return fixtureScorer{}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	result := comparison.RecordedSetComparisonRequest{Store: s, Seed: req.Seed, MinCases: req.MinCases}
	for arm, source := range []id.ID{req.BaselineID, req.CandidateID} {
		job, getErr := s.GetRecordedJob(ctx, source)
		if getErr != nil {
			t.Fatal(getErr)
		}
		for index, capture := range job.Request.Cases {
			single := job.Request
			single.Cases = []recorded.Case{capture}
			single.IdempotencyKey = fmt.Sprintf("split-%s-%d", source, index)
			var jobID id.ID
			if queued && arm == 1 && index == 0 {
				pending, enqueueErr := e.EnqueueRecorded(ctx, single)
				if enqueueErr != nil {
					t.Fatal(enqueueErr)
				}
				jobID = pending.ID
			} else {
				scored, scoreErr := e.EvaluateRecorded(ctx, single)
				if scoreErr != nil {
					t.Fatal(scoreErr)
				}
				jobID = scored.Recorded.ID
			}
			if arm == 0 {
				result.BaselineIDs = append(result.BaselineIDs, jobID)
			} else {
				result.CandidateIDs = append(result.CandidateIDs, jobID)
			}
		}
	}
	return result
}

func TestRecordedSetsMatchBatchStatisticsAndIgnoreJobOrder(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 20, 3, "varying", false)
	want, err := comparison.CompareRecorded(ctx, batch)
	if err != nil {
		t.Fatal(err)
	}
	req := splitRecordedJobs(ctx, t, batch, false)
	got, err := comparison.CompareRecordedSets(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Conclusive || got.CaseCount != 20 || got.PairCount != 60 || !reflect.DeepEqual(got.Pairs, want.Pairs) || !reflect.DeepEqual(got.Interval, want.Interval) || !reflect.DeepEqual(got.MeanDelta, want.MeanDelta) || !reflect.DeepEqual(got.DimensionDeltas, want.DimensionDeltas) {
		t.Fatalf("job boundaries changed evidence: got %+v want %+v", got, want)
	}
	if !got.BaselineID.IsNil() || !got.CandidateID.IsNil() || got.BaselineDigest != "" || got.CandidateDigest != "" || len(got.BaselineJobs) != 60 || len(got.CandidateJobs) != 60 {
		t.Fatalf("aggregate invented a single job identity: %+v", got)
	}
	for _, refs := range [][]comparison.RecordedJobRef{got.BaselineJobs, got.CandidateJobs} {
		for _, ref := range refs {
			job, getErr := req.Store.GetRecordedJob(ctx, ref.ID)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if ref.RequestDigest != job.RequestDigest {
				t.Fatal("job digest not preserved")
			}
		}
	}
	slices.Reverse(req.BaselineIDs)
	slices.Reverse(req.CandidateIDs)
	again, err := comparison.CompareRecordedSets(ctx, req)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("job order changed report: %v", err)
	}
	// Different batching on each arm is also valid.
	req.BaselineIDs = []id.ID{batch.BaselineID}
	mixed, err := comparison.CompareRecordedSets(ctx, req)
	if err != nil || !reflect.DeepEqual(mixed.Interval, want.Interval) || mixed.BaselineID != batch.BaselineID {
		t.Fatalf("mixed batching changed report: %+v %v", mixed, err)
	}
}

func TestRecordedSetsRejectRepeatedOrOverlappingEvidence(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 2, 2, "candidate", false)
	base := splitRecordedJobs(ctx, t, batch, false)
	for _, name := range []string{"empty", "nil", "repeat", "cross-arm", "overlap", "missing", "wrong-reference", "scope"} {
		t.Run(name, func(t *testing.T) {
			req := base
			req.BaselineIDs = slices.Clone(base.BaselineIDs)
			req.CandidateIDs = slices.Clone(base.CandidateIDs)
			callCtx := ctx
			switch name {
			case "empty":
				req.BaselineIDs = nil
			case "nil":
				req.BaselineIDs[0] = id.ID{}
			case "repeat":
				req.BaselineIDs = append(req.BaselineIDs, req.BaselineIDs[0])
			case "cross-arm":
				req.CandidateIDs[0] = req.BaselineIDs[0]
			case "overlap":
				req.BaselineIDs = append(req.BaselineIDs, batch.BaselineID)
			case "missing":
				req.CandidateIDs = req.CandidateIDs[1:]
			case "wrong-reference":
				req.Store = wrongRecordedReference{RecordedStore: base.Store, replacement: batch.BaselineID}
			case "scope":
				callCtx = sentinel.WithTenant(ctx, "foreign")
			}
			if _, err := comparison.CompareRecordedSets(callCtx, req); err == nil {
				t.Fatal("invalid job set accepted")
			}
		})
	}
}

func TestRecordedSetsWaitForEveryJob(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 20, 1, "candidate", false)
	req := splitRecordedJobs(ctx, t, batch, true)
	got, err := comparison.CompareRecordedSets(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Conclusive || got.MeanDelta != nil || got.Interval != nil || got.RequiredCriteriaPassed || got.CostComparable || len(got.IncompletenessReasons) == 0 {
		t.Fatalf("pending job became qualified evidence: %+v", got)
	}
}

func TestRecordedSetsPropagateCancellation(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 2, 1, "candidate", false)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err := comparison.CompareRecordedSets(cancelled, comparison.RecordedSetComparisonRequest{Store: batch.Store, BaselineIDs: []id.ID{batch.BaselineID}, CandidateIDs: []id.ID{batch.CandidateID}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestRecordedSetsRejectMixedPlanIdentity(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 2, 1, "candidate", false)
	req := splitRecordedJobs(ctx, t, batch, false)
	s, ok := batch.Store.(*memory.Store)
	if !ok {
		t.Fatal("fixture requires memory store")
	}
	source, err := s.GetRecordedJob(ctx, req.CandidateIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.GetEvaluationPlan(ctx, source.Request.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	// Equal content is insufficient: comparison must use the same frozen plan ID.
	plan.ID = id.New("eplan")
	if err = s.CreateEvaluationPlan(ctx, plan); err != nil {
		t.Fatal(err)
	}
	e, err := engine.New(engine.WithStore(s), engine.WithScorer(scorer.Descriptor{Name: "quality", Revision: "fixture/v1"}, func(map[string]any) (scorer.Scorer, error) { return fixtureScorer{}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	different := source.Request
	different.PlanID = plan.ID
	different.IdempotencyKey = "other-plan"
	scored, err := e.EvaluateRecorded(ctx, different)
	if err != nil {
		t.Fatal(err)
	}
	req.CandidateIDs[0] = scored.Recorded.ID
	if _, err = comparison.CompareRecordedSets(ctx, req); err == nil {
		t.Fatal("mixed plan IDs accepted")
	}
	// Singleton arms must enforce the same check across arms.
	req.BaselineIDs = req.BaselineIDs[:1]
	req.CandidateIDs = req.CandidateIDs[:1]
	if _, err = comparison.CompareRecordedSets(ctx, req); err == nil {
		t.Fatal("cross-arm plan mismatch accepted")
	}
}

func TestRecordedSetsRetainFailedRequiredCriteria(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 20, 1, "unsafe", false)
	got, err := comparison.CompareRecordedSets(ctx, splitRecordedJobs(ctx, t, batch, false))
	if err != nil {
		t.Fatal(err)
	}
	if got.MeanDelta == nil || *got.MeanDelta <= 0 || got.RequiredCriteriaPassed || len(got.RequiredFailures) != 20 {
		t.Fatalf("aggregate hid failed required criteria: %+v", got)
	}
}
