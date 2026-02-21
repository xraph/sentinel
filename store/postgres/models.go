package postgres

import (
	"time"

	"github.com/uptrace/bun"

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
	bun.BaseModel `bun:"table:sentinel_suites"`
	ID            string         `bun:"id,pk"`
	Name          string         `bun:"name,notnull"`
	Description   string         `bun:"description"`
	AppID         string         `bun:"app_id,notnull"`
	SystemPrompt  string         `bun:"system_prompt"`
	Model         string         `bun:"model,notnull"`
	Temperature   float64        `bun:"temperature,notnull"`
	PersonaRef    string         `bun:"persona_ref"`
	Metadata      map[string]any `bun:"metadata,type:jsonb"`
	CreatedAt     time.Time      `bun:"created_at,notnull"`
	UpdatedAt     time.Time      `bun:"updated_at,notnull"`
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
	bun.BaseModel `bun:"table:sentinel_cases"`
	ID            string                  `bun:"id,pk"`
	SuiteID       string                  `bun:"suite_id,notnull"`
	Name          string                  `bun:"name,notnull"`
	Input         string                  `bun:"input,notnull"`
	Expected      string                  `bun:"expected"`
	ScenarioType  string                  `bun:"scenario_type,notnull"`
	Scorers       []testcase.ScorerConfig `bun:"scorers,type:jsonb"`
	Tags          []string                `bun:"tags,type:jsonb"`
	Context       map[string]any          `bun:"context,type:jsonb"`
	Metadata      map[string]any          `bun:"metadata,type:jsonb"`
	CreatedAt     time.Time               `bun:"created_at,notnull"`
	UpdatedAt     time.Time               `bun:"updated_at,notnull"`
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
	bun.BaseModel   `bun:"table:sentinel_runs"`
	ID              string             `bun:"id,pk"`
	SuiteID         string             `bun:"suite_id,notnull"`
	Model           string             `bun:"model,notnull"`
	SystemPrompt    string             `bun:"system_prompt"`
	Temperature     float64            `bun:"temperature,notnull"`
	TotalCases      int                `bun:"total_cases,notnull"`
	Passed          int                `bun:"passed,notnull"`
	Failed          int                `bun:"failed,notnull"`
	PassRate        float64            `bun:"pass_rate,notnull"`
	AvgScore        float64            `bun:"avg_score,notnull"`
	AvgLatencyMs    int                `bun:"avg_latency_ms,notnull"`
	TotalTokens     int                `bun:"total_tokens,notnull"`
	TotalCost       float64            `bun:"total_cost,notnull"`
	AppID           string             `bun:"app_id,notnull"`
	TargetTenantID  string             `bun:"target_tenant_id"`
	PersonaRef      string             `bun:"persona_ref"`
	Config          map[string]any     `bun:"config,type:jsonb"`
	State           string             `bun:"state,notnull"`
	Error           string             `bun:"error"`
	CompletedAt     *time.Time         `bun:"completed_at"`
	DimensionScores map[string]float64 `bun:"dimension_scores,type:jsonb"`
	CreatedAt       time.Time          `bun:"created_at,notnull"`
	UpdatedAt       time.Time          `bun:"updated_at,notnull"`
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
	bun.BaseModel   `bun:"table:sentinel_results"`
	ID              string                 `bun:"id,pk"`
	RunID           string                 `bun:"run_id,notnull"`
	CaseID          string                 `bun:"case_id,notnull"`
	CaseName        string                 `bun:"case_name,notnull"`
	Status          string                 `bun:"status,notnull"`
	Score           float64                `bun:"score,notnull"`
	Output          string                 `bun:"output"`
	LatencyMs       int                    `bun:"latency_ms,notnull"`
	TokensUsed      int                    `bun:"tokens_used,notnull"`
	Cost            float64                `bun:"cost,notnull"`
	ScorerResults   []evalrun.ScorerResult `bun:"scorer_results,type:jsonb"`
	Error           string                 `bun:"error"`
	DimensionScores map[string]float64     `bun:"dimension_scores,type:jsonb"`
	RunTrace        *evalrun.RunTrace      `bun:"run_trace,type:jsonb"`
	CreatedAt       time.Time              `bun:"created_at,notnull"`
	UpdatedAt       time.Time              `bun:"updated_at,notnull"`
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
	bun.BaseModel   `bun:"table:sentinel_baselines"`
	ID              string             `bun:"id,pk"`
	SuiteID         string             `bun:"suite_id,notnull"`
	RunID           string             `bun:"run_id,notnull"`
	Name            string             `bun:"name,notnull"`
	Results         []baseline.Result  `bun:"results,type:jsonb"`
	PassRate        float64            `bun:"pass_rate,notnull"`
	AvgScore        float64            `bun:"avg_score,notnull"`
	DimensionScores map[string]float64 `bun:"dimension_scores,type:jsonb"`
	IsCurrent       bool               `bun:"is_current,notnull"`
	CreatedAt       time.Time          `bun:"created_at,notnull"`
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
	bun.BaseModel `bun:"table:sentinel_prompt_versions"`
	ID            string    `bun:"id,pk"`
	SuiteID       string    `bun:"suite_id,notnull"`
	Version       int       `bun:"version,notnull"`
	SystemPrompt  string    `bun:"system_prompt,notnull"`
	Changelog     string    `bun:"changelog"`
	IsCurrent     bool      `bun:"is_current,notnull"`
	RunID         string    `bun:"run_id"`
	PassRate      *float64  `bun:"pass_rate"`
	AvgScore      *float64  `bun:"avg_score"`
	CreatedAt     time.Time `bun:"created_at,notnull"`
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
