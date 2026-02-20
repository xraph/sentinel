// Package target defines the interface for evaluation targets — the system
// under test. Targets can be raw LLM APIs, Cortex agents, or plain functions.
package target

import (
	"context"
	"time"

	"github.com/xraph/sentinel/evalrun"
)

// Target is the interface for evaluation targets.
type Target interface {
	Name() string
	Call(ctx context.Context, input string) (*TargetOutput, error)
}

// TargetOutput includes both the text output and optional execution trace.
type TargetOutput struct {
	Output  string            // The text response
	Latency time.Duration     // Response time
	Tokens  int               // Tokens used
	Cost    float64           // Estimated cost
	Trace   *evalrun.RunTrace // Agent execution trace (nil for raw LLM targets)
}
