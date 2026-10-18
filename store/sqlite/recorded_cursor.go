package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/xraph/grove"
	"github.com/xraph/grove/migrate"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/recorded"
)

type recordedCursorModel struct {
	grove.BaseModel `grove:"table:sentinel_recorded_cursors"`
	ID              string `grove:"id,pk"`
	AppID           string `grove:"app_id,notnull"`
	TenantID        string `grove:"tenant_id,notnull"`
	BindingID       string `grove:"binding_id,notnull"`
	Revision        int64  `grove:"revision,notnull"`
	Payload         string `grove:"payload,notnull"`
}

// GetRecordedCursor reads only the exact app/tenant binding acknowledgment.
func (s *Store) GetRecordedCursor(ctx context.Context, binding string) (*recorded.SourceCursor, error) {
	if sentinel.AppFromContext(ctx) == "" {
		return nil, recorded.ErrNotFound
	}
	m := new(recordedCursorModel)
	err := s.sdb.NewSelect(m).Where("id = ?", recorded.CursorStorageKey(ctx, binding)).Where("app_id = ?", sentinel.AppFromContext(ctx)).Where("tenant_id = ?", sentinel.TenantFromContext(ctx)).Where("binding_id = ?", binding).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, recorded.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return recorded.DecodeCursor(ctx, []byte(m.Payload))
}

// AdvanceRecordedCursor requires a persisted job and an atomic cursor revision.
func (s *Store) AdvanceRecordedCursor(ctx context.Context, req recorded.CursorAdvance) (*recorded.SourceCursor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.GetRecordedJob(ctx, req.JobID)
	if err != nil {
		return nil, err
	}
	current, err := s.GetRecordedCursor(ctx, req.BindingID)
	if errors.Is(err, recorded.ErrNotFound) {
		current = nil
	} else if err != nil {
		return nil, err
	}
	next, err := recorded.NextCursor(ctx, current, j, req, time.Now())
	if err != nil {
		return nil, err
	}
	if current != nil && next.Revision == current.Revision {
		return current, nil
	}
	data, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	m := &recordedCursorModel{ID: recorded.CursorStorageKey(ctx, req.BindingID), AppID: next.AppID, TenantID: next.TenantID, BindingID: next.BindingID, Revision: next.Revision, Payload: string(data)}
	var n int64
	if current == nil {
		result, insertErr := s.sdb.NewInsert(m).OnConflict("DO NOTHING").Exec(ctx)
		if insertErr != nil {
			return nil, insertErr
		}
		n, err = result.RowsAffected()
	} else {
		result, updateErr := s.sdb.NewUpdate((*recordedCursorModel)(nil)).Set("payload = ?", string(data)).Set("revision = ?", next.Revision).Where("id = ?", m.ID).Where("app_id = ?", next.AppID).Where("tenant_id = ?", next.TenantID).Where("revision = ?", current.Revision).Exec(ctx)
		if updateErr != nil {
			return nil, updateErr
		}
		n, err = result.RowsAffected()
	}
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, recorded.ErrConflict
	}
	return next, nil
}

func init() {
	Migrations.MustRegister(&migrate.Migration{Name: "create_recorded_cursors", Version: "20240101000008",
		Up: func(ctx context.Context, exec migrate.Executor) error {
			_, err := exec.Exec(ctx, `CREATE TABLE IF NOT EXISTS sentinel_recorded_cursors (id TEXT PRIMARY KEY, app_id TEXT NOT NULL, tenant_id TEXT NOT NULL, binding_id TEXT NOT NULL, revision BIGINT NOT NULL, payload TEXT NOT NULL)`)
			return err
		},
		Down: func(ctx context.Context, exec migrate.Executor) error {
			_, err := exec.Exec(ctx, `DROP TABLE IF EXISTS sentinel_recorded_cursors`)
			return err
		},
	})
}
