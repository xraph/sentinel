package contract

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/suite"
)

// RegressionView is an explicit state machine, so no page has to infer
// whether a missing baseline means "passed":
//   - running: the run has not finished; no verdict yet.
//   - notComparable: Reason is runFailed, runCancelled or otherSuite.
//   - noBaseline: the suite has no current baseline to compare against.
//   - compared: the remaining fields are set.
type RegressionView struct {
	State             string              `json:"state"`
	Reason            string              `json:"reason,omitempty"`
	Baseline          *BaselineRef        `json:"baseline,omitempty"`
	Threshold         *float64            `json:"threshold,omitempty"`
	ThresholdSource   string              `json:"thresholdSource,omitempty"`
	HasRegression     bool                `json:"hasRegression"`
	WorstDelta        *float64            `json:"worstDelta,omitempty"`
	PassRateDelta     float64             `json:"passRateDelta"`
	AvgScoreDelta     float64             `json:"avgScoreDelta"`
	DimensionDeltas   map[string]float64  `json:"dimensionDeltas,omitempty"`
	RegressedCases    []RegressedCaseView `json:"regressedCases,omitempty"`
	MissingCases      []CaseNameView      `json:"missingCases,omitempty"`
	NewCases          []CaseNameView      `json:"newCases,omitempty"`
	MissingDimensions []string            `json:"missingDimensions,omitempty"`
}

// RegressedCaseView is a case that fell beyond the threshold.
type RegressedCaseView struct {
	CaseID   string  `json:"caseId"`
	CaseName string  `json:"caseName"`
	OldScore float64 `json:"oldScore"`
	NewScore float64 `json:"newScore"`
	Delta    float64 `json:"delta"`
}

// CaseNameView names a case.
type CaseNameView struct {
	CaseID   string `json:"caseId"`
	CaseName string `json:"caseName"`
}

// BaselineView is a saved baseline.
type BaselineView struct {
	ID              string             `json:"id"`
	SuiteID         string             `json:"suiteId"`
	SuiteName       string             `json:"suiteName"`
	RunID           string             `json:"runId"`
	Name            string             `json:"name"`
	PassRate        float64            `json:"passRate"`
	AvgScore        float64            `json:"avgScore"`
	DimensionScores map[string]float64 `json:"dimensionScores"`
	CaseCount       int                `json:"caseCount"`
	IsCurrent       bool               `json:"isCurrent"`
	CreatedAt       string             `json:"createdAt"`
}

// BaselineResultView is one case as the baseline recorded it.
type BaselineResultView struct {
	CaseID          string             `json:"caseId"`
	CaseName        string             `json:"caseName"`
	Score           float64            `json:"score"`
	Status          string             `json:"status"`
	DimensionScores map[string]float64 `json:"dimensionScores"`
}

type baselineDetailOutput struct {
	BaselineView
	Results []BaselineResultView `json:"results"`
}

type runsDetailOutput struct {
	Run        RunView        `json:"run"`
	Regression RegressionView `json:"regression"`
}

type runsRegressionInput struct {
	RunID      string   `json:"runId"`
	BaselineID string   `json:"baselineId"`
	Threshold  *float64 `json:"threshold"`
}

type baselineRef struct {
	BaselineID string `json:"baselineId"`
}

type baselinesListInput struct {
	SuiteID string `json:"suiteId"`
}

type baselinesListOutput struct {
	Items []BaselineView `json:"items"`
}

type baselinesSaveInput struct {
	RunID string `json:"runId"`
	Name  string `json:"name"`
}

type baselinesDeleteOutput struct {
	BaselineID string `json:"baselineId"`
}

