// Package report generates human-readable and machine-readable reports
// from evaluation runs.
package report

import (
	"io"
	"time"

	"github.com/xraph/sentinel/evalrun"
)

// Format identifies a report output format.
type Format string

const (
	FormatTerminal Format = "terminal"
	FormatJSON     Format = "json"
	FormatHTML     Format = "html"
	FormatCI       Format = "ci"
)

// Reporter generates a report in a specific format.
type Reporter interface {
	Format() Format
	Render(w io.Writer, report *Report) error
}

// Report holds all data needed to render an evaluation report.
type Report struct {
	Run             *evalrun.Run         `json:"run"`
	Results         []*evalrun.Result    `json:"results"`
	Stats           *evalrun.ResultStats `json:"stats"`
	DimensionScores map[string]float64   `json:"dimension_scores,omitempty"`
	GeneratedAt     time.Time            `json:"generated_at"`
}

// NewReport creates a report from run data.
func NewReport(run *evalrun.Run, results []*evalrun.Result, stats *evalrun.ResultStats) *Report {
	return &Report{
		Run:             run,
		Results:         results,
		Stats:           stats,
		DimensionScores: stats.DimensionScores,
		GeneratedAt:     time.Now().UTC(),
	}
}
