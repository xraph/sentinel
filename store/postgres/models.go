package postgres

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
	ID              string         `grove:"id,pk"`
	Name            string         `grove:"name,notnull"`
	Description     string         `grove:"description"`
	AppID           string         `grove:"app_id,notnull"`
	SystemPrompt    string         `grove:"system_prompt"`
	Model           string         `grove:"model,notnull"`
	Temperature     float64        `grove:"temperature,notnull"`
	PersonaRef      string         `grove:"persona_ref"`
	Metadata        map[string]any `grove:"metadata,type:jsonb"`
	CreatedAt       time.Time      `grove:"created_at,notnull"`
	UpdatedAt       time.Time      `grove:"updated_at,notnull"`
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
		Metadata:     nonNilMap(s.Metadata),
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
	ID              string                  `grove:"id,pk"`
	SuiteID         string                  `grove:"suite_id,notnull"`
	Name            string                  `grove:"name,notnull"`
	Input           string                  `grove:"input,notnull"`
	Expected        string                  `grove:"expected"`
	ScenarioType    string                  `grove:"scenario_type,notnull"`
	Scorers         []testcase.ScorerConfig `grove:"scorers,type:jsonb"`
	Tags            []string                `grove:"tags,type:jsonb"`
	Context         map[string]any          `grove:"context,type:jsonb"`
	Metadata        map[string]any          `grove:"metadata,type:jsonb"`
	CreatedAt       time.Time               `grove:"created_at,notnull"`
	UpdatedAt       time.Time               `grove:"updated_at,notnull"`
}

func caseToModel(tc *testcase.Case) *caseModel {
	return &caseModel{
		ID:           tc.ID.String(),
		SuiteID:      tc.SuiteID.String(),
		Name:         tc.Name,
		Input:        tc.Input,
		Expected:     tc.Expected,
		ScenarioType: string(tc.ScenarioType),
		Scorers:      nonNilSlice(tc.Scorers),
		Tags:         nonNilSlice(tc.Tags),
		Context:      nonNilMap(tc.Context),
		Metadata:     nonNilMap(tc.Metadata),
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
	ID              string             `grove:"id,pk"`
	SuiteID         string             `grove:"suite_id,notnull"`
	Model           string             `grove:"model,notnull"`
	SystemPrompt    string             `grove:"system_prompt"`
	Temperature     float64            `grove:"temperature,notnull"`
	TotalCases      int                `grove:"total_cases,notnull"`
	Passed          int                `grove:"passed,notnull"`
	Failed          int                `grove:"failed,notnull"`
	PassRate        float64            `grove:"pass_rate,notnull"`
	AvgScore        float64            `grove:"avg_score,notnull"`
	AvgLatencyMs    int                `grove:"avg_latency_ms,notnull"`
	TotalTokens     int                `grove:"total_tokens,notnull"`
	TotalCost       float64            `grove:"total_cost,notnull"`
	AppID           string             `grove:"app_id,notnull"`
	TargetTenantID  string             `grove:"target_tenant_id"`
	PersonaRef      string             `grove:"persona_ref"`
	Config          map[string]any     `grove:"config,type:jsonb"`
	State           string             `grove:"state,notnull"`
	Error           string             `grove:"error"`
	CompletedAt     *time.Time         `grove:"completed_at"`
	DimensionScores map[string]float64 `grove:"dimension_scores,type:jsonb"`
	CreatedAt       time.Time          `grove:"created_at,notnull"`
	UpdatedAt       time.Time          `grove:"updated_at,notnull"`
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
		Config:          nonNilMap(r.Config),
		State:           string(r.State),
		Error:           r.Error,
		CompletedAt:     r.CompletedAt,
		DimensionScores: nonNilMap(r.DimensionScores),
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
	ID              string                 `grove:"id,pk"`
	RunID           string                 `grove:"run_id,notnull"`
	CaseID          string                 `grove:"case_id,notnull"`
	CaseName        string                 `grove:"case_name,notnull"`
	Status          string                 `grove:"status,notnull"`
	Score           float64                `grove:"score,notnull"`
	Output          string                 `grove:"output"`
	LatencyMs       int                    `grove:"latency_ms,notnull"`
	TokensUsed      int                    `grove:"tokens_used,notnull"`
	Cost            float64                `grove:"cost,notnull"`
	ScorerResults   []evalrun.ScorerResult `grove:"scorer_results,type:jsonb"`
	Error           string                 `grove:"error"`
	DimensionScores map[string]float64     `grove:"dimension_scores,type:jsonb"`
	RunTrace        *evalrun.RunTrace      `grove:"run_trace,type:jsonb"`
	CreatedAt       time.Time              `grove:"created_at,notnull"`
	UpdatedAt       time.Time              `grove:"updated_at,notnull"`
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
		ScorerResults:   nonNilSlice(r.ScorerResults),
		Error:           r.Error,
		DimensionScores: nonNilMap(r.DimensionScores),
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
	ID              string             `grove:"id,pk"`
	SuiteID         string             `grove:"suite_id,notnull"`
	RunID           string             `grove:"run_id,notnull"`
	Name            string             `grove:"name,notnull"`
	Results         []baseline.Result  `grove:"results,type:jsonb"`
	PassRate        float64            `grove:"pass_rate,notnull"`
	AvgScore        float64            `grove:"avg_score,notnull"`
	DimensionScores map[string]float64 `grove:"dimension_scores,type:jsonb"`
	IsCurrent       bool               `grove:"is_current,notnull"`
	CreatedAt       time.Time          `grove:"created_at,notnull"`
}

func baselineToModel(b *baseline.Baseline) *baselineModel {
	return &baselineModel{
		ID:              b.ID.String(),
		SuiteID:         b.SuiteID.String(),
		RunID:           b.RunID.String(),
		Name:            b.Name,
		Results:         nonNilSlice(b.Results),
		PassRate:        b.PassRate,
		AvgScore:        b.AvgScore,
		DimensionScores: nonNilMap(b.DimensionScores),
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
	ID              string    `grove:"id,pk"`
	SuiteID         string    `grove:"suite_id,notnull"`
	Version         int       `grove:"version,notnull"`
	SystemPrompt    string    `grove:"system_prompt,notnull"`
	Changelog       string    `grove:"changelog"`
	IsCurrent       bool      `grove:"is_current,notnull"`
	RunID           string    `grove:"run_id"`
	PassRate        *float64  `grove:"pass_rate"`
	AvgScore        *float64  `grove:"avg_score"`
	CreatedAt       time.Time `grove:"created_at,notnull"`
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

// nonNilMap and nonNilSlice turn a nil map or slice into an empty one. pgx
// encodes nil as SQL NULL, and these jsonb columns are NOT NULL, so without
// this every run the engine starts and every failed result is rejected.
func nonNilMap[V any](m map[string]V) map[string]V {
	if m == nil {
		return map[string]V{}
	}
	return m
}

func nonNilSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
