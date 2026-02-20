// Package promptversion tracks system prompt iterations for A/B testing
// and regression tracking across prompt changes.
package promptversion

import (
	"time"

	"github.com/xraph/sentinel/id"
)

// PromptVersion represents a versioned system prompt for a suite.
type PromptVersion struct {
	ID           id.PromptVersionID `json:"id" bun:"id,pk"`
	SuiteID      id.SuiteID         `json:"suite_id" bun:"suite_id,notnull"`
	Version      int                `json:"version" bun:"version,notnull"`
	SystemPrompt string             `json:"system_prompt" bun:"system_prompt,notnull"`
	Changelog    string             `json:"changelog,omitempty" bun:"changelog"`
	IsCurrent    bool               `json:"is_current" bun:"is_current,notnull,default:false"`
	RunID        string             `json:"run_id,omitempty" bun:"run_id"`
	PassRate     *float64           `json:"pass_rate,omitempty" bun:"pass_rate"`
	AvgScore     *float64           `json:"avg_score,omitempty" bun:"avg_score"`
	CreatedAt    time.Time          `json:"created_at" bun:"created_at,notnull,default:current_timestamp"`
}
