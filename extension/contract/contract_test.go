package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"
	"github.com/xraph/forge/extensions/dashboard/contract/dispatcher"
	"github.com/xraph/forge/extensions/dashboard/contract/loader"
)

func TestRegisterRefusesNilEngine(t *testing.T) {
	if err := Register(dispatcher.New(nil), dashcontract.NewRegistry(), dashcontract.NewWardenRegistry(), Deps{}); err == nil {
		t.Fatal("Register with no engine must fail")
	}
}

// Every intent the manifest declares has a bound handler. A manifest entry
// with no dispatcher registration would otherwise surface only as a 404 in
// the browser. The handler's own answer does not matter here.
func TestEveryDeclaredIntentIsRegistered(t *testing.T) {
	deps := newTestDeps(t)
	d := dispatcher.New(nil)
	if err := Register(d, dashcontract.NewRegistry(), dashcontract.NewWardenRegistry(), deps); err != nil {
		t.Fatalf("Register: %v", err)
	}
	m := loadManifest(t)
	for _, intent := range m.Intents {
		kind := dashcontract.KindQuery
		req := dashcontract.Request{Envelope: "v1", Contributor: ContributorName, Intent: intent.Name, IntentVersion: 1}
		if intent.Kind == dashcontract.IntentKindCommand {
			kind = dashcontract.KindCommand
			req.Payload = json.RawMessage(`{}`)
		} else {
			req.Params = map[string]any{}
		}
		req.Kind = kind
		if _, _, err := d.Dispatch(context.Background(), req, operator); err != nil &&
			strings.Contains(strings.ToLower(err.Error()), "not registered") {
			t.Errorf("%s is declared but not registered: %v", intent.Name, err)
		}
	}
}

// Every command says what it invalidates, and every name it gives is a
// query this contributor declares. A command with no invalidates looks like
// a write that silently failed, because nothing refreshes.
func TestCommandsInvalidateDeclaredQueries(t *testing.T) {
	m := loadManifest(t)
	queries := map[string]bool{}
	for _, in := range m.Intents {
		if in.Kind == dashcontract.IntentKindQuery {
			queries[in.Name] = true
		}
	}
	for _, in := range m.Intents {
		if in.Kind != dashcontract.IntentKindCommand {
			continue
		}
		if len(in.Invalidates) == 0 {
			t.Errorf("%s invalidates nothing", in.Name)
		}
		for _, q := range in.Invalidates {
			if !queries[q] {
				t.Errorf("%s invalidates %q, which is not a declared query", in.Name, q)
			}
		}
	}
}

func loadManifest(t *testing.T) *dashcontract.ContractManifest {
	t.Helper()
	m, err := loader.Load(bytes.NewReader(manifestYAML), "sentinel/contract/manifest.yaml")
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if err := loader.Validate(m, dashcontract.NewWardenRegistry()); err != nil {
		t.Fatalf("validate manifest: %v", err)
	}
	return m
}

// The contract's surface, pinned. Adding or removing an intent should be a
// deliberate change to this list, the manifest and the React plugin.
func TestIntentSet(t *testing.T) {
	want := map[string]dashcontract.IntentKind{
		"overview.stats": dashcontract.IntentKindQuery, "config.get": dashcontract.IntentKindQuery,
		"suites.list": dashcontract.IntentKindQuery, "suites.detail": dashcontract.IntentKindQuery,
		"cases.list": dashcontract.IntentKindQuery, "cases.detail": dashcontract.IntentKindQuery,
		"prompts.list": dashcontract.IntentKindQuery, "prompts.detail": dashcontract.IntentKindQuery,
		"runs.list": dashcontract.IntentKindQuery, "runs.detail": dashcontract.IntentKindQuery,
		"runs.results": dashcontract.IntentKindQuery, "results.detail": dashcontract.IntentKindQuery,
		"runs.trend": dashcontract.IntentKindQuery, "runs.regression": dashcontract.IntentKindQuery,
		"runs.compare": dashcontract.IntentKindQuery, "baselines.list": dashcontract.IntentKindQuery,
		"baselines.detail": dashcontract.IntentKindQuery, "redteam.report": dashcontract.IntentKindQuery,
		"suites.create": dashcontract.IntentKindCommand, "suites.update": dashcontract.IntentKindCommand,
		"suites.delete": dashcontract.IntentKindCommand, "cases.create": dashcontract.IntentKindCommand,
		"cases.update": dashcontract.IntentKindCommand, "cases.delete": dashcontract.IntentKindCommand,
		"cases.import": dashcontract.IntentKindCommand, "prompts.create": dashcontract.IntentKindCommand,
		"prompts.setCurrent": dashcontract.IntentKindCommand, "baselines.save": dashcontract.IntentKindCommand,
		"baselines.delete": dashcontract.IntentKindCommand, "runs.start": dashcontract.IntentKindCommand,
		"runs.cancel": dashcontract.IntentKindCommand, "redteam.generate": dashcontract.IntentKindCommand,
	}
	m := loadManifest(t)
	if len(m.Intents) != len(want) {
		t.Errorf("manifest declares %d intents, want %d", len(m.Intents), len(want))
	}
	seen := map[string]bool{}
	for _, in := range m.Intents {
		seen[in.Name] = true
		kind, ok := want[in.Name]
		if !ok {
			t.Errorf("unexpected intent %s", in.Name)
			continue
		}
		if in.Kind != kind {
			t.Errorf("%s is a %s, want %s", in.Name, in.Kind, kind)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("intent %s is pinned but the manifest does not declare it", name)
		}
	}
}

// Every write refreshes the overview.
func TestEveryCommandRefreshesTheOverview(t *testing.T) {
	for _, in := range loadManifest(t).Intents {
		if in.Kind != dashcontract.IntentKindCommand {
			continue
		}
		found := false
		for _, q := range in.Invalidates {
			if q == "overview.stats" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s does not invalidate overview.stats", in.Name)
		}
	}
}
