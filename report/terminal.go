package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/xraph/sentinel/evalrun"
)

// TerminalReporter renders a colored table report to the terminal.
type TerminalReporter struct{}

// NewTerminalReporter creates a new terminal reporter.
func NewTerminalReporter() *TerminalReporter { return &TerminalReporter{} }

func (r *TerminalReporter) Format() Format { return FormatTerminal }

func (r *TerminalReporter) Render(w io.Writer, rpt *Report) error {
	sep := strings.Repeat("─", 80)

	fmt.Fprintf(w, "\n%s\n", sep)
	fmt.Fprintf(w, "  Evaluation Report: %s\n", rpt.Run.ID.String())
	fmt.Fprintf(w, "  Suite: %s  Model: %s\n", rpt.Run.SuiteID.String(), rpt.Run.Model)
	if rpt.Run.PersonaRef != "" {
		fmt.Fprintf(w, "  Persona: %s\n", rpt.Run.PersonaRef)
	}
	fmt.Fprintf(w, "%s\n\n", sep)

	// Summary.
	fmt.Fprintf(w, "  Pass Rate: %.1f%%  (%d/%d)\n", rpt.Stats.PassRate*100, rpt.Stats.Passed, rpt.Stats.TotalCases)
	fmt.Fprintf(w, "  Avg Score: %.3f\n", rpt.Stats.AvgScore)
	fmt.Fprintf(w, "  Avg Latency: %dms\n", rpt.Stats.AvgLatencyMs)
	fmt.Fprintf(w, "  Total Tokens: %d  Cost: $%.4f\n", rpt.Stats.TotalTokens, rpt.Stats.TotalCost)

	// Dimension scores.
	if len(rpt.DimensionScores) > 0 {
		fmt.Fprintf(w, "\n  Dimension Scores:\n")
		for dim, score := range rpt.DimensionScores {
			fmt.Fprintf(w, "    %-20s %.3f\n", dim, score)
		}
	}

	// Results table.
	fmt.Fprintf(w, "\n  %-4s %-30s %-6s %-8s %-10s\n", "#", "Case", "Status", "Score", "Latency")
	fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 62))

	for i, res := range rpt.Results {
		status := statusIndicator(res.Status)
		name := res.CaseName
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		fmt.Fprintf(w, "  %-4d %-30s %s  %.3f    %dms\n",
			i+1, name, status, res.Score, res.LatencyMs)
	}

	fmt.Fprintf(w, "\n%s\n", sep)
	return nil
}

func statusIndicator(status evalrun.ResultStatus) string {
	switch status {
	case evalrun.StatusPass:
		return "\033[32mPASS\033[0m"
	case evalrun.StatusFail:
		return "\033[31mFAIL\033[0m"
	case evalrun.StatusError:
		return "\033[33mERR \033[0m"
	default:
		return "????"
	}
}
