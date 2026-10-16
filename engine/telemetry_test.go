package engine_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/comparison"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/target"
	"github.com/xraph/sentinel/testcase"
)

// spanEnded signals each time a span with the given name ends, so a test
// can wait for a StartRun goroutine without polling.
type spanEnded struct {
	name string
	ch   chan sdktrace.ReadOnlySpan
}

func (p *spanEnded) OnStart(context.Context, sdktrace.ReadWriteSpan) {}
func (p *spanEnded) Shutdown(context.Context) error                  { return nil }
func (p *spanEnded) ForceFlush(context.Context) error                { return nil }
func (p *spanEnded) OnEnd(s sdktrace.ReadOnlySpan) {
	if s.Name() == p.name {
		select {
		case p.ch <- s:
		default:
		}
	}
}

// recordSpans installs a recording tracer provider as the global one for
// the test, and puts the previous one back afterwards.
func recordSpans(t *testing.T, extra ...sdktrace.SpanProcessor) (*tracetest.SpanRecorder, trace.Tracer) {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	opts := make([]sdktrace.TracerProviderOption, 0, 1+len(extra))
	opts = append(opts, sdktrace.WithSpanProcessor(rec))
	for _, p := range extra {
		opts = append(opts, sdktrace.WithSpanProcessor(p))
	}
	tp := sdktrace.NewTracerProvider(opts...)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})
	return rec, tp.Tracer("test")
}

// sentinelSpans are the ended spans sentinel started, by name.
func sentinelSpans(rec *tracetest.SpanRecorder) map[string][]sdktrace.ReadOnlySpan {
	out := map[string][]sdktrace.ReadOnlySpan{}
	for _, s := range rec.Ended() {
		if s.InstrumentationScope().Name == engine.InstrumentationName {
			out[s.Name()] = append(out[s.Name()], s)
		}
	}
	return out
}

