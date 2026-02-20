package scorer

import (
	"context"
	"fmt"
)

// CommunicationStyleScorer uses an LLM to evaluate tone, formality,
// and verbosity of the agent output.
type CommunicationStyleScorer struct {
	Client         LLMClient
	Model          string
	ExpectedTone   string
	ExpectedFormat string
}

// NewCommunicationStyleScorer creates a communication style scorer.
func NewCommunicationStyleScorer(client LLMClient, model, tone, format string) *CommunicationStyleScorer {
	return &CommunicationStyleScorer{
		Client:         client,
		Model:          model,
		ExpectedTone:   tone,
		ExpectedFormat: format,
	}
}

func (s *CommunicationStyleScorer) Name() string { return "communication_style" }

func (s *CommunicationStyleScorer) Score(ctx context.Context, input *ScorerInput) (*ScorerOutput, error) {
	systemPrompt := `You are a communication style evaluator. Assess the AI output for tone, formality, and format consistency.

Respond with ONLY a JSON object:
{"score": <0.0-1.0>, "reason": "<explanation>"}

Score 1.0 means the style perfectly matches expectations. Score 0.0 means completely mismatched.`

	userPrompt := fmt.Sprintf(`Expected tone: %s
Expected format: %s

User Input: %s

AI Output: %s

Rate how well the AI output matches the expected communication style.`,
		s.ExpectedTone, s.ExpectedFormat, input.Input, input.Actual)

	response, err := s.Client.Complete(ctx, s.Model, systemPrompt, userPrompt, 0)
	if err != nil {
		return nil, fmt.Errorf("communication_style: %w", err)
	}

	score, reason := parseLLMScore(response)

	return &ScorerOutput{
		Score:     score,
		Passed:    score >= 0.7,
		Reason:    reason,
		Dimension: "communication",
		Details: map[string]any{
			"expected_tone":   s.ExpectedTone,
			"expected_format": s.ExpectedFormat,
			"raw_response":    response,
		},
	}, nil
}
