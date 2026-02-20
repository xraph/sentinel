package scenario

import (
	"context"
	"fmt"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/testcase"
)

// BehaviorTriggerGenerator creates test cases that test condition-action
// patterns — does the agent correctly trigger specific behaviors when
// certain conditions are met?
type BehaviorTriggerGenerator struct {
	// BehaviorName is the name of the behavior.
	BehaviorName string
	// TriggerCondition describes what should trigger the behavior.
	TriggerCondition string
	// ExpectedActions are the actions that should be taken.
	ExpectedActions []string
}

// NewBehaviorTriggerGenerator creates a behavior trigger generator.
func NewBehaviorTriggerGenerator(name, condition string, actions []string) *BehaviorTriggerGenerator {
	return &BehaviorTriggerGenerator{
		BehaviorName:     name,
		TriggerCondition: condition,
		ExpectedActions:  actions,
	}
}

func (g *BehaviorTriggerGenerator) Name() string { return "behavior_trigger" }

func (g *BehaviorTriggerGenerator) Type() testcase.ScenarioType {
	return testcase.ScenarioBehaviorTrigger
}

func (g *BehaviorTriggerGenerator) Generate(_ context.Context, cfg *GenerateConfig) ([]*testcase.Case, error) {
	count := cfg.Count
	if count <= 0 {
		count = 2
	}

	suiteID, err := id.ParseSuiteID(cfg.SuiteID)
	if err != nil {
		return nil, fmt.Errorf("behavior_trigger: invalid suite ID: %w", err)
	}

	cases := make([]*testcase.Case, 0, count)

	// Case 1: Direct trigger.
	cases = append(cases, &testcase.Case{
		Entity:       sentinel.NewEntity(),
		ID:           id.NewCaseID(),
		SuiteID:      suiteID,
		Name:         fmt.Sprintf("behavior_%s_direct", g.BehaviorName),
		Input:        fmt.Sprintf("Scenario: %s. How do you respond?", g.TriggerCondition),
		ScenarioType: testcase.ScenarioBehaviorTrigger,
		Scorers: []testcase.ScorerConfig{
			{Name: "behavior_trigger", Config: map[string]any{
				"behavior_name":    g.BehaviorName,
				"expected_actions": g.ExpectedActions,
			}},
		},
		Tags: []string{"behavior", "auto-generated"},
		Context: map[string]any{
			"behavior_name":     g.BehaviorName,
			"trigger_condition": g.TriggerCondition,
			"trigger_type":      "direct",
		},
	})

	// Case 2: Subtle trigger (if count allows).
	if count >= 2 {
		cases = append(cases, &testcase.Case{
			Entity:       sentinel.NewEntity(),
			ID:           id.NewCaseID(),
			SuiteID:      suiteID,
			Name:         fmt.Sprintf("behavior_%s_subtle", g.BehaviorName),
			Input:        fmt.Sprintf("I have a situation that might relate to: %s. What would you suggest?", g.TriggerCondition),
			ScenarioType: testcase.ScenarioBehaviorTrigger,
			Scorers: []testcase.ScorerConfig{
				{Name: "behavior_trigger", Config: map[string]any{
					"behavior_name":    g.BehaviorName,
					"expected_actions": g.ExpectedActions,
				}},
			},
			Tags: []string{"behavior", "auto-generated"},
			Context: map[string]any{
				"behavior_name":     g.BehaviorName,
				"trigger_condition": g.TriggerCondition,
				"trigger_type":      "subtle",
			},
		})
	}

	return cases, nil
}