func baselineView(b *baseline.Baseline, suiteName string) BaselineView {
	return BaselineView{
		ID: b.ID.String(), SuiteID: b.SuiteID.String(), SuiteName: suiteName, RunID: b.RunID.String(), Name: b.Name,
		PassRate: b.PassRate, AvgScore: b.AvgScore, DimensionScores: dimsOrEmpty(b.DimensionScores),
		CaseCount: len(b.Results), IsCurrent: b.IsCurrent, CreatedAt: ts(b.CreatedAt),
	}
}

// baselineInApp loads a baseline through its suite and answers NOT_FOUND
// for a malformed id, a missing baseline and another app's alike.
func (d Deps) baselineInApp(ctx context.Context, app, rawID string) (*baseline.Baseline, *suite.Suite, error) {
	bid, err := id.ParseBaselineID(rawID)
	if err != nil {
		return nil, nil, notFound("baseline not found")
	}
	b, err := d.Engine.GetBaseline(ctx, bid)
	if errors.Is(err, sentinel.ErrBaselineNotFound) {
		return nil, nil, notFound("baseline not found")
	}
	if err != nil {
		return nil, nil, err
	}
	s, err := d.suiteInApp(ctx, app, b.SuiteID.String())
	if err != nil {
		// Only a contract NOT_FOUND collapses into the baseline's own
		// answer. Any other error (a store outage) passes through so the
		// caller's d.fail maps and logs it.
		var ce *dashcontract.Error
		if errors.As(err, &ce) && ce.Code == dashcontract.CodeNotFound {
			return nil, nil, notFound("baseline not found")
		}
		return nil, nil, err
	}
	return b, s, nil
}

// regressionFor compares a run with a baseline: the one named, or the
// suite's current one. The threshold comes from the override, else the
// run's recorded regression_threshold, else config, and the answer names
// which.
func (d Deps) regressionFor(ctx context.Context, app string, r *evalrun.Run, baselineID string, override *float64) (RegressionView, error) {
	switch r.State {
	case evalrun.StateRunning:
		return RegressionView{State: "running"}, nil
	case evalrun.StateFailed:
		return RegressionView{State: "notComparable", Reason: "runFailed"}, nil
	case evalrun.StateCancelled:
		return RegressionView{State: "notComparable", Reason: "runCancelled"}, nil
	}
	var b *baseline.Baseline
	if baselineID != "" {
		got, _, err := d.baselineInApp(ctx, app, baselineID)
		if err != nil {
			return RegressionView{}, err
		}
		if got.SuiteID.String() != r.SuiteID.String() {
			return RegressionView{State: "notComparable", Reason: "otherSuite"}, nil
		}
		b = got
	} else {
		got, err := d.Engine.GetLatestBaseline(ctx, r.SuiteID)
		if errors.Is(err, sentinel.ErrBaselineNotFound) {
			return RegressionView{State: "noBaseline"}, nil
		}
		if err != nil {
			return RegressionView{}, err
		}
		b = got
	}
	threshold, source := d.Engine.Config().RegressionThreshold, "config"
	if v := evalrun.SettingsFrom(r.Config).RegressionThreshold; v != nil {
		threshold, source = *v, "run"
	}
	if override != nil {
		threshold, source = *override, "override"
	}
	stats, err := d.Engine.GetResultStats(ctx, r.ID)
	if err != nil {
		return RegressionView{}, err
	}
	results, err := d.Engine.ListResults(ctx, r.ID)
	if err != nil {
		return RegressionView{}, err
	}
	rr := baseline.DetectRegression(stats, results, b, threshold)
	worst := rr.WorstDelta()
	v := RegressionView{
		State: "compared", Baseline: &BaselineRef{ID: b.ID.String(), Name: b.Name, PassRate: b.PassRate},
		Threshold: &threshold, ThresholdSource: source, HasRegression: rr.HasRegression, WorstDelta: &worst,
		PassRateDelta: rr.PassRateDelta, AvgScoreDelta: rr.AvgScoreDelta, DimensionDeltas: rr.DimensionDeltas,
		MissingDimensions: rr.MissingDimensions,
	}
	for _, c := range rr.RegressedCases {
		v.RegressedCases = append(v.RegressedCases, RegressedCaseView{CaseID: c.CaseID, CaseName: c.CaseName, OldScore: c.OldScore, NewScore: c.NewScore, Delta: c.Delta})
	}
	for _, c := range rr.MissingCases {
		v.MissingCases = append(v.MissingCases, CaseNameView{CaseID: c.CaseID, CaseName: c.CaseName})
	}
	for _, c := range rr.NewCases {
		v.NewCases = append(v.NewCases, CaseNameView{CaseID: c.CaseID, CaseName: c.CaseName})
	}
	return v, nil
}

