package scorer

import (
	"context"
	"fmt"
)

// LatencyScorer scores 1.0 if the latency is within the threshold.
// Latency is read from input.Context["latency_ms"].
type LatencyScorer struct {
	MaxMs int
}

func (s *LatencyScorer) Name() string { return "latency" }

func (s *LatencyScorer) Score(_ context.Context, input *ScorerInput) (*ScorerOutput, error) {
	latencyMs := 0
	if v, ok := input.Context["latency_ms"]; ok {
		switch val := v.(type) {
		case int:
			latencyMs = val
		case float64:
			latencyMs = int(val)
		}
	}

	passed := s.MaxMs <= 0 || latencyMs <= s.MaxMs
	score := 0.0
	if passed {
		score = 1.0
	}
	return &ScorerOutput{
		Score:  score,
		Passed: passed,
		Reason: fmt.Sprintf("latency: %dms (max=%dms)", latencyMs, s.MaxMs),
		Details: map[string]any{
			"latency_ms": latencyMs,
		},
	}, nil
}
