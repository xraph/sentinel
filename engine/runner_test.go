package engine_test

import (
	"context"
	"errors"
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
		{"no cases", engine.StartConfig{SuiteID: empty.ID, Target: "gated", Scorers: []string{"good"}}, sentinel.ErrEmptyInput},
	}
	for _, c := range cases {
		cfg := c.cfg
		if _, err := e.StartRun(bg(), &cfg); !errors.Is(err, c.want) {
			t.Errorf("%s: want %v, got %v", c.name, c.want, err)
		}
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
	want := "result stats could not be read: connection reset"
	if res == nil || res.Run.State != evalrun.StateFailed || res.Run.Error != want {
		t.Fatalf("run: %+v", res)
	}
	stored, gerr := st.GetRun(bg(), res.Run.ID)
	if gerr != nil || stored.State != evalrun.StateFailed || stored.Error != want {
		t.Fatalf("stored run: %+v %v", stored, gerr)
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
