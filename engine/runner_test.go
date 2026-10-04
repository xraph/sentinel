package engine_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/promptversion"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/store/memory"
	"github.com/xraph/sentinel/target"
)

// gated returns a target that answers one case each time release receives.
// It reports each input on entered (when entered is not nil) before it
// blocks, so a test can release a case it knows is waiting, and it records
// the call options it saw.
func gated(release <-chan struct{}, entered chan<- string, seen *target.CallOptions, mu *sync.Mutex) target.Target {
	return target.FromFunc("gated", func(ctx context.Context, in string) (string, error) {
		if o, ok := target.CallOptionsFrom(ctx); ok && seen != nil {
			mu.Lock()
			*seen = o
			mu.Unlock()
		}
		if entered != nil {
			entered <- in
		}
		select {
		case <-release:
			return "ok " + in, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
}

func startGated(t *testing.T, e *engine.Engine, suiteID id.SuiteID) *evalrun.Run {
	t.Helper()
	run, err := e.StartRun(bg(), &engine.StartConfig{SuiteID: suiteID, Target: "gated", Scorers: []string{"good"}})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	return run
}

func goodScorer() engine.Option {
	return engine.WithScorer(scorer.Descriptor{Name: "good"}, func(map[string]any) (scorer.Scorer, error) { return okScorer("good", 1, "skill"), nil })
}

func stateOf(e *engine.Engine, runID id.EvalRunID) evalrun.RunState {
	r, err := e.GetRun(bg(), runID)
	if err != nil {
		return ""
	}
	return r.State
}

func TestStartRunReturnsAtOnceAndPersistsPerCase(t *testing.T) {
	release := make(chan struct{})
	e := newEngine(t, engine.WithTarget("gated", "test", gated(release, nil, nil, nil)), goodScorer(),
		engine.WithConfig(configWith(1)))
	s := seedSuite(t, e, "p", "a", "b", "c")

	run := startGated(t, e, s.ID)
	if run.State != evalrun.StateRunning || run.TotalCases != 3 {
		t.Fatalf("started run: %+v", run)
	}

	release <- struct{}{}
	waitFor(t, "one stored result", func() bool { r, _ := e.ListResults(bg(), run.ID); return len(r) == 1 })
	if stateOf(e, run.ID) != evalrun.StateRunning {
		t.Fatal("the run must still be running with one of three results stored")
	}

	release <- struct{}{}
	release <- struct{}{}
	waitFor(t, "completion", func() bool { return stateOf(e, run.ID) == evalrun.StateCompleted })
	done, _ := e.GetRun(bg(), run.ID)
	if done.Passed != 3 || done.PassRate != 1 || done.CompletedAt == nil {
		t.Fatalf("final counters come from the stored results: %+v", done)
	}
}

func TestCancelStopsSchedulingAndKeepsPartialCounts(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan string, 10)
	e := newEngine(t, engine.WithTarget("gated", "test", gated(release, entered, nil, nil)), goodScorer(),
		engine.WithConfig(configWith(1)))
	s := seedSuite(t, e, "p", "a", "b", "c", "d")
	run := startGated(t, e, s.ID)

	<-entered             // case a is waiting in the target
	release <- struct{}{} // case a finishes
	<-entered             // case b is now in flight: concurrency is 1, so nothing else is
	if err := e.CancelRun(bg(), run.ID); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}
	release <- struct{}{} // let b finish; the runner must not start c

	waitFor(t, "the runner to finish", func() bool {
		r, _ := e.GetRun(bg(), run.ID)
		return r.State == evalrun.StateCancelled && r.Passed == 2
	})
	results, _ := e.ListResults(bg(), run.ID)
	if len(results) != 2 {
		t.Fatalf("in-flight case stored, nothing after it: %d results", len(results))
	}
	if err := e.CancelRun(bg(), run.ID); !errors.Is(err, sentinel.ErrInvalidState) {
		t.Fatalf("cancelling a cancelled run: want ErrInvalidState, got %v", err)
	}
	if err := e.CancelRun(bg(), id.NewEvalRunID()); !errors.Is(err, sentinel.ErrRunNotFound) {
		t.Fatalf("cancelling a missing run: want ErrRunNotFound, got %v", err)
	}
}

func TestStartRunRefusesBeforeWritingAnything(t *testing.T) {
	e := newEngine(t, engine.WithTarget("gated", "test", gated(make(chan struct{}), nil, nil, nil)), goodScorer())
	s := seedSuite(t, e, "p", "a")
	empty := seedSuite(t, e, "p")

	cases := []struct {
		name string
		cfg  engine.StartConfig
		want error
	}{
		{"unknown target", engine.StartConfig{SuiteID: s.ID, Target: "nope", Scorers: []string{"good"}}, sentinel.ErrUnknownTarget},
		{"unknown scorer", engine.StartConfig{SuiteID: s.ID, Target: "gated", Scorers: []string{"nope"}}, sentinel.ErrUnknownScorer},
		{"no scorers", engine.StartConfig{SuiteID: s.ID, Target: "gated"}, sentinel.ErrNoScorers},
		{"scorer that needs config", engine.StartConfig{SuiteID: s.ID, Target: "gated", Scorers: []string{"good", "regex"}}, sentinel.ErrInvalidInput},
		{"no cases", engine.StartConfig{SuiteID: empty.ID, Target: "gated", Scorers: []string{"good"}}, sentinel.ErrEmptyInput},
	}
	for _, c := range cases {
		cfg := c.cfg
		if _, err := e.StartRun(bg(), &cfg); !errors.Is(err, c.want) {
			t.Errorf("%s: want %v, got %v", c.name, c.want, err)
		}
	}
	// The dashboard shows this text to the caller, so it must say why.
	_, err := e.StartRun(bg(), &engine.StartConfig{SuiteID: empty.ID, Target: "gated", Scorers: []string{"good"}})
	if err == nil || !strings.Contains(err.Error(), "the suite has no cases") {
		t.Errorf("a no-cases refusal must say why: %v", err)
	}
	if runs, _ := e.ListRuns(bg(), &evalrun.ListFilter{}); len(runs) != 0 {
		t.Fatalf("a refused start must write no run, found %d", len(runs))
	}
}

// regex is registered, so asking for it is not an unknown scorer; it just
// cannot run at run level, where nobody gives it a pattern.
func TestStartRunNamesAScorerThatNeedsConfig(t *testing.T) {
	e := newEngine(t, engine.WithTarget("gated", "test", gated(make(chan struct{}), nil, nil, nil)))
	s := seedSuite(t, e, "p", "a")
	_, err := e.StartRun(bg(), &engine.StartConfig{SuiteID: s.ID, Target: "gated", Scorers: []string{"regex"}})
	if !errors.Is(err, sentinel.ErrInvalidInput) || errors.Is(err, sentinel.ErrUnknownScorer) {
		t.Fatalf("want ErrInvalidInput and not ErrUnknownScorer, got %v", err)
	}
	if !strings.Contains(err.Error(), `"regex"`) || !strings.Contains(err.Error(), "pattern") {
		t.Fatalf("the error must name the scorer and what it lacks: %v", err)
	}
	if runs, _ := e.ListRuns(bg(), &evalrun.ListFilter{}); len(runs) != 0 {
		t.Fatalf("a refused start must write no run, found %d", len(runs))
	}
}

func TestRunRecordsSettingsAndSendsTheCurrentPrompt(t *testing.T) {
	release := make(chan struct{}, 1)
	var seen target.CallOptions
	var mu sync.Mutex
	e := newEngine(t, engine.WithTarget("gated", "test", gated(release, nil, &seen, &mu)), goodScorer())
	s := seedSuite(t, e, "suite prompt", "a")
	pv := &promptversion.PromptVersion{SuiteID: s.ID, SystemPrompt: "version prompt", IsCurrent: true}
	if err := e.CreatePromptVersion(bg(), pv); err != nil {
		t.Fatal(err)
	}

	release <- struct{}{}
	run := startGated(t, e, s.ID)
	waitFor(t, "completion", func() bool { return stateOf(e, run.ID) == evalrun.StateCompleted })

	got, _ := e.GetRun(bg(), run.ID)
	st := evalrun.SettingsFrom(got.Config)
	if st.PassThreshold == nil || *st.PassThreshold != 0.7 || st.RegressionThreshold == nil || *st.RegressionThreshold != 0.05 {
		t.Fatalf("thresholds not recorded: %+v", st)
	}
	if st.Target != "gated" || len(st.Scorers) != 1 || st.Scorers[0] != "good" || st.PromptVersionID != pv.ID.String() {
		t.Fatalf("settings: %+v", st)
	}
	if got.SystemPrompt != "version prompt" {
		t.Fatalf("the run records the current version's prompt: %q", got.SystemPrompt)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen.SystemPrompt != "version prompt" {
		t.Fatalf("the target must receive the prompt the run records: %q", seen.SystemPrompt)
	}
}

// failingResults refuses to store results for one case name.
type failingResults struct {
	*memory.Store
	failFor string
}

func (f *failingResults) CreateResult(ctx context.Context, r *evalrun.Result) error {
	if r.CaseName == f.failFor {
		return errors.New("disk full")
	}
	return f.Store.CreateResult(ctx, r)
}

// Review focus 4.
func TestAnUnstoredResultFailsTheRun(t *testing.T) {
	st := &failingResults{Store: memory.New(), failFor: "b"}
	e, err := engine.New(engine.WithStore(st))
	if err != nil {
		t.Fatal(err)
	}
	s := seedSuite(t, e, "p", "a", "b", "c")
	res, err := e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID,
		Target:  target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return in, nil }),
		Scorers: []scorer.Scorer{okScorer("good", 1, "")}})
	if err == nil {
		t.Fatal("RunEval must report the failure")
	}
	if res == nil || res.Run.State != evalrun.StateFailed || res.Run.Error != "1 of 3 results could not be stored" {
		t.Fatalf("run: %+v", res)
	}
}

