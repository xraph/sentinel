package comparison

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalplan"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
)

// RecordedPlanRef preserves a frozen plan's storage identity and content digest.
type RecordedPlanRef struct {
	ID     id.ID  `json:"id"`
	Digest string `json:"digest"`
}

// RecordedComparisonGroup selects paired job sets under one exact frozen plan.
type RecordedComparisonGroup struct {
	Plan         RecordedPlanRef `json:"plan"`
	BaselineIDs  []id.ID         `json:"baseline_ids"`
	CandidateIDs []id.ID         `json:"candidate_ids"`
}

// RecordedGroupComparisonRequest compares disjoint case sets across frozen plans.
// Every plan appears once. MinCases counts distinct cases across all groups.
type RecordedGroupComparisonRequest struct {
	Store    RecordedStore
	Groups   []RecordedComparisonGroup
	Seed     uint64
	MinCases int // Zero requires 20 distinct cases.
}

// CompareRecordedGroups preserves each plan and job identity while applying the
// same paired case bootstrap as CompareRecordedSets. A case cannot appear in two
// groups, even with different repetitions. The caller owns the complete roster.
func CompareRecordedGroups(ctx context.Context, req RecordedGroupComparisonRequest) (*RecordedComparison, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Store == nil || req.MinCases < 0 || len(req.Groups) == 0 {
		return nil, fmt.Errorf("%w: invalid recorded comparison groups", sentinel.ErrInvalidInput)
	}
	groups := slices.Clone(req.Groups)
	slices.SortFunc(groups, func(a, b RecordedComparisonGroup) int { return strings.Compare(a.Plan.ID.String(), b.Plan.ID.String()) })
	arms := [2]*recordedArm{}
	for index := range arms {
		arms[index] = &recordedArm{cases: map[string]recorded.Case{}, results: map[string]*evalrun.Result{}, complete: true}
	}
	definitions := map[id.CaseID]evalplan.Case{}
	seenPlans, seenJobs := map[id.ID]bool{}, map[id.ID]bool{}
	report := &RecordedComparison{}
	for _, group := range groups {
		if group.Plan.ID.IsNil() || group.Plan.Digest == "" || seenPlans[group.Plan.ID] {
			return nil, fmt.Errorf("%w: repeated or missing comparison plan", sentinel.ErrInvalidInput)
		}
		seenPlans[group.Plan.ID] = true
		baseline, candidate, plan, err := loadRecordedPair(ctx, req.Store, group.BaselineIDs, group.CandidateIDs)
		if err != nil {
			return nil, err
		}
		if plan.ID != group.Plan.ID || plan.Digest != group.Plan.Digest {
			return nil, fmt.Errorf("%w: frozen comparison group changed", recorded.ErrConflict)
		}
		report.Plans = append(report.Plans, group.Plan)
		// Register only evaluated cases; unused discovery cases do not contribute.
		used := map[id.CaseID]bool{}
		for _, source := range []*recordedArm{baseline, candidate} {
			for _, capture := range source.cases {
				used[capture.CaseID] = true
			}
		}
		for _, definition := range plan.Cases {
			key := definition.Case.ID
			if !used[key] {
				continue
			}
			if _, exists := definitions[key]; exists {
				return nil, fmt.Errorf("%w: case appears in multiple comparison plans", sentinel.ErrInvalidInput)
			}
			definitions[key] = definition
		}
		for index, source := range []*recordedArm{baseline, candidate} {
			for _, ref := range source.refs {
				if seenJobs[ref.ID] {
					return nil, fmt.Errorf("%w: repeated comparison job", sentinel.ErrInvalidInput)
				}
				seenJobs[ref.ID] = true
				arms[index].refs = append(arms[index].refs, ref)
			}
			arms[index].complete = arms[index].complete && source.complete
			for key, capture := range source.cases {
				if _, exists := arms[index].cases[key]; exists {
					return nil, fmt.Errorf("%w: overlapping comparison capture", sentinel.ErrInvalidInput)
				}
				arms[index].cases[key] = capture
			}
			for key, result := range source.results {
				arms[index].results[key] = result
			}
		}
	}
	if len(report.Plans) == 1 {
		report.PlanDigest = report.Plans[0].Digest
	}
	for _, arm := range arms {
		slices.SortFunc(arm.refs, func(a, b RecordedJobRef) int { return strings.Compare(a.ID.String(), b.ID.String()) })
	}
	return compareRecordedArms(ctx, arms[0], arms[1], definitions, report, req.Seed, req.MinCases)
}
