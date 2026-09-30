package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/xraph/go-utils/log"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/target"
	"github.com/xraph/sentinel/testcase"
)

// RunConfig configures a single evaluation run.
type RunConfig struct {
	SuiteID     id.SuiteID
	Model       string
	Target      target.Target
	Scorers     []scorer.Scorer
	PersonaRef  string
	Tags        []string
	Concurrency int
}

// RunResult holds the outcome of an evaluation run.
type RunResult struct {
	Run     *evalrun.Run
	Results []*evalrun.Result
	Stats   *evalrun.ResultStats
}

// StartConfig starts a run by the names an application registered with
// WithTarget and WithScorer.
type StartConfig struct {
	SuiteID id.SuiteID
	Target  string
	Scorers []string
	Model   string
}

// runPlan is a validated run, created in the store and ready to evaluate.
type runPlan struct {
	run         *evalrun.Run
	cases       []*testcase.Case
	target      target.Target
	scorers     []scorer.Scorer
	concurrency int
}

// RunEval runs a suite synchronously with target and scorer values, for Go
// and CI callers. Results are stored as each case finishes, exactly as for
// StartRun, and cancelling ctx cancels the run. A run that ends cancelled,
// by ctx or by CancelRun, returns its partial result together with an error
// that wraps sentinel.ErrRunCancelled (and ctx's error when ctx ended).
func (e *Engine) RunEval(ctx context.Context, cfg *RunConfig) (*RunResult, error) {
	if cfg.Target == nil {
		return nil, sentinel.ErrNoTarget
	}
	if len(cfg.Scorers) == 0 {
		return nil, sentinel.ErrNoScorers
	}
	names := make([]string, len(cfg.Scorers))
	for i, s := range cfg.Scorers {
		names[i] = s.Name()
	}
	plan, err := e.planRun(ctx, cfg.SuiteID, cfg.Model, cfg.PersonaRef, cfg.Concurrency, cfg.Target, cfg.Target.Name(), cfg.Scorers, names)
	if err != nil {
		return nil, err
	}
	return e.executeRun(ctx, plan)
}

// StartRun validates a run, creates it and returns at once. Evaluation
// continues on the engine's own context, so the request that started it
// can return; poll the run and its results to watch it, and CancelRun to
// stop it. Every refusal happens before anything is written.
func (e *Engine) StartRun(ctx context.Context, cfg *StartConfig) (*evalrun.Run, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	rt, ok := e.targets[cfg.Target]
	if !ok {
		return nil, fmt.Errorf("%w %q", sentinel.ErrUnknownTarget, cfg.Target)
	}
	if len(cfg.Scorers) == 0 {
		return nil, sentinel.ErrNoScorers
	}
	scorers := make([]scorer.Scorer, 0, len(cfg.Scorers))
	for _, name := range cfg.Scorers {
		s, err := e.scorers.Get(name, nil)
		if err != nil {
			return nil, fmt.Errorf("%w %q", sentinel.ErrUnknownScorer, name)
		}
		scorers = append(scorers, s)
	}
	plan, err := e.planRun(ctx, cfg.SuiteID, cfg.Model, "", 0, rt.Target, rt.Name, scorers, cfg.Scorers)
	if err != nil {
		return nil, err
	}
	started := *plan.run
	e.runs.Add(1)
	go func() {
		defer e.runs.Done()
		_, err := e.executeRun(e.baseCtx, plan)
		switch {
		case err == nil:
		case errors.Is(err, sentinel.ErrRunCancelled):
			// CancelRun or shutdown stopped it: expected, not a failure.
			e.logger.Info("sentinel: run cancelled",
				log.String("run_id", plan.run.ID.String()), log.String("reason", err.Error()))
		default:
			e.logger.Warn("sentinel: run ended with an error",
				log.String("run_id", plan.run.ID.String()), log.String("error", err.Error()))
		}
	}()
	return &started, nil
}

// CancelRun asks a running run to stop. The runner stops scheduling cases
// at its next check; cases already in flight finish and are stored. It works
// from any replica because the signal lives in the store.
func (e *Engine) CancelRun(ctx context.Context, runID id.EvalRunID) error {
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	ok, err := e.store.CancelRun(ctx, runID, time.Now().UTC())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: run %s is not running", sentinel.ErrInvalidState, runID)
	}
	return nil
}

