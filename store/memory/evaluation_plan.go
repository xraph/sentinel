package memory

import (
	"context"
	"encoding/json"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalplan"
	"github.com/xraph/sentinel/id"
)

// CreateEvaluationPlan stores a detached immutable snapshot.
func (s *Store) CreateEvaluationPlan(ctx context.Context, p *evalplan.Plan) error {
	data, err := evalplan.Encode(ctx, p)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := p.ID.String()
	if _, exists := s.evaluationPlans[key]; exists {
		return sentinel.ErrEvaluationPlanExists
	}
	s.evaluationPlans[key] = data
	return nil
}

// GetEvaluationPlan reads an exact app and tenant snapshot.
func (s *Store) GetEvaluationPlan(ctx context.Context, planID id.ID) (*evalplan.Plan, error) {
	s.mu.RLock()
	data, exists := s.evaluationPlans[planID.String()]
	s.mu.RUnlock()
	if !exists || sentinel.AppFromContext(ctx) == "" {
		return nil, sentinel.ErrEvaluationPlanNotFound
	}
	var scope struct {
		AppID    string `json:"app_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.Unmarshal(data, &scope); err != nil {
		return nil, err
	}
	if scope.AppID != sentinel.AppFromContext(ctx) || scope.TenantID != sentinel.TenantFromContext(ctx) {
		return nil, sentinel.ErrEvaluationPlanNotFound
	}
	return evalplan.Decode(ctx, data)
}
