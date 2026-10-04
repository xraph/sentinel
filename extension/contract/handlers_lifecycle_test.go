package contract

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/redteam"
	"github.com/xraph/sentinel/testcase"
)

// waitCompleted polls runs.detail until the run is completed.
func waitCompleted(t *testing.T, d Deps, runID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, err := runsDetailHandler(d)(context.Background(), runRef{RunID: runID}, operator)
		if err == nil && got.Run.State == "completed" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s never completed", runID)
}

// seedResult stores one result for the case in the run, bypassing the
// runner so a test can pick the status.
func seedResult(t *testing.T, d Deps, run *evalrun.Run, tc *testcase.Case, status evalrun.ResultStatus) {
	t.Helper()
	res := &evalrun.Result{Entity: sentinel.NewEntity(), ID: id.NewEvalResultID(), RunID: run.ID, CaseID: tc.ID, CaseName: tc.Name,
		Status: status, ScorerResults: []evalrun.ScorerResult{}}
	if err := d.Engine.Store().CreateResult(context.Background(), res); err != nil {
		t.Fatalf("seed result: %v", err)
	}
}

// wantNoRuns fails if the app has any run, which is how a refused start is
// proved to have left nothing behind.
func wantNoRuns(t *testing.T, d Deps, where string, apps ...string) {
	t.Helper()
	for _, app := range apps {
		runs, err := d.Engine.ListRuns(context.Background(), &evalrun.ListFilter{AppID: app})
		if err != nil || len(runs) != 0 {
			t.Fatalf("%s: app %s has %d runs after a refused start (%v)", where, app, len(runs), err)
		}
	}
}

func TestRunsStartReturnsAtOnceAndCompletes(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "a", "x")
	ctx := context.Background()

	run, err := runsStartHandler(d)(ctx, runsStartInput{SuiteID: s.ID.String(), Target: "echo", Scorers: []string{"json_valid"}}, operator)
	if err != nil || run.ID == "" || run.TotalCases != 1 || run.Settings.Target != "echo" {
		t.Fatalf("start: %+v %v", run, err)
	}
	waitCompleted(t, d, run.ID)
}

func TestRunsStartRefusals(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "a", "x")
	empty := seedSuite(t, d, testApp, "empty", "p")
	theirs := seedSuite(t, d, "app_b", "theirs", "p")
	ctx := context.Background()
	for name, in := range map[string]runsStartInput{
		"unknown target":     {SuiteID: s.ID.String(), Target: "nope", Scorers: []string{"json_valid"}},
		"unknown scorer":     {SuiteID: s.ID.String(), Target: "echo", Scorers: []string{"nope"}},
		"no scorers":         {SuiteID: s.ID.String(), Target: "echo"},
		"misconfigured":      {SuiteID: s.ID.String(), Target: "echo", Scorers: []string{"regex"}},
		"suite with no case": {SuiteID: empty.ID.String(), Target: "echo", Scorers: []string{"json_valid"}},
	} {
		_, err := runsStartHandler(d)(ctx, in, operator)
		if err == nil {
			t.Errorf("%s: want BAD_REQUEST", name)
			continue
		}
		wantCode(t, err, "BAD_REQUEST")
		wantNoRuns(t, d, name, testApp, "app_b")
	}
	_, err := runsStartHandler(d)(ctx, runsStartInput{SuiteID: theirs.ID.String(), Target: "echo", Scorers: []string{"json_valid"}}, operator)
	wantCode(t, err, "NOT_FOUND")
	wantNoRuns(t, d, "another app's suite", testApp, "app_b")
}

func TestRunsCancel(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	running := seedRunningRun(t, d, s)
	ctx := context.Background()
	got, err := runsCancelHandler(d)(ctx, runRef{RunID: running.ID.String()}, operator)
	if err != nil || got.State != "cancelled" {
		t.Fatalf("cancel: %+v %v", got, err)
	}
	_, again := runsCancelHandler(d)(ctx, runRef{RunID: running.ID.String()}, operator)
	wantCode(t, again, "CONFLICT")
}

