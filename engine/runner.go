package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/scorer"
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

// RunEval executes an evaluation run: load suite, load cases, invoke the
// target for each case, score results, persist everything, and emit hooks.
func (e *Engine) RunEval(ctx context.Context, cfg *RunConfig) (*RunResult, error) {
	if cfg.Target == nil {
		return nil, sentinel.ErrNoTarget
	}
	if len(cfg.Scorers) == 0 {
		return nil, sentinel.ErrNoScorers
	}
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}

	// Load suite.
	s, err := e.store.GetSuite(ctx, cfg.SuiteID)
	if err != nil {
		return nil, fmt.Errorf("sentinel: load suite: %w", err)
	}

	// Load cases.
	cases, err := e.store.ListCases(ctx, cfg.SuiteID)
	if err != nil {
		return nil, fmt.Errorf("sentinel: load cases: %w", err)
	}
	if len(cases) == 0 {
		return nil, sentinel.ErrEmptyInput
	}

	// Determine model.
	model := cfg.Model
	if model == "" {
		model = s.Model
	}
	if model == "" {
		model = e.config.DefaultModel
	}

	// Determine persona.
	personaRef := cfg.PersonaRef
	if personaRef == "" {
		personaRef = s.PersonaRef
	}

	// Determine concurrency.
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = e.config.Concurrency
	}

	// Create the run record.
	run := &evalrun.Run{
		Entity:       sentinel.NewEntity(),
		ID:           id.NewEvalRunID(),
		SuiteID:      cfg.SuiteID,
		Model:        model,
		SystemPrompt: s.SystemPrompt,
		Temperature:  s.Temperature,
		TotalCases:   len(cases),
		AppID:        s.AppID,
		PersonaRef:   personaRef,
		State:        evalrun.StateRunning,
	}

	if err := e.store.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("sentinel: create run: %w", err)
	}

	// Emit run started hook.
	e.extensions.EmitEvalRunStarted(ctx, cfg.SuiteID, run.ID, model)
	if personaRef != "" {
		e.extensions.EmitPersonaEvalStarted(ctx, run.ID, personaRef)
	}

	// Evaluate cases concurrently.
	results := make([]*evalrun.Result, len(cases))
	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, tc := range cases {
		wg.Add(1)
		go func(idx int, tc *testcase.Case) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := e.evaluateCase(ctx, run.ID, tc, cfg.Target, cfg.Scorers)
			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, tc)
	}
	wg.Wait()

	// Persist results.
	if err := e.store.CreateResultBatch(ctx, results); err != nil {
		e.failRun(ctx, run, cfg.SuiteID, fmt.Errorf("store results: %w", err))
		return nil, fmt.Errorf("sentinel: store results: %w", err)
	}

	// Aggregate stats.
	stats := aggregateStats(results)

	// Update run with final stats.
	now := time.Now().UTC()
	run.Passed = stats.Passed
	run.Failed = stats.Failed
	run.PassRate = stats.PassRate
	run.AvgScore = stats.AvgScore
	run.AvgLatencyMs = stats.AvgLatencyMs
	run.TotalTokens = stats.TotalTokens
	run.TotalCost = stats.TotalCost
	run.DimensionScores = stats.DimensionScores
	run.State = evalrun.StateCompleted
	run.CompletedAt = &now

	if err := e.store.UpdateRun(ctx, run); err != nil {
		e.logger.Warn("failed to update run record",
			slog.String("run_id", run.ID.String()),
			slog.String("error", err.Error()),
		)
	}

	// Emit completion hooks.
	elapsed := time.Since(run.CreatedAt)
	e.extensions.EmitEvalRunCompleted(ctx, cfg.SuiteID, run.ID, stats.PassRate, elapsed)
	if personaRef != "" {
		e.extensions.EmitPersonaEvalCompleted(ctx, run.ID, personaRef, stats.DimensionScores)
	}

	return &RunResult{
		Run:     run,
		Results: results,
		Stats:   stats,
	}, nil
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

	// Build scorer input.
	scorerCtx := tc.Context
	if scorerCtx == nil {
		scorerCtx = make(map[string]any)
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

	// Run all scorers.
	var totalScore float64
	var scorerResults []evalrun.ScorerResult
	dimensionScores := make(map[string]float64)
	dimensionCounts := make(map[string]int)

	for _, s := range scorers {
		so, scoreErr := s.Score(ctx, input)
		if scoreErr != nil {
			scorerResults = append(scorerResults, evalrun.ScorerResult{
				ScorerName: s.Name(),
				Score:      0,
				Passed:     false,
				Reason:     fmt.Sprintf("scorer error: %v", scoreErr),
			})
			continue
		}

		sr := evalrun.ScorerResult{
			ScorerName: s.Name(),
			Score:      so.Score,
			Passed:     so.Passed,
			Reason:     so.Reason,
			Dimension:  so.Dimension,
			Details:    so.Details,
		}
		scorerResults = append(scorerResults, sr)
		totalScore += so.Score

		if so.Dimension != "" {
			dimensionScores[so.Dimension] += so.Score
			dimensionCounts[so.Dimension]++
		}
	}

	// Average score across all scorers.
	if len(scorerResults) > 0 {
		result.Score = totalScore / float64(len(scorerResults))
	}

	// Average dimension scores.
	for dim, total := range dimensionScores {
		if count := dimensionCounts[dim]; count > 0 {
			dimensionScores[dim] = total / float64(count)
		}
	}

	result.ScorerResults = scorerResults
	result.DimensionScores = dimensionScores

	// Determine pass/fail.
	if result.Score >= e.config.PassThreshold {
		result.Status = evalrun.StatusPass
	} else {
		result.Status = evalrun.StatusFail
	}

	elapsed := time.Since(start)
	e.extensions.EmitCaseCompleted(ctx, runID, tc.ID, result.Score, elapsed)

	return result
}

