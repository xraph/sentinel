package memory

import (
	"context"
	"sort"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
)

// CreateRecordedJob atomically deduplicates an immutable request.
func (s *Store) CreateRecordedJob(ctx context.Context, j *recorded.Job) (*recorded.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := j.ValidateNew(ctx); err != nil {
		return nil, err
	}
	data, err := recorded.Encode(ctx, j)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.recordedJobs {
		old, decodeErr := recorded.Decode(ctx, existing)
		if decodeErr != nil {
			continue
		} // Other scopes cannot deduplicate this request.
		if old.StorageKey() == j.StorageKey() {
			if old.RequestDigest != j.RequestDigest {
				return nil, recorded.ErrConflict
			}
			return old, nil
		}
	}
	if _, exists := s.recordedJobs[j.ID.String()]; exists {
		return nil, recorded.ErrConflict
	}
	s.recordedJobs[j.ID.String()] = data
	return recorded.Decode(ctx, data)
}

// GetRecordedJob reads only the exact app and tenant.
func (s *Store) GetRecordedJob(ctx context.Context, jobID id.ID) (*recorded.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recordedJobLocked(ctx, jobID)
}

func (s *Store) recordedJobLocked(ctx context.Context, jobID id.ID) (*recorded.Job, error) {
	data, exists := s.recordedJobs[jobID.String()]
	if !exists || sentinel.AppFromContext(ctx) == "" {
		return nil, recorded.ErrNotFound
	}
	j, err := recorded.Decode(ctx, data)
	if err != nil {
		return nil, recorded.ErrNotFound
	}
	return j, nil
}

// ListRecordedJobs returns a stable ID page within the exact scope.
func (s *Store) ListRecordedJobs(ctx context.Context, q recorded.Query) ([]*recorded.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sentinel.AppFromContext(ctx) == "" {
		return nil, recorded.ErrNotFound
	}
	limit, err := q.PageLimit()
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var jobs []*recorded.Job
	for key, data := range s.recordedJobs {
		if key <= q.After {
			continue
		}
		j, decodeErr := recorded.Decode(ctx, data)
		if decodeErr != nil {
			continue
		}
		if q.Active && j.Terminal() {
			continue
		}
		jobs = append(jobs, j)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].ID.String() < jobs[j].ID.String() })
	if len(jobs) > limit {
		jobs = jobs[:limit]
	}
	return jobs, nil
}

// UpdateRecordedJob writes a fenced checkpoint under an atomic revision check.
func (s *Store) UpdateRecordedJob(ctx context.Context, expected int64, j *recorded.Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := j.Validate(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.recordedJobLocked(ctx, j.ID)
	if err != nil {
		return err
	}
	if old.Revision != expected {
		return recorded.ErrConflict
	}
	if changeErr := recorded.ValidateChange(ctx, old, j, time.Now().UTC()); changeErr != nil {
		return changeErr
	}
	data, err := recorded.Encode(ctx, j)
	if err != nil {
		return err
	}
	s.recordedJobs[j.ID.String()] = data
	return nil
}
