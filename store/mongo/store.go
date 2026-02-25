package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/xraph/grove"
	"github.com/xraph/grove/drivers/mongodriver"
	"github.com/xraph/grove/drivers/mongodriver/mongomigrate"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/promptversion"
	"github.com/xraph/sentinel/store"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

const (
	colSuites         = "sentinel_suites"
	colCases          = "sentinel_cases"
	colRuns           = "sentinel_runs"
	colResults        = "sentinel_results"
	colBaselines      = "sentinel_baselines"
	colPromptVersions = "sentinel_prompt_versions"
)

// Compile-time interface check.
var _ store.Store = (*Store)(nil)

// Store is a MongoDB implementation of the composite Sentinel store.
type Store struct {
	db  *grove.DB
	mdb *mongodriver.MongoDB
}

// New creates a new MongoDB store.
func New(db *grove.DB) *Store {
	return &Store{
		db:  db,
		mdb: mongodriver.Unwrap(db),
	}
}

// Migrate runs grove migrations for the Sentinel schema (creates indexes).
func (s *Store) Migrate(ctx context.Context) error {
	exec := mongomigrate.New(s.mdb)
	for _, m := range Migrations.Migrations() {
		if m.Up != nil {
			if err := m.Up(ctx, exec); err != nil {
				return fmt.Errorf("sentinel: %w: %s: %w", sentinel.ErrMigrationFailed, m.Name, err)
			}
		}
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
	_, err := s.mdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create suite: %w", err)
	}
	return nil
}

func (s *Store) GetSuite(ctx context.Context, suiteID id.SuiteID) (*suite.Suite, error) {
	m := new(suiteModel)
	err := s.mdb.NewFind(m).Filter(bson.M{"_id": suiteID.String()}).Scan(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, sentinel.ErrSuiteNotFound
		}
		return nil, fmt.Errorf("sentinel: get suite: %w", err)
	}
	return suiteFromModel(m), nil
}

func (s *Store) GetSuiteByName(ctx context.Context, appID, name string) (*suite.Suite, error) {
	m := new(suiteModel)
	err := s.mdb.NewFind(m).
		Filter(bson.M{"app_id": appID}).
		Filter(bson.M{"name": name}).
		Scan(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, sentinel.ErrSuiteNotFound
		}
		return nil, fmt.Errorf("sentinel: get suite by name: %w", err)
	}
	return suiteFromModel(m), nil
}

func (s *Store) UpdateSuite(ctx context.Context, su *suite.Suite) error {
	su.UpdatedAt = time.Now().UTC()
	m := suiteToModel(su)
	res, err := s.mdb.NewUpdate(m).Filter(bson.M{"_id": m.ID}).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: update suite: %w", err)
	}
	if n := res.MatchedCount(); n == 0 {
		return sentinel.ErrSuiteNotFound
	}
	return nil
}

func (s *Store) DeleteSuite(ctx context.Context, suiteID id.SuiteID) error {
	_, err := s.mdb.NewDelete((*suiteModel)(nil)).
		Filter(bson.M{"_id": suiteID.String()}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: delete suite: %w", err)
	}
	return nil
}

func (s *Store) ListSuites(ctx context.Context, filter *suite.ListFilter) ([]*suite.Suite, error) {
	var models []suiteModel
	q := s.mdb.NewFind(&models).Sort(bson.D{{Key: "created_at", Value: 1}})
	if filter != nil {
		if filter.AppID != "" {
			q = q.Filter(bson.M{"app_id": filter.AppID})
		}
		if filter.Limit > 0 {
			q = q.Limit(int64(filter.Limit))
		}
		if filter.Offset > 0 {
			q = q.Skip(int64(filter.Offset))
		}
	}
	if err := q.Scan(ctx); err != nil {
		return nil, fmt.Errorf("sentinel: list suites: %w", err)
	}
	result := make([]*suite.Suite, len(models))
	for i := range models {
		result[i] = suiteFromModel(&models[i])
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
	_, err := s.mdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create case: %w", err)
	}
	return nil
}

func (s *Store) CreateCaseBatch(ctx context.Context, cases []*testcase.Case) error {
	if len(cases) == 0 {
		return nil
	}
	now := time.Now().UTC()
	models := make([]caseModel, len(cases))
	for i, tc := range cases {
		tc.CreatedAt = now
		tc.UpdatedAt = now
		models[i] = *caseToModel(tc)
	}
	_, err := s.mdb.NewInsert(&models).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create case batch: %w", err)
	}
	return nil
}

func (s *Store) GetCase(ctx context.Context, caseID id.CaseID) (*testcase.Case, error) {
	m := new(caseModel)
	err := s.mdb.NewFind(m).Filter(bson.M{"_id": caseID.String()}).Scan(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, sentinel.ErrCaseNotFound
		}
		return nil, fmt.Errorf("sentinel: get case: %w", err)
	}
	return caseFromModel(m), nil
}