func attrOf(s sdktrace.ReadOnlySpan, key string) (attribute.Value, bool) {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func wantAttr(t *testing.T, s sdktrace.ReadOnlySpan, key string, want any) {
	t.Helper()
	v, ok := attrOf(s, key)
	if !ok {
		t.Errorf("%s: no %s attribute", s.Name(), key)
		return
	}
	if got := v.AsInterface(); got != want {
		t.Errorf("%s: %s = %v (%T), want %v (%T)", s.Name(), key, got, got, want, want)
	}
}

type fakeLLM struct{ tokens int }

func (f fakeLLM) Complete(_ context.Context, _, _, input string, _ float64) (*target.LLMResponse, error) {
	if input == "boom" {
		return nil, errors.New("provider unavailable")
	}
	return &target.LLMResponse{Output: input, Tokens: f.tokens}, nil
}

func TestRunEvalTracesTheRunUnderTheCaller(t *testing.T) {
	rec, tr := recordSpans(t)
	e := newEngine(t)
	s := seedSuite(t, e, "p", "a", "boom")

	ctx, caller := tr.Start(bg(), "caller")
	res, err := e.RunEval(ctx, &engine.RunConfig{SuiteID: s.ID, Target: target.NewLLMTarget(fakeLLM{tokens: 5}, "m", "", 0),
		Scorers: []scorer.Scorer{okScorer("my-judge", 1, ""), &scorer.ExactScorer{}}})
	caller.End()
	if err != nil {
		t.Fatalf("RunEval: %v", err)
	}

	got := sentinelSpans(rec)
	runs := got["evaluate run"]
	if len(runs) != 1 {
		t.Fatalf("want one run span, got %d", len(runs))
	}
	run := runs[0]
	if run.SpanKind() != trace.SpanKindInternal {
		t.Errorf("run span kind = %v", run.SpanKind())
	}
	if run.Parent().SpanID() != caller.SpanContext().SpanID() {
		t.Error("a RunEval run must be a child of the caller's span")
	}
	wantAttr(t, run, "sentinel.run.id", res.Run.ID.String())
	wantAttr(t, run, "sentinel.suite.id", s.ID.String())
	wantAttr(t, run, "sentinel.run.mode", "sync")
	wantAttr(t, run, "sentinel.run.state", "completed")
	wantAttr(t, run, "sentinel.target.kind", "llm")
	wantAttr(t, run, "sentinel.run.cases", int64(2))
	wantAttr(t, run, "sentinel.run.scorers", int64(2))
	wantAttr(t, run, "sentinel.run.errored", int64(1))
	if run.Status().Code == codes.Error {
		t.Error("a completed run is not an error, even with an errored case")
	}

	cases := got["evaluate case"]
	if len(cases) != 2 {
		t.Fatalf("want two case spans, got %d", len(cases))
	}
	caseByID := map[trace.SpanID]sdktrace.ReadOnlySpan{}
	for _, c := range cases {
		if c.Parent().SpanID() != run.SpanContext().SpanID() {
			t.Errorf("case span is not a child of the run span")
		}
		caseByID[c.SpanContext().SpanID()] = c
		status, _ := attrOf(c, "sentinel.case.status")
		switch status.AsString() {
		case "error":
			if c.Status().Code != codes.Error || c.Status().Description != "target error" {
				t.Errorf("errored case status = %+v", c.Status())
			}
		case "fail":
		default:
			t.Errorf("case status = %q", status.AsString())
		}
	}

	calls := got["call target"]
	if len(calls) != 2 {
		t.Fatalf("want two target spans, got %d", len(calls))
	}
	var sawOK, sawErr bool
	for _, c := range calls {
		if _, ok := caseByID[c.Parent().SpanID()]; !ok {
			t.Error("target span is not a child of a case span")
		}
		wantAttr(t, c, "sentinel.target.kind", "llm")
		result, _ := attrOf(c, "sentinel.target.result")
		switch result.AsString() {
		case "ok":
			sawOK = true
			wantAttr(t, c, "sentinel.target.tokens", int64(5))
		case "error":
			sawErr = true
			if c.Status().Code != codes.Error || c.Status().Description != "target error" {
				t.Errorf("failed call status = %+v", c.Status())
			}
			ev := c.Events()
			if len(ev) != 1 || ev[0].Name != "exception" || len(ev[0].Attributes) != 1 ||
				ev[0].Attributes[0].Key != "exception.type" || ev[0].Attributes[0].Value.AsString() != "*errors.errorString" {
				t.Errorf("exception event must carry only exception.type: %+v", ev)
			}
		}
	}
	if !sawOK || !sawErr {
		t.Errorf("want one ok and one failed target call, ok=%v error=%v", sawOK, sawErr)
	}

	// Only the case whose target answered is scored: one custom scorer and
	// the built-in exact.
	scores := got["score output"]
	if len(scores) != 2 {
		t.Fatalf("want two scorer spans, got %d", len(scores))
	}
	kinds := map[string]bool{}
	for _, sc := range scores {
		if _, ok := caseByID[sc.Parent().SpanID()]; !ok {
			t.Error("scorer span is not a child of a case span")
		}
		k, _ := attrOf(sc, "sentinel.scorer.kind")
		kinds[k.AsString()] = true
		wantAttr(t, sc, "sentinel.scorer.result", "ok")
	}
	if !kinds["custom"] || !kinds["exact"] || len(kinds) != 2 {
		t.Errorf("scorer kinds = %v, want custom and exact", kinds)
	}
}

func TestStartRunIsARootLinkedToTheRequest(t *testing.T) {
	ended := &spanEnded{name: "evaluate run", ch: make(chan sdktrace.ReadOnlySpan, 4)}
	rec, tr := recordSpans(t, ended)
	echo := target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return in, nil })
	e := newEngine(t, engine.WithTarget("echo", "test", echo), goodScorer())
	s := seedSuite(t, e, "p", "a", "b")

	reqCtx, req := tr.Start(bg(), "request")
	run, err := e.StartRun(reqCtx, &engine.StartConfig{SuiteID: s.ID, Target: "echo", Scorers: []string{"good"}})
	req.End()
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	var span sdktrace.ReadOnlySpan
	select {
	case span = <-ended.ch:
	case <-time.After(5 * time.Second):
		t.Fatal("the run span never ended")
	}
	if span.Parent().IsValid() {
		t.Fatal("a StartRun run must be a root span, not a child of the request")
	}
	if span.SpanContext().TraceID() == req.SpanContext().TraceID() {
		t.Fatal("a StartRun run must start its own trace")
	}
	links := span.Links()
	if len(links) != 1 || links[0].SpanContext.SpanID() != req.SpanContext().SpanID() ||
		links[0].SpanContext.TraceID() != req.SpanContext().TraceID() {
		t.Fatalf("the run must link back to the request span: %+v", links)
	}
	wantAttr(t, span, "sentinel.run.mode", "async")
	wantAttr(t, span, "sentinel.run.id", run.ID.String())
	wantAttr(t, span, "sentinel.run.state", "completed")
	wantAttr(t, span, "sentinel.target.kind", "func")

	cases := 0
	for _, c := range sentinelSpans(rec)["evaluate case"] {
		if c.SpanContext().TraceID() == span.SpanContext().TraceID() {
			cases++
		}
	}
	if cases != 2 {
		t.Fatalf("both cases must nest in the run's trace, got %d", cases)
	}
}

