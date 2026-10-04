package contract

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
)

func float(v float64) *float64 { return &v }

func TestRegressionStates(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "a", "x")
	ctx := context.Background()

	good := seedCompletedRun(t, d, s, 1)
	got, err := runsRegressionHandler(d)(ctx, runsRegressionInput{RunID: good.ID.String()}, operator)
	if err != nil || got.State != "noBaseline" {
		t.Fatalf("no baseline is its own state, never a pass: %+v %v", got, err)
	}

	b, err := baselinesSaveHandler(d)(ctx, baselinesSaveInput{RunID: good.ID.String(), Name: "release-1"}, operator)
	if err != nil || !b.IsCurrent || b.SuiteID != s.ID.String() {
		t.Fatalf("save baseline: %+v %v", b, err)
	}

	bad := seedCompletedRun(t, d, s, 0.5)
	cmp, err := runsRegressionHandler(d)(ctx, runsRegressionInput{RunID: bad.ID.String()}, operator)
	if err != nil || cmp.State != "compared" || !cmp.HasRegression || cmp.ThresholdSource != "run" || cmp.Baseline == nil || cmp.Baseline.ID != b.ID {
		t.Fatalf("regression against the current baseline at the run's recorded threshold: %+v %v", cmp, err)
	}
	if cmp.WorstDelta == nil || *cmp.WorstDelta >= 0 {
		t.Fatalf("worst delta: %+v", cmp.WorstDelta)
	}

	loose, err := runsRegressionHandler(d)(ctx, runsRegressionInput{RunID: bad.ID.String(), Threshold: float(1)}, operator)
	if err != nil || loose.HasRegression || loose.ThresholdSource != "override" {
		t.Fatalf("an override of 1 forgives everything: %+v %v", loose, err)
	}
	// Review focus 5.
	_, err = runsRegressionHandler(d)(ctx, runsRegressionInput{RunID: bad.ID.String(), Threshold: float(1.5)}, operator)
	wantCode(t, err, "BAD_REQUEST")
}

func TestRegressionForNonCompletedRuns(t *testing.T) {
	d := newTestDeps(t)
	for state, want := range map[evalrun.RunState]RegressionView{
		evalrun.StateRunning:   {State: "running"},
		evalrun.StateFailed:    {State: "notComparable", Reason: "runFailed"},
		evalrun.StateCancelled: {State: "notComparable", Reason: "runCancelled"},
	} {
		got, err := d.regressionFor(context.Background(), testApp, &evalrun.Run{State: state}, "", nil)
		if err != nil || got.State != want.State || got.Reason != want.Reason {
			t.Errorf("%s: %+v %v", state, got, err)
		}
	}
}

func TestBaselineFromAnotherSuiteIsNotComparable(t *testing.T) {
	d := newTestDeps(t)
	a := seedSuite(t, d, testApp, "a", "p")
	seedCase(t, d, a.ID, "x", "x")
	b := seedSuite(t, d, testApp, "b", "p")
	seedCase(t, d, b.ID, "x", "x")
	ctx := context.Background()
	baseB, err := baselinesSaveHandler(d)(ctx, baselinesSaveInput{RunID: seedCompletedRun(t, d, b, 1).ID.String(), Name: "b"}, operator)
	if err != nil {
		t.Fatal(err)
	}
	got, err := runsRegressionHandler(d)(ctx, runsRegressionInput{RunID: seedCompletedRun(t, d, a, 1).ID.String(), BaselineID: baseB.ID}, operator)
	if err != nil || got.State != "notComparable" || got.Reason != "otherSuite" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestBaselinesSaveRefusesUnfinishedRunsAndOtherApps(t *testing.T) {
	d := newTestDeps(t)
	theirs := seedSuite(t, d, "app_b", "theirs", "p")
	seedCase(t, d, theirs.ID, "x", "x")
	ctx := context.Background()
	_, err := baselinesSaveHandler(d)(ctx, baselinesSaveInput{RunID: seedCompletedRun(t, d, theirs, 1).ID.String(), Name: "x"}, operator)
	wantCode(t, err, "NOT_FOUND")

	mine := seedSuite(t, d, testApp, "mine", "p")
	seedCase(t, d, mine.ID, "x", "x")
	run := seedCompletedRun(t, d, mine, 1)
	running := seedRunningRun(t, d, mine)
	_, err = baselinesSaveHandler(d)(ctx, baselinesSaveInput{RunID: running.ID.String(), Name: "x"}, operator)
	wantCode(t, err, "CONFLICT")
	_, err = baselinesSaveHandler(d)(ctx, baselinesSaveInput{RunID: run.ID.String(), Name: " "}, operator)
	wantCode(t, err, "BAD_REQUEST")
}

func TestBaselinesListDetailDelete(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "x", "x")
	ctx := context.Background()
	saved, err := baselinesSaveHandler(d)(ctx, baselinesSaveInput{RunID: seedCompletedRun(t, d, s, 1).ID.String(), Name: "b"}, operator)
	if err != nil {
		t.Fatal(err)
	}
	all, err := baselinesListHandler(d)(ctx, baselinesListInput{}, operator)
	if err != nil || len(all.Items) != 1 || all.Items[0].SuiteName != "s" {
		t.Fatalf("list across the app's suites: %+v %v", all, err)
	}
	detail, err := baselinesDetailHandler(d)(ctx, baselineRef{BaselineID: saved.ID}, operator)
	if err != nil || len(detail.Results) != 1 {
		t.Fatalf("detail: %+v %v", detail, err)
	}
	if _, err := baselinesDeleteHandler(d)(ctx, baselineRef{BaselineID: saved.ID}, operator); err != nil {
		t.Fatal(err)
	}
	_, gone := baselinesDetailHandler(d)(ctx, baselineRef{BaselineID: saved.ID}, operator)
	wantCode(t, gone, "NOT_FOUND")
}

