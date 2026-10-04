package contract

import (
	"context"
	"errors"
	"strings"

	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/promptversion"
	"github.com/xraph/sentinel/suite"
)

// PromptVersionView is one version of a suite's prompt. RunCount and
// LatestPassRate come from the runs that recorded this version, not from
// the never-written columns on the version row.
type PromptVersionView struct {
	ID             string   `json:"id"`
	SuiteID        string   `json:"suiteId"`
	Version        int      `json:"version"`
	SystemPrompt   string   `json:"systemPrompt"`
	Changelog      string   `json:"changelog,omitempty"`
	IsCurrent      bool     `json:"isCurrent"`
	RunCount       int      `json:"runCount"`
	LatestPassRate *float64 `json:"latestPassRate,omitempty"`
	CreatedAt      string   `json:"createdAt"`
}

type versionRef struct {
	VersionID string `json:"versionId"`
}

type promptsListOutput struct {
	Items []PromptVersionView `json:"items"`
}

type promptsDetailOutput struct {
	PromptVersionView
	Previous *PromptVersionView `json:"previous,omitempty"`
}

type promptsCreateInput struct {
	SuiteID      string `json:"suiteId"`
	SystemPrompt string `json:"systemPrompt"`
	Changelog    string `json:"changelog"`
	MakeCurrent  bool   `json:"makeCurrent"`
}

type promptsSetCurrentInput struct {
	SuiteID   string `json:"suiteId"`
	VersionID string `json:"versionId"`
}

type versionStats struct {
	runs     int
	latest   *float64
	latestAt int64
}

// statsByVersion groups the suite's runs by the prompt version they
// recorded. The latest pass rate is the newest completed run's.
func (d Deps) statsByVersion(ctx context.Context, suiteID id.SuiteID) (map[string]*versionStats, error) {
	runs, err := d.Engine.ListRunsBySuite(ctx, suiteID)
	if err != nil {
		return nil, err
	}
	out := map[string]*versionStats{}
	for _, r := range runs {
		pv := evalrun.SettingsFrom(r.Config).PromptVersionID
		if pv == "" {
			continue
		}
		st := out[pv]
		if st == nil {
			st = &versionStats{}
			out[pv] = st
		}
		st.runs++
		if r.State == evalrun.StateCompleted && r.CreatedAt.UnixNano() > st.latestAt {
			rate := r.PassRate
			st.latest, st.latestAt = &rate, r.CreatedAt.UnixNano()
		}
	}
	return out, nil
}

func versionView(pv *promptversion.PromptVersion, stats map[string]*versionStats) PromptVersionView {
	v := PromptVersionView{
		ID: pv.ID.String(), SuiteID: pv.SuiteID.String(), Version: pv.Version, SystemPrompt: pv.SystemPrompt,
		Changelog: pv.Changelog, IsCurrent: pv.IsCurrent, CreatedAt: ts(pv.CreatedAt),
	}
	if st := stats[v.ID]; st != nil {
		v.RunCount, v.LatestPassRate = st.runs, st.latest
	}
	return v
}

// versionInApp loads a prompt version through its suite and answers
// NOT_FOUND for a malformed id, a missing version and another app's alike.
func (d Deps) versionInApp(ctx context.Context, app, rawID string) (*promptversion.PromptVersion, *suite.Suite, error) {
	vid, err := id.ParsePromptVersionID(rawID)
	if err != nil {
		return nil, nil, notFound("prompt version not found")
	}
	pv, err := d.Engine.GetPromptVersion(ctx, vid)
	if errors.Is(err, sentinel.ErrPromptVersionNotFound) {
		return nil, nil, notFound("prompt version not found")
	}
	if err != nil {
		return nil, nil, err
	}
	s, err := d.suiteInApp(ctx, app, pv.SuiteID.String())
	if err != nil {
		return nil, nil, notFound("prompt version not found")
	}
	return pv, s, nil
}

func promptsListHandler(d Deps) func(context.Context, suiteRef, dashcontract.Principal) (promptsListOutput, error) {
	return func(ctx context.Context, in suiteRef, p dashcontract.Principal) (promptsListOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return promptsListOutput{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return promptsListOutput{}, d.fail("prompts.list", err)
		}
		list, err := d.Engine.ListPromptVersions(ctx, s.ID)
		if err != nil {
			return promptsListOutput{}, d.fail("prompts.list", err)
		}
		stats, err := d.statsByVersion(ctx, s.ID)
		if err != nil {
			return promptsListOutput{}, d.fail("prompts.list", err)
		}
		out := promptsListOutput{Items: make([]PromptVersionView, 0, len(list))}
		for _, pv := range list {
			out.Items = append(out.Items, versionView(pv, stats))
		}
		return out, nil
	}
}

func promptsDetailHandler(d Deps) func(context.Context, versionRef, dashcontract.Principal) (promptsDetailOutput, error) {
	return func(ctx context.Context, in versionRef, p dashcontract.Principal) (promptsDetailOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return promptsDetailOutput{}, err
		}
		pv, s, err := d.versionInApp(ctx, app, in.VersionID)
		if err != nil {
			return promptsDetailOutput{}, d.fail("prompts.detail", err)
		}
		list, err := d.Engine.ListPromptVersions(ctx, s.ID)
		if err != nil {
			return promptsDetailOutput{}, d.fail("prompts.detail", err)
		}
		stats, err := d.statsByVersion(ctx, s.ID)
		if err != nil {
			return promptsDetailOutput{}, d.fail("prompts.detail", err)
		}
		out := promptsDetailOutput{PromptVersionView: versionView(pv, stats)}
		var prev *promptversion.PromptVersion
		for _, other := range list {
			if other.Version < pv.Version && (prev == nil || other.Version > prev.Version) {
				prev = other
			}
		}
		if prev != nil {
			pvView := versionView(prev, stats)
			out.Previous = &pvView
		}
		return out, nil
	}
}

func promptsCreateHandler(d Deps) func(context.Context, promptsCreateInput, dashcontract.Principal) (PromptVersionView, error) {
	return func(ctx context.Context, in promptsCreateInput, p dashcontract.Principal) (PromptVersionView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return PromptVersionView{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return PromptVersionView{}, d.fail("prompts.create", err)
		}
		if strings.TrimSpace(in.SystemPrompt) == "" {
			return PromptVersionView{}, badRequest("a prompt version needs a system prompt")
		}
		pv := &promptversion.PromptVersion{SuiteID: s.ID, SystemPrompt: in.SystemPrompt, Changelog: in.Changelog, IsCurrent: in.MakeCurrent}
		if err := d.Engine.CreatePromptVersion(ctx, pv); err != nil {
			return PromptVersionView{}, d.fail("prompts.create", err)
		}
		return versionView(pv, nil), nil
	}
}

func promptsSetCurrentHandler(d Deps) func(context.Context, promptsSetCurrentInput, dashcontract.Principal) (PromptVersionView, error) {
	return func(ctx context.Context, in promptsSetCurrentInput, p dashcontract.Principal) (PromptVersionView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return PromptVersionView{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return PromptVersionView{}, d.fail("prompts.setCurrent", err)
		}
		pv, _, err := d.versionInApp(ctx, app, in.VersionID)
		if err != nil {
			return PromptVersionView{}, d.fail("prompts.setCurrent", err)
		}
		if err := d.Engine.SetCurrentPromptVersion(ctx, s.ID, pv.ID); err != nil {
			return PromptVersionView{}, d.fail("prompts.setCurrent", err)
		}
		pv.IsCurrent = true
		return versionView(pv, nil), nil
	}
}
