package scenario

import (
	"context"
	"fmt"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/testcase"
)

// SkillChallengeGenerator creates test cases that probe an agent's
// tool selection and usage proficiency.
type SkillChallengeGenerator struct {
	// AvailableTools is the list of tools the agent has access to.
	AvailableTools []string
}

// NewSkillChallengeGenerator creates a skill challenge generator.
func NewSkillChallengeGenerator(tools []string) *SkillChallengeGenerator {
	return &SkillChallengeGenerator{AvailableTools: tools}
}

func (g *SkillChallengeGenerator) Name() string { return "skill_challenge" }

func (g *SkillChallengeGenerator) Type() testcase.ScenarioType {
	return testcase.ScenarioSkillChallenge
}

func (g *SkillChallengeGenerator) Generate(_ context.Context, cfg *GenerateConfig) ([]*testcase.Case, error) {
	count := cfg.Count
	if count <= 0 {
		count = 3
	}

	suiteID, err := id.ParseSuiteID(cfg.SuiteID)
	if err != nil {
		return nil, fmt.Errorf("skill_challenge: invalid suite ID: %w", err)
	}

	cases := make([]*testcase.Case, 0, count)
	for i := 0; i < count && i < len(g.AvailableTools); i++ {
		tool := g.AvailableTools[i]
		tc := &testcase.Case{
			Entity:       sentinel.NewEntity(),
			ID:           id.NewCaseID(),
			SuiteID:      suiteID,
			Name:         fmt.Sprintf("skill_challenge_%s_%d", tool, i+1),
			Input:        fmt.Sprintf("Use the %s tool to accomplish the following task. Show your proficiency with this tool.", tool),
			ScenarioType: testcase.ScenarioSkillChallenge,
			Scorers: []testcase.ScorerConfig{
				{Name: "skill_usage", Config: map[string]any{"expected_tools": []string{tool}}},
			},
			Tags: []string{"skill", "auto-generated"},
			Context: map[string]any{
				"target_tool": tool,
				"difficulty":  cfg.Difficulty,
			},
		}
		cases = append(cases, tc)
	}

	return cases, nil
}
