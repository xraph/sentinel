package id_test

import (
	"strings"
	"testing"

	"github.com/xraph/sentinel/id"
)

func TestConstructors(t *testing.T) {
	tests := []struct {
		name   string
		newFn  func() id.ID
		prefix string
	}{
		{"SuiteID", id.NewSuiteID, "suite_"},
		{"CaseID", id.NewCaseID, "tcase_"},
		{"EvalRunID", id.NewEvalRunID, "erun_"},
		{"EvalResultID", id.NewEvalResultID, "eres_"},
		{"BaselineID", id.NewBaselineID, "base_"},
		{"RedTeamID", id.NewRedTeamID, "rteam_"},
		{"PromptVersionID", id.NewPromptVersionID, "pver_"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.newFn().String()
			if !strings.HasPrefix(got, tt.prefix) {
				t.Errorf("expected prefix %q, got %q", tt.prefix, got)
			}
		})
	}
}

func TestNew(t *testing.T) {
	i := id.New(id.PrefixSuite)
	if i.IsNil() {
		t.Fatal("expected non-nil ID")
	}
	if i.Prefix() != id.PrefixSuite {
		t.Errorf("expected prefix %q, got %q", id.PrefixSuite, i.Prefix())
	}
}

func TestParseRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		newFn   func() id.ID
		parseFn func(string) (id.ID, error)
	}{
		{"SuiteID", id.NewSuiteID, id.ParseSuiteID},
		{"CaseID", id.NewCaseID, id.ParseCaseID},
		{"EvalRunID", id.NewEvalRunID, id.ParseEvalRunID},
		{"EvalResultID", id.NewEvalResultID, id.ParseEvalResultID},
		{"BaselineID", id.NewBaselineID, id.ParseBaselineID},
		{"RedTeamID", id.NewRedTeamID, id.ParseRedTeamID},
		{"PromptVersionID", id.NewPromptVersionID, id.ParsePromptVersionID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.newFn()
			parsed, err := tt.parseFn(original.String())
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if parsed.String() != original.String() {
				t.Errorf("round-trip mismatch: %q != %q", parsed.String(), original.String())
			}
		})
	}
}

func TestCrossTypeRejection(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		parseFn func(string) (id.ID, error)
	}{
		{"ParseSuiteID rejects tcase_", id.NewCaseID().String(), id.ParseSuiteID},
		{"ParseCaseID rejects erun_", id.NewEvalRunID().String(), id.ParseCaseID},
		{"ParseEvalRunID rejects eres_", id.NewEvalResultID().String(), id.ParseEvalRunID},
		{"ParseEvalResultID rejects base_", id.NewBaselineID().String(), id.ParseEvalResultID},
		{"ParseBaselineID rejects rteam_", id.NewRedTeamID().String(), id.ParseBaselineID},
		{"ParseRedTeamID rejects pver_", id.NewPromptVersionID().String(), id.ParseRedTeamID},
		{"ParsePromptVersionID rejects suite_", id.NewSuiteID().String(), id.ParsePromptVersionID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.parseFn(tt.input)
			if err == nil {
				t.Errorf("expected error for cross-type parse of %q, got nil", tt.input)
			}
		})
	}
}

func TestParseAny(t *testing.T) {
	ids := []id.ID{
		id.NewSuiteID(),
		id.NewCaseID(),
		id.NewEvalRunID(),
		id.NewEvalResultID(),
		id.NewBaselineID(),
		id.NewRedTeamID(),
		id.NewPromptVersionID(),
	}

	for _, i := range ids {
		t.Run(i.String(), func(t *testing.T) {
			parsed, err := id.ParseAny(i.String())
			if err != nil {
				t.Fatalf("ParseAny(%q) failed: %v", i.String(), err)
			}
			if parsed.String() != i.String() {
				t.Errorf("round-trip mismatch: %q != %q", parsed.String(), i.String())
			}
		})
	}
}

func TestParseWithPrefix(t *testing.T) {
	i := id.NewSuiteID()
	parsed, err := id.ParseWithPrefix(i.String(), id.PrefixSuite)
	if err != nil {
		t.Fatalf("ParseWithPrefix failed: %v", err)
	}
	if parsed.String() != i.String() {
		t.Errorf("mismatch: %q != %q", parsed.String(), i.String())
	}

	_, err = id.ParseWithPrefix(i.String(), id.PrefixCase)
	if err == nil {
		t.Error("expected error for wrong prefix")
	}
}

