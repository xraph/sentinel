package baseline

import (
	"github.com/xraph/sentinel/evalrun"
)

// RegressionResult holds the outcome of a regression detection check.
type RegressionResult struct {
	HasRegression   bool               `json:"has_regression"`
	PassRateDelta   float64            `json:"pass_rate_delta"`
	AvgScoreDelta   float64            `json:"avg_score_delta"`
	DimensionDeltas map[string]float64 `json:"dimension_deltas,omitempty"`
	RegressedCases  []RegressedCase    `json:"regressed_cases,omitempty"`
}

// RegressedCase identifies a single case that regressed from baseline.
type RegressedCase struct {
	CaseID     string  `json:"case_id"`
	CaseName   string  `json:"case_name"`
	OldScore   float64 `json:"old_score"`
	NewScore   float64 `json:"new_score"`
	Delta      float64 `json:"delta"`
}

// DetectRegression compares a run's results against a baseline and reports
// any regressions that exceed the given threshold.
func DetectRegression(stats *evalrun.ResultStats, results []*evalrun.Result, b *Baseline, threshold float64) *RegressionResult {
	rr := &RegressionResult{
		PassRateDelta:   stats.PassRate - b.PassRate,
		AvgScoreDelta:   stats.AvgScore - b.AvgScore,
		DimensionDeltas: make(map[string]float64),
	}

	// Check overall regression.
	if rr.PassRateDelta < -threshold || rr.AvgScoreDelta < -threshold {
		rr.HasRegression = true
	}

	// Check dimension regressions.
	for dim, baselineScore := range b.DimensionScores {
		currentScore := stats.DimensionScores[dim]
		delta := currentScore - baselineScore
		rr.DimensionDeltas[dim] = delta
		if delta < -threshold {
			rr.HasRegression = true
		}
	}

	// Check per-case regressions.
	baselineLookup := make(map[string]BaselineResult)
	for _, br := range b.Results {
		baselineLookup[br.CaseID.String()] = br
	}

	for _, r := range results {
		if br, ok := baselineLookup[r.CaseID.String()]; ok {
			delta := r.Score - br.Score
			if delta < -threshold {
				rr.HasRegression = true
				rr.RegressedCases = append(rr.RegressedCases, RegressedCase{
					CaseID:   r.CaseID.String(),
					CaseName: r.CaseName,
					OldScore: br.Score,
					NewScore: r.Score,
					Delta:    delta,
				})
			}
		}
	}

	return rr
}
