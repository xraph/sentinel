package scorer

import (
	"context"
	"fmt"
	"strings"
)

// LengthScorer scores based on the token count of the output.
// Uses a simple word-split approximation for token counting.
type LengthScorer struct {
	MinTokens int
	MaxTokens int
}

func (s *LengthScorer) Name() string { return "length" }

func (s *LengthScorer) Score(_ context.Context, input *Input) (*Output, error) {
	tokens := len(strings.Fields(input.Actual))
	passed := (s.MinTokens <= 0 || tokens >= s.MinTokens) && (s.MaxTokens <= 0 || tokens <= s.MaxTokens)

	score := 0.0
	if passed {
		score = 1.0
	}
	return &Output{
		Score:  score,
		Passed: passed,
		Reason: fmt.Sprintf("length: %d tokens (min=%d, max=%d)", tokens, s.MinTokens, s.MaxTokens),
		Details: map[string]any{
			"tokens": tokens,
		},
	}, nil
}
