package scorer

import (
	"context"
	"fmt"
	"strings"
)

// PerceptionFocusScorer evaluates whether the agent attended to the right
// keywords and patterns in its input, measuring perceptual focus.
type PerceptionFocusScorer struct {
	// RequiredKeywords are terms the output must reference.
	RequiredKeywords []string
	// IgnoredKeywords are distractors the output should not focus on.
	IgnoredKeywords []string
}

// NewPerceptionFocusScorer creates a perception focus scorer.
func NewPerceptionFocusScorer(required, ignored []string) *PerceptionFocusScorer {
	return &PerceptionFocusScorer{RequiredKeywords: required, IgnoredKeywords: ignored}
}

func (s *PerceptionFocusScorer) Name() string { return "perception_focus" }

func (s *PerceptionFocusScorer) Score(ctx context.Context, input *ScorerInput) (*ScorerOutput, error) {
	outputLower := strings.ToLower(input.Actual)

	// Check required keywords.
	requiredFound := 0
	var missingKeywords []string
	for _, kw := range s.RequiredKeywords {
		if strings.Contains(outputLower, strings.ToLower(kw)) {
			requiredFound++
		} else {
			missingKeywords = append(missingKeywords, kw)
		}
	}

	// Check ignored keywords (distractors).
	distractorFocused := 0
	var focusedDistractors []string
	for _, kw := range s.IgnoredKeywords {
		if strings.Contains(outputLower, strings.ToLower(kw)) {
			distractorFocused++
			focusedDistractors = append(focusedDistractors, kw)
		}
	}

	// Calculate score.
	var score float64
	if len(s.RequiredKeywords) > 0 {
		score = float64(requiredFound) / float64(len(s.RequiredKeywords))
	} else {
		score = 1.0
	}

	// Penalize for distractor focus.
	if len(s.IgnoredKeywords) > 0 && distractorFocused > 0 {
		penalty := float64(distractorFocused) / float64(len(s.IgnoredKeywords)) * 0.3
		score = clampScore(score - penalty)
	}

	reason := fmt.Sprintf("attended to %d/%d required keywords", requiredFound, len(s.RequiredKeywords))
	if len(focusedDistractors) > 0 {
		reason += fmt.Sprintf("; distracted by: %s", strings.Join(focusedDistractors, ", "))
	}

	return &ScorerOutput{
		Score:     score,
		Passed:    score >= 0.7,
		Reason:    reason,
		Dimension: "perception",
		Details: map[string]any{
			"required_found":     requiredFound,
			"missing_keywords":   missingKeywords,
			"distractor_focused": distractorFocused,
			"focused_distractors": focusedDistractors,
		},
	}, nil
}
