package api

import (
	"net/http"

	"github.com/xraph/forge"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/testcase"
)

func (a *API) registerCaseRoutes(router forge.Router) {
	g := router.Group("/v1", forge.WithGroupTags("cases"))

	_ = g.POST("/suites/:suiteId/cases", a.createCases,
		forge.WithSummary("Create test cases"),
		forge.WithDescription("Creates one or more test cases in a suite."),
		forge.WithOperationID("createCases"),
		forge.WithRequestSchema(CreateCasesRequest{}),
		forge.WithCreatedResponse([]*testcase.Case{}),
		forge.WithErrorResponses(),
	)

	_ = g.POST("/suites/:suiteId/cases/import", a.importCases,
		forge.WithSummary("Import test cases"),
		forge.WithDescription("Imports test cases from JSON, CSV, or JSONL."),
		forge.WithOperationID("importCases"),
		forge.WithRequestSchema(ImportCasesRequest{}),
		forge.WithCreatedResponse(map[string]int64{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/suites/:suiteId/cases", a.getCases,
		forge.WithSummary("List test cases"),
		forge.WithDescription("Returns all test cases in a suite."),
		forge.WithOperationID("getCases"),
		forge.WithRequestSchema(ListCasesRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Case list", []*testcase.Case{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/cases/:caseId", a.getCase,
		forge.WithSummary("Get test case"),
		forge.WithDescription("Returns a specific test case."),
		forge.WithOperationID("getCase"),
		forge.WithRequestSchema(GetCaseRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Case details", &testcase.Case{}),
		forge.WithErrorResponses(),
	)

	_ = g.DELETE("/cases/:caseId", a.deleteCase,
		forge.WithSummary("Delete test case"),
		forge.WithDescription("Deletes a test case."),
		forge.WithOperationID("deleteCase"),
		forge.WithRequestSchema(DeleteCaseRequest{}),
		forge.WithNoContentResponse(),
		forge.WithErrorResponses(),
	)
}

func (a *API) createCases(ctx forge.Context, req *CreateCasesRequest) ([]*testcase.Case, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	cases := make([]*testcase.Case, len(req.Cases))
	for i, c := range req.Cases {
		scorers := make([]testcase.ScorerConfig, len(c.Scorers))
		for j, s := range c.Scorers {
			scorers[j] = testcase.ScorerConfig{Name: s.Name, Config: s.Config}
		}

		scenarioType := testcase.ScenarioStandard
		if c.ScenarioType != "" {
			scenarioType = testcase.ScenarioType(c.ScenarioType)
		}

		cases[i] = &testcase.Case{
			Entity:       sentinel.NewEntity(),
			SuiteID:      suiteID,
			Name:         c.Name,
			Input:        c.Input,
			Expected:     c.Expected,
			ScenarioType: scenarioType,
			Scorers:      scorers,
			Tags:         c.Tags,
			Context:      c.Context,
			Metadata:     c.Metadata,
		}
	}

	if err := a.eng.CreateCaseBatch(ctx.Request().Context(), cases); err != nil {
		return nil, mapStoreError(err)
	}

	return cases, ctx.JSON(http.StatusCreated, cases)
}

func (a *API) importCases(ctx forge.Context, req *ImportCasesRequest) (any, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	count, err := a.eng.ImportCases(ctx.Request().Context(), suiteID, req.Format, []byte(req.Data))
	if err != nil {
		return nil, mapStoreError(err)
	}

	result := map[string]int64{"imported": count}
	return result, ctx.JSON(http.StatusCreated, result)
}

func (a *API) getCases(ctx forge.Context, _ *ListCasesRequest) ([]*testcase.Case, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	cases, err := a.eng.ListCases(ctx.Request().Context(), suiteID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return cases, ctx.JSON(http.StatusOK, cases)
}

func (a *API) getCase(ctx forge.Context, _ *GetCaseRequest) (*testcase.Case, error) {
	caseID, err := id.ParseCaseID(ctx.Param("caseId"))
	if err != nil {
		return nil, forge.BadRequest("invalid case ID")
	}

	tc, err := a.eng.GetCase(ctx.Request().Context(), caseID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return tc, ctx.JSON(http.StatusOK, tc)
}

func (a *API) deleteCase(ctx forge.Context, _ *DeleteCaseRequest) (any, error) {
	caseID, err := id.ParseCaseID(ctx.Param("caseId"))
	if err != nil {
		return nil, forge.BadRequest("invalid case ID")
	}

	if err := a.eng.DeleteCase(ctx.Request().Context(), caseID); err != nil {
		return nil, mapStoreError(err)
	}

	return nil, ctx.NoContent(http.StatusNoContent)
}
