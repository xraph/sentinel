package contract

import (
	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"
)

// appClaim is the claim key an app selector would populate, the convention
// authsome uses.
const appClaim = "app_id"

func requireUser(p dashcontract.Principal) error {
	if p.User == nil || p.User.Subject == "" {
		return &dashcontract.Error{Code: dashcontract.CodeUnauthenticated, Message: "sentinel: authentication required"}
	}
	return nil
}

// resolveApp decides which app a request is for.
//
// READ THIS BEFORE CHANGING IT. An empty app id in a store list filter
// matches EVERY app (store/storetest/tenancy.go records it on all four
// backends), so a handler that resolved "" would serve every app's suites,
// runs and outputs. This never returns "".
//
//  0. A signed-in user, or refuse UNAUTHENTICATED.
//  1. An app_id claim that is a non-empty string.
//  2. A claim that is present and anything else refuses PERMISSION_DENIED.
//     It does not fall through to step 3: something tried to say which app
//     this is and failed, and answering for a different app is how
//     empty-matches-everything comes back.
//  3. No claim at all: DashboardAppID, when configured.
//  4. Otherwise refuse PERMISSION_DENIED.
//
// Nothing populates claims today (authsome's UserInfo carries none), so
// every deployment takes step 3 or 4. The claim read stays because it is
// where the app belongs once a selector exists.
func (d Deps) resolveApp(p dashcontract.Principal) (string, error) {
	if err := requireUser(p); err != nil {
		return "", err
	}
	if raw, present := p.Claims[appClaim]; present {
		s, ok := raw.(string)
		if !ok || s == "" {
			return "", permissionDenied("the app claim is present but unusable: refusing rather than falling back to a different app")
		}
		return s, nil
	}
	if d.DashboardAppID != "" {
		return d.DashboardAppID, nil
	}
	return "", permissionDenied("no app in scope: set extensions.sentinel.dashboard_app_id for this deployment")
}
