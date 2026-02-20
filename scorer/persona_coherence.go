package scorer

import (
	"context"
	"fmt"
)

// PersonaCoherenceScorer performs holistic LLM-based evaluation of whether
// the agent maintains a consistent identity across all dimensions.
type PersonaCoherenceScorer struct {
	Client      LLMClient
	Model       string
	PersonaDesc string
}

// NewPersonaCoherenceScorer creates a persona coherence scorer.
func NewPersonaCoherenceScorer(client LLMClient, model, personaDesc string) *PersonaCoherenceScorer {
	return &PersonaCoherenceScorer{
		Client:      client,
		Model:       model,
		PersonaDesc: personaDesc,
	}
}

func (s *PersonaCoherenceScorer) Name() string { return "persona_coherence" }

func (s *PersonaCoherenceScorer) Score(ctx context.Context, input *ScorerInput) (*ScorerOutput, error) {
	systemPrompt := `You are an AI persona evaluator. Assess whether the agent output maintains coherent identity — consistent personality, appropriate tool usage, matching communication style, and expected cognitive approach.

Respond with ONLY a JSON object:
{"score": <0.0-1.0>, "reason": "<explanation>"}

Score 1.0 means perfectly coherent persona. Score 0.0 means the agent showed no consistency with its defined persona.`

	traceInfo := "No execution trace available."
	if input.Trace != nil {
		traceInfo = fmt.Sprintf("Tool calls: %d, Steps: %d", len(input.Trace.ToolCalls), len(input.Trace.Steps))
	}

	userPrompt := fmt.Sprintf(`Persona Description: %s

User Input: %s

AI Output: %s

Execution Info: %s

Rate overall persona coherence — does the output feel like it comes from the described persona?`,
		s.PersonaDesc, input.Input, input.Actual, traceInfo)

	response, err := s.Client.Complete(ctx, s.Model, systemPrompt, userPrompt, 0)
	if err != nil {
		return nil, fmt.Errorf("persona_coherence: %w", err)
	}

	score, reason := parseLLMScore(response)

	return &ScorerOutput{
		Score:     score,
		Passed:    score >= 0.7,
		Reason:    reason,
		Dimension: "persona",
		Details: map[string]any{
			"persona_desc": s.PersonaDesc,
			"raw_response": response,
		},
	}, nil
}
