package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/xraph/grove"
	"github.com/xraph/grove/migrate"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalplan"
	"github.com/xraph/sentinel/id"
)

type evaluationPlanModel struct {
	grove.BaseModel `grove:"table:sentinel_evaluation_plans"`
	ID              string `grove:"id,pk"`
	AppID           string `grove:"app_id,notnull"`
	TenantID        string `grove:"tenant_id,notnull"`
	Payload         string `grove:"payload,notnull"`
}

// CreateEvaluationPlan inserts immutable content. Updates are intentionally absent.
func (s *Store) CreateEvaluationPlan(ctx context.Context, p *evalplan.Plan) error {
	data, err := evalplan.Encode(ctx, p)
	if err != nil {
		return err
	}

	m := &evaluationPlanModel{ID: p.ID.String(), AppID: p.AppID, TenantID: p.TenantID, Payload: string(data)}
	result, err := s.pgdb.NewInsert(m).OnConflict("(id) DO NOTHING").Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create evaluation plan: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sentinel.ErrEvaluationPlanExists
	}
	return nil
}

// GetEvaluationPlan requires exact app and tenant identity.
func (s *Store) GetEvaluationPlan(ctx context.Context, planID id.ID) (*evalplan.Plan, error) {
	if sentinel.AppFromContext(ctx) == "" {
		return nil, sentinel.ErrEvaluationPlanNotFound
	}
	m := new(evaluationPlanModel)
	err := s.pgdb.NewSelect(m).Where("id = ?", planID.String()).Where("app_id = ?", sentinel.AppFromContext(ctx)).Where("tenant_id = ?", sentinel.TenantFromContext(ctx)).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sentinel.ErrEvaluationPlanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sentinel: get evaluation plan: %w", err)
	}
	return evalplan.Decode(ctx, []byte(m.Payload))
}

func init() {
	Migrations.MustRegister(&migrate.Migration{Name: "create_evaluation_plans", Version: "20240101000006",
		Up: func(ctx context.Context, exec migrate.Executor) error {
			_, err := exec.Exec(ctx, `CREATE TABLE IF NOT EXISTS sentinel_evaluation_plans (id TEXT PRIMARY KEY, app_id TEXT NOT NULL, tenant_id TEXT NOT NULL, payload TEXT NOT NULL)`)
			return err
		},
		Down: func(ctx context.Context, exec migrate.Executor) error {
			_, err := exec.Exec(ctx, `DROP TABLE IF EXISTS sentinel_evaluation_plans`)
			return err
		},
	})
}
