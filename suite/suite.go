// Package suite defines evaluation suites — named collections of test cases
// targeting a specific system prompt, model, and optional persona.
package suite

import (
	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
)

// Suite is an evaluation suite that groups test cases together.
type Suite struct {
	sentinel.Entity
	ID           id.SuiteID     `json:"id" bun:"id,pk"`
	Name         string         `json:"name" bun:"name,notnull"`
	Description  string         `json:"description" bun:"description"`
	AppID        string         `json:"app_id" bun:"app_id,notnull"`
	SystemPrompt string         `json:"system_prompt" bun:"system_prompt"`
	Model        string         `json:"model" bun:"model,notnull"`
	Temperature  float64        `json:"temperature" bun:"temperature,notnull,default:0"`
	PersonaRef   string         `json:"persona_ref,omitempty" bun:"persona_ref"`
	Metadata     map[string]any `json:"metadata" bun:"metadata,type:jsonb,default:'{}'"`
}

// ListFilter constrains suite listing.
type ListFilter struct {
	AppID  string
	Limit  int
	Offset int
}
