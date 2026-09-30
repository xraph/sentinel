package evalrun

import (
	"encoding/json"
	"testing"
)

// bsonA stands in for mongo's bson.A, which is a named []any. A plain type
// assertion to []any fails on it.
type bsonA []any

func TestSettingsRoundTrip(t *testing.T) {
	pt, rt, c := 0.7, 0.05, 4
	in := Settings{PassThreshold: &pt, RegressionThreshold: &rt, Concurrency: &c, Target: "llm:x", Scorers: []string{"exact", "contains"}, Model: "m", PromptVersionID: "pv_1"}
	got := SettingsFrom(in.Config())
	if *got.PassThreshold != 0.7 || *got.RegressionThreshold != 0.05 || *got.Concurrency != 4 {
		t.Fatalf("numbers: %+v", got)
	}
	if got.Target != "llm:x" || got.Model != "m" || got.PromptVersionID != "pv_1" || len(got.Scorers) != 2 || got.Scorers[1] != "contains" {
		t.Fatalf("strings: %+v", got)
	}
}

func TestSettingsSurviveEachBackendsDecoding(t *testing.T) {
	var fromJSON map[string]any // sqlite and postgres: numbers come back float64, arrays []any
	_ = json.Unmarshal([]byte(`{"pass_threshold":0.7,"concurrency":4,"scorers":["exact"]}`), &fromJSON)
	fromMongo := map[string]any{"pass_threshold": 0.7, "concurrency": int32(4), "scorers": bsonA{"exact"}}

	for name, cfg := range map[string]map[string]any{"json": fromJSON, "mongo": fromMongo} {
		s := SettingsFrom(cfg)
		if s.PassThreshold == nil || *s.PassThreshold != 0.7 || s.Concurrency == nil || *s.Concurrency != 4 || len(s.Scorers) != 1 {
			t.Errorf("%s: %+v", name, s)
		}
	}
}

// A run written before settings were recorded has none of the keys. That
// must read as "not recorded", never as zero.
func TestMissingSettingsAreNil(t *testing.T) {
	s := SettingsFrom(nil)
	if s.PassThreshold != nil || s.RegressionThreshold != nil || s.Concurrency != nil || s.Scorers != nil || s.Target != "" {
		t.Fatalf("expected nothing recorded: %+v", s)
	}
}

func TestApplyStats(t *testing.T) {
	r := &Run{TotalCases: 10}
	r.ApplyStats(&ResultStats{TotalCases: 4, Passed: 3, Failed: 1, PassRate: 0.75, AvgScore: 0.8, AvgLatencyMs: 12, TotalTokens: 99, TotalCost: 0.5, DimensionScores: map[string]float64{"skill": 1}})
	if r.TotalCases != 10 {
		t.Fatal("ApplyStats must not overwrite the planned case count")
	}
	if r.Passed != 3 || r.Failed != 1 || r.PassRate != 0.75 || r.AvgScore != 0.8 || r.AvgLatencyMs != 12 || r.TotalTokens != 99 || r.TotalCost != 0.5 || r.DimensionScores["skill"] != 1 {
		t.Fatalf("stats not applied: %+v", r)
	}
}
