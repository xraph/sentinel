package scorer

import (
	"context"
	"fmt"
	"regexp"
)

// RegexScorer scores 1.0 if the actual output matches the given regex pattern.
type RegexScorer struct {
	Pattern string
	re      *regexp.Regexp
}

// NewRegexScorer creates a RegexScorer with a pre-compiled pattern.
func NewRegexScorer(pattern string) (*RegexScorer, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("scorer: invalid regex %q: %w", pattern, err)
	}
	return &RegexScorer{Pattern: pattern, re: re}, nil
}

func (s *RegexScorer) Name() string { return "regex" }

func (s *RegexScorer) Score(_ context.Context, input *ScorerInput) (*ScorerOutput, error) {
	match := s.re.MatchString(input.Actual)
	score := 0.0
	if match {
		score = 1.0
	}
	return &ScorerOutput{
		Score:  score,
		Passed: match,
		Reason: reasonFromMatch("regex", match),
	}, nil
}
