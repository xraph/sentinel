package contract

import (
	"context"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/suite"
)

func seedSuite(t *testing.T, d Deps, app, name, prompt string) *suite.Suite {
	t.Helper()
	s := &suite.Suite{Entity: sentinel.NewEntity(), ID: id.NewSuiteID(), Name: name, AppID: app, SystemPrompt: prompt, Model: "m", Metadata: map[string]any{}}
	if err := d.Engine.CreateSuite(context.Background(), s); err != nil {
		t.Fatalf("seed suite: %v", err)
	}
	return s
}

func strPtr(s string) *string { return &s }

func TestSuitesListIsScopedToTheApp(t *testing.T) {
	d := newTestDeps(t)
	mine := seedSuite(t, d, testApp, "mine", "answer briefly")
	seedSuite(t, d, "app_b", "theirs", "p")

	got, err := suitesListHandler(d)(context.Background(), suitesListInput{}, operator)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].ID != mine.ID.String() {
		t.Fatalf("app_a must see exactly its own suite, by identity: %+v", got.Items)
	}
}

// Review focus 2.
func TestSuitesDetailHidesOtherApps(t *testing.T) {
	d := newTestDeps(t)
	theirs := seedSuite(t, d, "app_b", "theirs", "p")
	_, err := suitesDetailHandler(d)(context.Background(), suiteRef{SuiteID: theirs.ID.String()}, operator)
	wantCode(t, err, "NOT_FOUND")
	_, missing := suitesDetailHandler(d)(context.Background(), suiteRef{SuiteID: id.NewSuiteID().String()}, operator)
	wantCode(t, missing, "NOT_FOUND")
	_, garbage := suitesDetailHandler(d)(context.Background(), suiteRef{SuiteID: "nonsense"}, operator)
	wantCode(t, garbage, "NOT_FOUND")
}

func TestSuitesCreateUpdateDelete(t *testing.T) {
	d := newTestDeps(t)
	ctx := context.Background()
	created, err := suitesCreateHandler(d)(ctx, suitesCreateInput{Name: "support", SystemPrompt: "be kind"}, operator)
	if err != nil {
		t.Fatal(err)
	}
	if created.PromptSource != "suite" || created.CaseCount != 0 || created.CurrentBaseline != nil {
		t.Fatalf("new suite: %+v", created)
	}
	stored, err := d.Engine.GetSuite(ctx, mustSuiteID(t, created.ID))
	if err != nil || stored.AppID != testApp {
		t.Fatalf("a create stamps the resolved app: %+v %v", stored, err)
	}

	_, dup := suitesCreateHandler(d)(ctx, suitesCreateInput{Name: "support"}, operator)
	wantCode(t, dup, "CONFLICT")
	_, blank := suitesCreateHandler(d)(ctx, suitesCreateInput{Name: "  "}, operator)
	wantCode(t, blank, "BAD_REQUEST")

	updated, err := suitesUpdateHandler(d)(ctx, suitesUpdateInput{SuiteID: created.ID, Description: strPtr("triage")}, operator)
	if err != nil || updated.Description != "triage" || updated.SystemPrompt != "be kind" {
		t.Fatalf("update changes only what it names: %+v %v", updated, err)
	}

	if _, err := suitesDeleteHandler(d)(ctx, suiteRef{SuiteID: created.ID}, operator); err != nil {
		t.Fatal(err)
	}
	_, gone := suitesDetailHandler(d)(ctx, suiteRef{SuiteID: created.ID}, operator)
	wantCode(t, gone, "NOT_FOUND")
}

func TestSuitesUpdateAndDeleteRefuseOtherApps(t *testing.T) {
	d := newTestDeps(t)
	theirs := seedSuite(t, d, "app_b", "theirs", "p")
	_, err := suitesUpdateHandler(d)(context.Background(), suitesUpdateInput{SuiteID: theirs.ID.String(), Name: strPtr("x")}, operator)
	wantCode(t, err, "NOT_FOUND")
	_, err = suitesDeleteHandler(d)(context.Background(), suiteRef{SuiteID: theirs.ID.String()}, operator)
	wantCode(t, err, "NOT_FOUND")
	if _, still := d.Engine.GetSuite(context.Background(), theirs.ID); still != nil {
		t.Fatal("another app's suite must survive a refused delete")
	}
}

func mustSuiteID(t *testing.T, s string) id.SuiteID {
	t.Helper()
	v, err := id.ParseSuiteID(s)
	if err != nil {
		t.Fatalf("parse suite id %q: %v", s, err)
	}
	return v
}
