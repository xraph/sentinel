package scenario

import (
	"context"
	"fmt"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/testcase"
)

// CommsAdaptationGenerator creates test cases that evaluate how well
// the agent adapts its communication style to different audiences.
type CommsAdaptationGenerator struct {
	// Audiences describes target audiences and their expected communication style.
	Audiences []Audience
}

// Audience describes a target audience and communication expectations.
type Audience struct {
	Name           string
	Description    string
	ExpectedTone   string
	ExpectedFormat string
}

// NewCommsAdaptationGenerator creates a communication adaptation generator.
func NewCommsAdaptationGenerator(audiences []Audience) *CommsAdaptationGenerator {
	return &CommsAdaptationGenerator{Audiences: audiences}
}

func (g *CommsAdaptationGenerator) Name() string { return "comms_adaptation" }

func (g *CommsAdaptationGenerator) Type() testcase.ScenarioType {
	return testcase.ScenarioCommsAdaptation
}

func (g *CommsAdaptationGenerator) Generate(_ context.Context, cfg *GenerateConfig) ([]*testcase.Case, error) {
	count := cfg.Count
	if count <= 0 {
		count = len(g.Audiences)
	}

	suiteID, err := id.ParseSuiteID(cfg.SuiteID)
	if err != nil {
		return nil, fmt.Errorf("comms_adaptation: invalid suite ID: %w", err)
	}

	cases := make([]*testcase.Case, 0, count)
	for i := 0; i < count && i < len(g.Audiences); i++ {
		aud := g.Audiences[i]
		tc := &testcase.Case{
			Entity:       sentinel.NewEntity(),
			ID:           id.NewCaseID(),
			SuiteID:      suiteID,
			Name:         fmt.Sprintf("comms_adapt_%s", aud.Name),
			Input:        fmt.Sprintf("You are speaking to %s (%s). Explain what you do and how you can help.", aud.Name, aud.Description),
			ScenarioType: testcase.ScenarioCommsAdaptation,
			Scorers: []testcase.ScorerConfig{
				{Name: "communication_style", Config: map[string]any{
					"expected_tone":   aud.ExpectedTone,
					"expected_format": aud.ExpectedFormat,
				}},
			},
			Tags: []string{"communication", "auto-generated"},
			Context: map[string]any{
				"audience_name": aud.Name,
				"audience_desc": aud.Description,
				"expected_tone": aud.ExpectedTone,
			},
		}
		cases = append(cases, tc)
	}

	return cases, nil
}
