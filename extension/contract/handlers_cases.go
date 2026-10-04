package contract

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/testcase"
)

// maxImportBytes caps cases.import. A file larger than this is a mistake
// more often than a suite.
const maxImportBytes = 1 << 20

// CaseView is a test case. A red-team case's not_contains substring is the
// system prompt it looks for, so it is redacted to its length.
type CaseView struct {
	ID           string             `json:"id"`
	SuiteID      string             `json:"suiteId"`
	Name         string             `json:"name"`
	Input        string             `json:"input"`
	Expected     string             `json:"expected,omitempty"`
	ScenarioType string             `json:"scenarioType"`
	Tags         []string           `json:"tags"`
	Scorers      []ScorerConfigView `json:"scorers"`
	Context      map[string]any     `json:"context"`
	Metadata     map[string]any     `json:"metadata"`
	RedTeam      *RedTeamRef        `json:"redTeam,omitempty"`
	CreatedAt    string             `json:"createdAt"`
	UpdatedAt    string             `json:"updatedAt"`
}

// ScorerConfigView is a case's scorer config as the client may see it.
type ScorerConfigView struct {
	Name     string         `json:"name"`
	Config   map[string]any `json:"config"`
	Redacted *Redaction     `json:"redacted,omitempty"`
}

// Redaction says a config value was withheld and how long it was.
type Redaction struct {
	Key    string `json:"key"`
	Length int    `json:"length"`
}

// RedTeamRef marks a red-team case or result.
type RedTeamRef struct {
	AttackType string `json:"attackType"`
}

type caseRef struct {
	CaseID string `json:"caseId"`
}

type scorerConfigInput struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
}

type casesListOutput struct {
	Items []CaseView `json:"items"`
}

type casesCreateInput struct {
	SuiteID      string              `json:"suiteId"`
	Name         string              `json:"name"`
	Input        string              `json:"input"`
	Expected     string              `json:"expected"`
	ScenarioType string              `json:"scenarioType"`
	Tags         []string            `json:"tags"`
	Scorers      []scorerConfigInput `json:"scorers"`
}

type casesUpdateInput struct {
	CaseID       string               `json:"caseId"`
	Name         *string              `json:"name"`
	Input        *string              `json:"input"`
	Expected     *string              `json:"expected"`
	ScenarioType *string              `json:"scenarioType"`
	Tags         *[]string            `json:"tags"`
	Scorers      *[]scorerConfigInput `json:"scorers"`
}

type casesDeleteOutput struct {
	CaseID string `json:"caseId"`
}

type casesImportInput struct {
	SuiteID string `json:"suiteId"`
	Format  string `json:"format"`
	Data    string `json:"data"`
}

type casesImportOutput struct {
	Imported int64 `json:"imported"`
}

var scenarioTypes = map[testcase.ScenarioType]bool{
	testcase.ScenarioStandard: true, testcase.ScenarioSkillChallenge: true, testcase.ScenarioTraitProbe: true,
	testcase.ScenarioBehaviorTrigger: true, testcase.ScenarioCognitiveStress: true, testcase.ScenarioCommsAdaptation: true,
	testcase.ScenarioPerceptionTest: true, testcase.ScenarioPersonaCoherence: true,
}

// attackTypeOf returns the case's red-team attack type, "unknown" for a
// red-team case that does not say, and "" for an ordinary case.
func attackTypeOf(tc *testcase.Case) string {
	red := false
	for _, t := range tc.Tags {
		if t == "redteam" {
			red = true
		}
	}
	if !red {
		return ""
	}
	if at, ok := tc.Context["attack_type"].(string); ok && at != "" {
		return at
	}
	return "unknown"
}

// promptHidden reports whether the case's scorers may embed the system
// prompt under test. It reads the stored Context, which no contract write
// can change, never the tags a client can edit.
func promptHidden(tc *testcase.Case) bool {
	at, ok := tc.Context["attack_type"].(string)
	return ok && at != ""
}

// scorerViews copies each scorer's config for the client. Under a red-team
// case every not_contains substring (leakage and injection cases both put
// the system prompt there) is replaced by its length. The stored config is
// never modified.
func scorerViews(tc *testcase.Case) []ScorerConfigView {
	hide := promptHidden(tc)
	out := make([]ScorerConfigView, 0, len(tc.Scorers))
	for _, sc := range tc.Scorers {
		v := ScorerConfigView{Name: sc.Name, Config: mapOrEmpty(sc.Config)}
		if secret, ok := sc.Config["substring"].(string); ok && hide && sc.Name == "not_contains" {
			clean := make(map[string]any, len(sc.Config))
			for k, val := range sc.Config {
				if k != "substring" {
					clean[k] = val
				}
			}
			v.Config = clean
			v.Redacted = &Redaction{Key: "substring", Length: utf8.RuneCountInString(secret)}
		}
		out = append(out, v)
	}
	return out
}

