package engine

import (
	"context"
	"fmt"

	log "github.com/xraph/go-utils/log"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/plugin"
	"github.com/xraph/sentinel/promptversion"
	"github.com/xraph/sentinel/store"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

// Engine is the central coordinator for the Sentinel evaluation pipeline.
type Engine struct {
	config      sentinel.Config
	logger      log.Logger
	store       store.Store
	extensions  *plugin.Registry
	pendingExts []plugin.Extension
}

// New creates a new Engine with the given options.
func New(opts ...Option) (*Engine, error) {
	e := &Engine{
		config: sentinel.DefaultConfig(),
		logger: log.NewNoopLogger(),
	}
	for _, opt := range opts {
		if err := opt(e); err != nil {
			return nil, fmt.Errorf("sentinel: apply option: %w", err)
		}
	}

	// Wire up the extension registry.
	e.extensions = plugin.NewRegistry(e.logger)
	for _, extension := range e.pendingExts {
		e.extensions.Register(extension)
	}
	e.pendingExts = nil

	return e, nil
}

// Health checks the health of the engine by pinging its store.
func (e *Engine) Health(ctx context.Context) error {
	if e.store != nil {
		return e.store.Ping(ctx)
	}
	return nil
}

// Start initialises the engine. Reserved for future background processes.
func (e *Engine) Start(_ context.Context) error {
	return nil
}

// Stop gracefully shuts down the engine.
func (e *Engine) Stop(ctx context.Context) error {
	if e.extensions != nil {
		e.extensions.EmitShutdown(ctx)
	}
	if e.store != nil {
		return e.store.Close()
	}
	return nil
}

// Store returns the engine's store.
func (e *Engine) Store() store.Store { return e.store }

// Logger returns the engine's logger.
func (e *Engine) Logger() log.Logger { return e.logger }

// Config returns a copy of the engine's configuration.
func (e *Engine) Config() sentinel.Config { return e.config }

// Extensions returns the extension registry.
func (e *Engine) Extensions() *plugin.Registry { return e.extensions }

// ──────────────────────────────────────────────────
// Suite operations
// ──────────────────────────────────────────────────

// CreateSuite creates a new evaluation suite.
func (e *Engine) CreateSuite(ctx context.Context, s *suite.Suite) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	if s.ID.String() == "" {
		s.ID = id.NewSuiteID()
	}
	if s.AppID == "" {
		s.AppID = sentinel.AppFromContext(ctx)
	}
	if s.Model == "" {
		s.Model = e.config.DefaultModel
	}
	return e.store.CreateSuite(ctx, s)
}

// GetSuite retrieves a suite by ID.
func (e *Engine) GetSuite(ctx context.Context, suiteID id.SuiteID) (*suite.Suite, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.GetSuite(ctx, suiteID)
}

// GetSuiteByName retrieves a suite by app and name.
func (e *Engine) GetSuiteByName(ctx context.Context, appID, name string) (*suite.Suite, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.GetSuiteByName(ctx, appID, name)
}

// ListSuites returns suites matching the given filter.
func (e *Engine) ListSuites(ctx context.Context, filter *suite.ListFilter) ([]*suite.Suite, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.ListSuites(ctx, filter)
}

// UpdateSuite updates an existing suite.
func (e *Engine) UpdateSuite(ctx context.Context, s *suite.Suite) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	return e.store.UpdateSuite(ctx, s)
}

// DeleteSuite removes a suite.
func (e *Engine) DeleteSuite(ctx context.Context, suiteID id.SuiteID) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	return e.store.DeleteSuite(ctx, suiteID)
}

// ──────────────────────────────────────────────────
// Test case operations
// ──────────────────────────────────────────────────

// CreateCase creates a new test case.
func (e *Engine) CreateCase(ctx context.Context, tc *testcase.Case) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	if tc.ID.String() == "" {
		tc.ID = id.NewCaseID()
	}
	return e.store.CreateCase(ctx, tc)
}

// CreateCaseBatch creates multiple test cases.
func (e *Engine) CreateCaseBatch(ctx context.Context, cases []*testcase.Case) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	for _, tc := range cases {
		if tc.ID.String() == "" {
			tc.ID = id.NewCaseID()
		}
	}
	return e.store.CreateCaseBatch(ctx, cases)
}

// GetCase retrieves a test case by ID.
func (e *Engine) GetCase(ctx context.Context, caseID id.CaseID) (*testcase.Case, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.GetCase(ctx, caseID)
}

// UpdateCase updates an existing test case.
func (e *Engine) UpdateCase(ctx context.Context, tc *testcase.Case) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	return e.store.UpdateCase(ctx, tc)
}

// DeleteCase removes a test case.
func (e *Engine) DeleteCase(ctx context.Context, caseID id.CaseID) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	return e.store.DeleteCase(ctx, caseID)
}

// ListCases returns all test cases for a suite.
func (e *Engine) ListCases(ctx context.Context, suiteID id.SuiteID) ([]*testcase.Case, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.ListCases(ctx, suiteID)
}

