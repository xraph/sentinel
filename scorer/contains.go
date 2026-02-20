package scorer

import (
	"context"
	"strings"
)

// ContainsScorer scores 1.0 if the actual output contains the expected substring.
type ContainsScorer struct {
	Substring       string
	CaseInsensitive bool
}

func (s *ContainsScorer) Name() string { return "contains" }

func (s *ContainsScorer) Score(_ context.Context, input *ScorerInput) (*ScorerOutput, error) {
	actual := input.Actual
	sub := s.Substring
	if sub == "" {
		sub = input.Expected
	}
	if s.CaseInsensitive {
		actual = strings.ToLower(actual)
		sub = strings.ToLower(sub)
	}

	match := strings.Contains(actual, sub)
	score := 0.0
	if match {
		score = 1.0
	}
	return &ScorerOutput{
		Score:  score,
		Passed: match,
		Reason: reasonFromMatch("contains", match),
	}, nil
}

// NotContainsScorer scores 1.0 if the actual output does NOT contain the substring.
type NotContainsScorer struct {
	Substring       string
	CaseInsensitive bool
}

func (s *NotContainsScorer) Name() string { return "not_contains" }

func (s *NotContainsScorer) Score(_ context.Context, input *ScorerInput) (*ScorerOutput, error) {
	actual := input.Actual
	sub := s.Substring
	if sub == "" {
		sub = input.Expected
	}
	if s.CaseInsensitive {
		actual = strings.ToLower(actual)
		sub = strings.ToLower(sub)
	}

	absent := !strings.Contains(actual, sub)
	score := 0.0
	if absent {
		score = 1.0
	}
	return &ScorerOutput{
		Score:  score,
		Passed: absent,
		Reason: reasonFromMatch("not_contains", absent),
	}, nil
}
