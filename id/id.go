// Package id defines TypeID-based identity types for all Sentinel entities.
//
// Every entity in Sentinel uses a single ID struct with a prefix that identifies
// the entity type. IDs are K-sortable (UUIDv7-based), globally unique,
// and URL-safe in the format "prefix_suffix".
package id

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"

	"go.jetify.com/typeid/v2"
)

// BSON type constants (avoids importing the mongo-driver bson package).
const (
	bsonTypeString byte = 0x02
	bsonTypeNull   byte = 0x0A
)

// Prefix identifies the entity type encoded in a TypeID.
type Prefix string

// Prefix constants for all Sentinel entity types.
const (
	PrefixSuite         Prefix = "suite"
	PrefixCase          Prefix = "tcase"
	PrefixEvalRun       Prefix = "erun"
	PrefixEvalResult    Prefix = "eres"
	PrefixBaseline      Prefix = "base"
	PrefixRedTeam       Prefix = "rteam"
	PrefixPromptVersion Prefix = "pver"
)

// ID is the primary identifier type for all Sentinel entities.
// It wraps a TypeID providing a prefix-qualified, globally unique,
// sortable, URL-safe identifier in the format "prefix_suffix".
//
//nolint:recvcheck // Value receivers for read-only methods, pointer receivers for UnmarshalText/Scan.
type ID struct {
	inner typeid.TypeID
	valid bool
}

// Nil is the zero-value ID.
var Nil ID

// New generates a new globally unique ID with the given prefix.
// It panics if prefix is not a valid TypeID prefix (programming error).
func New(prefix Prefix) ID {
	tid, err := typeid.Generate(string(prefix))
	if err != nil {
		panic(fmt.Sprintf("id: invalid prefix %q: %v", prefix, err))
	}

	return ID{inner: tid, valid: true}
}

// Parse parses a TypeID string (e.g., "suite_01h2xcejqtf2nbrexx3vqjhp41")
// into an ID. Returns an error if the string is not valid.
func Parse(s string) (ID, error) {
	if s == "" {
		return Nil, fmt.Errorf("id: parse %q: empty string", s)
	}

	tid, err := typeid.Parse(s)
	if err != nil {
		return Nil, fmt.Errorf("id: parse %q: %w", s, err)
	}

	return ID{inner: tid, valid: true}, nil
}

// ParseWithPrefix parses a TypeID string and validates that its prefix
// matches the expected value.
func ParseWithPrefix(s string, expected Prefix) (ID, error) {
	parsed, err := Parse(s)
	if err != nil {
		return Nil, err
	}

	if parsed.Prefix() != expected {
		return Nil, fmt.Errorf("id: expected prefix %q, got %q", expected, parsed.Prefix())
	}

	return parsed, nil
}

// MustParse is like Parse but panics on error. Use for hardcoded ID values.
func MustParse(s string) ID {
	parsed, err := Parse(s)
	if err != nil {
		panic(fmt.Sprintf("id: must parse %q: %v", s, err))
	}

	return parsed
}

// MustParseWithPrefix is like ParseWithPrefix but panics on error.
func MustParseWithPrefix(s string, expected Prefix) ID {
	parsed, err := ParseWithPrefix(s, expected)
	if err != nil {
		panic(fmt.Sprintf("id: must parse with prefix %q: %v", expected, err))
	}

	return parsed
}

// ──────────────────────────────────────────────────
// Type aliases for backward compatibility
// ──────────────────────────────────────────────────

// SuiteID is a type-safe identifier for evaluation suites (prefix: "suite").
type SuiteID = ID

// CaseID is a type-safe identifier for test cases (prefix: "tcase").
type CaseID = ID

// EvalRunID is a type-safe identifier for evaluation runs (prefix: "erun").
type EvalRunID = ID

// EvalResultID is a type-safe identifier for evaluation results (prefix: "eres").
type EvalResultID = ID

// BaselineID is a type-safe identifier for baselines (prefix: "base").
type BaselineID = ID

// RedTeamID is a type-safe identifier for red team sessions (prefix: "rteam").
type RedTeamID = ID

// PromptVersionID is a type-safe identifier for prompt versions (prefix: "pver").
type PromptVersionID = ID

// AnyID is a type alias that accepts any valid prefix.
type AnyID = ID

// ──────────────────────────────────────────────────
// Convenience constructors
// ──────────────────────────────────────────────────

// NewSuiteID generates a new unique suite ID.
func NewSuiteID() ID { return New(PrefixSuite) }

// NewCaseID generates a new unique case ID.
func NewCaseID() ID { return New(PrefixCase) }

// NewEvalRunID generates a new unique eval run ID.
func NewEvalRunID() ID { return New(PrefixEvalRun) }

// NewEvalResultID generates a new unique eval result ID.
func NewEvalResultID() ID { return New(PrefixEvalResult) }

// NewBaselineID generates a new unique baseline ID.
func NewBaselineID() ID { return New(PrefixBaseline) }

// NewRedTeamID generates a new unique red team ID.
func NewRedTeamID() ID { return New(PrefixRedTeam) }

