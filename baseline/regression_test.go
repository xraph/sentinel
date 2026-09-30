package baseline

import (
	"testing"

	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
)

func result(caseID id.CaseID, name string, score float64) *evalrun.Result {
	return &evalrun.Result{CaseID: caseID, CaseName: name, Score: score}
}

func TestDetectRegression(t *testing.T) {
	c1, c2, c3 := id.NewCaseID(), id.NewCaseID(), id.NewCaseID()
	base := &Baseline{
		PassRate: 0.5, AvgScore: 0.5,
		DimensionScores: map[string]float64{"skill": 0.5, "trait": 0.5},
		Results:         []Result{{CaseID: c1, CaseName: "one", Score: 0.5}, {CaseID: c2, CaseName: "two", Score: 0.5}},
	}

	t.Run("within threshold, on the boundary", func(t *testing.T) {
		stats := &evalrun.ResultStats{PassRate: 0.25, AvgScore: 0.25, DimensionScores: map[string]float64{"skill": 0.25, "trait": 0.25}}
		rr := DetectRegression(stats, []*evalrun.Result{result(c1, "one", 0.25), result(c2, "two", 0.25)}, base, 0.25)
		if rr.HasRegression {
			t.Fatalf("a drop exactly equal to the threshold is not a regression: %+v", rr)
		}
		if rr.Threshold != 0.25 {
			t.Fatalf("threshold not recorded: %v", rr.Threshold)
		}
	})

	t.Run("pass rate beyond threshold", func(t *testing.T) {
		stats := &evalrun.ResultStats{PassRate: 0.25, AvgScore: 0.5, DimensionScores: map[string]float64{"skill": 0.5, "trait": 0.5}}
		rr := DetectRegression(stats, []*evalrun.Result{result(c1, "one", 0.5), result(c2, "two", 0.5)}, base, 0.125)
		if !rr.HasRegression || rr.PassRateDelta != -0.25 {
			t.Fatalf("pass rate regression missed: %+v", rr)
		}
		if rr.WorstDelta() != -0.25 {
			t.Fatalf("worst delta: %v", rr.WorstDelta())
		}
	})

	t.Run("a missing dimension is not measured, and still regresses", func(t *testing.T) {
		stats := &evalrun.ResultStats{PassRate: 0.5, AvgScore: 0.5, DimensionScores: map[string]float64{"skill": 0.5}}
		rr := DetectRegression(stats, []*evalrun.Result{result(c1, "one", 0.5), result(c2, "two", 0.5)}, base, 0.125)
		if !rr.HasRegression {
			t.Fatal("a dimension that stopped being measured must still flag, for CI")
		}
		if len(rr.MissingDimensions) != 1 || rr.MissingDimensions[0] != "trait" {
			t.Fatalf("missing dimensions: %v", rr.MissingDimensions)
		}
		if _, ok := rr.DimensionDeltas["trait"]; ok {
			t.Fatal("a missing dimension must not appear as a delta of -0.5")
		}
	})

	t.Run("case regression, and missing and new cases", func(t *testing.T) {
		stats := &evalrun.ResultStats{PassRate: 0.5, AvgScore: 0.5, DimensionScores: map[string]float64{"skill": 0.5, "trait": 0.5}}
		rr := DetectRegression(stats, []*evalrun.Result{result(c1, "one", 0.125), result(c3, "three", 1)}, base, 0.125)
		if !rr.HasRegression || len(rr.RegressedCases) != 1 || rr.RegressedCases[0].CaseID != c1.String() {
			t.Fatalf("case one should regress: %+v", rr.RegressedCases)
		}
		if len(rr.MissingCases) != 1 || rr.MissingCases[0].CaseID != c2.String() || rr.MissingCases[0].CaseName != "two" {
			t.Fatalf("case two is missing from the run: %+v", rr.MissingCases)
		}
		if len(rr.NewCases) != 1 || rr.NewCases[0].CaseID != c3.String() {
			t.Fatalf("case three is new: %+v", rr.NewCases)
		}
	})

	t.Run("missing and new cases alone do not regress", func(t *testing.T) {
		stats := &evalrun.ResultStats{PassRate: 0.5, AvgScore: 0.5, DimensionScores: map[string]float64{"skill": 0.5, "trait": 0.5}}
		rr := DetectRegression(stats, []*evalrun.Result{result(c1, "one", 0.5), result(c3, "three", 0.5)}, base, 0.125)
		if rr.HasRegression {
			t.Fatalf("a changed case set is reported, not counted as a regression: %+v", rr)
		}
	})

	t.Run("missing dimension with baseline score 0.5 gives worst delta of -0.5", func(t *testing.T) {
		stats := &evalrun.ResultStats{PassRate: 0.5, AvgScore: 0.5, DimensionScores: map[string]float64{"skill": 0.5}}
		rr := DetectRegression(stats, []*evalrun.Result{result(c1, "one", 0.5), result(c2, "two", 0.5)}, base, 0.125)
		if !rr.HasRegression {
			t.Fatal("missing dimension must set HasRegression")
		}
		if rr.WorstDelta() != -0.5 {
			t.Fatalf("worst delta for missing dimension with baseline 0.5: %v, expected -0.5", rr.WorstDelta())
		}
	})
}

