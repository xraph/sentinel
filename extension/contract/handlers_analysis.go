package contract

import (
	"context"
	"errors"
	"sort"

	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/comparison"
	"github.com/xraph/sentinel/evalrun"
)

const (
	defaultTrendLimit = 30
	maxTrendLimit     = 100
)

// TrendPoint is one completed run on a suite's trend line.
type TrendPoint struct {
	RunID           string             `json:"runId"`
	CreatedAt       string             `json:"createdAt"`
	PassRate        float64            `json:"passRate"`
	AvgScore        float64            `json:"avgScore"`
	DimensionScores map[string]float64 `json:"dimensionScores"`
	TotalCost       float64            `json:"totalCost"`
	Settings        RunSettingsView    `json:"settings"`
}

type runsTrendInput struct {
	SuiteID string `json:"suiteId"`
	Limit   int    `json:"limit"`
}

type runsTrendOutput struct {
	Points   []TrendPoint `json:"points"`
	Baseline *BaselineRef `json:"baseline,omitempty"`
}

// MetricDelta is one aggregate metric for A, for B, and B minus A.
type MetricDelta struct {
	Metric string  `json:"metric"`
	A      float64 `json:"a"`
	B      float64 `json:"b"`
	Delta  float64 `json:"delta"`
}

// CasePair is one case's result in each run; either side may be missing.
type CasePair struct {
	CaseID   string     `json:"caseId"`
	CaseName string     `json:"caseName"`
	A        *ResultRow `json:"a,omitempty"`
	B        *ResultRow `json:"b,omitempty"`
}

type dimensionsOnly struct {
	A []string `json:"a"`
	B []string `json:"b"`
}

type runsCompareInput struct {
	RunID      string `json:"runId"`
	OtherRunID string `json:"otherRunId"`
}

type runsCompareOutput struct {
	A                RunView            `json:"a"`
	B                RunView            `json:"b"`
	Deltas           []MetricDelta      `json:"deltas"`
	DimensionDeltas  map[string]float64 `json:"dimensionDeltas"`
	DimensionsOnlyIn dimensionsOnly     `json:"dimensionsOnlyIn"`
	Cases            []CasePair         `json:"cases"`
}

func runsTrendHandler(d Deps) func(context.Context, runsTrendInput, dashcontract.Principal) (runsTrendOutput, error) {
	return func(ctx context.Context, in runsTrendInput, p dashcontract.Principal) (runsTrendOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return runsTrendOutput{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return runsTrendOutput{}, d.fail("runs.trend", err)
		}
		limit := in.Limit
		if limit <= 0 {
			limit = defaultTrendLimit
		}
		if limit > maxTrendLimit {
			limit = maxTrendLimit
		}
		runs, err := d.Engine.ListRuns(ctx, &evalrun.ListFilter{AppID: app, SuiteID: s.ID, State: evalrun.StateCompleted, Limit: limit})
		if err != nil {
			return runsTrendOutput{}, d.fail("runs.trend", err)
		}
		out := runsTrendOutput{Points: make([]TrendPoint, 0, len(runs))}
		for i := len(runs) - 1; i >= 0; i-- { // the store answers newest first
			r := runs[i]
			out.Points = append(out.Points, TrendPoint{RunID: r.ID.String(), CreatedAt: ts(r.CreatedAt), PassRate: r.PassRate,
				AvgScore: r.AvgScore, DimensionScores: dimsOrEmpty(r.DimensionScores), TotalCost: r.TotalCost, Settings: settingsView(r)})
		}
		b, err := d.Engine.GetLatestBaseline(ctx, s.ID)
		switch {
		case err == nil:
			out.Baseline = &BaselineRef{ID: b.ID.String(), Name: b.Name, PassRate: b.PassRate}
		case !errors.Is(err, sentinel.ErrBaselineNotFound):
			return runsTrendOutput{}, d.fail("runs.trend", err)
		}
		return out, nil
	}
}