func TestBaselineInAnotherAppIsIndistinguishableFromMissing(t *testing.T) {
	d := newTestDeps(t)
	theirs := seedSuite(t, d, "app_b", "theirs", "p")
	seedCase(t, d, theirs.ID, "x", "x")
	ctx := context.Background()
	run := seedCompletedRun(t, d, theirs, 1)
	results, err := d.Engine.ListResults(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	other := &baseline.Baseline{SuiteID: theirs.ID, RunID: run.ID, Name: "theirs", PassRate: 1, AvgScore: 1,
		DimensionScores: map[string]float64{}, Results: make([]baseline.Result, 0, len(results)), IsCurrent: true, CreatedAt: time.Now().UTC()}
	if err := d.Engine.SaveBaseline(ctx, other); err != nil {
		t.Fatal(err)
	}
	_, missing := baselinesDetailHandler(d)(ctx, baselineRef{BaselineID: id.NewBaselineID().String()}, operator)
	_, foreign := baselinesDetailHandler(d)(ctx, baselineRef{BaselineID: other.ID.String()}, operator)
	wantSameNotFound(t, missing, foreign)
}

func TestRunsDetailEmbedsTheRegression(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "x", "x")
	run := seedCompletedRun(t, d, s, 1)
	got, err := runsDetailHandler(d)(context.Background(), runRef{RunID: run.ID.String()}, operator)
	if err != nil || got.Run.ID != run.ID.String() || got.Regression.State != "noBaseline" || got.Run.LastProgressAt == nil {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestRegressionThresholdSourceIsConfigWithoutARecordedThreshold(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "a", "x")
	ctx := context.Background()
	if _, err := baselinesSaveHandler(d)(ctx, baselinesSaveInput{RunID: seedCompletedRun(t, d, s, 1).ID.String(), Name: "b"}, operator); err != nil {
		t.Fatal(err)
	}
	run := seedCompletedRun(t, d, s, 1)
	run.Config = map[string]any{}
	if err := d.Engine.Store().UpdateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	got, err := runsRegressionHandler(d)(ctx, runsRegressionInput{RunID: run.ID.String()}, operator)
	if err != nil || got.State != "compared" || got.ThresholdSource != "config" {
		t.Fatalf("a run with no recorded threshold falls back to config: %+v %v", got, err)
	}
	if got.Threshold == nil || *got.Threshold != d.Engine.Config().RegressionThreshold {
		t.Fatalf("threshold should be the engine config's: %+v", got.Threshold)
	}
}

func TestRegressionViewsAlwaysSendTheirCollections(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "a", "x")
	ctx := context.Background()
	good := seedCompletedRun(t, d, s, 1)
	none, err := runsRegressionHandler(d)(ctx, runsRegressionInput{RunID: good.ID.String()}, operator)
	if err != nil || none.State != "noBaseline" {
		t.Fatalf("%+v %v", none, err)
	}
	if _, err := baselinesSaveHandler(d)(ctx, baselinesSaveInput{RunID: good.ID.String(), Name: "b"}, operator); err != nil {
		t.Fatal(err)
	}
	compared, err := runsRegressionHandler(d)(ctx, runsRegressionInput{RunID: seedCompletedRun(t, d, s, 1).ID.String()}, operator)
	if err != nil || compared.State != "compared" || compared.HasRegression {
		t.Fatalf("nothing regressed: %+v %v", compared, err)
	}
	for name, v := range map[string]RegressionView{"noBaseline": none, "compared": compared} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		for key, open := range map[string]byte{"dimensionDeltas": '{', "regressedCases": '[', "missingCases": '[', "newCases": '[', "missingDimensions": '['} {
			val, ok := m[key]
			if !ok || len(val) == 0 || val[0] != open {
				t.Errorf("%s: %s must be present and a %c collection, got %s (present=%v)", name, key, open, val, ok)
			}
		}
	}
}
