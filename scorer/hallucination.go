package scorer

import (
	"context"
	"fmt"
)

// HallucinationScorer detects hallucinated content using an LLM judge.
type HallucinationScorer struct {
	Client LLMClient
	Model  string
}

// NewHallucinationScorer creates a hallucination detection scorer.
func NewHallucinationScorer(client LLMClient, model string) *HallucinationScorer {
	return &HallucinationScorer{Client: client, Model: model}
}

func (s *HallucinationScorer) Name() string { return "hallucination" }

func (s *HallucinationScorer) Score(ctx context.Context, input *ScorerInput) (*ScorerOutput, error) {
	systemPrompt := `You are a hallucination detector. Analyze the AI output for factual claims that are not supported by the provided context or expected output.

Respond with ONLY a JSON object:
{"score": <0.0-1.0>, "reason": "<explanation>"}

Score 1.0 means no hallucination detected. Score 0.0 means severe hallucination.`

	userPrompt := fmt.Sprintf(`User Input: %s

Reference/Expected: %s

AI Output to evaluate: %s

Rate how factually grounded the AI output is (1.0 = fully grounded, 0.0 = completely hallucinated).`,
		input.Input, input.Expected, input.Actual)

	response, err := s.Client.Complete(ctx, s.Model, systemPrompt, userPrompt, 0)
	if err != nil {
		return nil, fmt.Errorf("hallucination: %w", err)
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