// effectivePrompt is the prompt a run of this suite sends: the current
// prompt version's if the suite has one, otherwise the suite's own.
func (e *Engine) effectivePrompt(ctx context.Context, s *suite.Suite) (prompt, versionID string, err error) {
	pv, err := e.store.GetCurrentPromptVersion(ctx, s.ID)
	switch {
	case err == nil:
		return pv.SystemPrompt, pv.ID.String(), nil
	case errors.Is(err, sentinel.ErrPromptVersionNotFound):
		return s.SystemPrompt, "", nil
	default:
		return "", "", fmt.Errorf("sentinel: load current prompt version: %w", err)
	}
}

func (e *Engine) planRun(ctx context.Context, suiteID id.SuiteID, model, personaRef string, concurrency int,
	tgt target.Target, targetName string, scorers []scorer.Scorer, scorerNames []string) (*runPlan, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	s, err := e.store.GetSuite(ctx, suiteID)
	if err != nil {
		return nil, fmt.Errorf("sentinel: load suite: %w", err)
	}
	cases, err := e.store.ListCases(ctx, suiteID)
	if err != nil {
		return nil, fmt.Errorf("sentinel: load cases: %w", err)
	}
	if len(cases) == 0 {
		return nil, sentinel.ErrEmptyInput
	}
	prompt, pvID, err := e.effectivePrompt(ctx, s)
	if err != nil {
		return nil, err
	}
	if model == "" {
		model = s.Model
	}
	if model == "" {
		model = e.config.DefaultModel
	}
	if personaRef == "" {
		personaRef = s.PersonaRef
	}
	if concurrency <= 0 {
		concurrency = e.config.Concurrency
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	passThreshold, regressionThreshold := e.config.PassThreshold, e.config.RegressionThreshold
	settings := evalrun.Settings{
		PassThreshold: &passThreshold, RegressionThreshold: &regressionThreshold, Concurrency: &concurrency,
		Target: targetName, Scorers: scorerNames, Model: model, PromptVersionID: pvID,
	}
	run := &evalrun.Run{
		Entity:          sentinel.NewEntity(),
		ID:              id.NewEvalRunID(),
		SuiteID:         suiteID,
		Model:           model,
		SystemPrompt:    prompt,
		Temperature:     s.Temperature,
		TotalCases:      len(cases),
		AppID:           s.AppID,
		PersonaRef:      personaRef,
		Config:          settings.Config(),
		State:           evalrun.StateRunning,
		DimensionScores: map[string]float64{},
	}
	if err := e.store.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("sentinel: create run: %w", err)
	}
	e.extensions.EmitEvalRunStarted(ctx, suiteID, run.ID, model)
	if personaRef != "" {
		e.extensions.EmitPersonaEvalStarted(ctx, run.ID, personaRef)
	}
	return &runPlan{run: run, cases: cases, target: tgt, scorers: scorers, concurrency: concurrency}, nil
}

// executeRun evaluates the plan's cases, storing each result as it
// finishes, and stops scheduling when the run is cancelled or ctx ends.
func (e *Engine) executeRun(ctx context.Context, p *runPlan) (*RunResult, error) {
	run := p.run
	// Writes must land even after ctx is cancelled: a run stopped by
	// shutdown still records what it did.
	writeCtx := context.WithoutCancel(ctx)
	callCtx := target.WithCallOptions(ctx, target.CallOptions{SystemPrompt: run.SystemPrompt, Model: run.Model, Temperature: run.Temperature})

	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup
	var unwritten atomic.Int64
	for _, tc := range p.cases {
		sem <- struct{}{}
		if e.stopRequested(ctx, writeCtx, run.ID) {
			<-sem
			break
		}
		wg.Add(1)
		go func(tc *testcase.Case) {
			defer wg.Done()
			defer func() { <-sem }()
			result := e.evaluateCase(callCtx, run.ID, tc, p.target, p.scorers)
			if err := e.store.CreateResult(writeCtx, result); err != nil {
				unwritten.Add(1)
				e.logger.Warn("sentinel: store result",
					log.String("run_id", run.ID.String()), log.String("case_id", tc.ID.String()), log.String("error", err.Error()))
			}
		}(tc)
	}
	wg.Wait()
	// A ctx that ended after the last case was scheduled still stops the
	// run: its in-flight cases were cut short, so it did not complete.
	if ctx.Err() != nil {
		if _, err := e.store.CancelRun(writeCtx, run.ID, time.Now().UTC()); err != nil {
			e.logger.Warn("sentinel: record cancel", log.String("run_id", run.ID.String()), log.String("error", err.Error()))
		}
	}
	return e.finishRun(writeCtx, p, unwritten.Load(), ctx.Err())
}