// Review focus 5.
func TestStopCancelsActiveRuns(t *testing.T) {
	release := make(chan struct{})
	st := memory.New()
	e, err := engine.New(engine.WithStore(st), engine.WithTarget("gated", "test", gated(release, nil, nil, nil)), goodScorer(),
		engine.WithConfig(configWith(1)))
	if err != nil {
		t.Fatal(err)
	}
	s := seedSuite(t, e, "p", "a", "b", "c")
	run := startGated(t, e, s.ID)

	if err = e.Stop(bg()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	got, err := st.GetRun(bg(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != evalrun.StateCancelled {
		t.Fatalf("a run interrupted by shutdown must end cancelled, got %s", got.State)
	}
}

// Review focus 5: a stop that lands after every case is scheduled must
// still end the run cancelled.
func TestStopCancelsARunWithEveryCaseInFlight(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan string, 10)
	st := memory.New()
	e, err := engine.New(engine.WithStore(st), engine.WithTarget("gated", "test", gated(release, entered, nil, nil)), goodScorer(),
		engine.WithConfig(configWith(4)))
	if err != nil {
		t.Fatal(err)
	}
	s := seedSuite(t, e, "p", "a", "b", "c")
	run := startGated(t, e, s.ID)
	for i := 0; i < 3; i++ {
		<-entered
	}

	if err = e.Stop(bg()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	got, err := st.GetRun(bg(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != evalrun.StateCancelled {
		t.Fatalf("a run whose cases were all in flight at shutdown must end cancelled, got %s", got.State)
	}
}

// failingStats cannot compute a run's counters.
type failingStats struct {
	*memory.Store
}

func (f *failingStats) GetResultStats(context.Context, id.EvalRunID) (*evalrun.ResultStats, error) {
	return nil, errors.New("connection reset")
}

func TestUnreadableStatsFailTheRun(t *testing.T) {
	st := &failingStats{Store: memory.New()}
	e, err := engine.New(engine.WithStore(st))
	if err != nil {
		t.Fatal(err)
	}
	s := seedSuite(t, e, "p", "a", "b")
	res, err := e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID, Target: echo("echo"),
		Scorers: []scorer.Scorer{okScorer("good", 1, "")}})
	if err == nil {
		t.Fatal("RunEval must report the failure")
	}
	// The run row is shown to dashboard users, so it carries a generic
	// message; the cause goes to the engine logger only.
	want := "result stats could not be read"
	if res == nil || res.Run.State != evalrun.StateFailed || res.Run.Error != want {
		t.Fatalf("run: %+v", res)
	}
	stored, gerr := st.GetRun(bg(), res.Run.ID)
	if gerr != nil || stored.State != evalrun.StateFailed || stored.Error != want {
		t.Fatalf("stored run: %+v %v", stored, gerr)
	}
	if strings.Contains(stored.Error, "connection reset") {
		t.Fatalf("the store's error text must not reach the run row: %q", stored.Error)
	}
}

func TestRunEvalStaysSynchronous(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "p", "a", "b")
	res, err := e.RunEval(bg(), &engine.RunConfig{SuiteID: s.ID,
		Target:  target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return in, nil }),
		Scorers: []scorer.Scorer{okScorer("good", 1, "")}})
	if err != nil || res.Run.State != evalrun.StateCompleted || len(res.Results) != 2 || res.Stats.Passed != 2 {
		t.Fatalf("RunEval: %v %+v", err, res)
	}
}

