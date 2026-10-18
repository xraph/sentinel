package recorded_test

import (
	"testing"
	"time"

	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
)

func validRequest() recorded.Request {
	return recorded.Request{PlanID: id.New("eplan"), PlanDigest: recorded.InputDigest("plan"), IdempotencyKey: "source-run/v1", Cases: []recorded.Case{{CaseID: id.NewCaseID(), Repetition: 0, InputDigest: recorded.InputDigest("question"), Outcome: recorded.Completed, Output: "answer", Duration: time.Second}}}
}

func TestRecordedRequestPreservesUnknownUsage(t *testing.T) {
	r := validRequest()
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if r.Cases[0].Usage.CostKnown() {
		t.Fatal("historical zero cost became known")
	}
	before, err := r.Digest()
	if err != nil {
		t.Fatal(err)
	}
	r.Cases[0].Usage.CostQuality = recorded.Known
	r.Cases[0].Usage.Currency = "USD"
	after, err := r.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("accounting provenance excluded from request identity")
	}
}

func TestRecordedRequestRefusesAmbiguousIdentity(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*recorded.Request)
	}{
		{"duplicate", func(r *recorded.Request) { r.Cases = append(r.Cases, r.Cases[0]) }},
		{"missing key", func(r *recorded.Request) { r.IdempotencyKey = "" }},
		{"negative repetition", func(r *recorded.Request) { r.Cases[0].Repetition = -1 }},
		{"invalid digest", func(r *recorded.Request) { r.Cases[0].InputDigest = "question" }},
		{"missing outcome", func(r *recorded.Request) { r.Cases[0].Outcome = "" }},
		{"negative duration", func(r *recorded.Request) { r.Cases[0].Duration = -1 }},
		{"invalid quality", func(r *recorded.Request) { r.Cases[0].Usage.CostQuality = "probably" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := validRequest()
			test.edit(&r)
			if err := r.Validate(); err == nil {
				t.Fatal("ambiguous request accepted")
			}
		})
	}
}
