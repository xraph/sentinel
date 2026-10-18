package engine_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/xraph/forge"
	"go.opentelemetry.io/otel"

	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/target"
)

// newForgeApp builds a Forge app for its metrics registry. forge.New
// installs a global tracer provider and propagator; the test puts the old
// ones back.
func newForgeApp(t *testing.T) forge.App {
	t.Helper()
	prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})
	return forge.New(forge.WithAppName("sentinel-metrics-test"), forge.WithAppLogger(forge.NewNoopLogger()))
}

func scrape(t *testing.T, app forge.App) string {
	t.Helper()
	p, ok := app.Metrics().(interface{ PrometheusHandler() http.Handler })
	if !ok {
		t.Fatalf("app metrics %T have no Prometheus handler", app.Metrics())
	}
	rr := httptest.NewRecorder()
	p.PrometheusHandler().ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/_/metrics", nil))
	body, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// sample is one line of a Prometheus scrape.
type sample struct {
	labels map[string]string
	value  float64
}

// samples returns every line of the scrape for exactly the series name.
func samples(t *testing.T, body, name string) []sample {
	t.Helper()
	var out []sample
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		var series, value string
		if i := strings.LastIndexByte(line, ' '); i > 0 {
			series, value = line[:i], line[i+1:]
		} else {
			continue
		}
		labels := map[string]string{}
		if i := strings.IndexByte(series, '{'); i >= 0 {
			for _, pair := range strings.Split(strings.TrimSuffix(series[i+1:], "}"), ",") {
				if k, v, ok := strings.Cut(pair, "="); ok {
					labels[k] = strings.Trim(v, `"`)
				}
			}
			series = series[:i]
		}
		if series != name {
			continue
		}
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			t.Fatalf("%s: bad value %q", name, value)
		}
		out = append(out, sample{labels: labels, value: v})
	}
	return out
}

// valueOf returns the value of the series with exactly these labels.
func valueOf(t *testing.T, body, name string, labels map[string]string) float64 {
	t.Helper()
	for _, s := range samples(t, body, name) {
		if equalLabels(s.labels, labels) {
			return s.value
		}
	}
	t.Fatalf("no %s%v on the scrape:\n%s", name, labels, grepLines(body, "sentinel"))
	return 0
}

func equalLabels(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range b {
		if a[k] != v {
			return false
		}
	}
	return true
}

func grepLines(body, sub string) string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, sub) && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// labelKeys lists the label keys every series of name carries, failing if
// two series disagree.
func labelKeys(t *testing.T, body, name string) []string {
	t.Helper()
	var keys []string
	for i, s := range samples(t, body, name) {
		var ks []string
		for k := range s.labels {
			if k != "le" {
				ks = append(ks, k)
			}
		}
		sort.Strings(ks)
		if i == 0 {
			keys = ks
			continue
		}
		if strings.Join(ks, ",") != strings.Join(keys, ",") {
			t.Fatalf("%s: series disagree on label keys: %v and %v", name, keys, ks)
		}
	}
	return keys
}

