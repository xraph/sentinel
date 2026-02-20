package scorer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// LLMClient is the interface for calling a language model for scoring.
type LLMClient interface {
	Complete(ctx context.Context, model, systemPrompt, userInput string, temperature float64) (string, error)
}

// LLMJudgeScorer uses an LLM as a judge to evaluate outputs.
type LLMJudgeScorer struct {
	Client      LLMClient
	Model       string
	Criteria    string
	Temperature float64
}

// NewLLMJudgeScorer creates a new LLM-as-judge scorer.
func NewLLMJudgeScorer(client LLMClient, model, criteria string) *LLMJudgeScorer {
	return &LLMJudgeScorer{
		Client:   client,
		Model:    model,
		Criteria: criteria,
	}
}

func (s *LLMJudgeScorer) Name() string { return "llm_judge" }

func (s *LLMJudgeScorer) Score(ctx context.Context, input *ScorerInput) (*ScorerOutput, error) {
	systemPrompt := `You are an evaluation judge. Score the following AI output based on the given criteria.
Respond with ONLY a JSON object in this exact format:
{"score": <0.0-1.0>, "reason": "<brief explanation>"}
Do not include any other text.`

	userPrompt := fmt.Sprintf(`Criteria: %s

User Input: %s

Expected Output: %s

Actual Output: %s

Score the actual output from 0.0 (completely wrong) to 1.0 (perfect).`,
		s.Criteria, input.Input, input.Expected, input.Actual)

	response, err := s.Client.Complete(ctx, s.Model, systemPrompt, userPrompt, s.Temperature)
	if err != nil {
		return nil, fmt.Errorf("llm_judge: %w", err)
	}

	score, reason := parseLLMScore(response)

	return &ScorerOutput{
		Score:  score,
		Passed: score >= 0.7,
		Reason: reason,
		Details: map[string]any{
			"raw_response": response,
			"criteria":     s.Criteria,
		},
	}, nil
}

// parseLLMScore extracts a score and reason from an LLM response.
// It handles both JSON and plain text formats.
func parseLLMScore(response string) (float64, string) {
	response = strings.TrimSpace(response)

	// Try to find score in JSON-like format.
	if idx := strings.Index(response, `"score"`); idx >= 0 {
		rest := response[idx:]
		if colonIdx := strings.Index(rest, ":"); colonIdx >= 0 {
			numStr := strings.TrimSpace(rest[colonIdx+1:])
			// Find the end of the number.
			end := 0
			for end < len(numStr) && (numStr[end] == '.' || (numStr[end] >= '0' && numStr[end] <= '9')) {
				end++
			}
			if end > 0 {
				if score, err := strconv.ParseFloat(numStr[:end], 64); err == nil {
					reason := "LLM judge evaluation"
					if rIdx := strings.Index(response, `"reason"`); rIdx >= 0 {
						rRest := response[rIdx:]
						if rColon := strings.Index(rRest, ":"); rColon >= 0 {
							rVal := strings.TrimSpace(rRest[rColon+1:])
							if len(rVal) > 0 && rVal[0] == '"' {
								rVal = rVal[1:]
								if endQuote := strings.Index(rVal, `"`); endQuote >= 0 {
									reason = rVal[:endQuote]
								}
							}
						}
					}
					return clampScore(score), reason
				}
			}
		}
	}

	return 0.5, "could not parse LLM response"
}

// clampScore ensures a score is within [0.0, 1.0].
func clampScore(s float64) float64 {
	if s < 0 {
		return 0
	}
	if s > 1 {
		return 1
	}
	return s
}
