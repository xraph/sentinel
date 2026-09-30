package extension

import (
	"testing"
	"time"
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
