// Package id provides TypeID-based identity types for all Sentinel entities.
//
// Every entity in Sentinel gets a type-prefixed, K-sortable, UUIDv7-based
// identifier. IDs are validated at parse time to ensure the prefix matches
// the expected type.
//
// Examples:
//
//	suite_01h2xcejqtf2nbrexx3vqjhp41
//	tcase_01h2xcejqtf2nbrexx3vqjhp41
//	erun_01h455vb4pex5vsknk084sn02q
package id

import (
	"fmt"

	"go.jetify.com/typeid/v2"
)

// ──────────────────────────────────────────────────
// Prefix constants
// ──────────────────────────────────────────────────

const (
	// PrefixSuite is the TypeID prefix for evaluation suites.
	PrefixSuite = "suite"

	// PrefixCase is the TypeID prefix for test cases.
	PrefixCase = "tcase"

	// PrefixEvalRun is the TypeID prefix for evaluation runs.
	PrefixEvalRun = "erun"

	// PrefixEvalResult is the TypeID prefix for evaluation results.
	PrefixEvalResult = "eres"

	// PrefixBaseline is the TypeID prefix for baselines.
	PrefixBaseline = "base"

	// PrefixRedTeam is the TypeID prefix for red team sessions.
	PrefixRedTeam = "rteam"

	// PrefixPromptVersion is the TypeID prefix for prompt versions.
	PrefixPromptVersion = "pver"
)

// ──────────────────────────────────────────────────
// Type aliases for readability
// ──────────────────────────────────────────────────

// SuiteID is a type-safe identifier for evaluation suites (prefix: "suite").
type SuiteID = typeid.TypeID

// CaseID is a type-safe identifier for test cases (prefix: "tcase").
type CaseID = typeid.TypeID

// EvalRunID is a type-safe identifier for evaluation runs (prefix: "erun").
type EvalRunID = typeid.TypeID

// EvalResultID is a type-safe identifier for evaluation results (prefix: "eres").
type EvalResultID = typeid.TypeID

// BaselineID is a type-safe identifier for baselines (prefix: "base").
type BaselineID = typeid.TypeID

// RedTeamID is a type-safe identifier for red team sessions (prefix: "rteam").
type RedTeamID = typeid.TypeID

// PromptVersionID is a type-safe identifier for prompt versions (prefix: "pver").
type PromptVersionID = typeid.TypeID

// AnyID is a TypeID that accepts any valid prefix.
type AnyID = typeid.TypeID

// ──────────────────────────────────────────────────
// Constructors
// ──────────────────────────────────────────────────

// NewSuiteID returns a new random SuiteID.
func NewSuiteID() SuiteID { return must(typeid.Generate(PrefixSuite)) }

// NewCaseID returns a new random CaseID.
func NewCaseID() CaseID { return must(typeid.Generate(PrefixCase)) }

// NewEvalRunID returns a new random EvalRunID.
func NewEvalRunID() EvalRunID { return must(typeid.Generate(PrefixEvalRun)) }

// NewEvalResultID returns a new random EvalResultID.
func NewEvalResultID() EvalResultID { return must(typeid.Generate(PrefixEvalResult)) }

// NewBaselineID returns a new random BaselineID.
func NewBaselineID() BaselineID { return must(typeid.Generate(PrefixBaseline)) }

// NewRedTeamID returns a new random RedTeamID.
func NewRedTeamID() RedTeamID { return must(typeid.Generate(PrefixRedTeam)) }

// NewPromptVersionID returns a new random PromptVersionID.
func NewPromptVersionID() PromptVersionID { return must(typeid.Generate(PrefixPromptVersion)) }

// ──────────────────────────────────────────────────
// Parsing (validates prefix at parse time)
// ──────────────────────────────────────────────────

// ParseSuiteID parses a string into a SuiteID. Returns an error if the
// prefix is not "suite" or the suffix is invalid.
func ParseSuiteID(s string) (SuiteID, error) { return parseWithPrefix(PrefixSuite, s) }

// ParseCaseID parses a string into a CaseID.
func ParseCaseID(s string) (CaseID, error) { return parseWithPrefix(PrefixCase, s) }

// ParseEvalRunID parses a string into an EvalRunID.
func ParseEvalRunID(s string) (EvalRunID, error) { return parseWithPrefix(PrefixEvalRun, s) }

// ParseEvalResultID parses a string into an EvalResultID.
func ParseEvalResultID(s string) (EvalResultID, error) {
	return parseWithPrefix(PrefixEvalResult, s)
}

// ParseBaselineID parses a string into a BaselineID.
func ParseBaselineID(s string) (BaselineID, error) { return parseWithPrefix(PrefixBaseline, s) }

// ParseRedTeamID parses a string into a RedTeamID.
func ParseRedTeamID(s string) (RedTeamID, error) { return parseWithPrefix(PrefixRedTeam, s) }

// ParsePromptVersionID parses a string into a PromptVersionID.
func ParsePromptVersionID(s string) (PromptVersionID, error) {
	return parseWithPrefix(PrefixPromptVersion, s)
}

// ParseAny parses a string into an AnyID, accepting any valid prefix.
func ParseAny(s string) (AnyID, error) { return typeid.Parse(s) }

// ──────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────

// parseWithPrefix parses a TypeID and validates that its prefix matches expected.
func parseWithPrefix(expected, s string) (typeid.TypeID, error) {
	tid, err := typeid.Parse(s)
	if err != nil {
		return tid, err
	}
	if tid.Prefix() != expected {
		return tid, fmt.Errorf("id: expected prefix %q, got %q", expected, tid.Prefix())
	}
	return tid, nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
