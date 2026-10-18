package storetest

import (
	"context"
	"errors"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalplan"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/store"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

func testEvaluationPlan(t *testing.T, s store.Store) {
	t.Helper()
	ctx := sentinel.WithApp(sentinel.WithTenant(context.Background(), "tenant-a"), "app-a")
	suiteID := id.NewSuiteID()
	p := &evalplan.Plan{ID: id.New("eplan"), SchemaVersion: 1, AppID: "app-a", TenantID: "tenant-a", Suite: suite.Suite{ID: suiteID, AppID: "app-a"}, PassThreshold: 0.7, Cases: []evalplan.Case{{Case: testcase.Case{ID: id.NewCaseID(), SuiteID: suiteID, Expected: "answer"}, Criteria: []evalplan.Criterion{{ID: "quality", Scorer: "exact", Revision: "v1", Required: true, Config: map[string]any{"nested": map[string]any{"value": "old"}}}}}}}
	var err error
	p.Digest, err = p.ContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	if createErr := s.CreateEvaluationPlan(ctx, p); createErr != nil {
		t.Fatal(createErr)
	}
	if createErr := s.CreateEvaluationPlan(ctx, p); !errors.Is(createErr, sentinel.ErrEvaluationPlanExists) {
		t.Fatalf("duplicate: %v", createErr)
	}
	p.Cases[0].Case.Expected = "mutated"
	got, getErr := s.GetEvaluationPlan(ctx, p.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if got.Cases[0].Case.Expected != "answer" {
		t.Fatalf("caller mutation reached storage: %+v", got)
	}
	got.Cases[0].Criteria[0].Config["nested"] = map[string]any{"value": "changed"}
	again, getErr := s.GetEvaluationPlan(ctx, p.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if _, encodeErr := evalplan.Encode(ctx, again); encodeErr != nil {
		t.Fatalf("read mutation changed immutable bytes: %v", encodeErr)
	}
	for _, other := range []context.Context{context.Background(), sentinel.WithApp(ctx, "other"), sentinel.WithTenant(ctx, "other")} {
		if _, readErr := s.GetEvaluationPlan(other, p.ID); !errors.Is(readErr, sentinel.ErrEvaluationPlanNotFound) {
			t.Errorf("foreign scope read: %v", readErr)
		}
	}
	// IDs are globally unique in every backend, even across scope partitions.
	other := sentinel.WithTenant(ctx, "other")
	again.TenantID = "other"
	again.Digest, err = again.ContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	if createErr := s.CreateEvaluationPlan(other, again); !errors.Is(createErr, sentinel.ErrEvaluationPlanExists) {
		t.Errorf("cross-scope duplicate ID: %v", createErr)
	}
	p.ID = id.New("eplan")
	if createErr := s.CreateEvaluationPlan(ctx, p); createErr == nil {
		t.Fatal("modified plan with stale digest persisted")
	}
}