func TestStartRunWithoutARequestSpanHasNoLink(t *testing.T) {
	ended := &spanEnded{name: "evaluate run", ch: make(chan sdktrace.ReadOnlySpan, 4)}
	recordSpans(t, ended)
	echo := target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return in, nil })
	e := newEngine(t, engine.WithTarget("echo", "test", echo), goodScorer())
	s := seedSuite(t, e, "p", "a")

	if _, err := e.StartRun(bg(), &engine.StartConfig{SuiteID: s.ID, Target: "echo", Scorers: []string{"good"}}); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	select {
	case span := <-ended.ch:
		if span.Parent().IsValid() || len(span.Links()) != 0 {
			t.Fatalf("an untraced start gives a root with no link: parent=%v links=%v", span.Parent(), span.Links())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the run span never ended")
	}
}

func TestACancelledRunIsNotAnError(t *testing.T) {
	rec, _ := recordSpans(t)
	e := newEngine(t, engine.WithConfig(configWith(1)))
	s := seedSuite(t, e, "p", "a", "b")
	ctx, cancel := context.WithCancel(bg())
	entered := make(chan string, 4)
	done := runEvalAsync(ctx, e, &engine.RunConfig{SuiteID: s.ID, Target: gated(make(chan struct{}), entered, nil, nil),
		Scorers: []scorer.Scorer{okScorer("good", 1, "")}})
	<-entered
	cancel()
	if got := <-done; !errors.Is(got.err, sentinel.ErrRunCancelled) {
		t.Fatalf("want a cancelled run, got %v", got.err)
	}

	got := sentinelSpans(rec)
	if len(got["evaluate run"]) != 1 {
		t.Fatalf("want one run span, got %d", len(got["evaluate run"]))
	}
	run := got["evaluate run"][0]
	wantAttr(t, run, "sentinel.run.state", "cancelled")
	for name, spans := range got {
		for _, sp := range spans {
			if sp.Status().Code == codes.Error {
				t.Errorf("%s: cancellation is not an error: %+v", name, sp.Status())
			}
		}
	}
	for _, c := range got["call target"] {
		wantAttr(t, c, "sentinel.target.result", "cancelled")
	}
}

func TestRegressionIsAnEventOnTheRunSpan(t *testing.T) {
	rec, _ := recordSpans(t)
	e := newEngine(t)
	s := seedSuite(t, e, "p", "a")
	echo := target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return in, nil })
	good, err := e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID, Target: echo, Scorers: []scorer.Scorer{okScorer("s", 1, "")}})
	if err != nil {
		t.Fatal(err)
	}
	b := saveBaseline(t, e, good)
	if _, err = e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID, Target: echo, Scorers: []scorer.Scorer{okScorer("s", 0, "")}}); err != nil {
		t.Fatal(err)
	}

	runs := sentinelSpans(rec)["evaluate run"]
	if len(runs) != 2 {
		t.Fatalf("want two run spans, got %d", len(runs))
	}
	if _, ok := attrOf(runs[0], "sentinel.run.regressed"); ok {
		t.Error("with no baseline yet there is nothing to compare, so no regressed attribute")
	}
	wantAttr(t, runs[1], "sentinel.run.regressed", true)
	var found bool
	for _, ev := range runs[1].Events() {
		if ev.Name != "regression detected" {
			continue
		}
		found = true
		for _, kv := range ev.Attributes {
			switch kv.Key {
			case "sentinel.baseline.id":
				if kv.Value.AsString() != b.ID.String() {
					t.Errorf("baseline id = %s", kv.Value.AsString())
				}
			case "sentinel.regression.worst_delta":
				if kv.Value.AsFloat64() != -1 {
					t.Errorf("worst delta = %v", kv.Value.AsFloat64())
				}
			default:
				t.Errorf("unexpected event attribute %s", kv.Key)
			}
		}
	}
	if !found {
		t.Fatal("no regression event on the regressed run")
	}
}

func TestCompareGroupsItsRunsInOneTrace(t *testing.T) {
	rec, _ := recordSpans(t)
	e := newEngine(t)
	s := seedSuite(t, e, "p", "a")
	echo := target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return in, nil })
	shout := target.FromFunc("shout", func(_ context.Context, in string) (string, error) { return strings.ToUpper(in), nil })

	if _, err := comparison.Compare(bg(), e, &comparison.CompareConfig{SuiteID: s.ID, Targets: []target.Target{echo, shout},
		Scorers: []scorer.Scorer{okScorer("s", 1, "")}}); err != nil {
		t.Fatalf("Compare: %v", err)
	}
	got := sentinelSpans(rec)
	if len(got["compare targets"]) != 1 {
		t.Fatalf("want one compare span, got %d", len(got["compare targets"]))
	}
	cmp := got["compare targets"][0]
	wantAttr(t, cmp, "sentinel.compare.targets", int64(2))
	wantAttr(t, cmp, "sentinel.suite.id", s.ID.String())
	runs := got["evaluate run"]
	if len(runs) != 2 {
		t.Fatalf("want two run spans, got %d", len(runs))
	}
	for _, r := range runs {
		if r.Parent().SpanID() != cmp.SpanContext().SpanID() {
			t.Error("each compared run must be a child of the compare span")
		}
	}
}

