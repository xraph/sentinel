package mongo

import (
	"time"

	"github.com/xraph/grove"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/promptversion"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

// ──────────────────────────────────────────────────
// Suite model
// ──────────────────────────────────────────────────

type suiteModel struct {
	grove.BaseModel `grove:"table:sentinel_suites"`
	ID              string         `grove:"id,pk" bson:"_id"`
	Name            string         `grove:"name,notnull" bson:"name"`
	Description     string         `grove:"description" bson:"description"`
	AppID           string         `grove:"app_id,notnull" bson:"app_id"`
	SystemPrompt    string         `grove:"system_prompt" bson:"system_prompt"`
	Model           string         `grove:"model,notnull" bson:"model"`
	Temperature     float64        `grove:"temperature,notnull" bson:"temperature"`
	PersonaRef      string         `grove:"persona_ref" bson:"persona_ref"`
	Metadata        map[string]any `grove:"metadata" bson:"metadata"`
	CreatedAt       time.Time      `grove:"created_at,notnull" bson:"created_at"`
	UpdatedAt       time.Time      `grove:"updated_at,notnull" bson:"updated_at"`
}

func suiteToModel(s *suite.Suite) *suiteModel {
	return &suiteModel{
		ID:           s.ID.String(),
		Name:         s.Name,
		Description:  s.Description,
		AppID:        s.AppID,
		SystemPrompt: s.SystemPrompt,
		Model:        s.Model,
		Temperature:  s.Temperature,
		PersonaRef:   s.PersonaRef,
		Metadata:     s.Metadata,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

func suiteFromModel(m *suiteModel) *suite.Suite {
	sid, _ := id.ParseSuiteID(m.ID) //nolint:errcheck // stored IDs are always valid
	return &suite.Suite{
		Entity:       entityFromTimestamps(m.CreatedAt, m.UpdatedAt),
		ID:           sid,
		Name:         m.Name,
		Description:  m.Description,
		AppID:        m.AppID,
		SystemPrompt: m.SystemPrompt,
		Model:        m.Model,
		Temperature:  m.Temperature,
		PersonaRef:   m.PersonaRef,
		Metadata:     m.Metadata,
	}
}

// ──────────────────────────────────────────────────
// Case model
// ──────────────────────────────────────────────────

type caseModel struct {
	grove.BaseModel `grove:"table:sentinel_cases"`
	ID              string                  `grove:"id,pk" bson:"_id"`
	SuiteID         string                  `grove:"suite_id,notnull" bson:"suite_id"`
	Name            string                  `grove:"name,notnull" bson:"name"`
	Input           string                  `grove:"input,notnull" bson:"input"`
	Expected        string                  `grove:"expected" bson:"expected"`
	ScenarioType    string                  `grove:"scenario_type,notnull" bson:"scenario_type"`
	Scorers         []testcase.ScorerConfig `grove:"scorers" bson:"scorers"`
	Tags            []string                `grove:"tags" bson:"tags"`
	Context         map[string]any          `grove:"context" bson:"context"`
	Metadata        map[string]any          `grove:"metadata" bson:"metadata"`
	CreatedAt       time.Time               `grove:"created_at,notnull" bson:"created_at"`
	UpdatedAt       time.Time               `grove:"updated_at,notnull" bson:"updated_at"`
}

func caseToModel(tc *testcase.Case) *caseModel {
	return &caseModel{
		ID:           tc.ID.String(),
		SuiteID:      tc.SuiteID.String(),
		Name:         tc.Name,
		Input:        tc.Input,
		Expected:     tc.Expected,
		ScenarioType: string(tc.ScenarioType),
		Scorers:      tc.Scorers,
		Tags:         tc.Tags,
		Context:      tc.Context,
		Metadata:     tc.Metadata,
		CreatedAt:    tc.CreatedAt,
		UpdatedAt:    tc.UpdatedAt,
	}
}

func caseFromModel(m *caseModel) *testcase.Case {
	cid, _ := id.ParseCaseID(m.ID)       //nolint:errcheck // stored IDs are always valid
	sid, _ := id.ParseSuiteID(m.SuiteID) //nolint:errcheck // stored IDs are always valid
	return &testcase.Case{
		Entity:       entityFromTimestamps(m.CreatedAt, m.UpdatedAt),
		ID:           cid,
		SuiteID:      sid,
		Name:         m.Name,
		Input:        m.Input,
		Expected:     m.Expected,
		ScenarioType: testcase.ScenarioType(m.ScenarioType),
		Scorers:      m.Scorers,
		Tags:         m.Tags,
		Context:      m.Context,
		Metadata:     m.Metadata,
	}
}

// ──────────────────────────────────────────────────
// Run model
// ──────────────────────────────────────────────────

type runModel struct {
	grove.BaseModel `grove:"table:sentinel_runs"`
	ID              string             `grove:"id,pk" bson:"_id"`
	SuiteID         string             `grove:"suite_id,notnull" bson:"suite_id"`
	Model           string             `grove:"model,notnull" bson:"model"`
	SystemPrompt    string             `grove:"system_prompt" bson:"system_prompt"`
	Temperature     float64            `grove:"temperature,notnull" bson:"temperature"`
	TotalCases      int                `grove:"total_cases,notnull" bson:"total_cases"`
	Passed          int                `grove:"passed,notnull" bson:"passed"`
	Failed          int                `grove:"failed,notnull" bson:"failed"`
	PassRate        float64            `grove:"pass_rate,notnull" bson:"pass_rate"`
	AvgScore        float64            `grove:"avg_score,notnull" bson:"avg_score"`
	AvgLatencyMs    int                `grove:"avg_latency_ms,notnull" bson:"avg_latency_ms"`
	TotalTokens     int                `grove:"total_tokens,notnull" bson:"total_tokens"`
	TotalCost       float64            `grove:"total_cost,notnull" bson:"total_cost"`
	AppID           string             `grove:"app_id,notnull" bson:"app_id"`
	TargetTenantID  string             `grove:"target_tenant_id" bson:"target_tenant_id"`
	PersonaRef      string             `grove:"persona_ref" bson:"persona_ref"`
	Config          map[string]any     `grove:"config" bson:"config"`
	State           string             `grove:"state,notnull" bson:"state"`
	Error           string             `grove:"error" bson:"error"`
	CompletedAt     *time.Time         `grove:"completed_at" bson:"completed_at"`
	DimensionScores map[string]float64 `grove:"dimension_scores" bson:"dimension_scores"`
	CreatedAt       time.Time          `grove:"created_at,notnull" bson:"created_at"`
	UpdatedAt       time.Time          `grove:"updated_at,notnull" bson:"updated_at"`
}

func runToModel(r *evalrun.Run) *runModel {
	return &runModel{
		ID:              r.ID.String(),
		SuiteID:         r.SuiteID.String(),
		Model:           r.Model,
		SystemPrompt:    r.SystemPrompt,
		Temperature:     r.Temperature,
		TotalCases:      r.TotalCases,
		Passed:          r.Passed,
		Failed:          r.Failed,
		PassRate:        r.PassRate,
		AvgScore:        r.AvgScore,
		AvgLatencyMs:    r.AvgLatencyMs,
		TotalTokens:     r.TotalTokens,
		TotalCost:       r.TotalCost,
		AppID:           r.AppID,
		TargetTenantID:  r.TargetTenantID,
		PersonaRef:      r.PersonaRef,
		Config:          r.Config,
		State:           string(r.State),
		Error:           r.Error,
		CompletedAt:     r.CompletedAt,
		DimensionScores: r.DimensionScores,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func runFromModel(m *runModel) *evalrun.Run {
	rid, _ := id.ParseEvalRunID(m.ID)    //nolint:errcheck // stored IDs are always valid
	sid, _ := id.ParseSuiteID(m.SuiteID) //nolint:errcheck // stored IDs are always valid
	return &evalrun.Run{
		Entity:          entityFromTimestamps(m.CreatedAt, m.UpdatedAt),
		ID:              rid,
		SuiteID:         sid,
		Model:           m.Model,
		SystemPrompt:    m.SystemPrompt,
		Temperature:     m.Temperature,
		TotalCases:      m.TotalCases,
		Passed:          m.Passed,
		Failed:          m.Failed,
		PassRate:        m.PassRate,
		AvgScore:        m.AvgScore,
		AvgLatencyMs:    m.AvgLatencyMs,
		TotalTokens:     m.TotalTokens,
		TotalCost:       m.TotalCost,
		AppID:           m.AppID,
		TargetTenantID:  m.TargetTenantID,
		PersonaRef:      m.PersonaRef,
		Config:          m.Config,
		State:           evalrun.RunState(m.State),
		Error:           m.Error,
		CompletedAt:     m.CompletedAt,
		DimensionScores: m.DimensionScores,
	}
}

// ──────────────────────────────────────────────────
// Result model
// ──────────────────────────────────────────────────

type resultModel struct {
	grove.BaseModel `grove:"table:sentinel_results"`
	ID              string                 `grove:"id,pk" bson:"_id"`
	RunID           string                 `grove:"run_id,notnull" bson:"run_id"`
	CaseID          string                 `grove:"case_id,notnull" bson:"case_id"`
	CaseName        string                 `grove:"case_name,notnull" bson:"case_name"`
	Status          string                 `grove:"status,notnull" bson:"status"`
	Score           float64                `grove:"score,notnull" bson:"score"`
	Output          string                 `grove:"output" bson:"output"`
	LatencyMs       int                    `grove:"latency_ms,notnull" bson:"latency_ms"`
	TokensUsed      int                    `grove:"tokens_used,notnull" bson:"tokens_used"`
	Cost            float64                `grove:"cost,notnull" bson:"cost"`
	ScorerResults   []evalrun.ScorerResult `grove:"scorer_results" bson:"scorer_results"`
	Error           string                 `grove:"error" bson:"error"`
	DimensionScores map[string]float64     `grove:"dimension_scores" bson:"dimension_scores"`
	RunTrace        *evalrun.RunTrace      `grove:"run_trace" bson:"run_trace"`
	CreatedAt       time.Time              `grove:"created_at,notnull" bson:"created_at"`
	UpdatedAt       time.Time              `grove:"updated_at,notnull" bson:"updated_at"`
}

func resultToModel(r *evalrun.Result) *resultModel {
	return &resultModel{
		ID:              r.ID.String(),
		RunID:           r.RunID.String(),
		CaseID:          r.CaseID.String(),
		CaseName:        r.CaseName,
		Status:          string(r.Status),
		Score:           r.Score,
		Output:          r.Output,
		LatencyMs:       r.LatencyMs,
		TokensUsed:      r.TokensUsed,
		Cost:            r.Cost,
		ScorerResults:   r.ScorerResults,
		Error:           r.Error,
		DimensionScores: r.DimensionScores,
		RunTrace:        r.RunTrace,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func resultFromModel(m *resultModel) *evalrun.Result {
	rid, _ := id.ParseEvalResultID(m.ID)   //nolint:errcheck // stored IDs are always valid
	runID, _ := id.ParseEvalRunID(m.RunID) //nolint:errcheck // stored IDs are always valid
	caseID, _ := id.ParseCaseID(m.CaseID)  //nolint:errcheck // stored IDs are always valid
	return &evalrun.Result{
		Entity:          entityFromTimestamps(m.CreatedAt, m.UpdatedAt),
		ID:              rid,
		RunID:           runID,
		CaseID:          caseID,
		CaseName:        m.CaseName,
		Status:          evalrun.ResultStatus(m.Status),
		Score:           m.Score,
		Output:          m.Output,
		LatencyMs:       m.LatencyMs,
		TokensUsed:      m.TokensUsed,
		Cost:            m.Cost,
		ScorerResults:   m.ScorerResults,
		Error:           m.Error,
		DimensionScores: m.DimensionScores,
		RunTrace:        m.RunTrace,
	}
}

// ──────────────────────────────────────────────────
// Baseline model
// ──────────────────────────────────────────────────

type baselineModel struct {
	grove.BaseModel `grove:"table:sentinel_baselines"`
	ID              string             `grove:"id,pk" bson:"_id"`
	SuiteID         string             `grove:"suite_id,notnull" bson:"suite_id"`
	RunID           string             `grove:"run_id,notnull" bson:"run_id"`
	Name            string             `grove:"name,notnull" bson:"name"`
	Results         []baseline.Result  `grove:"results" bson:"results"`
	PassRate        float64            `grove:"pass_rate,notnull" bson:"pass_rate"`
	AvgScore        float64            `grove:"avg_score,notnull" bson:"avg_score"`
	DimensionScores map[string]float64 `grove:"dimension_scores" bson:"dimension_scores"`
	IsCurrent       bool               `grove:"is_current,notnull" bson:"is_current"`
	CreatedAt       time.Time          `grove:"created_at,notnull" bson:"created_at"`
}

func baselineToModel(b *baseline.Baseline) *baselineModel {
	return &baselineModel{
		ID:              b.ID.String(),
		SuiteID:         b.SuiteID.String(),
		RunID:           b.RunID.String(),
		Name:            b.Name,
		Results:         b.Results,
		PassRate:        b.PassRate,
		AvgScore:        b.AvgScore,
		DimensionScores: b.DimensionScores,
		IsCurrent:       b.IsCurrent,
		CreatedAt:       b.CreatedAt,
	}
}

func baselineFromModel(m *baselineModel) *baseline.Baseline {
	bid, _ := id.ParseBaselineID(m.ID)   //nolint:errcheck // stored IDs are always valid
	sid, _ := id.ParseSuiteID(m.SuiteID) //nolint:errcheck // stored IDs are always valid
	rid, _ := id.ParseEvalRunID(m.RunID) //nolint:errcheck // stored IDs are always valid
	return &baseline.Baseline{
		ID:              bid,
		SuiteID:         sid,
		RunID:           rid,
		Name:            m.Name,
		Results:         m.Results,
		PassRate:        m.PassRate,
		AvgScore:        m.AvgScore,
		DimensionScores: m.DimensionScores,
		IsCurrent:       m.IsCurrent,
		CreatedAt:       m.CreatedAt,
	}
}

// ──────────────────────────────────────────────────
// Prompt version model
// ──────────────────────────────────────────────────

type promptVersionModel struct {
	grove.BaseModel `grove:"table:sentinel_prompt_versions"`
	ID              string    `grove:"id,pk" bson:"_id"`
	SuiteID         string    `grove:"suite_id,notnull" bson:"suite_id"`
	Version         int       `grove:"version,notnull" bson:"version"`
	SystemPrompt    string    `grove:"system_prompt,notnull" bson:"system_prompt"`
	Changelog       string    `grove:"changelog" bson:"changelog"`
	IsCurrent       bool      `grove:"is_current,notnull" bson:"is_current"`
	RunID           string    `grove:"run_id" bson:"run_id"`
	PassRate        *float64  `grove:"pass_rate" bson:"pass_rate"`
	AvgScore        *float64  `grove:"avg_score" bson:"avg_score"`
	CreatedAt       time.Time `grove:"created_at,notnull" bson:"created_at"`
}

func promptVersionToModel(pv *promptversion.PromptVersion) *promptVersionModel {
	return &promptVersionModel{
		ID:           pv.ID.String(),
		SuiteID:      pv.SuiteID.String(),
		Version:      pv.Version,
		SystemPrompt: pv.SystemPrompt,
		Changelog:    pv.Changelog,
		IsCurrent:    pv.IsCurrent,
		RunID:        pv.RunID,
		PassRate:     pv.PassRate,
		AvgScore:     pv.AvgScore,
		CreatedAt:    pv.CreatedAt,
	}
}

func promptVersionFromModel(m *promptVersionModel) *promptversion.PromptVersion {
	pvid, _ := id.ParsePromptVersionID(m.ID) //nolint:errcheck // stored IDs are always valid
	sid, _ := id.ParseSuiteID(m.SuiteID)     //nolint:errcheck // stored IDs are always valid
	return &promptversion.PromptVersion{
		ID:           pvid,
		SuiteID:      sid,
		Version:      m.Version,
		SystemPrompt: m.SystemPrompt,
		Changelog:    m.Changelog,
		IsCurrent:    m.IsCurrent,
		RunID:        m.RunID,
		PassRate:     m.PassRate,
		AvgScore:     m.AvgScore,
		CreatedAt:    m.CreatedAt,
	}
}

// ──────────────────────────────────────────────────
// Helper
// ──────────────────────────────────────────────────

func entityFromTimestamps(createdAt, updatedAt time.Time) sentinel.Entity {
	return sentinel.Entity{CreatedAt: createdAt, UpdatedAt: updatedAt}
}
