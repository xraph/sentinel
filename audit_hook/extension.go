package audithook

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/xraph/sentinel/plugin"
	"github.com/xraph/sentinel/id"
)

// Compile-time interface checks.
var (
	_ plugin.Extension           = (*Extension)(nil)
	_ plugin.EvalRunStarted      = (*Extension)(nil)
	_ plugin.EvalRunCompleted    = (*Extension)(nil)
	_ plugin.EvalRunFailed       = (*Extension)(nil)
	_ plugin.CaseCompleted       = (*Extension)(nil)
	_ plugin.CaseFailed          = (*Extension)(nil)
	_ plugin.RegressionDetected  = (*Extension)(nil)
	_ plugin.BaselineSaved       = (*Extension)(nil)
	_ plugin.RedTeamStarted      = (*Extension)(nil)
	_ plugin.RedTeamCompleted    = (*Extension)(nil)
	_ plugin.PersonaEvalStarted  = (*Extension)(nil)
	_ plugin.PersonaEvalCompleted = (*Extension)(nil)
	_ plugin.PromptVersionCreated = (*Extension)(nil)
	_ plugin.ComparisonCompleted  = (*Extension)(nil)
)

// Recorder is the interface that audit backends must implement.
// Matches chronicle.Emitter but defined locally to avoid the import.
type Recorder interface {
	Record(ctx context.Context, event *AuditEvent) error
}

