package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/redteam"
	"github.com/xraph/sentinel/testcase"
)

// GenerateRedTeam writes attack cases for each type into a suite, up to
// redteam.MaxPerType per type. It validates every type before generating
// anything, so a refusal writes nothing.
//
// Leakage and injection cases carry the suite's effective prompt as the
// substring their not_contains scorer looks for. With no prompt that scorer
// falls back to the case's Expected text and checks the wrong thing, so both
// are refused for a suite without one.
func (e *Engine) GenerateRedTeam(ctx context.Context, suiteID id.SuiteID, types []redteam.AttackType, count int) ([]*testcase.Case, error) {
	if e.store == nil {
		return nil, sentinel.ErrNoStore
	}
	if len(types) == 0 {
		return nil, fmt.Errorf("%w: choose at least one attack type", sentinel.ErrInvalidInput)
	}
	if count < 1 {
		return nil, fmt.Errorf("%w: count must be at least 1", sentinel.ErrInvalidInput)
	}
	if count > redteam.MaxPerType {
		count = redteam.MaxPerType
	}
	s, err := e.store.GetSuite(ctx, suiteID)
	if err != nil {
		return nil, err
	}
	prompt, _, err := e.effectivePrompt(ctx, s)
	if err != nil {
		return nil, err
	}

	generators := make([]redteam.Generator, 0, len(types))
	for _, t := range types {
		g, ok := redteam.GeneratorFor(t)
		if !ok {
			return nil, fmt.Errorf("%w: unknown attack type %q", sentinel.ErrInvalidInput, t)
		}
		if (t == redteam.AttackLeakage || t == redteam.AttackInjection) && strings.TrimSpace(prompt) == "" {
			return nil, fmt.Errorf("%w: %s attacks need a system prompt to look for, and this suite has none", sentinel.ErrInvalidInput, t)
		}
		generators = append(generators, g)
	}

	var out []*testcase.Case
	for _, g := range generators {
		cases, err := g.Generate(ctx, &redteam.GenerateConfig{SuiteID: suiteID.String(), Count: count, SystemPrompt: prompt})
		if err != nil {
			return nil, fmt.Errorf("sentinel: generate %s attacks: %w", g.Type(), err)
		}
		for _, tc := range cases {
			if tc.Metadata == nil {
				tc.Metadata = map[string]any{}
			}
		}
		out = append(out, cases...)
	}
	if err := e.store.CreateCaseBatch(ctx, out); err != nil {
		return nil, fmt.Errorf("sentinel: store red team cases: %w", err)
	}
	return out, nil
}
