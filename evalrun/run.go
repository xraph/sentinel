// Package evalrun defines evaluation runs and their results — the execution
// records of running a suite against a target.
package evalrun

import (
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
)

// Run represents a single execution of an evaluation suite.
type Run struct {
	sentinel.Entity
	ID              id.EvalRunID       `json:"id" bun:"id,pk"`
	SuiteID         id.SuiteID         `json:"suite_id" bun:"suite_id,notnull"`
	Model           string             `json:"model" bun:"model,notnull"`
	SystemPrompt    string             `json:"system_prompt" bun:"system_prompt"`
	Temperature     float64            `json:"temperature" bun:"temperature,notnull,default:0"`
	TotalCases      int                `json:"total_cases" bun:"total_cases,notnull,default:0"`
	Passed          int                `json:"passed" bun:"passed,notnull,default:0"`
	Failed          int                `json:"failed" bun:"failed,notnull,default:0"`
	PassRate        float64            `json:"pass_rate" bun:"pass_rate,notnull,default:0"`
	AvgScore        float64            `json:"avg_score" bun:"avg_score,notnull,default:0"`
	AvgLatencyMs    int                `json:"avg_latency_ms" bun:"avg_latency_ms,notnull,default:0"`
	TotalTokens     int                `json:"total_tokens" bun:"total_tokens,notnull,default:0"`
	TotalCost       float64            `json:"total_cost" bun:"total_cost,notnull,default:0"`
	AppID           string             `json:"app_id" bun:"app_id,notnull"`
	TargetTenantID  string             `json:"target_tenant_id,omitempty" bun:"target_tenant_id"`
	PersonaRef      string             `json:"persona_ref,omitempty" bun:"persona_ref"`
	Config          map[string]any     `json:"config" bun:"config,type:jsonb,default:'{}'"`
	State           RunState           `json:"state" bun:"state,notnull,default:'running'"`
	Error           string             `json:"error,omitempty" bun:"error"`
	CompletedAt     *time.Time         `json:"completed_at,omitempty" bun:"completed_at"`
	DimensionScores map[string]float64 `json:"dimension_scores,omitempty" bun:"dimension_scores,type:jsonb,default:'{}'"`
}

// RunState is the lifecycle state of an evaluation run.
type RunState string

const (
	StateRunning   RunState = "running"
	StateCompleted RunState = "completed"
	StateFailed    RunState = "failed"
	StateCancelled RunState = "cancelled"
)

// ListFilter constrains run listing.
type ListFilter struct {
	SuiteID id.SuiteID
	AppID   string
	State   RunState
	Limit   int
	Offset  int
}