// One newly failing case in a 20-case suite drops the pass rate by exactly
// the default threshold, but 19.0/20 - 1.0 is -0.050000000000000044 in
// float64. A drop equal to the threshold must not flip on that rounding.
func TestDropEqualToThresholdIgnoresFloatRounding(t *testing.T) {
	c1 := id.NewCaseID()
	base := &Baseline{
		PassRate: 1.0, AvgScore: 1.0,
		DimensionScores: map[string]float64{"skill": 1.0},
		Results:         []Result{{CaseID: c1, CaseName: "one", Score: 1.0}},
	}
	drop := 19.0 / 20
	if drop-1.0 >= -0.05 {
		t.Fatalf("precondition: 19/20 - 1 should round below -0.05, got %v", drop-1.0)
	}

	cases := []struct {
		name    string
		stats   *evalrun.ResultStats
		results []*evalrun.Result
	}{
		{"pass rate", &evalrun.ResultStats{PassRate: drop, AvgScore: 1.0, DimensionScores: map[string]float64{"skill": 1.0}},
			[]*evalrun.Result{result(c1, "one", 1.0)}},
		{"average score", &evalrun.ResultStats{PassRate: 1.0, AvgScore: drop, DimensionScores: map[string]float64{"skill": 1.0}},
			[]*evalrun.Result{result(c1, "one", 1.0)}},
		{"dimension", &evalrun.ResultStats{PassRate: 1.0, AvgScore: 1.0, DimensionScores: map[string]float64{"skill": drop}},
			[]*evalrun.Result{result(c1, "one", 1.0)}},
		{"case", &evalrun.ResultStats{PassRate: 1.0, AvgScore: 1.0, DimensionScores: map[string]float64{"skill": 1.0}},
			[]*evalrun.Result{result(c1, "one", drop)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := DetectRegression(c.stats, c.results, base, 0.05)
			if rr.HasRegression {
				t.Fatalf("a drop equal to the threshold is not a regression: %+v", rr)
			}
		})
	}

	t.Run("a drop just past the threshold still regresses", func(t *testing.T) {
		stats := &evalrun.ResultStats{PassRate: 0.94, AvgScore: 1.0, DimensionScores: map[string]float64{"skill": 1.0}}
		if rr := DetectRegression(stats, []*evalrun.Result{result(c1, "one", 1.0)}, base, 0.05); !rr.HasRegression {
			t.Fatalf("a 0.06 drop against 0.05 must regress: %+v", rr)
		}
	})
}
