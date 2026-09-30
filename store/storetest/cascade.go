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
	su := mustSuite(t, s)
	tc := mustCase(t, s, su.ID)
	run := mustRun(t, s, su.ID)
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

	cases, err := s.ListCases(bg(), gone.suiteID)
	if err != nil || len(cases) != 0 {
		t.Errorf("cases survived: %d (%v)", len(cases), err)
	}
	if _, err = s.GetRun(bg(), gone.runID); !errors.Is(err, sentinel.ErrRunNotFound) {
		t.Errorf("run survived: %v", err)
	}
	res, err := s.ListResults(bg(), gone.runID)
	if err != nil || len(res) != 0 {
		t.Errorf("results survived: %d (%v)", len(res), err)
	}
	bl, err := s.ListBaselines(bg(), gone.suiteID)
	if err != nil || len(bl) != 0 {
		t.Errorf("baselines survived: %d (%v)", len(bl), err)
	}
	pvs, err := s.ListPromptVersions(bg(), gone.suiteID)
	if err != nil || len(pvs) != 0 {
		t.Errorf("prompt versions survived: %d (%v)", len(pvs), err)
	}

	// The other suite is untouched, checked by identity.
	cases, err = s.ListCases(bg(), kept.suiteID)
	if err != nil || len(cases) != 1 || cases[0].ID.String() != kept.caseID.String() {
		t.Errorf("other suite's case changed: %+v (%v)", cases, err)
	}
	if run, runErr := s.GetRun(bg(), kept.runID); runErr != nil || run.ID.String() != kept.runID.String() {
		t.Errorf("other suite's run changed: %v", runErr)
	}
	res, err = s.ListResults(bg(), kept.runID)
	if err != nil || len(res) != 1 || res[0].ID.String() != kept.resultID.String() {
		t.Errorf("other suite's result changed: %+v (%v)", res, err)
	}
	bl, err = s.ListBaselines(bg(), kept.suiteID)
	if err != nil || len(bl) != 1 {
		t.Errorf("other suite's baseline changed: %d (%v)", len(bl), err)
	}
	pvs, err = s.ListPromptVersions(bg(), kept.suiteID)
	if err != nil || len(pvs) != 1 {
		t.Errorf("other suite's prompt versions changed: %d (%v)", len(pvs), err)
	}
}

// Deleting a suite that does not exist is not an error on any backend: the
// caller wanted it gone and it is.
func testDeleteMissingSuite(t *testing.T, s store.Store) {
	if err := s.DeleteSuite(bg(), id.NewSuiteID()); err != nil {
		t.Fatalf("delete missing suite: want nil, got %v", err)
	}
}
