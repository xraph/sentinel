package extension

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	dashauth "github.com/xraph/forge/extensions/dashboard/auth"
	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"
	"github.com/xraph/forge/extensions/dashboard/contract/dispatcher"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/store/memory"
	"github.com/xraph/sentinel/suite"
)

// contractExtension builds an extension the way Register would, minus the
// forge app: merged config, an engine on a memory store, and the contributor
// registered on a real dispatcher.
func contractExtension(t *testing.T, cfg Config) (*Extension, *dispatcher.Dispatcher) {
	t.Helper()
	e := New(WithStore(memory.New()), WithConfig(cfg))
	e.config = e.mergeWithDefaults(e.config)
	eng, err := e.newEngine(nil)
	if err != nil {
		t.Fatalf("newEngine: %v", err)
	}
	e.eng = eng
	t.Cleanup(func() { _ = eng.Stop(context.Background()) })
	disp := dispatcher.New(nil)
	if err := e.RegisterContractContributor(disp, dashcontract.NewRegistry(), dashcontract.NewWardenRegistry()); err != nil {
		t.Fatalf("RegisterContractContributor: %v", err)
	}
	return e, disp
}

func seedExtSuite(t *testing.T, e *Extension, app, name string) *suite.Suite {
	t.Helper()
	s := &suite.Suite{Entity: sentinel.NewEntity(), ID: id.NewSuiteID(), Name: name, AppID: app, Model: "m", Metadata: map[string]any{}}
	if err := e.eng.CreateSuite(context.Background(), s); err != nil {
		t.Fatalf("seed suite: %v", err)
	}
	return s
}

func dispatchQuery(disp *dispatcher.Dispatcher, intent string, p dashcontract.Principal) (json.RawMessage, error) {
	req := dashcontract.Request{Envelope: "v1", Kind: dashcontract.KindQuery, Contributor: "sentinel", Intent: intent, IntentVersion: 1, Params: map[string]any{}}
	data, _, err := disp.Dispatch(context.Background(), req, p)
	return data, err
}

// The deployment's dashboard_app_id is the only app source today, because
// nothing populates an app_id claim. A signed-in user with no claim must be
// served that app, and only that app, through the real wiring.
func TestDashboardAppIDScopesRequestsWithoutAClaim(t *testing.T) {
	e, disp := contractExtension(t, Config{DashboardAppID: "app_cfg"})
	mine := seedExtSuite(t, e, "app_cfg", "mine")
	seedExtSuite(t, e, "app_other", "theirs")
	user := dashcontract.Principal{User: &dashauth.UserInfo{Subject: "operator@example.com"}}

	data, err := dispatchQuery(disp, "suites.list", user)
	if err != nil {
		t.Fatalf("suites.list with no app_id claim: %v", err)
	}
	var out struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode: %v (%s)", err, data)
	}
	if len(out.Items) != 1 || out.Items[0].ID != mine.ID.String() {
		t.Fatalf("the configured app must see exactly its own suite: %s", data)
	}

	if _, err := dispatchQuery(disp, "config.get", user); err != nil {
		t.Fatalf("config.get with no app_id claim: %v", err)
	}
}

func TestNoDashboardAppIDAndNoClaimIsRefused(t *testing.T) {
	e, disp := contractExtension(t, Config{})
	seedExtSuite(t, e, "app_other", "theirs")
	user := dashcontract.Principal{User: &dashauth.UserInfo{Subject: "operator@example.com"}}

	_, err := dispatchQuery(disp, "suites.list", user)
	var ce *dashcontract.Error
	if !errors.As(err, &ce) || ce.Code != dashcontract.CodePermissionDenied {
		t.Fatalf("want PERMISSION_DENIED, never every app's suites: %v", err)
	}
}