// failRun marks a run as failed and emits the failure hook.
func (e *Engine) failRun(ctx context.Context, run *evalrun.Run, suiteID id.SuiteID, runErr error) {
	now := time.Now().UTC()
	run.State = evalrun.StateFailed
	run.Error = runErr.Error()
	run.CompletedAt = &now

	if err := e.store.UpdateRun(ctx, run); err != nil {
		e.logger.Warn("failed to update failed run",
			slog.String("run_id", run.ID.String()),
			slog.String("error", err.Error()),
		)
	}

	e.extensions.EmitEvalRunFailed(ctx, suiteID, run.ID, runErr)
}

// aggregateStats computes summary statistics from a slice of results.
func aggregateStats(results []*evalrun.Result) *evalrun.ResultStats {
	stats := &evalrun.ResultStats{
		TotalCases:      len(results),
		DimensionScores: make(map[string]float64),
	}

	if len(results) == 0 {
		return stats
	}

	var totalScore float64
	var totalLatency int
	var totalTokens int
	var totalCost float64
	dimensionSums := make(map[string]float64)
	dimensionCounts := make(map[string]int)

	for _, r := range results {
		switch r.Status {
		case evalrun.StatusPass:
			stats.Passed++
		case evalrun.StatusFail:
			stats.Failed++
		case evalrun.StatusError:
			stats.Errored++
		}

		totalScore += r.Score
		totalLatency += r.LatencyMs
		totalTokens += r.TokensUsed
		totalCost += r.Cost

		for dim, score := range r.DimensionScores {
			dimensionSums[dim] += score
			dimensionCounts[dim]++
		}
	}

	n := float64(len(results))
	stats.PassRate = float64(stats.Passed) / n
	stats.AvgScore = totalScore / n
	stats.AvgLatencyMs = totalLatency / len(results)
	stats.TotalTokens = totalTokens
	stats.TotalCost = totalCost

	for dim, sum := range dimensionSums {
		if count := dimensionCounts[dim]; count > 0 {
			stats.DimensionScores[dim] = sum / float64(count)
		}
	}

	return stats
}
