package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/xraph/grove"
	"github.com/xraph/grove/drivers/sqlitedriver"
	"github.com/xraph/grove/migrate"

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
	db  *grove.DB
	sdb *sqlitedriver.SqliteDB
}

// New creates a new SQLite store backed by grove ORM.
func New(db *grove.DB) *Store {
	return &Store{
		db:  db,
		sdb: sqlitedriver.Unwrap(db),
	}
}

// Migrate runs programmatic migrations via the grove orchestrator.
func (s *Store) Migrate(ctx context.Context) error {
	executor, err := migrate.NewExecutorFor(s.sdb)
	if err != nil {
		return fmt.Errorf("sentinel/sqlite: create migration executor: %w", err)
	}
	orch := migrate.NewOrchestrator(executor, Migrations)
	if _, err := orch.Migrate(ctx); err != nil {
		return fmt.Errorf("sentinel/sqlite: migration failed: %w", err)
	}
	return nil
}

// Ping verifies the database connection.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.Ping(ctx)
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
	m := suiteToModel(su)
	_, err := s.sdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create suite: %w", err)
	}
	return nil
}

func (s *Store) GetSuite(ctx context.Context, suiteID id.SuiteID) (*suite.Suite, error) {
	m := new(suiteModel)
	err := s.sdb.NewSelect(m).Where("id = ?", suiteID.String()).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sentinel.ErrSuiteNotFound
		}
		return nil, fmt.Errorf("sentinel: get suite: %w", err)
	}
	return suiteFromModel(m)
}

func (s *Store) GetSuiteByName(ctx context.Context, appID, name string) (*suite.Suite, error) {
	m := new(suiteModel)
	err := s.sdb.NewSelect(m).
		Where("app_id = ?", appID).
		Where("name = ?", name).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sentinel.ErrSuiteNotFound
		}
		return nil, fmt.Errorf("sentinel: get suite by name: %w", err)
	}
	return suiteFromModel(m)
}

func (s *Store) UpdateSuite(ctx context.Context, su *suite.Suite) error {
	su.UpdatedAt = time.Now().UTC()
	m := suiteToModel(su)
	_, err := s.sdb.NewUpdate(m).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: update suite: %w", err)
	}
	return nil
}

func (s *Store) DeleteSuite(ctx context.Context, suiteID id.SuiteID) error {
	_, err := s.sdb.NewDelete((*suiteModel)(nil)).Where("id = ?", suiteID.String()).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: delete suite: %w", err)
	}
	return nil
}

func (s *Store) ListSuites(ctx context.Context, filter *suite.ListFilter) ([]*suite.Suite, error) {
	var models []suiteModel
	q := s.sdb.NewSelect(&models).OrderExpr("created_at ASC")
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
	result := make([]*suite.Suite, len(models))
	for i := range models {
		su, convErr := suiteFromModel(&models[i])
		if convErr != nil {
			return nil, convErr
		}
		result[i] = su
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
	m := caseToModel(tc)
	_, err := s.sdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create case: %w", err)
	}
	return nil
}

func (s *Store) CreateCaseBatch(ctx context.Context, cases []*testcase.Case) error {
	now := time.Now().UTC()
	models := make([]caseModel, len(cases))
	for i, tc := range cases {
		tc.CreatedAt = now
		tc.UpdatedAt = now
		models[i] = *caseToModel(tc)
	}
	_, err := s.sdb.NewInsert(&models).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create case batch: %w", err)
	}
	return nil
}

func (s *Store) GetCase(ctx context.Context, caseID id.CaseID) (*testcase.Case, error) {
	m := new(caseModel)
	err := s.sdb.NewSelect(m).Where("id = ?", caseID.String()).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sentinel.ErrCaseNotFound
		}
		return nil, fmt.Errorf("sentinel: get case: %w", err)
	}
	return caseFromModel(m)
}

func (s *Store) UpdateCase(ctx context.Context, tc *testcase.Case) error {
	tc.UpdatedAt = time.Now().UTC()
	m := caseToModel(tc)
	_, err := s.sdb.NewUpdate(m).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: update case: %w", err)
	}
	return nil
}