func lastProgress(ctx context.Context, d Deps, r *evalrun.Run) (*string, error) {
	results, err := d.Engine.ListResults(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	var latest time.Time
	for _, res := range results {
		if res.CreatedAt.After(latest) {
			latest = res.CreatedAt
		}
	}
	return tsPtr(&latest), nil
}

func runsDetailHandler(d Deps) func(context.Context, runRef, dashcontract.Principal) (runsDetailOutput, error) {
	return func(ctx context.Context, in runRef, p dashcontract.Principal) (runsDetailOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return runsDetailOutput{}, err
		}
		r, err := d.runInApp(ctx, app, in.RunID)
		if err != nil {
			return runsDetailOutput{}, d.fail("runs.detail", err)
		}
		names, err := d.suiteNames(ctx, app)
		if err != nil {
			return runsDetailOutput{}, d.fail("runs.detail", err)
		}
		view, err := d.runView(ctx, r, names[r.SuiteID.String()])
		if err != nil {
			return runsDetailOutput{}, d.fail("runs.detail", err)
		}
		if view.LastProgressAt, err = lastProgress(ctx, d, r); err != nil {
			return runsDetailOutput{}, d.fail("runs.detail", err)
		}
		reg, err := d.regressionFor(ctx, app, r, "", nil)
		if err != nil {
			return runsDetailOutput{}, d.fail("runs.detail", err)
		}
		return runsDetailOutput{Run: view, Regression: reg}, nil
	}
}

func runsRegressionHandler(d Deps) func(context.Context, runsRegressionInput, dashcontract.Principal) (RegressionView, error) {
	return func(ctx context.Context, in runsRegressionInput, p dashcontract.Principal) (RegressionView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return RegressionView{}, err
		}
		if in.Threshold != nil && (*in.Threshold < 0 || *in.Threshold > 1) {
			return RegressionView{}, badRequest("threshold must be between 0 and 1")
		}
		r, err := d.runInApp(ctx, app, in.RunID)
		if err != nil {
			return RegressionView{}, d.fail("runs.regression", err)
		}
		v, err := d.regressionFor(ctx, app, r, in.BaselineID, in.Threshold)
		if err != nil {
			return RegressionView{}, d.fail("runs.regression", err)
		}
		return v, nil
	}
}

func baselinesListHandler(d Deps) func(context.Context, baselinesListInput, dashcontract.Principal) (baselinesListOutput, error) {
	return func(ctx context.Context, in baselinesListInput, p dashcontract.Principal) (baselinesListOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return baselinesListOutput{}, err
		}
		var suites []*suite.Suite
		if in.SuiteID != "" {
			s, serr := d.suiteInApp(ctx, app, in.SuiteID)
			if serr != nil {
				return baselinesListOutput{}, d.fail("baselines.list", serr)
			}
			suites = []*suite.Suite{s}
		} else if suites, err = d.Engine.ListSuites(ctx, &suite.ListFilter{AppID: app}); err != nil {
			return baselinesListOutput{}, d.fail("baselines.list", err)
		}
		out := baselinesListOutput{Items: []BaselineView{}}
		for _, s := range suites {
			list, err := d.Engine.ListBaselines(ctx, s.ID)
			if err != nil {
				return baselinesListOutput{}, d.fail("baselines.list", err)
			}
			for _, b := range list {
				out.Items = append(out.Items, baselineView(b, s.Name))
			}
		}
		sort.SliceStable(out.Items, func(i, j int) bool { return out.Items[i].CreatedAt > out.Items[j].CreatedAt })
		return out, nil
	}
}

