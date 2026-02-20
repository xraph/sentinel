package dataset

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xraph/sentinel/scorer"
)

// LLMGenerator creates test cases using an LLM.
type LLMGenerator struct {
	Client scorer.LLMClient
	Model  string
}

// NewLLMGenerator creates an LLM-based dataset generator.
func NewLLMGenerator(client scorer.LLMClient, model string) *LLMGenerator {
	return &LLMGenerator{Client: client, Model: model}
}

// GenerateConfig configures LLM-based test case generation.
type GenerateConfig struct {
	Description string
	Count       int
	Domain      string
	Difficulty  string
}

// Generate creates test cases using the LLM.
func (g *LLMGenerator) Generate(ctx context.Context, cfg *GenerateConfig) ([]CaseData, error) {
	count := cfg.Count
	if count <= 0 {
		count = 5
	}

	systemPrompt := `You are a test case generator for AI evaluation. Generate test cases in JSON array format.
Each test case should have: name, input, expected (optional), tags (array).
Output ONLY the JSON array, no other text.`

	userPrompt := fmt.Sprintf(`Generate %d test cases for the following:
Domain: %s
Description: %s
Difficulty: %s

Output a JSON array of objects with fields: name, input, expected, tags.`,
		count, cfg.Domain, cfg.Description, cfg.Difficulty)

	response, err := g.Client.Complete(ctx, g.Model, systemPrompt, userPrompt, 0.7)
	if err != nil {
		return nil, fmt.Errorf("dataset: generate: %w", err)
	}

	var cases []CaseData
	if err := json.Unmarshal([]byte(response), &cases); err != nil {
		return nil, fmt.Errorf("dataset: parse generated cases: %w", err)
	}

	return cases, nil
}
