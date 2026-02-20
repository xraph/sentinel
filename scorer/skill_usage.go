package scorer

import (
	"context"
	"fmt"
	"strings"
)

// SkillUsageScorer analyzes agent trace tool calls to verify expected tool
// usage and detect forbidden tool usage.
type SkillUsageScorer struct {
	ExpectedTools  []string
	ForbiddenTools []string
}

// NewSkillUsageScorer creates a skill usage scorer.
func NewSkillUsageScorer(expected, forbidden []string) *SkillUsageScorer {
	return &SkillUsageScorer{ExpectedTools: expected, ForbiddenTools: forbidden}
}

func (s *SkillUsageScorer) Name() string { return "skill_usage" }

func (s *SkillUsageScorer) Score(ctx context.Context, input *ScorerInput) (*ScorerOutput, error) {
	if input.Trace == nil {
		return &ScorerOutput{
			Score:     0,
			Passed:    false,
			Reason:    "no execution trace available",
			Dimension: "skill",
		}, nil
	}

	// Collect all tool names from trace.
	usedTools := make(map[string]bool)
	for _, tc := range input.Trace.ToolCalls {
		usedTools[tc.ToolName] = true
	}

	// Check expected tools.
	expectedFound := 0
	var missingTools []string
	for _, tool := range s.ExpectedTools {
		if usedTools[tool] {
			expectedFound++
		} else {
			missingTools = append(missingTools, tool)
		}
	}

	// Check forbidden tools.
	var forbiddenUsed []string
	for _, tool := range s.ForbiddenTools {
		if usedTools[tool] {
			forbiddenUsed = append(forbiddenUsed, tool)
		}
	}

	// Calculate score.
	var score float64
	if len(s.ExpectedTools) > 0 {
		score = float64(expectedFound) / float64(len(s.ExpectedTools))
	} else {
		score = 1.0
	}

	// Penalize for forbidden tool usage.
	if len(forbiddenUsed) > 0 {
		penalty := float64(len(forbiddenUsed)) * 0.3
		score = clampScore(score - penalty)
	}

	reason := fmt.Sprintf("used %d/%d expected tools", expectedFound, len(s.ExpectedTools))
	if len(missingTools) > 0 {
		reason += fmt.Sprintf("; missing: %s", strings.Join(missingTools, ", "))
	}
	if len(forbiddenUsed) > 0 {
		reason += fmt.Sprintf("; forbidden used: %s", strings.Join(forbiddenUsed, ", "))
	}

	return &ScorerOutput{
		Score:     score,
		Passed:    score >= 0.7,
		Reason:    reason,
		Dimension: "skill",
		Details: map[string]any{
			"expected_tools":  s.ExpectedTools,
			"forbidden_tools": s.ForbiddenTools,
			"used_tools":      toolNames(usedTools),
			"missing_tools":   missingTools,
			"forbidden_used":  forbiddenUsed,
		},
	}, nil
}

func toolNames(m map[string]bool) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	return names
}
