package comparison

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalplan"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
)

// RecordedStore provides immutable evidence without any target execution API.
type RecordedStore interface {
	GetRecordedJob(context.Context, id.ID) (*recorded.Job, error)
	GetEvaluationPlan(context.Context, id.ID) (*evalplan.Plan, error)
}

// RecordedComparisonRequest selects persisted evaluations and a reproducible seed.
type RecordedComparisonRequest struct {
	Store       RecordedStore
	BaselineID  id.ID
	CandidateID id.ID
	Seed        uint64
	MinCases    int // Zero uses 20 distinct cases. Repetitions never count as cases.
}

// RecordedPair preserves the identities and scores behind one paired difference.
type RecordedPair struct {
	CaseID     id.CaseID `json:"case_id"`
	Repetition int       `json:"repetition"`
	Baseline   float64   `json:"baseline"`
	Candidate  float64   `json:"candidate"`
	Delta      float64   `json:"delta"`
}

// CriterionFailure retains a candidate's failed required criterion.
type CriterionFailure struct {
	CaseID      id.CaseID `json:"case_id"`
	Repetition  int       `json:"repetition"`
	CriterionID string    `json:"criterion_id"`
	Reason      string    `json:"reason"`
}

// ConfidenceInterval is a 95 percent paired bootstrap interval over distinct cases.
type ConfidenceInterval struct {
	Lower float64 `json:"lower"`
	Upper float64 `json:"upper"`
}

// RecordedComparison reports evidence without deciding production activation.
type RecordedComparison struct {
	BaselineID             id.ID               `json:"baseline_id"`
	CandidateID            id.ID               `json:"candidate_id"`
	BaselineDigest         string              `json:"baseline_digest"`
	CandidateDigest        string              `json:"candidate_digest"`
	PlanDigest             string              `json:"plan_digest"`
	Seed                   uint64              `json:"seed"`
	Method                 string              `json:"method"`
	BootstrapSamples       int                 `json:"bootstrap_samples"`
	CaseCount              int                 `json:"case_count"`
	PairCount              int                 `json:"pair_count"`
	Pairs                  []RecordedPair      `json:"pairs"`
	MeanDelta              *float64            `json:"mean_delta,omitempty"`
	Interval               *ConfidenceInterval `json:"interval,omitempty"`
	DimensionDeltas        map[string]float64  `json:"dimension_deltas"`
	UnavailableDimensions  []string            `json:"unavailable_dimensions,omitempty"`
	RequiredCriteriaPassed bool                `json:"required_criteria_passed"`
	RequiredFailures       []CriterionFailure  `json:"required_failures,omitempty"`
	CostComparable         bool                `json:"cost_comparable"`
	Currency               string              `json:"currency,omitempty"`
	MeanCostDelta          *float64            `json:"mean_cost_delta,omitempty"`
	MeanLatencyDeltaMs     *float64            `json:"mean_latency_delta_ms,omitempty"`
	Conclusive             bool                `json:"conclusive"`
	IncompletenessReasons  []string            `json:"incompleteness_reasons,omitempty"`
}