// NewPromptVersionID generates a new unique prompt version ID.
func NewPromptVersionID() ID { return New(PrefixPromptVersion) }

// ──────────────────────────────────────────────────
// Convenience parsers
// ──────────────────────────────────────────────────

// ParseSuiteID parses a string and validates the "suite" prefix.
func ParseSuiteID(s string) (ID, error) { return ParseWithPrefix(s, PrefixSuite) }

// ParseCaseID parses a string and validates the "tcase" prefix.
func ParseCaseID(s string) (ID, error) { return ParseWithPrefix(s, PrefixCase) }

// ParseEvalRunID parses a string and validates the "erun" prefix.
func ParseEvalRunID(s string) (ID, error) { return ParseWithPrefix(s, PrefixEvalRun) }

// ParseEvalResultID parses a string and validates the "eres" prefix.
func ParseEvalResultID(s string) (ID, error) { return ParseWithPrefix(s, PrefixEvalResult) }

// ParseBaselineID parses a string and validates the "base" prefix.
func ParseBaselineID(s string) (ID, error) { return ParseWithPrefix(s, PrefixBaseline) }

// ParseRedTeamID parses a string and validates the "rteam" prefix.
func ParseRedTeamID(s string) (ID, error) { return ParseWithPrefix(s, PrefixRedTeam) }

// ParsePromptVersionID parses a string and validates the "pver" prefix.
func ParsePromptVersionID(s string) (ID, error) { return ParseWithPrefix(s, PrefixPromptVersion) }

// ParseAny parses a string into an ID without type checking the prefix.
func ParseAny(s string) (ID, error) { return Parse(s) }

// ──────────────────────────────────────────────────
// ID methods
// ──────────────────────────────────────────────────

// String returns the full TypeID string representation (prefix_suffix).
// Returns an empty string for the Nil ID.
func (i ID) String() string {
	if !i.valid {
		return ""
	}

	return i.inner.String()
}

// Prefix returns the prefix component of this ID.
func (i ID) Prefix() Prefix {
	if !i.valid {
		return ""
	}

	return Prefix(i.inner.Prefix())
}

// IsNil reports whether this ID is the zero value.
func (i ID) IsNil() bool {
	return !i.valid
}

// MarshalText implements encoding.TextMarshaler.
func (i ID) MarshalText() ([]byte, error) {
	if !i.valid {
		return []byte{}, nil
	}

	return []byte(i.inner.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (i *ID) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		*i = Nil

		return nil
	}

	parsed, err := Parse(string(data))
	if err != nil {
		return err
	}

	*i = parsed

	return nil
}

// MarshalBSONValue satisfies bson.ValueMarshaler (mongo-driver v2) so the ID
// is stored as a BSON string instead of an opaque struct. No bson import needed
// because Go uses structural typing for interface satisfaction.
func (i ID) MarshalBSONValue() (bsonType byte, data []byte, err error) {
	if !i.valid {
		return bsonTypeNull, nil, nil
	}

	s := i.inner.String()
	l := len(s) + 1 // length includes null terminator

	buf := make([]byte, 4+len(s)+1)
	binary.LittleEndian.PutUint32(buf, uint32(l)) //nolint:gosec // TypeID strings are <64 bytes; no overflow
	copy(buf[4:], s)
	// trailing 0x00 is already zero from make

	return bsonTypeString, buf, nil
}

// UnmarshalBSONValue satisfies bson.ValueUnmarshaler (mongo-driver v2).
func (i *ID) UnmarshalBSONValue(t byte, data []byte) error {
	if t == bsonTypeNull {
		*i = Nil

		return nil
	}

	if t != bsonTypeString {
		return fmt.Errorf("id: cannot unmarshal BSON type 0x%02x into ID", t)
	}

	if len(data) < 5 { //nolint:mnd // 4-byte length + at least 1 null terminator
		*i = Nil

		return nil
	}

	l := binary.LittleEndian.Uint32(data[:4])
	if l <= 1 { // empty string (just null terminator)
		*i = Nil

		return nil
	}

	s := string(data[4 : 4+l-1]) // exclude null terminator

	return i.UnmarshalText([]byte(s))
}

// Value implements driver.Valuer for database storage.
// Returns nil for the Nil ID so that optional foreign key columns store NULL.
func (i ID) Value() (driver.Value, error) {
	if !i.valid {
		return nil, nil //nolint:nilnil // nil is the canonical NULL for driver.Valuer
	}

	return i.inner.String(), nil
}

// Scan implements sql.Scanner for database retrieval.
func (i *ID) Scan(src any) error {
	if src == nil {
		*i = Nil

		return nil
	}

	switch v := src.(type) {
	case string:
		if v == "" {
			*i = Nil

			return nil
		}

		return i.UnmarshalText([]byte(v))
	case []byte:
		if len(v) == 0 {
			*i = Nil

			return nil
		}

		return i.UnmarshalText(v)
	default:
		return fmt.Errorf("id: cannot scan %T into ID", src)
	}
}
