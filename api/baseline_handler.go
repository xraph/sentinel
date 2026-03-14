package api

import (
	"net/http"

	"github.com/xraph/forge"

	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/id"
)

func (a *API) registerBaselineRoutes(router forge.Router) {
	g := router.Group("/v1", forge.WithGroupTags("baselines"))

	_ = g.POST("/suites/:suiteId/baselines", a.saveBaseline,
		forge.WithSummary("Save baseline"),
		forge.WithDescription("Saves an evaluation run as a baseline for regression detection."),
		forge.WithOperationID("saveBaseline"),
		forge.WithRequestSchema(SaveBaselineRequest{}),
		forge.WithCreatedResponse(&baseline.Baseline{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/suites/:suiteId/baselines", a.listBaselines,
		forge.WithSummary("List baselines"),
		forge.WithDescription("Returns all baselines for a suite."),
		forge.WithOperationID("listBaselines"),
		forge.WithRequestSchema(ListBaselinesRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Baseline list", []*baseline.Baseline{}),
		forge.WithErrorResponses(),
	)

	_ = g.GET("/baselines/:baselineId", a.getBaseline,
		forge.WithSummary("Get baseline"),
		forge.WithDescription("Returns details of a specific baseline."),
		forge.WithOperationID("getBaseline"),
		forge.WithRequestSchema(GetBaselineRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Baseline details", &baseline.Baseline{}),
		forge.WithErrorResponses(),
	)

	_ = g.POST("/suites/:suiteId/runs/with-baseline", a.runWithBaseline,
		forge.WithSummary("Run with baseline comparison"),
		forge.WithDescription("Runs evaluation and compares against the latest baseline."),
		forge.WithOperationID("runWithBaseline"),
		forge.WithRequestSchema(RunWithBaselineRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Run with baseline result", map[string]any{}),
		forge.WithErrorResponses(),
	)
}

func (a *API) saveBaseline(ctx forge.Context, req *SaveBaselineRequest) (*baseline.Baseline, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	runID, err := id.ParseEvalRunID(req.RunID)
	if err != nil {
		return nil, forge.BadRequest("invalid run ID")
	}

	// Load the run and its results to create the baseline.
	run, err := a.eng.GetRun(ctx.Request().Context(), runID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	results, err := a.eng.ListResults(ctx.Request().Context(), runID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	baselineResults := make([]baseline.Result, len(results))
	for i, r := range results {
		baselineResults[i] = baseline.Result{
			CaseID:          r.CaseID,
			CaseName:        r.CaseName,
			Score:           r.Score,
			Status:          string(r.Status),
			DimensionScores: r.DimensionScores,
		}
	}

	b := &baseline.Baseline{
		SuiteID:         suiteID,
		RunID:           runID,
		Name:            req.Name,
		Results:         baselineResults,
		PassRate:        run.PassRate,
		AvgScore:        run.AvgScore,
		DimensionScores: run.DimensionScores,
		IsCurrent:       true,
	}

	if err := a.eng.SaveBaseline(ctx.Request().Context(), b); err != nil {
		return nil, mapStoreError(err)
	}

	return b, ctx.JSON(http.StatusCreated, b)
}

func (a *API) listBaselines(ctx forge.Context, _ *ListBaselinesRequest) ([]*baseline.Baseline, error) {
	suiteID, err := id.ParseSuiteID(ctx.Param("suiteId"))
	if err != nil {
		return nil, forge.BadRequest("invalid suite ID")
	}

	baselines, err := a.eng.ListBaselines(ctx.Request().Context(), suiteID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return baselines, ctx.JSON(http.StatusOK, baselines)
}

func (a *API) getBaseline(ctx forge.Context, _ *GetBaselineRequest) (*baseline.Baseline, error) {
	baselineID, err := id.ParseBaselineID(ctx.Param("baselineId"))
	if err != nil {
		return nil, forge.BadRequest("invalid baseline ID")
	}

	b, err := a.eng.GetBaseline(ctx.Request().Context(), baselineID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return b, ctx.JSON(http.StatusOK, b)
}

func (a *API) runWithBaseline(_ forge.Context, _ *RunWithBaselineRequest) (any, error) {
	// Run-with-baseline requires a target to be configured programmatically.
	// The HTTP API provides a placeholder.
	return nil, forge.BadRequest("run-with-baseline must be configured programmatically with target adapters")
}
