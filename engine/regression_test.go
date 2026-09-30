package engine_test

import (
	"context"
	"sync"
	"testing"

	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/target"
)

type regressionRecorder struct {
	mu        sync.Mutex
	baselines []string
	deltas    []float64
}

func (r *regressionRecorder) Name() string { return "recorder" }
func (r *regressionRecorder) OnRegressionDetected(_ context.Context, _ id.SuiteID, baselineID id.BaselineID, delta float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.baselines = append(r.baselines, baselineID.String())
	r.deltas = append(r.deltas, delta)
	return nil
}

func TestRegressionHookFiresAgainstTheCurrentBaseline(t *testing.T) {
	rec := &regressionRecorder{}
	e, _ := newEngine(t, engine.WithExtension(rec))
	s := seedSuite(t, e, "p", "a", "b")
	echo := target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return in, nil })

	good, err := e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID, Target: echo, Scorers: []scorer.Scorer{okScorer("s", 1, "")}})
	if err != nil {
		t.Fatal(err)
	}
	b := &baseline.Baseline{SuiteID: s.ID, RunID: good.Run.ID, Name: "good", PassRate: good.Run.PassRate, AvgScore: good.Run.AvgScore,
		DimensionScores: map[string]float64{}, IsCurrent: true}
	for _, r := range good.Results {
		b.Results = append(b.Results, baseline.Result{CaseID: r.CaseID, CaseName: r.CaseName, Score: r.Score, Status: string(r.Status)})
	}
	if err := e.SaveBaseline(bg(), b); err != nil {
		t.Fatal(err)
	}

	if _, err := e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID, Target: echo, Scorers: []scorer.Scorer{okScorer("s", 0.5, "")}}); err != nil {
		t.Fatal(err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.baselines) != 1 || rec.baselines[0] != b.ID.String() {
		t.Fatalf("hook should fire once against the current baseline: %v", rec.baselines)
	}
	if rec.deltas[0] != -1 {
		t.Fatalf("worst delta is the pass rate falling from 1 to 0: %v", rec.deltas[0])
	}
}

func TestNoBaselineNoHook(t *testing.T) {
	rec := &regressionRecorder{}
	e, _ := newEngine(t, engine.WithExtension(rec))
	s := seedSuite(t, e, "p", "a")
	echo := target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return in, nil })
	if _, err := e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID, Target: echo, Scorers: []scorer.Scorer{okScorer("s", 0, "")}}); err != nil {
		t.Fatal(err)
	}
	if len(rec.baselines) != 0 {
		t.Fatal("with no baseline there is nothing to regress against")
	}
}
