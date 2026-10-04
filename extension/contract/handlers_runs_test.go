package contract

import (
	"context"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/target"
)

func fixedScorer(score float64) scorer.Scorer {
	return scorer.FromFunc("fixed", func(context.Context, *scorer.Input) (*scorer.Output, error) {
		return &scorer.Output{Score: score, Passed: score >= 0.7, Dimension: "skill"}, nil
	})
}

// seedCompletedRun runs the suite synchronously with a scorer that gives
// every case the same score.
func seedCompletedRun(t *testing.T, d Deps, s *suite.Suite, score float64) *evalrun.Run {
	t.Helper()
	res, err := d.Engine.RunEval(context.Background(), &engine.RunConfig{
		SuiteID: s.ID,
		Target:  target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return "echo: " + in, nil }),
		Scorers: []scorer.Scorer{fixedScorer(score)},
	})
	if err != nil {
		t.Fatalf("seed run: %v", err)
	}
	return res.Run
}

func TestRunsListPagesAndCountsFromResults(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "a", "x")
	seedCase(t, d, s.ID, "b", "y")
	for i := 0; i < 3; i++ {
		seedCompletedRun(t, d, s, 1)
	}
	ctx := context.Background()

	page, err := runsListHandler(d)(ctx, runsListInput{SuiteID: s.ID.String(), Limit: 2}, operator)
	if err != nil || len(page.Items) != 2 || !page.HasMore {
		t.Fatalf("first page: %+v %v", page, err)
	}
	r := page.Items[0]
	if r.SuiteName != "s" || r.CompletedCases != 2 || r.Errored != 0 || r.Passed != 2 || r.Settings.Target != "echo" {
		t.Fatalf("run view: %+v", r)
	}
	last, err := runsListHandler(d)(ctx, runsListInput{SuiteID: s.ID.String(), Limit: 2, Offset: 2}, operator)
	if err != nil || len(last.Items) != 1 || last.HasMore {
		t.Fatalf("last page: %+v %v", last, err)
	}
	// Review focus 3.
	past, err := runsListHandler(d)(ctx, runsListInput{SuiteID: s.ID.String(), Limit: 2, Offset: 4}, operator)
	if err != nil || len(past.Items) != 0 || past.HasMore {
		t.Fatalf("past the end: %+v %v", past, err)
	}
	_, bad := runsListHandler(d)(ctx, runsListInput{State: "exploded"}, operator)
	wantCode(t, bad, "BAD_REQUEST")
}

func TestRunsListIsScopedToTheApp(t *testing.T) {
	d := newTestDeps(t)
	mine := seedSuite(t, d, testApp, "mine", "p")
	seedCase(t, d, mine.ID, "a", "x")
	theirs := seedSuite(t, d, "app_b", "theirs", "p")
	seedCase(t, d, theirs.ID, "a", "x")
	wantRun := seedCompletedRun(t, d, mine, 1)
	seedCompletedRun(t, d, theirs, 1)

	got, err := runsListHandler(d)(context.Background(), runsListInput{}, operator)
	if err != nil || len(got.Items) != 1 || got.Items[0].ID != wantRun.ID.String() {
		t.Fatalf("app_a must see exactly its own run: %+v %v", got.Items, err)
	}
	_, err = runsListHandler(d)(context.Background(), runsListInput{SuiteID: theirs.ID.String()}, operator)
	wantCode(t, err, "NOT_FOUND")
}

func TestRunsResultsAndResultDetail(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "secret")
	seedCase(t, d, s.ID, "plain", "x")
	seedLeakageCase(t, d, s.ID, "secret")
	// Score 0, not 0.5: the leakage case also runs its own not_contains
	// scorer, which passes against the echo target, so a 0.5 run scorer
	// would average to 0.75 and pass.
	run := seedCompletedRun(t, d, s, 0)
	ctx := context.Background()

	rows, err := runsResultsHandler(d)(ctx, runsResultsInput{RunID: run.ID.String()}, operator)
	if err != nil || len(rows.Items) != 2 || rows.Counts.Fail != 2 {
		t.Fatalf("results: %+v %v", rows, err)
	}
	var red *ResultRow
	for i := range rows.Items {
		if rows.Items[i].RedTeam != nil {
			red = &rows.Items[i]
		}
	}
	if red == nil || red.RedTeam.AttackType != "leakage" {
		t.Fatalf("the leakage result must be marked red team: %+v", rows.Items)
	}
	filtered, _ := runsResultsHandler(d)(ctx, runsResultsInput{RunID: run.ID.String(), Status: "pass"}, operator)
	if len(filtered.Items) != 0 || filtered.Counts.Fail != 2 {
		t.Fatalf("a status filter narrows items but counts cover the run: %+v", filtered)
	}

	detail, err := resultsDetailHandler(d)(ctx, resultRef{RunID: run.ID.String(), ResultID: red.ID}, operator)
	if err != nil || detail.Output == "" || detail.OutputLength == 0 || len(detail.ScorerResults) == 0 {
		t.Fatalf("result detail: %+v %v", detail, err)
	}
}

func TestRunReadsRefuseOtherApps(t *testing.T) {
	d := newTestDeps(t)
	theirs := seedSuite(t, d, "app_b", "theirs", "p")
	seedCase(t, d, theirs.ID, "a", "x")
	run := seedCompletedRun(t, d, theirs, 1)
	ctx := context.Background()
	_, err := runsResultsHandler(d)(ctx, runsResultsInput{RunID: run.ID.String()}, operator)
	wantCode(t, err, "NOT_FOUND")
	_, err = resultsDetailHandler(d)(ctx, resultRef{RunID: run.ID.String(), ResultID: "x"}, operator)
	wantCode(t, err, "NOT_FOUND")

	// Another app's run and a run that never existed answer identically.
	_, other := runsResultsHandler(d)(ctx, runsResultsInput{RunID: run.ID.String()}, operator)
	_, missing := runsResultsHandler(d)(ctx, runsResultsInput{RunID: id.NewEvalRunID().String()}, operator)
	wantSameNotFound(t, missing, other)
}

// seedRunningRun writes a run row in the running state directly, without
// evaluating anything.
func seedRunningRun(t *testing.T, d Deps, s *suite.Suite) *evalrun.Run {
	t.Helper()
	r := &evalrun.Run{Entity: sentinel.NewEntity(), ID: id.NewEvalRunID(), SuiteID: s.ID, Model: "m", AppID: s.AppID,
		TotalCases: 1, State: evalrun.StateRunning, Config: map[string]any{}, DimensionScores: map[string]float64{}}
	if err := d.Engine.Store().CreateRun(context.Background(), r); err != nil {
		t.Fatalf("seed running run: %v", err)
	}
	return r
}
