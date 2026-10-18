package recorded

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
)

var (
	// ErrConflict means an idempotency key, revision or lease no longer matches.
	ErrConflict = errors.New("sentinel: recorded evaluation conflict")
	// ErrNotFound means the exact scope has no matching recorded job.
	ErrNotFound = errors.New("sentinel: recorded evaluation not found")
)

// State is the durable recorded-evaluation lifecycle.
type State string

const (
	// JobQueued is waiting for a worker.
	JobQueued State = "queued"
	// JobRunning has a lease, which may need recovery after expiry.
	JobRunning State = "running"
	// JobCompleted has a scored result for every captured case.
	JobCompleted State = "completed"
	// JobFailed stopped because of infrastructure failure or exhausted retries.
	JobFailed State = "failed"
	// JobCancelled retains partial results after explicit cancellation.
	JobCancelled State = "cancelled"
)

// CaseResult records scoring separately from original execution accounting.
type CaseResult struct {
	Key             string          `json:"key"`
	Result          *evalrun.Result `json:"result"`
	ScoringDuration time.Duration   `json:"scoring_duration_ns"`
	JudgeUsage      Usage           `json:"judge_usage"`
	ExecutionError  string          `json:"execution_error,omitempty"`
	ScoringError    string          `json:"scoring_error,omitempty"`
}

