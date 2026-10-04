package contract

import (
	"context"
	"testing"
)

func TestOverviewStats(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "a", "x")
	theirs := seedSuite(t, d, "app_b", "theirs", "p")
	seedCase(t, d, theirs.ID, "a", "x")
	seedCompletedRun(t, d, theirs, 1)
	ctx := context.Background()

	good := seedCompletedRun(t, d, s, 1)
	if _, err := baselinesSaveHandler(d)(ctx, baselinesSaveInput{RunID: good.ID.String(), Name: "b"}, operator); err != nil {
		t.Fatal(err)
	}
	bad := seedCompletedRun(t, d, s, 0.5)
	seedRunningRun(t, d, s)

	got, err := overviewStatsHandler(d)(ctx, overviewInput{}, operator)
	if err != nil {
		t.Fatal(err)
	}
	if got.SuiteCount != 1 || got.CaseCount != 1 || got.RunCount != 3 || !got.TargetsRegistered {
		t.Fatalf("counts are this app's only: %+v", got)
	}
	if len(got.ActiveRuns) != 1 || len(got.RecentRuns) != 3 {
		t.Fatalf("active and recent: %+v", got)
	}
	if len(got.RecentRegressions) != 1 || got.RecentRegressions[0].RunID != bad.ID.String() {
		t.Fatalf("the regressed run: %+v", got.RecentRegressions)
	}
}

func TestOverviewStatsEmptyAppSendsEmptyLists(t *testing.T) {
	d := newTestDeps(t)
	got, err := overviewStatsHandler(d)(context.Background(), overviewInput{}, operator)
	if err != nil {
		t.Fatal(err)
	}
	if got.RecentRuns == nil || got.ActiveRuns == nil || got.RecentRegressions == nil {
		t.Fatalf("lists must be [] not null: %+v", got)
	}
	if got.SuiteCount != 0 || got.CaseCount != 0 || got.RunCount != 0 {
		t.Fatalf("empty app: %+v", got)
	}
}
