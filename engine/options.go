// Package engine provides the central Sentinel evaluation coordinator.
package engine

import (
	"errors"
	"fmt"

	log "github.com/xraph/go-utils/log"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/plugin"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/store"
	"github.com/xraph/sentinel/target"
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
func WithLogger(l log.Logger) Option {
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

// WithTarget registers a named target, so a run can be started by name.
func WithTarget(name, description string, t target.Target) Option {
	return func(e *Engine) error {
		if name == "" || t == nil {
			return errors.New("sentinel: WithTarget needs a name and a target")
		}
		if e.targets == nil {
			e.targets = make(map[string]RegisteredTarget)
		}
		if _, dup := e.targets[name]; dup {
			return fmt.Errorf("sentinel: target %q registered twice", name)
		}
		e.targets[name] = RegisteredTarget{Name: name, Description: description, Target: t}
		return nil
	}
}

// WithScorer registers a scorer factory with a descriptor. This is how an
// application makes the LLM scorers available, since only it holds an
// LLMClient.
func WithScorer(d scorer.Descriptor, f scorer.Factory) Option {
	return func(e *Engine) error {
		if d.Name == "" || f == nil {
			return errors.New("sentinel: WithScorer needs a name and a factory")
		}
		e.pendingScorers = append(e.pendingScorers, pendingScorer{desc: d, factory: f})
		return nil
	}
}
