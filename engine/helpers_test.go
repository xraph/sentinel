package engine_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/store/memory"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

func bg() context.Context { return context.Background() }

func newEngine(t *testing.T, opts ...engine.Option) *engine.Engine {
	t.Helper()
	st := memory.New()
	e, err := engine.New(append([]engine.Option{engine.WithStore(st)}, opts...)...)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Stop(context.Background()) })
	return e
}

// seedSuite creates a suite with the given system prompt and one case per
// input, named after the input.
func seedSuite(t *testing.T, e *engine.Engine, prompt string, inputs ...string) *suite.Suite {
	t.Helper()
	s := &suite.Suite{Entity: sentinel.NewEntity(), Name: "suite-" + id.NewSuiteID().String(), SystemPrompt: prompt, Metadata: map[string]any{}}
	if err := e.CreateSuite(bg(), s); err != nil {
		t.Fatalf("create suite: %v", err)
	}
	for _, in := range inputs {
		tc := &testcase.Case{Entity: sentinel.NewEntity(), SuiteID: s.ID, Name: in, Input: in, ScenarioType: testcase.ScenarioStandard,
			Scorers: []testcase.ScorerConfig{}, Tags: []string{}, Context: map[string]any{}, Metadata: map[string]any{}}
		if err := e.CreateCase(bg(), tc); err != nil {
			t.Fatalf("create case: %v", err)
		}
	}
	return s
}

func okScorer(name string, score float64, dim string) scorer.Scorer {
	return scorer.FromFunc(name, func(context.Context, *scorer.Input) (*scorer.Output, error) {
		return &scorer.Output{Score: score, Passed: score >= 0.7, Reason: "ok", Dimension: dim}, nil
	})
}

func failingScorer(name string) scorer.Scorer {
	return scorer.FromFunc(name, func(context.Context, *scorer.Input) (*scorer.Output, error) {
		return nil, errors.New("judge timed out")
	})
}

// waitFor polls cond every 10ms for up to five seconds.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
