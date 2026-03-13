package dashboard

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xraph/sentinel/dashboard/shared"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/promptversion"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

// PaginationMeta is an alias for shared.PaginationMeta.
type PaginationMeta = shared.PaginationMeta

// NewPaginationMeta is a convenience re-export.
var NewPaginationMeta = shared.NewPaginationMeta

// --- Helper Functions ---

func parseIntParam(params map[string]string, key string, defaultVal int) int {
	v, ok := params[key]
	if !ok || v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}

func parseIDParam(params map[string]string, key string) (id.ID, error) {
	v, ok := params[key]
	if !ok || v == "" {
		return id.ID{}, fmt.Errorf("missing parameter %q", key)
	}
	return id.Parse(v)
}

// EntityCounts is an alias for shared.EntityCounts.
type EntityCounts = shared.EntityCounts

// --- Entity Counts ---

func fetchEntityCounts(ctx context.Context, eng *engine.Engine, appID string) EntityCounts {
	var c EntityCounts

	suites, err := eng.ListSuites(ctx, &suite.ListFilter{AppID: appID})
	if err == nil {
		c.Suites = len(suites)

		// Count all cases and baselines across suites.
		for _, s := range suites {
			caseCount, cErr := eng.CountCases(ctx, s.ID)
			if cErr == nil {
				c.Cases += int(caseCount)
			}

			baselines, bErr := eng.ListBaselines(ctx, s.ID)
			if bErr == nil {
				c.Baselines += len(baselines)
			}
		}
	}

	runs, err := eng.ListRuns(ctx, &evalrun.ListFilter{AppID: appID})
	if err == nil {
		c.Runs = len(runs)
	}

	return c
}

// --- Paginated Fetch Functions ---

func fetchSuitesPaginated(ctx context.Context, eng *engine.Engine, appID string, limit, offset int) ([]*suite.Suite, int64, error) {
	// Fetch all suites matching appID (store doesn't support search/pagination natively in filter).
	all, err := eng.ListSuites(ctx, &suite.ListFilter{AppID: appID})
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(all))
	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

func fetchRunsPaginated(ctx context.Context, eng *engine.Engine, suiteID id.SuiteID, state string, limit, offset int) ([]*evalrun.Run, int64, error) {
	filter := &evalrun.ListFilter{
		SuiteID: suiteID,
		State:   evalrun.RunState(state),
		Limit:   limit,
		Offset:  offset,
	}
	items, err := eng.ListRuns(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Get total count (without pagination).
	allFilter := &evalrun.ListFilter{SuiteID: suiteID, State: evalrun.RunState(state)}
	allItems, aErr := eng.ListRuns(ctx, allFilter)
	var total int64
	if aErr == nil {
		total = int64(len(allItems))
	}
	return items, total, nil
}

func fetchCasesList(ctx context.Context, eng *engine.Engine, suiteID id.SuiteID) ([]*testcase.Case, int64, error) {
	items, err := eng.ListCases(ctx, suiteID)
	if err != nil {
		return nil, 0, err
	}
	return items, int64(len(items)), nil
}

func fetchPromptVersionsList(ctx context.Context, eng *engine.Engine, suiteID id.SuiteID) ([]*promptversion.PromptVersion, error) {
	return eng.ListPromptVersions(ctx, suiteID)
}

func fetchRecentRuns(ctx context.Context, eng *engine.Engine, limit int) ([]*evalrun.Run, error) {
	return eng.ListRuns(ctx, &evalrun.ListFilter{Limit: limit})
}

func fetchActiveRuns(ctx context.Context, eng *engine.Engine) ([]*evalrun.Run, error) {
	return eng.ListRuns(ctx, &evalrun.ListFilter{State: evalrun.StateRunning})
}

// computeAveragePassRate returns the average pass rate across recent completed runs.
func computeAveragePassRate(ctx context.Context, eng *engine.Engine) float64 {
	runs, err := eng.ListRuns(ctx, &evalrun.ListFilter{State: evalrun.StateCompleted})
	if err != nil || len(runs) == 0 {
		return 0
	}
	var total float64
	for _, r := range runs {
		total += r.PassRate
	}
	return total / float64(len(runs))
}

// --- Suite Name Map ---

func buildSuiteNameMap(ctx context.Context, eng *engine.Engine) map[string]string {
	suites, err := eng.ListSuites(ctx, &suite.ListFilter{})
	if err != nil {
		return map[string]string{}
	}
	m := make(map[string]string, len(suites))
	for _, s := range suites {
		m[s.ID.String()] = s.Name
	}
	return m
}

func resolveSuiteName(suiteIDStr string, nameMap map[string]string) string {
	if name, ok := nameMap[suiteIDStr]; ok {
		return name
	}
	return suiteIDStr
}

// --- Formatting Helpers ---

func formatDuration(start time.Time, end *time.Time) string {
	if end == nil {
		return "-"
	}
	d := end.Sub(start)
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func containsInsensitive(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
