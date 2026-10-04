package contract

import (
	"context"
	"errors"
	"strings"

	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/suite"
)

// SuiteView is a suite as the dashboard shows it. SystemPrompt is the
// suite's own prompt; PromptSource says which prompt runs actually send:
// "version" when a prompt version is current, otherwise "suite".
type SuiteView struct {
	ID                   string       `json:"id"`
	Name                 string       `json:"name"`
	Description          string       `json:"description"`
	Model                string       `json:"model"`
	Temperature          float64      `json:"temperature"`
	PersonaRef           string       `json:"personaRef,omitempty"`
	SystemPrompt         string       `json:"systemPrompt"`
	PromptSource         string       `json:"promptSource"`
	CurrentPromptVersion *VersionRef  `json:"currentPromptVersion,omitempty"`
	CurrentBaseline      *BaselineRef `json:"currentBaseline,omitempty"`
	CaseCount            int64        `json:"caseCount"`
	CreatedAt            string       `json:"createdAt"`
	UpdatedAt            string       `json:"updatedAt"`
}

// VersionRef names a prompt version.
type VersionRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

// BaselineRef names a baseline with the pass rate a trend line draws.
type BaselineRef struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	PassRate float64 `json:"passRate"`
}

type suiteRef struct {
	SuiteID string `json:"suiteId"`
}

type suitesListInput struct{}

type suitesListOutput struct {
	Items []SuiteView `json:"items"`
}

type suitesCreateInput struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Model        string   `json:"model"`
	Temperature  *float64 `json:"temperature"`
	PersonaRef   string   `json:"personaRef"`
	SystemPrompt string   `json:"systemPrompt"`
}

// suitesUpdateInput uses pointers so an omitted field is left alone and an
// empty string clears it.
type suitesUpdateInput struct {
	SuiteID      string   `json:"suiteId"`
	Name         *string  `json:"name"`
	Description  *string  `json:"description"`
	Model        *string  `json:"model"`
	Temperature  *float64 `json:"temperature"`
	PersonaRef   *string  `json:"personaRef"`
	SystemPrompt *string  `json:"systemPrompt"`
}

type suitesDeleteOutput struct {
	SuiteID string `json:"suiteId"`
}

// suiteInApp loads a suite and answers NOT_FOUND for a malformed id, a
// missing suite and another app's suite alike, so an id from another app
// cannot be told apart from one that does not exist.
func (d Deps) suiteInApp(ctx context.Context, app, rawID string) (*suite.Suite, error) {
	sid, err := id.ParseSuiteID(rawID)
	if err != nil {
		return nil, notFound("suite not found")
	}
	s, err := d.Engine.GetSuite(ctx, sid)
	if errors.Is(err, sentinel.ErrSuiteNotFound) {
		return nil, notFound("suite not found")
	}
	if err != nil {
		return nil, err
	}
	if s.AppID != app {
		return nil, notFound("suite not found")
	}
	return s, nil
}

func (d Deps) suiteView(ctx context.Context, s *suite.Suite) (SuiteView, error) {
	n, err := d.Engine.CountCases(ctx, s.ID)
	if err != nil {
		return SuiteView{}, err
	}
	v := SuiteView{
		ID: s.ID.String(), Name: s.Name, Description: s.Description, Model: s.Model, Temperature: s.Temperature,
		PersonaRef: s.PersonaRef, SystemPrompt: s.SystemPrompt, PromptSource: "suite", CaseCount: n,
		CreatedAt: ts(s.CreatedAt), UpdatedAt: ts(s.UpdatedAt),
	}
	pv, err := d.Engine.GetCurrentPromptVersion(ctx, s.ID)
	switch {
	case err == nil:
		v.PromptSource = "version"
		v.CurrentPromptVersion = &VersionRef{ID: pv.ID.String(), Version: pv.Version}
	case !errors.Is(err, sentinel.ErrPromptVersionNotFound):
		return SuiteView{}, err
	}
	b, err := d.Engine.GetLatestBaseline(ctx, s.ID)
	switch {
	case err == nil:
		v.CurrentBaseline = &BaselineRef{ID: b.ID.String(), Name: b.Name, PassRate: b.PassRate}
	case !errors.Is(err, sentinel.ErrBaselineNotFound):
		return SuiteView{}, err
	}
	return v, nil
}

func validTemperature(t *float64) error {
	if t != nil && (*t < 0 || *t > 2) {
		return badRequest("temperature must be between 0 and 2")
	}
	return nil
}

