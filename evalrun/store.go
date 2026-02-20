package evalrun

import (
	"context"

	"github.com/xraph/sentinel/id"
)

// Store defines persistence operations for evaluation runs and results.
type Store interface {
	CreateRun(ctx context.Context, run *Run) error
	GetRun(ctx context.Context, runID id.EvalRunID) (*Run, error)
	UpdateRun(ctx context.Context, run *Run) error
	ListRuns(ctx context.Context, filter *ListFilter) ([]*Run, error)
	ListRunsBySuite(ctx context.Context, suiteID id.SuiteID) ([]*Run, error)

	CreateResult(ctx context.Context, result *Result) error
	CreateResultBatch(ctx context.Context, results []*Result) error
	ListResults(ctx context.Context, runID id.EvalRunID) ([]*Result, error)
	GetResultStats(ctx context.Context, runID id.EvalRunID) (*ResultStats, error)
}
