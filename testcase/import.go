package testcase

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
)

// ImportJSON parses test cases from a JSON array.
func ImportJSON(suiteID id.SuiteID, data []byte) ([]*Case, error) {
	var raw []struct {
		Name     string         `json:"name"`
		Input    string         `json:"input"`
		Expected string         `json:"expected,omitempty"`
		Tags     []string       `json:"tags,omitempty"`
		Context  map[string]any `json:"context,omitempty"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("testcase: import json: %w", err)
	}

	cases := make([]*Case, len(raw))
	for i, r := range raw {
		cases[i] = &Case{
			Entity:       sentinel.NewEntity(),
			ID:           id.NewCaseID(),
			SuiteID:      suiteID,
			Name:         r.Name,
			Input:        r.Input,
			Expected:     r.Expected,
			ScenarioType: ScenarioStandard,
			Tags:         r.Tags,
			Context:      r.Context,
		}
	}

	return cases, nil
}

// ImportCSV parses test cases from CSV data. Expected columns: name, input, expected.
func ImportCSV(suiteID id.SuiteID, data []byte) ([]*Case, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("testcase: import csv header: %w", err)
	}

	colMap := make(map[string]int)
	for i, col := range header {
		colMap[strings.TrimSpace(strings.ToLower(col))] = i
	}

	var cases []*Case
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("testcase: import csv row: %w", err)
		}

		tc := &Case{
			Entity:       sentinel.NewEntity(),
			ID:           id.NewCaseID(),
			SuiteID:      suiteID,
			ScenarioType: ScenarioStandard,
		}
		if idx, ok := colMap["name"]; ok && idx < len(record) {
			tc.Name = record[idx]
		}
		if idx, ok := colMap["input"]; ok && idx < len(record) {
			tc.Input = record[idx]
		}
		if idx, ok := colMap["expected"]; ok && idx < len(record) {
			tc.Expected = record[idx]
		}
		if idx, ok := colMap["tags"]; ok && idx < len(record) && record[idx] != "" {
			tc.Tags = strings.Split(record[idx], ";")
		}

		cases = append(cases, tc)
	}

	return cases, nil
}

// ImportJSONL parses test cases from JSON Lines format.
func ImportJSONL(suiteID id.SuiteID, data []byte) ([]*Case, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	cases := make([]*Case, 0, len(lines))

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var raw struct {
			Name     string         `json:"name"`
			Input    string         `json:"input"`
			Expected string         `json:"expected,omitempty"`
			Tags     []string       `json:"tags,omitempty"`
			Context  map[string]any `json:"context,omitempty"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, fmt.Errorf("testcase: import jsonl line %d: %w", i+1, err)
		}

		tc := &Case{
			Entity:       sentinel.NewEntity(),
			ID:           id.NewCaseID(),
			SuiteID:      suiteID,
			Name:         raw.Name,
			Input:        raw.Input,
			Expected:     raw.Expected,
			ScenarioType: ScenarioStandard,
			Tags:         raw.Tags,
			Context:      raw.Context,
		}
		cases = append(cases, tc)
	}

	return cases, nil
}
