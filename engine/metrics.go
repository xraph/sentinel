package engine

import (
	"sync"
	"time"

	"github.com/xraph/forge"
	gometrics "github.com/xraph/go-utils/metrics"
)

// Metric names. A Forge app exports them with its namespace in front, so
// sentinel_run_duration_seconds scrapes as
// forge_sentinel_run_duration_seconds.
const (
	MetricRunDuration    = "sentinel_run_duration_seconds"
	MetricRunsInFlight   = "sentinel_runs_in_flight"
	MetricCasesEvaluated = "sentinel_cases_evaluated_total"
	MetricTargetDuration = "sentinel_target_call_duration_seconds"
	MetricTargetTokens   = "sentinel_target_tokens_total"
	MetricScorerDuration = "sentinel_scorer_duration_seconds"
	MetricRegressions    = "sentinel_regressions_detected_total"
)

// Buckets, in seconds. A run takes from seconds to hours; a target call is
// a model or an agent answering; a scorer is either a string check that
// takes microseconds or an LLM judge that takes seconds.
var (
	runDurationBuckets    = []float64{1, 5, 15, 30, 60, 120, 300, 600, 1200, 1800, 3600, 7200}
	targetDurationBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30, 60, 120, 300}
	scorerDurationBuckets = []float64{0.0001, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2.5, 5, 10, 30, 60}
)

// runMetrics holds every instrument the engine records. A nil *runMetrics
// records nothing, which is what an engine built without WithMetrics has.
type runMetrics struct {
	runDuration    *forge.Vec[forge.Histogram] // mode, state
	runsInFlight   forge.Gauge
	casesEvaluated *forge.Vec[forge.Counter]   // status
	targetDuration *forge.Vec[forge.Histogram] // target_kind, result
	targetTokens   *forge.Vec[forge.Counter]   // target_kind
	scorerDuration *forge.Vec[forge.Histogram] // scorer, result
	regressions    forge.Counter

	// inFlight is the number of runs this process is evaluating. The gauge
	// is set from it under mu, so two runs finishing at once can't leave it
	// showing a stale value.
	mu       sync.Mutex
	inFlight int
}

func newRunMetrics(m gometrics.Metrics) *runMetrics {
	if m == nil {
		return nil
	}
	return &runMetrics{
		runDuration: forge.HistogramVec(m, MetricRunDuration, forge.HistogramOpts{
			Buckets: runDurationBuckets, Unit: "s",
			Help: "Time an evaluation run took, from its first case to its final state.",
		}, "mode", "state"),
		runsInFlight:   m.Gauge(MetricRunsInFlight, gometrics.WithDescription("Evaluation runs this process is evaluating now.")),
		casesEvaluated: forge.CounterVec(m, MetricCasesEvaluated, "status"),
		targetDuration: forge.HistogramVec(m, MetricTargetDuration, forge.HistogramOpts{
			Buckets: targetDurationBuckets, Unit: "s",
			Help: "Time the system under test took to answer one case.",
		}, "target_kind", "result"),
		targetTokens: forge.CounterVec(m, MetricTargetTokens, "target_kind"),
		scorerDuration: forge.HistogramVec(m, MetricScorerDuration, forge.HistogramOpts{
			Buckets: scorerDurationBuckets, Unit: "s",
			Help: "Time one scorer took to score one case.",
		}, "scorer", "result"),
		regressions: m.Counter(MetricRegressions, gometrics.WithDescription("Completed runs that regressed against their suite's baseline.")),
	}
}

func (m *runMetrics) runStarted() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inFlight++
	m.runsInFlight.Set(float64(m.inFlight))
}

func (m *runMetrics) runFinished(mode, state string, d time.Duration) {
	if m == nil {
		return
	}
	m.runDuration.With(mode, state).Observe(d.Seconds())
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inFlight--
	m.runsInFlight.Set(float64(m.inFlight))
}

func (m *runMetrics) caseEvaluated(status string) {
	if m == nil {
		return
	}
	m.casesEvaluated.With(status).Inc()
}

func (m *runMetrics) targetCalled(kind, result string, d time.Duration, tokens int) {
	if m == nil {
		return
	}
	m.targetDuration.With(kind, result).Observe(d.Seconds())
	if tokens > 0 {
		m.targetTokens.With(kind).Add(float64(tokens))
	}
}

func (m *runMetrics) scored(kind, result string, d time.Duration) {
	if m == nil {
		return
	}
	m.scorerDuration.With(kind, result).Observe(d.Seconds())
}

func (m *runMetrics) regressionDetected() {
	if m == nil {
		return
	}
	m.regressions.Inc()
}
