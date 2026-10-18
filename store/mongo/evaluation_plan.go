package mongo

import (
	"context"
	"fmt"

	"github.com/xraph/grove"
	"github.com/xraph/grove/drivers/mongodriver/mongomigrate"
	"github.com/xraph/grove/migrate"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalplan"
	"github.com/xraph/sentinel/id"
)

type evaluationPlanModel struct {
	grove.BaseModel `grove:"table:sentinel_evaluation_plans"`
	ID              string `grove:"id,pk" bson:"_id"`
	AppID           string `grove:"app_id" bson:"app_id"`
	TenantID        string `grove:"tenant_id" bson:"tenant_id"`
	Payload         string `grove:"payload" bson:"payload"`
}

// CreateEvaluationPlan inserts an immutable, exactly scoped snapshot.
func (s *Store) CreateEvaluationPlan(ctx context.Context, p *evalplan.Plan) error {
	data, err := evalplan.Encode(ctx, p)
	if err != nil {
		return err
	}
	_, err = s.mdb.NewInsert(&evaluationPlanModel{ID: p.ID.String(), AppID: p.AppID, TenantID: p.TenantID, Payload: string(data)}).Exec(ctx)
	if mongo.IsDuplicateKeyError(err) {
		return sentinel.ErrEvaluationPlanExists
	}
	if err != nil {
		return fmt.Errorf("sentinel: create evaluation plan: %w", err)
	}
	return nil
}

// GetEvaluationPlan reads only the exact app and tenant's snapshot.
func (s *Store) GetEvaluationPlan(ctx context.Context, planID id.ID) (*evalplan.Plan, error) {
	if sentinel.AppFromContext(ctx) == "" {
		return nil, sentinel.ErrEvaluationPlanNotFound
	}
	m := new(evaluationPlanModel)
	err := s.mdb.NewFind(m).Filter(bson.M{"_id": planID.String(), "app_id": sentinel.AppFromContext(ctx), "tenant_id": sentinel.TenantFromContext(ctx)}).Scan(ctx)
	if isNotFound(err) {
		return nil, sentinel.ErrEvaluationPlanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sentinel: get evaluation plan: %w", err)
	}
	return evalplan.Decode(ctx, []byte(m.Payload))
}

func init() {
	Migrations.MustRegister(&migrate.Migration{Name: "create_evaluation_plans", Version: "20240101000007",
		Up: func(ctx context.Context, exec migrate.Executor) error {
			mexec, ok := exec.(*mongomigrate.Executor)
			if !ok {
				return fmt.Errorf("sentinel: expected mongo migration executor, got %T", exec)
			}
			return mexec.CreateCollection(ctx, (*evaluationPlanModel)(nil))
		},
		Down: func(ctx context.Context, exec migrate.Executor) error {
			mexec, ok := exec.(*mongomigrate.Executor)
			if !ok {
				return fmt.Errorf("sentinel: expected mongo migration executor, got %T", exec)
			}
			return mexec.DropCollection(ctx, (*evaluationPlanModel)(nil))
		},
	})
}