// AuditEvent mirrors chronicle/audit.Event without a module dependency.
type AuditEvent struct {
	Action     string         `json:"action"`
	Resource   string         `json:"resource"`
	Category   string         `json:"category"`
	ResourceID string         `json:"resource_id,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Outcome    string         `json:"outcome"`
	Severity   string         `json:"severity"`
	Reason     string         `json:"reason,omitempty"`
}

// RecorderFunc is an adapter to use a plain function as a Recorder.
type RecorderFunc func(ctx context.Context, event *AuditEvent) error

func (f RecorderFunc) Record(ctx context.Context, event *AuditEvent) error {
	return f(ctx, event)
}

// Extension bridges Sentinel lifecycle events to an audit trail backend.
type Extension struct {
	recorder Recorder
	enabled  map[string]bool
	logger   *slog.Logger
}

// New creates an Extension that emits audit events through the provided Recorder.
func New(r Recorder, opts ...Option) *Extension {
	e := &Extension{
		recorder: r,
		logger:   slog.Default(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Name implements plugin.Extension.
func (e *Extension) Name() string { return "audit-hook" }

func (e *Extension) OnEvalRunStarted(ctx context.Context, suiteID id.SuiteID, runID id.EvalRunID, model string) error {
	return e.record(ctx, ActionEvalRunStarted, SeverityInfo, OutcomeSuccess,
		ResourceRun, runID.String(), CategoryEval, nil,
		"suite_id", suiteID.String(),
		"model", model,
	)
}

func (e *Extension) OnEvalRunCompleted(ctx context.Context, suiteID id.SuiteID, runID id.EvalRunID, passRate float64, elapsed time.Duration) error {
	return e.record(ctx, ActionEvalRunCompleted, SeverityInfo, OutcomeSuccess,
		ResourceRun, runID.String(), CategoryEval, nil,
		"suite_id", suiteID.String(),
		"pass_rate", passRate,
		"elapsed_ms", elapsed.Milliseconds(),
	)
}

func (e *Extension) OnEvalRunFailed(ctx context.Context, suiteID id.SuiteID, runID id.EvalRunID, runErr error) error {
	return e.record(ctx, ActionEvalRunFailed, SeverityCritical, OutcomeFailure,
		ResourceRun, runID.String(), CategoryEval, runErr,
		"suite_id", suiteID.String(),
	)
}

func (e *Extension) OnCaseCompleted(ctx context.Context, runID id.EvalRunID, caseID id.CaseID, score float64, elapsed time.Duration) error {
	return e.record(ctx, ActionCaseCompleted, SeverityInfo, OutcomeSuccess,
		ResourceCase, caseID.String(), CategoryCase, nil,
		"run_id", runID.String(),
		"score", score,
		"elapsed_ms", elapsed.Milliseconds(),
	)
}

func (e *Extension) OnCaseFailed(ctx context.Context, runID id.EvalRunID, caseID id.CaseID, caseErr error) error {
	return e.record(ctx, ActionCaseFailed, SeverityCritical, OutcomeFailure,
		ResourceCase, caseID.String(), CategoryCase, caseErr,
		"run_id", runID.String(),
	)
}

func (e *Extension) OnRegressionDetected(ctx context.Context, suiteID id.SuiteID, baselineID id.BaselineID, delta float64) error {
	return e.record(ctx, ActionRegressionDetected, SeverityWarning, OutcomeFailure,
		ResourceBaseline, baselineID.String(), CategoryRegression, nil,
		"suite_id", suiteID.String(),
		"delta", delta,
	)
}

func (e *Extension) OnBaselineSaved(ctx context.Context, suiteID id.SuiteID, baselineID id.BaselineID) error {
	return e.record(ctx, ActionBaselineSaved, SeverityInfo, OutcomeSuccess,
		ResourceBaseline, baselineID.String(), CategoryBaseline, nil,
		"suite_id", suiteID.String(),
	)
}

func (e *Extension) OnRedTeamStarted(ctx context.Context, suiteID id.SuiteID, attackCount int) error {
	return e.record(ctx, ActionRedTeamStarted, SeverityInfo, OutcomeSuccess,
		ResourceRedTeam, suiteID.String(), CategoryRedTeam, nil,
		"attack_count", attackCount,
	)
}

func (e *Extension) OnRedTeamCompleted(ctx context.Context, suiteID id.SuiteID, bypassCount int, elapsed time.Duration) error {
	return e.record(ctx, ActionRedTeamCompleted, SeverityInfo, OutcomeSuccess,
		ResourceRedTeam, suiteID.String(), CategoryRedTeam, nil,
		"bypass_count", bypassCount,
		"elapsed_ms", elapsed.Milliseconds(),
	)
}

func (e *Extension) OnPersonaEvalStarted(ctx context.Context, runID id.EvalRunID, personaName string) error {
	return e.record(ctx, ActionPersonaEvalStarted, SeverityInfo, OutcomeSuccess,
		ResourcePersona, runID.String(), CategoryPersona, nil,
		"persona_name", personaName,
	)
}

func (e *Extension) OnPersonaEvalCompleted(ctx context.Context, runID id.EvalRunID, personaName string, dimensions map[string]float64) error {
	return e.record(ctx, ActionPersonaEvalCompleted, SeverityInfo, OutcomeSuccess,
		ResourcePersona, runID.String(), CategoryPersona, nil,
		"persona_name", personaName,
		"dimensions", dimensions,
	)
}

func (e *Extension) OnPromptVersionCreated(ctx context.Context, suiteID id.SuiteID, pvID id.PromptVersionID, version int) error {
	return e.record(ctx, ActionPromptVersionCreated, SeverityInfo, OutcomeSuccess,
		ResourcePromptVersion, pvID.String(), CategoryPrompt, nil,
		"suite_id", suiteID.String(),
		"version", version,
	)
}

func (e *Extension) OnComparisonCompleted(ctx context.Context, suiteID id.SuiteID, models []string, elapsed time.Duration) error {
	return e.record(ctx, ActionComparisonCompleted, SeverityInfo, OutcomeSuccess,
		ResourceComparison, suiteID.String(), CategoryComparison, nil,
		"models", models,
		"elapsed_ms", elapsed.Milliseconds(),
	)
}

func (e *Extension) record(
	ctx context.Context,
	action, severity, outcome string,
	resource, resourceID, category string,
	err error,
	kvPairs ...any,
) error {
	if e.enabled != nil && !e.enabled[action] {
		return nil
	}

	meta := make(map[string]any, len(kvPairs)/2+1)
	for i := 0; i+1 < len(kvPairs); i += 2 {
		key, ok := kvPairs[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", kvPairs[i])
		}
		meta[key] = kvPairs[i+1]
	}

	var reason string
	if err != nil {
		reason = err.Error()
		meta["error"] = err.Error()
	}

	evt := &AuditEvent{
		Action:     action,
		Resource:   resource,
		Category:   category,
		ResourceID: resourceID,
		Metadata:   meta,
		Outcome:    outcome,
		Severity:   severity,
		Reason:     reason,
	}

	if recErr := e.recorder.Record(ctx, evt); recErr != nil {
		e.logger.Warn("audit_hook: failed to record audit event",
			"action", action,
			"resource_id", resourceID,
			"error", recErr,
		)
	}
	return nil
}