func TestRunMetricsLandOnTheForgeScrape(t *testing.T) {
	app := newForgeApp(t)
	e := newEngine(t, engine.WithMetrics(app.Metrics()))
	s := seedSuite(t, e, "p", "a", "boom")

	// One case answers and fails exact (nothing is expected), one case's
	// target errors. The run completes.
	_, err := e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID, Target: target.NewLLMTarget(fakeLLM{tokens: 7}, "m", "", 0),
		Scorers: []scorer.Scorer{okScorer("my-judge", 1, ""), &scorer.ExactScorer{}}})
	if err != nil {
		t.Fatalf("RunEval: %v", err)
	}
	body := scrape(t, app)

	if strings.Contains(body, "\nsentinel_") {
		t.Fatal("sentinel metrics must scrape with the forge_ prefix")
	}

	for name, want := range map[string][]string{
		"forge_sentinel_run_duration_seconds_count":         {"mode", "state"},
		"forge_sentinel_cases_evaluated_total":              {"status"},
		"forge_sentinel_target_call_duration_seconds_count": {"result", "target_kind"},
		"forge_sentinel_target_tokens_total":                {"target_kind"},
		"forge_sentinel_scorer_duration_seconds_count":      {"result", "scorer"},
	} {
		if got := labelKeys(t, body, name); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s label keys = %v, want %v", name, got, want)
		}
	}

	checks := []struct {
		name   string
		labels map[string]string
		want   float64
	}{
		{"forge_sentinel_run_duration_seconds_count", map[string]string{"mode": "sync", "state": "completed"}, 1},
		{"forge_sentinel_cases_evaluated_total", map[string]string{"status": "fail"}, 1},
		{"forge_sentinel_cases_evaluated_total", map[string]string{"status": "error"}, 1},
		{"forge_sentinel_target_call_duration_seconds_count", map[string]string{"target_kind": "llm", "result": "ok"}, 1},
		{"forge_sentinel_target_call_duration_seconds_count", map[string]string{"target_kind": "llm", "result": "error"}, 1},
		{"forge_sentinel_target_tokens_total", map[string]string{"target_kind": "llm"}, 7},
		{"forge_sentinel_scorer_duration_seconds_count", map[string]string{"scorer": "custom", "result": "ok"}, 1},
		{"forge_sentinel_scorer_duration_seconds_count", map[string]string{"scorer": "exact", "result": "ok"}, 1},
		{"forge_sentinel_runs_in_flight", map[string]string{}, 0},
	}
	for _, c := range checks {
		if got := valueOf(t, body, c.name, c.labels); got != c.want {
			t.Errorf("%s%v = %v, want %v", c.name, c.labels, got, c.want)
		}
	}
	if strings.Contains(body, "my-judge") {
		t.Error("a custom scorer's name must collapse to custom")
	}

	// Another run adds observations, not series.
	if _, err = e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID, Target: target.NewLLMTarget(fakeLLM{tokens: 7}, "m", "", 0),
		Scorers: []scorer.Scorer{okScorer("my-judge", 1, ""), &scorer.ExactScorer{}}}); err != nil {
		t.Fatal(err)
	}
	again := scrape(t, app)
	if n := len(samples(t, again, "forge_sentinel_target_call_duration_seconds_count")); n != 2 {
		t.Fatalf("a repeat run added target series: %d", n)
	}
	if got := valueOf(t, again, "forge_sentinel_run_duration_seconds_count", map[string]string{"mode": "sync", "state": "completed"}); got != 2 {
		t.Fatalf("run count after two runs = %v", got)
	}
}

func TestRunsInFlightCountsRunningRuns(t *testing.T) {
	app := newForgeApp(t)
	release := make(chan struct{})
	entered := make(chan string, 4)
	e := newEngine(t, engine.WithMetrics(app.Metrics()), engine.WithConfig(configWith(1)))
	s := seedSuite(t, e, "p", "a")

	done := runEvalAsync(bg(), e, &engine.RunConfig{SuiteID: s.ID, Target: gated(release, entered, nil, nil),
		Scorers: []scorer.Scorer{okScorer("good", 1, "")}})
	<-entered
	if got := valueOf(t, scrape(t, app), "forge_sentinel_runs_in_flight", map[string]string{}); got != 1 {
		t.Fatalf("in flight while a case runs = %v, want 1", got)
	}
	release <- struct{}{}
	if got := <-done; got.err != nil {
		t.Fatal(got.err)
	}
	if got := valueOf(t, scrape(t, app), "forge_sentinel_runs_in_flight", map[string]string{}); got != 0 {
		t.Fatalf("in flight after the run = %v, want 0", got)
	}
}

func TestRegressionsAreCounted(t *testing.T) {
	app := newForgeApp(t)
	e := newEngine(t, engine.WithMetrics(app.Metrics()))
	s := seedSuite(t, e, "p", "a")
	echo := target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return in, nil })
	good, err := e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID, Target: echo, Scorers: []scorer.Scorer{okScorer("s", 1, "")}})
	if err != nil {
		t.Fatal(err)
	}
	saveBaseline(t, e, good)
	if _, err = e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID, Target: echo, Scorers: []scorer.Scorer{okScorer("s", 0, "")}}); err != nil {
		t.Fatal(err)
	}
	body := scrape(t, app)
	if got := valueOf(t, body, "forge_sentinel_regressions_detected_total", map[string]string{}); got != 1 {
		t.Fatalf("regressions = %v, want 1", got)
	}
	if got := valueOf(t, body, "forge_sentinel_target_call_duration_seconds_count", map[string]string{"target_kind": "func", "result": "ok"}); got != 2 {
		t.Fatalf("func target calls = %v, want 2", got)
	}
}

// An engine without WithMetrics records nothing and doesn't panic.
func TestNoMetricsIsANoOp(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "p", "a")
	runOnce(t, e, s.ID, okScorer("s", 1, ""))
}
