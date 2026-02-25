package extension

import "time"

// Config holds the Sentinel extension configuration.
// Fields can be set programmatically via ExtOption functions or loaded from
// YAML configuration files (under "extensions.sentinel" or "sentinel" keys).
type Config struct {
	// DisableRoutes prevents HTTP route registration.
	DisableRoutes bool `json:"disable_routes" mapstructure:"disable_routes" yaml:"disable_routes"`

	// DisableMigrate prevents auto-migration on start.
	DisableMigrate bool `json:"disable_migrate" mapstructure:"disable_migrate" yaml:"disable_migrate"`

	// BasePath is the URL prefix for all sentinel routes.
	BasePath string `json:"base_path" mapstructure:"base_path" yaml:"base_path"`

	// DefaultModel is the LLM model to use when not specified per-suite.
	DefaultModel string `json:"default_model" mapstructure:"default_model" yaml:"default_model"`

	// Temperature is the default sampling temperature.
	Temperature float64 `json:"temperature" mapstructure:"temperature" yaml:"temperature"`

	// PassThreshold is the minimum score to consider a test case passed.
	PassThreshold float64 `json:"pass_threshold" mapstructure:"pass_threshold" yaml:"pass_threshold"`

	// Concurrency is the number of parallel evaluation workers.
	Concurrency int `json:"concurrency" mapstructure:"concurrency" yaml:"concurrency"`

	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	ShutdownTimeout time.Duration `json:"shutdown_timeout" mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`

	// GroveDatabase is the name of a grove.DB registered in the DI container.
	// When set, the extension resolves this named database and auto-constructs
	// the appropriate store based on the driver type (pg/sqlite/mongo).
	// When empty and WithGroveDatabase was called, the default (unnamed) DB is used.
	GroveDatabase string `json:"grove_database" mapstructure:"grove_database" yaml:"grove_database"`

	// RequireConfig requires config to be present in YAML files.
	// If true and no config is found, Register returns an error.
	RequireConfig bool `json:"-" yaml:"-"`
}

// DefaultConfig returns the default configuration for the Sentinel extension.
func DefaultConfig() Config {
	return Config{
		DefaultModel:    "smart",
		Temperature:     0,
		PassThreshold:   0.7,
		Concurrency:     4,
		ShutdownTimeout: 30 * time.Second,
	}
}
