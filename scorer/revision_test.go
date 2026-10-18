package scorer

import (
	"context"
	"testing"
)

func TestPinnedScorerRejectsChangedImplementation(t *testing.T) {
	r := NewRegistry()
	revision, err := r.Revision("exact")
	if err != nil || revision == "" {
		t.Fatalf("builtin revision: %q %v", revision, err)
	}
	pinned, err := r.GetVersioned("exact", revision, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.RegisterDescribed(Descriptor{Name: "exact", Revision: "custom-v2"}, func(map[string]any) (Scorer, error) { return &ContainsScorer{Substring: "different"}, nil })
	if _, err = r.GetVersioned("exact", revision, nil); err == nil {
		t.Fatal("accepted replacement implementation under an old revision")
	}
	result, err := pinned.Score(context.Background(), &Input{Expected: "answer", Actual: "answer"})
	if err != nil || result.Score != 1 {
		t.Fatalf("already captured scorer changed: %+v %v", result, err)
	}
}

func TestUnversionedCustomScorerCannotFreeze(t *testing.T) {
	r := NewRegistry()
	r.Register("custom", func(map[string]any) (Scorer, error) { return &ExactScorer{}, nil })
	if _, err := r.Revision("custom"); err == nil {
		t.Fatal("unversioned custom scorer treated as reproducible")
	}
	if _, err := r.Get("custom", nil); err != nil {
		t.Fatalf("legacy scorer registration changed: %v", err)
	}
}
