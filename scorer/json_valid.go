package scorer

import (
	"context"
	"encoding/json"
)

// JSONValidScorer scores 1.0 if the actual output is valid JSON.
type JSONValidScorer struct{}

func (s *JSONValidScorer) Name() string { return "json_valid" }

func (s *JSONValidScorer) Score(_ context.Context, input *Input) (*Output, error) {
	valid := json.Valid([]byte(input.Actual))
	score := 0.0
	if valid {
		score = 1.0
	}
	return &Output{
		Score:  score,
		Passed: valid,
		Reason: reasonFromMatch("json_valid", valid),
	}, nil
}
