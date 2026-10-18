package engine

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/scorer"
)

// capturedCriterion includes resolution failures so a missing scorer cannot
// silently reduce the denominator or remove a required gate.
type capturedCriterion struct {
	ID       string
	Name     string
	Required bool
	Scorer   scorer.Scorer
	Err      error
}

// scoreCapturedCase scores an already captured output. It has no target access.
func (e *Engine) scoreCapturedCase(ctx context.Context, result *evalrun.Result, input *scorer.Input, criteria []capturedCriterion, threshold float64) error {
	var total float64
	var scored int
	var firstErr error
	var failures []string
	var requiredFailed bool
	result.ScorerResults = make([]evalrun.ScorerResult, 0, len(criteria))
	result.DimensionScores = make(map[string]float64)
	counts := make(map[string]int)
	for _, criterion := range criteria {
		name := criterion.Name
		if criterion.Scorer != nil {
			name = criterion.Scorer.Name()
		}
		sr := evalrun.ScorerResult{CriterionID: criterion.ID, Required: criterion.Required, ScorerName: name}
		scoreErr := criterion.Err
		var out *scorer.Output
		if scoreErr == nil {
			if criterion.Scorer == nil {
				scoreErr = fmt.Errorf("sentinel: scorer %s unavailable", name)
			} else {
				out, scoreErr = e.score(ctx, criterion.Scorer, input)
			}
		}
		if scoreErr == nil && (out == nil || math.IsNaN(out.Score) || math.IsInf(out.Score, 0) || out.Score < 0 || out.Score > 1) {
			scoreErr = fmt.Errorf("sentinel: scorer %s returned an invalid score", name)
		}
		if out != nil && out.Usage != nil {
			if usageErr := out.Usage.Validate(); usageErr != nil {
				if scoreErr == nil {
					scoreErr = usageErr
				}
			} else {
				usage := *out.Usage
				sr.Usage = &usage
			}
		}
		if scoreErr != nil {
			sr.Reason = evalrun.ScorerErrorPrefix + scoreErr.Error()
			failures = append(failures, fmt.Sprintf("scorer %s: %v", name, scoreErr))
			if firstErr == nil {
				firstErr = scoreErr
			}
		} else {
			sr.Score, sr.Passed, sr.Reason, sr.Dimension, sr.Details = out.Score, out.Passed, out.Reason, out.Dimension, out.Details
			total += out.Score
			scored++
			requiredFailed = requiredFailed || (criterion.Required && !out.Passed)
			if out.Dimension != "" {
				result.DimensionScores[out.Dimension] += out.Score
				counts[out.Dimension]++
			}
		}
		result.ScorerResults = append(result.ScorerResults, sr)
	}
	result.Score = 0
	if scored > 0 {
		result.Score = total / float64(scored)
	}
	for dimension, sum := range result.DimensionScores {
		result.DimensionScores[dimension] = sum / float64(counts[dimension])
	}
	switch {
	case firstErr != nil:
		result.Status = evalrun.StatusError
		result.Error = strings.Join(failures, "; ")
	case requiredFailed || result.Score < threshold:
		result.Status = evalrun.StatusFail
	default:
		result.Status = evalrun.StatusPass
	}
	return firstErr
}
