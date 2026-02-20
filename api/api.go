// Package api provides Forge-style HTTP handlers for the Sentinel evaluation engine.
package api

import (
	"net/http"

	"github.com/xraph/forge"

	"github.com/xraph/sentinel/engine"
)

// API wires all Forge-style HTTP handlers together for the Sentinel system.
type API struct {
	eng    *engine.Engine
	router forge.Router
}

// New creates an API from a Sentinel Engine.
func New(eng *engine.Engine, router forge.Router) *API {
	return &API{eng: eng, router: router}
}

// Handler returns the fully assembled http.Handler with all routes.
func (a *API) Handler() http.Handler {
	if a.router == nil {
		a.router = forge.NewRouter()
	}
	a.RegisterRoutes(a.router)
	return a.router.Handler()
}

// RegisterRoutes registers all Sentinel API routes into the given Forge router.
func (a *API) RegisterRoutes(router forge.Router) {
	a.registerSuiteRoutes(router)
	a.registerCaseRoutes(router)
	a.registerRunRoutes(router)
	a.registerBaselineRoutes(router)
	a.registerRedTeamRoutes(router)
	a.registerPromptRoutes(router)
	a.registerScenarioRoutes(router)
	a.registerReportRoutes(router)
}
