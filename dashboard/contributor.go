package dashboard

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/a-h/templ"

	"github.com/xraph/forge/extensions/dashboard/contributor"

	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/dashboard/components"
	"github.com/xraph/sentinel/dashboard/pages"
	"github.com/xraph/sentinel/dashboard/settings"
	"github.com/xraph/sentinel/dashboard/widgets"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/suite"
)

var _ contributor.LocalContributor = (*Contributor)(nil)

// Contributor implements the dashboard LocalContributor interface for the
// sentinel extension.
type Contributor struct {
	manifest *contributor.Manifest
	engine   *engine.Engine
}

// New creates a new sentinel dashboard contributor.
func New(manifest *contributor.Manifest, eng *engine.Engine) *Contributor {
	return &Contributor{
		manifest: manifest,
		engine:   eng,
	}
}

// Manifest returns the contributor manifest.
func (c *Contributor) Manifest() *contributor.Manifest { return c.manifest }

// RenderPage renders a page for the given route.
func (c *Contributor) RenderPage(ctx context.Context, route string, params contributor.Params) (templ.Component, error) {
	if c.engine == nil {
		return components.EmptyState("alert-circle", "Engine not initialized", "The Sentinel engine is not available. Please check extension configuration."), nil
	}
	if c.engine.Store() == nil {
		return components.EmptyState("database", "No store configured", "The Sentinel dashboard requires a database store. Please configure a Grove driver or provide a store via engine options."), nil
	}
	comp, err := c.renderPageRoute(ctx, route, params)
	if err != nil {
		return nil, err
	}
	// Wrap every page in the PathRewriter so bare hx-get paths
	// are rewritten to the fully-qualified dashboard extension path at runtime.
	pagesBase := params.BasePath + "/ext/" + c.manifest.Name + "/pages"
	return templ.ComponentFunc(func(tCtx context.Context, w io.Writer) error {
		return components.PathRewriter(pagesBase).Render(templ.WithChildren(tCtx, comp), w)
	}), nil
}

func (c *Contributor) renderPageRoute(ctx context.Context, pageRoute string, params contributor.Params) (templ.Component, error) {
	// Normalize route: trim trailing slashes (except root), collapse doubles.
	pageRoute = strings.TrimRight(pageRoute, "/")
	if pageRoute == "" {
		pageRoute = "/"
	}

	switch pageRoute {
	case "/":
		return c.renderOverview(ctx)
	// --- Suites ---
	case "/suites":
		return c.renderSuites(ctx, params)
	case "/suites/create":
		return c.renderSuiteForm(ctx, params)
	case "/suites/edit":
		return c.renderSuiteForm(ctx, params)
	case "/suites/detail":
		return c.renderSuiteDetail(ctx, params)
	// --- Cases (within suite) ---
	case "/suites/cases":
		return c.renderCases(ctx, params)
	case "/suites/cases/create":
		return c.renderCaseForm(ctx, params)
	case "/suites/cases/edit":
		return c.renderCaseForm(ctx, params)
	case "/suites/cases/detail":
		return c.renderCaseDetail(ctx, params)
	// --- Runs ---
	case "/runs":
		return c.renderRuns(ctx, params)
	case "/runs/detail":
		return c.renderRunDetail(ctx, params)
	case "/runs/results/detail":
		return c.renderResultDetail(ctx, params)
	case "/runs/report":
		return c.renderReport(ctx, params)
	// --- Baselines ---
	case "/baselines":
		return c.renderBaselines(ctx, params)
	case "/baselines/detail":
		return c.renderBaselineDetail(ctx, params)
	// --- Prompt Versions ---
	case "/prompts":
		return c.renderPromptVersions(ctx, params)
	case "/prompts/create":
		return c.renderPromptVersionForm(ctx, params)
	case "/prompts/detail":
		return c.renderPromptVersionDetail(ctx, params)
	// --- Scorers ---
	case "/scorers":
		return c.renderScorers(ctx, params)
	default:
		return components.EmptyState("alert-circle", "Page not found", "The requested page '"+pageRoute+"' does not exist in the Sentinel dashboard."), nil
	}
}

// RenderWidget renders a widget by ID.
func (c *Contributor) RenderWidget(ctx context.Context, widgetID string) (templ.Component, error) {
	if c.engine == nil || c.engine.Store() == nil {
		return nil, contributor.ErrWidgetNotFound
	}

	switch widgetID {
	case "sentinel-stats":
		return c.renderStatsWidget(ctx)
	case "sentinel-recent-runs":
		return c.renderRecentRunsWidget(ctx)
	default:
		return nil, contributor.ErrWidgetNotFound
	}
}

