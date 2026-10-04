package contract

import (
	"context"
	"testing"

	"github.com/xraph/sentinel/id"
)

func TestPromptVersionsLifecycle(t *testing.T) {
	d := newTestDeps(t)
	s := seedSuite(t, d, testApp, "s", "base")
	ctx := context.Background()

	v1, err := promptsCreateHandler(d)(ctx, promptsCreateInput{SuiteID: s.ID.String(), SystemPrompt: "one", Changelog: "first"}, operator)
	if err != nil || v1.Version != 1 || v1.IsCurrent {
		t.Fatalf("v1: %+v %v", v1, err)
	}
	v2, err := promptsCreateHandler(d)(ctx, promptsCreateInput{SuiteID: s.ID.String(), SystemPrompt: "two", MakeCurrent: true}, operator)
	if err != nil || v2.Version != 2 || !v2.IsCurrent {
		t.Fatalf("v2: %+v %v", v2, err)
	}
	_, blank := promptsCreateHandler(d)(ctx, promptsCreateInput{SuiteID: s.ID.String(), SystemPrompt: "  "}, operator)
	wantCode(t, blank, "BAD_REQUEST")

	detail, err := promptsDetailHandler(d)(ctx, versionRef{VersionID: v2.ID}, operator)
	if err != nil || detail.Previous == nil || detail.Previous.ID != v1.ID {
		t.Fatalf("detail carries the previous version for the diff: %+v %v", detail, err)
	}

	current, err := promptsSetCurrentHandler(d)(ctx, promptsSetCurrentInput{SuiteID: s.ID.String(), VersionID: v1.ID}, operator)
	if err != nil || !current.IsCurrent {
		t.Fatalf("set current: %+v %v", current, err)
	}
	list, err := promptsListHandler(d)(ctx, suiteRef{SuiteID: s.ID.String()}, operator)
	if err != nil || len(list.Items) != 2 || !list.Items[0].IsCurrent || list.Items[1].IsCurrent {
		t.Fatalf("exactly v1 current, ascending: %+v %v", list.Items, err)
	}
}

func TestPromptVersionsRefuseOtherApps(t *testing.T) {
	d := newTestDeps(t)
	mine := seedSuite(t, d, testApp, "mine", "p")
	theirs := seedSuite(t, d, "app_b", "theirs", "p")
	ctx := context.Background()
	other, err := promptsCreateHandler(Deps{Engine: d.Engine, DashboardAppID: "app_b"})(ctx, promptsCreateInput{SuiteID: theirs.ID.String(), SystemPrompt: "x"}, operator)
	if err != nil {
		t.Fatal(err)
	}
	_, err = promptsDetailHandler(d)(ctx, versionRef{VersionID: other.ID}, operator)
	wantCode(t, err, "NOT_FOUND")
	_, missing := promptsDetailHandler(d)(ctx, versionRef{VersionID: id.NewPromptVersionID().String()}, operator)
	wantSameNotFound(t, missing, err)
	_, malformed := promptsDetailHandler(d)(ctx, versionRef{VersionID: "not-an-id"}, operator)
	wantSameNotFound(t, malformed, err)
	_, err = promptsSetCurrentHandler(d)(ctx, promptsSetCurrentInput{SuiteID: mine.ID.String(), VersionID: other.ID}, operator)
	wantCode(t, err, "NOT_FOUND")
}
