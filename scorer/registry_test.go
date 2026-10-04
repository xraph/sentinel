package scorer

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuiltinsAreDescribed(t *testing.T) {
	r := NewRegistry()
	ds := r.Descriptors()
	want := []string{"contains", "cost", "exact", "json_schema", "json_valid", "latency", "length", "not_contains", "regex"}
	if len(ds) != len(want) {
		t.Fatalf("got %d descriptors, want %d: %+v", len(ds), len(want), ds)
	}
	for i, d := range ds {
		if d.Name != want[i] {
			t.Errorf("descriptor %d is %q, want %q (sorted by name)", i, d.Name, want[i])
		}
		if d.Description == "" {
			t.Errorf("%s has no description", d.Name)
		}
		if d.UsesLLM {
			t.Errorf("%s is deterministic and must not claim to call an LLM", d.Name)
		}
	}
}

// custom always returned an error from the registry: it can only be built
// with FromFunc. Listing it would offer a scorer that fails every case.
func TestCustomIsNotOffered(t *testing.T) {
	if NewRegistry().Has("custom") {
		t.Fatal("custom must not be registered")
	}
}

func TestRegisterDescribed(t *testing.T) {
	r := NewRegistry()
	r.RegisterDescribed(Descriptor{Name: "judge", Description: "LLM judge", Dimension: "persona", UsesLLM: true},
		func(map[string]any) (Scorer, error) {
			return FromFunc("judge", func(context.Context, *Input) (*Output, error) { return &Output{Score: 1}, nil }), nil
		})
	if !r.Has("judge") {
		t.Fatal("judge not registered")
	}
	s, err := r.Get("judge", nil)
	if err != nil || s.Name() != "judge" {
		t.Fatalf("get judge: %v", err)
	}
	var found bool
	for _, d := range r.Descriptors() {
		if d.Name == "judge" && d.UsesLLM && d.Dimension == "persona" {
			found = true
		}
	}
	if !found {
		t.Fatal("judge descriptor missing")
	}
	if _, err := r.Get("nope", nil); err == nil {
		t.Fatal("unknown scorer should error")
	}
}

// Without their config these four pass everything: an empty pattern
// matches any output and absent bounds bound nothing. They must refuse.
func TestScorersThatNeedConfigRefuseWithoutIt(t *testing.T) {
	r := NewRegistry()
	cases := []struct {
		name    string
		missing string
		valid   map[string]any
	}{
		{"regex", "pattern", map[string]any{"pattern": "^ok"}},
		{"length", "min or max", map[string]any{"max": 50.0}},
		{"latency", "max_ms", map[string]any{"max_ms": 500.0}},
		{"cost", "max_cost", map[string]any{"max_cost": 0.01}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, cfg := range []map[string]any{nil, {}} {
				_, err := r.Get(c.name, cfg)
				if err == nil {
					t.Fatalf("%s built with config %v must refuse", c.name, cfg)
				}
				if !strings.Contains(err.Error(), c.name) || !strings.Contains(err.Error(), c.missing) {
					t.Fatalf("the error must name the scorer and %q: %v", c.missing, err)
				}
			}
			s, err := r.Get(c.name, c.valid)
			if err != nil || s.Name() != c.name {
				t.Fatalf("%s with %v: %v", c.name, c.valid, err)
			}
		})
	}
	if _, err := r.Get("length", map[string]any{"min": 3.0}); err != nil {
		t.Fatalf("length with only min is configured: %v", err)
	}
	if _, err := r.Get("regex", map[string]any{"pattern": ""}); err == nil {
		t.Fatal("an empty regex pattern matches everything and must refuse")
	}
}

func TestRequiresConfigMarksExactlyTheFour(t *testing.T) {
	want := map[string]bool{"regex": true, "length": true, "latency": true, "cost": true}
	for _, d := range NewRegistry().Descriptors() {
		if d.RequiresConfig != want[d.Name] {
			t.Errorf("%s: RequiresConfig = %v, want %v", d.Name, d.RequiresConfig, want[d.Name])
		}
	}
}

// Mongo decodes small integers as int32, so a latency config written from
// Go as max_ms: 500 comes back as int32(500). It is still a valid config.
func TestNumberConfigAcceptsEveryBackendsNumbers(t *testing.T) {
	r := NewRegistry()
	for name, v := range map[string]any{
		"float64": float64(500), "float32": float32(500), "int": 500,
		"int32": int32(500), "int64": int64(500), "json.Number": json.Number("500"),
	} {
		if _, err := r.Get("latency", map[string]any{"max_ms": v}); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
