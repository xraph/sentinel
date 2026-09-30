package evalrun

import (
	"context"
	"time"

	"github.com/xraph/sentinel/id"
)

// Store defines persistence operations for evaluation runs and results.
type Store interface {
	CreateRun(ctx context.Context, run *Run) error
	GetRun(ctx context.Context, runID id.EvalRunID) (*Run, error)
	UpdateRun(ctx context.Context, run *Run) error

	// CancelRun marks a running run cancelled at the given time. It reports
	// false, with no error, when the run exists and is no longer running.
	// The condition is part of the write, so it is safe against a runner
	// finishing at the same moment on another replica.
	CancelRun(ctx context.Context, runID id.EvalRunID, at time.Time) (bool, error)

	// FinalizeRun writes a finished run's counters, dimension scores and
	// completion time, and its error when one is given. It writes the state
	// only if the stored state is still running, and returns the state the
	// store holds afterwards, so a cancel is never overwritten.
	FinalizeRun(ctx context.Context, runID id.EvalRunID, f *Finalization) (RunState, error)

	ListRuns(ctx context.Context, filter *ListFilter) ([]*Run, error)
	ListRunsBySuite(ctx context.Context, suiteID id.SuiteID) ([]*Run, error)

	CreateResult(ctx context.Context, result *Result) error
	CreateResultBatch(ctx context.Context, results []*Result) error
	ListResults(ctx context.Context, runID id.EvalRunID) ([]*Result, error)
	GetResultStats(ctx context.Context, runID id.EvalRunID) (*ResultStats, error)
}