// RenderSettings renders a settings panel by ID.
func (c *Contributor) RenderSettings(ctx context.Context, settingID string) (templ.Component, error) {
	switch settingID {
	case "sentinel-config":
		return c.renderSettings(ctx)
	default:
		return nil, contributor.ErrSettingNotFound
	}
}

// --- Page Renderers ---

func (c *Contributor) renderOverview(ctx context.Context) (templ.Component, error) {
	counts := fetchEntityCounts(ctx, c.engine, "")
	runs, err := fetchRecentRuns(ctx, c.engine, 10)
	if err != nil {
		runs = nil
	}
	activeRuns, err := fetchActiveRuns(ctx, c.engine)
	if err != nil {
		activeRuns = nil
	}
	avgPassRate := computeAveragePassRate(ctx, c.engine)
	suiteNames := buildSuiteNameMap(ctx, c.engine)
	return pages.OverviewPage(counts, runs, activeRuns, avgPassRate, suiteNames), nil
}

func (c *Contributor) renderSuites(ctx context.Context, params contributor.Params) (templ.Component, error) {
	search := params.QueryParams["search"]
	limit := parseIntParam(params.QueryParams, "limit", 20)
	offset := parseIntParam(params.QueryParams, "offset", 0)
	items, total, err := fetchSuitesPaginated(ctx, c.engine, "", limit, offset)
	if err != nil {
		items = nil
		total = 0
	}
	// Filter by search client-side since store doesn't support it.
	if search != "" {
		var filtered []*suite.Suite
		for _, s := range items {
			if containsInsensitive(s.Name, search) || containsInsensitive(s.Description, search) {
				filtered = append(filtered, s)
			}
		}
		items = filtered
	}
	pg := NewPaginationMeta(total, limit, offset)
	return pages.SuitesPage(items, search, pg), nil
}

func (c *Contributor) renderSuiteForm(ctx context.Context, params contributor.Params) (templ.Component, error) {
	idStr := params.QueryParams["id"]
	if idStr != "" {
		suiteID, err := parseIDParam(params.QueryParams, "id")
		if err != nil {
			return nil, contributor.ErrPageNotFound
		}
		s, err := c.engine.GetSuite(ctx, suiteID)
		if err != nil {
			return nil, fmt.Errorf("dashboard: resolve suite for edit: %w", err)
		}
		return pages.SuiteFormPage(s), nil
	}
	return pages.SuiteFormPage(nil), nil
}

func (c *Contributor) renderSuiteDetail(ctx context.Context, params contributor.Params) (templ.Component, error) {
	suiteID, err := parseIDParam(params.QueryParams, "id")
	if err != nil {
		return nil, contributor.ErrPageNotFound
	}
	s, err := c.engine.GetSuite(ctx, suiteID)
	if err != nil {
		return nil, fmt.Errorf("dashboard: resolve suite: %w", err)
	}
	cases, err := c.engine.ListCases(ctx, s.ID)
	if err != nil {
		cases = nil
	}
	caseCount, err := c.engine.CountCases(ctx, s.ID)
	if err != nil {
		caseCount = 0
	}
	runs, err := c.engine.ListRunsBySuite(ctx, s.ID)
	if err != nil {
		runs = nil
	}
	baselines, err := c.engine.ListBaselines(ctx, s.ID)
	if err != nil {
		baselines = nil
	}
	prompts, err := c.engine.ListPromptVersions(ctx, s.ID)
	if err != nil {
		prompts = nil
	}
	return pages.SuiteDetailPage(s, cases, caseCount, runs, baselines, prompts), nil
}

// --- Cases ---

func (c *Contributor) renderCases(ctx context.Context, params contributor.Params) (templ.Component, error) {
	suiteID, err := parseIDParam(params.QueryParams, "suite_id")
	if err != nil {
		return nil, contributor.ErrPageNotFound
	}
	s, err := c.engine.GetSuite(ctx, suiteID)
	if err != nil {
		return nil, fmt.Errorf("dashboard: resolve suite: %w", err)
	}
	items, total, err := fetchCasesList(ctx, c.engine, suiteID)
	if err != nil {
		items = nil
		total = 0
	}
	pg := NewPaginationMeta(total, 20, 0)
	return pages.CasesPage(s, items, pg), nil
}

