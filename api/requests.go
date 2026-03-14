package api

// ──────────────────────────────────────────────────
// Suite requests
// ──────────────────────────────────────────────────

// CreateSuiteRequest is the request body for creating a suite.
type CreateSuiteRequest struct {
	Name         string         `json:"name" description:"Suite name" required:"true"`
	Description  string         `json:"description,omitempty" description:"Suite description"`
	SystemPrompt string         `json:"system_prompt,omitempty" description:"System prompt for evaluation"`
	Model        string         `json:"model,omitempty" description:"LLM model name"`
	Temperature  float64        `json:"temperature,omitempty" description:"LLM temperature"`
	PersonaRef   string         `json:"persona_ref,omitempty" description:"Persona reference for persona-aware eval"`
	Metadata     map[string]any `json:"metadata,omitempty" description:"Suite metadata"`
}

// UpdateSuiteRequest is the request body for updating a suite.
type UpdateSuiteRequest struct {
	SuiteID string `path:"suiteId" description:"Suite ID"`
	CreateSuiteRequest
}

// GetSuiteRequest binds the path for GET /suites/:suiteId.
type GetSuiteRequest struct {
	SuiteID string `path:"suiteId" description:"Suite ID"`
}

// DeleteSuiteRequest binds the path for DELETE /suites/:suiteId.
type DeleteSuiteRequest struct {
	SuiteID string `path:"suiteId" description:"Suite ID"`
}

// ListSuitesRequest holds query params for listing suites.
type ListSuitesRequest struct {
	Limit  int `query:"limit" description:"Max results to return"`
	Offset int `query:"offset" description:"Pagination offset"`
}

// ──────────────────────────────────────────────────
// Case requests
// ──────────────────────────────────────────────────

// CreateCaseRequest is the request body for creating a test case.
type CreateCaseRequest struct {
	Name         string                `json:"name" description:"Case name" required:"true"`
	Input        string                `json:"input" description:"Case input" required:"true"`
	Expected     string                `json:"expected,omitempty" description:"Expected output"`
	ScenarioType string                `json:"scenario_type,omitempty" description:"Scenario type"`
	Scorers      []ScorerConfigRequest `json:"scorers,omitempty" description:"Scorer configurations"`
	Tags         []string              `json:"tags,omitempty" description:"Tags"`
	Context      map[string]any        `json:"context,omitempty" description:"Case context"`
	Metadata     map[string]any        `json:"metadata,omitempty" description:"Case metadata"`
}

// CreateCasesRequest holds a batch of cases to create.
type CreateCasesRequest struct {
	SuiteID string              `path:"suiteId" description:"Suite ID"`
	Cases   []CreateCaseRequest `json:"cases" description:"Cases to create" required:"true"`
}

// ScorerConfigRequest defines scorer configuration in API requests.
type ScorerConfigRequest struct {
	Name   string         `json:"name" description:"Scorer name"`
	Config map[string]any `json:"config,omitempty" description:"Scorer config"`
}

// ImportCasesRequest is the request body for importing cases.
type ImportCasesRequest struct {
	SuiteID string `path:"suiteId" description:"Suite ID"`
	Format  string `json:"format" description:"Import format: json, csv, jsonl" required:"true"`
	Data    string `json:"data" description:"Raw data to import" required:"true"`
}

// GetCaseRequest binds the path for GET /cases/:caseId.
type GetCaseRequest struct {
	CaseID string `path:"caseId" description:"Case ID"`
}

// DeleteCaseRequest binds the path for DELETE /cases/:caseId.
type DeleteCaseRequest struct {
	CaseID string `path:"caseId" description:"Case ID"`
}

// ListCasesRequest binds the path for GET /suites/:suiteId/cases.
type ListCasesRequest struct {
	SuiteID string `path:"suiteId" description:"Suite ID"`
}

// ──────────────────────────────────────────────────
// Run requests
// ──────────────────────────────────────────────────

// RunEvalRequest is the request body for triggering an evaluation run.
type RunEvalRequest struct {
	SuiteID     string   `path:"suiteId" description:"Suite ID"`
	Model       string   `json:"model,omitempty" description:"LLM model name"`
	PersonaRef  string   `json:"persona_ref,omitempty" description:"Persona reference"`
	Scorers     []string `json:"scorers,omitempty" description:"Scorer names to use"`
	Tags        []string `json:"tags,omitempty" description:"Tags for the run"`
	Concurrency int      `json:"concurrency,omitempty" description:"Max concurrent cases"`
}

// CompareModelsRequest is the request body for comparing models.
type CompareModelsRequest struct {
	SuiteID string   `path:"suiteId" description:"Suite ID"`
	Models  []string `json:"models" description:"Model names to compare" required:"true"`
	Scorers []string `json:"scorers,omitempty" description:"Scorer names to use"`
	Tags    []string `json:"tags,omitempty" description:"Tags for the run"`
}

