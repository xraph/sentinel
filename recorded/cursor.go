package recorded

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
)

// SourceCursor marks the latest acknowledged source event for one binding.
// Position uses a host-defined, lexically ordered completion-time/run-ID key.
type SourceCursor struct {
	AppID     string    `json:"app_id"`
	TenantID  string    `json:"tenant_id"`
	BindingID string    `json:"binding_id"`
	Position  string    `json:"position"`
	JobID     id.ID     `json:"job_id"`
	Revision  int64     `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CursorAdvance acknowledges a persisted job. Position comes from that job,
// never from an unchecked caller-supplied high-water mark.
type CursorAdvance struct {
	BindingID        string
	ExpectedRevision int64
	JobID            id.ID
}

// CursorStorageKey encodes exact app, tenant and binding identity.
func CursorStorageKey(ctx context.Context, binding string) string {
	return InputDigest(strconv.Quote(sentinel.AppFromContext(ctx)) + "," + strconv.Quote(sentinel.TenantFromContext(ctx)) + "," + strconv.Quote(binding))
}

// Validate checks scope and acknowledgment identity.
func (c *SourceCursor) Validate(ctx context.Context) error {
	if c == nil || c.AppID == "" || c.AppID != sentinel.AppFromContext(ctx) || c.TenantID != sentinel.TenantFromContext(ctx) || c.BindingID == "" || len(c.BindingID) > 256 || c.Position == "" || len(c.Position) > 512 || c.JobID.IsNil() || c.Revision < 1 {
		return fmt.Errorf("%w: invalid recorded source cursor", sentinel.ErrInvalidInput)
	}
	return nil
}

// NextCursor validates the durable acknowledgment and computes a monotonic CAS.
// An overlapping older event returns the current cursor without a write.
func NextCursor(ctx context.Context, current *SourceCursor, j *Job, req CursorAdvance, now time.Time) (*SourceCursor, error) {
	if req.ExpectedRevision < 0 || req.BindingID == "" || req.JobID.IsNil() {
		return nil, fmt.Errorf("%w: invalid cursor acknowledgment", sentinel.ErrInvalidInput)
	}
	if err := j.Validate(ctx); err != nil {
		return nil, err
	}
	if j.ID != req.JobID || j.Request.SourceBindingID != req.BindingID || j.Request.SourceCursor == "" {
		return nil, ErrConflict
	}
	revision := int64(0)
	if current != nil {
		if err := current.Validate(ctx); err != nil {
			return nil, err
		}
		if current.BindingID != req.BindingID {
			return nil, ErrConflict
		}
		if j.Request.SourceCursor == current.Position && j.ID != current.JobID {
			return nil, ErrConflict
		}
		if j.Request.SourceCursor <= current.Position {
			copyCursor := *current
			return &copyCursor, nil
		}
		revision = current.Revision
	}
	if req.ExpectedRevision != revision {
		return nil, ErrConflict
	}
	next := &SourceCursor{AppID: j.AppID, TenantID: j.TenantID, BindingID: req.BindingID, Position: j.Request.SourceCursor, JobID: j.ID, Revision: revision + 1, UpdatedAt: now.UTC()}
	if err := next.Validate(ctx); err != nil {
		return nil, err
	}
	return next, nil
}

// DecodeCursor validates a stored cursor and returns a detached value.
func DecodeCursor(ctx context.Context, data []byte) (*SourceCursor, error) {
	var c SourceCursor
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if err := c.Validate(ctx); err != nil {
		return nil, err
	}
	return &c, nil
}
