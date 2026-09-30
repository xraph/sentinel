package sentinel

import "time"

// Config holds configuration for the Sentinel engine.
type Config struct {
	// DefaultModel is the LLM model name used when a suite does
	// not specify one. Special values "smart" and "fast" are resolved
	// by the target adapter.
	DefaultModel string

	// Temperature is the default LLM temperature for evaluations. A run
	// uses its suite's temperature when that is non-zero and this value
	// otherwise, so a suite temperature of 0 inherits it.
	Temperature float64

	// PassThreshold is the minimum score (0-1) for a case to pass.
	PassThreshold float64

	// RegressionThreshold is how far a metric may fall below the current
	// baseline, as an absolute amount on its 0 to 1 scale, before a run
	// counts as regressed. One value covers pass rate, average score, each
	// dimension and each case, and lower is always worse.
	RegressionThreshold float64

	// Concurrency is the maximum number of cases evaluated in parallel.
	Concurrency int

	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	ShutdownTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		DefaultModel:        "smart",
		Temperature:         0,
		PassThreshold:       0.7,
		RegressionThreshold: 0.05,
		Concurrency:         4,
		ShutdownTimeout:     30 * time.Second,
	}
}
