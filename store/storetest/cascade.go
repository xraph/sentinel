package storetest

import (
	"errors"
	"testing"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/store"
)

type suiteRows struct {
	suiteID  id.SuiteID
	caseID   id.CaseID
	runID    id.EvalRunID
	resultID id.EvalResultID
}

func seedSuiteRows(t *testing.T, s store.Store) suiteRows {
	t.Helper()
	su := mustSuite(t, s, "app_a")
	tc := mustCase(t, s, su.ID)
	run := mustRun(t, s, su.ID, "app_a")
	res := mustResult(t, s, run.ID, tc.ID, evalrun.StatusPass, 1)
	if err := s.SaveBaseline(bg(), &baseline.Baseline{ID: id.NewBaselineID(), SuiteID: su.ID, RunID: run.ID, Name: "b",
		Results: []baseline.Result{}, DimensionScores: map[string]float64{}, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("save baseline: %v", err)
	}
	mustPV(t, s, su.ID, 1)
	return suiteRows{su.ID, tc.ID, run.ID, res.ID}
}

func testDeleteSuiteCascades(t *testing.T, s store.Store) {
	gone := seedSuiteRows(t, s)
	kept := seedSuiteRows(t, s)

	if err := s.DeleteSuite(bg(), gone.suiteID); err != nil {
		t.Fatalf("delete suite: %v", err)
	}

	if cases, _ := s.ListCases(bg(), gone.suiteID); len(cases) != 0 {
		t.Errorf("cases survived: %d", len(cases))
	}
	if _, err := s.GetRun(bg(), gone.runID); !errors.Is(err, sentinel.ErrRunNotFound) {
		t.Errorf("run survived: %v", err)
	}
	if res, _ := s.ListResults(bg(), gone.runID); len(res) != 0 {
		t.Errorf("results survived: %d", len(res))
	}
	if bl, _ := s.ListBaselines(bg(), gone.suiteID); len(bl) != 0 {
		t.Errorf("baselines survived: %d", len(bl))
	}
	if pvs, _ := s.ListPromptVersions(bg(), gone.suiteID); len(pvs) != 0 {
		t.Errorf("prompt versions survived: %d", len(pvs))
	}

	// The other suite is untouched, checked by identity.
	cases, _ := s.ListCases(bg(), kept.suiteID)
	if len(cases) != 1 || cases[0].ID.String() != kept.caseID.String() {
		t.Errorf("other suite's case changed: %+v", cases)
	}
	if run, err := s.GetRun(bg(), kept.runID); err != nil || run.ID.String() != kept.runID.String() {
		t.Errorf("other suite's run changed: %v", err)
	}
	if res, _ := s.ListResults(bg(), kept.runID); len(res) != 1 || res[0].ID.String() != kept.resultID.String() {
		t.Errorf("other suite's result changed: %+v", res)
	}
	if bl, _ := s.ListBaselines(bg(), kept.suiteID); len(bl) != 1 {
		t.Errorf("other suite's baseline changed: %d", len(bl))
	}
	if pvs, _ := s.ListPromptVersions(bg(), kept.suiteID); len(pvs) != 1 {
		t.Errorf("other suite's prompt versions changed: %d", len(pvs))
	}
}
