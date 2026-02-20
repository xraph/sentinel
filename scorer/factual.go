package scorer

import (
	"context"
	"fmt"
)

// FactualScorer evaluates factual accuracy using an LLM judge.
type FactualScorer struct {
	Client LLMClient
	Model  string
}

// NewFactualScorer creates a factual accuracy scorer.
func NewFactualScorer(client LLMClient, model string) *FactualScorer {
	return &FactualScorer{Client: client, Model: model}
}

func (s *FactualScorer) Name() string { return "factual" }

func (s *FactualScorer) Score(ctx context.Context, input *ScorerInput) (*ScorerOutput, error) {
	systemPrompt := `You are a factual accuracy evaluator. Compare the AI output against the expected/reference output for factual correctness.

Respond with ONLY a JSON object:
{"score": <0.0-1.0>, "reason": "<explanation>"}

Score 1.0 means perfectly factually accurate. Score 0.0 means completely inaccurate.`

	userPrompt := fmt.Sprintf(`User Input: %s

Reference/Expected: %s

AI Output to evaluate: %s

Rate the factual accuracy of the AI output compared to the reference.`,
		input.Input, input.Expected, input.Actual)

	response, err := s.Client.Complete(ctx, s.Model, systemPrompt, userPrompt, 0)
	if err != nil {
		return nil, fmt.Errorf("factual: %w", err)
	}

	score, reason := parseLLMScore(response)

	return &ScorerOutput{
		Score:  score,
		Passed: score >= 0.7,
		Reason: reason,
		Details: map[string]any{
			"raw_response": response,
		},
	}, nil
}