func baselinesDetailHandler(d Deps) func(context.Context, baselineRef, dashcontract.Principal) (baselineDetailOutput, error) {
	return func(ctx context.Context, in baselineRef, p dashcontract.Principal) (baselineDetailOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return baselineDetailOutput{}, err
		}
		b, s, err := d.baselineInApp(ctx, app, in.BaselineID)
		if err != nil {
			return baselineDetailOutput{}, d.fail("baselines.detail", err)
		}
		out := baselineDetailOutput{BaselineView: baselineView(b, s.Name), Results: []BaselineResultView{}}
		for _, r := range b.Results {
			out.Results = append(out.Results, BaselineResultView{CaseID: r.CaseID.String(), CaseName: r.CaseName, Score: r.Score,
				Status: r.Status, DimensionScores: dimsOrEmpty(r.DimensionScores)})
		}
		return out, nil
	}
}

// baselinesSave saves a completed run as its suite's current baseline. The
// suite comes from the run, never from the request: a baseline pointing at
// another suite's run would block that suite's delete under foreign keys.
func baselinesSaveHandler(d Deps) func(context.Context, baselinesSaveInput, dashcontract.Principal) (BaselineView, error) {
	return func(ctx context.Context, in baselinesSaveInput, p dashcontract.Principal) (BaselineView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return BaselineView{}, err
		}
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return BaselineView{}, badRequest("a baseline needs a name")
		}
		r, err := d.runInApp(ctx, app, in.RunID)
		if err != nil {
			return BaselineView{}, d.fail("baselines.save", err)
		}
		if r.State != evalrun.StateCompleted {
			return BaselineView{}, conflict("only a completed run can become a baseline")
		}
		results, err := d.Engine.ListResults(ctx, r.ID)
		if err != nil {
			return BaselineView{}, d.fail("baselines.save", err)
		}
		b := &baseline.Baseline{SuiteID: r.SuiteID, RunID: r.ID, Name: name, PassRate: r.PassRate, AvgScore: r.AvgScore,
			DimensionScores: dimsOrEmpty(r.DimensionScores), Results: make([]baseline.Result, 0, len(results)), IsCurrent: true,
			CreatedAt: time.Now().UTC()}
		for _, res := range results {
			b.Results = append(b.Results, baseline.Result{CaseID: res.CaseID, CaseName: res.CaseName, Score: res.Score,
				Status: string(res.Status), DimensionScores: dimsOrEmpty(res.DimensionScores)})
		}
		if err = d.Engine.SaveBaseline(ctx, b); err != nil {
			return BaselineView{}, d.fail("baselines.save", err)
		}
		names, err := d.suiteNames(ctx, app)
		if err != nil {
			return BaselineView{}, d.fail("baselines.save", err)
		}
		return baselineView(b, names[b.SuiteID.String()]), nil
	}
}

func baselinesDeleteHandler(d Deps) func(context.Context, baselineRef, dashcontract.Principal) (baselinesDeleteOutput, error) {
	return func(ctx context.Context, in baselineRef, p dashcontract.Principal) (baselinesDeleteOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return baselinesDeleteOutput{}, err
		}
		b, _, err := d.baselineInApp(ctx, app, in.BaselineID)
		if err != nil {
			return baselinesDeleteOutput{}, d.fail("baselines.delete", err)
		}
		if err := d.Engine.DeleteBaseline(ctx, b.ID); err != nil {
			return baselinesDeleteOutput{}, d.fail("baselines.delete", err)
		}
		return baselinesDeleteOutput{BaselineID: b.ID.String()}, nil
	}
}
