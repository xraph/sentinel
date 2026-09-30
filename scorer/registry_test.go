package scorer

import (
	"context"
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
