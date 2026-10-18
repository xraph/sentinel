package memory

import (
	"context"
	"encoding/json"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/recorded"
)

// GetRecordedCursor reads an exact app/tenant binding acknowledgment.
func (s *Store) GetRecordedCursor(ctx context.Context, binding string) (*recorded.SourceCursor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sentinel.AppFromContext(ctx) == "" {
		return nil, recorded.ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, exists := s.recordedCursors[recorded.CursorStorageKey(ctx, binding)]
	if !exists {
		return nil, recorded.ErrNotFound
	}
	return recorded.DecodeCursor(ctx, data)
}

// AdvanceRecordedCursor acknowledges only a job this store already persisted.
func (s *Store) AdvanceRecordedCursor(ctx context.Context, req recorded.CursorAdvance) (*recorded.SourceCursor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.recordedJobLocked(ctx, req.JobID)
	if err != nil {
		return nil, err
	}
	key := recorded.CursorStorageKey(ctx, req.BindingID)
	var current *recorded.SourceCursor
	if data, exists := s.recordedCursors[key]; exists {
		current, err = recorded.DecodeCursor(ctx, data)
		if err != nil {
			return nil, err
		}
	}
	next, err := recorded.NextCursor(ctx, current, j, req, time.Now())
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	s.recordedCursors[key] = data
	return next, nil
}
