// Package recorded defines captured execution evidence and durable evaluation work.
package recorded

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
)

// Quality identifies measured, estimated or unknown accounting.
type Quality = evalrun.Quality

// Usage keeps original accounting provenance.
type Usage = evalrun.Usage

const (
	// Unknown includes historical accounting without provenance.
	Unknown = evalrun.Unknown
	// Known identifies measured accounting.
	Known = evalrun.Known
	// Estimated identifies inferred accounting.
	Estimated = evalrun.Estimated
)

// Outcome records how execution ended independently of evaluation status.
type Outcome string

const (
	// Completed means the executor reported a complete output.
	Completed Outcome = "completed"
	// Incomplete includes exhausted or truncated execution.
	Incomplete Outcome = "incomplete"
	// Failed means execution returned an error.
	Failed Outcome = "failed"
	// Cancelled means execution was cancelled.
	Cancelled Outcome = "cancelled"
	// Suspended means execution still requires external input.
	Suspended Outcome = "suspended"
	// UnknownOutcome covers historical records without termination provenance.
	UnknownOutcome Outcome = "unknown"
)

// Case is one captured execution. No executable target is part of the request.
type Case struct {
	CaseID         id.CaseID         `json:"case_id"`
	Repetition     int               `json:"repetition"`
	InputDigest    string            `json:"input_digest"`
	Outcome        Outcome           `json:"outcome"`
	Output         string            `json:"output"`
	ExecutionError string            `json:"execution_error,omitempty"`
	Duration       time.Duration     `json:"duration_ns"`
	Usage          Usage             `json:"usage"`
	Trace          *evalrun.RunTrace `json:"trace,omitempty"`
	ArtifactRefs   []string          `json:"artifact_refs,omitempty"`
	SourceRunID    string            `json:"source_run_id,omitempty"`
}

// Key identifies a case and repetition within a frozen evaluation.
func (c Case) Key() string { return fmt.Sprintf("%s/%d", c.CaseID, c.Repetition) }

// Request pins a plan and captured executions for idempotent ingestion.
type Request struct {
	PlanID             id.ID  `json:"plan_id"`
	PlanDigest         string `json:"plan_digest"`
	IdempotencyKey     string `json:"idempotency_key"`
	SourceExperimentID string `json:"source_experiment_id,omitempty"`
	SourceCandidateID  string `json:"source_candidate_id,omitempty"`
	SourceBindingID    string `json:"source_binding_id,omitempty"`
	SourceCursor       string `json:"source_cursor,omitempty"`
	Cases              []Case `json:"cases"`
}

// InputDigest identifies the exact UTF-8 input supplied to an execution.
func InputDigest(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

// Digest hashes the entire immutable request, including metric provenance.
func (r Request) Digest() (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("sentinel: encode recorded request: %w", err)
	}
	return InputDigest(string(data)), nil
}

func validDigest(digest string) bool {
	data, err := hex.DecodeString(digest)
	return err == nil && len(data) == sha256.Size
}

// Validate refuses missing identities, duplicate repetitions and invalid metrics.
func (r Request) Validate() error {
	if r.PlanID.IsNil() || !validDigest(r.PlanDigest) || r.IdempotencyKey == "" || len(r.IdempotencyKey) > 512 || len(r.Cases) == 0 {
		return fmt.Errorf("%w: invalid recorded request", sentinel.ErrInvalidInput)
	}
	if (r.SourceBindingID == "") != (r.SourceCursor == "") {
		return fmt.Errorf("%w: binding and cursor must be supplied together", sentinel.ErrInvalidInput)
	}
	seen := make(map[string]bool, len(r.Cases))
	for _, c := range r.Cases {
		if c.CaseID.IsNil() || c.Repetition < 0 || !validDigest(c.InputDigest) || c.Duration < 0 || seen[c.Key()] {
			return fmt.Errorf("%w: invalid or duplicate recorded case", sentinel.ErrInvalidInput)
		}
		seen[c.Key()] = true
		switch c.Outcome {
		case Completed, Incomplete, Failed, Cancelled, Suspended, UnknownOutcome:
		default:
			return fmt.Errorf("%w: unknown execution outcome", sentinel.ErrInvalidInput)
		}
		if err := c.Usage.Validate(); err != nil {
			return err
		}
	}
	return nil
}
