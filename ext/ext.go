// Package ext defines the extension system for Sentinel.
//
// Extensions are notified of lifecycle events (eval run started, case
// completed, regression detected, etc.) and can react to them — logging,
// metrics, tracing, auditing, etc.
//
// Each lifecycle hook is a separate interface so extensions opt in only
// to the events they care about.
package ext

import (
	"context"
	"time"

	"github.com/xraph/sentinel/id"
)

// ──────────────────────────────────────────────────
// Base extension interface
// ──────────────────────────────────────────────────

// Extension is the base interface all Sentinel extensions must implement.
type Extension interface {
	// Name returns a unique human-readable name for the extension.
	Name() string
}

// ──────────────────────────────────────────────────
// Eval lifecycle hooks
// ──────────────────────────────────────────────────

// EvalRunStarted is called when an evaluation run begins.
type EvalRunStarted interface {
	OnEvalRunStarted(ctx context.Context, suiteID id.SuiteID, runID id.EvalRunID, model string) error
}

// EvalRunCompleted is called when an evaluation run finishes successfully.
type EvalRunCompleted interface {
	OnEvalRunCompleted(ctx context.Context, suiteID id.SuiteID, runID id.EvalRunID, passRate float64, elapsed time.Duration) error
}

// EvalRunFailed is called when an evaluation run fails.
type EvalRunFailed interface {
	OnEvalRunFailed(ctx context.Context, suiteID id.SuiteID, runID id.EvalRunID, err error) error
}

// ──────────────────────────────────────────────────
// Case lifecycle hooks
// ──────────────────────────────────────────────────

// CaseStarted is called when evaluation of a single case begins.
type CaseStarted interface {
	OnCaseStarted(ctx context.Context, runID id.EvalRunID, caseID id.CaseID) error
}

// CaseCompleted is called when evaluation of a single case finishes.
type CaseCompleted interface {
	OnCaseCompleted(ctx context.Context, runID id.EvalRunID, caseID id.CaseID, score float64, elapsed time.Duration) error
}

// CaseFailed is called when evaluation of a single case fails.
type CaseFailed interface {
	OnCaseFailed(ctx context.Context, runID id.EvalRunID, caseID id.CaseID, err error) error
}

// ──────────────────────────────────────────────────
// Regression hooks
// ──────────────────────────────────────────────────

// RegressionDetected is called when a regression is found against a baseline.
type RegressionDetected interface {
	OnRegressionDetected(ctx context.Context, suiteID id.SuiteID, baselineID id.BaselineID, delta float64) error
}

// ──────────────────────────────────────────────────
// Baseline hooks
// ──────────────────────────────────────────────────

// BaselineSaved is called when a baseline is saved.
type BaselineSaved interface {
	OnBaselineSaved(ctx context.Context, suiteID id.SuiteID, baselineID id.BaselineID) error
}

// ──────────────────────────────────────────────────
// Red team hooks
// ──────────────────────────────────────────────────

// RedTeamStarted is called when a red team evaluation begins.
type RedTeamStarted interface {
	OnRedTeamStarted(ctx context.Context, suiteID id.SuiteID, attackCount int) error
}

// RedTeamCompleted is called when a red team evaluation finishes.
type RedTeamCompleted interface {
	OnRedTeamCompleted(ctx context.Context, suiteID id.SuiteID, bypassCount int, elapsed time.Duration) error
}

// ──────────────────────────────────────────────────
// Persona eval hooks (Cortex-aware)
// ──────────────────────────────────────────────────

// PersonaEvalStarted is called when a persona-aware evaluation begins.
type PersonaEvalStarted interface {
	OnPersonaEvalStarted(ctx context.Context, runID id.EvalRunID, personaName string) error
}

// PersonaEvalCompleted is called when a persona-aware evaluation finishes.
type PersonaEvalCompleted interface {
	OnPersonaEvalCompleted(ctx context.Context, runID id.EvalRunID, personaName string, dimensions map[string]float64) error
}

// ──────────────────────────────────────────────────
// Prompt version hooks
// ──────────────────────────────────────────────────

// PromptVersionCreated is called when a prompt version is created.
type PromptVersionCreated interface {
	OnPromptVersionCreated(ctx context.Context, suiteID id.SuiteID, pvID id.PromptVersionID, version int) error
}

// ──────────────────────────────────────────────────
// Comparison hooks
// ──────────────────────────────────────────────────

// ComparisonCompleted is called when a multi-model comparison finishes.
type ComparisonCompleted interface {
	OnComparisonCompleted(ctx context.Context, suiteID id.SuiteID, models []string, elapsed time.Duration) error
}

// ──────────────────────────────────────────────────
// Shutdown hook
// ──────────────────────────────────────────────────

// Shutdown is called during graceful shutdown.
type Shutdown interface {
	OnShutdown(ctx context.Context) error
}
