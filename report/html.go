package report

import (
	"html/template"
	"io"
)

// HTMLReporter renders a report as an HTML page.
type HTMLReporter struct{}

// NewHTMLReporter creates a new HTML reporter.
func NewHTMLReporter() *HTMLReporter { return &HTMLReporter{} }

func (r *HTMLReporter) Format() Format { return FormatHTML }

func (r *HTMLReporter) Render(w io.Writer, rpt *Report) error {
	return htmlTmpl.Execute(w, rpt)
}

var htmlTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"pct": func(v float64) float64 { return v * 100 },
}).Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Sentinel Evaluation Report</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 900px; margin: 2rem auto; padding: 0 1rem; }
  h1 { border-bottom: 2px solid #333; padding-bottom: 0.5rem; }
  .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 1rem; margin: 1rem 0; }
  .stat { background: #f5f5f5; padding: 1rem; border-radius: 4px; }
  .stat .value { font-size: 1.5rem; font-weight: bold; }
  .stat .label { color: #666; font-size: 0.85rem; }
  table { width: 100%; border-collapse: collapse; margin: 1rem 0; }
  th, td { text-align: left; padding: 0.5rem; border-bottom: 1px solid #ddd; }
  th { background: #f5f5f5; }
  .pass { color: #22863a; font-weight: bold; }
  .fail { color: #cb2431; font-weight: bold; }
  .error { color: #e36209; font-weight: bold; }
</style>
</head>
<body>
<h1>Sentinel Evaluation Report</h1>
<p>Run: {{.Run.ID}} | Model: {{.Run.Model}}{{if .Run.PersonaRef}} | Persona: {{.Run.PersonaRef}}{{end}}</p>

<div class="summary">
  <div class="stat"><div class="value">{{printf "%.1f" (pct .Stats.PassRate)}}%</div><div class="label">Pass Rate</div></div>
  <div class="stat"><div class="value">{{printf "%.3f" .Stats.AvgScore}}</div><div class="label">Avg Score</div></div>
  <div class="stat"><div class="value">{{.Stats.AvgLatencyMs}}ms</div><div class="label">Avg Latency</div></div>
  <div class="stat"><div class="value">{{.Stats.TotalTokens}}</div><div class="label">Total Tokens</div></div>
  <div class="stat"><div class="value">${{printf "%.4f" .Stats.TotalCost}}</div><div class="label">Total Cost</div></div>
</div>

{{if .DimensionScores}}
<h2>Dimension Scores</h2>
<table>
  <tr><th>Dimension</th><th>Score</th></tr>
  {{range $dim, $score := .DimensionScores}}
  <tr><td>{{$dim}}</td><td>{{printf "%.3f" $score}}</td></tr>
  {{end}}
</table>
{{end}}

<h2>Results</h2>
<table>
  <tr><th>#</th><th>Case</th><th>Status</th><th>Score</th><th>Latency</th></tr>
  {{range $i, $r := .Results}}
  <tr>
    <td>{{$i}}</td>
    <td>{{$r.CaseName}}</td>
    <td class="{{if eq (printf "%s" $r.Status) "pass"}}pass{{else if eq (printf "%s" $r.Status) "fail"}}fail{{else}}error{{end}}">{{$r.Status}}</td>
    <td>{{printf "%.3f" $r.Score}}</td>
    <td>{{$r.LatencyMs}}ms</td>
  </tr>
  {{end}}
</table>

<p><small>Generated at {{.GeneratedAt.Format "2006-01-02 15:04:05 UTC"}}</small></p>
</body>
</html>`))
