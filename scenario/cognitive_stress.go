package scenario

import (
	"context"
	"fmt"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/testcase"
)

// CognitiveStressGenerator creates test cases that stress-test an agent's
// thinking strategy transitions under varying complexity.
type CognitiveStressGenerator struct {
	// ExpectedPhases are the cognitive phases the agent should progress through.
	ExpectedPhases []string
}

// NewCognitiveStressGenerator creates a cognitive stress generator.
func NewCognitiveStressGenerator(phases []string) *CognitiveStressGenerator {
	return &CognitiveStressGenerator{ExpectedPhases: phases}
}

func (g *CognitiveStressGenerator) Name() string { return "cognitive_stress" }

func (g *CognitiveStressGenerator) Type() testcase.ScenarioType {
	return testcase.ScenarioCognitiveStress
}

func (g *CognitiveStressGenerator) Generate(_ context.Context, cfg *GenerateConfig) ([]*testcase.Case, error) {
	count := cfg.Count
	if count <= 0 {
		count = 3
	}

	suiteID, err := id.ParseSuiteID(cfg.SuiteID)
	if err != nil {
		return nil, fmt.Errorf("cognitive_stress: invalid suite ID: %w", err)
	}

	stressLevels := []struct {
		name   string
		prompt string
	}{
		{"simple", "Answer this straightforward question: What is 2+2?"},
		{"moderate", "Analyze this multi-step problem: A company has 3 departments, each with different budgets. Department A has $100K, B has $150K, C has $200K. If we need to cut 20% total, how should we distribute the cuts fairly?"},
		{"complex", "Solve this complex scenario that requires careful reasoning: Design a system that handles concurrent user requests while maintaining data consistency, considering edge cases like network failures, race conditions, and partial updates."},
	}

	cases := make([]*testcase.Case, 0, count)
	for i := 0; i < count && i < len(stressLevels); i++ {
		lvl := stressLevels[i]
		tc := &testcase.Case{
			Entity:       sentinel.NewEntity(),
			ID:           id.NewCaseID(),
			SuiteID:      suiteID,
			Name:         fmt.Sprintf("cognitive_stress_%s", lvl.name),
			Input:        lvl.prompt,
			ScenarioType: testcase.ScenarioCognitiveStress,
			Scorers: []testcase.ScorerConfig{
				{Name: "cognitive_phase", Config: map[string]any{"expected_phases": g.ExpectedPhases}},
			},
			Tags: []string{"cognition", "auto-generated"},
			Context: map[string]any{
				"stress_level":    lvl.name,
				"expected_phases": g.ExpectedPhases,
			},
		}
		cases = append(cases, tc)
	}

	return cases, nil
}