// stopRequested reports whether the runner should stop scheduling. A done
// ctx means this process is stopping the run, so it records the cancel
// itself; otherwise it reads the run, because the cancel may have come from
// another replica.
func (e *Engine) stopRequested(ctx, writeCtx context.Context, runID id.EvalRunID) bool {
	if ctx.Err() != nil {
		if _, err := e.store.CancelRun(writeCtx, runID, time.Now().UTC()); err != nil {
			e.logger.Warn("sentinel: record cancel", log.String("run_id", runID.String()), log.String("error", err.Error()))
		}
		return true
	}
	r, err := e.store.GetRun(writeCtx, runID)
	if err != nil {
		e.logger.Warn("sentinel: check run state", log.String("run_id", runID.String()), log.String("error", err.Error()))
		return false
	}
	return r.State == evalrun.StateCancelled
}

// checkRegression compares a completed run with its suite's current
// baseline, using the regression threshold the run recorded, and fires
// RegressionDetected when it regressed. No baseline means nothing to
// compare against, which is not a pass and not a regression.
func (e *Engine) checkRegression(ctx context.Context, run *evalrun.Run, stats *evalrun.ResultStats, results []*evalrun.Result) {
	b, err := e.store.GetLatestBaseline(ctx, run.SuiteID)
	if err != nil {
		if !errors.Is(err, sentinel.ErrBaselineNotFound) {
			e.logger.Warn("sentinel: load baseline", log.String("run_id", run.ID.String()), log.String("error", err.Error()))
		}
		return
	}
	threshold := e.config.RegressionThreshold
	if v := evalrun.SettingsFrom(run.Config).RegressionThreshold; v != nil {
		threshold = *v
	}
	rr := baseline.DetectRegression(stats, results, b, threshold)
	if rr.HasRegression {
		e.extensions.EmitRegressionDetected(ctx, run.SuiteID, b.ID, rr.WorstDelta())
	}
}

// finishRun computes counters from the stored results and finalises the
// run. The store keeps a cancel that arrived meanwhile. A run that ends
// cancelled returns its result with an error wrapping ErrRunCancelled, and
// cause (the caller's ctx error) when ctx ended: its counters cover only the
// cases that finished, so a caller must not read them as a verdict.
func (e *Engine) finishRun(ctx context.Context, p *runPlan, unwritten int64, cause error) (*RunResult, error) {
	run := p.run
	// A run whose counters cannot be read is failed, never completed with
	// zeros that a regression check would take as real.
	var failures []string
	stats, err := e.store.GetResultStats(ctx, run.ID)
	if err != nil {
		e.logger.Warn("sentinel: read result stats", log.String("run_id", run.ID.String()), log.String("error", err.Error()))
		stats = &evalrun.ResultStats{DimensionScores: map[string]float64{}}
		failures = append(failures, "result stats could not be read: "+err.Error())
	}
	if unwritten > 0 {
		failures = append(failures, fmt.Sprintf("%d of %d results could not be stored", unwritten, run.TotalCases))
	}
	f := &evalrun.Finalization{Stats: stats, State: evalrun.StateCompleted, CompletedAt: time.Now().UTC()}
	if len(failures) > 0 {
		f.State = evalrun.StateFailed
		f.Error = strings.Join(failures, "; ")
	}
	state, err := e.store.FinalizeRun(ctx, run.ID, f)
	if err != nil {
		return nil, fmt.Errorf("sentinel: finalize run: %w", err)
	}
	run.ApplyStats(stats)
	run.State = state
	run.CompletedAt = &f.CompletedAt
	if f.Error != "" {
		run.Error = f.Error
	}
	results, err := e.store.ListResults(ctx, run.ID)
	if err != nil {
		e.logger.Warn("sentinel: list results", log.String("run_id", run.ID.String()), log.String("error", err.Error()))
	}
	res := &RunResult{Run: run, Results: results, Stats: stats}

	switch state {
	case evalrun.StateCompleted:
		e.extensions.EmitEvalRunCompleted(ctx, run.SuiteID, run.ID, stats.PassRate, time.Since(run.CreatedAt))
		if run.PersonaRef != "" {
			e.extensions.EmitPersonaEvalCompleted(ctx, run.ID, run.PersonaRef, stats.DimensionScores)
		}
		e.checkRegression(ctx, run, stats, results)
	case evalrun.StateFailed:
		failure := errors.New(run.Error)
		e.extensions.EmitEvalRunFailed(ctx, run.SuiteID, run.ID, failure)
		return res, fmt.Errorf("sentinel: %w", failure)
	case evalrun.StateCancelled:
		if cause != nil {
			return res, fmt.Errorf("%w: %w", sentinel.ErrRunCancelled, cause)
		}
		return res, sentinel.ErrRunCancelled
	}
	return res, nil
}

