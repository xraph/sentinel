package contract

import (
	"context"
	"testing"

	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/scorer"
)

func TestConfigGet(t *testing.T) {
	deps := newTestDeps(t, engine.WithScorer(scorer.Descriptor{Name: "judge", Description: "LLM judge", Dimension: "persona", UsesLLM: true},
		func(map[string]any) (scorer.Scorer, error) {
			return scorer.FromFunc("judge", func(context.Context, *scorer.Input) (*scorer.Output, error) { return &scorer.Output{Score: 1}, nil }), nil
		}))
	got, err := configGetHandler(deps)(context.Background(), configGetInput{}, operator)
	if err != nil {
		t.Fatal(err)
	}
	if got.PassThreshold != 0.7 || got.RegressionThreshold != 0.05 || got.Concurrency != 4 {
		t.Fatalf("effective config: %+v", got)
	}
	if len(got.Targets) != 1 || got.Targets[0].Name != "echo" || got.Targets[0].Description != "returns its input" {
		t.Fatalf("targets: %+v", got.Targets)
	}
	var judge, regex *ScorerView
	for i := range got.Scorers {
		switch got.Scorers[i].Name {
		case "judge":
			judge = &got.Scorers[i]
		case "regex":
			regex = &got.Scorers[i]
		}
	}
	if judge == nil || !judge.UsesLLM || judge.Dimension != "persona" {
		t.Fatalf("judge scorer: %+v", judge)
	}
	if regex == nil || !regex.RequiresConfig {
		t.Fatalf("regex must be flagged as needing config: %+v", regex)
	}
}

func TestConfigGetRefusesWithoutAnApp(t *testing.T) {
	deps := newTestDeps(t)
	deps.DashboardAppID = ""
	_, err := configGetHandler(deps)(context.Background(), configGetInput{}, operator)
	wantCode(t, err, "PERMISSION_DENIED")
}
