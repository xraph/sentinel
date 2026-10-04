package contract

import (
	"context"

	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"

	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/suite"
)

const (
	recentRunsLimit        = 10
	regressionLookbackRuns = 20
)

// RegressionSummary is one recently regressed run.
type RegressionSummary struct {
	RunID      string      `json:"runId"`
	SuiteID    string      `json:"suiteId"`
	SuiteName  string      `json:"suiteName"`
	CreatedAt  string      `json:"createdAt"`
	Baseline   BaselineRef `json:"baseline"`
	WorstDelta float64     `json:"worstDelta"`
}

type overviewInput struct{}

type overviewOutput struct {
	SuiteCount        int                 `json:"suiteCount"`
	CaseCount         int64               `json:"caseCount"`
	RunCount          int                 `json:"runCount"`
	RecentRuns        []RunView           `json:"recentRuns"`
	ActiveRuns        []RunView           `json:"activeRuns"`
	RecentRegressions []RegressionSummary `json:"recentRegressions"`
	TargetsRegistered bool                `json:"targetsRegistered"`
}

// overviewStatsHandler counts this app's suites, cases and runs. RunCount
// reads every run id for the app, since no store has a count; it is the
// one unbounded read here and fine at dashboard scale.
func overviewStatsHandler(d Deps) func(context.Context, overviewInput, dashcontract.Principal) (overviewOutput, error) {
	return func(ctx context.Context, _ overviewInput, p dashcontract.Principal) (overviewOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return overviewOutput{}, err
		}
		out := overviewOutput{RecentRuns: []RunView{}, ActiveRuns: []RunView{}, RecentRegressions: []RegressionSummary{},
			TargetsRegistered: len(d.Engine.Targets()) > 0}
		suites, err := d.Engine.ListSuites(ctx, &suite.ListFilter{AppID: app})
		if err != nil {
			return overviewOutput{}, d.fail("overview.stats", err)
		}
		out.SuiteCount = len(suites)
		names := make(map[string]string, len(suites))
		for _, s := range suites {
			names[s.ID.String()] = s.Name
			n, cerr := d.Engine.CountCases(ctx, s.ID)
			if cerr != nil {
				return overviewOutput{}, d.fail("overview.stats", cerr)
			}
			out.CaseCount += n
		}
		all, err := d.Engine.ListRuns(ctx, &evalrun.ListFilter{AppID: app})
		if err != nil {
			return overviewOutput{}, d.fail("overview.stats", err)
		}
		out.RunCount = len(all)
		for i, r := range all { // newest first
			recent := i < recentRunsLimit
			running := r.State == evalrun.StateRunning
			if !recent && !running {
				continue
			}
			// One view serves both lists when a running run is also recent.
			v, verr := d.runView(ctx, r, names[r.SuiteID.String()])
			if verr != nil {
				return overviewOutput{}, d.fail("overview.stats", verr)
			}
			if recent {
				out.RecentRuns = append(out.RecentRuns, v)
			}
			if running {
				out.ActiveRuns = append(out.ActiveRuns, v)
			}
		}
		completed, err := d.Engine.ListRuns(ctx, &evalrun.ListFilter{AppID: app, State: evalrun.StateCompleted, Limit: regressionLookbackRuns})
		if err != nil {
			return overviewOutput{}, d.fail("overview.stats", err)
		}
		for _, r := range completed {
			reg, rerr := d.regressionFor(ctx, app, r, "", nil)
			if rerr != nil {
				return overviewOutput{}, d.fail("overview.stats", rerr)
			}
			// A compared answer without its baseline or delta is skipped, not a crash.
			if reg.State != "compared" || !reg.HasRegression || reg.Baseline == nil || reg.WorstDelta == nil {
				continue
			}
			out.RecentRegressions = append(out.RecentRegressions, RegressionSummary{
				RunID: r.ID.String(), SuiteID: r.SuiteID.String(), SuiteName: names[r.SuiteID.String()],
				CreatedAt: ts(r.CreatedAt), Baseline: *reg.Baseline, WorstDelta: *reg.WorstDelta,
			})
		}
		return out, nil
	}
}