func TestParseEmpty(t *testing.T) {
	_, err := id.Parse("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestNilID(t *testing.T) {
	var i id.ID
	if !i.IsNil() {
		t.Error("zero-value ID should be nil")
	}
	if i.String() != "" {
		t.Errorf("expected empty string, got %q", i.String())
	}
	if i.Prefix() != "" {
		t.Errorf("expected empty prefix, got %q", i.Prefix())
	}
}

func TestMarshalUnmarshalText(t *testing.T) {
	original := id.NewSuiteID()
	data, err := original.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	var restored id.ID
	if unmarshalErr := restored.UnmarshalText(data); unmarshalErr != nil {
		t.Fatalf("UnmarshalText failed: %v", unmarshalErr)
	}
	if restored.String() != original.String() {
		t.Errorf("mismatch: %q != %q", restored.String(), original.String())
	}

	// Nil round-trip.
	var nilID id.ID
	data, err = nilID.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText(nil) failed: %v", err)
	}
	var restored2 id.ID
	if err := restored2.UnmarshalText(data); err != nil {
		t.Fatalf("UnmarshalText(nil) failed: %v", err)
	}
	if !restored2.IsNil() {
		t.Error("expected nil after round-trip of nil ID")
	}
}

func TestValueScan(t *testing.T) {
	original := id.NewCaseID()
	val, err := original.Value()
	if err != nil {
		t.Fatalf("Value failed: %v", err)
	}

	var scanned id.ID
	if scanErr := scanned.Scan(val); scanErr != nil {
		t.Fatalf("Scan failed: %v", scanErr)
	}
	if scanned.String() != original.String() {
		t.Errorf("mismatch: %q != %q", scanned.String(), original.String())
	}

	// Nil round-trip.
	var nilID id.ID
	val, err = nilID.Value()
	if err != nil {
		t.Fatalf("Value(nil) failed: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil value for nil ID, got %v", val)
	}

	var scanned2 id.ID
	if err := scanned2.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) failed: %v", err)
	}
	if !scanned2.IsNil() {
		t.Error("expected nil after scan of nil")
	}
}

func TestUniqueness(t *testing.T) {
	a := id.NewSuiteID()
	b := id.NewSuiteID()
	if a.String() == b.String() {
		t.Errorf("two consecutive NewSuiteID() calls returned the same ID: %q", a.String())
	}
}

func TestBSONRoundTrip(t *testing.T) {
	original := id.NewSuiteID()

	bsonType, data, err := original.MarshalBSONValue()
	if err != nil {
		t.Fatalf("MarshalBSONValue failed: %v", err)
	}
	if bsonType != 0x02 {
		t.Fatalf("expected BSON string type 0x02, got 0x%02x", bsonType)
	}

	var restored id.ID
	if unmarshalErr := restored.UnmarshalBSONValue(bsonType, data); unmarshalErr != nil {
		t.Fatalf("UnmarshalBSONValue failed: %v", unmarshalErr)
	}
	if restored.String() != original.String() {
		t.Errorf("BSON round-trip mismatch: %q != %q", restored.String(), original.String())
	}

	var nilID id.ID
	bsonType, data, err = nilID.MarshalBSONValue()
	if err != nil {
		t.Fatalf("MarshalBSONValue(nil) failed: %v", err)
	}
	if bsonType != 0x0A {
		t.Fatalf("expected BSON null type 0x0A, got 0x%02x", bsonType)
	}

	var restored2 id.ID
	if unmarshalErr := restored2.UnmarshalBSONValue(bsonType, data); unmarshalErr != nil {
		t.Fatalf("UnmarshalBSONValue(nil) failed: %v", unmarshalErr)
	}
	if !restored2.IsNil() {
		t.Error("expected nil after BSON round-trip of nil ID")
	}
}

func TestBSONUnmarshalInvalidType(t *testing.T) {
	var restored id.ID
	err := restored.UnmarshalBSONValue(0x01, []byte{0x00, 0x00, 0x00, 0x00})
	if err == nil {
		t.Error("expected error for invalid BSON type, got nil")
	}
}
