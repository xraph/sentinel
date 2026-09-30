package baseline

import (
	"sort"

	"github.com/xraph/sentinel/evalrun"
)

// regressionEpsilon absorbs float64 rounding in a delta. Scores and pass
// rates are fractions such as 19/20, whose difference from 1.0 comes out as
// -0.050000000000000044, so a drop exactly equal to the threshold would
// otherwise regress in one suite and not in another. No real drop is this
// small.
const regressionEpsilon = 1e-9

// fellBelow reports whether delta is a drop larger than threshold.
func fellBelow(delta, threshold float64) bool {
	return delta < -threshold-regressionEpsilon
}

// RegressionResult holds the outcome of a regression detection check.
type RegressionResult struct {
	HasRegression   bool               `json:"has_regression"`
	Threshold       float64            `json:"threshold"`
	PassRateDelta   float64            `json:"pass_rate_delta"`
	AvgScoreDelta   float64            `json:"avg_score_delta"`
	DimensionDeltas map[string]float64 `json:"dimension_deltas,omitempty"`
	RegressedCases  []RegressedCase    `json:"regressed_cases,omitempty"`

	// MissingCases are baseline cases with no result in the run. They do not
	// set HasRegression: the suite changed, which is a different question.
	MissingCases []CaseRef `json:"missing_cases,omitempty"`
	// NewCases are run results with no baseline case to compare against.
	NewCases []CaseRef `json:"new_cases,omitempty"`
	// MissingDimensions are baseline dimensions the run did not measure.
	// They set HasRegression, the conservative answer for CI, and are kept
	// out of DimensionDeltas so nobody reads "not measured" as a large drop.
	MissingDimensions []string `json:"missing_dimensions,omitempty"`

	// worstMissing is the most negative delta contributed by an unmeasured dimension.
	// It is not exported and has no JSON tag.
	worstMissing float64
}

// RegressedCase identifies a single case that regressed from baseline.
type RegressedCase struct {
	CaseID   string  `json:"case_id"`
	CaseName string  `json:"case_name"`
	OldScore float64 `json:"old_score"`
	NewScore float64 `json:"new_score"`
	Delta    float64 `json:"delta"`
}

// CaseRef names a case without scores.
type CaseRef struct {
	CaseID   string `json:"case_id"`
	CaseName string `json:"case_name"`
}

// DetectRegression compares a run's results against a baseline. A metric
// regresses when it falls more than threshold below the baseline; one
// absolute threshold covers pass rate, average score, each dimension and
// each case, and lower is always worse.
func DetectRegression(stats *evalrun.ResultStats, results []*evalrun.Result, b *Baseline, threshold float64) *RegressionResult {
	rr := &RegressionResult{
		Threshold:       threshold,
		PassRateDelta:   stats.PassRate - b.PassRate,
		AvgScoreDelta:   stats.AvgScore - b.AvgScore,
		DimensionDeltas: make(map[string]float64),
	}
	if fellBelow(rr.PassRateDelta, threshold) || fellBelow(rr.AvgScoreDelta, threshold) {
		rr.HasRegression = true
	}

	for dim, baselineScore := range b.DimensionScores {
		current, measured := stats.DimensionScores[dim]
		if !measured {
			rr.MissingDimensions = append(rr.MissingDimensions, dim)
			rr.HasRegression = true
			missingDelta := -baselineScore
			if missingDelta < rr.worstMissing {
				rr.worstMissing = missingDelta
			}
			continue
		}
		delta := current - baselineScore
		rr.DimensionDeltas[dim] = delta
		if fellBelow(delta, threshold) {
			rr.HasRegression = true
		}
	}
	sort.Strings(rr.MissingDimensions)

	baselineLookup := make(map[string]Result, len(b.Results))
	for _, br := range b.Results {
		baselineLookup[br.CaseID.String()] = br
	}
	seen := make(map[string]bool, len(results))
	for _, r := range results {
		key := r.CaseID.String()
		seen[key] = true
		br, ok := baselineLookup[key]
		if !ok {
			rr.NewCases = append(rr.NewCases, CaseRef{CaseID: key, CaseName: r.CaseName})
			continue
		}
		delta := r.Score - br.Score
		if fellBelow(delta, threshold) {
			rr.HasRegression = true
			rr.RegressedCases = append(rr.RegressedCases, RegressedCase{
				CaseID: key, CaseName: r.CaseName, OldScore: br.Score, NewScore: r.Score, Delta: delta,
			})
		}
	}
	for _, br := range b.Results {
		if !seen[br.CaseID.String()] {
			rr.MissingCases = append(rr.MissingCases, CaseRef{CaseID: br.CaseID.String(), CaseName: br.CaseName})
		}
	}
	return rr
}

// WorstDelta is the most negative delta found: pass rate, average score,
// any dimension or any regressed case. An unmeasured dimension counts as
// losing its whole baseline score, the most it could have fallen, so the
// value always agrees with HasRegression. It is what RegressionDetected
// hooks receive.
func (rr *RegressionResult) WorstDelta() float64 {
	worst := rr.PassRateDelta
	if rr.AvgScoreDelta < worst {
		worst = rr.AvgScoreDelta
	}
	for _, d := range rr.DimensionDeltas {
		if d < worst {
			worst = d
		}
	}
	if rr.worstMissing < worst {
		worst = rr.worstMissing
	}
	for _, c := range rr.RegressedCases {
		if c.Delta < worst {
			worst = c.Delta
		}
	}
	return worst
}