// CompareRecorded checks exact plan/case/repetition identities before comparing.
// It averages repetitions within each case, then resamples 2,000 sets of cases.
// The nearest-rank 2.5 and 97.5 percentiles form the interval. No target is called.
func CompareRecorded(ctx context.Context, req RecordedComparisonRequest) (*RecordedComparison, error) {
	if req.Store == nil || req.BaselineID.IsNil() || req.CandidateID.IsNil() || req.BaselineID == req.CandidateID || req.MinCases < 0 {
		return nil, fmt.Errorf("%w: invalid recorded comparison", sentinel.ErrInvalidInput)
	}
	baseline, err := req.Store.GetRecordedJob(ctx, req.BaselineID)
	if err != nil {
		return nil, err
	}
	candidate, err := req.Store.GetRecordedJob(ctx, req.CandidateID)
	if err != nil {
		return nil, err
	}
	if baseline == nil || candidate == nil || baseline.ID != req.BaselineID || candidate.ID != req.CandidateID {
		return nil, fmt.Errorf("%w: returned evaluation identity mismatch", sentinel.ErrInvalidInput)
	}
	if baselineErr := baseline.Validate(ctx); baselineErr != nil {
		return nil, baselineErr
	}
	if candidateErr := candidate.Validate(ctx); candidateErr != nil {
		return nil, candidateErr
	}
	if baseline.Request.PlanID != candidate.Request.PlanID || baseline.Request.PlanDigest != candidate.Request.PlanDigest {
		return nil, fmt.Errorf("%w: comparison plans differ", sentinel.ErrInvalidInput)
	}
	plan, err := req.Store.GetEvaluationPlan(ctx, baseline.Request.PlanID)
	if err != nil {
		return nil, err
	}
	if planErr := plan.Validate(ctx); planErr != nil {
		return nil, planErr
	}
	if plan.ID != baseline.Request.PlanID || plan.Digest != baseline.Request.PlanDigest {
		return nil, fmt.Errorf("%w: stored comparison plan changed", recorded.ErrConflict)
	}
	baseInputs := caseMap(baseline.Request.Cases)
	candidateInputs := caseMap(candidate.Request.Cases)
	if len(baseInputs) != len(candidateInputs) {
		return nil, fmt.Errorf("%w: comparison trial sets differ", sentinel.ErrInvalidInput)
	}
	definitions := make(map[id.CaseID]evalplan.Case, len(plan.Cases))
	for _, c := range plan.Cases {
		definitions[c.Case.ID] = c
	}
	keys := make([]string, 0, len(baseInputs))
	inputOwners := make(map[string]id.CaseID)
	for key, c := range baseInputs {
		other, exists := candidateInputs[key]
		definition, inPlan := definitions[c.CaseID]
		content, encodeErr := json.Marshal(struct {
			Input   string         `json:"input"`
			Context map[string]any `json:"context"`
		}{Input: definition.Case.Input, Context: definition.Case.Context})
		if encodeErr != nil {
			return nil, encodeErr
		}
		contentKey := recorded.InputDigest(string(content))
		if owner, seen := inputOwners[contentKey]; seen && owner != c.CaseID {
			return nil, fmt.Errorf("%w: duplicate input and frozen context under distinct case IDs", sentinel.ErrInvalidInput)
		}
		inputOwners[contentKey] = c.CaseID
		if !exists || !inPlan || c.InputDigest != other.InputDigest || c.InputDigest != recorded.InputDigest(definition.Case.Input) {
			return nil, fmt.Errorf("%w: comparison input identities differ", sentinel.ErrInvalidInput)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	report := &RecordedComparison{BaselineID: baseline.ID, CandidateID: candidate.ID, BaselineDigest: baseline.RequestDigest, CandidateDigest: candidate.RequestDigest, PlanDigest: plan.Digest, Seed: req.Seed, Method: "paired-case-bootstrap-pcg-v1", BootstrapSamples: 2000, RequiredCriteriaPassed: true, CostComparable: true, DimensionDeltas: make(map[string]float64), PairCount: len(keys)}
	if baseline.State != recorded.JobCompleted || candidate.State != recorded.JobCompleted {
		report.CostComparable = false
		report.RequiredCriteriaPassed = false
		report.IncompletenessReasons = append(report.IncompletenessReasons, "both evaluations must be completed")
		return report, nil
	}
	baseResults := resultMap(baseline.Results)
	candidateResults := resultMap(candidate.Results)
	groups := make(map[id.CaseID][]float64)
	costs := make(map[id.CaseID][]float64)
	latency := make(map[id.CaseID][]float64)
	dimensions := make(map[string]map[id.CaseID][]float64)
	dimensionCounts := make(map[string]int)
	for _, key := range keys {
		c, other := baseInputs[key], candidateInputs[key]
		b, k := baseResults[key], candidateResults[key]
		if b == nil || k == nil {
			return nil, fmt.Errorf("%w: missing recorded result", sentinel.ErrInvalidInput)
		}
		definition := definitions[c.CaseID]
		for _, result := range []*evalrun.Result{b, k} {
			if result.Status == evalrun.StatusError {
				continue
			}
			if criteriaErr := validateCriteria(definition, result); criteriaErr != nil {
				return nil, criteriaErr
			}
		}
		if c.Outcome != recorded.Completed || other.Outcome != recorded.Completed || b.Status == evalrun.StatusError || k.Status == evalrun.StatusError {
			report.IncompletenessReasons = append(report.IncompletenessReasons, "unscored or incomplete execution: "+key)
		}
		for _, judgment := range k.ScorerResults {
			if judgment.Required && !judgment.Passed {
				report.RequiredFailures = append(report.RequiredFailures, CriterionFailure{CaseID: c.CaseID, Repetition: c.Repetition, CriterionID: judgment.CriterionID, Reason: judgment.Reason})
			}
		}
		report.Pairs = append(report.Pairs, RecordedPair{CaseID: c.CaseID, Repetition: c.Repetition, Baseline: b.Score, Candidate: k.Score, Delta: k.Score - b.Score})
		groups[c.CaseID] = append(groups[c.CaseID], k.Score-b.Score)
		latency[c.CaseID] = append(latency[c.CaseID], float64(other.Duration-c.Duration)/1e6)
		if !c.Usage.CostKnown() || !other.Usage.CostKnown() || c.Usage.Currency != other.Usage.Currency || (report.Currency != "" && report.Currency != c.Usage.Currency) {
			report.CostComparable = false
		}
		if report.Currency == "" {
			report.Currency = c.Usage.Currency
		}
		costs[c.CaseID] = append(costs[c.CaseID], other.Usage.Cost-c.Usage.Cost)
		union := make(map[string]bool)
		for dim := range b.DimensionScores {
			union[dim] = true
		}
		for dim := range k.DimensionScores {
			union[dim] = true
		}
		for dim := range union {
			if dimensions[dim] == nil {
				dimensions[dim] = make(map[id.CaseID][]float64)
			}
			bv, bok := b.DimensionScores[dim]
			cv, cok := k.DimensionScores[dim]
			if !bok || !cok || math.IsNaN(bv) || math.IsNaN(cv) || math.IsInf(bv, 0) || math.IsInf(cv, 0) {
				continue
			}
			dimensions[dim][c.CaseID] = append(dimensions[dim][c.CaseID], cv-bv)
			dimensionCounts[dim]++
		}
	}
	report.CaseCount = len(groups)
	report.RequiredCriteriaPassed = len(report.RequiredFailures) == 0 && len(report.IncompletenessReasons) == 0
	if len(report.IncompletenessReasons) > 0 {
		report.CostComparable = false
		return report, nil
	}
	caseDeltas := caseMeans(groups)
	mean := average(caseDeltas)
	report.MeanDelta = &mean
	report.Interval = bootstrap(ctx, caseDeltas, req.Seed)
	if contextErr := ctx.Err(); contextErr != nil {
		return nil, contextErr
	}
	latencyMean := average(caseMeans(latency))
	report.MeanLatencyDeltaMs = &latencyMean
	if report.CostComparable {
		costMean := average(caseMeans(costs))
		report.MeanCostDelta = &costMean
	}
	for dim, values := range dimensions {
		if dimensionCounts[dim] != len(keys) {
			report.UnavailableDimensions = append(report.UnavailableDimensions, dim)
			continue
		}
		report.DimensionDeltas[dim] = average(caseMeans(values))
	}
	sort.Strings(report.UnavailableDimensions)
	minimum := req.MinCases
	if minimum == 0 {
		minimum = 20
	}
	if len(groups) < minimum {
		report.IncompletenessReasons = append(report.IncompletenessReasons, fmt.Sprintf("requires %d distinct cases; has %d", minimum, len(groups)))
	}
	report.Conclusive = len(report.IncompletenessReasons) == 0
	return report, nil
}

func caseMap(cases []recorded.Case) map[string]recorded.Case {
	out := make(map[string]recorded.Case, len(cases))
	for _, c := range cases {
		out[c.Key()] = c
	}
	return out
}
func resultMap(results []recorded.CaseResult) map[string]*evalrun.Result {
	out := make(map[string]*evalrun.Result, len(results))
	for _, r := range results {
		out[r.Key] = r.Result
	}
	return out
}
func validateCriteria(definition evalplan.Case, result *evalrun.Result) error {
	criteria := make(map[string]evalplan.Criterion, len(definition.Criteria))
	for _, c := range definition.Criteria {
		criteria[c.ID] = c
	}
	if len(result.ScorerResults) != len(criteria) {
		return fmt.Errorf("%w: recorded criterion set changed", sentinel.ErrInvalidInput)
	}
	seen := make(map[string]bool)
	for _, r := range result.ScorerResults {
		c, ok := criteria[r.CriterionID]
		if !ok || seen[r.CriterionID] || r.Required != c.Required || r.Errored() {
			return fmt.Errorf("%w: invalid recorded criterion identity", sentinel.ErrInvalidInput)
		}
		seen[r.CriterionID] = true
	}
	return nil
}
func caseMeans(groups map[id.CaseID][]float64) []float64 {
	keys := make([]id.CaseID, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	values := make([]float64, 0, len(keys))
	for _, key := range keys {
		values = append(values, average(groups[key]))
	}
	return values
}
func average(values []float64) float64 {
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}
func bootstrap(ctx context.Context, values []float64, seed uint64) *ConfidenceInterval {
	rng := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15)) //nolint:gosec // Reproducible statistical resampling, not security randomness.
	samples := make([]float64, 2000)
	for i := range samples {
		if ctx.Err() != nil {
			return nil
		}
		var sum float64
		for range values {
			sum += values[rng.IntN(len(values))]
		}
		samples[i] = sum / float64(len(values))
	}
	sort.Float64s(samples)
	return &ConfidenceInterval{Lower: samples[49], Upper: samples[1949]}
}