func (s *Store) UpdateCase(ctx context.Context, tc *testcase.Case) error {
	tc.UpdatedAt = time.Now().UTC()
	m := caseToModel(tc)
	res, err := s.mdb.NewUpdate(m).Filter(bson.M{"_id": m.ID}).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: update case: %w", err)
	}
	if n := res.MatchedCount(); n == 0 {
		return sentinel.ErrCaseNotFound
	}
	return nil
}

func (s *Store) DeleteCase(ctx context.Context, caseID id.CaseID) error {
	_, err := s.mdb.NewDelete((*caseModel)(nil)).
		Filter(bson.M{"_id": caseID.String()}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: delete case: %w", err)
	}
	return nil
}

func (s *Store) ListCases(ctx context.Context, suiteID id.SuiteID) ([]*testcase.Case, error) {
	var models []caseModel
	err := s.mdb.NewFind(&models).
		Filter(bson.M{"suite_id": suiteID.String()}).
		Sort(bson.D{{Key: "created_at", Value: 1}}).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list cases: %w", err)
	}
	result := make([]*testcase.Case, len(models))
	for i := range models {
		result[i] = caseFromModel(&models[i])
	}
	return result, nil
}

func (s *Store) CountCases(ctx context.Context, suiteID id.SuiteID) (int64, error) {
	count, err := s.mdb.NewFind((*caseModel)(nil)).
		Filter(bson.M{"suite_id": suiteID.String()}).
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
	_, err := s.mdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create run: %w", err)
	}
	return nil
}

func (s *Store) GetRun(ctx context.Context, runID id.EvalRunID) (*evalrun.Run, error) {
	m := new(runModel)
	err := s.mdb.NewFind(m).Filter(bson.M{"_id": runID.String()}).Scan(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, sentinel.ErrRunNotFound
		}
		return nil, fmt.Errorf("sentinel: get run: %w", err)
	}
	return runFromModel(m), nil
}

func (s *Store) UpdateRun(ctx context.Context, run *evalrun.Run) error {
	run.UpdatedAt = time.Now().UTC()
	m := runToModel(run)
	res, err := s.mdb.NewUpdate(m).Filter(bson.M{"_id": m.ID}).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: update run: %w", err)
	}
	if n := res.MatchedCount(); n == 0 {
		return sentinel.ErrRunNotFound
	}
	return nil
}

func (s *Store) ListRuns(ctx context.Context, filter *evalrun.ListFilter) ([]*evalrun.Run, error) {
	var models []runModel
	q := s.mdb.NewFind(&models).Sort(bson.D{{Key: "created_at", Value: -1}})
	if filter != nil {
		if filter.SuiteID.String() != "" {
			q = q.Filter(bson.M{"suite_id": filter.SuiteID.String()})
		}
		if filter.AppID != "" {
			q = q.Filter(bson.M{"app_id": filter.AppID})
		}
		if filter.State != "" {
			q = q.Filter(bson.M{"state": string(filter.State)})
		}
		if filter.Limit > 0 {
			q = q.Limit(int64(filter.Limit))
		}
		if filter.Offset > 0 {
			q = q.Skip(int64(filter.Offset))
		}
	}
	if err := q.Scan(ctx); err != nil {
		return nil, fmt.Errorf("sentinel: list runs: %w", err)
	}
	result := make([]*evalrun.Run, len(models))
	for i := range models {
		result[i] = runFromModel(&models[i])
	}
	return result, nil
}

func (s *Store) ListRunsBySuite(ctx context.Context, suiteID id.SuiteID) ([]*evalrun.Run, error) {
	var models []runModel
	err := s.mdb.NewFind(&models).
		Filter(bson.M{"suite_id": suiteID.String()}).
		Sort(bson.D{{Key: "created_at", Value: -1}}).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list runs by suite: %w", err)
	}
	result := make([]*evalrun.Run, len(models))
	for i := range models {
		result[i] = runFromModel(&models[i])
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
	_, err := s.mdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create result: %w", err)
	}
	return nil
}

func (s *Store) CreateResultBatch(ctx context.Context, results []*evalrun.Result) error {
	if len(results) == 0 {
		return nil
	}
	now := time.Now().UTC()
	models := make([]resultModel, len(results))
	for i, r := range results {
		r.CreatedAt = now
		r.UpdatedAt = now
		models[i] = *resultToModel(r)
	}
	_, err := s.mdb.NewInsert(&models).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create result batch: %w", err)
	}
	return nil
}

