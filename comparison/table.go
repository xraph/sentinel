package comparison

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// FormatTable writes a terminal comparison table.
func FormatTable(w io.Writer, report *CompareReport) {
	if len(report.Models) == 0 {
		fmt.Fprintln(w, "No models to compare.")
		return
	}

	sep := strings.Repeat("─", 80)
	fmt.Fprintf(w, "\n%s\n", sep)
	fmt.Fprintf(w, "  Model Comparison — Suite: %s\n", report.SuiteID.String())
	fmt.Fprintf(w, "%s\n\n", sep)

	// Header.
	fmt.Fprintf(w, "  %-25s %8s %8s %8s %8s %10s\n",
		"Model", "Pass%", "AvgScore", "Latency", "Tokens", "Cost")
	fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 73))

	for _, m := range report.Models {
		stats := m.RunResult.Stats
		fmt.Fprintf(w, "  %-25s %7.1f%% %8.3f %6dms %8d $%8.4f\n",
			m.ModelName,
			stats.PassRate*100,
			stats.AvgScore,
			stats.AvgLatencyMs,
			stats.TotalTokens,
			stats.TotalCost,
		)
	}

	// Dimension comparison if available.
	hasDimensions := false
	for _, m := range report.Models {
		if len(m.RunResult.Stats.DimensionScores) > 0 {
			hasDimensions = true
			break
		}
	}

	if hasDimensions {
		// Collect all dimensions.
		dims := make(map[string]bool)
		for _, m := range report.Models {
			for dim := range m.RunResult.Stats.DimensionScores {
				dims[dim] = true
			}
		}

		fmt.Fprintf(w, "\n  Dimension Scores:\n")
		fmt.Fprintf(w, "  %-25s", "Model")
		for dim := range dims {
			fmt.Fprintf(w, " %12s", dim)
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 25+13*len(dims)))

		for _, m := range report.Models {
			fmt.Fprintf(w, "  %-25s", m.ModelName)
			for dim := range dims {
				score := m.RunResult.Stats.DimensionScores[dim]
				fmt.Fprintf(w, " %12.3f", score)
			}
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintf(w, "\n  Duration: %s\n", report.Duration.Truncate(time.Millisecond))
	fmt.Fprintf(w, "%s\n", sep)
}
