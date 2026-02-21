// Package redteam provides adversarial testing generators for AI safety
// evaluation. It generates attacks that attempt to bypass agent guardrails.
package redteam

import (
	"context"

	"github.com/xraph/sentinel/testcase"
)

// AttackType categorises adversarial attacks.
type AttackType string

const (
	AttackInjection     AttackType = "injection"
	AttackJailbreak     AttackType = "jailbreak"
	AttackLeakage       AttackType = "leakage"
	AttackHallucination AttackType = "hallucination"
	AttackOfftopic      AttackType = "offtopic"
)

// Generator creates adversarial test cases.
type Generator interface {
	Name() string
	Type() AttackType
	Generate(ctx context.Context, cfg *GenerateConfig) ([]*testcase.Case, error)
}

// GenerateConfig provides context for attack generation.
type GenerateConfig struct {
	SuiteID      string
	Count        int
	SystemPrompt string
	Difficulty   string
	Extra        map[string]any
}

// Config configures a red team evaluation.
type Config struct {
	Generators []Generator
	Config     *GenerateConfig
}

// Report holds the results of a red team evaluation.
type Report struct {
	TotalAttacks int                       `json:"total_attacks"`
	Bypasses     int                       `json:"bypasses"`
	BypassRate   float64                   `json:"bypass_rate"`
	ByType       map[AttackType]TypeResult `json:"by_type"`
}

// TypeResult summarises results for a specific attack type.
type TypeResult struct {
	Total    int     `json:"total"`
	Bypasses int     `json:"bypasses"`
	Rate     float64 `json:"rate"`
}
