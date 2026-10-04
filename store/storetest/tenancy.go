package storetest

import (
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/store"
	"github.com/xraph/sentinel/suite"
)

// This records a dangerous fact; it does not endorse it. On every backend
// an empty AppID in a list filter matches EVERY app rather than none. The
// dashboard contract therefore never sends one: it resolves an app or
// refuses. If a backend ever changes this, the assertion fails and whoever
// changed it should read sentinel/extension/contract/tenancy.go.
func testEmptyAppFilterMatchesEverything(t *testing.T, s store.Store) {
	a := mustSuite(t, s)
	b := &suite.Suite{Entity: sentinel.NewEntity(), ID: id.NewSuiteID(), Name: "other-app", AppID: "app_b", Model: "m", Metadata: map[string]any{}}
	if err := s.CreateSuite(bg(), b); err != nil {
		t.Fatalf("create app_b suite: %v", err)
	}
	ra := mustRun(t, s, a.ID)
	rb := &evalrun.Run{Entity: sentinel.NewEntity(), ID: id.NewEvalRunID(), SuiteID: b.ID, Model: "m", AppID: "app_b",
		State: evalrun.StateRunning, Config: map[string]any{}, DimensionScores: map[string]float64{}}
	if err := s.CreateRun(bg(), rb); err != nil {
		t.Fatalf("create app_b run: %v", err)
	}

	suites, err := s.ListSuites(bg(), &suite.ListFilter{})
	if err != nil {
		t.Fatalf("list suites: %v", err)
	}
	if !hasSuite(suites, a.ID) || !hasSuite(suites, b.ID) {
		t.Fatalf("empty AppID is recorded as matching every app; got %d suites", len(suites))
	}
	runs, err := s.ListRuns(bg(), &evalrun.ListFilter{})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if !hasRun(runs, ra.ID) || !hasRun(runs, rb.ID) {
		t.Fatalf("empty AppID is recorded as matching every app; got %d runs", len(runs))
	}

	// And the scoped filter isolates, checked by identity.
	scoped, err := s.ListSuites(bg(), &suite.ListFilter{AppID: "app_b"})
	if err != nil || len(scoped) != 1 || scoped[0].ID.String() != b.ID.String() {
		t.Fatalf("AppID app_b should list exactly the app_b suite: %v %v", scoped, err)
	}
}

func hasSuite(list []*suite.Suite, want id.SuiteID) bool {
	for _, s := range list {
		if s.ID.String() == want.String() {
			return true
		}
	}
	return false
}

func hasRun(list []*evalrun.Run, want id.EvalRunID) bool {
	for _, r := range list {
		if r.ID.String() == want.String() {
			return true
		}
	}
	return false
}
