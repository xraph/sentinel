// Package evalplan defines immutable evaluation inputs and scorer identities.
package evalplan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

// SchemaVersion is the supported frozen-plan wire version.
const SchemaVersion = 1

// Criterion pins a scorer and the constraints it must satisfy.
type Criterion struct {
	ID               string         `json:"id"`
	Scorer           string         `json:"scorer"`
	Revision         string         `json:"revision"`
	Config           map[string]any `json:"config,omitempty"`
	Required         bool           `json:"required"`
	RequiresExpected bool           `json:"requires_expected"`
}

// Case contains private scoring inputs and the exact criteria applied to them.
type Case struct {
	Case     testcase.Case `json:"case"`
	Criteria []Criterion   `json:"criteria"`
}

// Plan freezes scoring inputs independently of any target or agent runtime.
type Plan struct {
	ID              id.ID       `json:"id"`
	SchemaVersion   int         `json:"schema_version"`
	AppID           string      `json:"app_id"`
	TenantID        string      `json:"tenant_id"`
	Suite           suite.Suite `json:"suite"`
	PromptVersionID string      `json:"prompt_version_id,omitempty"`
	Cases           []Case      `json:"cases"`
	PassThreshold   float64     `json:"pass_threshold"`
	Digest          string      `json:"digest"`
	CreatedAt       time.Time   `json:"created_at"`
}

// ContentDigest hashes scoring content, excluding storage identity and time.
func (p Plan) ContentDigest() (string, error) {
	p.ID = id.Nil
	p.Digest = ""
	p.CreatedAt = time.Time{}
	data, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("sentinel: encode evaluation plan: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Validate refuses unsupported, unscoped or modified frozen plans.
func (p *Plan) Validate(ctx context.Context) error {
	if p == nil || p.ID.IsNil() || p.SchemaVersion != SchemaVersion || len(p.Cases) == 0 {
		return fmt.Errorf("%w: invalid evaluation plan", sentinel.ErrInvalidInput)
	}
	if sentinel.AppFromContext(ctx) == "" || p.AppID != sentinel.AppFromContext(ctx) || p.TenantID != sentinel.TenantFromContext(ctx) || p.Suite.AppID != p.AppID {
		return fmt.Errorf("%w: evaluation plan scope", sentinel.ErrInvalidInput)
	}
	if math.IsNaN(p.PassThreshold) || p.PassThreshold < 0 || p.PassThreshold > 1 {
		return fmt.Errorf("%w: pass threshold", sentinel.ErrInvalidInput)
	}
	seen := make(map[id.CaseID]bool)
	for _, c := range p.Cases {
		if c.Case.ID.IsNil() || c.Case.SuiteID != p.Suite.ID || seen[c.Case.ID] || len(c.Criteria) == 0 {
			return fmt.Errorf("%w: duplicate or invalid frozen case", sentinel.ErrInvalidInput)
		}
		seen[c.Case.ID] = true
		criteria := make(map[string]bool)
		for _, criterion := range c.Criteria {
			if criterion.ID == "" || criterion.Scorer == "" || criterion.Revision == "" || criteria[criterion.ID] {
				return fmt.Errorf("%w: invalid frozen criterion", sentinel.ErrInvalidInput)
			}
			criteria[criterion.ID] = true
		}
	}
	digest, err := p.ContentDigest()
	if err != nil {
		return err
	}
	if p.Digest != digest {
		return fmt.Errorf("%w: plan digest mismatch", sentinel.ErrInvalidInput)
	}
	return nil
}

// Encode validates a plan before crossing a store boundary.
func Encode(ctx context.Context, p *Plan) ([]byte, error) {
	if err := p.Validate(ctx); err != nil {
		return nil, err
	}
	return json.Marshal(p)
}

// Decode reads and validates a stored plan without sharing mutable data.
func Decode(ctx context.Context, data []byte) (*Plan, error) {
	var p Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("sentinel: decode evaluation plan: %w", err)
	}
	if err := p.Validate(ctx); err != nil {
		return nil, err
	}
	return &p, nil
}
