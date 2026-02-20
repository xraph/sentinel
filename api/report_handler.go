package api

import (
	"bytes"
	"net/http"

	"github.com/xraph/forge"

	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/report"
)

func (a *API) registerReportRoutes(router forge.Router) {
	g := router.Group("/v1", forge.WithGroupTags("reports"))

	_ = g.GET("/runs/:runId/report", a.getReport,
		forge.WithSummary("Get report"),
		forge.WithDescription("Returns a JSON report for an evaluation run."),
		forge.WithOperationID("getReport"),
		forge.WithResponseSchema(http.StatusOK, "Evaluation report", &report.Report{}),
		forge.WithErrorResponses(),
	)

	_ = g.POST("/runs/:runId/report/export", a.exportReport,
		forge.WithSummary("Export report"),
		forge.WithDescription("Exports a report in a specified format (terminal, json, html, ci)."),
		forge.WithOperationID("exportReport"),
		forge.WithRequestSchema(ExportReportRequest{}),
		forge.WithResponseSchema(http.StatusOK, "Exported report", ""),
		forge.WithErrorResponses(),
	)
}

func (a *API) getReport(ctx forge.Context, _ *struct{}) (*report.Report, error) {
	runID, err := id.ParseEvalRunID(ctx.Param("runId"))
	if err != nil {
		return nil, forge.BadRequest("invalid run ID")
	}

	run, err := a.eng.GetRun(ctx.Request().Context(), runID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	results, err := a.eng.ListResults(ctx.Request().Context(), runID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	stats, err := a.eng.GetResultStats(ctx.Request().Context(), runID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	rpt := report.NewReport(run, results, stats)
	return rpt, ctx.JSON(http.StatusOK, rpt)
}

func (a *API) exportReport(ctx forge.Context, req *ExportReportRequest) (any, error) {
	runID, err := id.ParseEvalRunID(ctx.Param("runId"))
	if err != nil {
		return nil, forge.BadRequest("invalid run ID")
	}

	run, err := a.eng.GetRun(ctx.Request().Context(), runID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	results, err := a.eng.ListResults(ctx.Request().Context(), runID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	stats, err := a.eng.GetResultStats(ctx.Request().Context(), runID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	rpt := report.NewReport(run, results, stats)

	format := report.Format(req.Format)
	if format == "" {
		format = report.FormatJSON
	}

	var reporter report.Reporter
	switch format {
	case report.FormatTerminal:
		reporter = report.NewTerminalReporter()
	case report.FormatJSON:
		reporter = report.NewJSONReporter()
	case report.FormatHTML:
		reporter = report.NewHTMLReporter()
	case report.FormatCI:
		reporter = report.NewCIReporter()
	default:
		return nil, forge.BadRequest("unsupported format: " + string(format))
	}

	var buf bytes.Buffer
	if err := reporter.Render(&buf, rpt); err != nil {
		return nil, err
	}

	contentType := "text/plain"
	if format == report.FormatJSON {
		contentType = "application/json"
	} else if format == report.FormatHTML {
		contentType = "text/html"
	}

	ctx.Response().Header().Set("Content-Type", contentType)
	ctx.Response().WriteHeader(http.StatusOK)
	_, _ = ctx.Response().Write(buf.Bytes())

	return nil, nil
}
