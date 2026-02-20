package plugin

import (
	"context"
	"log/slog"
	"time"

	"github.com/xraph/sentinel/id"
)

// Named entry types pair a hook implementation with the plugin name
// captured at registration time.
type evalRunStartedEntry struct {
	name string
	hook EvalRunStarted
}

type evalRunCompletedEntry struct {
	name string
	hook EvalRunCompleted
}

type evalRunFailedEntry struct {
	name string
	hook EvalRunFailed
}

type caseStartedEntry struct {
	name string
	hook CaseStarted
}

type caseCompletedEntry struct {
	name string
	hook CaseCompleted
}

type caseFailedEntry struct {
	name string
	hook CaseFailed
}

type regressionDetectedEntry struct {
	name string
	hook RegressionDetected
}

type baselineSavedEntry struct {
	name string
	hook BaselineSaved
}

type redTeamStartedEntry struct {
	name string
	hook RedTeamStarted
}

type redTeamCompletedEntry struct {
	name string
	hook RedTeamCompleted
}

type personaEvalStartedEntry struct {
	name string
	hook PersonaEvalStarted
}

type personaEvalCompletedEntry struct {
	name string
	hook PersonaEvalCompleted
}

type promptVersionCreatedEntry struct {
	name string
	hook PromptVersionCreated
}

type comparisonCompletedEntry struct {
	name string
	hook ComparisonCompleted
}

type shutdownEntry struct {
	name string
	hook Shutdown
}

// Registry holds registered plugins and dispatches lifecycle events
// to them. It type-caches plugins at registration time so emit calls
// iterate only over plugins that implement the relevant hook.
type Registry struct {
	extensions []Extension
	logger     *slog.Logger

	// Type-cached slices for each lifecycle hook.
	evalRunStarted       []evalRunStartedEntry
	evalRunCompleted     []evalRunCompletedEntry
	evalRunFailed        []evalRunFailedEntry
	caseStarted          []caseStartedEntry
	caseCompleted        []caseCompletedEntry
	caseFailed           []caseFailedEntry
	regressionDetected   []regressionDetectedEntry
	baselineSaved        []baselineSavedEntry
	redTeamStarted       []redTeamStartedEntry
	redTeamCompleted     []redTeamCompletedEntry
	personaEvalStarted   []personaEvalStartedEntry
	personaEvalCompleted []personaEvalCompletedEntry
	promptVersionCreated []promptVersionCreatedEntry
	comparisonCompleted  []comparisonCompletedEntry
	shutdown             []shutdownEntry
}

// NewRegistry creates a plugin registry with the given logger.
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{logger: logger}
}

// Register adds a plugin and type-asserts it into all applicable
// hook caches. Plugins are notified in registration order.
func (r *Registry) Register(e Extension) {
	r.extensions = append(r.extensions, e)
	name := e.Name()

	if h, ok := e.(EvalRunStarted); ok {
		r.evalRunStarted = append(r.evalRunStarted, evalRunStartedEntry{name, h})
	}
	if h, ok := e.(EvalRunCompleted); ok {
		r.evalRunCompleted = append(r.evalRunCompleted, evalRunCompletedEntry{name, h})
	}
	if h, ok := e.(EvalRunFailed); ok {
		r.evalRunFailed = append(r.evalRunFailed, evalRunFailedEntry{name, h})
	}
	if h, ok := e.(CaseStarted); ok {
		r.caseStarted = append(r.caseStarted, caseStartedEntry{name, h})
	}
	if h, ok := e.(CaseCompleted); ok {
		r.caseCompleted = append(r.caseCompleted, caseCompletedEntry{name, h})
	}
	if h, ok := e.(CaseFailed); ok {
		r.caseFailed = append(r.caseFailed, caseFailedEntry{name, h})
	}
	if h, ok := e.(RegressionDetected); ok {
		r.regressionDetected = append(r.regressionDetected, regressionDetectedEntry{name, h})
	}
	if h, ok := e.(BaselineSaved); ok {
		r.baselineSaved = append(r.baselineSaved, baselineSavedEntry{name, h})
	}
	if h, ok := e.(RedTeamStarted); ok {
		r.redTeamStarted = append(r.redTeamStarted, redTeamStartedEntry{name, h})
	}
	if h, ok := e.(RedTeamCompleted); ok {
		r.redTeamCompleted = append(r.redTeamCompleted, redTeamCompletedEntry{name, h})
	}
	if h, ok := e.(PersonaEvalStarted); ok {
		r.personaEvalStarted = append(r.personaEvalStarted, personaEvalStartedEntry{name, h})
	}
	if h, ok := e.(PersonaEvalCompleted); ok {
		r.personaEvalCompleted = append(r.personaEvalCompleted, personaEvalCompletedEntry{name, h})
	}
	if h, ok := e.(PromptVersionCreated); ok {
		r.promptVersionCreated = append(r.promptVersionCreated, promptVersionCreatedEntry{name, h})
	}
	if h, ok := e.(ComparisonCompleted); ok {
		r.comparisonCompleted = append(r.comparisonCompleted, comparisonCompletedEntry{name, h})
	}
	if h, ok := e.(Shutdown); ok {
		r.shutdown = append(r.shutdown, shutdownEntry{name, h})
	}
}

