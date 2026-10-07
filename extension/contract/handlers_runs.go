package contract

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

const (
	defaultRunsLimit = 25
	maxRunsLimit     = 100
)

// RunSettingsView is what a run recorded when it started. A field is
// absent when the run predates recording, which means "not recorded",
// never today's configuration.
type RunSettingsView struct {
	PassThreshold       *float64 `json:"passThreshold,omitempty"`
	RegressionThreshold *float64 `json:"regressionThreshold,omitempty"`
	Concurrency         *int     `json:"concurrency,omitempty"`
	Target              string   `json:"target,omitempty"`
	Scorers             []string `json:"scorers,omitempty"`
	Model               string   `json:"model,omitempty"`
	PromptVersionID     string   `json:"promptVersionId,omitempty"`
}

// RunView is a run. CompletedCases and Errored always come from the stored
// results, which are the only place those numbers live. While a run is
// running its counters come from the results stored so far.
type RunView struct {
	ID              string             `json:"id"`
	SuiteID         string             `json:"suiteId"`
	SuiteName       string             `json:"suiteName"`
	Model           string             `json:"model"`
	Temperature     float64            `json:"temperature"`
	State           string             `json:"state"`
	TotalCases      int                `json:"totalCases"`
	CompletedCases  int                `json:"completedCases"`
	Passed          int                `json:"passed"`
	Failed          int                `json:"failed"`
	Errored         int                `json:"errored"`
	PassRate        float64            `json:"passRate"`
	AvgScore        float64            `json:"avgScore"`
	AvgLatencyMs    int                `json:"avgLatencyMs"`
	TotalTokens     int                `json:"totalTokens"`
	TotalCost       float64            `json:"totalCost"`
	DimensionScores map[string]float64 `json:"dimensionScores"`
	Settings        RunSettingsView    `json:"settings"`
	// Error is why the run failed, as the engine recorded it. The engine
	// writes a generic message here for a store failure; it never carries
	// store error text.
	Error          string  `json:"error,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	CompletedAt    *string `json:"completedAt,omitempty"`
	LastProgressAt *string `json:"lastProgressAt,omitempty"`
}

// ScorerVerdict is one scorer's pass or fail on a result.
type ScorerVerdict struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
}

// ResultRow is one case's result without its output, which can be large.
type ResultRow struct {
	ID              string             `json:"id"`
	CaseID          string             `json:"caseId"`
	CaseName        string             `json:"caseName"`
	Status          string             `json:"status"`
	Score           float64            `json:"score"`
	LatencyMs       int                `json:"latencyMs"`
	TokensUsed      int                `json:"tokensUsed"`
	Cost            float64            `json:"cost"`
	DimensionScores map[string]float64 `json:"dimensionScores"`
	RedTeam         *RedTeamRef        `json:"redTeam,omitempty"`
	// Scorers is each scorer's verdict on the case, in the order they ran.
	// Reasons and details stay on results.detail: a reason can quote the
	// output, and a red-team output is only shown when asked for.
	Scorers []ScorerVerdict `json:"scorers"`
	// Error carries the target's own error text for this case (what
	// target.Call returned), or the scorers' when one could not judge it,
	// stored as written. It comes from the code under test and the judges,
	// never from the store.
	Error string `json:"error,omitempty"`
}

// ScorerResultView is one scorer's verdict on a case.
type ScorerResultView struct {
	ScorerName string         `json:"scorerName"`
	Score      float64        `json:"score"`
	Passed     bool           `json:"passed"`
	Reason     string         `json:"reason"`
	Dimension  string         `json:"dimension,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

// TraceView is an agent run's trace.
type TraceView struct {
	Steps     []StepView `json:"steps"`
	ToolCalls []ToolView `json:"toolCalls"`
}

// StepView is one reasoning step.
type StepView struct {
	Index      int    `json:"index"`
	Type       string `json:"type"`
	Output     string `json:"output"`
	TokensUsed int    `json:"tokensUsed"`
}

// ToolView is one tool call.
type ToolView struct {
	ToolName  string `json:"toolName"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
	Error     string `json:"error,omitempty"`
}

// ResultView is one case's full result, output included. The output may
// be hostile content (a jailbreak the model complied with); the client
// renders it as text and collapses red-team outputs.
type ResultView struct {
	ResultRow
	Output        string             `json:"output"`
	OutputLength  int                `json:"outputLength"`
	ScorerResults []ScorerResultView `json:"scorerResults"`
	RunTrace      *TraceView         `json:"runTrace,omitempty"`
}

type runRef struct {
	RunID string `json:"runId"`
}

type runsListInput struct {
	SuiteID string `json:"suiteId"`
	State   string `json:"state"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

type runsListOutput struct {
	Items   []RunView `json:"items"`
	HasMore bool      `json:"hasMore"`
}

type runsResultsInput struct {
	RunID  string `json:"runId"`
	Status string `json:"status"`
}

type resultCounts struct {
	Pass  int `json:"pass"`
	Fail  int `json:"fail"`
	Error int `json:"error"`
}

type runsResultsOutput struct {
	Items  []ResultRow  `json:"items"`
	Counts resultCounts `json:"counts"`
}

type resultRef struct {
	RunID    string `json:"runId"`
	ResultID string `json:"resultId"`
}

var runStates = map[string]bool{"": true, "running": true, "completed": true, "failed": true, "cancelled": true}

// runInApp loads a run and answers NOT_FOUND for a malformed id, a missing
// run and another app's run alike. Runs carry their suite's app id.
func (d Deps) runInApp(ctx context.Context, app, rawID string) (*evalrun.Run, error) {
	rid, err := id.ParseEvalRunID(rawID)
	if err != nil {
		return nil, notFound("run not found")
	}
	r, err := d.Engine.GetRun(ctx, rid)
	if errors.Is(err, sentinel.ErrRunNotFound) {
		return nil, notFound("run not found")
	}
	if err != nil {
		return nil, err
	}
	if r.AppID != app {
		return nil, notFound("run not found")
	}
	return r, nil
}

func settingsView(r *evalrun.Run) RunSettingsView {
	s := evalrun.SettingsFrom(r.Config)
	return RunSettingsView{
		PassThreshold: s.PassThreshold, RegressionThreshold: s.RegressionThreshold, Concurrency: s.Concurrency,
		Target: s.Target, Scorers: s.Scorers, Model: s.Model, PromptVersionID: s.PromptVersionID,
	}
}

func (d Deps) runView(ctx context.Context, r *evalrun.Run, suiteName string) (RunView, error) {
	st, err := d.Engine.GetResultStats(ctx, r.ID)
	if err != nil {
		return RunView{}, err
	}
	v := RunView{
		ID: r.ID.String(), SuiteID: r.SuiteID.String(), SuiteName: suiteName, Model: r.Model, Temperature: r.Temperature,
		State: string(r.State), TotalCases: r.TotalCases, CompletedCases: st.TotalCases, Errored: st.Errored,
		Passed: r.Passed, Failed: r.Failed, PassRate: r.PassRate, AvgScore: r.AvgScore, AvgLatencyMs: r.AvgLatencyMs,
		TotalTokens: r.TotalTokens, TotalCost: r.TotalCost, DimensionScores: dimsOrEmpty(r.DimensionScores),
		Settings: settingsView(r), Error: r.Error, CreatedAt: ts(r.CreatedAt), CompletedAt: tsPtr(r.CompletedAt),
	}
	if r.State == evalrun.StateRunning {
		v.Passed, v.Failed, v.PassRate, v.AvgScore = st.Passed, st.Failed, st.PassRate, st.AvgScore
		v.AvgLatencyMs, v.TotalTokens, v.TotalCost = st.AvgLatencyMs, st.TotalTokens, st.TotalCost
		v.DimensionScores = dimsOrEmpty(st.DimensionScores)
	}
	return v, nil
}

func (d Deps) suiteNames(ctx context.Context, app string) (map[string]string, error) {
	list, err := d.Engine.ListSuites(ctx, &suite.ListFilter{AppID: app})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(list))
	for _, s := range list {
		out[s.ID.String()] = s.Name
	}
	return out, nil
}

// caseIndex maps a suite's case ids to cases. A result whose case was
// deleted after the run is simply absent from it.
func (d Deps) caseIndex(ctx context.Context, suiteID id.SuiteID) (map[string]*testcase.Case, error) {
	list, err := d.Engine.ListCases(ctx, suiteID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*testcase.Case, len(list))
	for _, tc := range list {
		out[tc.ID.String()] = tc
	}
	return out, nil
}

func resultRow(r *evalrun.Result, cases map[string]*testcase.Case) ResultRow {
	row := ResultRow{
		ID: r.ID.String(), CaseID: r.CaseID.String(), CaseName: r.CaseName, Status: string(r.Status), Score: r.Score,
		LatencyMs: r.LatencyMs, TokensUsed: r.TokensUsed, Cost: r.Cost, DimensionScores: dimsOrEmpty(r.DimensionScores), Error: r.Error,
		Scorers: make([]ScorerVerdict, 0, len(r.ScorerResults)),
	}
	for _, sr := range r.ScorerResults {
		row.Scorers = append(row.Scorers, ScorerVerdict{Name: sr.ScorerName, Passed: sr.Passed})
	}
	if tc := cases[row.CaseID]; tc != nil {
		if at := attackTypeOf(tc); at != "" {
			row.RedTeam = &RedTeamRef{AttackType: at}
		}
	}
	return row
}

func traceView(t *evalrun.RunTrace) *TraceView {
	if t == nil {
		return nil
	}
	v := &TraceView{Steps: []StepView{}, ToolCalls: []ToolView{}}
	for _, s := range t.Steps {
		v.Steps = append(v.Steps, StepView{Index: s.Index, Type: s.Type, Output: s.Output, TokensUsed: s.TokensUsed})
	}
	for _, c := range t.ToolCalls {
		v.ToolCalls = append(v.ToolCalls, ToolView{ToolName: c.ToolName, Arguments: c.Arguments, Result: c.Result, Error: c.Error})
	}
	return v
}

func runsListHandler(d Deps) func(context.Context, runsListInput, dashcontract.Principal) (runsListOutput, error) {
	return func(ctx context.Context, in runsListInput, p dashcontract.Principal) (runsListOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return runsListOutput{}, err
		}
		if !runStates[in.State] {
			return runsListOutput{}, badRequest(fmt.Sprintf("unknown run state %q", in.State))
		}
		limit := in.Limit
		if limit <= 0 {
			limit = defaultRunsLimit
		}
		if limit > maxRunsLimit {
			limit = maxRunsLimit
		}
		if in.Offset < 0 {
			return runsListOutput{}, badRequest("offset cannot be negative")
		}
		filter := &evalrun.ListFilter{AppID: app, State: evalrun.RunState(in.State), Limit: limit + 1, Offset: in.Offset}
		if in.SuiteID != "" {
			s, serr := d.suiteInApp(ctx, app, in.SuiteID)
			if serr != nil {
				return runsListOutput{}, d.fail("runs.list", serr)
			}
			filter.SuiteID = s.ID
		}
		runs, err := d.Engine.ListRuns(ctx, filter)
		if err != nil {
			return runsListOutput{}, d.fail("runs.list", err)
		}
		out := runsListOutput{Items: []RunView{}}
		if len(runs) > limit {
			out.HasMore = true
			runs = runs[:limit]
		}
		names, err := d.suiteNames(ctx, app)
		if err != nil {
			return runsListOutput{}, d.fail("runs.list", err)
		}
		for _, r := range runs {
			v, err := d.runView(ctx, r, names[r.SuiteID.String()])
			if err != nil {
				return runsListOutput{}, d.fail("runs.list", err)
			}
			out.Items = append(out.Items, v)
		}
		return out, nil
	}
}

