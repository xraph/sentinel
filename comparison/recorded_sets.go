package comparison

import (
	"context"
	"fmt"
	"sort"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
)

// RecordedSetComparisonRequest selects explicit job sets for each comparison arm.
// Every job must belong to the authenticated scope and the same frozen plan.
type RecordedSetComparisonRequest struct {
	Store        RecordedStore
	BaselineIDs  []id.ID
	CandidateIDs []id.ID
	Seed         uint64
	MinCases     int // Zero uses 20 distinct cases, regardless of job or repetition count.
}

// RecordedJobRef identifies an authoritative job and its immutable input digest.
type RecordedJobRef struct {
	ID            id.ID  `json:"id"`
	RequestDigest string `json:"request_digest"`
}

type recordedArm struct {
	refs       []RecordedJobRef
	planID     id.ID
	planDigest string
	cases      map[string]recorded.Case
	results    map[string]*evalrun.Result
	complete   bool
}

func loadRecordedArm(ctx context.Context, store RecordedStore, ids []id.ID) (*recordedArm, error) {
	ordered := append([]id.ID(nil), ids...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].String() < ordered[j].String() })
	arm := &recordedArm{cases: make(map[string]recorded.Case), results: make(map[string]*evalrun.Result), complete: true}
	for _, jobID := range ordered {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		job, err := store.GetRecordedJob(ctx, jobID)
		if err != nil {
			return nil, err
		}
		if job == nil || job.ID != jobID {
			return nil, fmt.Errorf("%w: returned evaluation identity mismatch", sentinel.ErrInvalidInput)
		}
		if validateErr := job.Validate(ctx); validateErr != nil {
			return nil, validateErr
		}
		if arm.planID.IsNil() {
			arm.planID = job.Request.PlanID
			arm.planDigest = job.Request.PlanDigest
		}
		if job.Request.PlanID != arm.planID || job.Request.PlanDigest != arm.planDigest {
			return nil, fmt.Errorf("%w: comparison plans differ", sentinel.ErrInvalidInput)
		}
		arm.refs = append(arm.refs, RecordedJobRef{ID: job.ID, RequestDigest: job.RequestDigest})
		arm.complete = arm.complete && job.State == recorded.JobCompleted
		for _, capture := range job.Request.Cases {
			if _, exists := arm.cases[capture.Key()]; exists {
				return nil, fmt.Errorf("%w: overlapping comparison case and repetition", sentinel.ErrInvalidInput)
			}
			arm.cases[capture.Key()] = capture
		}
		for _, result := range job.Results {
			arm.results[result.Key] = result.Result
		}
	}
	return arm, nil
}
