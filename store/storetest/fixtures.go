package storetest

import (
	"context"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/store"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

func bg() context.Context { return context.Background() }

// fixtureAppID is the app every fixture belongs to.
const fixtureAppID = "app_a"

// The fixtures populate every map and slice with an empty value, so a test
// that is not about nil handling never trips over it.

func mustSuite(t *testing.T, s store.Store) *suite.Suite {
	t.Helper()
	su := &suite.Suite{
		Entity:   sentinel.NewEntity(),
		ID:       id.NewSuiteID(),
		Name:     "suite-" + id.NewSuiteID().String(),
		AppID:    fixtureAppID,
		Model:    "test-model",
		Metadata: map[string]any{},
	}
	if err := s.CreateSuite(bg(), su); err != nil {
		t.Fatalf("create suite: %v", err)
	}
	return su
}

func mustCase(t *testing.T, s store.Store, suiteID id.SuiteID) *testcase.Case {
	t.Helper()
	tc := &testcase.Case{
		Entity:       sentinel.NewEntity(),
		ID:           id.NewCaseID(),
		SuiteID:      suiteID,
		Name:         "case-" + id.NewCaseID().String(),
		Input:        "hello",
		ScenarioType: testcase.ScenarioStandard,
		Scorers:      []testcase.ScorerConfig{},
		Tags:         []string{},
		Context:      map[string]any{},
		Metadata:     map[string]any{},
	}
	if err := s.CreateCase(bg(), tc); err != nil {
		t.Fatalf("create case: %v", err)
	}
	return tc
}

func mustRun(t *testing.T, s store.Store, suiteID id.SuiteID) *evalrun.Run {
	t.Helper()
	r := &evalrun.Run{
		Entity:          sentinel.NewEntity(),
		ID:              id.NewEvalRunID(),
		SuiteID:         suiteID,
		Model:           "test-model",
		TotalCases:      3,
		AppID:           fixtureAppID,
		Config:          map[string]any{},
		State:           evalrun.StateRunning,
		DimensionScores: map[string]float64{},
	}
	if err := s.CreateRun(bg(), r); err != nil {
		t.Fatalf("create run: %v", err)
	}
	return r
}

func mustResult(t *testing.T, s store.Store, runID id.EvalRunID, caseID id.CaseID, status evalrun.ResultStatus, score float64) *evalrun.Result {
	t.Helper()
	r := &evalrun.Result{
		Entity:          sentinel.NewEntity(),
		ID:              id.NewEvalResultID(),
		RunID:           runID,
		CaseID:          caseID,
		CaseName:        "case",
		Status:          status,
		Score:           score,
		ScorerResults:   []evalrun.ScorerResult{},
		DimensionScores: map[string]float64{},
	}
	if err := s.CreateResult(bg(), r); err != nil {
		t.Fatalf("create result: %v", err)
	}
	return r
}