func (c *Contributor) renderCaseForm(ctx context.Context, params contributor.Params) (templ.Component, error) {
	suiteIDStr := params.QueryParams["suite_id"]
	idStr := params.QueryParams["id"]

	if idStr != "" {
		caseID, err := parseIDParam(params.QueryParams, "id")
		if err != nil {
			return nil, contributor.ErrPageNotFound
		}
		tc, err := c.engine.GetCase(ctx, caseID)
		if err != nil {
			return nil, fmt.Errorf("dashboard: resolve case for edit: %w", err)
		}
		return pages.CaseFormPage(tc, suiteIDStr), nil
	}
	return pages.CaseFormPage(nil, suiteIDStr), nil
}

func (c *Contributor) renderCaseDetail(ctx context.Context, params contributor.Params) (templ.Component, error) {
	caseID, err := parseIDParam(params.QueryParams, "id")
	if err != nil {
		return nil, contributor.ErrPageNotFound
	}
	tc, err := c.engine.GetCase(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("dashboard: resolve case: %w", err)
	}
	return pages.CaseDetailPage(tc), nil
}

// --- Runs ---

func (c *Contributor) renderRuns(ctx context.Context, params contributor.Params) (templ.Component, error) {
	stateFilter := params.QueryParams["state"]
	suiteFilter := params.QueryParams["suite"]
	limit := parseIntParam(params.QueryParams, "limit", 20)
	offset := parseIntParam(params.QueryParams, "offset", 0)

	var suiteID id.SuiteID
	if suiteFilter != "" {
		parsed, err := id.Parse(suiteFilter)
		if err == nil {
			suiteID = parsed
		}
	}

	items, total, err := fetchRunsPaginated(ctx, c.engine, suiteID, stateFilter, limit, offset)
	if err != nil {
		items = nil
		total = 0
	}
	suiteNames := buildSuiteNameMap(ctx, c.engine)
	suites, err := c.engine.ListSuites(ctx, &suite.ListFilter{})
	if err != nil {
		suites = nil
	}
	pg := NewPaginationMeta(total, limit, offset)
	return pages.RunsPage(items, stateFilter, suiteFilter, suites, suiteNames, pg), nil
}

func (c *Contributor) renderRunDetail(ctx context.Context, params contributor.Params) (templ.Component, error) {
	runID, err := parseIDParam(params.QueryParams, "id")
	if err != nil {
		return nil, contributor.ErrPageNotFound
	}
	r, err := c.engine.GetRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("dashboard: resolve run: %w", err)
	}
	results, err := c.engine.ListResults(ctx, r.ID)
	if err != nil {
		results = nil
	}
	stats, err := c.engine.GetResultStats(ctx, r.ID)
	if err != nil {
		stats = nil
	}
	suiteNames := buildSuiteNameMap(ctx, c.engine)
	suiteName := resolveSuiteName(r.SuiteID.String(), suiteNames)
	duration := formatDuration(r.CreatedAt, r.CompletedAt)
	return pages.RunDetailPage(r, results, stats, suiteName, duration), nil
}

func (c *Contributor) renderResultDetail(ctx context.Context, params contributor.Params) (templ.Component, error) {
	runIDParsed, err := parseIDParam(params.QueryParams, "run_id")
	if err != nil {
		return nil, contributor.ErrPageNotFound
	}
	resultIDStr := params.QueryParams["id"]
	if resultIDStr == "" {
		return nil, contributor.ErrPageNotFound
	}

	results, err := c.engine.ListResults(ctx, runIDParsed)
	if err != nil {
		return nil, fmt.Errorf("dashboard: list results: %w", err)
	}

	for _, res := range results {
		if res.ID.String() == resultIDStr {
			return pages.ResultDetailPage(res), nil
		}
	}
	return nil, contributor.ErrPageNotFound
}

func (c *Contributor) renderReport(ctx context.Context, params contributor.Params) (templ.Component, error) {
	runID, err := parseIDParam(params.QueryParams, "id")
	if err != nil {
		return nil, contributor.ErrPageNotFound
	}
	r, err := c.engine.GetRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("dashboard: resolve run for report: %w", err)
	}
	results, err := c.engine.ListResults(ctx, r.ID)
	if err != nil {
		results = nil
	}
	stats, err := c.engine.GetResultStats(ctx, r.ID)
	if err != nil {
		stats = nil
	}
	return pages.ReportPage(r, results, stats), nil
}

