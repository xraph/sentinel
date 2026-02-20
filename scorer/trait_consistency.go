package scorer

import (
	"context"
	"fmt"
)

// TraitConsistencyScorer uses an LLM judge to assess whether the agent's
// output is consistent with a specified personality trait.
type TraitConsistencyScorer struct {
	Client    LLMClient
	Model     string
	TraitName string
	TraitDesc string
}

// NewTraitConsistencyScorer creates a trait consistency scorer.
func NewTraitConsistencyScorer(client LLMClient, model, traitName, traitDesc string) *TraitConsistencyScorer {
	return &TraitConsistencyScorer{
		Client:    client,
		Model:     model,
		TraitName: traitName,
		TraitDesc: traitDesc,
	}
}

func (s *TraitConsistencyScorer) Name() string { return "trait_consistency" }

func (s *TraitConsistencyScorer) Score(ctx context.Context, input *ScorerInput) (*ScorerOutput, error) {
	systemPrompt := `You are a personality consistency evaluator. Assess whether the AI output demonstrates the specified personality trait.

Respond with ONLY a JSON object:
{"score": <0.0-1.0>, "reason": "<explanation>"}

Score 1.0 means the trait is perfectly demonstrated. Score 0.0 means the trait is completely absent or contradicted.`

	userPrompt := fmt.Sprintf(`Trait: %s
Trait Description: %s

User Input: %s

AI Output: %s

Rate how consistently the AI output demonstrates the specified trait.`,
		s.TraitName, s.TraitDesc, input.Input, input.Actual)

	response, err := s.Client.Complete(ctx, s.Model, systemPrompt, userPrompt, 0)
	if err != nil {
		return nil, fmt.Errorf("trait_consistency: %w", err)
	}

	score, reason := parseLLMScore(response)

	return &ScorerOutput{
		Score:     score,
		Passed:    score >= 0.7,
		Reason:    reason,
		Dimension: "trait",
		Details: map[string]any{
			"trait_name":   s.TraitName,
			"trait_desc":   s.TraitDesc,
			"raw_response": response,
		},
	}, nil
}
