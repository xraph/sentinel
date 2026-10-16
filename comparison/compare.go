// Package comparison enables side-by-side evaluation of multiple models
// or configurations against the same suite.
package comparison

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/target"
)

// CompareConfig configures a multi-model comparison run.
type CompareConfig struct {
	SuiteID id.SuiteID
	Targets []target.Target
	Scorers []scorer.Scorer
	Tags    []string
}

// ModelResult holds the eval result for a single model in a comparison.
type ModelResult struct {
	ModelName string            `json:"model_name"`
	RunResult *engine.RunResult `json:"run_result"`
}

// CompareReport holds the comparison results across all models.
type CompareReport struct {
	SuiteID   id.SuiteID    `json:"suite_id"`
	Models    []ModelResult `json:"models"`
	Duration  time.Duration `json:"duration"`
	CreatedAt time.Time     `json:"created_at"`
}

// Compare runs the same suite against multiple targets and produces a
// comparison report. It runs inside a "compare targets" span, so each
// target's "evaluate run" span lands in one trace.
func Compare(ctx context.Context, eng *engine.Engine, cfg *CompareConfig) (*CompareReport, error) {
	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("sentinel: comparison requires at least one target")
	}

	ctx, span := otel.Tracer(engine.InstrumentationName).Start(ctx, "compare targets", trace.WithAttributes(
		attribute.String("sentinel.suite.id", cfg.SuiteID.String()),
		attribute.Int("sentinel.compare.targets", len(cfg.Targets)),
	))
	defer span.End()

	start := time.Now()
	report := &CompareReport{
		SuiteID:   cfg.SuiteID,
		CreatedAt: start.UTC(),
	}

	models := make([]string, 0, len(cfg.Targets))
	for _, tgt := range cfg.Targets {
		runCfg := &engine.RunConfig{
			SuiteID: cfg.SuiteID,
			Model:   tgt.Name(),
			Target:  tgt,
			Scorers: cfg.Scorers,
			Tags:    cfg.Tags,
		}

		result, err := eng.RunEval(ctx, runCfg)
		if err != nil {
			// A caller that gave up cancelled the comparison; anything
			// else failed it. Only the error's type goes on the span.
			if ctx.Err() == nil {
				span.SetStatus(codes.Error, "run error")
				span.AddEvent("exception", trace.WithAttributes(attribute.String("exception.type", innermostType(err))))
			}
			return nil, fmt.Errorf("sentinel: compare %s: %w", tgt.Name(), err)
		}

		report.Models = append(report.Models, ModelResult{
			ModelName: tgt.Name(),
			RunResult: result,
		})
		models = append(models, tgt.Name())
	}

	report.Duration = time.Since(start)

	eng.Extensions().EmitComparisonCompleted(ctx, cfg.SuiteID, models, report.Duration)

	return report, nil
}

// innermostType is the Go type of the innermost error in err's chain, for
// exception.type. The message never goes on a span.
func innermostType(err error) string {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return fmt.Sprintf("%T", err)
		}
		err = next
	}
}
