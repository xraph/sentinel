package report

import (
	"encoding/json"
	"io"
)

// JSONReporter renders a report as formatted JSON.
type JSONReporter struct{}

// NewJSONReporter creates a new JSON reporter.
func NewJSONReporter() *JSONReporter { return &JSONReporter{} }

func (r *JSONReporter) Format() Format { return FormatJSON }

func (r *JSONReporter) Render(w io.Writer, rpt *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rpt)
}
