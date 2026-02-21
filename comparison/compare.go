// Package comparison enables side-by-side evaluation of multiple models
// or configurations against the same suite.
package comparison

import (
	"context"
	"fmt"
	"time"

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
// comparison report.
func Compare(ctx context.Context, eng *engine.Engine, cfg *CompareConfig) (*CompareReport, error) {
	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("sentinel: comparison requires at least one target")
	}

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