func TestRedTeamGenerateAndReport(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "the secret")
	ctx := context.Background()
	gen, err := redteamGenerateHandler(d)(ctx, redteamGenerateInput{SuiteID: s.ID.String(), AttackTypes: []string{"injection", "injection", "leakage"}, Count: 2}, operator)
	if err != nil || gen.Created != 4 || gen.Cap != 5 {
		t.Fatalf("duplicate types are collapsed: %+v %v", gen, err)
	}
	_, bad := redteamGenerateHandler(d)(ctx, redteamGenerateInput{SuiteID: s.ID.String(), AttackTypes: []string{"injection"}, Count: 0}, operator)
	wantCode(t, bad, "BAD_REQUEST")

	// Every generated case also runs its own not_contains scorer, which
	// passes against the echo target; a run scorer of 0 makes each case
	// average 0.5 and fail, so every attack counts as bypassed.
	run := seedCompletedRun(t, d, s, 0)
	report, err := redteamReportHandler(d)(ctx, runRef{RunID: run.ID.String()}, operator)
	if err != nil || report == nil || report.Total != 4 || report.Bypassed != 4 || len(report.ByType) != 2 {
		t.Fatalf("report: %+v %v", report, err)
	}

	plain := seedSuite(t, d, testApp, "plain", "p")
	seedCase(t, d, plain.ID, "a", "x")
	none, err := redteamReportHandler(d)(ctx, runRef{RunID: seedCompletedRun(t, d, plain, 1).ID.String()}, operator)
	if err != nil || none != nil {
		t.Fatalf("a run with no red-team cases has no report: %+v %v", none, err)
	}
}

func TestRunsCancelOfACompletedRunConflicts(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "a", "x")
	done := seedCompletedRun(t, d, s, 1)
	_, err := runsCancelHandler(d)(context.Background(), runRef{RunID: done.ID.String()}, operator)
	wantCode(t, err, "CONFLICT")
}

func TestLifecycleHidesOtherApps(t *testing.T) {
	d := newTestDeps(t)
	ctx := context.Background()
	theirSuite := seedSuite(t, d, "app_b", "theirs", "p")
	theirRun := seedRunningRun(t, d, theirSuite)

	_, missing := runsCancelHandler(d)(ctx, runRef{RunID: id.NewEvalRunID().String()}, operator)
	_, other := runsCancelHandler(d)(ctx, runRef{RunID: theirRun.ID.String()}, operator)
	wantSameNotFound(t, missing, other)
	if got, _ := d.Engine.GetRun(ctx, theirRun.ID); got == nil || got.State != evalrun.StateRunning {
		t.Fatalf("a refused cancel must leave the other app's run running: %+v", got)
	}

	_, missing = redteamReportHandler(d)(ctx, runRef{RunID: id.NewEvalRunID().String()}, operator)
	_, other = redteamReportHandler(d)(ctx, runRef{RunID: theirRun.ID.String()}, operator)
	wantSameNotFound(t, missing, other)

	_, missing = redteamGenerateHandler(d)(ctx, redteamGenerateInput{SuiteID: id.NewSuiteID().String(), AttackTypes: []string{"injection"}, Count: 1}, operator)
	_, other = redteamGenerateHandler(d)(ctx, redteamGenerateInput{SuiteID: theirSuite.ID.String(), AttackTypes: []string{"injection"}, Count: 1}, operator)
	wantSameNotFound(t, missing, other)
	cases, err := d.Engine.ListCases(ctx, theirSuite.ID)
	if err != nil || len(cases) != 0 {
		t.Fatalf("a refused generate must create nothing in the other app: %d %v", len(cases), err)
	}
}

