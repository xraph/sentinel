package memory

import "context"

// RecordedPersistent reports that memory jobs cannot survive process restart.
func (s *Store) RecordedPersistent(_ context.Context) (bool, error) { return false, nil }
