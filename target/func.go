package target

import (
	"context"
	"time"
)

// FuncTarget wraps a plain function as an eval target.
type FuncTarget struct {
	name string
	fn   func(ctx context.Context, input string) (string, error)
}

// FromFunc creates a named target from a function. The function receives the
// input string and returns the output. Latency is measured automatically.
func FromFunc(name string, fn func(ctx context.Context, input string) (string, error)) *FuncTarget {
	return &FuncTarget{name: name, fn: fn}
}

func (t *FuncTarget) Name() string { return t.name }

func (t *FuncTarget) Call(ctx context.Context, input string) (*TargetOutput, error) {
	start := time.Now()
	output, err := t.fn(ctx, input)
	elapsed := time.Since(start)
	if err != nil {
		return nil, err
	}
	return &TargetOutput{
		Output:  output,
		Latency: elapsed,
	}, nil
}
