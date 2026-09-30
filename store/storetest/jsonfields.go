package storetest

import (
	"math"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/store"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-5 }

func testJSONFieldsRoundTrip(t *testing.T, s store.Store) {
	// Nil everywhere: what the engine writes today for a run it has just
	// started, a result whose target call failed, and an imported case.
	nilSuite := &suite.Suite{Entity: sentinel.NewEntity(), ID: id.NewSuiteID(), Name: "nil-suite", AppID: "app_a"}
	if err := s.CreateSuite(bg(), nilSuite); err != nil {
		t.Fatalf("create suite with nil metadata: %v", err)
	}
	nilCase := &testcase.Case{Entity: sentinel.NewEntity(), ID: id.NewCaseID(), SuiteID: nilSuite.ID, Name: "nil-case", Input: "x", ScenarioType: testcase.ScenarioStandard}
	if err := s.CreateCase(bg(), nilCase); err != nil {
		t.Fatalf("create case with nil tags, scorers, context, metadata: %v", err)
	}
	nilRun := &evalrun.Run{Entity: sentinel.NewEntity(), ID: id.NewEvalRunID(), SuiteID: nilSuite.ID, Model: "m", AppID: "app_a", State: evalrun.StateRunning}
	if err := s.CreateRun(bg(), nilRun); err != nil {
		t.Fatalf("create run with nil config and dimension scores: %v", err)
	}
	nilResult := &evalrun.Result{Entity: sentinel.NewEntity(), ID: id.NewEvalResultID(), RunID: nilRun.ID, CaseID: nilCase.ID, CaseName: "nil-case", Status: evalrun.StatusError, Error: "target failed"}
	if err := s.CreateResult(bg(), nilResult); err != nil {
		t.Fatalf("create result with nil scorer results and dimension scores: %v", err)
	}
	nilBaseline := &baseline.Baseline{ID: id.NewBaselineID(), SuiteID: nilSuite.ID, RunID: nilRun.ID, Name: "nil-baseline"}
	if err := s.SaveBaseline(bg(), nilBaseline); err != nil {
		t.Fatalf("save baseline with nil results and dimension scores: %v", err)
	}
	if _, err := s.GetRun(bg(), nilRun.ID); err != nil {
		t.Fatalf("read back nil run: %v", err)
	}

	// Populated: what the dashboard's writes and the new run settings put there.
	su := mustSuite(t, s)
	tc := &testcase.Case{
		Entity: sentinel.NewEntity(), ID: id.NewCaseID(), SuiteID: su.ID, Name: "full", Input: "in",
		ScenarioType: testcase.ScenarioTraitProbe,
		Scorers:      []testcase.ScorerConfig{{Name: "contains", Config: map[string]any{"substring": "x"}}},
		Tags:         []string{"redteam", "leakage"},
		Context:      map[string]any{"attack_type": "leakage"},
		Metadata:     map[string]any{"owner": "qa"},
	}
	if err := s.CreateCase(bg(), tc); err != nil {
		t.Fatalf("create populated case: %v", err)
	}
	gotCase, err := s.GetCase(bg(), tc.ID)
	if err != nil {
		t.Fatalf("get case: %v", err)
	}
	if len(gotCase.Scorers) != 1 || gotCase.Scorers[0].Name != "contains" || gotCase.Scorers[0].Config["substring"] != "x" {
		t.Errorf("case scorers did not round-trip: %+v", gotCase.Scorers)
	}
	if len(gotCase.Tags) != 2 || gotCase.Tags[1] != "leakage" {
		t.Errorf("case tags did not round-trip: %v", gotCase.Tags)
	}
	if gotCase.Context["attack_type"] != "leakage" || gotCase.Metadata["owner"] != "qa" {
		t.Errorf("case context or metadata did not round-trip: %v %v", gotCase.Context, gotCase.Metadata)
	}

	// The settings a run records when it starts, exactly as the engine
	// writes them. Every backend decodes numbers and arrays into its own
	// types; SettingsFrom must still read every field back.
	passThreshold, regressionThreshold, concurrency := 0.7, 0.05, 4
	settings := evalrun.Settings{
		PassThreshold: &passThreshold, RegressionThreshold: &regressionThreshold, Concurrency: &concurrency,
		Target: "llm:x", Scorers: []string{"exact", "contains"}, Model: "m", PromptVersionID: id.NewPromptVersionID().String(),
	}
	run := &evalrun.Run{
		Entity: sentinel.NewEntity(), ID: id.NewEvalRunID(), SuiteID: su.ID, Model: "m", AppID: "app_a",
		State:           evalrun.StateRunning,
		Config:          settings.Config(),
		DimensionScores: map[string]float64{"skill": 0.25},
	}
	if err = s.CreateRun(bg(), run); err != nil {
		t.Fatalf("create populated run: %v", err)
	}
	gotRun, err := s.GetRun(bg(), run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	gotSettings := evalrun.SettingsFrom(gotRun.Config)
	if gotSettings.PassThreshold == nil || !near(*gotSettings.PassThreshold, 0.7) {
		t.Errorf("pass threshold did not round-trip: %v (config %v)", gotSettings.PassThreshold, gotRun.Config)
	}
	if gotSettings.RegressionThreshold == nil || !near(*gotSettings.RegressionThreshold, 0.05) {
		t.Errorf("regression threshold did not round-trip: %v (config %v)", gotSettings.RegressionThreshold, gotRun.Config)
	}
	if gotSettings.Concurrency == nil || *gotSettings.Concurrency != 4 {
		t.Errorf("concurrency did not round-trip: %v (config %v)", gotSettings.Concurrency, gotRun.Config)
	}
	if gotSettings.Target != "llm:x" || gotSettings.Model != "m" || gotSettings.PromptVersionID != settings.PromptVersionID {
		t.Errorf("target, model or prompt version did not round-trip: %+v", gotSettings)
	}
	if len(gotSettings.Scorers) != 2 || gotSettings.Scorers[0] != "exact" || gotSettings.Scorers[1] != "contains" {
		t.Errorf("scorers did not round-trip: %v (config %v)", gotSettings.Scorers, gotRun.Config)
	}
	if !near(gotRun.DimensionScores["skill"], 0.25) {
		t.Errorf("run dimension scores did not round-trip: %v", gotRun.DimensionScores)
	}

	res := &evalrun.Result{
		Entity: sentinel.NewEntity(), ID: id.NewEvalResultID(), RunID: run.ID, CaseID: tc.ID, CaseName: "full",
		Status: evalrun.StatusPass, Score: 0.9, Output: "out",
		ScorerResults:   []evalrun.ScorerResult{{ScorerName: "exact", Score: 1, Passed: true, Reason: "match", Dimension: "skill"}},
		DimensionScores: map[string]float64{"skill": 0.5},
		RunTrace:        &evalrun.RunTrace{Steps: []evalrun.StepTrace{{Index: 0, Type: "plan", Output: "p"}}},
	}
	if err = s.CreateResult(bg(), res); err != nil {
		t.Fatalf("create populated result: %v", err)
	}
	results, err := s.ListResults(bg(), run.ID)
	if err != nil || len(results) != 1 {
		t.Fatalf("list results: %v (%d)", err, len(results))
	}
	got := results[0]
	if len(got.ScorerResults) != 1 || got.ScorerResults[0].Reason != "match" || got.ScorerResults[0].Dimension != "skill" {
		t.Errorf("scorer results did not round-trip: %+v", got.ScorerResults)
	}
	if got.RunTrace == nil || len(got.RunTrace.Steps) != 1 || got.RunTrace.Steps[0].Type != "plan" {
		t.Errorf("run trace did not round-trip: %+v", got.RunTrace)
	}

	b := &baseline.Baseline{
		ID: id.NewBaselineID(), SuiteID: su.ID, RunID: run.ID, Name: "full", PassRate: 0.5, AvgScore: 0.5,
		Results:         []baseline.Result{{CaseID: tc.ID, CaseName: "full", Score: 0.9, Status: "pass", DimensionScores: map[string]float64{"skill": 0.5}}},
		DimensionScores: map[string]float64{"skill": 0.5},
		IsCurrent:       true,
	}
	if err = s.SaveBaseline(bg(), b); err != nil {
		t.Fatalf("save populated baseline: %v", err)
	}
	gotB, err := s.GetBaseline(bg(), b.ID)
	if err != nil {
		t.Fatalf("get baseline: %v", err)
	}
	if len(gotB.Results) != 1 || gotB.Results[0].CaseID.String() != tc.ID.String() || !near(gotB.DimensionScores["skill"], 0.5) {
		t.Errorf("baseline did not round-trip: %+v", gotB)
	}
}
