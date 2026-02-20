package api

import (
	"net/http"

	"github.com/xraph/forge"

	"github.com/xraph/sentinel/testcase"
)

func (a *API) registerRedTeamRoutes(router forge.Router) {
	g := router.Group("/v1", forge.WithGroupTags("redteam"))

	_ = g.POST("/suites/:suiteId/redteam/generate", a.generateAttacks,
		forge.WithSummary("Generate red team attacks"),
		forge.WithDescription("Generates adversarial test cases for a suite."),
		forge.WithOperationID("generateAttacks"),
		forge.WithRequestSchema(GenerateAttacksRequest{}),
		forge.WithCreatedResponse([]*testcase.Case{}),
		forge.WithErrorResponses(),
	)

	_ = g.POST("/suites/:suiteId/redteam/run", a.runRedTeam,
		forge.WithSummary("Run red team evaluation"),
		forge.WithDescription("Generates and runs adversarial test cases against the target."),
		forge.WithOperationID("runRedTeam"),
		forge.WithRequestSchema(RunRedTeamRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Red team report", map[string]any{}),
		forge.WithErrorResponses(),
	)
}

func (a *API) generateAttacks(_ forge.Context, _ *GenerateAttacksRequest) (any, error) {
	// Red team attack generation requires configuring generators programmatically.
	return nil, forge.BadRequest("red team attack generation must be configured programmatically with attack generators")
}

func (a *API) runRedTeam(_ forge.Context, _ *RunRedTeamRequest) (any, error) {
	// Red team runs require a target to be configured programmatically.
	return nil, forge.BadRequest("red team evaluation must be configured programmatically with target adapters")
}