// Nothing a suite, a case, a target, a scorer or a judge says reaches a
// span: not the prompt, the input, the expected or actual output, a
// scorer's reason, an error's text, or the names a user gave a scorer.
func TestSpansCarryNoContent(t *testing.T) {
	const secret = "alice@example.com"
	rec, _ := recordSpans(t)
	e := newEngine(t)
	s := seedSuite(t, e, "prompt for "+secret)
	for i, in := range []string{"hi " + secret, "boom " + secret} {
		tc := &testcase.Case{Entity: sentinel.NewEntity(), SuiteID: s.ID, Name: "case " + secret, Input: in, Expected: "expect " + secret,
			ScenarioType: testcase.ScenarioStandard, Tags: []string{secret}, Context: map[string]any{"email": secret},
			Metadata: map[string]any{"owner": secret}, Scorers: []testcase.ScorerConfig{}}
		if i == 0 {
			// A case scorer nobody registered: its name is user data.
			tc.Scorers = []testcase.ScorerConfig{{Name: "judge " + secret}}
		}
		if err := e.CreateCase(bg(), tc); err != nil {
			t.Fatal(err)
		}
	}
	tgt := target.FromFunc("target "+secret, func(_ context.Context, in string) (string, error) {
		if strings.HasPrefix(in, "boom") {
			return "", errors.New("upstream rejected " + secret)
		}
		return "answer for " + secret, nil
	})
	reasoned := scorer.FromFunc("scorer "+secret, func(context.Context, *scorer.Input) (*scorer.Output, error) {
		return &scorer.Output{Score: 0.9, Passed: true, Reason: "because " + secret, Dimension: "dim " + secret,
			Details: map[string]any{"why": secret}}, nil
	})
	failing := scorer.FromFunc("judge2 "+secret, func(context.Context, *scorer.Input) (*scorer.Output, error) {
		return nil, errors.New("judge said " + secret)
	})

	if _, err := e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID, Target: tgt, Scorers: []scorer.Scorer{reasoned, failing},
		PersonaRef: "persona " + secret, Tags: []string{secret}}); err != nil {
		t.Fatalf("RunEval: %v", err)
	}

	spans := rec.Ended()
	if len(spans) == 0 {
		t.Fatal("no spans recorded")
	}
	var sawError bool
	for _, sp := range spans {
		check := func(where, v string) {
			if strings.Contains(v, "alice") || strings.Contains(v, "example.com") {
				t.Errorf("%s: %s carries content: %q", sp.Name(), where, v)
			}
		}
		check("name", sp.Name())
		check("status", sp.Status().Description)
		if sp.Status().Code == codes.Error {
			sawError = true
		}
		for _, kv := range sp.Attributes() {
			check(string(kv.Key), kv.Value.String())
		}
		for _, ev := range sp.Events() {
			check("event", ev.Name)
			for _, kv := range ev.Attributes {
				if kv.Key == "exception.message" || kv.Key == "exception.stacktrace" {
					t.Errorf("%s: %s must never be recorded", sp.Name(), kv.Key)
				}
				check("event "+string(kv.Key), kv.Value.String())
			}
		}
		for _, l := range sp.Links() {
			for _, kv := range l.Attributes {
				check("link "+string(kv.Key), kv.Value.String())
			}
		}
	}
	if !sawError {
		t.Fatal("the failing target and scorer should have produced error spans to inspect")
	}
}

func saveBaseline(t *testing.T, e *engine.Engine, res *engine.RunResult) *baseline.Baseline {
	t.Helper()
	b := &baseline.Baseline{SuiteID: res.Run.SuiteID, RunID: res.Run.ID, Name: "good", PassRate: res.Run.PassRate, AvgScore: res.Run.AvgScore,
		DimensionScores: map[string]float64{}, IsCurrent: true}
	for _, r := range res.Results {
		b.Results = append(b.Results, baseline.Result{CaseID: r.CaseID, CaseName: r.CaseName, Score: r.Score, Status: string(r.Status)})
	}
	if err := e.SaveBaseline(bg(), b); err != nil {
		t.Fatal(err)
	}
	return b
}