func runsResultsHandler(d Deps) func(context.Context, runsResultsInput, dashcontract.Principal) (runsResultsOutput, error) {
	return func(ctx context.Context, in runsResultsInput, p dashcontract.Principal) (runsResultsOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return runsResultsOutput{}, err
		}
		switch in.Status {
		case "", "pass", "fail", "error":
		default:
			return runsResultsOutput{}, badRequest(fmt.Sprintf("unknown result status %q", in.Status))
		}
		r, err := d.runInApp(ctx, app, in.RunID)
		if err != nil {
			return runsResultsOutput{}, d.fail("runs.results", err)
		}
		results, err := d.Engine.ListResults(ctx, r.ID)
		if err != nil {
			return runsResultsOutput{}, d.fail("runs.results", err)
		}
		cases, err := d.caseIndex(ctx, r.SuiteID)
		if err != nil {
			return runsResultsOutput{}, d.fail("runs.results", err)
		}
		out := runsResultsOutput{Items: []ResultRow{}}
		for _, res := range results {
			switch res.Status {
			case evalrun.StatusPass:
				out.Counts.Pass++
			case evalrun.StatusFail:
				out.Counts.Fail++
			case evalrun.StatusError:
				out.Counts.Error++
			}
			if in.Status == "" || string(res.Status) == in.Status {
				out.Items = append(out.Items, resultRow(res, cases))
			}
		}
		return out, nil
	}
}

