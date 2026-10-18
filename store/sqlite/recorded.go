package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/xraph/grove"
	"github.com/xraph/grove/migrate"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
)

type recordedJobModel struct {
	grove.BaseModel `grove:"table:sentinel_recorded_jobs"`
	ID              string `grove:"id,pk"`
	JobKey          string `grove:"job_key,notnull"`
	AppID           string `grove:"app_id,notnull"`
	TenantID        string `grove:"tenant_id,notnull"`
	Revision        int64  `grove:"revision,notnull"`
	State           string `grove:"state,notnull"`
	Payload         string `grove:"payload,notnull"`
}

// CreateRecordedJob persists a request once per exact scope and idempotency key.
func (s *Store) CreateRecordedJob(ctx context.Context, j *recorded.Job) (*recorded.Job, error) {
	if err := j.ValidateNew(ctx); err != nil {
		return nil, err
	}
	data, err := recorded.Encode(ctx, j)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m := &recordedJobModel{ID: j.ID.String(), JobKey: j.StorageKey(), AppID: j.AppID, TenantID: j.TenantID, Revision: j.Revision, State: string(j.State), Payload: string(data)}
	result, err := s.sdb.NewInsert(m).OnConflict("DO NOTHING").Exec(ctx)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return recorded.Decode(ctx, data)
	}
	existing := new(recordedJobModel)
	err = s.sdb.NewSelect(existing).Where("job_key = ?", j.StorageKey()).Where("app_id = ?", j.AppID).Where("tenant_id = ?", j.TenantID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, recorded.ErrConflict
	}
	if err != nil {
		return nil, err
	}
	old, err := recorded.Decode(ctx, []byte(existing.Payload))
	if err != nil {
		return nil, err
	}
	if old.RequestDigest != j.RequestDigest {
		return nil, recorded.ErrConflict
	}
	return old, nil
}

// GetRecordedJob reads only the exact app and tenant's evidence.
func (s *Store) GetRecordedJob(ctx context.Context, jobID id.ID) (*recorded.Job, error) {
	if sentinel.AppFromContext(ctx) == "" {
		return nil, recorded.ErrNotFound
	}
	m := new(recordedJobModel)
	err := s.sdb.NewSelect(m).Where("id = ?", jobID.String()).Where("app_id = ?", sentinel.AppFromContext(ctx)).Where("tenant_id = ?", sentinel.TenantFromContext(ctx)).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, recorded.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return recorded.Decode(ctx, []byte(m.Payload))
}

// ListRecordedJobs returns a stable ID page within the exact scope.
func (s *Store) ListRecordedJobs(ctx context.Context, query recorded.Query) ([]*recorded.Job, error) {
	if sentinel.AppFromContext(ctx) == "" {
		return nil, recorded.ErrNotFound
	}
	limit, err := query.PageLimit()
	if err != nil {
		return nil, err
	}
	var models []recordedJobModel
	q := s.sdb.NewSelect(&models).Where("app_id = ?", sentinel.AppFromContext(ctx)).Where("tenant_id = ?", sentinel.TenantFromContext(ctx)).Where("id > ?", query.After).OrderExpr("id ASC").Limit(limit)
	if query.Active {
		q = q.Where("state IN ('queued', 'running')")
	}
	if scanErr := q.Scan(ctx); scanErr != nil {
		return nil, scanErr
	}
	jobs := make([]*recorded.Job, 0, len(models))
	for _, m := range models {
		j, decodeErr := recorded.Decode(ctx, []byte(m.Payload))
		if decodeErr != nil {
			return nil, decodeErr
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// UpdateRecordedJob atomically compares the revision before a fenced checkpoint.
func (s *Store) UpdateRecordedJob(ctx context.Context, expected int64, j *recorded.Job) error {
	data, err := recorded.Encode(ctx, j)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.GetRecordedJob(ctx, j.ID)
	if err != nil {
		return err
	}
	if old.Revision != expected {
		return recorded.ErrConflict
	}
	if changeErr := recorded.ValidateChange(ctx, old, j, time.Now().UTC()); changeErr != nil {
		return changeErr
	}
	result, err := s.sdb.NewUpdate((*recordedJobModel)(nil)).Set("payload = ?", string(data)).Set("revision = ?", j.Revision).Set("state = ?", string(j.State)).Where("id = ?", j.ID.String()).Where("app_id = ?", j.AppID).Where("tenant_id = ?", j.TenantID).Where("revision = ?", expected).Exec(ctx)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return recorded.ErrConflict
	}
	return nil
}

func init() {
	Migrations.MustRegister(&migrate.Migration{Name: "create_recorded_jobs", Version: "20240101000007",
		Up: func(ctx context.Context, exec migrate.Executor) error {
			_, err := exec.Exec(ctx, `CREATE TABLE IF NOT EXISTS sentinel_recorded_jobs (id TEXT PRIMARY KEY, job_key TEXT NOT NULL UNIQUE, app_id TEXT NOT NULL, tenant_id TEXT NOT NULL, revision BIGINT NOT NULL, state TEXT NOT NULL, payload TEXT NOT NULL)`)
			if err != nil {
				return err
			}
			_, err = exec.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_sentinel_recorded_jobs_scope ON sentinel_recorded_jobs (app_id, tenant_id, state, id)`)
			return err
		},
		Down: func(ctx context.Context, exec migrate.Executor) error {
			_, err := exec.Exec(ctx, `DROP TABLE IF EXISTS sentinel_recorded_jobs`)
			return err
		},
	})
}
