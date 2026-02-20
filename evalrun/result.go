package evalrun

import (
	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
)

// Result is the outcome of evaluating a single test case within a run.
type Result struct {
	sentinel.Entity
	ID              id.EvalResultID    `json:"id" bun:"id,pk"`
	RunID           id.EvalRunID       `json:"run_id" bun:"run_id,notnull"`
	CaseID          id.CaseID          `json:"case_id" bun:"case_id,notnull"`
	CaseName        string             `json:"case_name" bun:"case_name,notnull"`
	Status          ResultStatus       `json:"status" bun:"status,notnull"`
	Score           float64            `json:"score" bun:"score,notnull,default:0"`
	Output          string             `json:"output,omitempty" bun:"output"`
	LatencyMs       int                `json:"latency_ms" bun:"latency_ms,notnull,default:0"`
	TokensUsed      int                `json:"tokens_used" bun:"tokens_used,notnull,default:0"`
	Cost            float64            `json:"cost" bun:"cost,notnull,default:0"`
	ScorerResults   []ScorerResult     `json:"scorer_results" bun:"scorer_results,type:jsonb,default:'[]'"`
	Error           string             `json:"error,omitempty" bun:"error"`
	DimensionScores map[string]float64 `json:"dimension_scores,omitempty" bun:"dimension_scores,type:jsonb,default:'{}'"`
	RunTrace        *RunTrace          `json:"run_trace,omitempty" bun:"run_trace,type:jsonb"`
}

// ResultStatus is the outcome status of a single case evaluation.
type ResultStatus string

const (
	StatusPass  ResultStatus = "pass"
	StatusFail  ResultStatus = "fail"
	StatusError ResultStatus = "error"
)

// ScorerResult captures the output of a single scorer for a case.
type ScorerResult struct {
	ScorerName string         `json:"scorer_name"`
	Score      float64        `json:"score"`
	Passed     bool           `json:"passed"`
	Reason     string         `json:"reason"`
	Dimension  string         `json:"dimension,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

// RunTrace captures agent execution data for persona-aware evaluation.
type RunTrace struct {
	Steps     []StepTrace `json:"steps,omitempty"`
	ToolCalls []ToolTrace `json:"tool_calls,omitempty"`
}

// StepTrace records a single reasoning step in an agent run.
type StepTrace struct {
	Index      int    `json:"index"`
	Type       string `json:"type"`
	Output     string `json:"output"`
	TokensUsed int    `json:"tokens_used"`
}

// ToolTrace records a single tool invocation in an agent run.
type ToolTrace struct {
	ToolName  string `json:"tool_name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
	Error     string `json:"error,omitempty"`
}

// ResultStats holds aggregate statistics for a run's results.
type ResultStats struct {
	TotalCases      int                `json:"total_cases"`
	Passed          int                `json:"passed"`
	Failed          int                `json:"failed"`
	Errored         int                `json:"errored"`
	PassRate        float64            `json:"pass_rate"`
	AvgScore        float64            `json:"avg_score"`
	AvgLatencyMs    int                `json:"avg_latency_ms"`
	TotalTokens     int                `json:"total_tokens"`
	TotalCost       float64            `json:"total_cost"`
	DimensionScores map[string]float64 `json:"dimension_scores,omitempty"`
}