func (s *Store) ListResults(ctx context.Context, runID id.EvalRunID) ([]*evalrun.Result, error) {
	var models []resultModel
	err := s.mdb.NewFind(&models).
		Filter(bson.M{"run_id": runID.String()}).
		Sort(bson.D{{Key: "created_at", Value: 1}}).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list results: %w", err)
	}
	result := make([]*evalrun.Result, len(models))
	for i := range models {
		result[i] = resultFromModel(&models[i])
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
		coll := s.mdb.Collection(colBaselines)
		_, err := coll.UpdateMany(ctx,
			bson.M{"suite_id": b.SuiteID.String()},
			bson.M{"$set": bson.M{"is_current": false}},
		)
		if err != nil {
			return fmt.Errorf("sentinel: reset baselines: %w", err)
		}
	}
	m := baselineToModel(b)
	_, err := s.mdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: save baseline: %w", err)
	}
	return nil
}

func (s *Store) GetBaseline(ctx context.Context, baselineID id.BaselineID) (*baseline.Baseline, error) {
	m := new(baselineModel)
	err := s.mdb.NewFind(m).Filter(bson.M{"_id": baselineID.String()}).Scan(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, sentinel.ErrBaselineNotFound
		}
		return nil, fmt.Errorf("sentinel: get baseline: %w", err)
	}
	return baselineFromModel(m), nil
}

func (s *Store) GetLatestBaseline(ctx context.Context, suiteID id.SuiteID) (*baseline.Baseline, error) {
	m := new(baselineModel)
	err := s.mdb.NewFind(m).
		Filter(bson.M{"suite_id": suiteID.String()}).
		Filter(bson.M{"is_current": true}).
		Scan(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, sentinel.ErrBaselineNotFound
		}
		return nil, fmt.Errorf("sentinel: get latest baseline: %w", err)
	}
	return baselineFromModel(m), nil
}

func (s *Store) ListBaselines(ctx context.Context, suiteID id.SuiteID) ([]*baseline.Baseline, error) {
	var models []baselineModel
	err := s.mdb.NewFind(&models).
		Filter(bson.M{"suite_id": suiteID.String()}).
		Sort(bson.D{{Key: "created_at", Value: -1}}).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("sentinel: list baselines: %w", err)
	}
	result := make([]*baseline.Baseline, len(models))
	for i := range models {
		result[i] = baselineFromModel(&models[i])
	}
	return result, nil
}

func (s *Store) DeleteBaseline(ctx context.Context, baselineID id.BaselineID) error {
	_, err := s.mdb.NewDelete((*baselineModel)(nil)).
		Filter(bson.M{"_id": baselineID.String()}).
		Exec(ctx)
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
	_, err := s.mdb.NewInsert(m).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sentinel: create prompt version: %w", err)
	}
	return nil
}

func (s *Store) GetPromptVersion(ctx context.Context, pvID id.PromptVersionID) (*promptversion.PromptVersion, error) {
	m := new(promptVersionModel)
	err := s.mdb.NewFind(m).Filter(bson.M{"_id": pvID.String()}).Scan(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, sentinel.ErrPromptVersionNotFound
		}
		return nil, fmt.Errorf("sentinel: get prompt version: %w", err)
	}
	return promptVersionFromModel(m), nil
}

func (s *Store) ListPromptVersions(ctx context.Context, suiteID id.SuiteID) ([]*promptversion.PromptVersion, error) {
	var models []promptVersionModel
	err := s.mdb.NewFind(&models).
		Filter(bson.M{"suite_id": suiteID.String()}).
		Sort(bson.D{{Key: "version", Value: 1}}).
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
	err := s.mdb.NewFind(m).
		Filter(bson.M{"suite_id": suiteID.String()}).
		Filter(bson.M{"is_current": true}).
		Scan(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, sentinel.ErrPromptVersionNotFound
		}
		return nil, fmt.Errorf("sentinel: get current prompt version: %w", err)
	}
	return promptVersionFromModel(m), nil
}

func (s *Store) SetCurrentPromptVersion(ctx context.Context, suiteID id.SuiteID, pvID id.PromptVersionID) error {
	// Reset all prompt versions for this suite to not current.
	coll := s.mdb.Collection(colPromptVersions)
	_, err := coll.UpdateMany(ctx,
		bson.M{"suite_id": suiteID.String()},
		bson.M{"$set": bson.M{"is_current": false}},
	)
	if err != nil {
		return fmt.Errorf("sentinel: reset prompt versions: %w", err)
	}
	_, err = coll.UpdateOne(ctx,
		bson.M{"_id": pvID.String()},
		bson.M{"$set": bson.M{"is_current": true}},
	)
	if err != nil {
		return fmt.Errorf("sentinel: set current prompt version: %w", err)
	}
	return nil
}

// isNotFound checks whether an error indicates no documents were found.
func isNotFound(err error) bool {
	return errors.Is(err, mongo.ErrNoDocuments) ||
		errors.Is(err, grove.ErrNoRows)
}