func configWith(concurrency int) sentinel.Config {
	c := sentinel.DefaultConfig()
	c.Concurrency = concurrency
	c.ShutdownTimeout = 2 * time.Second
	return c
}

// evalOutcome is what RunEval returned.
type evalOutcome struct {
	res *engine.RunResult
	err error
}

// runEvalAsync runs RunEval on a goroutine and delivers its outcome.
func runEvalAsync(ctx context.Context, e *engine.Engine, cfg *engine.RunConfig) <-chan evalOutcome {
	out := make(chan evalOutcome, 1)
	go func() {
		res, err := e.RunEval(ctx, cfg)
		out <- evalOutcome{res, err}
	}()
	return out
}

func TestRunEvalReportsACancelledRun(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan string, 10)
	e := newEngine(t, engine.WithConfig(configWith(1)))
	s := seedSuite(t, e, "p", "a", "b", "c", "d")
	ctx, cancel := context.WithCancel(bg())
	defer cancel()

	done := runEvalAsync(ctx, e, &engine.RunConfig{SuiteID: s.ID, Target: gated(release, entered, nil, nil),
		Scorers: []scorer.Scorer{okScorer("good", 1, "")}})
	<-entered             // a is in flight
	release <- struct{}{} // a passes
	<-entered             // b is in flight
	cancel()              // b is cut short; c and d never start

	got := <-done
	if !errors.Is(got.err, sentinel.ErrRunCancelled) {
		t.Fatalf("a cancelled run must return ErrRunCancelled, got %v", got.err)
	}
	if !errors.Is(got.err, context.Canceled) {
		t.Fatalf("the error must carry the caller's ctx error, got %v", got.err)
	}
	if got.res == nil || got.res.Run.State != evalrun.StateCancelled {
		t.Fatalf("a cancelled run still returns its result, cancelled: %+v", got.res)
	}
}

