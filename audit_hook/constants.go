package audithook

// Severity constants (mirror chronicle/audit).
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Outcome constants (mirror chronicle/audit).
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
)

// Action constants.
const (
	ActionEvalRunStarted       = "sentinel.eval.run.started"
	ActionEvalRunCompleted     = "sentinel.eval.run.completed"
	ActionEvalRunFailed        = "sentinel.eval.run.failed"
	ActionCaseCompleted        = "sentinel.case.completed"
	ActionCaseFailed           = "sentinel.case.failed"
	ActionRegressionDetected   = "sentinel.regression.detected"
	ActionBaselineSaved        = "sentinel.baseline.saved"
	ActionRedTeamStarted       = "sentinel.redteam.started"
	ActionRedTeamCompleted     = "sentinel.redteam.completed"
	ActionPersonaEvalStarted   = "sentinel.persona.eval.started"
	ActionPersonaEvalCompleted = "sentinel.persona.eval.completed"
	ActionPromptVersionCreated = "sentinel.prompt.version.created"
	ActionComparisonCompleted  = "sentinel.comparison.completed"
)

// Resource constants.
const (
	ResourceRun           = "eval_run"
	ResourceCase          = "case"
	ResourceBaseline      = "baseline"
	ResourceRedTeam       = "redteam"
	ResourcePersona       = "persona"
	ResourcePromptVersion = "prompt_version"
	ResourceComparison    = "comparison"
)

// Category constants.
const (
	CategoryEval       = "eval"
	CategoryCase       = "case"
	CategoryRegression = "regression"
	CategoryBaseline   = "baseline"
	CategoryRedTeam    = "redteam"
	CategoryPersona    = "persona"
	CategoryPrompt     = "prompt"
	CategoryComparison = "comparison"
)
