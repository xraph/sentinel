package contract

import (
	"context"

	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"
)

type configGetInput struct{}

// TargetView is a registered target.
type TargetView struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ScorerView is a registered scorer. RequiresConfig scorers cannot be
// picked for a run without configuration; UsesLLM scorers cost money.
type ScorerView struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Dimension      string `json:"dimension,omitempty"`
	UsesLLM        bool   `json:"usesLlm"`
	RequiresConfig bool   `json:"requiresConfig"`
}

// ConfigView is the engine's effective configuration and what it can run.
type ConfigView struct {
	DefaultModel        string       `json:"defaultModel"`
	Temperature         float64      `json:"temperature"`
	PassThreshold       float64      `json:"passThreshold"`
	RegressionThreshold float64      `json:"regressionThreshold"`
	Concurrency         int          `json:"concurrency"`
	Targets             []TargetView `json:"targets"`
	Scorers             []ScorerView `json:"scorers"`
}

func configGetHandler(d Deps) func(context.Context, configGetInput, dashcontract.Principal) (ConfigView, error) {
	return func(_ context.Context, _ configGetInput, p dashcontract.Principal) (ConfigView, error) {
		if _, err := d.resolveApp(p); err != nil {
			return ConfigView{}, err
		}
		cfg := d.Engine.Config()
		out := ConfigView{
			DefaultModel: cfg.DefaultModel, Temperature: cfg.Temperature, PassThreshold: cfg.PassThreshold,
			RegressionThreshold: cfg.RegressionThreshold, Concurrency: cfg.Concurrency,
			Targets: []TargetView{}, Scorers: []ScorerView{},
		}
		for _, t := range d.Engine.Targets() {
			out.Targets = append(out.Targets, TargetView{Name: t.Name, Description: t.Description})
		}
		for _, s := range d.Engine.Scorers().Descriptors() {
			out.Scorers = append(out.Scorers, ScorerView{
				Name: s.Name, Description: s.Description, Dimension: s.Dimension, UsesLLM: s.UsesLLM, RequiresConfig: s.RequiresConfig,
			})
		}
		return out, nil
	}
}