// Extensions returns all registered plugins.
func (r *Registry) Extensions() []Extension { return r.extensions }

// ──────────────────────────────────────────────────
// Eval lifecycle emitters
// ──────────────────────────────────────────────────

// EmitEvalRunStarted notifies all plugins that implement EvalRunStarted.
func (r *Registry) EmitEvalRunStarted(ctx context.Context, suiteID id.SuiteID, runID id.EvalRunID, model string) {
	for _, e := range r.evalRunStarted {
		if err := e.hook.OnEvalRunStarted(ctx, suiteID, runID, model); err != nil {
			r.logHookError("OnEvalRunStarted", e.name, err)
		}
	}
}

// EmitEvalRunCompleted notifies all plugins that implement EvalRunCompleted.
func (r *Registry) EmitEvalRunCompleted(ctx context.Context, suiteID id.SuiteID, runID id.EvalRunID, passRate float64, elapsed time.Duration) {
	for _, e := range r.evalRunCompleted {
		if err := e.hook.OnEvalRunCompleted(ctx, suiteID, runID, passRate, elapsed); err != nil {
			r.logHookError("OnEvalRunCompleted", e.name, err)
		}
	}
}

// EmitEvalRunFailed notifies all plugins that implement EvalRunFailed.
func (r *Registry) EmitEvalRunFailed(ctx context.Context, suiteID id.SuiteID, runID id.EvalRunID, runErr error) {
	for _, e := range r.evalRunFailed {
		if err := e.hook.OnEvalRunFailed(ctx, suiteID, runID, runErr); err != nil {
			r.logHookError("OnEvalRunFailed", e.name, err)
		}
	}
}

// ──────────────────────────────────────────────────
// Case lifecycle emitters
// ──────────────────────────────────────────────────

// EmitCaseStarted notifies all plugins that implement CaseStarted.
func (r *Registry) EmitCaseStarted(ctx context.Context, runID id.EvalRunID, caseID id.CaseID) {
	for _, e := range r.caseStarted {
		if err := e.hook.OnCaseStarted(ctx, runID, caseID); err != nil {
			r.logHookError("OnCaseStarted", e.name, err)
		}
	}
}

// EmitCaseCompleted notifies all plugins that implement CaseCompleted.
func (r *Registry) EmitCaseCompleted(ctx context.Context, runID id.EvalRunID, caseID id.CaseID, score float64, elapsed time.Duration) {
	for _, e := range r.caseCompleted {
		if err := e.hook.OnCaseCompleted(ctx, runID, caseID, score, elapsed); err != nil {
			r.logHookError("OnCaseCompleted", e.name, err)
		}
	}
}

// EmitCaseFailed notifies all plugins that implement CaseFailed.
func (r *Registry) EmitCaseFailed(ctx context.Context, runID id.EvalRunID, caseID id.CaseID, caseErr error) {
	for _, e := range r.caseFailed {
		if err := e.hook.OnCaseFailed(ctx, runID, caseID, caseErr); err != nil {
			r.logHookError("OnCaseFailed", e.name, err)
		}
	}
}

// ──────────────────────────────────────────────────
// Regression emitter
// ──────────────────────────────────────────────────

// EmitRegressionDetected notifies all plugins that implement RegressionDetected.
func (r *Registry) EmitRegressionDetected(ctx context.Context, suiteID id.SuiteID, baselineID id.BaselineID, delta float64) {
	for _, e := range r.regressionDetected {
		if err := e.hook.OnRegressionDetected(ctx, suiteID, baselineID, delta); err != nil {
			r.logHookError("OnRegressionDetected", e.name, err)
		}
	}
}

// ──────────────────────────────────────────────────
// Baseline emitter
// ──────────────────────────────────────────────────