func (s *Store) DeleteCase(ctx context.Context, caseID id.CaseID) error {
	_, err := s.sdb.NewDelete((*caseModel)(nil)).Where("id = ?", caseID.String()).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: delete case: %w", err)
	}
	return nil
}

func (s *Store) ListCases(ctx context.Context, suiteID id.SuiteID) ([]*testcase.Case, error) {
	var models []caseModel
	err := s.sdb.NewSelect(&models).
		Where("suite_id = ?", suiteID.String()).
		OrderExpr("created_at ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list cases: %w", err)
	}
	result := make([]*testcase.Case, len(models))
	for i := range models {
		tc, convErr := caseFromModel(&models[i])
		if convErr != nil {
			return nil, convErr
		}
		result[i] = tc
	}
	return result, nil
}

func (s *Store) CountCases(ctx context.Context, suiteID id.SuiteID) (int64, error) {
	count, err := s.sdb.NewSelect((*caseModel)(nil)).
		Where("suite_id = ?", suiteID.String()).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("sentinel: count cases: %w", err)
	}
	return count, nil
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
	m := runToModel(run)
	_, err := s.sdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create run: %w", err)
	}
	return nil
}

func (s *Store) GetRun(ctx context.Context, runID id.EvalRunID) (*evalrun.Run, error) {
	m := new(runModel)
	err := s.sdb.NewSelect(m).Where("id = ?", runID.String()).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sentinel.ErrRunNotFound
		}
		return nil, fmt.Errorf("sentinel: get run: %w", err)
	}
	return runFromModel(m)
}

func (s *Store) UpdateRun(ctx context.Context, run *evalrun.Run) error {
	run.UpdatedAt = time.Now().UTC()
	m := runToModel(run)
	_, err := s.sdb.NewUpdate(m).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: update run: %w", err)
	}
	return nil
}

func (s *Store) ListRuns(ctx context.Context, filter *evalrun.ListFilter) ([]*evalrun.Run, error) {
	var models []runModel
	q := s.sdb.NewSelect(&models).OrderExpr("created_at DESC")
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
	result := make([]*evalrun.Run, len(models))
	for i := range models {
		r, convErr := runFromModel(&models[i])
		if convErr != nil {
			return nil, convErr
		}
		result[i] = r
	}
	return result, nil
}

func (s *Store) ListRunsBySuite(ctx context.Context, suiteID id.SuiteID) ([]*evalrun.Run, error) {
	var models []runModel
	err := s.sdb.NewSelect(&models).
		Where("suite_id = ?", suiteID.String()).
		OrderExpr("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list runs by suite: %w", err)
	}
	result := make([]*evalrun.Run, len(models))
	for i := range models {
		r, convErr := runFromModel(&models[i])
		if convErr != nil {
			return nil, convErr
		}
		result[i] = r
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
	m := resultToModel(result)
	_, err := s.sdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create result: %w", err)
	}
	return nil
}

func (s *Store) CreateResultBatch(ctx context.Context, results []*evalrun.Result) error {
	now := time.Now().UTC()
	models := make([]resultModel, len(results))
	for i, r := range results {
		r.CreatedAt = now
		r.UpdatedAt = now
		models[i] = *resultToModel(r)
	}
	_, err := s.sdb.NewInsert(&models).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create result batch: %w", err)
	}
	return nil
}

func (s *Store) ListResults(ctx context.Context, runID id.EvalRunID) ([]*evalrun.Result, error) {
	var models []resultModel
	err := s.sdb.NewSelect(&models).
		Where("run_id = ?", runID.String()).
		OrderExpr("created_at ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list results: %w", err)
	}
	result := make([]*evalrun.Result, len(models))
	for i := range models {
		r, convErr := resultFromModel(&models[i])
		if convErr != nil {
			return nil, convErr
		}
		result[i] = r
	}
	return result, nil
}

func (s *Store) GetResultStats(ctx context.Context, runID id.EvalRunID) (*evalrun.ResultStats, error) {
	results, err := s.ListResults(ctx, runID)
	if err != nil {
		return nil, err
	}
	stats := &evalrun.ResultStats{
		DimensionScores: make(map[string]float64),
	}
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
		_, err := s.sdb.NewUpdate((*baselineModel)(nil)).
			Set("is_current = ?", false).
			Where("suite_id = ?", b.SuiteID.String()).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("sentinel: reset baselines: %w", err)
		}
	}
	m := baselineToModel(b)
	_, err := s.sdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: save baseline: %w", err)
	}
	return nil
}

