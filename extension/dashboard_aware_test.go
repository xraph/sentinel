package extension

import "github.com/xraph/forge/extensions/dashboard"

// The dashboard finds contributors by type assertion at runtime. The check
// lives here so the production build never imports the dashboard's root
// package, which would pull the dashboard's own server (its handlers,
// collector and discovery) into it.
var _ dashboard.ContractContributorAware = (*Extension)(nil)
