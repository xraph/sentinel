package api

import (
	"net/http"

	"github.com/xraph/forge"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/suite"
)

func (a *API) registerSuiteRoutes(router forge.Router) {
	g := router.Group("/v1", forge.WithGroupTags("suites"))

	_ = g.POST("/suites", a.createSuite,
		forge.WithSummary("Create suite"),
		forge.WithDescription("Creates a new evaluation suite."),
		forge.WithOperationID("createSuite"),
		forge.WithRequestSchema(CreateSuiteRequest{}),
		forge.WithCreatedResponse(&suite.Suite{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/suites", a.listSuites,
		forge.WithSummary("List suites"),
		forge.WithDescription("Returns evaluation suites with optional pagination."),
		forge.WithOperationID("listSuites"),
		forge.WithRequestSchema(ListSuitesRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Suite list", []*suite.Suite{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/suites/:suiteId", a.getSuite,
		forge.WithSummary("Get suite"),
		forge.WithDescription("Returns details of a specific suite."),
		forge.WithOperationID("getSuite"),
		forge.WithResponseSchema(http.StatusOK, "Suite details", &suite.Suite{}),
		forge.WithErrorResponses(),
	)

	_ = g.PUT("/suites/:suiteId", a.updateSuite,
		forge.WithSummary("Update suite"),
		forge.WithDescription("Updates an evaluation suite."),
		forge.WithOperationID("updateSuite"),
		forge.WithRequestSchema(CreateSuiteRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Updated suite", &suite.Suite{}),
		forge.WithErrorResponses(),
	)

	_ = g.DELETE("/suites/:suiteId", a.deleteSuite,
		forge.WithSummary("Delete suite"),
		forge.WithDescription("Deletes an evaluation suite."),
		forge.WithOperationID("deleteSuite"),
		forge.WithNoContentResponse(),
		forge.WithErrorResponses(),
	)
}

func (a *API) createSuite(ctx forge.Context, req *CreateSuiteRequest) (*suite.Suite, error) {
	s := &suite.Suite{
		Entity:       sentinel.NewEntity(),
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		Model:        req.Model,
		Temperature:  req.Temperature,
		PersonaRef:   req.PersonaRef,
		Metadata:     req.Metadata,
	}

	if err := a.eng.CreateSuite(ctx.Request().Context(), s); err != nil {
		return nil, mapStoreError(err)
	}

	return s, ctx.JSON(http.StatusCreated, s)
}

func (a *API) listSuites(ctx forge.Context, req *ListSuitesRequest) ([]*suite.Suite, error) {
	filter := &suite.ListFilter{
		AppID:  sentinel.AppFromContext(ctx.Request().Context()),
		Limit:  defaultLimit(req.Limit),
		Offset: req.Offset,
	}

	suites, err := a.eng.ListSuites(ctx.Request().Context(), filter)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return suites, ctx.JSON(http.StatusOK, suites)
}

func (a *API) getSuite(ctx forge.Context, _ *struct{}) (*suite.Suite, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	s, err := a.eng.GetSuite(ctx.Request().Context(), suiteID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return s, ctx.JSON(http.StatusOK, s)
}

func (a *API) updateSuite(ctx forge.Context, req *CreateSuiteRequest) (*suite.Suite, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	s, err := a.eng.GetSuite(ctx.Request().Context(), suiteID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	s.Name = req.Name
	s.Description = req.Description
	s.SystemPrompt = req.SystemPrompt
	s.Model = req.Model
	s.Temperature = req.Temperature
	s.PersonaRef = req.PersonaRef
	if req.Metadata != nil {
		s.Metadata = req.Metadata
	}

	if err := a.eng.UpdateSuite(ctx.Request().Context(), s); err != nil {
		return nil, mapStoreError(err)
	}

	return s, ctx.JSON(http.StatusOK, s)
}

func (a *API) deleteSuite(ctx forge.Context, _ *struct{}) (any, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	if err := a.eng.DeleteSuite(ctx.Request().Context(), suiteID); err != nil {
		return nil, mapStoreError(err)
	}

	return nil, ctx.NoContent(http.StatusNoContent)
}
