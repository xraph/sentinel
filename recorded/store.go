package recorded

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
)

// Store owns exactly scoped recorded jobs and atomic revision changes.
type Store interface {
	CreateRecordedJob(context.Context, *Job) (*Job, error)
	GetRecordedJob(context.Context, id.ID) (*Job, error)
	ListRecordedJobs(context.Context, Query) ([]*Job, error)
	UpdateRecordedJob(context.Context, int64, *Job) error
}

// Query pages jobs in ascending ID order. Active limits results to claimable work.
type Query struct {
	After  string
	Limit  int
	Active bool
}

// PageLimit returns a bounded limit shared by all backends.
func (q Query) PageLimit() (int, error) {
	if q.Limit < 0 || q.Limit > 100 {
		return 0, fmt.Errorf("%w: recorded page limit must be at most 100", sentinel.ErrInvalidInput)
	}
	if q.After != "" {
		if _, err := id.Parse(q.After); err != nil {
			return 0, fmt.Errorf("%w: invalid recorded cursor", sentinel.ErrInvalidInput)
		}
	}
	if q.Limit == 0 {
		return 20, nil
	}
	return q.Limit, nil
}

// StorageKey hashes an unambiguous scope and idempotency tuple.
func (j *Job) StorageKey() string {
	return InputDigest(strconv.Quote(j.AppID) + "," + strconv.Quote(j.TenantID) + "," + strconv.Quote(j.Request.IdempotencyKey))
}

// ValidateNew restricts insertion to a fresh queued request.
func (j *Job) ValidateNew(ctx context.Context) error {
	if err := j.Validate(ctx); err != nil {
		return err
	}
	if j.State != JobQueued || j.Revision != 1 || j.Attempts != 0 || j.Generation != 0 || j.Owner != "" || !j.LeaseUntil.IsZero() || len(j.Results) != 0 {
		return fmt.Errorf("%w: recorded job is not new", sentinel.ErrInvalidInput)
	}
	return nil
}

// Encode validates records before crossing backend boundaries.
func Encode(ctx context.Context, j *Job) ([]byte, error) {
	if err := j.Validate(ctx); err != nil {
		return nil, err
	}
	return json.Marshal(j)
}

// Decode validates records and detaches all nested values from storage.
func Decode(ctx context.Context, data []byte) (*Job, error) {
	var j Job
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("sentinel: decode recorded job: %w", err)
	}
	if err := j.Validate(ctx); err != nil {
		return nil, err
	}
	return &j, nil
}
