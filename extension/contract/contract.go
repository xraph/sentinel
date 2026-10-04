// Package contract wires Sentinel into the Forge dashboard's contract path.
// It registers the `sentinel` contributor and answers its intents from the
// engine. This is the surface the React shell reads; the templ dashboard in
// sentinel/dashboard is retired once every surface has an equivalent here.
package contract

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/xraph/forge"
	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"
	"github.com/xraph/forge/extensions/dashboard/contract/dispatcher"
	"github.com/xraph/forge/extensions/dashboard/contract/loader"

	"github.com/xraph/sentinel/engine"
)

//go:embed manifest.yaml
var manifestYAML []byte

// ContributorName is the join key between this contract and
// packages/plugin-sentinel's `extension` field. A mismatch hides the React
// plugin with no error anywhere, which is what an uninstalled extension
// looks like.
const ContributorName = "sentinel"

// Deps bundles what the handlers need.
type Deps struct {
	// Engine is required.
	Engine *engine.Engine
	// DashboardAppID is the app served when the principal carries no
	// app_id claim. Empty means such requests are refused.
	DashboardAppID string
	// Logger receives every error a handler maps to INTERNAL. Optional.
	Logger forge.Logger
}

// Register loads and validates the embedded manifest, registers the
// contributor, and binds every intent's handler.
func Register(d *dispatcher.Dispatcher, reg dashcontract.Registry, wreg dashcontract.WardenRegistry, deps Deps) error {
	if deps.Engine == nil {
		return errors.New("sentinel/contract: Engine is required")
	}
	m, err := loader.Load(bytes.NewReader(manifestYAML), "sentinel/contract/manifest.yaml")
	if err != nil {
		return fmt.Errorf("sentinel/contract: load manifest: %w", err)
	}
	if err := loader.Validate(m, wreg); err != nil {
		return fmt.Errorf("sentinel/contract: validate manifest: %w", err)
	}
	if err := reg.Register(m); err != nil {
		return fmt.Errorf("sentinel/contract: register manifest: %w", err)
	}
	for _, bind := range []func() error{
		func() error { return query(d, "config.get", configGetHandler(deps)) },
		func() error { return query(d, "suites.list", suitesListHandler(deps)) },
		func() error { return query(d, "suites.detail", suitesDetailHandler(deps)) },
		func() error { return command(d, "suites.create", suitesCreateHandler(deps)) },
		func() error { return command(d, "suites.update", suitesUpdateHandler(deps)) },
		func() error { return command(d, "suites.delete", suitesDeleteHandler(deps)) },
		func() error { return query(d, "cases.list", casesListHandler(deps)) },
		func() error { return query(d, "cases.detail", casesDetailHandler(deps)) },
		func() error { return command(d, "cases.create", casesCreateHandler(deps)) },
		func() error { return command(d, "cases.update", casesUpdateHandler(deps)) },
		func() error { return command(d, "cases.delete", casesDeleteHandler(deps)) },
		func() error { return command(d, "cases.import", casesImportHandler(deps)) },
		func() error { return query(d, "prompts.list", promptsListHandler(deps)) },
		func() error { return query(d, "prompts.detail", promptsDetailHandler(deps)) },
		func() error { return command(d, "prompts.create", promptsCreateHandler(deps)) },
		func() error { return command(d, "prompts.setCurrent", promptsSetCurrentHandler(deps)) },
	} {
		if err := bind(); err != nil {
			return fmt.Errorf("sentinel/contract: %w", err)
		}
	}
	return nil
}

func query[I, O any](d *dispatcher.Dispatcher, intent string, fn func(context.Context, I, dashcontract.Principal) (O, error)) error {
	return dispatcher.RegisterQuery(d, ContributorName, intent, 1, fn)
}

func command[I, O any](d *dispatcher.Dispatcher, intent string, fn func(context.Context, I, dashcontract.Principal) (O, error)) error {
	return dispatcher.RegisterCommand(d, ContributorName, intent, 1, fn)
}
