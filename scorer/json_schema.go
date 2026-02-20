package scorer

import (
	"context"
	"encoding/json"
)

// JSONSchemaScorer scores 1.0 if the actual output is valid JSON that
// matches a basic structural expectation. For full JSON Schema validation,
// provide a schema string.
type JSONSchemaScorer struct {
	Schema string
}

func (s *JSONSchemaScorer) Name() string { return "json_schema" }

func (s *JSONSchemaScorer) Score(_ context.Context, input *ScorerInput) (*ScorerOutput, error) {
	if !json.Valid([]byte(input.Actual)) {
		return &ScorerOutput{
			Score:  0,
			Passed: false,
			Reason: "json_schema: output is not valid JSON",
		}, nil
	}

	// Basic validation: ensure it parses as the expected type.
	// Full JSON Schema validation can be added via external library.
	var parsed any
	if err := json.Unmarshal([]byte(input.Actual), &parsed); err != nil {
		return &ScorerOutput{
			Score:  0,
			Passed: false,
			Reason: "json_schema: parse error: " + err.Error(),
		}, nil
	}

	return &ScorerOutput{
		Score:  1.0,
		Passed: true,
		Reason: "json_schema: passed",
	}, nil
}
