package contract

import (
	"context"
	"encoding/json"
	"strings"
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
	return seedRunningRunOf(t, d, s, 1)
}

// seedRunningRunOf is seedRunningRun for a suite of totalCases cases.
func seedRunningRunOf(t *testing.T, d Deps, s *suite.Suite, totalCases int) *evalrun.Run {
	t.Helper()
	r := &evalrun.Run{Entity: sentinel.NewEntity(), ID: id.NewEvalRunID(), SuiteID: s.ID, Model: "m", AppID: s.AppID,
		TotalCases: totalCases, State: evalrun.StateRunning, Config: map[string]any{}, DimensionScores: map[string]float64{}}
	if err := d.Engine.Store().CreateRun(context.Background(), r); err != nil {
		t.Fatalf("seed running run: %v", err)
	}
	return r
}

// A running run's row counters are still zero: its progress lives in the
// results stored so far. Both reads must report that progress, never the
// zeroed row.
func TestRunningRunCountersComeFromStoredResults(t *testing.T) {
	d := newTestDeps(t)
	ctx := context.Background()
	s := seedSuite(t, d, testApp, "s", "p")
	c1 := seedCase(t, d, s.ID, "a", "x")
	seedCase(t, d, s.ID, "b", "y")
	run := seedRunningRunOf(t, d, s, 2)

	res := &evalrun.Result{Entity: sentinel.NewEntity(), ID: id.NewEvalResultID(), RunID: run.ID, CaseID: c1.ID, CaseName: c1.Name,
		Status: evalrun.StatusPass, Score: 0.9, DimensionScores: map[string]float64{"skill": 0.9}, ScorerResults: []evalrun.ScorerResult{}}
	if err := d.Engine.Store().CreateResult(ctx, res); err != nil {
		t.Fatalf("seed result: %v", err)
	}
	// The store stamps CreatedAt itself, so read the stored value back.
	got, err := d.Engine.ListResults(ctx, run.ID)
	if err != nil || len(got) != 1 {
		t.Fatalf("stored results: %d %v", len(got), err)
	}
	stored := got[0].CreatedAt
	if row, _ := d.Engine.GetRun(ctx, run.ID); row.Passed != 0 || row.PassRate != 0 || row.CompletedAt != nil {
		t.Fatalf("the run row must still be zeroed for this test to mean anything: %+v", row)
	}

	check := func(where string, v RunView) {
		t.Helper()
		if v.State != "running" || v.TotalCases != 2 || v.CompletedCases != 1 || v.Passed != 1 || v.Failed != 0 || v.Errored != 0 || v.PassRate != 1 || v.AvgScore != 0.9 {
			t.Fatalf("%s: counters must come from the stored result: %+v", where, v)
		}
		if v.DimensionScores["skill"] != 0.9 {
			t.Fatalf("%s: dimension scores must come from the stored result: %+v", where, v.DimensionScores)
		}
	}
	detail, err := runsDetailHandler(d)(ctx, runRef{RunID: run.ID.String()}, operator)
	if err != nil {
		t.Fatal(err)
	}
	check("runs.detail", detail.Run)
	if detail.Run.LastProgressAt == nil || *detail.Run.LastProgressAt != ts(stored) {
		t.Fatalf("lastProgressAt must be the stored result's timestamp %s: %v", ts(stored), detail.Run.LastProgressAt)
	}
	if detail.Regression.State != "running" {
		t.Fatalf("a running run has no verdict yet: %+v", detail.Regression)
	}
	list, err := runsListHandler(d)(ctx, runsListInput{SuiteID: s.ID.String()}, operator)
	if err != nil || len(list.Items) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
	check("runs.list", list.Items[0])
}

// Each row carries every scorer's verdict, so the results table can show
// which scorer failed a case without opening it. Only the name and the
// verdict: a reason can quote the output, and a red-team output stays behind
// the result page's reveal.
func TestRunsResultsCarryEachScorersVerdict(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "secret")
	seedCase(t, d, s.ID, "plain", "x")
	seedLeakageCase(t, d, s.ID, "secret")
	run := seedCompletedRun(t, d, s, 0)
	ctx := context.Background()

	rows, err := runsResultsHandler(d)(ctx, runsResultsInput{RunID: run.ID.String()}, operator)
	if err != nil || len(rows.Items) != 2 {
		t.Fatalf("results: %+v %v", rows, err)
	}
	stored, err := d.Engine.ListResults(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string][]ScorerVerdict{}
	for _, r := range stored {
		v := []ScorerVerdict{}
		for _, sr := range r.ScorerResults {
			v = append(v, ScorerVerdict{Name: sr.ScorerName, Passed: sr.Passed})
		}
		byID[r.ID.String()] = v
	}
	for _, row := range rows.Items {
		want := byID[row.ID]
		if len(want) == 0 || len(row.Scorers) != len(want) {
			t.Fatalf("%s: verdicts %+v, want %+v", row.CaseName, row.Scorers, want)
		}
		for i := range want {
			if row.Scorers[i] != want[i] {
				t.Fatalf("%s: verdict %d is %+v, want %+v", row.CaseName, i, row.Scorers[i], want[i])
			}
		}
	}
	raw, _ := json.Marshal(rows.Items[0])
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	verdict := m["scorers"].([]any)[0].(map[string]any)
	if len(verdict) != 2 || verdict["name"] == nil || verdict["passed"] == nil {
		t.Fatalf("a verdict is its name and passed, nothing else: %v", verdict)
	}
	if empty, _ := json.Marshal(resultRow(&evalrun.Result{}, nil)); !strings.Contains(string(empty), `"scorers":[]`) {
		t.Fatalf("no scorer results must still send an array: %s", empty)
	}
}