// Job stores an immutable request and append-only result checkpoints.
type Job struct {
	ID            id.ID        `json:"id"`
	AppID         string       `json:"app_id"`
	TenantID      string       `json:"tenant_id"`
	Request       Request      `json:"request"`
	RequestDigest string       `json:"request_digest"`
	RunID         id.EvalRunID `json:"run_id"`
	Revision      int64        `json:"revision"`
	State         State        `json:"state"`
	Owner         string       `json:"owner,omitempty"`
	Generation    int64        `json:"generation"`
	LeaseUntil    time.Time    `json:"lease_until"`
	Attempts      int          `json:"attempts"`
	MaxAttempts   int          `json:"max_attempts"`
	Results       []CaseResult `json:"results"`
	Error         string       `json:"error,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// NewJob constructs detached queued work in the authenticated app and tenant.
func NewJob(ctx context.Context, req Request) (*Job, error) {
	digest, err := req.Digest()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	j := &Job{ID: id.New("ejob"), AppID: sentinel.AppFromContext(ctx), TenantID: sentinel.TenantFromContext(ctx), Request: req, RequestDigest: digest, RunID: id.NewEvalRunID(), Revision: 1, State: JobQueued, MaxAttempts: 3, CreatedAt: now, UpdatedAt: now}
	if validateErr := j.Validate(ctx); validateErr != nil {
		return nil, validateErr
	}
	return j.Clone()
}

// Clone detaches nested request data and result maps.
func (j *Job) Clone() (*Job, error) {
	data, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	var copyJob Job
	if decodeErr := json.Unmarshal(data, &copyJob); decodeErr != nil {
		return nil, decodeErr
	}
	return &copyJob, nil
}

// Terminal reports whether work can no longer be claimed or changed.
func (j *Job) Terminal() bool {
	return j.State == JobCompleted || j.State == JobFailed || j.State == JobCancelled
}

// Validate checks scope, request identity and every persisted result identity.
func (j *Job) Validate(ctx context.Context) error {
	if j == nil || j.ID.IsNil() || j.RunID.IsNil() || j.AppID == "" || j.AppID != sentinel.AppFromContext(ctx) || j.TenantID != sentinel.TenantFromContext(ctx) || j.Revision < 1 || j.MaxAttempts < 1 || j.MaxAttempts > 10 || j.Attempts < 0 || j.Attempts > j.MaxAttempts || j.Generation < 0 {
		return fmt.Errorf("%w: invalid recorded job", sentinel.ErrInvalidInput)
	}
	if err := j.Request.Validate(); err != nil {
		return err
	}
	digest, err := j.Request.Digest()
	if err != nil {
		return err
	}
	if digest != j.RequestDigest {
		return fmt.Errorf("%w: recorded request digest mismatch", sentinel.ErrInvalidInput)
	}
	switch j.State {
	case JobQueued, JobRunning, JobCompleted, JobFailed, JobCancelled:
	default:
		return fmt.Errorf("%w: invalid recorded state", sentinel.ErrInvalidState)
	}
	if j.State == JobRunning && (j.Owner == "" || j.LeaseUntil.IsZero() || j.Generation == 0 || j.Attempts == 0) {
		return fmt.Errorf("%w: missing recorded lease", sentinel.ErrInvalidInput)
	}
	if len(j.Results) > len(j.Request.Cases) || (j.State == JobCompleted && len(j.Results) != len(j.Request.Cases)) {
		return fmt.Errorf("%w: incomplete recorded results", sentinel.ErrInvalidInput)
	}
	for i, r := range j.Results {
		c := j.Request.Cases[i]
		if r.Key != c.Key() || r.Result == nil || r.Result.ID.IsNil() || r.Result.RunID != j.RunID || r.Result.CaseID != c.CaseID || r.ScoringDuration < 0 || math.IsNaN(r.Result.Score) || math.IsInf(r.Result.Score, 0) || r.Result.Score < 0 || r.Result.Score > 1 {
			return fmt.Errorf("%w: invalid recorded result", sentinel.ErrInvalidInput)
		}
		switch r.Result.Status {
		case evalrun.StatusPass, evalrun.StatusFail, evalrun.StatusError:
		default:
			return fmt.Errorf("%w: invalid recorded result status", sentinel.ErrInvalidInput)
		}
		if usageErr := r.JudgeUsage.Validate(); usageErr != nil {
			return usageErr
		}
	}
	return nil
}

// Claim creates the next fenced lease, or a terminal failure after bounded retries.
func (j *Job) Claim(owner string, ttl time.Duration, now time.Time) (*Job, error) {
	if j.Terminal() || (j.State == JobRunning && j.LeaseUntil.After(now)) || owner == "" || ttl <= 0 || ttl > 5*time.Minute {
		return nil, ErrConflict
	}
	next, err := j.Clone()
	if err != nil {
		return nil, err
	}
	next.Revision++
	next.UpdatedAt = now.UTC()
	if j.Attempts >= j.MaxAttempts {
		next.State = JobFailed
		next.Error = "recorded evaluation retry limit exhausted"
		return next, nil
	}
	next.State = JobRunning
	next.Owner = owner
	next.Generation++
	next.Attempts++
	next.LeaseUntil = now.Add(ttl).UTC()
	return next, nil
}

// ValidateChange enforces immutable inputs, append-only results and lease fences.
// A backend must also compare the old revision atomically when writing.
func ValidateChange(ctx context.Context, old, next *Job, now time.Time) error {
	if err := old.Validate(ctx); err != nil {
		return err
	}
	if err := next.Validate(ctx); err != nil {
		return err
	}
	if old.Terminal() || next.ID != old.ID || next.RunID != old.RunID || next.RequestDigest != old.RequestDigest || next.MaxAttempts != old.MaxAttempts || next.Revision != old.Revision+1 || !next.CreatedAt.Equal(old.CreatedAt) || len(next.Results) < len(old.Results) {
		return ErrConflict
	}
	before, err := json.Marshal(old.Results)
	if err != nil {
		return err
	}
	after, err := json.Marshal(next.Results[:len(old.Results)])
	if err != nil {
		return err
	}
	if len(old.Results) > 0 && !bytes.Equal(before, after) {
		return ErrConflict
	}
	claimable := old.State == JobQueued || (old.State == JobRunning && !old.LeaseUntil.After(now))
	if claimable {
		if len(next.Results) != len(old.Results) {
			return ErrConflict
		}
		if old.Attempts == old.MaxAttempts && next.State == JobFailed && next.Attempts == old.Attempts && next.Generation == old.Generation {
			return nil
		}
		if old.Attempts >= old.MaxAttempts || next.State != JobRunning || next.Generation != old.Generation+1 || next.Attempts != old.Attempts+1 || next.Owner == "" || !next.LeaseUntil.After(now) || next.LeaseUntil.After(now.Add(5*time.Minute)) {
			return ErrConflict
		}
		return nil
	}
	if old.State != JobRunning || next.Owner != old.Owner || next.Generation != old.Generation || next.Attempts != old.Attempts || next.State == JobQueued || next.LeaseUntil.Before(old.LeaseUntil) || next.LeaseUntil.After(now.Add(5*time.Minute)) {
		return ErrConflict
	}
	return nil
}
