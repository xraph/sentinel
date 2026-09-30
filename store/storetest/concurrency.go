package storetest

import (
	"fmt"
	"sync"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/store"
)

// testConcurrentResultWrites is what the runner does: several goroutines
// storing one result each for the same run at once. Every write must land.
func testConcurrentResultWrites(t *testing.T, s store.Store) {
	const workers, perWorker = 8, 6
	su := mustSuite(t, s)
	tc := mustCase(t, s, su.ID)
	run := mustRun(t, s, su.ID)

	var wg sync.WaitGroup
	errs := make(chan error, workers*perWorker)
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range perWorker {
				r := &evalrun.Result{
					Entity:          sentinel.NewEntity(),
					ID:              id.NewEvalResultID(),
					RunID:           run.ID,
					CaseID:          tc.ID,
					CaseName:        fmt.Sprintf("case-%d-%d", w, i),
					Status:          evalrun.StatusPass,
					Score:           1,
					ScorerResults:   []evalrun.ScorerResult{},
					DimensionScores: map[string]float64{},
				}
				if err := s.CreateResult(bg(), r); err != nil {
					errs <- fmt.Errorf("worker %d result %d: %w", w, i, err)
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	failed := 0
	for err := range errs {
		failed++
		if failed <= 3 {
			t.Errorf("concurrent create result: %v", err)
		}
	}
	if failed > 0 {
		t.Errorf("%d of %d concurrent writes failed", failed, workers*perWorker)
	}

	got, err := s.ListResults(bg(), run.ID)
	if err != nil {
		t.Fatalf("list results: %v", err)
	}
	if len(got) != workers*perWorker {
		t.Fatalf("stored %d results, want %d", len(got), workers*perWorker)
	}
}
