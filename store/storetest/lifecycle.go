package storetest

import (
	"errors"
	"testing"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/store"
)

func finalStats() *evalrun.ResultStats {
	return &evalrun.ResultStats{TotalCases: 3, Passed: 2, Failed: 1, PassRate: 2.0 / 3, AvgScore: 0.75, AvgLatencyMs: 40, TotalTokens: 300, TotalCost: 0.25,
		DimensionScores: map[string]float64{"skill": 0.5}}
}

func testCancelRun(t *testing.T, s store.Store) {
	su := mustSuite(t, s)
	run := mustRun(t, s, su.ID)

	ok, err := s.CancelRun(bg(), run.ID, time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("cancel running run: ok=%v err=%v", ok, err)
	}
	got, err := s.GetRun(bg(), run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.State != evalrun.StateCancelled || got.CompletedAt == nil {
		t.Fatalf("after cancel: state=%s completedAt=%v", got.State, got.CompletedAt)
	}

	ok, err = s.CancelRun(bg(), run.ID, time.Now().UTC())
	if err != nil || ok {
		t.Fatalf("cancelling a cancelled run must report false without error: ok=%v err=%v", ok, err)
	}
	if _, err := s.CancelRun(bg(), id.NewEvalRunID(), time.Now().UTC()); !errors.Is(err, sentinel.ErrRunNotFound) {
		t.Fatalf("cancel missing run: want ErrRunNotFound, got %v", err)
	}
}

func testFinalizeRun(t *testing.T, s store.Store) {
	su := mustSuite(t, s)
	run := mustRun(t, s, su.ID)

	state, err := s.FinalizeRun(bg(), run.ID, &evalrun.Finalization{Stats: finalStats(), State: evalrun.StateCompleted, CompletedAt: time.Now().UTC()})
	if err != nil || state != evalrun.StateCompleted {
		t.Fatalf("finalize: state=%s err=%v", state, err)
	}
	got, err := s.GetRun(bg(), run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.State != evalrun.StateCompleted || got.Passed != 2 || got.Failed != 1 || got.TotalTokens != 300 || !near(got.AvgScore, 0.75) || !near(got.DimensionScores["skill"], 0.5) || got.CompletedAt == nil {
		t.Fatalf("finalized run: %+v", got)
	}
	if got.TotalCases != 3 {
		t.Fatalf("finalize must not touch total_cases: %d", got.TotalCases)
	}

	if _, err := s.FinalizeRun(bg(), id.NewEvalRunID(), &evalrun.Finalization{Stats: finalStats(), State: evalrun.StateCompleted, CompletedAt: time.Now().UTC()}); !errors.Is(err, sentinel.ErrRunNotFound) {
		t.Fatalf("finalize missing run: want ErrRunNotFound, got %v", err)
	}
}

// Review focus 2: a cancel that lands after the last case but before the
// finalize. The run stays cancelled, and the counters still arrive.
func testFinalizeKeepsCancel(t *testing.T, s store.Store) {
	su := mustSuite(t, s)
	run := mustRun(t, s, su.ID)
	if ok, err := s.CancelRun(bg(), run.ID, time.Now().UTC()); err != nil || !ok {
		t.Fatalf("cancel: %v", err)
	}
	state, err := s.FinalizeRun(bg(), run.ID, &evalrun.Finalization{Stats: finalStats(), State: evalrun.StateCompleted, CompletedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if state != evalrun.StateCancelled {
		t.Fatalf("finalize overwrote a cancel: %s", state)
	}
	got, err := s.GetRun(bg(), run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.State != evalrun.StateCancelled || got.Passed != 2 {
		t.Fatalf("cancelled run should keep its state and gain counters: %+v", got)
	}
}

func testFinalizeRecordsFailure(t *testing.T, s store.Store) {
	su := mustSuite(t, s)
	run := mustRun(t, s, su.ID)
	state, err := s.FinalizeRun(bg(), run.ID, &evalrun.Finalization{Stats: finalStats(), State: evalrun.StateFailed, Error: "1 of 3 results could not be stored", CompletedAt: time.Now().UTC()})
	if err != nil || state != evalrun.StateFailed {
		t.Fatalf("finalize failed: state=%s err=%v", state, err)
	}
	got, err := s.GetRun(bg(), run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.Error != "1 of 3 results could not be stored" {
		t.Fatalf("error not recorded: %q", got.Error)
	}
}