func caseView(tc *testcase.Case) CaseView {
	v := CaseView{
		ID: tc.ID.String(), SuiteID: tc.SuiteID.String(), Name: tc.Name, Input: tc.Input, Expected: tc.Expected,
		ScenarioType: string(tc.ScenarioType), Tags: stringsOrEmpty(tc.Tags), Scorers: scorerViews(tc),
		Context: mapOrEmpty(tc.Context), Metadata: mapOrEmpty(tc.Metadata), CreatedAt: ts(tc.CreatedAt), UpdatedAt: ts(tc.UpdatedAt),
	}
	if at := attackTypeOf(tc); at != "" {
		v.RedTeam = &RedTeamRef{AttackType: at}
	}
	return v
}

// caseInApp loads a case through its suite and answers NOT_FOUND for a
// malformed id, a missing case and another app's case alike.
func (d Deps) caseInApp(ctx context.Context, app, rawID string) (*testcase.Case, error) {
	cid, err := id.ParseCaseID(rawID)
	if err != nil {
		return nil, notFound("case not found")
	}
	tc, err := d.Engine.GetCase(ctx, cid)
	if errors.Is(err, sentinel.ErrCaseNotFound) {
		return nil, notFound("case not found")
	}
	if err != nil {
		return nil, err
	}
	if _, err := d.suiteInApp(ctx, app, tc.SuiteID.String()); err != nil {
		// Only a NOT_FOUND collapses into the case's own answer. A store
		// failure goes back unchanged so the handler's fail maps and logs it.
		var ce *dashcontract.Error
		if errors.As(err, &ce) && ce.Code == dashcontract.CodeNotFound {
			return nil, notFound("case not found")
		}
		return nil, err
	}
	return tc, nil
}

func cleanTags(in []string) []string {
	out := []string{}
	for _, t := range in {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// validScorers checks every scorer can be built with its config, so a case
// is never stored with a scorer that would make every run of it an error.
func (d Deps) validScorers(in []scorerConfigInput) ([]testcase.ScorerConfig, error) {
	out := make([]testcase.ScorerConfig, 0, len(in))
	for _, sc := range in {
		if _, err := d.Engine.Scorers().Get(sc.Name, sc.Config); err != nil {
			return nil, badRequest(fmt.Sprintf("scorer %q: %v", sc.Name, err))
		}
		out = append(out, testcase.ScorerConfig{Name: sc.Name, Config: sc.Config})
	}
	return out, nil
}

func validScenario(s string) (testcase.ScenarioType, error) {
	if s == "" {
		return testcase.ScenarioStandard, nil
	}
	st := testcase.ScenarioType(s)
	if !scenarioTypes[st] {
		return "", badRequest(fmt.Sprintf("unknown scenario type %q", s))
	}
	return st, nil
}

func casesListHandler(d Deps) func(context.Context, suiteRef, dashcontract.Principal) (casesListOutput, error) {
	return func(ctx context.Context, in suiteRef, p dashcontract.Principal) (casesListOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return casesListOutput{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return casesListOutput{}, d.fail("cases.list", err)
		}
		list, err := d.Engine.ListCases(ctx, s.ID)
		if err != nil {
			return casesListOutput{}, d.fail("cases.list", err)
		}
		out := casesListOutput{Items: make([]CaseView, 0, len(list))}
		for _, tc := range list {
			out.Items = append(out.Items, caseView(tc))
		}
		return out, nil
	}
}

func casesDetailHandler(d Deps) func(context.Context, caseRef, dashcontract.Principal) (CaseView, error) {
	return func(ctx context.Context, in caseRef, p dashcontract.Principal) (CaseView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return CaseView{}, err
		}
		tc, err := d.caseInApp(ctx, app, in.CaseID)
		if err != nil {
			return CaseView{}, d.fail("cases.detail", err)
		}
		return caseView(tc), nil
	}
}

func casesCreateHandler(d Deps) func(context.Context, casesCreateInput, dashcontract.Principal) (CaseView, error) {
	return func(ctx context.Context, in casesCreateInput, p dashcontract.Principal) (CaseView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return CaseView{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return CaseView{}, d.fail("cases.create", err)
		}
		if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Input) == "" {
			return CaseView{}, badRequest("a case needs a name and an input")
		}
		st, err := validScenario(in.ScenarioType)
		if err != nil {
			return CaseView{}, err
		}
		scorers, err := d.validScorers(in.Scorers)
		if err != nil {
			return CaseView{}, err
		}
		tc := &testcase.Case{
			Entity: sentinel.NewEntity(), ID: id.NewCaseID(), SuiteID: s.ID, Name: strings.TrimSpace(in.Name), Input: in.Input,
			Expected: in.Expected, ScenarioType: st, Scorers: scorers, Tags: cleanTags(in.Tags),
			Context: map[string]any{}, Metadata: map[string]any{},
		}
		if err := d.Engine.CreateCase(ctx, tc); err != nil {
			return CaseView{}, d.fail("cases.create", err)
		}
		return caseView(tc), nil
	}
}

