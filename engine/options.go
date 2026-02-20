// Package engine provides the central Sentinel evaluation coordinator.
package engine

import (
	"log/slog"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/plugin"
	"github.com/xraph/sentinel/store"
)

// Option configures the Engine.
type Option func(*Engine) error

// WithStore sets the evaluation store.
func WithStore(s store.Store) Option {
	return func(e *Engine) error {
		e.store = s
		return nil
	}
}

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) Option {
	return func(e *Engine) error {
		e.logger = l
		return nil
	}
}

// WithExtension registers an extension with the engine.
func WithExtension(extension plugin.Extension) Option {
	return func(e *Engine) error {
		e.pendingExts = append(e.pendingExts, extension)
		return nil
	}
}

// WithConfig sets the engine configuration.
func WithConfig(cfg sentinel.Config) Option {
	return func(e *Engine) error {
		e.config = cfg
		return nil
	}
}