func TestRunEvalReportsADeadline(t *testing.T) {
	e := newEngine(t, engine.WithConfig(configWith(1)))
	s := seedSuite(t, e, "p", "a", "b", "c")
	ctx, cancel := context.WithTimeout(bg(), 50*time.Millisecond)
	defer cancel()

	// A target that never answers until ctx ends.
	res, err := e.RunEval(ctx, &engine.RunConfig{SuiteID: s.ID, Target: gated(make(chan struct{}), nil, nil, nil),
		Scorers: []scorer.Scorer{okScorer("good", 1, "")}})
	if !errors.Is(err, sentinel.ErrRunCancelled) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a run past its deadline must return ErrRunCancelled and DeadlineExceeded, got %v", err)
	}
	if res == nil || res.Run.State != evalrun.StateCancelled {
		t.Fatalf("run: %+v", res)
	}
}

func TestRunEvalReportsARunCancelledElsewhere(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan string, 10)
	e := newEngine(t, engine.WithConfig(configWith(1)))
	s := seedSuite(t, e, "p", "a", "b", "c")

	done := runEvalAsync(bg(), e, &engine.RunConfig{SuiteID: s.ID, Target: gated(release, entered, nil, nil),
		Scorers: []scorer.Scorer{okScorer("good", 1, "")}})
	<-entered // a is in flight
	runs, err := e.ListRuns(bg(), &evalrun.ListFilter{})
	if err != nil || len(runs) != 1 {
		t.Fatalf("list runs: %v (%d)", err, len(runs))
	}
	if err = e.CancelRun(bg(), runs[0].ID); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}
	release <- struct{}{} // a finishes; the runner sees the cancel and stops

	got := <-done
	if !errors.Is(got.err, sentinel.ErrRunCancelled) {
		t.Fatalf("a run cancelled from another caller must return ErrRunCancelled, got %v", got.err)
	}
	if errors.Is(got.err, context.Canceled) || errors.Is(got.err, context.DeadlineExceeded) {
		t.Fatalf("the caller's ctx did not end, so no ctx error belongs in %v", got.err)
	}
	if got.res == nil || got.res.Run.State != evalrun.StateCancelled {
		t.Fatalf("run: %+v", got.res)
	}
}

// A suite temperature of 0 inherits the configured default; a suite that
// sets one keeps it. The run records the value the target was sent.
func TestRunTemperatureFallsBackToTheConfiguredDefault(t *testing.T) {
	cases := []struct {
		name  string
		suite float64
		want  float64
	}{
		{"suite unset inherits config", 0, 0.3},
		{"suite set keeps its own", 0.9, 0.9},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			release := make(chan struct{}, 1)
			var seen target.CallOptions
			var mu sync.Mutex
			cfg := configWith(1)
			cfg.Temperature = 0.3
			e := newEngine(t, engine.WithTarget("gated", "test", gated(release, nil, &seen, &mu)), goodScorer(), engine.WithConfig(cfg))
			s := seedSuite(t, e, "p", "a")
			s.Temperature = c.suite
			if err := e.UpdateSuite(bg(), s); err != nil {
				t.Fatal(err)
			}

			release <- struct{}{}
			run := startGated(t, e, s.ID)
			if run.Temperature != c.want {
				t.Fatalf("started run temperature: want %v, got %v", c.want, run.Temperature)
			}
			waitFor(t, "completion", func() bool { return stateOf(e, run.ID) == evalrun.StateCompleted })
			got, _ := e.GetRun(bg(), run.ID)
			if got.Temperature != c.want {
				t.Fatalf("stored run temperature: want %v, got %v", c.want, got.Temperature)
			}
			mu.Lock()
			defer mu.Unlock()
			if seen.Temperature != c.want {
				t.Fatalf("target temperature: want %v, got %v", c.want, seen.Temperature)
			}
		})
	}
}
