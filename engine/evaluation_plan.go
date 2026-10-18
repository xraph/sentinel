package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalplan"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

// EvaluationPlan is an immutable evaluation definition persisted by Sentinel.
type EvaluationPlan = evalplan.Plan

// FreezeRequest selects cases and additional criteria. Empty CaseIDs selects all.
type FreezeRequest struct {
	SuiteID id.SuiteID
	CaseIDs []id.CaseID
	Scorers []evalplan.Criterion
}

// FreezeEvaluation resolves case, prompt and scorer definitions before execution.
func (e *Engine) FreezeEvaluation(ctx context.Context, req FreezeRequest) (*EvaluationPlan, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	if sentinel.AppFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: frozen evaluation requires app scope", sentinel.ErrInvalidInput)
	}
	before, err := e.evaluationSources(ctx, req.SuiteID)
	if err != nil {
		return nil, err
	}
	// JSON detaches caller-owned maps as well as mutable memory-store values.
	var frozen evaluationSources
	if decodeErr := json.Unmarshal(before, &frozen); decodeErr != nil {
		return nil, decodeErr
	}
	selected := make(map[id.CaseID]bool, len(req.CaseIDs))
	for _, caseID := range req.CaseIDs {
		if caseID.IsNil() || selected[caseID] {
			return nil, fmt.Errorf("%w: duplicate or empty case ID", sentinel.ErrInvalidInput)
		}
		selected[caseID] = true
	}
	p := &evalplan.Plan{ID: id.New("eplan"), SchemaVersion: evalplan.SchemaVersion, AppID: sentinel.AppFromContext(ctx), TenantID: sentinel.TenantFromContext(ctx), Suite: frozen.Suite, PromptVersionID: frozen.PromptVersionID, PassThreshold: e.config.PassThreshold, CreatedAt: time.Now().UTC()}
	for _, tc := range frozen.Cases {
		if len(req.CaseIDs) > 0 && !selected[tc.ID] {
			continue
		}
		delete(selected, tc.ID)
		criteria := append([]evalplan.Criterion(nil), req.Scorers...)
		for i, cfg := range tc.Scorers {
			criteria = append(criteria, evalplan.Criterion{ID: fmt.Sprintf("case:%s:%d", cfg.Name, i), Scorer: cfg.Name, Config: cfg.Config, Required: true})
		}
		for i := range criteria {
			c := &criteria[i]
			revision, revErr := e.scorers.Revision(c.Scorer)
			if revErr != nil {
				return nil, revErr
			}
			if c.Revision != "" && c.Revision != revision {
				return nil, fmt.Errorf("%w: scorer %s revision unavailable", sentinel.ErrInvalidInput, c.Scorer)
			}
			c.Revision = revision
			if c.Scorer == "exact" {
				c.RequiresExpected = true
			}
			if _, getErr := e.scorers.GetVersioned(c.Scorer, c.Revision, c.Config); getErr != nil {
				return nil, getErr
			}
		}
		p.Cases = append(p.Cases, evalplan.Case{Case: tc, Criteria: criteria})
	}
	if len(selected) > 0 {
		return nil, fmt.Errorf("%w: requested case is outside suite", sentinel.ErrInvalidInput)
	}
	p.Digest, err = p.ContentDigest()
	if err != nil {
		return nil, err
	}
	if validateErr := p.Validate(ctx); validateErr != nil {
		return nil, validateErr
	}
	after, err := e.evaluationSources(ctx, req.SuiteID)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(before, after) {
		return nil, fmt.Errorf("%w: evaluation inputs changed while freezing", sentinel.ErrInvalidState)
	}
	if storeErr := e.store.CreateEvaluationPlan(ctx, p); storeErr != nil {
		return nil, storeErr
	}
	return e.store.GetEvaluationPlan(ctx, p.ID)
}

type evaluationSources struct {
	Suite           suite.Suite     `json:"suite"`
	Cases           []testcase.Case `json:"cases"`
	PromptVersionID string          `json:"prompt_version_id"`
}

func (e *Engine) evaluationSources(ctx context.Context, suiteID id.SuiteID) ([]byte, error) {
	su, err := e.store.GetSuite(ctx, suiteID)
	if err != nil {
		return nil, err
	}
	if su.AppID != sentinel.AppFromContext(ctx) {
		return nil, sentinel.ErrSuiteNotFound
	}
	cases, err := e.store.ListCases(ctx, suiteID)
	if err != nil {
		return nil, err
	}
	systemPrompt, versionID, err := e.effectivePrompt(ctx, su)
	if err != nil {
		return nil, err
	}
	source := evaluationSources{Suite: *su, PromptVersionID: versionID}
	source.Suite.SystemPrompt = systemPrompt
	for _, tc := range cases {
		source.Cases = append(source.Cases, *tc)
	}
	sort.Slice(source.Cases, func(i, j int) bool { return source.Cases[i].ID.String() < source.Cases[j].ID.String() })
	data, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("sentinel: freeze evaluation inputs: %w", err)
	}
	return data, nil
}
