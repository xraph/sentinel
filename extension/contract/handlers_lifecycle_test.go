package contract

import (
	"context"
	"testing"
	"time"
)

func TestRunsStartReturnsAtOnceAndCompletes(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "a", "x")
	ctx := context.Background()

	run, err := runsStartHandler(d)(ctx, runsStartInput{SuiteID: s.ID.String(), Target: "echo", Scorers: []string{"json_valid"}}, operator)
	if err != nil || run.ID == "" || run.TotalCases != 1 || run.Settings.Target != "echo" {
		t.Fatalf("start: %+v %v", run, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, err := runsDetailHandler(d)(ctx, runRef{RunID: run.ID}, operator)
		if err == nil && got.Run.State == "completed" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the started run never completed")
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
	}
	_, err := runsStartHandler(d)(ctx, runsStartInput{SuiteID: theirs.ID.String(), Target: "echo", Scorers: []string{"json_valid"}}, operator)
	wantCode(t, err, "NOT_FOUND")
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