func suitesListHandler(d Deps) func(context.Context, suitesListInput, dashcontract.Principal) (suitesListOutput, error) {
	return func(ctx context.Context, _ suitesListInput, p dashcontract.Principal) (suitesListOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return suitesListOutput{}, err
		}
		list, err := d.Engine.ListSuites(ctx, &suite.ListFilter{AppID: app})
		if err != nil {
			return suitesListOutput{}, d.fail("suites.list", err)
		}
		out := suitesListOutput{Items: make([]SuiteView, 0, len(list))}
		for _, s := range list {
			v, err := d.suiteView(ctx, s)
			if err != nil {
				return suitesListOutput{}, d.fail("suites.list", err)
			}
			out.Items = append(out.Items, v)
		}
		return out, nil
	}
}

func suitesDetailHandler(d Deps) func(context.Context, suiteRef, dashcontract.Principal) (SuiteView, error) {
	return func(ctx context.Context, in suiteRef, p dashcontract.Principal) (SuiteView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return SuiteView{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return SuiteView{}, d.fail("suites.detail", err)
		}
		v, err := d.suiteView(ctx, s)
		if err != nil {
			return SuiteView{}, d.fail("suites.detail", err)
		}
		return v, nil
	}
}

func suitesCreateHandler(d Deps) func(context.Context, suitesCreateInput, dashcontract.Principal) (SuiteView, error) {
	return func(ctx context.Context, in suitesCreateInput, p dashcontract.Principal) (SuiteView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return SuiteView{}, err
		}
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return SuiteView{}, badRequest("a suite needs a name")
		}
		if terr := validTemperature(in.Temperature); terr != nil {
			return SuiteView{}, terr
		}
		if _, lerr := d.Engine.GetSuiteByName(ctx, app, name); lerr == nil {
			return SuiteView{}, conflict("a suite with this name already exists")
		} else if !errors.Is(lerr, sentinel.ErrSuiteNotFound) {
			return SuiteView{}, d.fail("suites.create", lerr)
		}
		s := &suite.Suite{
			Entity: sentinel.NewEntity(), ID: id.NewSuiteID(), Name: name, Description: in.Description, AppID: app,
			SystemPrompt: in.SystemPrompt, Model: in.Model, PersonaRef: in.PersonaRef, Metadata: map[string]any{},
		}
		if in.Temperature != nil {
			s.Temperature = *in.Temperature
		}
		if cerr := d.Engine.CreateSuite(ctx, s); cerr != nil {
			return SuiteView{}, d.fail("suites.create", cerr)
		}
		v, err := d.suiteView(ctx, s)
		if err != nil {
			return SuiteView{}, d.fail("suites.create", err)
		}
		return v, nil
	}
}

func suitesUpdateHandler(d Deps) func(context.Context, suitesUpdateInput, dashcontract.Principal) (SuiteView, error) {
	return func(ctx context.Context, in suitesUpdateInput, p dashcontract.Principal) (SuiteView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return SuiteView{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return SuiteView{}, d.fail("suites.update", err)
		}
		if terr := validTemperature(in.Temperature); terr != nil {
			return SuiteView{}, terr
		}
		if in.Name != nil {
			name := strings.TrimSpace(*in.Name)
			if name == "" {
				return SuiteView{}, badRequest("a suite needs a name")
			}
			if name != s.Name {
				if other, lerr := d.Engine.GetSuiteByName(ctx, app, name); lerr == nil && other.ID.String() != s.ID.String() {
					return SuiteView{}, conflict("a suite with this name already exists")
				} else if lerr != nil && !errors.Is(lerr, sentinel.ErrSuiteNotFound) {
					return SuiteView{}, d.fail("suites.update", lerr)
				}
			}
			s.Name = name
		}
		if in.Description != nil {
			s.Description = *in.Description
		}
		if in.Model != nil {
			s.Model = *in.Model
		}
		if in.Temperature != nil {
			s.Temperature = *in.Temperature
		}
		if in.PersonaRef != nil {
			s.PersonaRef = *in.PersonaRef
		}
		if in.SystemPrompt != nil {
			s.SystemPrompt = *in.SystemPrompt
		}
		if uerr := d.Engine.UpdateSuite(ctx, s); uerr != nil {
			return SuiteView{}, d.fail("suites.update", uerr)
		}
		v, err := d.suiteView(ctx, s)
		if err != nil {
			return SuiteView{}, d.fail("suites.update", err)
		}
		return v, nil
	}
}

func suitesDeleteHandler(d Deps) func(context.Context, suiteRef, dashcontract.Principal) (suitesDeleteOutput, error) {
	return func(ctx context.Context, in suiteRef, p dashcontract.Principal) (suitesDeleteOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return suitesDeleteOutput{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return suitesDeleteOutput{}, d.fail("suites.delete", err)
		}
		if err := d.Engine.DeleteSuite(ctx, s.ID); err != nil {
			return suitesDeleteOutput{}, d.fail("suites.delete", err)
		}
		return suitesDeleteOutput{SuiteID: s.ID.String()}, nil
	}
}
