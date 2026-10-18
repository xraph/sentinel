package engine

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/scorer"
)

type capturedTestScorer struct {
	output *scorer.Output
	err    error
}

func (s capturedTestScorer) Name() string { return "test" }
func (s capturedTestScorer) Score(context.Context, *scorer.Input) (*scorer.Output, error) {
	return s.output, s.err
}

func TestCapturedRequiredFailureCannotBeAveragedAway(t *testing.T) {
	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	r := new(evalrun.Result)
	criteria := []capturedCriterion{
		{ID: "quality", Scorer: capturedTestScorer{output: &scorer.Output{Score: 1, Passed: true}}},
		{ID: "protected", Required: true, Scorer: capturedTestScorer{output: &scorer.Output{Score: .9, Passed: false}}},
	}
	if err = e.scoreCapturedCase(context.Background(), r, &scorer.Input{}, criteria, .7); err != nil {
		t.Fatal(err)
	}
	if r.Status != evalrun.StatusFail || r.Score != .95 {
		t.Fatalf("required failure averaged away: %+v", r)
	}
	if r.ScorerResults[1].CriterionID != "protected" || !r.ScorerResults[1].Required {
		t.Fatalf("criterion identity lost: %+v", r.ScorerResults)
	}
}

func TestCapturedScorerFailuresAreErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		s    capturedTestScorer
	}{
		{"error", capturedTestScorer{err: errors.New("judge unavailable")}},
		{"nil", capturedTestScorer{}},
		{"nan", capturedTestScorer{output: &scorer.Output{Score: math.NaN(), Passed: true}}},
		{"above range", capturedTestScorer{output: &scorer.Output{Score: 1.1, Passed: true}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e, err := New()
			if err != nil {
				t.Fatal(err)
			}
			r := new(evalrun.Result)
			err = e.scoreCapturedCase(context.Background(), r, &scorer.Input{}, []capturedCriterion{{ID: "quality", Required: true, Scorer: tt.s}}, .7)
			if err == nil || r.Status != evalrun.StatusError || len(r.ScorerResults) != 1 || !r.ScorerResults[0].Errored() {
				t.Fatalf("invalid scorer result accepted: %+v %v", r, err)
			}
		})
	}
}