func resultsDetailHandler(d Deps) func(context.Context, resultRef, dashcontract.Principal) (ResultView, error) {
	return func(ctx context.Context, in resultRef, p dashcontract.Principal) (ResultView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return ResultView{}, err
		}
		r, err := d.runInApp(ctx, app, in.RunID)
		if err != nil {
			return ResultView{}, d.fail("results.detail", err)
		}
		results, err := d.Engine.ListResults(ctx, r.ID)
		if err != nil {
			return ResultView{}, d.fail("results.detail", err)
		}
		cases, err := d.caseIndex(ctx, r.SuiteID)
		if err != nil {
			return ResultView{}, d.fail("results.detail", err)
		}
		for _, res := range results {
			if res.ID.String() != in.ResultID {
				continue
			}
			v := ResultView{ResultRow: resultRow(res, cases), Output: res.Output, OutputLength: utf8.RuneCountInString(res.Output),
				ScorerResults: []ScorerResultView{}, RunTrace: traceView(res.RunTrace)}
			for _, sr := range res.ScorerResults {
				v.ScorerResults = append(v.ScorerResults, ScorerResultView{ScorerName: sr.ScorerName, Score: sr.Score, Passed: sr.Passed,
					Reason: sr.Reason, Dimension: sr.Dimension, Details: sr.Details})
			}
			return v, nil
		}
		return ResultView{}, notFound("result not found")
	}
}
