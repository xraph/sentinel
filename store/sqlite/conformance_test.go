package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/xraph/grove"
	"github.com/xraph/grove/drivers/sqlitedriver"
	_ "github.com/xraph/grove/drivers/sqlitedriver/sqlitemigrate"

	"github.com/xraph/sentinel/store"
	sentinelsqlite "github.com/xraph/sentinel/store/sqlite"
	"github.com/xraph/sentinel/store/storetest"
)

// SQLite here is pure Go (modernc.org/sqlite), so this runs under a plain
// `go test ./...` with no external service.
func TestConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		t.Helper()
		sdb := sqlitedriver.New()
		if err := sdb.Open(context.Background(), filepath.Join(t.TempDir(), "sentinel.db")); err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		db, err := grove.Open(sdb)
		if err != nil {
			t.Fatalf("grove open: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })
		s := sentinelsqlite.New(db)
		if err := s.Migrate(context.Background()); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		return s
	})
}
