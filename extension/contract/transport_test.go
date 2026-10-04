package contract

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	dashauth "github.com/xraph/forge/extensions/dashboard/auth"
	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"
	"github.com/xraph/forge/extensions/dashboard/contract/dispatcher"
	"github.com/xraph/forge/extensions/dashboard/contract/transport"
)

// A command sent through forge's real HTTP transport comes back with the
// manifest's invalidates in its meta. The handlers return none of their
// own; the client learns what to refetch only if the transport merges the
// manifest's list in, which forge v1.10.0 did not.
//
// The transport derives the principal from the request context with
// dashauth.UserFromContext, so the request carries the harness's operator
// the way forge's auth middleware would attach a signed-in user.
func TestCommandInvalidatesReachTheClient(t *testing.T) {
	deps := newTestDeps(t)
	reg, wreg := dashcontract.NewRegistry(), dashcontract.NewWardenRegistry()
	d := dispatcher.New(nil)
	if err := Register(d, reg, wreg, deps); err != nil {
		t.Fatalf("Register: %v", err)
	}
	var want []string
	for _, in := range loadManifest(t).Intents {
		if in.Name == "suites.create" {
			want = in.Invalidates
		}
	}
	if len(want) == 0 {
		t.Fatal("suites.create declares no invalidates in the manifest")
	}

	body := `{"envelope":"v1","kind":"command","contributor":"sentinel","intent":"suites.create",` +
		`"csrf":"test","idempotencyKey":"test","payload":{"name":"over-the-wire"}}`
	ctx := dashauth.WithUser(context.Background(), operator.User)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/dashboard/v1", strings.NewReader(body))
	rec := httptest.NewRecorder()
	transport.NewHandler(reg, wreg, d, nil).ServeHTTP(rec, req)

	var resp dashcontract.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body)
	}
	if !resp.OK {
		t.Fatalf("suites.create failed over the wire: %s", rec.Body)
	}
	if !reflect.DeepEqual(resp.Meta.Invalidates, want) {
		t.Fatalf("meta.invalidates = %v, want %v", resp.Meta.Invalidates, want)
	}
}