// EmitBaselineSaved notifies all plugins that implement BaselineSaved.
func (r *Registry) EmitBaselineSaved(ctx context.Context, suiteID id.SuiteID, baselineID id.BaselineID) {
	for _, e := range r.baselineSaved {
		if err := e.hook.OnBaselineSaved(ctx, suiteID, baselineID); err != nil {
			r.logHookError("OnBaselineSaved", e.name, err)
		}
	}
}

// ──────────────────────────────────────────────────
// Red team emitters
// ──────────────────────────────────────────────────

// EmitRedTeamStarted notifies all plugins that implement RedTeamStarted.
func (r *Registry) EmitRedTeamStarted(ctx context.Context, suiteID id.SuiteID, attackCount int) {
	for _, e := range r.redTeamStarted {
		if err := e.hook.OnRedTeamStarted(ctx, suiteID, attackCount); err != nil {
			r.logHookError("OnRedTeamStarted", e.name, err)
		}
	}
}

// EmitRedTeamCompleted notifies all plugins that implement RedTeamCompleted.
func (r *Registry) EmitRedTeamCompleted(ctx context.Context, suiteID id.SuiteID, bypassCount int, elapsed time.Duration) {
	for _, e := range r.redTeamCompleted {
		if err := e.hook.OnRedTeamCompleted(ctx, suiteID, bypassCount, elapsed); err != nil {
			r.logHookError("OnRedTeamCompleted", e.name, err)
		}
	}
}

// ──────────────────────────────────────────────────
// Persona eval emitters
// ──────────────────────────────────────────────────

// EmitPersonaEvalStarted notifies all plugins that implement PersonaEvalStarted.
func (r *Registry) EmitPersonaEvalStarted(ctx context.Context, runID id.EvalRunID, personaName string) {
	for _, e := range r.personaEvalStarted {
		if err := e.hook.OnPersonaEvalStarted(ctx, runID, personaName); err != nil {
			r.logHookError("OnPersonaEvalStarted", e.name, err)
		}
	}
}

// EmitPersonaEvalCompleted notifies all plugins that implement PersonaEvalCompleted.
func (r *Registry) EmitPersonaEvalCompleted(ctx context.Context, runID id.EvalRunID, personaName string, dimensions map[string]float64) {
	for _, e := range r.personaEvalCompleted {
		if err := e.hook.OnPersonaEvalCompleted(ctx, runID, personaName, dimensions); err != nil {
			r.logHookError("OnPersonaEvalCompleted", e.name, err)
		}
	}
}

// ──────────────────────────────────────────────────
// Prompt version emitter
// ──────────────────────────────────────────────────

// EmitPromptVersionCreated notifies all plugins that implement PromptVersionCreated.
func (r *Registry) EmitPromptVersionCreated(ctx context.Context, suiteID id.SuiteID, pvID id.PromptVersionID, version int) {
	for _, e := range r.promptVersionCreated {
		if err := e.hook.OnPromptVersionCreated(ctx, suiteID, pvID, version); err != nil {
			r.logHookError("OnPromptVersionCreated", e.name, err)
		}
	}
}

// ──────────────────────────────────────────────────
// Comparison emitter
// ──────────────────────────────────────────────────

// EmitComparisonCompleted notifies all plugins that implement ComparisonCompleted.
func (r *Registry) EmitComparisonCompleted(ctx context.Context, suiteID id.SuiteID, models []string, elapsed time.Duration) {
	for _, e := range r.comparisonCompleted {
		if err := e.hook.OnComparisonCompleted(ctx, suiteID, models, elapsed); err != nil {
			r.logHookError("OnComparisonCompleted", e.name, err)
		}
	}
}

// ──────────────────────────────────────────────────
// Shutdown emitter
// ──────────────────────────────────────────────────

// EmitShutdown notifies all plugins that implement Shutdown.
func (r *Registry) EmitShutdown(ctx context.Context) {
	for _, e := range r.shutdown {
		if err := e.hook.OnShutdown(ctx); err != nil {
			r.logHookError("OnShutdown", e.name, err)
		}
	}
}

// logHookError logs a warning when a lifecycle hook returns an error.
// Errors from hooks are never propagated — they must not block the eval pipeline.
func (r *Registry) logHookError(hook, extName string, err error) {
	r.logger.Warn("plugin hook error",
		slog.String("hook", hook),
		slog.String("plugin", extName),
		slog.String("error", err.Error()),
	)
}
