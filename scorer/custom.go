package scorer

import "context"

// ScorerFunc is a function that implements the Scorer interface.
type ScorerFunc func(ctx context.Context, input *ScorerInput) (*ScorerOutput, error)

// CustomScorer wraps a user-provided function as a Scorer.
type CustomScorer struct {
	name string
	fn   ScorerFunc
}

// FromFunc creates a named scorer from a function.
func FromFunc(name string, fn ScorerFunc) *CustomScorer {
	return &CustomScorer{name: name, fn: fn}
}

func (s *CustomScorer) Name() string { return s.name }

func (s *CustomScorer) Score(ctx context.Context, input *ScorerInput) (*ScorerOutput, error) {
	return s.fn(ctx, input)
}
