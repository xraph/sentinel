package scorer

import (
	"context"
	"fmt"
)

// CostScorer scores 1.0 if the cost is within the threshold.
// Cost is read from input.Context["cost"].
type CostScorer struct {
	MaxCost float64
}

func (s *CostScorer) Name() string { return "cost" }

func (s *CostScorer) Score(_ context.Context, input *ScorerInput) (*ScorerOutput, error) {
	cost := 0.0
	if v, ok := input.Context["cost"]; ok {
		switch val := v.(type) {
		case float64:
			cost = val
		case int:
			cost = float64(val)
		}
	}

	passed := s.MaxCost <= 0 || cost <= s.MaxCost
	score := 0.0
	if passed {
		score = 1.0
	}
	return &ScorerOutput{
		Score:  score,
		Passed: passed,
		Reason: fmt.Sprintf("cost: $%.4f (max=$%.4f)", cost, s.MaxCost),
		Details: map[string]any{
			"cost": cost,
		},
	}, nil
}
