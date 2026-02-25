package sqlite

import (
	"encoding/json"
	"fmt"
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
	ID              string    `grove:"id,pk"`
	Name            string    `grove:"name,notnull"`
	Description     string    `grove:"description"`
	AppID           string    `grove:"app_id,notnull"`
	SystemPrompt    string    `grove:"system_prompt"`
	Model           string    `grove:"model,notnull"`
	Temperature     float64   `grove:"temperature,notnull"`
	PersonaRef      string    `grove:"persona_ref"`
	Metadata        string    `grove:"metadata"`
	CreatedAt       time.Time `grove:"created_at,notnull"`
	UpdatedAt       time.Time `grove:"updated_at,notnull"`
}

func suiteToModel(s *suite.Suite) *suiteModel {
	metadata, err := json.Marshal(s.Metadata)
	if err != nil {
		metadata = []byte("{}")
	}
	return &suiteModel{
		ID:           s.ID.String(),
		Name:         s.Name,
		Description:  s.Description,
		AppID:        s.AppID,
		SystemPrompt: s.SystemPrompt,
		Model:        s.Model,
		Temperature:  s.Temperature,
		PersonaRef:   s.PersonaRef,
		Metadata:     string(metadata),
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

func suiteFromModel(m *suiteModel) (*suite.Suite, error) {
	sid, _ := id.ParseSuiteID(m.ID) //nolint:errcheck // stored IDs are always valid

	var metadata map[string]any
	if m.Metadata != "" {
		if err := json.Unmarshal([]byte(m.Metadata), &metadata); err != nil {
			return nil, fmt.Errorf("unmarshal suite metadata: %w", err)
		}
	}

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
		Metadata:     metadata,
	}, nil
}

// ──────────────────────────────────────────────────
// Case model
// ──────────────────────────────────────────────────

type caseModel struct {
	grove.BaseModel `grove:"table:sentinel_cases"`
	ID              string    `grove:"id,pk"`
	SuiteID         string    `grove:"suite_id,notnull"`
	Name            string    `grove:"name,notnull"`
	Input           string    `grove:"input,notnull"`
	Expected        string    `grove:"expected"`
	ScenarioType    string    `grove:"scenario_type,notnull"`
	Scorers         string    `grove:"scorers"`
	Tags            string    `grove:"tags"`
	Context         string    `grove:"context"`
	Metadata        string    `grove:"metadata"`
	CreatedAt       time.Time `grove:"created_at,notnull"`
	UpdatedAt       time.Time `grove:"updated_at,notnull"`
}

func caseToModel(tc *testcase.Case) *caseModel {
	scorers, err := json.Marshal(tc.Scorers)
	if err != nil {
		scorers = []byte("[]")
	}
	tags, err := json.Marshal(tc.Tags)
	if err != nil {
		tags = []byte("[]")
	}
	ctx, err := json.Marshal(tc.Context)
	if err != nil {
		ctx = []byte("{}")
	}
	metadata, err := json.Marshal(tc.Metadata)
	if err != nil {
		metadata = []byte("{}")
	}
	return &caseModel{
		ID:           tc.ID.String(),
		SuiteID:      tc.SuiteID.String(),
		Name:         tc.Name,
		Input:        tc.Input,
		Expected:     tc.Expected,
		ScenarioType: string(tc.ScenarioType),
		Scorers:      string(scorers),
		Tags:         string(tags),
		Context:      string(ctx),
		Metadata:     string(metadata),
		CreatedAt:    tc.CreatedAt,
		UpdatedAt:    tc.UpdatedAt,
	}
}

func caseFromModel(m *caseModel) (*testcase.Case, error) {
	cid, _ := id.ParseCaseID(m.ID)       //nolint:errcheck // stored IDs are always valid
	sid, _ := id.ParseSuiteID(m.SuiteID) //nolint:errcheck // stored IDs are always valid

	var scorers []testcase.ScorerConfig
	if m.Scorers != "" {
		if err := json.Unmarshal([]byte(m.Scorers), &scorers); err != nil {
			return nil, fmt.Errorf("unmarshal case scorers: %w", err)
		}
	}

	var tags []string
	if m.Tags != "" {
		if err := json.Unmarshal([]byte(m.Tags), &tags); err != nil {
			return nil, fmt.Errorf("unmarshal case tags: %w", err)
		}
	}

	var ctx map[string]any
	if m.Context != "" {
		if err := json.Unmarshal([]byte(m.Context), &ctx); err != nil {
			return nil, fmt.Errorf("unmarshal case context: %w", err)
		}
	}

	var metadata map[string]any
	if m.Metadata != "" {
		if err := json.Unmarshal([]byte(m.Metadata), &metadata); err != nil {
			return nil, fmt.Errorf("unmarshal case metadata: %w", err)
		}
	}

	return &testcase.Case{
		Entity:       entityFromTimestamps(m.CreatedAt, m.UpdatedAt),
		ID:           cid,
		SuiteID:      sid,
		Name:         m.Name,
		Input:        m.Input,
		Expected:     m.Expected,
		ScenarioType: testcase.ScenarioType(m.ScenarioType),
		Scorers:      scorers,
		Tags:         tags,
		Context:      ctx,
		Metadata:     metadata,
	}, nil
}

// ──────────────────────────────────────────────────
// Run model
// ──────────────────────────────────────────────────

type runModel struct {
	grove.BaseModel `grove:"table:sentinel_runs"`
	ID              string     `grove:"id,pk"`
	SuiteID         string     `grove:"suite_id,notnull"`
	Model           string     `grove:"model,notnull"`
	SystemPrompt    string     `grove:"system_prompt"`
	Temperature     float64    `grove:"temperature,notnull"`
	TotalCases      int        `grove:"total_cases,notnull"`
	Passed          int        `grove:"passed,notnull"`
	Failed          int        `grove:"failed,notnull"`
	PassRate        float64    `grove:"pass_rate,notnull"`
	AvgScore        float64    `grove:"avg_score,notnull"`
	AvgLatencyMs    int        `grove:"avg_latency_ms,notnull"`
	TotalTokens     int        `grove:"total_tokens,notnull"`
	TotalCost       float64    `grove:"total_cost,notnull"`
	AppID           string     `grove:"app_id,notnull"`
	TargetTenantID  string     `grove:"target_tenant_id"`
	PersonaRef      string     `grove:"persona_ref"`
	Config          string     `grove:"config"`
	State           string     `grove:"state,notnull"`
	Error           string     `grove:"error"`
	CompletedAt     *time.Time `grove:"completed_at"`
	DimensionScores string     `grove:"dimension_scores"`
	CreatedAt       time.Time  `grove:"created_at,notnull"`
	UpdatedAt       time.Time  `grove:"updated_at,notnull"`
}

func runToModel(r *evalrun.Run) *runModel {
	config, err := json.Marshal(r.Config)
	if err != nil {
		config = []byte("{}")
	}
	dimScores, err := json.Marshal(r.DimensionScores)
	if err != nil {
		dimScores = []byte("{}")
	}
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
		Config:          string(config),
		State:           string(r.State),
		Error:           r.Error,
		CompletedAt:     r.CompletedAt,
		DimensionScores: string(dimScores),
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func runFromModel(m *runModel) (*evalrun.Run, error) {
	rid, _ := id.ParseEvalRunID(m.ID)    //nolint:errcheck // stored IDs are always valid
	sid, _ := id.ParseSuiteID(m.SuiteID) //nolint:errcheck // stored IDs are always valid

	var config map[string]any
	if m.Config != "" {
		if err := json.Unmarshal([]byte(m.Config), &config); err != nil {
			return nil, fmt.Errorf("unmarshal run config: %w", err)
		}
	}

	var dimScores map[string]float64
	if m.DimensionScores != "" {
		if err := json.Unmarshal([]byte(m.DimensionScores), &dimScores); err != nil {
			return nil, fmt.Errorf("unmarshal run dimension scores: %w", err)
		}
	}

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
		Config:          config,
		State:           evalrun.RunState(m.State),
		Error:           m.Error,
		CompletedAt:     m.CompletedAt,
		DimensionScores: dimScores,
	}, nil
}

// ──────────────────────────────────────────────────
// Result model
// ──────────────────────────────────────────────────

type resultModel struct {
	grove.BaseModel `grove:"table:sentinel_results"`
	ID              string    `grove:"id,pk"`
	RunID           string    `grove:"run_id,notnull"`
	CaseID          string    `grove:"case_id,notnull"`
	CaseName        string    `grove:"case_name,notnull"`
	Status          string    `grove:"status,notnull"`
	Score           float64   `grove:"score,notnull"`
	Output          string    `grove:"output"`
	LatencyMs       int       `grove:"latency_ms,notnull"`
	TokensUsed      int       `grove:"tokens_used,notnull"`
	Cost            float64   `grove:"cost,notnull"`
	ScorerResults   string    `grove:"scorer_results"`
	Error           string    `grove:"error"`
	DimensionScores string    `grove:"dimension_scores"`
	RunTrace        string    `grove:"run_trace"`
	CreatedAt       time.Time `grove:"created_at,notnull"`
	UpdatedAt       time.Time `grove:"updated_at,notnull"`
}

func resultToModel(r *evalrun.Result) *resultModel {
	scorerResults, err := json.Marshal(r.ScorerResults)
	if err != nil {
		scorerResults = []byte("[]")
	}
	dimScores, err := json.Marshal(r.DimensionScores)
	if err != nil {
		dimScores = []byte("{}")
	}
	var runTrace string
	if r.RunTrace != nil {
		rt, err := json.Marshal(r.RunTrace)
		if err == nil {
			runTrace = string(rt)
		}
	}
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
		ScorerResults:   string(scorerResults),
		Error:           r.Error,
		DimensionScores: string(dimScores),
		RunTrace:        runTrace,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func resultFromModel(m *resultModel) (*evalrun.Result, error) {
	rid, _ := id.ParseEvalResultID(m.ID)   //nolint:errcheck // stored IDs are always valid
	runID, _ := id.ParseEvalRunID(m.RunID) //nolint:errcheck // stored IDs are always valid
	caseID, _ := id.ParseCaseID(m.CaseID)  //nolint:errcheck // stored IDs are always valid

	var scorerResults []evalrun.ScorerResult
	if m.ScorerResults != "" {
		if err := json.Unmarshal([]byte(m.ScorerResults), &scorerResults); err != nil {
			return nil, fmt.Errorf("unmarshal result scorer results: %w", err)
		}
	}

	var dimScores map[string]float64
	if m.DimensionScores != "" {
		if err := json.Unmarshal([]byte(m.DimensionScores), &dimScores); err != nil {
			return nil, fmt.Errorf("unmarshal result dimension scores: %w", err)
		}
	}

	var runTrace *evalrun.RunTrace
	if m.RunTrace != "" {
		runTrace = new(evalrun.RunTrace)
		if err := json.Unmarshal([]byte(m.RunTrace), runTrace); err != nil {
			return nil, fmt.Errorf("unmarshal result run trace: %w", err)
		}
	}

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
		ScorerResults:   scorerResults,
		Error:           m.Error,
		DimensionScores: dimScores,
		RunTrace:        runTrace,
	}, nil
}

// ──────────────────────────────────────────────────
// Baseline model
// ──────────────────────────────────────────────────

type baselineModel struct {
	grove.BaseModel `grove:"table:sentinel_baselines"`
	ID              string    `grove:"id,pk"`
	SuiteID         string    `grove:"suite_id,notnull"`
	RunID           string    `grove:"run_id,notnull"`
	Name            string    `grove:"name,notnull"`
	Results         string    `grove:"results"`
	PassRate        float64   `grove:"pass_rate,notnull"`
	AvgScore        float64   `grove:"avg_score,notnull"`
	DimensionScores string    `grove:"dimension_scores"`
	IsCurrent       bool      `grove:"is_current,notnull"`
	CreatedAt       time.Time `grove:"created_at,notnull"`
}

func baselineToModel(b *baseline.Baseline) *baselineModel {
	results, err := json.Marshal(b.Results)
	if err != nil {
		results = []byte("[]")
	}
	dimScores, err := json.Marshal(b.DimensionScores)
	if err != nil {
		dimScores = []byte("{}")
	}
	return &baselineModel{
		ID:              b.ID.String(),
		SuiteID:         b.SuiteID.String(),
		RunID:           b.RunID.String(),
		Name:            b.Name,
		Results:         string(results),
		PassRate:        b.PassRate,
		AvgScore:        b.AvgScore,
		DimensionScores: string(dimScores),
		IsCurrent:       b.IsCurrent,
		CreatedAt:       b.CreatedAt,
	}
}

func baselineFromModel(m *baselineModel) (*baseline.Baseline, error) {
	bid, _ := id.ParseBaselineID(m.ID)   //nolint:errcheck // stored IDs are always valid
	sid, _ := id.ParseSuiteID(m.SuiteID) //nolint:errcheck // stored IDs are always valid
	rid, _ := id.ParseEvalRunID(m.RunID) //nolint:errcheck // stored IDs are always valid

	var results []baseline.Result
	if m.Results != "" {
		if err := json.Unmarshal([]byte(m.Results), &results); err != nil {
			return nil, fmt.Errorf("unmarshal baseline results: %w", err)
		}
	}

	var dimScores map[string]float64
	if m.DimensionScores != "" {
		if err := json.Unmarshal([]byte(m.DimensionScores), &dimScores); err != nil {
			return nil, fmt.Errorf("unmarshal baseline dimension scores: %w", err)
		}
	}

	return &baseline.Baseline{
		ID:              bid,
		SuiteID:         sid,
		RunID:           rid,
		Name:            m.Name,
		Results:         results,
		PassRate:        m.PassRate,
		AvgScore:        m.AvgScore,
		DimensionScores: dimScores,
		IsCurrent:       m.IsCurrent,
		CreatedAt:       m.CreatedAt,
	}, nil
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