// --- Baselines ---

func (c *Contributor) renderBaselines(ctx context.Context, params contributor.Params) (templ.Component, error) {
	suiteFilter := params.QueryParams["suite_id"]
	suites, err := c.engine.ListSuites(ctx, &suite.ListFilter{})
	if err != nil {
		suites = nil
	}

	var allBaselines []*baseline.Baseline
	if suiteFilter != "" {
		parsed, pErr := id.Parse(suiteFilter)
		if pErr == nil {
			allBaselines, err = c.engine.ListBaselines(ctx, parsed)
			if err != nil {
				allBaselines = nil
			}
		}
	} else {
		// Aggregate baselines across all suites.
		for _, s := range suites {
			bl, blErr := c.engine.ListBaselines(ctx, s.ID)
			if blErr == nil {
				allBaselines = append(allBaselines, bl...)
			}
		}
	}

	suiteNames := buildSuiteNameMap(ctx, c.engine)
	pg := NewPaginationMeta(int64(len(allBaselines)), 20, 0)
	return pages.BaselinesPage(allBaselines, suiteFilter, suites, suiteNames, pg), nil
}

func (c *Contributor) renderBaselineDetail(ctx context.Context, params contributor.Params) (templ.Component, error) {
	baselineID, err := parseIDParam(params.QueryParams, "id")
	if err != nil {
		return nil, contributor.ErrPageNotFound
	}
	b, err := c.engine.GetBaseline(ctx, baselineID)
	if err != nil {
		return nil, fmt.Errorf("dashboard: resolve baseline: %w", err)
	}
	suiteNames := buildSuiteNameMap(ctx, c.engine)
	suiteName := resolveSuiteName(b.SuiteID.String(), suiteNames)
	return pages.BaselineDetailPage(b, suiteName), nil
}

// --- Prompt Versions ---

func (c *Contributor) renderPromptVersions(ctx context.Context, params contributor.Params) (templ.Component, error) {
	suiteID, err := parseIDParam(params.QueryParams, "suite_id")
	if err != nil {
		return nil, contributor.ErrPageNotFound
	}
	s, err := c.engine.GetSuite(ctx, suiteID)
	if err != nil {
		return nil, fmt.Errorf("dashboard: resolve suite: %w", err)
	}
	items, err := fetchPromptVersionsList(ctx, c.engine, suiteID)
	if err != nil {
		items = nil
	}
	return pages.PromptVersionsPage(s, items), nil
}

func (c *Contributor) renderPromptVersionForm(_ context.Context, params contributor.Params) (templ.Component, error) {
	suiteIDStr := params.QueryParams["suite_id"]
	return pages.PromptVersionFormPage(suiteIDStr), nil
}

func (c *Contributor) renderPromptVersionDetail(ctx context.Context, params contributor.Params) (templ.Component, error) {
	pvID, err := parseIDParam(params.QueryParams, "id")
	if err != nil {
		return nil, contributor.ErrPageNotFound
	}
	pv, err := c.engine.GetPromptVersion(ctx, pvID)
	if err != nil {
		return nil, fmt.Errorf("dashboard: resolve prompt version: %w", err)
	}
	return pages.PromptVersionDetailPage(pv), nil
}

// --- Scorers ---

func (c *Contributor) renderScorers(_ context.Context, _ contributor.Params) (templ.Component, error) {
	return pages.ScorersPage(), nil
}

// --- Widget Renderers ---

func (c *Contributor) renderStatsWidget(ctx context.Context) (templ.Component, error) {
	counts := fetchEntityCounts(ctx, c.engine, "")
	avgPassRate := computeAveragePassRate(ctx, c.engine)
	return widgets.StatsWidget(counts, avgPassRate), nil
}

func (c *Contributor) renderRecentRunsWidget(ctx context.Context) (templ.Component, error) {
	runs, err := fetchRecentRuns(ctx, c.engine, 10)
	if err != nil {
		runs = nil
	}
	suiteNames := buildSuiteNameMap(ctx, c.engine)
	return widgets.RecentRunsWidget(runs, suiteNames), nil
}

// --- Settings Renderer ---

func (c *Contributor) renderSettings(_ context.Context) (templ.Component, error) {
	if c.engine == nil {
		return nil, contributor.ErrSettingNotFound
	}
	cfg := c.engine.Config()
	return settings.ConfigPanel(cfg), nil
}
