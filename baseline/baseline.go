// Package baseline defines evaluation baselines for regression detection.
// A baseline captures the results of a known-good evaluation run against
// which future runs can be compared.
package baseline

import (
	"time"

	"github.com/xraph/sentinel/id"
)

// Baseline captures the results of a known-good evaluation run.
type Baseline struct {
	ID              id.BaselineID      `json:"id" bun:"id,pk"`
	SuiteID         id.SuiteID         `json:"suite_id" bun:"suite_id,notnull"`
	RunID           id.EvalRunID       `json:"run_id" bun:"run_id,notnull"`
	Name            string             `json:"name" bun:"name,notnull"`
	Results         []BaselineResult   `json:"results" bun:"results,type:jsonb"`
	PassRate        float64            `json:"pass_rate" bun:"pass_rate,notnull,default:0"`
	AvgScore        float64            `json:"avg_score" bun:"avg_score,notnull,default:0"`
	DimensionScores map[string]float64 `json:"dimension_scores,omitempty" bun:"dimension_scores,type:jsonb,default:'{}'"`
	IsCurrent       bool               `json:"is_current" bun:"is_current,notnull,default:false"`
	CreatedAt       time.Time          `json:"created_at" bun:"created_at,notnull,default:current_timestamp"`
}

// BaselineResult stores per-case baseline data for comparison.
type BaselineResult struct {
	CaseID          id.CaseID          `json:"case_id"`
	CaseName        string             `json:"case_name"`
	Score           float64            `json:"score"`
	Status          string             `json:"status"`
	DimensionScores map[string]float64 `json:"dimension_scores,omitempty"`
}
