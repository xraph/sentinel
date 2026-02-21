package scorer

import (
	"fmt"
	"sync"
)

// Factory creates a scorer from a configuration map.
type Factory func(config map[string]any) (Scorer, error)

// Registry holds named scorer factories and creates scorer instances from config.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates a scorer registry with built-in scorers pre-registered.
func NewRegistry() *Registry {
	r := &Registry{
		factories: make(map[string]Factory),
	}
	r.registerBuiltins()
	return r
}

// Register adds a scorer factory by name.
func (r *Registry) Register(name string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Get creates a scorer instance from a name and config.
func (r *Registry) Get(name string, config map[string]any) (Scorer, error) {
	r.mu.RLock()
	factory, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("scorer: unknown scorer %q", name)
	}
	return factory(config)
}

func (r *Registry) registerBuiltins() {
	r.factories["exact"] = func(config map[string]any) (Scorer, error) {
		s := &ExactScorer{}
		if v, ok := config["case_insensitive"].(bool); ok {
			s.CaseInsensitive = v
		}
		return s, nil
	}
	r.factories["contains"] = func(config map[string]any) (Scorer, error) {
		s := &ContainsScorer{}
		if v, ok := config["substring"].(string); ok {
			s.Substring = v
		}
		if v, ok := config["case_insensitive"].(bool); ok {
			s.CaseInsensitive = v
		}
		return s, nil
	}
	r.factories["not_contains"] = func(config map[string]any) (Scorer, error) {
		s := &NotContainsScorer{}
		if v, ok := config["substring"].(string); ok {
			s.Substring = v
		}
		if v, ok := config["case_insensitive"].(bool); ok {
			s.CaseInsensitive = v
		}
		return s, nil
	}
	r.factories["regex"] = func(config map[string]any) (Scorer, error) {
		pattern, _ := config["pattern"].(string) //nolint:errcheck // type assertion returns zero-value
		return NewRegexScorer(pattern)
	}
	r.factories["json_valid"] = func(_ map[string]any) (Scorer, error) {
		return &JSONValidScorer{}, nil
	}
	r.factories["json_schema"] = func(config map[string]any) (Scorer, error) {
		schema, _ := config["schema"].(string) //nolint:errcheck // type assertion returns zero-value
		return &JSONSchemaScorer{Schema: schema}, nil
	}
	r.factories["length"] = func(config map[string]any) (Scorer, error) {
		s := &LengthScorer{}
		if v, ok := config["min"].(float64); ok {
			s.MinTokens = int(v)
		}
		if v, ok := config["max"].(float64); ok {
			s.MaxTokens = int(v)
		}
		return s, nil
	}
	r.factories["latency"] = func(config map[string]any) (Scorer, error) {
		s := &LatencyScorer{}
		if v, ok := config["max_ms"].(float64); ok {
			s.MaxMs = int(v)
		}
		return s, nil
	}
	r.factories["cost"] = func(config map[string]any) (Scorer, error) {
		s := &CostScorer{}
		if v, ok := config["max_cost"].(float64); ok {
			s.MaxCost = v
		}
		return s, nil
	}
	r.factories["custom"] = func(_ map[string]any) (Scorer, error) {
		return nil, fmt.Errorf("scorer: custom scorers must be registered with FromFunc")
	}
}
