package scenario

import (
	"context"
	"fmt"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/testcase"
)

// PerceptionGenerator creates test cases that evaluate the agent's ability
// to focus on relevant information and ignore distractors.
type PerceptionGenerator struct {
	// RelevantKeywords are terms the agent should attend to.
	RelevantKeywords []string
	// DistractorKeywords are terms the agent should ignore.
	DistractorKeywords []string
}

// NewPerceptionGenerator creates a perception test generator.
func NewPerceptionGenerator(relevant, distractors []string) *PerceptionGenerator {
	return &PerceptionGenerator{
		RelevantKeywords:   relevant,
		DistractorKeywords: distractors,
	}
}

func (g *PerceptionGenerator) Name() string { return "perception" }

func (g *PerceptionGenerator) Type() testcase.ScenarioType {
	return testcase.ScenarioPerceptionTest
}

func (g *PerceptionGenerator) Generate(_ context.Context, cfg *GenerateConfig) ([]*testcase.Case, error) {
	count := cfg.Count
	if count <= 0 {
		count = 2
	}

	suiteID, err := id.ParseSuiteID(cfg.SuiteID)
	if err != nil {
		return nil, fmt.Errorf("perception: invalid suite ID: %w", err)
	}

	cases := make([]*testcase.Case, 0, count)

	// Case 1: Focus test with relevant keywords.
	cases = append(cases, &testcase.Case{
		Entity:       sentinel.NewEntity(),
		ID:           id.NewCaseID(),
		SuiteID:      suiteID,
		Name:         "perception_focus_relevant",
		Input:        fmt.Sprintf("Given these key topics: %v — summarize the most important points and ignore any distractions.", g.RelevantKeywords),
		ScenarioType: testcase.ScenarioPerceptionTest,
		Scorers: []testcase.ScorerConfig{
			{Name: "perception_focus", Config: map[string]any{
				"required_keywords": g.RelevantKeywords,
				"ignored_keywords":  g.DistractorKeywords,
			}},
		},
		Tags: []string{"perception", "auto-generated"},
		Context: map[string]any{
			"relevant_keywords":   g.RelevantKeywords,
			"distractor_keywords": g.DistractorKeywords,
		},
	})

	// Case 2: Distractor resistance test.
	if count >= 2 {
		cases = append(cases, &testcase.Case{
			Entity:       sentinel.NewEntity(),
			ID:           id.NewCaseID(),
			SuiteID:      suiteID,
			Name:         "perception_distractor_resistance",
			Input:        fmt.Sprintf("Focus only on %v and do not mention %v in your response. Explain the key topics concisely.", g.RelevantKeywords, g.DistractorKeywords),
			ScenarioType: testcase.ScenarioPerceptionTest,
			Scorers: []testcase.ScorerConfig{
				{Name: "perception_focus", Config: map[string]any{
					"required_keywords": g.RelevantKeywords,
					"ignored_keywords":  g.DistractorKeywords,
				}},
			},
			Tags: []string{"perception", "auto-generated"},
			Context: map[string]any{
				"relevant_keywords":   g.RelevantKeywords,
				"distractor_keywords": g.DistractorKeywords,
				"test_type":           "distractor_resistance",
			},
		})
	}

	return cases, nil
}
