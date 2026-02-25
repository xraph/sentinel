package extension

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/xraph/forge"
	"github.com/xraph/grove"
	"github.com/xraph/vessel"

	"github.com/xraph/sentinel/api"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/store"
	mongostore "github.com/xraph/sentinel/store/mongo"
	pgstore "github.com/xraph/sentinel/store/postgres"
	sqlitestore "github.com/xraph/sentinel/store/sqlite"
)

// ExtensionName is the name registered with Forge.
const ExtensionName = "sentinel"

// ExtensionDescription is the human-readable description.
const ExtensionDescription = "Composable AI evaluation and testing framework with persona-aware scoring"

// ExtensionVersion is the semantic version.
const ExtensionVersion = "0.1.0"

// Ensure Extension implements forge.Extension at compile time.
var _ forge.Extension = (*Extension)(nil)

// Extension adapts Sentinel as a Forge extension.
type Extension struct {
	*forge.BaseExtension

	config     Config
	eng        *engine.Engine
	apiHandler *api.API
	engineOpts []engine.Option
	useGrove   bool
}

// New creates a Sentinel Forge extension with the given options.
func New(opts ...ExtOption) *Extension {
	e := &Extension{
		BaseExtension: forge.NewBaseExtension(ExtensionName, ExtensionVersion, ExtensionDescription),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Engine returns the underlying Sentinel engine.
// This is nil until Register is called.
func (e *Extension) Engine() *engine.Engine { return e.eng }

// API returns the API handler.
func (e *Extension) API() *api.API { return e.apiHandler }

// Register implements [forge.Extension]. It initializes the engine,
// builds the API, and optionally registers HTTP routes.
func (e *Extension) Register(fapp forge.App) error {
	// 1. BaseExtension.Register stores app, logger, metrics.
	if err := e.BaseExtension.Register(fapp); err != nil {
		return err
	}

	// 2. Load config: YAML -> merge with programmatic -> fallback.
	if err := e.loadConfiguration(); err != nil {
		return err
	}

	// 3. Initialize the engine and API handler.
	if err := e.init(fapp); err != nil {
		return err
	}

	// 4. Register the engine in the DI container so other extensions can use it.
	if err := vessel.Provide(fapp.Container(), func() (*engine.Engine, error) {
		return e.eng, nil
	}); err != nil {
		return fmt.Errorf("sentinel: register engine in container: %w", err)
	}

	return nil
}

// init builds the engine and API handler.
func (e *Extension) init(fapp forge.App) error {
	// Resolve store from grove DI if configured.
	if e.useGrove {
		groveDB, err := e.resolveGroveDB(fapp)
		if err != nil {
			return fmt.Errorf("sentinel: %w", err)
		}
		s, err := e.buildStoreFromGroveDB(groveDB)
		if err != nil {
			return err
		}
		e.engineOpts = append(e.engineOpts, engine.WithStore(s))
	}

	opts := make([]engine.Option, 0, len(e.engineOpts)+1)
	opts = append(opts, e.engineOpts...)
	opts = append(opts, engine.WithLogger(slog.Default()))

	eng, err := engine.New(opts...)
	if err != nil {
		return fmt.Errorf("sentinel: create engine: %w", err)
	}
	e.eng = eng

	// Create the API handler.
	e.apiHandler = api.New(e.eng, fapp.Router())

	// Register HTTP routes unless disabled.
	if !e.config.DisableRoutes {
		e.apiHandler.RegisterRoutes(fapp.Router())
	}

	return nil
}

// Start begins the Sentinel engine and runs auto-migration if enabled.
func (e *Extension) Start(ctx context.Context) error {
	if e.eng == nil {
		return errors.New("sentinel: extension not initialized")
	}

	// Run migrations unless disabled.
	if !e.config.DisableMigrate {
		s := e.eng.Store()
		if s != nil {
			if err := s.Migrate(ctx); err != nil {
				return fmt.Errorf("sentinel: migration failed: %w", err)
			}
		}
	}

	if err := e.eng.Start(ctx); err != nil {
		return err
	}

	e.MarkStarted()
	return nil
}

// Stop gracefully shuts down the Sentinel engine.
func (e *Extension) Stop(ctx context.Context) error {
	if e.eng != nil {
		if err := e.eng.Stop(ctx); err != nil {
			return err
		}
	}
	e.MarkStopped()
	return nil
}

// Health implements [forge.Extension].
func (e *Extension) Health(ctx context.Context) error {
	if e.eng == nil {
		return errors.New("sentinel: extension not initialized")
	}

	s := e.eng.Store()
	if s == nil {
		return errors.New("sentinel: no store configured")
	}

	return s.Ping(ctx)
}

// Handler returns the HTTP handler for all API routes.
func (e *Extension) Handler() http.Handler {
	if e.apiHandler == nil {
		return http.NotFoundHandler()
	}
	return e.apiHandler.Handler()
}

// RegisterRoutes registers all Sentinel API routes into a Forge router.
func (e *Extension) RegisterRoutes(router forge.Router) {
	if e.apiHandler != nil {
		e.apiHandler.RegisterRoutes(router)
	}
}

// --- Config Loading (mirrors grove extension pattern) ---

// loadConfiguration loads config from YAML files or programmatic sources.
func (e *Extension) loadConfiguration() error {
	programmaticConfig := e.config

	// Try loading from config file.
	fileConfig, configLoaded := e.tryLoadFromConfigFile()

	if !configLoaded {
		if programmaticConfig.RequireConfig {
			return errors.New("sentinel: configuration is required but not found in config files; " +
				"ensure 'extensions.sentinel' or 'sentinel' key exists in your config")
		}

		// No config file found; merge programmatic config with defaults.
		e.config = e.mergeWithDefaults(programmaticConfig)
	} else {
		// Config loaded from YAML -- merge with programmatic options.
		e.config = e.mergeConfigurations(fileConfig, programmaticConfig)
	}

	// Enable grove resolution if YAML config specifies a grove database.
	if e.config.GroveDatabase != "" {
		e.useGrove = true
	}

	e.Logger().Debug("sentinel: configuration loaded",
		forge.F("disable_routes", e.config.DisableRoutes),
		forge.F("disable_migrate", e.config.DisableMigrate),
		forge.F("grove_database", e.config.GroveDatabase),
		forge.F("default_model", e.config.DefaultModel),
	)
	return nil
}

// tryLoadFromConfigFile attempts to load config from YAML files.
func (e *Extension) tryLoadFromConfigFile() (Config, bool) {
	cm := e.App().Config()
	var cfg Config

	// Try "extensions.sentinel" first (namespaced pattern).
	if cm.IsSet("extensions.sentinel") {
		if err := cm.Bind("extensions.sentinel", &cfg); err == nil {
			e.Logger().Debug("sentinel: loaded config from file",
				forge.F("key", "extensions.sentinel"),
			)
			return cfg, true
		}
		e.Logger().Warn("sentinel: failed to bind extensions.sentinel config",
			forge.F("error", "bind failed"),
		)
	}

	// Try legacy "sentinel" key.
	if cm.IsSet("sentinel") {
		if err := cm.Bind("sentinel", &cfg); err == nil {
			e.Logger().Debug("sentinel: loaded config from file",
				forge.F("key", "sentinel"),
			)
			return cfg, true
		}
		e.Logger().Warn("sentinel: failed to bind sentinel config",
			forge.F("error", "bind failed"),
		)
	}

	return Config{}, false
}

// mergeConfigurations merges YAML config with programmatic options.
// YAML config takes precedence; programmatic fills gaps.
func (e *Extension) mergeConfigurations(yamlConfig, programmaticConfig Config) Config {
	defaults := DefaultConfig()

	// Programmatic bool flags override YAML when true.
	if programmaticConfig.DisableRoutes {
		yamlConfig.DisableRoutes = true
	}
	if programmaticConfig.DisableMigrate {
		yamlConfig.DisableMigrate = true
	}
	if programmaticConfig.RequireConfig {
		yamlConfig.RequireConfig = true
	}

	// BasePath: YAML takes precedence.
	if yamlConfig.BasePath == "" && programmaticConfig.BasePath != "" {
		yamlConfig.BasePath = programmaticConfig.BasePath
	}
	if yamlConfig.GroveDatabase == "" && programmaticConfig.GroveDatabase != "" {
		yamlConfig.GroveDatabase = programmaticConfig.GroveDatabase
	}

	// DefaultModel: YAML takes precedence, then programmatic, then default.
	if yamlConfig.DefaultModel == "" {
		if programmaticConfig.DefaultModel != "" {
			yamlConfig.DefaultModel = programmaticConfig.DefaultModel
		} else {
			yamlConfig.DefaultModel = defaults.DefaultModel
		}
	}

	// PassThreshold: YAML takes precedence, then programmatic, then default.
	if yamlConfig.PassThreshold == 0 {
		if programmaticConfig.PassThreshold != 0 {
			yamlConfig.PassThreshold = programmaticConfig.PassThreshold
		} else {
			yamlConfig.PassThreshold = defaults.PassThreshold
		}
	}

	// Concurrency: YAML takes precedence, then programmatic, then default.
	if yamlConfig.Concurrency == 0 {
		if programmaticConfig.Concurrency != 0 {
			yamlConfig.Concurrency = programmaticConfig.Concurrency
		} else {
			yamlConfig.Concurrency = defaults.Concurrency
		}
	}

	// ShutdownTimeout: YAML takes precedence, then programmatic, then default.
	if yamlConfig.ShutdownTimeout == 0 {
		if programmaticConfig.ShutdownTimeout != 0 {
			yamlConfig.ShutdownTimeout = programmaticConfig.ShutdownTimeout
		} else {
			yamlConfig.ShutdownTimeout = defaults.ShutdownTimeout
		}
	}

	return yamlConfig
}

// mergeWithDefaults merges the programmatic config with defaults.
func (e *Extension) mergeWithDefaults(programmatic Config) Config {
	defaults := DefaultConfig()

	// Bool flags: programmatic true overrides default false.
	result := defaults
	if programmatic.DisableRoutes {
		result.DisableRoutes = true
	}
	if programmatic.DisableMigrate {
		result.DisableMigrate = true
	}
	if programmatic.RequireConfig {
		result.RequireConfig = true
	}

	// String/numeric fields: programmatic takes precedence over default.
	if programmatic.BasePath != "" {
		result.BasePath = programmatic.BasePath
	}
	if programmatic.DefaultModel != "" {
		result.DefaultModel = programmatic.DefaultModel
	}
	if programmatic.Temperature != 0 {
		result.Temperature = programmatic.Temperature
	}
	if programmatic.PassThreshold != 0 {
		result.PassThreshold = programmatic.PassThreshold
	}
	if programmatic.Concurrency != 0 {
		result.Concurrency = programmatic.Concurrency
	}
	if programmatic.ShutdownTimeout != 0 {
		result.ShutdownTimeout = programmatic.ShutdownTimeout
	}
	if programmatic.GroveDatabase != "" {
		result.GroveDatabase = programmatic.GroveDatabase
	}

	return result
}

// resolveGroveDB resolves a *grove.DB from the DI container.
// If GroveDatabase is set, it looks up the named DB; otherwise it uses the default.
func (e *Extension) resolveGroveDB(fapp forge.App) (*grove.DB, error) {
	if e.config.GroveDatabase != "" {
		db, err := vessel.InjectNamed[*grove.DB](fapp.Container(), e.config.GroveDatabase)
		if err != nil {
			return nil, fmt.Errorf("grove database %q not found in container: %w", e.config.GroveDatabase, err)
		}
		return db, nil
	}
	db, err := vessel.Inject[*grove.DB](fapp.Container())
	if err != nil {
		return nil, fmt.Errorf("default grove database not found in container: %w", err)
	}
	return db, nil
}

// buildStoreFromGroveDB constructs the appropriate store backend
// based on the grove driver type (pg, sqlite, mongo).
func (e *Extension) buildStoreFromGroveDB(db *grove.DB) (store.Store, error) {
	driverName := db.Driver().Name()
	switch driverName {
	case "pg":
		return pgstore.New(db), nil
	case "sqlite":
		return sqlitestore.New(db), nil
	case "mongo":
		return mongostore.New(db), nil
	default:
		return nil, fmt.Errorf("sentinel: unsupported grove driver %q", driverName)
	}
}
