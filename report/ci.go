package report

import (
	"fmt"
	"io"

	"github.com/xraph/sentinel/evalrun"
)

// CIReporter outputs GitHub Actions workflow annotations.
type CIReporter struct{}

// NewCIReporter creates a new CI reporter.
func NewCIReporter() *CIReporter { return &CIReporter{} }

func (r *CIReporter) Format() Format { return FormatCI }

func (r *CIReporter) Render(w io.Writer, rpt *Report) error {
	// Summary as a notice.
	fmt.Fprintf(w, "::notice title=Sentinel Eval Summary::Pass Rate: %.1f%% (%d/%d) | Avg Score: %.3f | Model: %s\n",
		rpt.Stats.PassRate*100, rpt.Stats.Passed, rpt.Stats.TotalCases,
		rpt.Stats.AvgScore, rpt.Run.Model,
	)

	// Per-case annotations.
	for _, res := range rpt.Results {
		switch res.Status {
		case evalrun.StatusPass:
			// No annotation for passing cases.
		case evalrun.StatusFail:
			fmt.Fprintf(w, "::warning title=FAIL: %s::Score: %.3f | %s\n",
				res.CaseName, res.Score, firstReason(res))
		case evalrun.StatusError:
			fmt.Fprintf(w, "::error title=ERROR: %s::%s\n",
				res.CaseName, res.Error)
		}
	}

	// Overall pass/fail.
	if rpt.Stats.PassRate < 1.0 {
		fmt.Fprintf(w, "::warning title=Evaluation Not Perfect::%d of %d cases failed\n",
			rpt.Stats.Failed+rpt.Stats.Errored, rpt.Stats.TotalCases)
	}

	return nil
}

func firstReason(res *evalrun.Result) string {
	for _, sr := range res.ScorerResults {
		if !sr.Passed && sr.Reason != "" {
			return sr.Reason
		}
	}
	return "no reason provided"
}
