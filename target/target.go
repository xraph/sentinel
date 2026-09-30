// Package target defines the interface for evaluation targets — the system
// under test. Targets can be raw LLM APIs, agents, or plain functions.
package target

import (
	"context"
	"time"

	"github.com/xraph/sentinel/evalrun"
)

// Target is the interface for evaluation targets.
type Target interface {
	Name() string
	Call(ctx context.Context, input string) (*Output, error)
}

// Output includes both the text output and optional execution trace.
type Output struct {
	Output  string            // The text response
	Latency time.Duration     // Response time
	Tokens  int               // Tokens used
	Cost    float64           // Estimated cost
	Trace   *evalrun.RunTrace // Agent execution trace (nil for raw LLM targets)
}

// CallOptions carry what the run decided for this call: the prompt of the
// suite's current prompt version (or the suite's own prompt), the model and
// the temperature. A target that builds its own request should prefer these
// to whatever it was constructed with, so the prompt a run records is the
// prompt the model actually saw.
type CallOptions struct {
	SystemPrompt string
	Model        string
	Temperature  float64
}

type callOptionsKey struct{}

// WithCallOptions attaches the run's choices to ctx.
func WithCallOptions(ctx context.Context, o CallOptions) context.Context {
	return context.WithValue(ctx, callOptionsKey{}, o)
}

// CallOptionsFrom returns the run's choices, if the engine attached any.
func CallOptionsFrom(ctx context.Context) (CallOptions, bool) {
	o, ok := ctx.Value(callOptionsKey{}).(CallOptions)
	return o, ok
}
