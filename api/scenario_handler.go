package api

import (
	"net/http"

	"github.com/xraph/forge"

	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/testcase"
)

func (a *API) registerScenarioRoutes(router forge.Router) {
	g := router.Group("/v1", forge.WithGroupTags("scenarios"))

	_ = g.POST("/suites/:suiteId/scenarios/generate", a.generateScenarios,
		forge.WithSummary("Generate test scenarios"),
		forge.WithDescription("Generates persona-aware test scenarios for a suite."),
		forge.WithOperationID("generateScenarios"),
		forge.WithRequestSchema(GenerateScenariosRequest{}),
		forge.WithCreatedResponse([]*testcase.Case{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/suites/:suiteId/scenarios", a.listScenarios,
		forge.WithSummary("List scenarios"),
		forge.WithDescription("Returns all scenario-type test cases for a suite."),
		forge.WithOperationID("listScenarios"),
		forge.WithRequestSchema(ListScenariosRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Scenario list", []*testcase.Case{}),
		forge.WithErrorResponses(),
	)
}

func (a *API) generateScenarios(_ forge.Context, _ *GenerateScenariosRequest) (any, error) {
	// Scenario generation requires configuring generators programmatically.
	return nil, forge.BadRequest("scenario generation must be configured programmatically with scenario generators")
}

func (a *API) listScenarios(ctx forge.Context, _ *ListScenariosRequest) ([]*testcase.Case, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	cases, err := a.eng.ListCases(ctx.Request().Context(), suiteID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	// Filter to scenario-type cases only (non-standard).
	scenarios := make([]*testcase.Case, 0)
	for _, c := range cases {
		if c.ScenarioType != testcase.ScenarioStandard {
			scenarios = append(scenarios, c)
		}
	}

	return scenarios, ctx.JSON(http.StatusOK, scenarios)
}
