// Package scenario provides test case generators for persona-aware
// evaluation scenarios. Each generator creates test cases targeting
// a specific human-like evaluation dimension.
package scenario

import (
	"context"

	"github.com/xraph/sentinel/testcase"
)

// Generator creates test cases for a specific scenario type.
type Generator interface {
	Name() string
	Type() testcase.ScenarioType
	Generate(ctx context.Context, cfg *GenerateConfig) ([]*testcase.Case, error)
}

// GenerateConfig provides context for test case generation.
type GenerateConfig struct {
	// SuiteID is the target suite.
	SuiteID string
	// PersonaDesc is a description of the persona being tested.
	PersonaDesc string
	// Count is the number of test cases to generate.
	Count int
	// Difficulty is a hint for test difficulty (easy, medium, hard).
	Difficulty string
	// Extra holds generator-specific configuration.
	Extra map[string]any
}
