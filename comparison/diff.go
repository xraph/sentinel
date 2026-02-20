package comparison

import (
	"github.com/xraph/sentinel/evalrun"
)

// ScoreDelta records the change in a metric between current and baseline.
type ScoreDelta struct {
	Metric   string  `json:"metric"`
	Current  float64 `json:"current"`
	Baseline float64 `json:"baseline"`
	Delta    float64 `json:"delta"`
}

// DiffReport holds the deltas between a current run and a baseline.
type DiffReport struct {
	Deltas          []ScoreDelta       `json:"deltas"`
	DimensionDeltas map[string]float64 `json:"dimension_deltas,omitempty"`
}

// Diff computes score deltas between current run stats and baseline stats.
func Diff(current, baseline *evalrun.ResultStats) *DiffReport {
	report := &DiffReport{
		DimensionDeltas: make(map[string]float64),
	}

	report.Deltas = append(report.Deltas,
		ScoreDelta{"pass_rate", current.PassRate, baseline.PassRate, current.PassRate - baseline.PassRate},
		ScoreDelta{"avg_score", current.AvgScore, baseline.AvgScore, current.AvgScore - baseline.AvgScore},
		ScoreDelta{"avg_latency_ms", float64(current.AvgLatencyMs), float64(baseline.AvgLatencyMs), float64(current.AvgLatencyMs - baseline.AvgLatencyMs)},
		ScoreDelta{"total_cost", current.TotalCost, baseline.TotalCost, current.TotalCost - baseline.TotalCost},
	)

	// Dimension deltas.
	allDims := make(map[string]bool)
	for dim := range current.DimensionScores {
		allDims[dim] = true
	}
	for dim := range baseline.DimensionScores {
		allDims[dim] = true
	}

	for dim := range allDims {
		cur := current.DimensionScores[dim]
		base := baseline.DimensionScores[dim]
		report.DimensionDeltas[dim] = cur - base
	}

	return report
}