func runsCompareHandler(d Deps) func(context.Context, runsCompareInput, dashcontract.Principal) (runsCompareOutput, error) {
	return func(ctx context.Context, in runsCompareInput, p dashcontract.Principal) (runsCompareOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return runsCompareOutput{}, err
		}
		a, err := d.runInApp(ctx, app, in.RunID)
		if err != nil {
			return runsCompareOutput{}, d.fail("runs.compare", err)
		}
		b, err := d.runInApp(ctx, app, in.OtherRunID)
		if err != nil {
			return runsCompareOutput{}, d.fail("runs.compare", err)
		}
		if a.SuiteID.String() != b.SuiteID.String() {
			return runsCompareOutput{}, badRequest("runs from different suites have no cases in common to compare")
		}
		names, err := d.suiteNames(ctx, app)
		if err != nil {
			return runsCompareOutput{}, d.fail("runs.compare", err)
		}
		out := runsCompareOutput{Deltas: []MetricDelta{}, DimensionDeltas: map[string]float64{}, DimensionsOnlyIn: dimensionsOnly{A: []string{}, B: []string{}}, Cases: []CasePair{}}
		if out.A, err = d.runView(ctx, a, names[a.SuiteID.String()]); err != nil {
			return runsCompareOutput{}, d.fail("runs.compare", err)
		}
		if out.B, err = d.runView(ctx, b, names[b.SuiteID.String()]); err != nil {
			return runsCompareOutput{}, d.fail("runs.compare", err)
		}
		statsA, err := d.Engine.GetResultStats(ctx, a.ID)
		if err != nil {
			return runsCompareOutput{}, d.fail("runs.compare", err)
		}
		statsB, err := d.Engine.GetResultStats(ctx, b.ID)
		if err != nil {
			return runsCompareOutput{}, d.fail("runs.compare", err)
		}
		for _, sd := range comparison.Diff(statsB, statsA).Deltas { // Diff(current, baseline): B is current
			out.Deltas = append(out.Deltas, MetricDelta{Metric: sd.Metric, A: sd.Baseline, B: sd.Current, Delta: sd.Delta})
		}
		// Diff counts a dimension missing on one side as zero; compare only
		// shared dimensions and list the rest.
		for dim, va := range statsA.DimensionScores {
			if vb, ok := statsB.DimensionScores[dim]; ok {
				out.DimensionDeltas[dim] = vb - va
			} else {
				out.DimensionsOnlyIn.A = append(out.DimensionsOnlyIn.A, dim)
			}
		}
		for dim := range statsB.DimensionScores {
			if _, ok := statsA.DimensionScores[dim]; !ok {
				out.DimensionsOnlyIn.B = append(out.DimensionsOnlyIn.B, dim)
			}
		}
		sort.Strings(out.DimensionsOnlyIn.A)
		sort.Strings(out.DimensionsOnlyIn.B)

		cases, err := d.caseIndex(ctx, a.SuiteID)
		if err != nil {
			return runsCompareOutput{}, d.fail("runs.compare", err)
		}
		resA, err := d.Engine.ListResults(ctx, a.ID)
		if err != nil {
			return runsCompareOutput{}, d.fail("runs.compare", err)
		}
		resB, err := d.Engine.ListResults(ctx, b.ID)
		if err != nil {
			return runsCompareOutput{}, d.fail("runs.compare", err)
		}
		index := map[string]int{}
		for _, r := range resA {
			row := resultRow(r, cases)
			index[row.CaseID] = len(out.Cases)
			out.Cases = append(out.Cases, CasePair{CaseID: row.CaseID, CaseName: row.CaseName, A: &row})
		}
		for _, r := range resB {
			row := resultRow(r, cases)
			if i, ok := index[row.CaseID]; ok {
				out.Cases[i].B = &row
				continue
			}
			out.Cases = append(out.Cases, CasePair{CaseID: row.CaseID, CaseName: row.CaseName, B: &row})
		}
		return out, nil
	}
}
