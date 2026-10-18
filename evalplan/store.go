package evalplan

import (
	"context"

	"github.com/xraph/sentinel/id"
)

// Store persists immutable plans with exact app and tenant scope.
type Store interface {
	CreateEvaluationPlan(context.Context, *Plan) error
	GetEvaluationPlan(context.Context, id.ID) (*Plan, error)
}