func TestRedTeamReportCountsEachOutcome(t *testing.T) {
	d := newTestDeps(t)
	ctx := context.Background()
	s := seedSuite(t, d, testApp, "s", "the secret")
	run := seedRunningRun(t, d, s)

	inj := []*testcase.Case{seedInjectionCase(t, d, s.ID, "the secret"), seedInjectionCase(t, d, s.ID, "the secret"), seedInjectionCase(t, d, s.ID, "the secret")}
	seedInjectionCase(t, d, s.ID, "the secret") // no result in this run: not counted
	leak := []*testcase.Case{seedLeakageCase(t, d, s.ID, "the secret"), seedLeakageCase(t, d, s.ID, "the secret"), seedLeakageCase(t, d, s.ID, "the secret")}
	odd := &testcase.Case{Entity: sentinel.NewEntity(), ID: id.NewCaseID(), SuiteID: s.ID, Name: "odd", Input: "x",
		ScenarioType: testcase.ScenarioStandard, Scorers: []testcase.ScorerConfig{}, Tags: []string{"redteam"},
		Context: map[string]any{}, Metadata: map[string]any{}}
	if err := d.Engine.CreateCase(ctx, odd); err != nil {
		t.Fatal(err)
	}
	plain := &testcase.Case{Entity: sentinel.NewEntity(), ID: id.NewCaseID(), SuiteID: s.ID, Name: "plain", Input: "x",
		ScenarioType: testcase.ScenarioStandard, Scorers: []testcase.ScorerConfig{{Name: "plain_only"}}, Tags: []string{},
		Context: map[string]any{}, Metadata: map[string]any{}}
	if err := d.Engine.CreateCase(ctx, plain); err != nil {
		t.Fatal(err)
	}

	seedResult(t, d, run, inj[0], evalrun.StatusPass)
	seedResult(t, d, run, inj[1], evalrun.StatusFail)
	seedResult(t, d, run, inj[2], evalrun.StatusError)
	seedResult(t, d, run, leak[0], evalrun.StatusFail)
	seedResult(t, d, run, leak[1], evalrun.StatusFail)
	seedResult(t, d, run, leak[2], evalrun.StatusPass)
	seedResult(t, d, run, odd, evalrun.StatusPass)
	seedResult(t, d, run, plain, evalrun.StatusFail) // not a red-team case

	report, err := redteamReportHandler(d)(ctx, runRef{RunID: run.ID.String()}, operator)
	if err != nil || report == nil {
		t.Fatalf("report: %+v %v", report, err)
	}
	want := []RedTeamTally{
		{AttackType: "injection", Total: 3, Bypassed: 1, Unscored: 1},
		{AttackType: "leakage", Total: 3, Bypassed: 2, Unscored: 0},
		{AttackType: "unknown", Total: 1, Bypassed: 0, Unscored: 0},
	}
	if !reflect.DeepEqual(report.ByType, want) {
		t.Fatalf("byType (sorted by attack type, pass counts as neither): got %+v want %+v", report.ByType, want)
	}
	if report.Total != 7 || report.Bypassed != 3 || report.Unscored != 1 {
		t.Fatalf("totals: %+v", report)
	}
	if !reflect.DeepEqual(report.JudgedBy, []string{"not_contains"}) {
		t.Fatalf("judgedBy takes red-team case scorers only, never an ordinary case's: %v", report.JudgedBy)
	}
}

func TestRedTeamReportJudgedByUnionsRunAndCaseScorers(t *testing.T) {
	d := newTestDeps(t)
	ctx := context.Background()
	s := seedSuite(t, d, testApp, "s", "the secret")
	seedInjectionCase(t, d, s.ID, "the secret")

	run, err := runsStartHandler(d)(ctx, runsStartInput{SuiteID: s.ID.String(), Target: "echo", Scorers: []string{"json_valid"}}, operator)
	if err != nil {
		t.Fatal(err)
	}
	waitCompleted(t, d, run.ID)
	report, err := redteamReportHandler(d)(ctx, runRef{RunID: run.ID}, operator)
	if err != nil || report == nil {
		t.Fatalf("report: %+v %v", report, err)
	}
	if !reflect.DeepEqual(report.JudgedBy, []string{"json_valid", "not_contains"}) {
		t.Fatalf("judgedBy is the sorted union of the run's and the cases' scorers: %v", report.JudgedBy)
	}
}

func TestRedTeamReportBeforeAnyResult(t *testing.T) {
	d := newTestDeps(t)
	ctx := context.Background()
	s := seedSuite(t, d, testApp, "s", "the secret")
	seedInjectionCase(t, d, s.ID, "the secret")
	run := seedRunningRun(t, d, s)

	report, err := redteamReportHandler(d)(ctx, runRef{RunID: run.ID.String()}, operator)
	if err != nil || report == nil {
		t.Fatalf("a red-team suite whose run has no results yet still has a report: %+v %v", report, err)
	}
	if report.Total != 0 || report.Bypassed != 0 || report.Unscored != 0 || report.ByType == nil || len(report.ByType) != 0 || report.JudgedBy == nil || len(report.JudgedBy) != 0 {
		t.Fatalf("want zero totals and empty, non-null lists: %+v", report)
	}

	plain := seedSuite(t, d, testApp, "plain", "p")
	seedCase(t, d, plain.ID, "a", "x")
	none, err := redteamReportHandler(d)(ctx, runRef{RunID: seedRunningRun(t, d, plain).ID.String()}, operator)
	if err != nil || none != nil {
		t.Fatalf("a suite with no red-team case has no report: %+v %v", none, err)
	}
}

func TestRedTeamGenerateCountMessageNamesTheCap(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	_, err := redteamGenerateHandler(d)(context.Background(), redteamGenerateInput{SuiteID: s.ID.String(), AttackTypes: []string{"injection"}, Count: redteam.MaxPerType + 1}, operator)
	wantCode(t, err, "BAD_REQUEST")
	var ce *dashcontract.Error
	if !errors.As(err, &ce) || !strings.Contains(ce.Message, fmt.Sprintf("between 1 and %d,", redteam.MaxPerType)) {
		t.Fatalf("the message must name the cap: %v", err)
	}
}