// ListRunsRequest holds query params for listing runs.
type ListRunsRequest struct {
	SuiteID string `path:"suiteId" description:"Suite ID"`
	State   string `query:"state" description:"Filter by run state"`
	Limit   int    `query:"limit" description:"Max results to return"`
	Offset  int    `query:"offset" description:"Pagination offset"`
}

// GetRunRequest binds the path for GET /runs/:runId.
type GetRunRequest struct {
	RunID string `path:"runId" description:"Run ID"`
}

// GetRunResultsRequest binds the path for GET /runs/:runId/results.
type GetRunResultsRequest struct {
	RunID string `path:"runId" description:"Run ID"`
}

// GetRunStatsRequest binds the path for GET /runs/:runId/stats.
type GetRunStatsRequest struct {
	RunID string `path:"runId" description:"Run ID"`
}

// ──────────────────────────────────────────────────
// Baseline requests
// ──────────────────────────────────────────────────

// SaveBaselineRequest is the request body for saving a baseline.
type SaveBaselineRequest struct {
	SuiteID string `path:"suiteId" description:"Suite ID"`
	RunID   string `json:"run_id" description:"Run ID to save as baseline" required:"true"`
	Name    string `json:"name" description:"Baseline name" required:"true"`
}

// ListBaselinesRequest binds the path for GET /suites/:suiteId/baselines.
type ListBaselinesRequest struct {
	SuiteID string `path:"suiteId" description:"Suite ID"`
}

// GetBaselineRequest binds the path for GET /baselines/:baselineId.
type GetBaselineRequest struct {
	BaselineID string `path:"baselineId" description:"Baseline ID"`
}

// RunWithBaselineRequest is the request body for running eval with baseline comparison.
type RunWithBaselineRequest struct {
	SuiteID   string   `path:"suiteId" description:"Suite ID"`
	Model     string   `json:"model,omitempty" description:"LLM model name"`
	Scorers   []string `json:"scorers,omitempty" description:"Scorer names"`
	Threshold float64  `json:"threshold,omitempty" description:"Regression threshold"`
}

// ──────────────────────────────────────────────────
// Prompt version requests
// ──────────────────────────────────────────────────

// CreatePromptVersionRequest is the request body for creating a prompt version.
type CreatePromptVersionRequest struct {
	SuiteID      string `path:"suiteId" description:"Suite ID"`
	SystemPrompt string `json:"system_prompt" description:"The system prompt" required:"true"`
	Label        string `json:"label,omitempty" description:"Human-readable label"`
}

// ListPromptVersionsRequest binds the path for GET /suites/:suiteId/prompts.
type ListPromptVersionsRequest struct {
	SuiteID string `path:"suiteId" description:"Suite ID"`
}

// SetCurrentPromptRequest is the request body for setting the current prompt version.
type SetCurrentPromptRequest struct {
	SuiteID   string `path:"suiteId" description:"Suite ID"`
	VersionID string `json:"version_id" description:"Prompt version ID to set as current" required:"true"`
}

// ──────────────────────────────────────────────────
// Red team requests
// ──────────────────────────────────────────────────

// GenerateAttacksRequest is the request body for generating red team attacks.
type GenerateAttacksRequest struct {
	SuiteID     string   `path:"suiteId" description:"Suite ID"`
	AttackTypes []string `json:"attack_types,omitempty" description:"Attack types to generate"`
	Count       int      `json:"count,omitempty" description:"Number of attacks per type"`
}

// RunRedTeamRequest is the request body for running a red team evaluation.
type RunRedTeamRequest struct {
	SuiteID     string   `path:"suiteId" description:"Suite ID"`
	AttackTypes []string `json:"attack_types,omitempty" description:"Attack types to use"`
	Count       int      `json:"count,omitempty" description:"Number of attacks per type"`
	Scorers     []string `json:"scorers,omitempty" description:"Scorer names"`
}

// ──────────────────────────────────────────────────
// Scenario requests
// ──────────────────────────────────────────────────

// GenerateScenariosRequest is the request body for generating test scenarios.
type GenerateScenariosRequest struct {
	SuiteID    string `path:"suiteId" description:"Suite ID"`
	Type       string `json:"type" description:"Scenario type to generate" required:"true"`
	Count      int    `json:"count,omitempty" description:"Number of scenarios"`
	Difficulty string `json:"difficulty,omitempty" description:"Difficulty level"`
}

// ListScenariosRequest binds the path for GET /suites/:suiteId/scenarios.
type ListScenariosRequest struct {
	SuiteID string `path:"suiteId" description:"Suite ID"`
}

// ──────────────────────────────────────────────────
// Report requests
// ──────────────────────────────────────────────────

// GetReportRequest binds the path for GET /runs/:runId/report.
type GetReportRequest struct {
	RunID string `path:"runId" description:"Run ID"`
}

// ExportReportRequest is the request body for exporting a report.
type ExportReportRequest struct {
	RunID  string `path:"runId" description:"Run ID"`
	Format string `json:"format,omitempty" description:"Report format: terminal, json, html, ci"`
}

// ──────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────

func defaultLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}
