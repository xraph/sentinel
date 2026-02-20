// Package dataset provides utilities for loading, generating, and sampling
// test case data from various formats.
package dataset

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/xraph/sentinel/testcase"
)

// CaseData is the serialisation format for dataset entries.
type CaseData struct {
	Name     string         `json:"name"`
	Input    string         `json:"input"`
	Expected string         `json:"expected,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Context  map[string]any `json:"context,omitempty"`
}

// LoadJSON loads test case data from a JSON array.
func LoadJSON(data []byte) ([]CaseData, error) {
	var cases []CaseData
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("dataset: load json: %w", err)
	}
	return cases, nil
}

// LoadCSV loads test case data from CSV. Expected columns: name, input, expected.
func LoadCSV(data []byte) ([]CaseData, error) {
	r := csv.NewReader(strings.NewReader(string(data)))

	// Read header.
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("dataset: read csv header: %w", err)
	}

	colMap := make(map[string]int)
	for i, col := range header {
		colMap[strings.TrimSpace(strings.ToLower(col))] = i
	}

	var cases []CaseData
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("dataset: read csv row: %w", err)
		}

		cd := CaseData{}
		if idx, ok := colMap["name"]; ok && idx < len(record) {
			cd.Name = record[idx]
		}
		if idx, ok := colMap["input"]; ok && idx < len(record) {
			cd.Input = record[idx]
		}
		if idx, ok := colMap["expected"]; ok && idx < len(record) {
			cd.Expected = record[idx]
		}
		if idx, ok := colMap["tags"]; ok && idx < len(record) && record[idx] != "" {
			cd.Tags = strings.Split(record[idx], ";")
		}

		cases = append(cases, cd)
	}

	return cases, nil
}

// LoadJSONL loads test case data from JSON Lines format.
func LoadJSONL(data []byte) ([]CaseData, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	cases := make([]CaseData, 0, len(lines))

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var cd CaseData
		if err := json.Unmarshal([]byte(line), &cd); err != nil {
			return nil, fmt.Errorf("dataset: parse jsonl line %d: %w", i+1, err)
		}
		cases = append(cases, cd)
	}

	return cases, nil
}

// ToCases converts dataset entries to test cases for a given suite.
func ToCases(data []CaseData, converter func(CaseData) *testcase.Case) []*testcase.Case {
	cases := make([]*testcase.Case, len(data))
	for i, d := range data {
		cases[i] = converter(d)
	}
	return cases
}
