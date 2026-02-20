package scorer

import (
	"context"
	"strings"
)

// ExactScorer scores 1.0 if the actual output exactly matches the expected output.
type ExactScorer struct {
	CaseInsensitive bool
}

func (s *ExactScorer) Name() string { return "exact" }

func (s *ExactScorer) Score(_ context.Context, input *ScorerInput) (*ScorerOutput, error) {
	actual := input.Actual
	expected := input.Expected
	if s.CaseInsensitive {
		actual = strings.ToLower(actual)
		expected = strings.ToLower(expected)
	}

	match := actual == expected
	score := 0.0
	if match {
		score = 1.0
	}
	return &ScorerOutput{
		Score:  score,
		Passed: match,
		Reason: reasonFromMatch("exact match", match),
	}, nil
}

func reasonFromMatch(name string, match bool) string {
	if match {
		return name + ": passed"
	}
	return name + ": failed"
}
