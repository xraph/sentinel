package contract

import (
	"context"
	"errors"
	"testing"

	dashauth "github.com/xraph/forge/extensions/dashboard/auth"
	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"

	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/store/memory"
	"github.com/xraph/sentinel/target"
)

const testApp = "app_a"

var operator = dashcontract.Principal{User: &dashauth.UserInfo{Subject: "operator@example.com"}}

// newTestDeps builds an engine on a fresh memory store with one registered
// target, "echo", and Deps scoped to testApp through DashboardAppID.
func newTestDeps(t *testing.T, opts ...engine.Option) Deps {
	t.Helper()
	echo := target.FromFunc("echo", func(_ context.Context, in string) (string, error) { return "echo: " + in, nil })
	all := append([]engine.Option{engine.WithStore(memory.New()), engine.WithTarget("echo", "returns its input", echo)}, opts...)
	eng, err := engine.New(all...)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	t.Cleanup(func() { _ = eng.Stop(context.Background()) })
	return Deps{Engine: eng, DashboardAppID: testApp}
}

// wantSameNotFound asserts a missing id and another app's id are
// indistinguishable to the caller: both NOT_FOUND with the same message.
func wantSameNotFound(t *testing.T, missing, other error) {
	t.Helper()
	var m, o *dashcontract.Error
	if !errors.As(missing, &m) || m.Code != dashcontract.CodeNotFound {
		t.Fatalf("missing id: want NOT_FOUND, got %v", missing)
	}
	if !errors.As(other, &o) || o.Code != dashcontract.CodeNotFound {
		t.Fatalf("other app's id: want NOT_FOUND, got %v", other)
	}
	if m.Message != o.Message {
		t.Fatalf("the two answers differ: %q vs %q", m.Message, o.Message)
	}
}

func wantCode(t *testing.T, err error, code dashcontract.ErrorCode) {
	t.Helper()
	var ce *dashcontract.Error
	if !errors.As(err, &ce) {
		t.Fatalf("want a contract error with code %s, got %v", code, err)
	}
	if ce.Code != code {
		t.Fatalf("want code %s, got %s (%s)", code, ce.Code, ce.Message)
	}
}
