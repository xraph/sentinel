package scenario

import (
	"context"
	"fmt"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/testcase"
)

// TraitProbeGenerator creates test cases that probe personality trait
// consistency across different contexts.
type TraitProbeGenerator struct {
	// TraitName is the trait being tested.
	TraitName string
	// TraitDesc describes the expected trait behavior.
	TraitDesc string
}

// NewTraitProbeGenerator creates a trait probe generator.
func NewTraitProbeGenerator(traitName, traitDesc string) *TraitProbeGenerator {
	return &TraitProbeGenerator{TraitName: traitName, TraitDesc: traitDesc}
}

func (g *TraitProbeGenerator) Name() string { return "trait_probe" }

func (g *TraitProbeGenerator) Type() testcase.ScenarioType {
	return testcase.ScenarioTraitProbe
}

func (g *TraitProbeGenerator) Generate(_ context.Context, cfg *GenerateConfig) ([]*testcase.Case, error) {
	count := cfg.Count
	if count <= 0 {
		count = 3
	}

	suiteID, err := id.ParseSuiteID(cfg.SuiteID)
	if err != nil {
		return nil, fmt.Errorf("trait_probe: invalid suite ID: %w", err)
	}

	// Generate probes that test the trait in different contexts.
	probeContexts := []string{
		"under normal conditions",
		"when challenged or contradicted",
		"when handling a complex request",
	}

	cases := make([]*testcase.Case, 0, count)
	for i := 0; i < count && i < len(probeContexts); i++ {
		tc := &testcase.Case{
			Entity:       sentinel.NewEntity(),
			ID:           id.NewCaseID(),
			SuiteID:      suiteID,
			Name:         fmt.Sprintf("trait_probe_%s_%d", g.TraitName, i+1),
			Input:        fmt.Sprintf("Respond to this prompt %s, demonstrating your characteristic %s: Tell me about yourself and how you approach problems.", probeContexts[i], g.TraitName),
			ScenarioType: testcase.ScenarioTraitProbe,
			Scorers: []testcase.ScorerConfig{
				{Name: "trait_consistency", Config: map[string]any{"trait_name": g.TraitName, "trait_desc": g.TraitDesc}},
			},
			Tags: []string{"trait", "auto-generated"},
			Context: map[string]any{
				"trait_name":    g.TraitName,
				"trait_desc":    g.TraitDesc,
				"probe_context": probeContexts[i],
			},
		}
		cases = append(cases, tc)
	}

	return cases, nil
}