func (s *Store) GetBaseline(ctx context.Context, baselineID id.BaselineID) (*baseline.Baseline, error) {
	m := new(baselineModel)
	err := s.sdb.NewSelect(m).Where("id = ?", baselineID.String()).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sentinel.ErrBaselineNotFound
		}
		return nil, fmt.Errorf("sentinel: get baseline: %w", err)
	}
	return baselineFromModel(m)
}

func (s *Store) GetLatestBaseline(ctx context.Context, suiteID id.SuiteID) (*baseline.Baseline, error) {
	m := new(baselineModel)
	err := s.sdb.NewSelect(m).
		Where("suite_id = ?", suiteID.String()).
		Where("is_current = ?", true).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sentinel.ErrBaselineNotFound
		}
		return nil, fmt.Errorf("sentinel: get latest baseline: %w", err)
	}
	return baselineFromModel(m)
}

func (s *Store) ListBaselines(ctx context.Context, suiteID id.SuiteID) ([]*baseline.Baseline, error) {
	var models []baselineModel
	err := s.sdb.NewSelect(&models).
		Where("suite_id = ?", suiteID.String()).
		OrderExpr("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list baselines: %w", err)
	}
	result := make([]*baseline.Baseline, len(models))
	for i := range models {
		b, convErr := baselineFromModel(&models[i])
		if convErr != nil {
			return nil, convErr
		}
		result[i] = b
	}
	return result, nil
}

func (s *Store) DeleteBaseline(ctx context.Context, baselineID id.BaselineID) error {
	_, err := s.sdb.NewDelete((*baselineModel)(nil)).Where("id = ?", baselineID.String()).Exec(ctx)
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
	m := promptVersionToModel(pv)
	_, err := s.sdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create prompt version: %w", err)
	}
	return nil
}

func (s *Store) GetPromptVersion(ctx context.Context, pvID id.PromptVersionID) (*promptversion.PromptVersion, error) {
	m := new(promptVersionModel)
	err := s.sdb.NewSelect(m).Where("id = ?", pvID.String()).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sentinel.ErrPromptVersionNotFound
		}
		return nil, fmt.Errorf("sentinel: get prompt version: %w", err)
	}
	return promptVersionFromModel(m), nil
}

func (s *Store) ListPromptVersions(ctx context.Context, suiteID id.SuiteID) ([]*promptversion.PromptVersion, error) {
	var models []promptVersionModel
	err := s.sdb.NewSelect(&models).
		Where("suite_id = ?", suiteID.String()).
		OrderExpr("version ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list prompt versions: %w", err)
	}
	result := make([]*promptversion.PromptVersion, len(models))
	for i := range models {
		result[i] = promptVersionFromModel(&models[i])
	}
	return result, nil
}

func (s *Store) GetCurrentPromptVersion(ctx context.Context, suiteID id.SuiteID) (*promptversion.PromptVersion, error) {
	m := new(promptVersionModel)
	err := s.sdb.NewSelect(m).
		Where("suite_id = ?", suiteID.String()).
		Where("is_current = ?", true).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sentinel.ErrPromptVersionNotFound
		}
		return nil, fmt.Errorf("sentinel: get current prompt version: %w", err)
	}
	return promptVersionFromModel(m), nil
}

func (s *Store) SetCurrentPromptVersion(ctx context.Context, suiteID id.SuiteID, pvID id.PromptVersionID) error {
	_, err := s.sdb.NewUpdate((*promptVersionModel)(nil)).
		Set("is_current = ?", false).
		Where("suite_id = ?", suiteID.String()).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: reset prompt versions: %w", err)
	}
	_, err = s.sdb.NewUpdate((*promptVersionModel)(nil)).
		Set("is_current = ?", true).
		Where("id = ?", pvID.String()).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: set current prompt version: %w", err)
	}
	return nil
}
