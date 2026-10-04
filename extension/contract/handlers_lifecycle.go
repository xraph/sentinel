package contract

import (
	"context"
	"fmt"
	"sort"

	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"

	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/redteam"
)

type runsStartInput struct {
	SuiteID string   `json:"suiteId"`
	Target  string   `json:"target"`
	Scorers []string `json:"scorers"`
	Model   string   `json:"model"`
}

type redteamGenerateInput struct {
	SuiteID     string   `json:"suiteId"`
	AttackTypes []string `json:"attackTypes"`
	Count       int      `json:"count"`
}

type redteamGenerateOutput struct {
	Created int `json:"created"`
	Cap     int `json:"cap"`
}

// RedTeamTally is one attack type's outcome in a run. A bypass is a result
// that failed its scoring; unscored counts results that errored.
type RedTeamTally struct {
	AttackType string `json:"attackType"`
	Total      int    `json:"total"`
	Bypassed   int    `json:"bypassed"`
	Unscored   int    `json:"unscored"`
}

// RedTeamReport is built when read from a run's results and its cases.
// JudgedBy is the sorted, deduplicated union of the scorers the run
// recorded and the scorers carried by every red-team case that has a result
// in the run (the engine runs a case's own scorers on top of the run's).
// Three of the five generators attach no scorer of their own, so the rate
// means only what those scorers can detect. JudgedBy and ByType are never
// null.
type RedTeamReport struct {
	JudgedBy []string       `json:"judgedBy"`
	ByType   []RedTeamTally `json:"byType"`
	Total    int            `json:"total"`
	Bypassed int            `json:"bypassed"`
	Unscored int            `json:"unscored"`
}

func runsStartHandler(d Deps) func(context.Context, runsStartInput, dashcontract.Principal) (RunView, error) {
	return func(ctx context.Context, in runsStartInput, p dashcontract.Principal) (RunView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return RunView{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return RunView{}, d.fail("runs.start", err)
		}
		run, err := d.Engine.StartRun(ctx, &engine.StartConfig{SuiteID: s.ID, Target: in.Target, Scorers: in.Scorers, Model: in.Model})
		if err != nil {
			return RunView{}, d.fail("runs.start", err)
		}
		v, err := d.runView(ctx, run, s.Name)
		if err != nil {
			return RunView{}, d.fail("runs.start", err)
		}
		return v, nil
	}
}

func runsCancelHandler(d Deps) func(context.Context, runRef, dashcontract.Principal) (RunView, error) {
	return func(ctx context.Context, in runRef, p dashcontract.Principal) (RunView, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return RunView{}, err
		}
		r, err := d.runInApp(ctx, app, in.RunID)
		if err != nil {
			return RunView{}, d.fail("runs.cancel", err)
		}
		if err = d.Engine.CancelRun(ctx, r.ID); err != nil {
			return RunView{}, d.fail("runs.cancel", err)
		}
		fresh, err := d.Engine.GetRun(ctx, r.ID)
		if err != nil {
			return RunView{}, d.fail("runs.cancel", err)
		}
		names, err := d.suiteNames(ctx, app)
		if err != nil {
			return RunView{}, d.fail("runs.cancel", err)
		}
		v, err := d.runView(ctx, fresh, names[fresh.SuiteID.String()])
		if err != nil {
			return RunView{}, d.fail("runs.cancel", err)
		}
		return v, nil
	}
}

func redteamGenerateHandler(d Deps) func(context.Context, redteamGenerateInput, dashcontract.Principal) (redteamGenerateOutput, error) {
	return func(ctx context.Context, in redteamGenerateInput, p dashcontract.Principal) (redteamGenerateOutput, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return redteamGenerateOutput{}, err
		}
		s, err := d.suiteInApp(ctx, app, in.SuiteID)
		if err != nil {
			return redteamGenerateOutput{}, d.fail("redteam.generate", err)
		}
		if in.Count < 1 || in.Count > redteam.MaxPerType {
			return redteamGenerateOutput{}, badRequest(fmt.Sprintf("count must be between 1 and %d, the number of templates each attack type has", redteam.MaxPerType))
		}
		seen := map[string]bool{}
		var types []redteam.AttackType
		for _, t := range in.AttackTypes {
			if !seen[t] {
				seen[t] = true
				types = append(types, redteam.AttackType(t))
			}
		}
		cases, err := d.Engine.GenerateRedTeam(ctx, s.ID, types, in.Count)
		if err != nil {
			return redteamGenerateOutput{}, d.fail("redteam.generate", err)
		}
		return redteamGenerateOutput{Created: len(cases), Cap: redteam.MaxPerType}, nil
	}
}

// redteamReportHandler answers null only when the run's suite has no
// red-team case at all. A suite that has red-team cases but whose run has no
// result for any of them yet answers a report with zero totals and an empty
// ByType.
func redteamReportHandler(d Deps) func(context.Context, runRef, dashcontract.Principal) (*RedTeamReport, error) {
	return func(ctx context.Context, in runRef, p dashcontract.Principal) (*RedTeamReport, error) {
		app, err := d.resolveApp(p)
		if err != nil {
			return nil, err
		}
		r, err := d.runInApp(ctx, app, in.RunID)
		if err != nil {
			return nil, d.fail("redteam.report", err)
		}
		results, err := d.Engine.ListResults(ctx, r.ID)
		if err != nil {
			return nil, d.fail("redteam.report", err)
		}
		cases, err := d.caseIndex(ctx, r.SuiteID)
		if err != nil {
			return nil, d.fail("redteam.report", err)
		}
		hasRedTeam := false
		for _, tc := range cases {
			if attackTypeOf(tc) != "" {
				hasRedTeam = true
				break
			}
		}
		if !hasRedTeam {
			return nil, nil
		}
		judges := map[string]bool{}
		for _, n := range evalrun.SettingsFrom(r.Config).Scorers {
			judges[n] = true
		}
		tallies := map[string]*RedTeamTally{}
		report := &RedTeamReport{ByType: []RedTeamTally{}}
		for _, res := range results {
			tc := cases[res.CaseID.String()]
			if tc == nil {
				continue // the case was deleted after the run
			}
			at := attackTypeOf(tc)
			if at == "" {
				continue
			}
			for _, sc := range tc.Scorers {
				judges[sc.Name] = true
			}
			tl := tallies[at]
			if tl == nil {
				tl = &RedTeamTally{AttackType: at}
				tallies[at] = tl
			}
			tl.Total++
			report.Total++
			switch res.Status {
			case evalrun.StatusFail:
				tl.Bypassed++
				report.Bypassed++
			case evalrun.StatusError:
				tl.Unscored++
				report.Unscored++
			}
		}
		report.JudgedBy = make([]string, 0, len(judges))
		for n := range judges {
			report.JudgedBy = append(report.JudgedBy, n)
		}
		sort.Strings(report.JudgedBy)
		for _, tl := range tallies {
			report.ByType = append(report.ByType, *tl)
		}
		sort.Slice(report.ByType, func(i, j int) bool { return report.ByType[i].AttackType < report.ByType[j].AttackType })
		return report, nil
	}
}
