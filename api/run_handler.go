package api

import (
	"net/http"

	"github.com/xraph/forge"

	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
)

func (a *API) registerRunRoutes(router forge.Router) {
	g := router.Group("/v1", forge.WithGroupTags("runs"))

	_ = g.POST("/suites/:suiteId/runs", a.runEval,
		forge.WithSummary("Run evaluation"),
		forge.WithDescription("Triggers an evaluation run for a suite."),
		forge.WithOperationID("runEval"),
		forge.WithRequestSchema(RunEvalRequest{}),
		forge.WithCreatedResponse(&engine.RunResult{}),
		forge.WithErrorResponses(),
	)

	_ = g.POST("/suites/:suiteId/compare", a.compareModels,
		forge.WithSummary("Compare models"),
		forge.WithDescription("Runs evaluation across multiple models and compares results."),
		forge.WithOperationID("compareModels"),
		forge.WithRequestSchema(CompareModelsRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Comparison report", map[string]any{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/runs/:runId", a.getRun,
		forge.WithSummary("Get run"),
		forge.WithDescription("Returns details of an evaluation run."),
		forge.WithOperationID("getRun"),
		forge.WithResponseSchema(http.StatusOK, "Run details", &evalrun.Run{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/suites/:suiteId/runs", a.listRuns,
		forge.WithSummary("List runs"),
		forge.WithDescription("Returns evaluation runs for a suite."),
		forge.WithOperationID("listRuns"),
		forge.WithRequestSchema(ListRunsRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Run list", []*evalrun.Run{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/runs/:runId/results", a.getRunResults,
		forge.WithSummary("Get run results"),
		forge.WithDescription("Returns all results for an evaluation run."),
		forge.WithOperationID("getRunResults"),
		forge.WithResponseSchema(http.StatusOK, "Run results", []*evalrun.Result{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/runs/:runId/stats", a.getRunStats,
		forge.WithSummary("Get run statistics"),
		forge.WithDescription("Returns aggregate statistics for a run."),
		forge.WithOperationID("getRunStats"),
		forge.WithResponseSchema(http.StatusOK, "Run statistics", &evalrun.ResultStats{}),
		forge.WithErrorResponses(),
	)
}

func (a *API) runEval(ctx forge.Context, req *RunEvalRequest) (*engine.RunResult, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	cfg := &engine.RunConfig{
		SuiteID:     suiteID,
		Model:       req.Model,
		PersonaRef:  req.PersonaRef,
		Tags:        req.Tags,
		Concurrency: req.Concurrency,
	}

	// Note: Target and Scorers must be configured at the engine level
	// or via a more advanced API. This endpoint uses engine defaults.

	result, err := a.eng.RunEval(ctx.Request().Context(), cfg)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return result, ctx.JSON(http.StatusCreated, result)
}

func (a *API) compareModels(ctx forge.Context, _ *CompareModelsRequest) (any, error) {
	// Comparison requires targets to be provided programmatically.
	// The HTTP API provides a placeholder; full comparison is done via the Go API.
	return nil, forge.BadRequest("model comparison must be configured programmatically with target adapters")
}

func (a *API) getRun(ctx forge.Context, _ *struct{}) (*evalrun.Run, error) {
	runID, err := id.ParseEvalRunID(ctx.Param("runId"))
	if err != nil {
		return nil, forge.BadRequest("invalid run ID")
	}

	run, err := a.eng.GetRun(ctx.Request().Context(), runID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return run, ctx.JSON(http.StatusOK, run)
}

func (a *API) listRuns(ctx forge.Context, req *ListRunsRequest) ([]*evalrun.Run, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	filter := &evalrun.ListFilter{
		SuiteID: suiteID,
		State:   evalrun.RunState(req.State),
		Limit:   defaultLimit(req.Limit),
		Offset:  req.Offset,
	}

	runs, err := a.eng.ListRuns(ctx.Request().Context(), filter)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return runs, ctx.JSON(http.StatusOK, runs)
}

func (a *API) getRunResults(ctx forge.Context, _ *struct{}) ([]*evalrun.Result, error) {
	runID, err := id.ParseEvalRunID(ctx.Param("runId"))
	if err != nil {
		return nil, forge.BadRequest("invalid run ID")
	}

	results, err := a.eng.ListResults(ctx.Request().Context(), runID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return results, ctx.JSON(http.StatusOK, results)
}

func (a *API) getRunStats(ctx forge.Context, _ *struct{}) (*evalrun.ResultStats, error) {
	runID, err := id.ParseEvalRunID(ctx.Param("runId"))
	if err != nil {
		return nil, forge.BadRequest("invalid run ID")
	}

	stats, err := a.eng.GetResultStats(ctx.Request().Context(), runID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return stats, ctx.JSON(http.StatusOK, stats)
}
