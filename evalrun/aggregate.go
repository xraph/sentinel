package evalrun

// Aggregate computes the shared result counters used by stores and recorded runs.
// Results must contain non-nil entries.
func Aggregate(results []*Result) *ResultStats {
	stats := &ResultStats{
		DimensionScores: make(map[string]float64),
	}
	dimCounts := make(map[string]int)
	var totalScore float64
	var totalLatency int64

	for _, r := range results {
		stats.TotalCases++
		totalScore += r.Score
		totalLatency += int64(r.LatencyMs)
		stats.TotalTokens += r.TokensUsed
		stats.TotalCost += r.Cost

		switch r.Status {
		case StatusPass:
			stats.Passed++
		case StatusFail:
			stats.Failed++
		case StatusError:
			stats.Errored++
		}
		for dim, score := range r.DimensionScores {
			stats.DimensionScores[dim] += score
			dimCounts[dim]++
		}
	}
	if stats.TotalCases > 0 {
		stats.PassRate = float64(stats.Passed) / float64(stats.TotalCases)
		stats.AvgScore = totalScore / float64(stats.TotalCases)
		stats.AvgLatencyMs = int(totalLatency / int64(stats.TotalCases))
	}
	for dim, total := range stats.DimensionScores {
		if dimCounts[dim] > 0 {
			stats.DimensionScores[dim] = total / float64(dimCounts[dim])
		}
	}
	return stats
}
