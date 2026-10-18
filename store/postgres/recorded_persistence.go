package postgres

import "context"

// RecordedPersistent requires a reachable persistent backend.
func (s *Store) RecordedPersistent(ctx context.Context) (bool, error) {
	if err := s.Ping(ctx); err != nil {
		return false, err
	}
	return true, nil
}
