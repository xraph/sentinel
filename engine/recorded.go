package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalplan"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
	"github.com/xraph/sentinel/scorer"
)

// RecordedRunRequest carries captured evidence without an executable target.
type RecordedRunRequest = recorded.Request

// RecordedCase is one captured case and repetition.
type RecordedCase = recorded.Case

// RecordedJob is durable captured-evaluation work and its result checkpoints.
type RecordedJob = recorded.Job

// EnqueueRecorded acknowledges only persisted, exactly scoped captured evidence.
func (e *Engine) EnqueueRecorded(ctx context.Context, req RecordedRunRequest) (*RecordedJob, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	j, err := recorded.NewJob(ctx, req)
	if err != nil {
		return nil, err
	}
	if _, err = e.recordedPlan(ctx, j.Request); err != nil {
		return nil, err
	}
	return e.store.CreateRecordedJob(ctx, j)
}

func (e *Engine) recordedPlan(ctx context.Context, req RecordedRunRequest) (*evalplan.Plan, error) {
	plan, err := e.store.GetEvaluationPlan(ctx, req.PlanID)
	if err != nil {
		return nil, err
	}
	if plan.Digest != req.PlanDigest {
		return nil, fmt.Errorf("%w: evaluation plan digest changed", recorded.ErrConflict)
	}
	inputs := make(map[id.CaseID]string, len(plan.Cases))
	for _, c := range plan.Cases {
		inputs[c.Case.ID] = recorded.InputDigest(c.Case.Input)
	}
	for _, c := range req.Cases {
		if inputs[c.CaseID] != c.InputDigest {
			return nil, fmt.Errorf("%w: recorded case does not match frozen input", sentinel.ErrInvalidInput)
		}
	}
	return plan, nil
}

// EvaluateRecorded scores captured outputs and reuses durable completed results.
// Caller cancellation returns the persisted partial result with ErrRunCancelled.
func (e *Engine) EvaluateRecorded(ctx context.Context, req RecordedRunRequest) (*RunResult, error) {
	j, err := e.EnqueueRecorded(ctx, req)
	if err != nil {
		return nil, err
	}
	return e.evaluateRecordedJob(ctx, j, true)
}

