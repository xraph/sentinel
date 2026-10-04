package contract

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRunsTrendIsOldestFirstAndCompletedOnly(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "a", "x")
	first := seedCompletedRun(t, d, s, 1)
	second := seedCompletedRun(t, d, s, 0.5)
	seedRunningRun(t, d, s)

	got, err := runsTrendHandler(d)(context.Background(), runsTrendInput{SuiteID: s.ID.String()}, operator)
	if err != nil || len(got.Points) != 2 {
		t.Fatalf("trend: %+v %v", got, err)
	}
	if got.Points[0].RunID != first.ID.String() || got.Points[1].RunID != second.ID.String() {
		t.Fatalf("oldest first: %+v", got.Points)
	}
	if got.Points[0].Settings.Target != "echo" || got.Baseline != nil {
		t.Fatalf("settings and no baseline yet: %+v", got)
	}
}

func TestRunsCompare(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	seedCase(t, d, s.ID, "a", "x")
	seedCase(t, d, s.ID, "b", "y")
	a := seedCompletedRun(t, d, s, 1)
	b := seedCompletedRun(t, d, s, 0.5)

	got, err := runsCompareHandler(d)(context.Background(), runsCompareInput{RunID: a.ID.String(), OtherRunID: b.ID.String()}, operator)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Cases) != 2 || got.Cases[0].A == nil || got.Cases[0].B == nil {
		t.Fatalf("pairs: %+v", got.Cases)
	}
	var passRate *MetricDelta
	for i := range got.Deltas {
		if got.Deltas[i].Metric == "pass_rate" {
			passRate = &got.Deltas[i]
		}
	}
	if passRate == nil || passRate.A != 1 || passRate.B != 0 || passRate.Delta != -1 {
		t.Fatalf("pass rate A to B: %+v", passRate)
	}
	if got.DimensionDeltas["skill"] != -0.5 {
		t.Fatalf("dimension deltas over shared dimensions: %+v", got.DimensionDeltas)
	}
}

func TestRunsCompareRefusesOtherSuitesAndApps(t *testing.T) {
	d := newTestDeps(t)
	s1 := seedSuite(t, d, testApp, "one", "p")
	seedCase(t, d, s1.ID, "a", "x")
	s2 := seedSuite(t, d, testApp, "two", "p")
	seedCase(t, d, s2.ID, "a", "x")
	theirs := seedSuite(t, d, "app_b", "theirs", "p")
	seedCase(t, d, theirs.ID, "a", "x")
	ctx := context.Background()
	r1, r2, rt := seedCompletedRun(t, d, s1, 1), seedCompletedRun(t, d, s2, 1), seedCompletedRun(t, d, theirs, 1)
	_, err := runsCompareHandler(d)(ctx, runsCompareInput{RunID: r1.ID.String(), OtherRunID: r2.ID.String()}, operator)
	wantCode(t, err, "BAD_REQUEST")
	_, err = runsCompareHandler(d)(ctx, runsCompareInput{RunID: r1.ID.String(), OtherRunID: rt.ID.String()}, operator)
	wantCode(t, err, "NOT_FOUND")
}

func TestAnalysisCollectionsAreNeverNullOnTheWire(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "p")
	ctx := context.Background()

	empty, err := runsTrendHandler(d)(ctx, runsTrendInput{SuiteID: s.ID.String()}, operator)
	if err != nil {
		t.Fatal(err)
	}
	seedCase(t, d, s.ID, "a", "x")
	a, b := seedCompletedRun(t, d, s, 1), seedCompletedRun(t, d, s, 1)
	cmp, err := runsCompareHandler(d)(ctx, runsCompareInput{RunID: a.ID.String(), OtherRunID: b.ID.String()}, operator)
	if err != nil {
		t.Fatal(err)
	}
	wantShape := func(name string, v any, want map[string]byte) {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		for key, open := range want {
			val, ok := m[key]
			if !ok || len(val) == 0 || val[0] != open {
				t.Errorf("%s: %s must be present and a %c collection, got %s (present=%v)", name, key, open, val, ok)
			}
		}
	}
	wantShape("empty trend", empty, map[string]byte{"points": '['})
	wantShape("compare", cmp, map[string]byte{"deltas": '[', "dimensionDeltas": '{', "dimensionsOnlyIn": '{', "cases": '['})
	var only map[string]json.RawMessage
	raw, _ := json.Marshal(cmp.DimensionsOnlyIn)
	if err := json.Unmarshal(raw, &only); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"a", "b"} {
		if v := only[k]; len(v) == 0 || v[0] != '[' {
			t.Errorf("dimensionsOnlyIn.%s must be a list, got %s", k, v)
		}
	}
	for _, rv := range []RunView{cmp.A, cmp.B} {
		raw, _ := json.Marshal(rv)
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		if v := m["dimensionScores"]; len(v) == 0 || v[0] != '{' {
			t.Errorf("run view dimensionScores must be a map, got %s", v)
		}
	}
}
