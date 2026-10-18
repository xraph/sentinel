package engine_test

import (
	"context"
	"errors"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/evalplan"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/store/memory"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

func TestFreezeEvaluationPinsCaseAndScorer(t *testing.T) {
	s := memory.New()
	ctx := sentinel.WithApp(sentinel.WithTenant(context.Background(), "tenant-a"), "app-a")
	su := &suite.Suite{ID: id.NewSuiteID(), Name: "frozen", AppID: "app-a", SystemPrompt: "original"}
	if err := s.CreateSuite(ctx, su); err != nil {
		t.Fatal(err)
	}
	tc := &testcase.Case{ID: id.NewCaseID(), SuiteID: su.ID, Name: "one", Input: "question", Expected: "answer", Context: map[string]any{"nested": map[string]any{"value": "old"}}, Scorers: []testcase.ScorerConfig{{Name: "exact"}}}
	if err := s.CreateCase(ctx, tc); err != nil {
		t.Fatal(err)
	}
	e, err := engine.New(engine.WithStore(s))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := e.FreezeEvaluation(ctx, engine.FreezeRequest{SuiteID: su.ID})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Digest == "" || len(plan.Cases) != 1 || len(plan.Cases[0].Criteria) != 1 || plan.Cases[0].Criteria[0].Revision == "" {
		t.Fatalf("incomplete plan: %+v", plan)
	}
	tc.Expected = "changed"
	if err = s.UpdateCase(ctx, tc); err != nil {
		t.Fatal(err)
	}
	plan.Cases[0].Case.Expected = "caller mutation"
	stored, err := s.GetEvaluationPlan(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Cases[0].Case.Expected != "answer" {
		t.Fatalf("frozen case drifted: %+v", stored)
	}
	other := sentinel.WithTenant(ctx, "tenant-b")
	if _, err = s.GetEvaluationPlan(other, plan.ID); !errors.Is(err, sentinel.ErrEvaluationPlanNotFound) {
		t.Fatalf("foreign tenant: %v", err)
	}
	if _, err = e.FreezeEvaluation(context.Background(), engine.FreezeRequest{SuiteID: su.ID}); err == nil {
		t.Fatal("missing app accepted")
	}
}

func TestFreezeEvaluationRejectsUnversionedAndForeignInputs(t *testing.T) {
	s := memory.New()
	ctx := sentinel.WithApp(context.Background(), "app-a")
	su := &suite.Suite{ID: id.NewSuiteID(), Name: "suite", AppID: "app-a"}
	if err := s.CreateSuite(ctx, su); err != nil {
		t.Fatal(err)
	}
	tc := &testcase.Case{ID: id.NewCaseID(), SuiteID: su.ID, Name: "case", Input: "input"}
	if err := s.CreateCase(ctx, tc); err != nil {
		t.Fatal(err)
	}
	e, err := engine.New(engine.WithStore(s))
	if err != nil {
		t.Fatal(err)
	}
	e.Scorers().Register("custom", func(map[string]any) (scorer.Scorer, error) { return &scorer.ExactScorer{}, nil })
	req := engine.FreezeRequest{SuiteID: su.ID, Scorers: []evalplan.Criterion{{ID: "quality", Scorer: "custom", Required: true}}}
	if _, err = e.FreezeEvaluation(ctx, req); err == nil {
		t.Fatal("unversioned scorer accepted")
	}
	req.Scorers[0].Scorer = "exact"
	req.CaseIDs = []id.CaseID{id.NewCaseID()}
	if _, err = e.FreezeEvaluation(ctx, req); err == nil {
		t.Fatal("foreign case accepted")
	}
	req.CaseIDs = nil
	if _, err = e.FreezeEvaluation(sentinel.WithApp(ctx, "other"), req); err == nil {
		t.Fatal("foreign app suite accepted")
	}
}
