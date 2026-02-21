package scorer

import (
	"context"
	"fmt"
)

// CognitivePhaseScorer analyzes the agent trace for thinking strategy
// transitions — does the agent progress through expected cognitive phases
// (e.g. analysis → planning → execution → verification)?
type CognitivePhaseScorer struct {
	// ExpectedPhases is the ordered list of cognitive phases expected.
	ExpectedPhases []string
}

// NewCognitivePhaseScorer creates a cognitive phase scorer.
func NewCognitivePhaseScorer(expectedPhases []string) *CognitivePhaseScorer {
	return &CognitivePhaseScorer{ExpectedPhases: expectedPhases}
}

func (s *CognitivePhaseScorer) Name() string { return "cognitive_phase" }

func (s *CognitivePhaseScorer) Score(_ context.Context, input *Input) (*Output, error) {
	if input.Trace == nil {
		return &Output{
			Score:     0,
			Passed:    false,
			Reason:    "no execution trace available for cognitive phase analysis",
			Dimension: "cognition",
		}, nil
	}

	if len(s.ExpectedPhases) == 0 {
		return &Output{
			Score:     1.0,
			Passed:    true,
			Reason:    "no expected phases configured",
			Dimension: "cognition",
		}, nil
	}

	// Match trace steps to expected phases in order.
	phaseIdx := 0
	for _, step := range input.Trace.Steps {
		if phaseIdx < len(s.ExpectedPhases) && step.Type == s.ExpectedPhases[phaseIdx] {
			phaseIdx++
		}
	}

	score := float64(phaseIdx) / float64(len(s.ExpectedPhases))
	reason := fmt.Sprintf("completed %d/%d cognitive phases", phaseIdx, len(s.ExpectedPhases))

	return &Output{
		Score:     score,
		Passed:    score >= 0.7,
		Reason:    reason,
		Dimension: "cognition",
		Details: map[string]any{
			"expected_phases": s.ExpectedPhases,
			"phases_matched":  phaseIdx,
			"total_steps":     len(input.Trace.Steps),
		},
	}, nil
}
