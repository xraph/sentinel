package engine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/target"
	"github.com/xraph/sentinel/testcase"
)

func runOnce(t *testing.T, e *engine.Engine, suiteID id.SuiteID, scorers ...scorer.Scorer) *engine.RunResult {
	t.Helper()
	res, err := e.RunEval(bg(), &engine.RunConfig{
		SuiteID: suiteID,
		Target:  target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return "hello " + in, nil }),
		Scorers: scorers,
	})
	if err != nil {
		t.Fatalf("RunEval: %v", err)
	}
	return res
}

func TestScorerErrorMakesTheCaseError(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "p", "a")
	res := runOnce(t, e, s.ID, okScorer("good", 1, "skill"), failingScorer("judge"))

	r := res.Results[0]
	if r.Status != evalrun.StatusError {
		t.Fatalf("a scorer error must make the case error, got %s", r.Status)
	}
	if r.Score != 1 {
		t.Fatalf("score is the mean over scorers that returned: %v", r.Score)
	}
	if !strings.Contains(r.Error, "scorer judge") || !strings.Contains(r.Error, "judge timed out") {
		t.Fatalf("error must name the scorer: %q", r.Error)
	}
	if r.DimensionScores["skill"] != 1 {
		t.Fatalf("the good scorer's dimension survives: %v", r.DimensionScores)
	}
	if res.Stats.Passed != 0 || res.Stats.Errored != 1 {
		t.Fatalf("an errored case never counts as a pass: %+v", res.Stats)
	}
}

func TestCaseScorersAreApplied(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "p")
	tc := &testcase.Case{Entity: sentinel.NewEntity(), SuiteID: s.ID, Name: "c", Input: "world", ScenarioType: testcase.ScenarioStandard,
		Scorers: []testcase.ScorerConfig{{Name: "contains", Config: map[string]any{"substring": "hello"}}},
		Tags:    []string{}, Context: map[string]any{}, Metadata: map[string]any{}}
	if err := e.CreateCase(bg(), tc); err != nil {
		t.Fatal(err)
	}
	res := runOnce(t, e, s.ID, okScorer("good", 1, ""))
	if n := len(res.Results[0].ScorerResults); n != 2 {
		t.Fatalf("run scorer plus case scorer: got %d scorer results", n)
	}
	if res.Results[0].Status != evalrun.StatusPass {
		t.Fatalf("both scorers pass: %+v", res.Results[0])
	}
}

func TestUnknownCaseScorerIsVisiblyUnscored(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "p")
	tc := &testcase.Case{Entity: sentinel.NewEntity(), SuiteID: s.ID, Name: "c", Input: "x", ScenarioType: testcase.ScenarioStandard,
		Scorers: []testcase.ScorerConfig{{Name: "nope"}}, Tags: []string{}, Context: map[string]any{}, Metadata: map[string]any{}}
	if err := e.CreateCase(bg(), tc); err != nil {
		t.Fatal(err)
	}
	r := runOnce(t, e, s.ID, okScorer("good", 1, "")).Results[0]
	if r.Status != evalrun.StatusError || !strings.Contains(r.Error, `unknown scorer "nope"`) {
		t.Fatalf("unknown case scorer: status=%s error=%q", r.Status, r.Error)
	}
}

func TestCaseContextIsNotMutated(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "p", "a")
	runOnce(t, e, s.ID, okScorer("good", 1, ""))
	cases, _ := e.ListCases(bg(), s.ID)
	if _, leaked := cases[0].Context["latency_ms"]; leaked {
		t.Fatal("the engine wrote latency_ms into the stored case's context")
	}
}