func (e *Engine) evaluateRecordedJob(ctx context.Context, j *recorded.Job, cancelTerminal bool) (res *RunResult, retErr error) {
	plan, err := e.recordedPlan(ctx, j.Request)
	if err != nil {
		return nil, err
	}
	if j.Terminal() {
		return recordedRunResult(j, plan), recordedTerminalError(j)
	}
	claim, err := j.Claim(id.New("eworker").String(), 2*time.Minute, time.Now().UTC())
	if err != nil {
		return recordedRunResult(j, plan), err
	}
	if claimErr := e.store.UpdateRecordedJob(ctx, j.Revision, claim); claimErr != nil {
		return nil, claimErr
	}
	j = claim
	if j.Terminal() {
		return recordedRunResult(j, plan), recordedTerminalError(j)
	}
	ctx, span := tracer().Start(ctx, "evaluate recorded run", trace.WithAttributes(attribute.String("sentinel.run.id", j.RunID.String())))
	defer span.End()
	start := time.Now()
	e.metrics.runStarted()
	defer func() {
		state := evalrun.StateFailed
		if res != nil {
			state = res.Run.State
		}
		e.metrics.runFinished("recorded", string(state), time.Since(start))
		if retErr != nil {
			e.extensions.EmitEvalRunFailed(ctx, plan.Suite.ID, j.RunID, retErr)
		}
	}()
	e.extensions.EmitEvalRunStarted(ctx, plan.Suite.ID, j.RunID, plan.Suite.Model)
	cases := make(map[id.CaseID]evalplan.Case, len(plan.Cases))
	for _, c := range plan.Cases {
		cases[c.Case.ID] = c
	}
	for i := len(j.Results); i < len(j.Request.Cases); i++ {
		if ctx.Err() != nil {
			return e.cancelRecorded(ctx, j, plan, cancelTerminal)
		}
		// Renew between cases. Each scorer receives a deadline inside this lease.
		renewed, cloneErr := j.Clone()
		if cloneErr != nil {
			return nil, cloneErr
		}
		renewed.Revision++
		renewed.LeaseUntil = time.Now().UTC().Add(2 * time.Minute)
		renewed.UpdatedAt = time.Now().UTC()
		if updateErr := e.store.UpdateRecordedJob(ctx, j.Revision, renewed); updateErr != nil {
			return recordedRunResult(j, plan), updateErr
		}
		j = renewed
		c := j.Request.Cases[i]
		scoreCtx, cancel := context.WithDeadline(ctx, j.LeaseUntil.Add(-time.Second))
		result := e.scoreRecordedCase(scoreCtx, j.RunID, c, cases[c.CaseID], plan.PassThreshold)
		cancel()
		next, cloneErr := j.Clone()
		if cloneErr != nil {
			return nil, cloneErr
		}
		next.Results = append(next.Results, result)
		next.Revision++
		next.UpdatedAt = time.Now().UTC()
		// A completed scoring call can be saved after caller cancellation. The
		// detached context is bounded, retains scope, and cannot bypass the lease.
		persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		updateErr := e.store.UpdateRecordedJob(persistCtx, j.Revision, next)
		persistCancel()
		if updateErr != nil {
			return recordedRunResult(j, plan), updateErr
		}
		j = next
	}
	if ctx.Err() != nil {
		return e.cancelRecorded(ctx, j, plan, cancelTerminal)
	}
	final, err := j.Clone()
	if err != nil {
		return nil, err
	}
	final.State = recorded.JobCompleted
	final.Revision++
	final.UpdatedAt = time.Now().UTC()
	if finalErr := e.store.UpdateRecordedJob(ctx, j.Revision, final); finalErr != nil {
		return recordedRunResult(j, plan), finalErr
	}
	result := recordedRunResult(final, plan)
	e.extensions.EmitEvalRunCompleted(ctx, plan.Suite.ID, j.RunID, result.Stats.PassRate, time.Since(start))
	return result, nil
}

func (e *Engine) cancelRecorded(ctx context.Context, j *recorded.Job, p *evalplan.Plan, terminal bool) (*RunResult, error) {
	if terminal {
		next, err := j.Clone()
		if err != nil {
			return nil, err
		}
		next.Revision++
		next.State = recorded.JobCancelled
		next.Error = ctx.Err().Error()
		next.UpdatedAt = time.Now().UTC()
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err = e.store.UpdateRecordedJob(persistCtx, j.Revision, next)
		cancel()
		if err != nil {
			return recordedRunResult(j, p), err
		}
		j = next
	}
	return recordedRunResult(j, p), fmt.Errorf("%w: %w", sentinel.ErrRunCancelled, ctx.Err())
}

func recordedTerminalError(j *recorded.Job) error {
	switch j.State {
	case recorded.JobCancelled:
		return sentinel.ErrRunCancelled
	case recorded.JobFailed:
		return fmt.Errorf("%w: %s", sentinel.ErrInvalidState, j.Error)
	default:
		return nil
	}
}