// CountCases returns the number of test cases for a suite.
func (e *Engine) CountCases(ctx context.Context, suiteID id.SuiteID) (int64, error) {
	if e.store == nil {
		return 0, sentinel.ErrNoStore
	}
	return e.store.CountCases(ctx, suiteID)
}

// ImportCases imports test cases from a file format.
func (e *Engine) ImportCases(ctx context.Context, suiteID id.SuiteID, format string, data []byte) (int64, error) {
	if e.store == nil {
		return 0, sentinel.ErrNoStore
	}
	return e.store.ImportCases(ctx, suiteID, format, data)
}

// ──────────────────────────────────────────────────
// Eval run operations
// ──────────────────────────────────────────────────

// GetRun retrieves an evaluation run by ID.
func (e *Engine) GetRun(ctx context.Context, runID id.EvalRunID) (*evalrun.Run, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.GetRun(ctx, runID)
}

// ListRuns returns evaluation runs matching the given filter.
func (e *Engine) ListRuns(ctx context.Context, filter *evalrun.ListFilter) ([]*evalrun.Run, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.ListRuns(ctx, filter)
}

// ListRunsBySuite returns all evaluation runs for a suite.
func (e *Engine) ListRunsBySuite(ctx context.Context, suiteID id.SuiteID) ([]*evalrun.Run, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.ListRunsBySuite(ctx, suiteID)
}

// ListResults returns all results for an evaluation run.
func (e *Engine) ListResults(ctx context.Context, runID id.EvalRunID) ([]*evalrun.Result, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.ListResults(ctx, runID)
}

// GetResultStats returns aggregate statistics for a run.
func (e *Engine) GetResultStats(ctx context.Context, runID id.EvalRunID) (*evalrun.ResultStats, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.GetResultStats(ctx, runID)
}

// ──────────────────────────────────────────────────
// Baseline operations
// ──────────────────────────────────────────────────

// SaveBaseline saves a baseline.
func (e *Engine) SaveBaseline(ctx context.Context, b *baseline.Baseline) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	if b.ID.String() == "" {
		b.ID = id.NewBaselineID()
	}
	if err := e.store.SaveBaseline(ctx, b); err != nil {
		return err
	}
	e.extensions.EmitBaselineSaved(ctx, b.SuiteID, b.ID)
	return nil
}

// GetBaseline retrieves a baseline by ID.
func (e *Engine) GetBaseline(ctx context.Context, baselineID id.BaselineID) (*baseline.Baseline, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.GetBaseline(ctx, baselineID)
}

// GetLatestBaseline retrieves the latest baseline for a suite.
func (e *Engine) GetLatestBaseline(ctx context.Context, suiteID id.SuiteID) (*baseline.Baseline, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.GetLatestBaseline(ctx, suiteID)
}

// ListBaselines returns all baselines for a suite.
func (e *Engine) ListBaselines(ctx context.Context, suiteID id.SuiteID) ([]*baseline.Baseline, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.ListBaselines(ctx, suiteID)
}

// DeleteBaseline removes a baseline.
func (e *Engine) DeleteBaseline(ctx context.Context, baselineID id.BaselineID) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	return e.store.DeleteBaseline(ctx, baselineID)
}

// ──────────────────────────────────────────────────
// Prompt version operations
// ──────────────────────────────────────────────────

// CreatePromptVersion creates a new prompt version.
func (e *Engine) CreatePromptVersion(ctx context.Context, pv *promptversion.PromptVersion) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	if pv.ID.String() == "" {
		pv.ID = id.NewPromptVersionID()
	}
	if err := e.store.CreatePromptVersion(ctx, pv); err != nil {
		return err
	}
	e.extensions.EmitPromptVersionCreated(ctx, pv.SuiteID, pv.ID, pv.Version)
	return nil
}

// GetPromptVersion retrieves a prompt version by ID.
func (e *Engine) GetPromptVersion(ctx context.Context, pvID id.PromptVersionID) (*promptversion.PromptVersion, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.GetPromptVersion(ctx, pvID)
}

// ListPromptVersions returns all prompt versions for a suite.
func (e *Engine) ListPromptVersions(ctx context.Context, suiteID id.SuiteID) ([]*promptversion.PromptVersion, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.ListPromptVersions(ctx, suiteID)
}

// GetCurrentPromptVersion returns the current prompt version for a suite.
func (e *Engine) GetCurrentPromptVersion(ctx context.Context, suiteID id.SuiteID) (*promptversion.PromptVersion, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	return e.store.GetCurrentPromptVersion(ctx, suiteID)
}

// SetCurrentPromptVersion sets the current prompt version for a suite.
func (e *Engine) SetCurrentPromptVersion(ctx context.Context, suiteID id.SuiteID, pvID id.PromptVersionID) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	return e.store.SetCurrentPromptVersion(ctx, suiteID, pvID)
}
