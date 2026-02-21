package scorer

import "context"

// Func is a function that implements the Scorer interface.
type Func func(ctx context.Context, input *Input) (*Output, error)

// CustomScorer wraps a user-provided function as a Scorer.
type CustomScorer struct {
	name string
	fn   Func
}

// FromFunc creates a named scorer from a function.
func FromFunc(name string, fn Func) *CustomScorer {
	return &CustomScorer{name: name, fn: fn}
}

func (s *CustomScorer) Name() string { return s.name }

func (s *CustomScorer) Score(ctx context.Context, input *Input) (*Output, error) {
	return s.fn(ctx, input)
}