func (e *Engine) scoreRecordedCase(ctx context.Context, runID id.EvalRunID, c recorded.Case, frozen evalplan.Case, threshold float64) recorded.CaseResult {
	ctx, span := tracer().Start(ctx, "evaluate recorded case", trace.WithAttributes(attribute.String("sentinel.case.id", c.CaseID.String())))
	defer span.End()
	start := time.Now()
	e.extensions.EmitCaseStarted(ctx, runID, c.CaseID)
	result := &evalrun.Result{Entity: sentinel.NewEntity(), ID: id.NewEvalResultID(), RunID: runID, CaseID: c.CaseID, CaseName: frozen.Case.Name, Output: c.Output, RunTrace: c.Trace, LatencyMs: int(c.Duration.Milliseconds()), TokensUsed: int(c.Usage.Tokens), Cost: c.Usage.Cost}
	entry := recorded.CaseResult{Key: c.Key(), Result: result, ExecutionError: c.ExecutionError}
	if c.Outcome != recorded.Completed || c.ExecutionError != "" {
		result.Status = evalrun.StatusError
		if entry.ExecutionError == "" {
			entry.ExecutionError = "execution ended " + string(c.Outcome)
		}
		result.Error = entry.ExecutionError
		e.metrics.caseEvaluated(caseStatus(result.Status, ctx.Err() != nil))
		e.extensions.EmitCaseFailed(ctx, runID, c.CaseID, errors.New(entry.ExecutionError))
		return entry
	}
	values := make(map[string]any, len(frozen.Case.Context)+3)
	for key, value := range frozen.Case.Context {
		values[key] = value
	}
	values["latency_ms"] = float64(result.LatencyMs)
	values["cost_quality"] = string(c.Usage.CostQuality)
	delete(values, "cost")
	if c.Usage.CostKnown() {
		values["cost"] = c.Usage.Cost
	}
	criteria := make([]capturedCriterion, 0, len(frozen.Criteria))
	for _, criterion := range frozen.Criteria {
		s, err := e.scorers.GetVersioned(criterion.Scorer, criterion.Revision, criterion.Config)
		if criterion.Scorer == "cost" && !c.Usage.CostKnown() {
			err = errors.New("captured cost is not known")
		}
		criteria = append(criteria, capturedCriterion{ID: criterion.ID, Name: criterion.Scorer, Required: criterion.Required, Scorer: s, Err: err})
	}
	input := &scorer.Input{Input: frozen.Case.Input, Expected: frozen.Case.Expected, Actual: c.Output, Trace: c.Trace, Context: values}
	if err := e.scoreCapturedCase(ctx, result, input, criteria, threshold); err != nil {
		entry.ScoringError = result.Error
	}
	var judgeSamples []evalrun.Usage
	for _, judgment := range result.ScorerResults {
		usage := evalrun.Usage{}
		if judgment.Usage != nil {
			usage = *judgment.Usage
		}
		judgeSamples = append(judgeSamples, usage)
	}
	judgeUsage, usageErr := evalrun.AggregateUsage(judgeSamples)
	if usageErr != nil {
		result.Status = evalrun.StatusError
		result.Error = usageErr.Error()
		entry.ScoringError = usageErr.Error()
	} else {
		entry.JudgeUsage = judgeUsage
	}
	entry.ScoringDuration = time.Since(start)
	e.metrics.caseEvaluated(caseStatus(result.Status, ctx.Err() != nil))
	e.extensions.EmitCaseCompleted(ctx, runID, c.CaseID, result.Score, entry.ScoringDuration)
	return entry
}

func recordedRunResult(j *recorded.Job, p *evalplan.Plan) *RunResult {
	results := make([]*evalrun.Result, 0, len(j.Results))
	for _, entry := range j.Results {
		results = append(results, entry.Result)
	}
	stats := evalrun.Aggregate(results)
	state := evalrun.StateRunning
	switch j.State {
	case recorded.JobCompleted:
		state = evalrun.StateCompleted
	case recorded.JobCancelled:
		state = evalrun.StateCancelled
	case recorded.JobFailed:
		state = evalrun.StateFailed
	}
	run := &evalrun.Run{ID: j.RunID, SuiteID: p.Suite.ID, AppID: j.AppID, TargetTenantID: j.TenantID, Model: p.Suite.Model, SystemPrompt: p.Suite.SystemPrompt, TotalCases: len(j.Request.Cases), State: state, Error: j.Error, Config: map[string]any{"source": "recorded", "plan_id": p.ID.String(), "plan_digest": p.Digest, evalrun.SettingPassThreshold: p.PassThreshold}}
	run.CreatedAt = j.CreatedAt
	run.UpdatedAt = j.UpdatedAt
	if j.Terminal() {
		at := j.UpdatedAt
		run.CompletedAt = &at
	}
	run.ApplyStats(stats)
	return &RunResult{Run: run, Results: results, Stats: stats, Recorded: j}
}
