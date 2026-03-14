package api

import (
	"net/http"

	"github.com/xraph/forge"

	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/promptversion"
)

func (a *API) registerPromptRoutes(router forge.Router) {
	g := router.Group("/v1", forge.WithGroupTags("prompts"))

	_ = g.POST("/suites/:suiteId/prompts", a.createPromptVersion,
		forge.WithSummary("Create prompt version"),
		forge.WithDescription("Creates a new prompt version for a suite."),
		forge.WithOperationID("createPromptVersion"),
		forge.WithRequestSchema(CreatePromptVersionRequest{}),
		forge.WithCreatedResponse(&promptversion.PromptVersion{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/suites/:suiteId/prompts", a.listPromptVersions,
		forge.WithSummary("List prompt versions"),
		forge.WithDescription("Returns all prompt versions for a suite."),
		forge.WithOperationID("listPromptVersions"),
		forge.WithRequestSchema(ListPromptVersionsRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Prompt version list", []*promptversion.PromptVersion{}),
		forge.WithErrorResponses(),
	)

	_ = g.POST("/suites/:suiteId/prompts/current", a.setCurrentPrompt,
		forge.WithSummary("Set current prompt"),
		forge.WithDescription("Sets the current prompt version for a suite."),
		forge.WithOperationID("setCurrentPrompt"),
		forge.WithRequestSchema(SetCurrentPromptRequest{}),
		forge.WithNoContentResponse(),
		forge.WithErrorResponses(),
	)
}

func (a *API) createPromptVersion(ctx forge.Context, req *CreatePromptVersionRequest) (*promptversion.PromptVersion, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	pv := &promptversion.PromptVersion{
		SuiteID:      suiteID,
		SystemPrompt: req.SystemPrompt,
		Changelog:    req.Label,
	}

	if err := a.eng.CreatePromptVersion(ctx.Request().Context(), pv); err != nil {
		return nil, mapStoreError(err)
	}

	return pv, ctx.JSON(http.StatusCreated, pv)
}

func (a *API) listPromptVersions(ctx forge.Context, _ *ListPromptVersionsRequest) ([]*promptversion.PromptVersion, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	versions, err := a.eng.ListPromptVersions(ctx.Request().Context(), suiteID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return versions, ctx.JSON(http.StatusOK, versions)
}

func (a *API) setCurrentPrompt(ctx forge.Context, req *SetCurrentPromptRequest) (any, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	pvID, err := id.ParsePromptVersionID(req.VersionID)
	if err != nil {
		return nil, forge.BadRequest("invalid prompt version ID")
	}

	if err := a.eng.SetCurrentPromptVersion(ctx.Request().Context(), suiteID, pvID); err != nil {
		return nil, mapStoreError(err)
	}

	return nil, ctx.NoContent(http.StatusNoContent)
}
