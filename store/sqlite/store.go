// Package sqlite provides a SQLite implementation of the Sentinel composite
// store using bun ORM. Suitable for local development and single-node testing.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/promptversion"
	"github.com/xraph/sentinel/store"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

// Compile-time interface check.
var _ store.Store = (*Store)(nil)

// Store is a SQLite implementation of the composite Sentinel store.
type Store struct {
	db *bun.DB
}

// New creates a new SQLite store.
func New(db *bun.DB) *Store {
	return &Store{db: db}
}

// Migrate creates tables if they don't exist.
func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sentinel_suites (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT DEFAULT '',
			app_id TEXT NOT NULL, system_prompt TEXT DEFAULT '', model TEXT NOT NULL,
			temperature REAL DEFAULT 0, persona_ref TEXT DEFAULT '',
			metadata TEXT DEFAULT '{}', created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			UNIQUE(app_id, name))`,
		`CREATE TABLE IF NOT EXISTS sentinel_cases (
			id TEXT PRIMARY KEY, suite_id TEXT NOT NULL REFERENCES sentinel_suites(id),
			name TEXT NOT NULL, input TEXT NOT NULL, expected TEXT DEFAULT '',
			scenario_type TEXT DEFAULT 'standard', scorers TEXT DEFAULT '[]',
			tags TEXT DEFAULT '[]', context TEXT DEFAULT '{}', metadata TEXT DEFAULT '{}',
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS sentinel_runs (
			id TEXT PRIMARY KEY, suite_id TEXT NOT NULL REFERENCES sentinel_suites(id),
			model TEXT NOT NULL, system_prompt TEXT DEFAULT '', temperature REAL DEFAULT 0,
			total_cases INT DEFAULT 0, passed INT DEFAULT 0, failed INT DEFAULT 0,
			pass_rate REAL DEFAULT 0, avg_score REAL DEFAULT 0, avg_latency_ms INT DEFAULT 0,
			total_tokens INT DEFAULT 0, total_cost REAL DEFAULT 0,
			app_id TEXT NOT NULL, target_tenant_id TEXT DEFAULT '', persona_ref TEXT DEFAULT '',
			config TEXT DEFAULT '{}', state TEXT DEFAULT 'running', error TEXT DEFAULT '',
			completed_at TEXT, dimension_scores TEXT DEFAULT '{}',
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS sentinel_results (
			id TEXT PRIMARY KEY, run_id TEXT NOT NULL REFERENCES sentinel_runs(id),
			case_id TEXT NOT NULL, case_name TEXT NOT NULL, status TEXT NOT NULL,
			score REAL DEFAULT 0, output TEXT DEFAULT '', latency_ms INT DEFAULT 0,
			tokens_used INT DEFAULT 0, cost REAL DEFAULT 0, scorer_results TEXT DEFAULT '[]',
			error TEXT DEFAULT '', dimension_scores TEXT DEFAULT '{}', run_trace TEXT,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS sentinel_baselines (
			id TEXT PRIMARY KEY, suite_id TEXT NOT NULL REFERENCES sentinel_suites(id),
			run_id TEXT NOT NULL, name TEXT NOT NULL, results TEXT NOT NULL,
			pass_rate REAL DEFAULT 0, avg_score REAL DEFAULT 0,
			dimension_scores TEXT DEFAULT '{}', is_current INTEGER DEFAULT 0,
			created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS sentinel_prompt_versions (
			id TEXT PRIMARY KEY, suite_id TEXT NOT NULL REFERENCES sentinel_suites(id),
			version INT NOT NULL, system_prompt TEXT NOT NULL, changelog TEXT DEFAULT '',
			is_current INTEGER DEFAULT 0, run_id TEXT DEFAULT '', pass_rate REAL, avg_score REAL,
			created_at TEXT NOT NULL, UNIQUE(suite_id, version))`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("sentinel: %w: %w", sentinel.ErrMigrationFailed, err)
		}
	}
	return nil
}

// Ping verifies the database connection.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// ──────────────────────────────────────────────────
// Suite operations
// ──────────────────────────────────────────────────

func (s *Store) CreateSuite(ctx context.Context, su *suite.Suite) error {
	now := time.Now().UTC()
	su.CreatedAt = now
	su.UpdatedAt = now
	_, err := s.db.NewInsert().Model(su).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create suite: %w", err)
	}
	return nil
}

func (s *Store) GetSuite(ctx context.Context, suiteID id.SuiteID) (*suite.Suite, error) {
	su := new(suite.Suite)
	err := s.db.NewSelect().Model(su).Where("id = ?", suiteID.String()).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sentinel.ErrSuiteNotFound
		}
		return nil, fmt.Errorf("sentinel: get suite: %w", err)
	}
	return su, nil
}

func (s *Store) GetSuiteByName(ctx context.Context, appID, name string) (*suite.Suite, error) {
	su := new(suite.Suite)
	err := s.db.NewSelect().Model(su).Where("app_id = ?", appID).Where("name = ?", name).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sentinel.ErrSuiteNotFound
		}
		return nil, fmt.Errorf("sentinel: get suite by name: %w", err)
	}
	return su, nil
}

func (s *Store) UpdateSuite(ctx context.Context, su *suite.Suite) error {
	su.UpdatedAt = time.Now().UTC()
	_, err := s.db.NewUpdate().Model(su).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: update suite: %w", err)
	}
	return nil
}

func (s *Store) DeleteSuite(ctx context.Context, suiteID id.SuiteID) error {
	_, err := s.db.NewDelete().Model((*suite.Suite)(nil)).Where("id = ?", suiteID.String()).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: delete suite: %w", err)
	}
	return nil
}

func (s *Store) ListSuites(ctx context.Context, filter *suite.ListFilter) ([]*suite.Suite, error) {
	var result []*suite.Suite
	q := s.db.NewSelect().Model(&result).OrderExpr("created_at ASC")
	if filter != nil {
		if filter.AppID != "" {
			q = q.Where("app_id = ?", filter.AppID)
		}
		if filter.Limit > 0 {
			q = q.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			q = q.Offset(filter.Offset)
		}
	}
	if err := q.Scan(ctx); err != nil {
		return nil, fmt.Errorf("sentinel: list suites: %w", err)
	}
	return result, nil
}

// ──────────────────────────────────────────────────
// Case operations
// ──────────────────────────────────────────────────

func (s *Store) CreateCase(ctx context.Context, tc *testcase.Case) error {
	now := time.Now().UTC()
	tc.CreatedAt = now
	tc.UpdatedAt = now
	_, err := s.db.NewInsert().Model(tc).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create case: %w", err)
	}
	return nil
}

func (s *Store) CreateCaseBatch(ctx context.Context, cases []*testcase.Case) error {
	now := time.Now().UTC()
	for _, tc := range cases {
		tc.CreatedAt = now
		tc.UpdatedAt = now
	}
	_, err := s.db.NewInsert().Model(&cases).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create case batch: %w", err)
	}
	return nil
}

func (s *Store) GetCase(ctx context.Context, caseID id.CaseID) (*testcase.Case, error) {
	tc := new(testcase.Case)
	err := s.db.NewSelect().Model(tc).Where("id = ?", caseID.String()).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sentinel.ErrCaseNotFound
		}
		return nil, fmt.Errorf("sentinel: get case: %w", err)
	}
	return tc, nil
}

func (s *Store) UpdateCase(ctx context.Context, tc *testcase.Case) error {
	tc.UpdatedAt = time.Now().UTC()
	_, err := s.db.NewUpdate().Model(tc).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: update case: %w", err)
	}
	return nil
}

func (s *Store) DeleteCase(ctx context.Context, caseID id.CaseID) error {
	_, err := s.db.NewDelete().Model((*testcase.Case)(nil)).Where("id = ?", caseID.String()).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: delete case: %w", err)
	}
	return nil
}

func (s *Store) ListCases(ctx context.Context, suiteID id.SuiteID) ([]*testcase.Case, error) {
	var result []*testcase.Case
	err := s.db.NewSelect().Model(&result).Where("suite_id = ?", suiteID.String()).OrderExpr("created_at ASC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list cases: %w", err)
	}
	return result, nil
}

func (s *Store) CountCases(ctx context.Context, suiteID id.SuiteID) (int64, error) {
	count, err := s.db.NewSelect().Model((*testcase.Case)(nil)).Where("suite_id = ?", suiteID.String()).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("sentinel: count cases: %w", err)
	}
	return int64(count), nil
}

func (s *Store) ImportCases(_ context.Context, _ id.SuiteID, _ string, _ []byte) (int64, error) {
	return 0, nil
}

// ──────────────────────────────────────────────────
// Run operations
// ──────────────────────────────────────────────────

func (s *Store) CreateRun(ctx context.Context, run *evalrun.Run) error {
	now := time.Now().UTC()
	run.CreatedAt = now
	run.UpdatedAt = now
	_, err := s.db.NewInsert().Model(run).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create run: %w", err)
	}
	return nil
}

func (s *Store) GetRun(ctx context.Context, runID id.EvalRunID) (*evalrun.Run, error) {
	run := new(evalrun.Run)
	err := s.db.NewSelect().Model(run).Where("id = ?", runID.String()).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sentinel.ErrRunNotFound
		}
		return nil, fmt.Errorf("sentinel: get run: %w", err)
	}
	return run, nil
}

func (s *Store) UpdateRun(ctx context.Context, run *evalrun.Run) error {
	run.UpdatedAt = time.Now().UTC()
	_, err := s.db.NewUpdate().Model(run).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: update run: %w", err)
	}
	return nil
}

func (s *Store) ListRuns(ctx context.Context, filter *evalrun.ListFilter) ([]*evalrun.Run, error) {
	var result []*evalrun.Run
	q := s.db.NewSelect().Model(&result).OrderExpr("created_at DESC")
	if filter != nil {
		if filter.SuiteID.String() != "" {
			q = q.Where("suite_id = ?", filter.SuiteID.String())
		}
		if filter.AppID != "" {
			q = q.Where("app_id = ?", filter.AppID)
		}
		if filter.State != "" {
			q = q.Where("state = ?", string(filter.State))
		}
		if filter.Limit > 0 {
			q = q.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			q = q.Offset(filter.Offset)
		}
	}
	if err := q.Scan(ctx); err != nil {
		return nil, fmt.Errorf("sentinel: list runs: %w", err)
	}
	return result, nil
}

func (s *Store) ListRunsBySuite(ctx context.Context, suiteID id.SuiteID) ([]*evalrun.Run, error) {
	var result []*evalrun.Run
	err := s.db.NewSelect().Model(&result).Where("suite_id = ?", suiteID.String()).OrderExpr("created_at DESC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list runs by suite: %w", err)
	}
	return result, nil
}

// ──────────────────────────────────────────────────
// Result operations
// ──────────────────────────────────────────────────

func (s *Store) CreateResult(ctx context.Context, result *evalrun.Result) error {
	now := time.Now().UTC()
	result.CreatedAt = now
	result.UpdatedAt = now
	_, err := s.db.NewInsert().Model(result).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create result: %w", err)
	}
	return nil
}

func (s *Store) CreateResultBatch(ctx context.Context, results []*evalrun.Result) error {
	now := time.Now().UTC()
	for _, r := range results {
		r.CreatedAt = now
		r.UpdatedAt = now
	}
	_, err := s.db.NewInsert().Model(&results).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create result batch: %w", err)
	}
	return nil
}

func (s *Store) ListResults(ctx context.Context, runID id.EvalRunID) ([]*evalrun.Result, error) {
	var result []*evalrun.Result
	err := s.db.NewSelect().Model(&result).Where("run_id = ?", runID.String()).OrderExpr("created_at ASC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list results: %w", err)
	}
	return result, nil
}

func (s *Store) GetResultStats(ctx context.Context, runID id.EvalRunID) (*evalrun.ResultStats, error) {
	results, err := s.ListResults(ctx, runID)
	if err != nil {
		return nil, err
	}
	stats := &evalrun.ResultStats{DimensionScores: make(map[string]float64)}
	dimCounts := make(map[string]int)
	var totalScore float64
	var totalLatency int64
	for _, r := range results {
		stats.TotalCases++
		totalScore += r.Score
		totalLatency += int64(r.LatencyMs)
		stats.TotalTokens += r.TokensUsed
		stats.TotalCost += r.Cost
		switch r.Status {
		case evalrun.StatusPass:
			stats.Passed++
		case evalrun.StatusFail:
			stats.Failed++
		case evalrun.StatusError:
			stats.Errored++
		}
		for dim, score := range r.DimensionScores {
			stats.DimensionScores[dim] += score
			dimCounts[dim]++
		}
	}
	if stats.TotalCases > 0 {
		stats.PassRate = float64(stats.Passed) / float64(stats.TotalCases)
		stats.AvgScore = totalScore / float64(stats.TotalCases)
		stats.AvgLatencyMs = int(totalLatency / int64(stats.TotalCases))
	}
	for dim, total := range stats.DimensionScores {
		if dimCounts[dim] > 0 {
			stats.DimensionScores[dim] = total / float64(dimCounts[dim])
		}
	}
	return stats, nil
}

// ──────────────────────────────────────────────────
// Baseline operations
// ──────────────────────────────────────────────────

func (s *Store) SaveBaseline(ctx context.Context, b *baseline.Baseline) error {
	b.CreatedAt = time.Now().UTC()
	if b.IsCurrent {
		_, _ = s.db.NewUpdate().Model((*baseline.Baseline)(nil)).Set("is_current = ?", false).Where("suite_id = ?", b.SuiteID.String()).Exec(ctx)
	}
	_, err := s.db.NewInsert().Model(b).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: save baseline: %w", err)
	}
	return nil
}

func (s *Store) GetBaseline(ctx context.Context, baselineID id.BaselineID) (*baseline.Baseline, error) {
	b := new(baseline.Baseline)
	err := s.db.NewSelect().Model(b).Where("id = ?", baselineID.String()).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sentinel.ErrBaselineNotFound
		}
		return nil, fmt.Errorf("sentinel: get baseline: %w", err)
	}
	return b, nil
}

func (s *Store) GetLatestBaseline(ctx context.Context, suiteID id.SuiteID) (*baseline.Baseline, error) {
	b := new(baseline.Baseline)
	err := s.db.NewSelect().Model(b).Where("suite_id = ?", suiteID.String()).Where("is_current = ?", true).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sentinel.ErrBaselineNotFound
		}
		return nil, fmt.Errorf("sentinel: get latest baseline: %w", err)
	}
	return b, nil
}

func (s *Store) ListBaselines(ctx context.Context, suiteID id.SuiteID) ([]*baseline.Baseline, error) {
	var result []*baseline.Baseline
	err := s.db.NewSelect().Model(&result).Where("suite_id = ?", suiteID.String()).OrderExpr("created_at DESC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list baselines: %w", err)
	}
	return result, nil
}

func (s *Store) DeleteBaseline(ctx context.Context, baselineID id.BaselineID) error {
	_, err := s.db.NewDelete().Model((*baseline.Baseline)(nil)).Where("id = ?", baselineID.String()).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: delete baseline: %w", err)
	}
	return nil
}

// ──────────────────────────────────────────────────
// Prompt version operations
// ──────────────────────────────────────────────────

func (s *Store) CreatePromptVersion(ctx context.Context, pv *promptversion.PromptVersion) error {
	pv.CreatedAt = time.Now().UTC()
	_, err := s.db.NewInsert().Model(pv).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create prompt version: %w", err)
	}
	return nil
}

func (s *Store) GetPromptVersion(ctx context.Context, pvID id.PromptVersionID) (*promptversion.PromptVersion, error) {
	pv := new(promptversion.PromptVersion)
	err := s.db.NewSelect().Model(pv).Where("id = ?", pvID.String()).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sentinel.ErrPromptVersionNotFound
		}
		return nil, fmt.Errorf("sentinel: get prompt version: %w", err)
	}
	return pv, nil
}

func (s *Store) ListPromptVersions(ctx context.Context, suiteID id.SuiteID) ([]*promptversion.PromptVersion, error) {
	var result []*promptversion.PromptVersion
	err := s.db.NewSelect().Model(&result).Where("suite_id = ?", suiteID.String()).OrderExpr("version ASC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list prompt versions: %w", err)
	}
	return result, nil
}

func (s *Store) GetCurrentPromptVersion(ctx context.Context, suiteID id.SuiteID) (*promptversion.PromptVersion, error) {
	pv := new(promptversion.PromptVersion)
	err := s.db.NewSelect().Model(pv).Where("suite_id = ?", suiteID.String()).Where("is_current = ?", true).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sentinel.ErrPromptVersionNotFound
		}
		return nil, fmt.Errorf("sentinel: get current prompt version: %w", err)
	}
	return pv, nil
}

func (s *Store) SetCurrentPromptVersion(ctx context.Context, suiteID id.SuiteID, pvID id.PromptVersionID) error {
	_, _ = s.db.NewUpdate().Model((*promptversion.PromptVersion)(nil)).Set("is_current = ?", false).Where("suite_id = ?", suiteID.String()).Exec(ctx)
	_, err := s.db.NewUpdate().Model((*promptversion.PromptVersion)(nil)).Set("is_current = ?", true).Where("id = ?", pvID.String()).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: set current prompt version: %w", err)
	}
	return nil
}