func casesUpdateHandler(d Deps) func(context.Context, casesUpdateInput, dashcontract.Principal) (CaseView, error) {
	return func(ctx context.Context, in casesUpdateInput, p dashcontract.Principal) (CaseView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return CaseView{}, err
		}
		tc, err := d.caseInApp(ctx, app, in.CaseID)
		if err != nil {
			return CaseView{}, d.fail("cases.update", err)
		}
		// Decided from the case as loaded, before any request field is applied.
		hide := promptHidden(tc)
		if in.Name != nil {
			if strings.TrimSpace(*in.Name) == "" {
				return CaseView{}, badRequest("a case needs a name")
			}
			tc.Name = strings.TrimSpace(*in.Name)
		}
		if in.Input != nil {
			if strings.TrimSpace(*in.Input) == "" {
				return CaseView{}, badRequest("a case needs an input")
			}
			tc.Input = *in.Input
		}
		if in.Expected != nil {
			tc.Expected = *in.Expected
		}
		if in.ScenarioType != nil {
			st, err := validScenario(*in.ScenarioType)
			if err != nil {
				return CaseView{}, err
			}
			tc.ScenarioType = st
		}
		if in.Tags != nil {
			tc.Tags = cleanTags(*in.Tags)
		}
		if in.Scorers != nil {
			submitted := *in.Scorers
			// The client only ever saw a red-team substring redacted. A
			// submitted not_contains config without one keeps the stored
			// substring rather than wiping the check.
			if hide {
				stored := ""
				for _, sc := range tc.Scorers {
					if s, ok := sc.Config["substring"].(string); ok && sc.Name == "not_contains" {
						stored = s
					}
				}
				for i := range submitted {
					if submitted[i].Name == "not_contains" {
						if _, has := submitted[i].Config["substring"]; !has && stored != "" {
							if submitted[i].Config == nil {
								submitted[i].Config = map[string]any{}
							}
							submitted[i].Config["substring"] = stored
						}
					}
				}
			}
			scorers, err := d.validScorers(submitted)
			if err != nil {
				return CaseView{}, err
			}
			tc.Scorers = scorers
		}
		if err := d.Engine.UpdateCase(ctx, tc); err != nil {
			return CaseView{}, d.fail("cases.update", err)
		}
		return caseView(tc), nil
	}
}

func casesDeleteHandler(d Deps) func(context.Context, caseRef, dashcontract.Principal) (casesDeleteOutput, error) {
	return func(ctx context.Context, in caseRef, p dashcontract.Principal) (casesDeleteOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return casesDeleteOutput{}, err
		}
		tc, err := d.caseInApp(ctx, app, in.CaseID)
		if err != nil {
			return casesDeleteOutput{}, d.fail("cases.delete", err)
		}
		if err := d.Engine.DeleteCase(ctx, tc.ID); err != nil {
			return casesDeleteOutput{}, d.fail("cases.delete", err)
		}
		return casesDeleteOutput{CaseID: tc.ID.String()}, nil
	}
}

func casesImportHandler(d Deps) func(context.Context, casesImportInput, dashcontract.Principal) (casesImportOutput, error) {
	return func(ctx context.Context, in casesImportInput, p dashcontract.Principal) (casesImportOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return casesImportOutput{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return casesImportOutput{}, d.fail("cases.import", err)
		}
		if len(in.Data) > maxImportBytes {
			return casesImportOutput{}, badRequest(fmt.Sprintf("import data is larger than %d bytes", maxImportBytes))
		}
		n, err := d.Engine.ImportCases(ctx, s.ID, in.Format, []byte(in.Data))
		if err != nil {
			return casesImportOutput{}, d.fail("cases.import", err)
		}
		return casesImportOutput{Imported: n}, nil
	}
}
