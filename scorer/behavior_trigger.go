package scorer

import (
	"context"
	"fmt"
	"strings"
)

// BehaviorTriggerScorer analyzes the agent trace to verify that specific
// behaviors were triggered in response to conditions.
type BehaviorTriggerScorer struct {
	// BehaviorName is the name of the behavior being tested.
	BehaviorName string
	// ExpectedActions are actions that should appear in the trace.
	ExpectedActions []string
	// TriggerKeywords are keywords that indicate the trigger was detected.
	TriggerKeywords []string
}

// NewBehaviorTriggerScorer creates a behavior trigger scorer.
func NewBehaviorTriggerScorer(name string, expectedActions, triggerKeywords []string) *BehaviorTriggerScorer {
	return &BehaviorTriggerScorer{
		BehaviorName:    name,
		ExpectedActions: expectedActions,
		TriggerKeywords: triggerKeywords,
	}
}

func (s *BehaviorTriggerScorer) Name() string { return "behavior_trigger" }

func (s *BehaviorTriggerScorer) Score(_ context.Context, input *Input) (*Output, error) {
	if input.Trace == nil {
		return &Output{
			Score:     0,
			Passed:    false,
			Reason:    "no execution trace available for behavior analysis",
			Dimension: "behavior",
		}, nil
	}

	// Check for trigger keyword detection in output or trace.
	triggerDetected := false
	outputLower := strings.ToLower(input.Actual)
	for _, kw := range s.TriggerKeywords {
		if strings.Contains(outputLower, strings.ToLower(kw)) {
			triggerDetected = true
			break
		}
	}

	// Check for expected actions in tool calls.
	actionsFound := 0
	var missingActions []string
	toolNames := make(map[string]bool)
	for _, tc := range input.Trace.ToolCalls {
		toolNames[tc.ToolName] = true
	}

	for _, action := range s.ExpectedActions {
		if toolNames[action] {
			actionsFound++
		} else {
			missingActions = append(missingActions, action)
		}
	}

	// Calculate score.
	var score float64
	if len(s.ExpectedActions) > 0 {
		actionScore := float64(actionsFound) / float64(len(s.ExpectedActions))
		if triggerDetected || len(s.TriggerKeywords) == 0 {
			score = actionScore
		} else {
			score = actionScore * 0.5
		}
	} else if triggerDetected {
		score = 1.0
	}

	reason := fmt.Sprintf("behavior '%s': %d/%d actions triggered",
		s.BehaviorName, actionsFound, len(s.ExpectedActions))
	if !triggerDetected && len(s.TriggerKeywords) > 0 {
		reason += "; trigger not detected in output"
	}

	return &Output{
		Score:     clampScore(score),
		Passed:    score >= 0.7,
		Reason:    reason,
		Dimension: "behavior",
		Details: map[string]any{
			"behavior_name":    s.BehaviorName,
			"trigger_detected": triggerDetected,
			"actions_found":    actionsFound,
			"missing_actions":  missingActions,
		},
	}, nil
}
