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

// groupedRecordedJobs partitions real scored captures into disjoint frozen plans.
func groupedRecordedJobs(ctx context.Context, t *testing.T, batch comparison.RecordedComparisonRequest, pending bool) comparison.RecordedGroupComparisonRequest {
	t.Helper()
	s, ok := batch.Store.(*memory.Store)
	if !ok {
		t.Fatal("fixture requires memory store")
	}
	baseline, err := s.GetRecordedJob(ctx, batch.BaselineID)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := s.GetRecordedJob(ctx, batch.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	original, err := s.GetEvaluationPlan(ctx, baseline.Request.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	e, err := engine.New(engine.WithStore(s), engine.WithScorer(scorer.Descriptor{Name: "quality", Revision: "fixture/v1"}, func(map[string]any) (scorer.Scorer, error) { return fixtureScorer{}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	out := comparison.RecordedGroupComparisonRequest{Store: s, Seed: batch.Seed, MinCases: batch.MinCases}
	for group := range 2 {
		plan := *original
		plan.ID = id.New("eplan")
		plan.Cases = nil
		members := map[id.CaseID]bool{}
		for index, definition := range original.Cases {
			if index%2 == group {
				plan.Cases = append(plan.Cases, definition)
				members[definition.Case.ID] = true
			}
		}
		plan.Digest, err = plan.ContentDigest()
		if err != nil {
			t.Fatal(err)
		}
		if err = s.CreateEvaluationPlan(ctx, &plan); err != nil {
			t.Fatal(err)
		}
		value := comparison.RecordedComparisonGroup{Plan: comparison.RecordedPlanRef{ID: plan.ID, Digest: plan.Digest}}
		for arm, source := range []*recorded.Job{baseline, candidate} {
			request := source.Request
			request.PlanID, request.PlanDigest = plan.ID, plan.Digest
			request.Cases = nil
			request.IdempotencyKey = fmt.Sprintf("group-%d-arm-%d", group, arm)
			for _, capture := range source.Request.Cases {
				if members[capture.CaseID] {
					request.Cases = append(request.Cases, capture)
				}
			}
			job, enqueueErr := e.EnqueueRecorded(ctx, request)
			if enqueueErr != nil {
				t.Fatal(enqueueErr)
			}
			if !pending || group != 1 || arm != 1 {
				if _, err = e.EvaluateRecorded(ctx, request); err != nil {
					t.Fatal(err)
				}
			}
			if arm == 0 {
				value.BaselineIDs = []id.ID{job.ID}
			} else {
				value.CandidateIDs = []id.ID{job.ID}
			}
		}
		out.Groups = append(out.Groups, value)
	}
	return out
}

func TestRecordedGroupsPreserveStatisticsAndPlanProvenance(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 20, 3, "varying", false)
	want, err := comparison.CompareRecorded(ctx, batch)
	if err != nil {
		t.Fatal(err)
	}
	req := groupedRecordedJobs(ctx, t, batch, false)
	got, err := comparison.CompareRecordedGroups(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Conclusive || got.CaseCount != 20 || got.PairCount != 60 || !reflect.DeepEqual(got.Pairs, want.Pairs) || !reflect.DeepEqual(got.Interval, want.Interval) || !reflect.DeepEqual(got.MeanDelta, want.MeanDelta) || !reflect.DeepEqual(got.DimensionDeltas, want.DimensionDeltas) {
		t.Fatalf("plan boundaries changed statistics: got %+v want %+v", got, want)
	}
	if got.PlanDigest != "" || len(got.Plans) != 2 || len(got.BaselineJobs) != 2 || len(got.CandidateJobs) != 2 || !got.BaselineID.IsNil() || !got.CandidateID.IsNil() {
		t.Fatalf("lost or fabricated aggregate identity: %+v", got)
	}
	for _, group := range req.Groups {
		if !slices.Contains(got.Plans, group.Plan) {
			t.Fatalf("missing frozen plan: %+v", group.Plan)
		}
	}
	for _, refs := range [][]comparison.RecordedJobRef{got.BaselineJobs, got.CandidateJobs} {
		for _, ref := range refs {
			job, getErr := req.Store.GetRecordedJob(ctx, ref.ID)
			if getErr != nil || job.RequestDigest != ref.RequestDigest {
				t.Fatalf("job provenance changed: %+v %v", ref, getErr)
			}
		}
	}
	slices.Reverse(req.Groups)
	again, err := comparison.CompareRecordedGroups(ctx, req)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("group order changed report: %v", err)
	}
	// The minimum applies to distinct cases across all groups, not job count.
	req.MinCases = 21
	insufficient, err := comparison.CompareRecordedGroups(ctx, req)
	if err != nil || insufficient.Conclusive || insufficient.Interval == nil {
		t.Fatalf("wrong global case threshold: %+v %v", insufficient, err)
	}
}

func TestRecordedGroupsRejectChangedOrOverlappingRoster(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 4, 2, "candidate", false)
	base := groupedRecordedJobs(ctx, t, batch, false)
	for _, name := range []string{"empty", "nil-store", "nil-plan", "plan-digest", "plan-identity", "empty-arm", "cross-plan-job", "repeated-plan", "negative-minimum", "foreign-scope", "wrong-job-reference"} {
		t.Run(name, func(t *testing.T) {
			req := base
			req.Groups = slices.Clone(base.Groups)
			callCtx := ctx
			switch name {
			case "empty":
				req.Groups = nil
			case "nil-store":
				req.Store = nil
			case "nil-plan":
				req.Groups[0].Plan.ID = id.ID{}
			case "plan-digest":
				req.Groups[0].Plan.Digest = "changed"
			case "plan-identity":
				req.Groups[0].Plan.ID = req.Groups[1].Plan.ID
			case "empty-arm":
				req.Groups[0].BaselineIDs = nil
			case "cross-plan-job":
				req.Groups[0].CandidateIDs = req.Groups[1].CandidateIDs
			case "repeated-plan":
				req.Groups = append(req.Groups, req.Groups[0])
			case "negative-minimum":
				req.MinCases = -1
			case "foreign-scope":
				callCtx = sentinel.WithTenant(ctx, "foreign")
			case "wrong-job-reference":
				req.Store = wrongRecordedReference{RecordedStore: base.Store, replacement: batch.BaselineID}
			}
			if _, err := comparison.CompareRecordedGroups(callCtx, req); err == nil {
				t.Fatal("invalid group roster accepted")
			}
		})
	}
}

func TestRecordedGroupsRejectDuplicateInputsAcrossPlans(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 2, 1, "candidate", false, true)
	if _, err := comparison.CompareRecordedGroups(ctx, groupedRecordedJobs(ctx, t, batch, false)); err == nil {
		t.Fatal("duplicate input/context across plans counted twice")
	}
	ctx, batch = recordedComparisonFixture(t, 2, 1, "candidate", false, true, true)
	req := groupedRecordedJobs(ctx, t, batch, false)
	req.MinCases = 2
	if got, err := comparison.CompareRecordedGroups(ctx, req); err != nil || !got.Conclusive {
		t.Fatalf("different frozen contexts rejected: %+v %v", got, err)
	}
}

func TestRecordedGroupsRejectCaseReuseWithDisjointRepetitions(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 2, 2, "candidate", false)
	req := groupedRecordedJobs(ctx, t, batch, false)
	s, ok := req.Store.(*memory.Store)
	if !ok {
		t.Fatal("fixture requires memory store")
	}
	e, err := engine.New(engine.WithStore(s), engine.WithScorer(scorer.Descriptor{Name: "quality", Revision: "fixture/v1"}, func(map[string]any) (scorer.Scorer, error) { return fixtureScorer{}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.GetEvaluationPlan(ctx, req.Groups[0].Plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	sources := []id.ID{req.Groups[0].BaselineIDs[0], req.Groups[0].CandidateIDs[0]}
	for repetition := range 2 {
		copyPlan := *plan
		copyPlan.ID = id.New("eplan")
		if err = s.CreateEvaluationPlan(ctx, &copyPlan); err != nil {
			t.Fatal(err)
		}
		group := comparison.RecordedComparisonGroup{Plan: comparison.RecordedPlanRef{ID: copyPlan.ID, Digest: copyPlan.Digest}}
		for arm, jobID := range sources {
			job, getErr := s.GetRecordedJob(ctx, jobID)
			if getErr != nil {
				t.Fatal(getErr)
			}
			request := job.Request
			request.PlanID = copyPlan.ID
			request.IdempotencyKey = fmt.Sprintf("reuse-%d-%d", repetition, arm)
			request.Cases = nil
			for _, capture := range job.Request.Cases {
				if capture.Repetition == repetition {
					request.Cases = append(request.Cases, capture)
				}
			}
			scored, scoreErr := e.EvaluateRecorded(ctx, request)
			if scoreErr != nil {
				t.Fatal(scoreErr)
			}
			if arm == 0 {
				group.BaselineIDs = []id.ID{scored.Recorded.ID}
			} else {
				group.CandidateIDs = []id.ID{scored.Recorded.ID}
			}
		}
		req.Groups[repetition] = group
	}
	if _, err = comparison.CompareRecordedGroups(ctx, req); err == nil {
		t.Fatal("one case counted under two frozen plans")
	}
}

func TestRecordedGroupsSinglePlanMatchesRecordedSets(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 4, 2, "candidate", false)
	req := groupedRecordedJobs(ctx, t, batch, false)
	req.Groups = req.Groups[:1]
	group := req.Groups[0]
	want, err := comparison.CompareRecordedSets(ctx, comparison.RecordedSetComparisonRequest{Store: req.Store, BaselineIDs: group.BaselineIDs, CandidateIDs: group.CandidateIDs, Seed: req.Seed})
	if err != nil {
		t.Fatal(err)
	}
	got, err := comparison.CompareRecordedGroups(ctx, req)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("single-group path changed report: got %+v want %+v error %v", got, want, err)
	}
}

func TestRecordedGroupsWaitForEveryJob(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 20, 1, "candidate", false)
	got, err := comparison.CompareRecordedGroups(ctx, groupedRecordedJobs(ctx, t, batch, true))
	if err != nil {
		t.Fatal(err)
	}
	if got.Conclusive || got.MeanDelta != nil || got.Interval != nil || got.RequiredCriteriaPassed || got.CostComparable || len(got.IncompletenessReasons) == 0 || len(got.Plans) != 2 {
		t.Fatalf("pending group became qualified evidence: %+v", got)
	}
}

func TestRecordedGroupsRetainRequiredFailuresAndCancellation(t *testing.T) {
	ctx, batch := recordedComparisonFixture(t, 20, 1, "unsafe", false)
	req := groupedRecordedJobs(ctx, t, batch, false)
	got, err := comparison.CompareRecordedGroups(ctx, req)
	if err != nil || got.MeanDelta == nil || *got.MeanDelta <= 0 || got.RequiredCriteriaPassed || len(got.RequiredFailures) != 20 {
		t.Fatalf("group hid required failures: %+v %v", got, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = comparison.CompareRecordedGroups(cancelled, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
}
