package extension

import (
	"context"
	"testing"
	"time"

	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/target"
)

func TestEngineReceivesConfiguredValues(t *testing.T) {
	e := New(WithConfig(Config{PassThreshold: 0.9, Concurrency: 2, DefaultModel: "fast", RegressionThreshold: 0.1}))
	e.config = e.mergeWithDefaults(e.config)

	eng, err := e.newEngine(nil)
	if err != nil {
		t.Fatalf("newEngine: %v", err)
	}
	got := eng.Config()
	if got.PassThreshold != 0.9 || got.Concurrency != 2 || got.DefaultModel != "fast" || got.RegressionThreshold != 0.1 {
		t.Fatalf("engine did not receive the configured values: %+v", got)
	}
}

func TestRegressionThresholdDefaults(t *testing.T) {
	e := New()
	e.config = e.mergeWithDefaults(e.config)
	if got := engineConfig(e.config); got.RegressionThreshold != 0.05 || got.PassThreshold != 0.7 || got.ShutdownTimeout != 30*time.Second {
		t.Fatalf("defaults: %+v", got)
	}
}

func TestYAMLRegressionThresholdWins(t *testing.T) {
	e := New(WithConfig(Config{RegressionThreshold: 0.2}))
	merged := e.mergeConfigurations(Config{RegressionThreshold: 0.1}, e.config)
	if merged.RegressionThreshold != 0.1 {
		t.Fatalf("yaml should win: %v", merged.RegressionThreshold)
	}
	merged = e.mergeConfigurations(Config{}, e.config)
	if merged.RegressionThreshold != 0.2 {
		t.Fatalf("programmatic should fill a gap: %v", merged.RegressionThreshold)
	}
}

func TestExtensionRegistersTargetsAndScorers(t *testing.T) {
	echo := target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return in, nil })
	e := New(
		WithTarget("support-bot", "the production support agent", echo),
		WithScorer(scorer.Descriptor{Name: "judge", UsesLLM: true}, func(map[string]any) (scorer.Scorer, error) {
			return scorer.FromFunc("judge", func(context.Context, *scorer.Input) (*scorer.Output, error) { return &scorer.Output{Score: 1}, nil }), nil
		}),
	)
	e.config = e.mergeWithDefaults(e.config)
	eng, err := e.newEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	if rt, ok := eng.Target("support-bot"); !ok || rt.Description != "the production support agent" {
		t.Fatalf("target not registered: %+v", rt)
	}
	if !eng.Scorers().Has("judge") {
		t.Fatal("scorer not registered")
	}
}

func TestDashboardAppIDMerges(t *testing.T) {
	e := New(WithConfig(Config{DashboardAppID: "from-code"}))
	if got := e.mergeConfigurations(Config{DashboardAppID: "from-yaml"}, e.config).DashboardAppID; got != "from-yaml" {
		t.Fatalf("yaml should win: %q", got)
	}
	if got := e.mergeConfigurations(Config{}, e.config).DashboardAppID; got != "from-code" {
		t.Fatalf("programmatic should fill a gap: %q", got)
	}
	if got := e.mergeWithDefaults(e.config).DashboardAppID; got != "from-code" {
		t.Fatalf("defaults path: %q", got)
	}
	if got := New().mergeWithDefaults(Config{}).DashboardAppID; got != "" {
		t.Fatalf("there is no default app: %q", got)
	}
}
