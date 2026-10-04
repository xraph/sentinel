package contract

import (
	"testing"

	dashauth "github.com/xraph/forge/extensions/dashboard/auth"
	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"
)

func withClaims(claims map[string]any) dashcontract.Principal {
	return dashcontract.Principal{User: &dashauth.UserInfo{Subject: "operator@example.com"}, Claims: claims}
}

func TestResolveApp(t *testing.T) {
	configured := Deps{DashboardAppID: "app_a"}
	unconfigured := Deps{}

	cases := []struct {
		name string
		deps Deps
		p    dashcontract.Principal
		want string
		code dashcontract.ErrorCode
	}{
		{"no user at all", configured, dashcontract.Principal{}, "", dashcontract.CodeUnauthenticated},
		{"user with empty subject", configured, dashcontract.Principal{User: &dashauth.UserInfo{}}, "", dashcontract.CodeUnauthenticated},
		{"claim wins", configured, withClaims(map[string]any{"app_id": "app_z"}), "app_z", ""},
		{"no claim falls back to config", configured, withClaims(nil), "app_a", ""},
		{"no claim and no config refuses", unconfigured, withClaims(nil), "", dashcontract.CodePermissionDenied},
		// Review focus 1: present but unusable refuses; it never falls through to config.
		{"empty-string claim refuses", configured, withClaims(map[string]any{"app_id": ""}), "", dashcontract.CodePermissionDenied},
		{"numeric claim refuses", configured, withClaims(map[string]any{"app_id": 42}), "", dashcontract.CodePermissionDenied},
		{"nil claim refuses", configured, withClaims(map[string]any{"app_id": nil}), "", dashcontract.CodePermissionDenied},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.deps.resolveApp(c.p)
			if c.code != "" {
				wantCode(t, err, c.code)
				if got != "" {
					t.Fatalf("a refusal must return no app, got %q", got)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("got %q, %v; want %q", got, err, c.want)
			}
		})
	}
}
