// Package observability provides a metrics extension for Sentinel that records
// lifecycle event counts via go-utils MetricFactory.
package observability

import (
	"context"
	"time"

	gu "github.com/xraph/go-utils/metrics"

	"github.com/xraph/sentinel/plugin"
	"github.com/xraph/sentinel/id"
)

// Compile-time interface checks.
var (
	_ plugin.Extension            = (*MetricsExtension)(nil)
	_ plugin.EvalRunStarted       = (*MetricsExtension)(nil)
	_ plugin.EvalRunCompleted     = (*MetricsExtension)(nil)
	_ plugin.EvalRunFailed        = (*MetricsExtension)(nil)
	_ plugin.CaseCompleted        = (*MetricsExtension)(nil)
	_ plugin.CaseFailed           = (*MetricsExtension)(nil)
	_ plugin.RegressionDetected   = (*MetricsExtension)(nil)
	_ plugin.BaselineSaved        = (*MetricsExtension)(nil)
	_ plugin.RedTeamStarted       = (*MetricsExtension)(nil)
	_ plugin.RedTeamCompleted     = (*MetricsExtension)(nil)
	_ plugin.PersonaEvalStarted   = (*MetricsExtension)(nil)
	_ plugin.PersonaEvalCompleted = (*MetricsExtension)(nil)
	_ plugin.ComparisonCompleted  = (*MetricsExtension)(nil)
)

// MetricsExtension records lifecycle metrics via go-utils MetricFactory.
type MetricsExtension struct {
	EvalRunStartedCount       gu.Counter
	EvalRunCompletedCount     gu.Counter
	EvalRunFailedCount        gu.Counter
	CaseCompletedCount        gu.Counter
	CaseFailedCount           gu.Counter
	RegressionDetectedCount   gu.Counter
	BaselineSavedCount        gu.Counter
	RedTeamStartedCount       gu.Counter
	RedTeamCompletedCount     gu.Counter
	RedTeamBypassCount        gu.Counter
	PersonaEvalStartedCount   gu.Counter
	PersonaEvalCompletedCount gu.Counter
	ComparisonCompletedCount  gu.Counter
}

// NewMetricsExtension creates a MetricsExtension with a default metrics collector.
func NewMetricsExtension() *MetricsExtension {
	return NewMetricsExtensionWithFactory(gu.NewMetricsCollector("sentinel/observability"))
}

// NewMetricsExtensionWithFactory creates a MetricsExtension with the provided MetricFactory.
func NewMetricsExtensionWithFactory(factory gu.MetricFactory) *MetricsExtension {
	return &MetricsExtension{
		EvalRunStartedCount:       factory.Counter("sentinel.eval.run.started"),
		EvalRunCompletedCount:     factory.Counter("sentinel.eval.run.completed"),
		EvalRunFailedCount:        factory.Counter("sentinel.eval.run.failed"),
		CaseCompletedCount:        factory.Counter("sentinel.case.completed"),
		CaseFailedCount:           factory.Counter("sentinel.case.failed"),
		RegressionDetectedCount:   factory.Counter("sentinel.regression.detected"),
		BaselineSavedCount:        factory.Counter("sentinel.baseline.saved"),
		RedTeamStartedCount:       factory.Counter("sentinel.redteam.started"),
		RedTeamCompletedCount:     factory.Counter("sentinel.redteam.completed"),
		RedTeamBypassCount:        factory.Counter("sentinel.redteam.bypass"),
		PersonaEvalStartedCount:   factory.Counter("sentinel.persona.eval.started"),
		PersonaEvalCompletedCount: factory.Counter("sentinel.persona.eval.completed"),
		ComparisonCompletedCount:  factory.Counter("sentinel.comparison.completed"),
	}
}

// Name implements plugin.Extension.
func (m *MetricsExtension) Name() string { return "observability-metrics" }

func (m *MetricsExtension) OnEvalRunStarted(_ context.Context, _ id.SuiteID, _ id.EvalRunID, _ string) error {
	m.EvalRunStartedCount.Inc()
	return nil
}

func (m *MetricsExtension) OnEvalRunCompleted(_ context.Context, _ id.SuiteID, _ id.EvalRunID, _ float64, _ time.Duration) error {
	m.EvalRunCompletedCount.Inc()
	return nil
}

func (m *MetricsExtension) OnEvalRunFailed(_ context.Context, _ id.SuiteID, _ id.EvalRunID, _ error) error {
	m.EvalRunFailedCount.Inc()
	return nil
}

func (m *MetricsExtension) OnCaseCompleted(_ context.Context, _ id.EvalRunID, _ id.CaseID, _ float64, _ time.Duration) error {
	m.CaseCompletedCount.Inc()
	return nil
}

func (m *MetricsExtension) OnCaseFailed(_ context.Context, _ id.EvalRunID, _ id.CaseID, _ error) error {
	m.CaseFailedCount.Inc()
	return nil
}

func (m *MetricsExtension) OnRegressionDetected(_ context.Context, _ id.SuiteID, _ id.BaselineID, _ float64) error {
	m.RegressionDetectedCount.Inc()
	return nil
}

func (m *MetricsExtension) OnBaselineSaved(_ context.Context, _ id.SuiteID, _ id.BaselineID) error {
	m.BaselineSavedCount.Inc()
	return nil
}

func (m *MetricsExtension) OnRedTeamStarted(_ context.Context, _ id.SuiteID, _ int) error {
	m.RedTeamStartedCount.Inc()
	return nil
}

func (m *MetricsExtension) OnRedTeamCompleted(_ context.Context, _ id.SuiteID, bypassCount int, _ time.Duration) error {
	m.RedTeamCompletedCount.Inc()
	m.RedTeamBypassCount.Add(float64(bypassCount))
	return nil
}

func (m *MetricsExtension) OnPersonaEvalStarted(_ context.Context, _ id.EvalRunID, _ string) error {
	m.PersonaEvalStartedCount.Inc()
	return nil
}

func (m *MetricsExtension) OnPersonaEvalCompleted(_ context.Context, _ id.EvalRunID, _ string, _ map[string]float64) error {
	m.PersonaEvalCompletedCount.Inc()
	return nil
}

func (m *MetricsExtension) OnComparisonCompleted(_ context.Context, _ id.SuiteID, _ []string, _ time.Duration) error {
	m.ComparisonCompletedCount.Inc()
	return nil
}
