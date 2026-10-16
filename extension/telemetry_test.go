package extension

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xraph/forge"
	"go.opentelemetry.io/otel"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/store/memory"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/target"
	"github.com/xraph/sentinel/testcase"
)

// Register hands the engine Forge's metrics, so a run shows up on the
// app's scrape with no option from the application.
func TestRegisterWiresForgeMetricsIntoTheEngine(t *testing.T) {
	prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})
	app := forge.New(forge.WithAppName("sentinel-ext-telemetry-test"), forge.WithAppLogger(forge.NewNoopLogger()))

	ext := New(WithStore(memory.New()), WithDisableRoutes())
	if err := ext.Register(app); err != nil {
		t.Fatalf("Register: %v", err)
	}
	eng := ext.Engine()
	t.Cleanup(func() { _ = eng.Stop(context.Background()) })

	ctx := context.Background()
	s := &suite.Suite{Entity: sentinel.NewEntity(), Name: "telemetry", Metadata: map[string]any{}}
	if err := eng.CreateSuite(ctx, s); err != nil {
		t.Fatal(err)
	}
	tc := &testcase.Case{Entity: sentinel.NewEntity(), SuiteID: s.ID, Name: "a", Input: "a", ScenarioType: testcase.ScenarioStandard,
		Scorers: []testcase.ScorerConfig{}, Tags: []string{}, Context: map[string]any{}, Metadata: map[string]any{}}
	if err := eng.CreateCase(ctx, tc); err != nil {
		t.Fatal(err)
	}
	echo := target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return in, nil })
	if _, err := eng.RunEval(ctx, &engine.RunConfig{SuiteID: s.ID, Target: echo, Scorers: []scorer.Scorer{&scorer.ExactScorer{}}}); err != nil {
		t.Fatalf("RunEval: %v", err)
	}

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
	for _, want := range []string{
		`forge_sentinel_run_duration_seconds_count{mode="sync",state="completed"} 1`,
		`forge_sentinel_cases_evaluated_total{status="fail"} 1`,
		`forge_sentinel_target_call_duration_seconds_count{result="ok",target_kind="func"} 1`,
		`forge_sentinel_scorer_duration_seconds_count{result="ok",scorer="exact"} 1`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("scrape is missing %s", want)
		}
	}
}
