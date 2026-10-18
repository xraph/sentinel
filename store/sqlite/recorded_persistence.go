package sqlite

import "context"

// RecordedPersistent checks the active main database, excluding memory and
// temporary databases whose PRAGMA file name is empty.
func (s *Store) RecordedPersistent(ctx context.Context) (bool, error) {
	var file string
	if err := s.sdb.QueryRow(ctx, "SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&file); err != nil {
		return false, err
	}
	return file != "", nil
}
