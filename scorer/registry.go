package scorer

import (
	"fmt"
	"sort"
	"sync"
)

// Factory creates a scorer from a configuration map.
type Factory func(config map[string]any) (Scorer, error)

// Descriptor says what a registered scorer is, for anyone listing them.
type Descriptor struct {
	Name        string
	Description string
	Dimension   string // the human-like dimension it reports, empty for none
	UsesLLM     bool   // true when scoring makes a model call, which costs money
}

// Registry holds named scorer factories and creates scorer instances from config.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
	descs     map[string]Descriptor
}

// NewRegistry creates a scorer registry with built-in scorers pre-registered.
func NewRegistry() *Registry {
	r := &Registry{
		factories: make(map[string]Factory),
		descs:     make(map[string]Descriptor),
	}
	r.registerBuiltins()
	r.describeBuiltins()
	return r
}

// Register adds a scorer factory by name, with no description.
func (r *Registry) Register(name string, factory Factory) {
	r.RegisterDescribed(Descriptor{Name: name}, factory)
}

// RegisterDescribed adds a scorer factory with a descriptor.
func (r *Registry) RegisterDescribed(d Descriptor, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[d.Name] = factory
	r.descs[d.Name] = d
}

// Has reports whether a scorer is registered under name.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[name]
	return ok
}

// Descriptors lists every registered scorer, sorted by name.
func (r *Registry) Descriptors() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Descriptor, 0, len(r.descs))
	for _, d := range r.descs {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
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
}

// describeBuiltins records what each built-in does. The text is what an
// operator reads when picking scorers for a run.
func (r *Registry) describeBuiltins() {
	for name, desc := range map[string]string{
		"exact":        "Passes when the output equals the expected value.",
		"contains":     "Passes when the output contains a substring (config: substring, case_insensitive).",
		"not_contains": "Passes when the output does not contain a substring (config: substring, case_insensitive).",
		"regex":        "Passes when the output matches a regular expression (config: pattern).",
		"json_valid":   "Passes when the output is valid JSON.",
		"json_schema":  "Passes when the output is valid JSON. The schema option is not enforced yet.",
		"length":       "Passes when the output's word count is within min and max.",
		"latency":      "Passes when the target answered within max_ms milliseconds.",
		"cost":         "Passes when the target reported a cost at or below max_cost.",
	} {
		r.descs[name] = Descriptor{Name: name, Description: desc}
	}
}
