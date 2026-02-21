// Package scorer defines the interface and built-in implementations for
// evaluating AI outputs. Scorers range from deterministic string matchers
// to AI-based judges and persona-aware evaluators.
package scorer

import (
	"context"

	"github.com/xraph/sentinel/evalrun"
)

// Scorer evaluates an AI output and returns a score between 0.0 and 1.0.
type Scorer interface {
	Name() string
	Score(ctx context.Context, input *Input) (*Output, error)
}

// Input provides all data needed for scoring.
type Input struct {
	Input    string            // User input / prompt
	Expected string            // Expected output (optional)
	Actual   string            // Actual output
	Trace    *evalrun.RunTrace // Agent execution trace (optional, for persona scorers)
	Context  map[string]any    // Additional context
}

// Output captures the result of a scoring evaluation.
type Output struct {
	Score     float64        // 0.0 to 1.0
	Passed    bool           // Score >= threshold
	Reason    string         // Human-readable explanation
	Dimension string         // Which evaluation dimension (skill, trait, etc.)
	Details   map[string]any // Scorer-specific details
}