// evaluateCase invokes the target and runs all scorers for a single test case.
func (e *Engine) evaluateCase(
	ctx context.Context,
	runID id.EvalRunID,
	tc *testcase.Case,
	tgt target.Target,
	scorers []scorer.Scorer,
) *evalrun.Result {
	e.extensions.EmitCaseStarted(ctx, runID, tc.ID)
	start := time.Now()

	result := &evalrun.Result{
		Entity:   sentinel.NewEntity(),
		ID:       id.NewEvalResultID(),
		RunID:    runID,
		CaseID:   tc.ID,
		CaseName: tc.Name,

		ScorerResults:   []evalrun.ScorerResult{},
		DimensionScores: map[string]float64{},
	}

	// Invoke the target.
	output, err := tgt.Call(ctx, tc.Input)
	if err != nil {
		result.Status = evalrun.StatusError
		result.Error = err.Error()
		result.LatencyMs = int(time.Since(start).Milliseconds())
		e.extensions.EmitCaseFailed(ctx, runID, tc.ID, err)
		return result
	}

	result.Output = output.Output
	result.LatencyMs = int(output.Latency.Milliseconds())
	result.TokensUsed = output.Tokens
	result.Cost = output.Cost
	result.RunTrace = output.Trace

	// Copy the case's context: the stored case must not gain latency_ms and
	// cost, and two runs of one suite must not write the same map.
	scorerCtx := make(map[string]any, len(tc.Context)+2)
	for k, v := range tc.Context {
		scorerCtx[k] = v
	}
	scorerCtx["latency_ms"] = float64(result.LatencyMs)
	scorerCtx["cost"] = result.Cost

	input := &scorer.Input{
		Input:    tc.Input,
		Expected: tc.Expected,
		Actual:   output.Output,
		Trace:    output.Trace,
		Context:  scorerCtx,
	}

	// The run's scorers, then the case's own, resolved by name. A case scorer
	// nobody registered is a scorer error like any other, so the case is
	// visibly unscored instead of silently scored by fewer scorers.
	all := append([]scorer.Scorer(nil), scorers...)
	var scorerErrs []string
	var scorerResults []evalrun.ScorerResult
	for _, cfg := range tc.Scorers {
		s, err := e.scorers.Get(cfg.Name, cfg.Config)
		if err != nil {
			scorerErrs = append(scorerErrs, fmt.Sprintf("scorer %s: %v", cfg.Name, err))
			scorerResults = append(scorerResults, evalrun.ScorerResult{ScorerName: cfg.Name, Reason: "scorer error: " + err.Error()})
			continue
		}
		all = append(all, s)
	}

	var total float64
	var scored int
	dimensionScores := make(map[string]float64)
	dimensionCounts := make(map[string]int)
	for _, s := range all {
		so, err := s.Score(ctx, input)
		if err != nil {
			scorerErrs = append(scorerErrs, fmt.Sprintf("scorer %s: %v", s.Name(), err))
			scorerResults = append(scorerResults, evalrun.ScorerResult{ScorerName: s.Name(), Reason: "scorer error: " + err.Error()})
			continue
		}
		scorerResults = append(scorerResults, evalrun.ScorerResult{
			ScorerName: s.Name(), Score: so.Score, Passed: so.Passed, Reason: so.Reason, Dimension: so.Dimension, Details: so.Details,
		})
		total += so.Score
		scored++
		if so.Dimension != "" {
			dimensionScores[so.Dimension] += so.Score
			dimensionCounts[so.Dimension]++
		}
	}
	if scored > 0 {
		result.Score = total / float64(scored)
	}
	for dim, sum := range dimensionScores {
		dimensionScores[dim] = sum / float64(dimensionCounts[dim])
	}
	result.ScorerResults = scorerResults
	result.DimensionScores = dimensionScores

	// A case any scorer could not judge is an error, never a pass: its score
	// covers fewer scorers than the run asked for.
	switch {
	case len(scorerErrs) > 0:
		result.Status = evalrun.StatusError
		result.Error = strings.Join(scorerErrs, "; ")
	case result.Score >= e.config.PassThreshold:
		result.Status = evalrun.StatusPass
	default:
		result.Status = evalrun.StatusFail
	}

	elapsed := time.Since(start)
	e.extensions.EmitCaseCompleted(ctx, runID, tc.ID, result.Score, elapsed)

	return result
}
