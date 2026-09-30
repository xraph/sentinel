package storetest

import (
	"testing"

	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/store"
)

// A value read from the store is the caller's to change. Mutating it must
// not change what the store holds, which is trivially true for the SQL and
// document backends and was false for memory.
func testReturnedValuesAreIndependent(t *testing.T, s store.Store) {
	su := mustSuite(t, s)
	run := mustRun(t, s, su.ID)

	run.State = evalrun.StateFailed // the caller's own value after CreateRun
	got, err := s.GetRun(bg(), run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.State != evalrun.StateRunning {
		t.Fatalf("mutating the value passed to CreateRun changed the store: %s", got.State)
	}

	got.State = evalrun.StateCancelled
	again, err := s.GetRun(bg(), run.ID)
	if err != nil {
		t.Fatalf("get run again: %v", err)
	}
	if again.State != evalrun.StateRunning {
		t.Fatalf("mutating a returned run changed the store: %s", again.State)
	}

	listed, err := s.ListRuns(bg(), &evalrun.ListFilter{SuiteID: su.ID})
	if err != nil || len(listed) != 1 {
		t.Fatalf("list runs: %v (%d)", err, len(listed))
	}
	listed[0].Model = "changed"
	again, err = s.GetRun(bg(), run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if again.Model != "test-model" {
		t.Fatalf("mutating a listed run changed the store: %s", again.Model)
	}
}
