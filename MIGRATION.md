# Dashboard migration: templ to React shell

Sentinel's dashboard used to render server-side with templ and ForgeUI, from
`dashboard/`. It now lives in the Forge dashboard's React shell as
`@forge-go/dashboard-plugin-sentinel`, reading the `sentinel` contract
contributor in `extension/contract`. The commit after this file deletes the
templ package.

This file is the record of that move. We wrote it from an inventory of every
templ source, taken before anything changed: 31 `.templ` files (17 pages, a
helpers file, 10 components, 2 widgets and a settings panel), plus
`contributor.go`, `data.go`, `manifest.go`, `pages/form_helpers.go` and the
`shared` package. Once the directory is gone there's nothing left to check
against, so anything missing from here is a feature that went missing by
accident. Every page, column, action, form field, filter, badge, empty state,
widget and setting is listed below, and each one says whether it moved,
changed, or was dropped, and why.

The cut happened in two steps. Commit 21dca4b stopped the extension from
registering the templ dashboard, so nothing has served those pages since. The
commit that follows this file deletes `dashboard/` and the templ and ForgeUI
requirements with it.

## What you need to do

If you ran the templ dashboard through `DashboardAware`, you don't need to
change anything in Sentinel's code. The extension no longer implements
`DashboardAware` and has no `DashboardContributor()` method. It registers its
contract contributor through `ContractContributorAware`, and the shell finds
it. If your own code imports `github.com/xraph/sentinel/dashboard` (its
widgets, say), that import has to go: the package no longer exists.

Add the plugin to your shell:

```tsx
import sentinelPlugin from "@forge-go/dashboard-plugin-sentinel"

const plugins = [corePlugin, sentinelPlugin]
```

The pages mount at `/@sentinel`, under a nav group called Evaluation. They use
Tailwind classes of their own, so the shell's stylesheet has to scan the
package. If your shell declares its sources with `@source`, add
`@source "<path to>/packages/plugin-sentinel/src";` next to the others.

Tell Sentinel which app the dashboard works in:

```yaml
extensions:
  sentinel:
    dashboard_app_id: app_yourapp
```

The dashboard reads and writes inside one app. It takes the app from the
signed-in principal's `app_id` claim first, and nothing sets that claim yet, so
today it comes from `dashboard_app_id`. There's no default. With neither, every
dashboard request is refused, because an empty app id matches every app in
every store, and a dashboard that can see every tenant's suites is worse than
one that can't start.

Register at least one target if you want to start runs from the dashboard:

```go
sentinelext.New(
    sentinelext.WithTarget("support-bot", "The support assistant under test", myTarget),
)
```

A run sends each case to a target and scores the answer. The dashboard lists
what you registered, and with none it says so and links to Setup instead of
offering a button. Scorers you write yourself go in the same way, with
`WithScorer`, so the run dialog can offer them.

Sentinel needs forge v1.11.2 or later, because that's the first forge whose
dashboard transport passes a manifest's `invalidates` to the client, so a
write refreshes the pages that read what it changed. We build against v1.12.0,
which also stops pulling in templ and ForgeUI. The REST API under `base_path` is unchanged and
still mounts unless you set `disable_routes`.

## What changed underneath

The templ pages sat on an engine that had several bugs of its own, and the new
pages would have shown them faithfully. We fixed them first, in the library,
before writing a single page. If you use the engine directly, these matter more
than the UI does.

- Your config is applied. The extension merged YAML and code config and
  then never passed it to the engine, so every engine ran on the defaults
  (model "smart", temperature 0, pass threshold 0.7, concurrency 4). It passes
  the merged config now.
- Runs start asynchronously and store results per case. A run used to hold
  the request open and write every result at the end, so a running run looked
  like a 0% failure until it finished. Results land as each case is scored,
  and a run in flight can be cancelled, from any replica.
- A run records the settings it was scored with: pass threshold,
  regression threshold, concurrency, target, scorers, model and prompt
  version. Comparing two runs compares like with like, and a run's verdict
  does not move when you change the config later.
- Regression is checked when a run completes, against the suite's current
  baseline, with one `regression_threshold` (default 0.05) recorded on the run.
  It reports missing and new cases and dimensions, and an unmeasured dimension
  counts as a drop.
- Deleting a suite deletes its cases, runs, results, baselines and prompt
  versions on every backend. The old delete dialog promised this. Memory and
  Mongo removed only the suite row, and the cascade on Postgres and SQLite
  was not reliable.
- Prompt versions get numbers, and making one current stays inside its
  suite.
- Case import imports. It was a stub returning zero in all four stores.
- Red-team generation writes cases into a suite, and a run's red-team
  report counts bypasses by attack type.
- Targets and scorers are registered by name, so a run can name them, and
  a scorer that needs config refuses to build without it.
- Targets get the run's prompt, model and temperature, not the ones the
  target was built with.
- SQLite no longer loses concurrent writes, and Postgres and Mongo store
  empty JSON for nil maps instead of failing.

## Where each surface went

Every route below sits under `/@sentinel` in the shell. The intents are the
contract calls the page makes.

| templ surface | templ route | Now | Intents |
|---|---|---|---|
| Overview | `/` | Overview, `/` | `overview.stats` |
| Suites list | `/suites` | Suites, `/suites` | `suites.list`, `suites.create` |
| Suite form | `/suites/create`, `/suites/edit` | Create and Edit dialogs on the list and detail pages | `suites.create`, `suites.update` |
| Suite detail | `/suites/detail` | Suite detail, `/suites/:id`, with tabs | `suites.detail`, `suites.update`, `suites.delete` |
| Cases list | `/suites/cases` | The suite's Cases tab, `/suites/:id` | `cases.list`, `cases.create`, `cases.import` |
| Case form | `/suites/cases/create`, `/suites/cases/edit` | Add and Edit dialogs | `cases.create`, `cases.update`, `config.get` |
| Case detail | `/suites/cases/detail` | Case detail, `/suites/:id/cases/:caseId` | `cases.detail`, `cases.update`, `cases.delete` |
| Prompt versions list | `/prompts` | The suite's Prompts tab, `/suites/:id/prompts` | `prompts.list`, `prompts.create`, `prompts.setCurrent` |
| Prompt version form | `/prompts/create` | New version dialog | `prompts.create` |
| Prompt version detail | `/prompts/detail` | Version page with a diff, `/suites/:id/prompts/:versionId` | `prompts.detail`, `prompts.setCurrent` |
| Runs list | `/runs` | Runs, `/runs` | `runs.list`, `suites.list` |
| (none) | | The suite's Runs tab, `/suites/:id/runs`, with the trend and Start run | `runs.list`, `runs.trend`, `runs.start`, `config.get` |
| Run detail | `/runs/detail` | Run detail, `/runs/:id` | `runs.detail`, `runs.results`, `runs.regression`, `runs.cancel`, `baselines.save`, `baselines.detail`, `baselines.list`, `runs.list`, `redteam.report` |
| Run report | `/runs/report` | Folded into run detail | as run detail |
| Result detail | `/runs/results/detail` | Result detail, `/runs/:id/results/:resultId` | `results.detail`, `runs.detail`, `cases.detail` |
| (none) | | Comparison, `/runs/:id/compare/:otherId` | `runs.compare`, `results.detail` |
| Baselines list | `/baselines` | Baselines, `/baselines`, and the suite's Baselines tab | `baselines.list`, `baselines.delete` |
| Baseline detail | `/baselines/detail` | Baseline detail, `/baselines/:id` | `baselines.detail`, `baselines.delete` |
| Scorers reference | `/scorers` | Setup, `/setup` | `config.get` |
| (none) | | The suite's Red team tab, `/suites/:id/redteam` | `redteam.generate`, `redteam.report`, `cases.list` |
| `sentinel-stats` widget | | Overview counts | `overview.stats` |
| `sentinel-recent-runs` widget | | Overview recent runs | `overview.stats` |
| `sentinel-config` settings panel | | Setup | `config.get` |

## Page by page

### Overview

- The four stat cards become three counts: Suites, Cases and Runs.
- Avg Pass Rate is dropped. It was the unweighted mean of every completed
  run in history, and it showed "0.0%" when there were none, which reads as a
  failure. A number averaged across unrelated suites does not answer a question
  anyone asks, and the overview leads with the ones they do: is a target
  registered, what is running, and what regressed.
- New: a notice when no target is registered, with a link to Setup, above
  everything else. Without a target no run can start.
- Recent Runs keeps suite, state and pass rate, and adds the run id, cases
  scored, passed, errored, the cost the target reported and the start time.
  Model moved to the run page. Times are local and include the date.
- Active Runs shows a progress meter and "X of Y" from the cases actually
  scored. The old Progress column was always "0/N cases", because results only
  existed at the end.
- New: Recent regressions. Regressed runs among the twenty newest
  completed ones, with the baseline they fell against and their worst drop.
- The page refreshes every three seconds while a run is active.
- The "View All Suites" and "View All Runs" buttons are gone; the nav has
  both, and the recent runs table has "Every run".

### Suites list

- Name and Model stay. Model says "Engine default" when the suite sets
  none. Description, Temperature and Persona left the list; they are on the
  suite page.
- New columns: the case count, the prompt in use (the suite's own, or the
  current version by number), the current baseline's name, and when the suite
  last changed.
- Search is dropped. It filtered only the page it was on and left the count
  unfiltered. The list comes back whole, oldest first.
- Pagination is dropped. It was done in memory over the full list anyway.
- New Suite opens a dialog instead of a page. View and Edit become the row's
  name link and the detail page's Edit.
- Delete moved to the suite page, behind a confirm that lists what goes with
  the suite. That is now true; it was not before (see What changed underneath).

### Suite form

- Name, Description, Model, Temperature, Persona and System prompt stay, in a
  dialog. Edit prefills every field and sends every field, so nothing you
  did not touch changes.
- Temperature keeps its value. The old edit form rounded it to one decimal
  on prefill, so saving 0.75 quietly stored 0.8. An empty temperature now means
  "use the engine's".
- The form refuses a blank name and an out-of-range temperature before
  sending, in the server's words, and shows a refusal inside the dialog
  instead of swapping JSON into an error div.

### Suite detail

- The header keeps the name, description and Edit, and gains Delete.
- The stat cards become facts: model, temperature, persona, prompt, current
  baseline with its pass rate, case count, created and updated. Temperature
  says "Engine default" when the suite sets none, which is what the engine now
  does with 0.
- Tabs replace the stacked cards: Cases, Runs, Prompts, Baselines and Red
  team. The tab is in the address, so a link can land on a suite's runs.
- The suite's own system prompt is edited in the Edit dialog. Each version's
  prompt is on its version page.
- New: a Start run button on the Runs tab, with a confirm that names the
  suite, case count, target, model and scorers before anything runs. The old
  page had no way to start a run.
- The page shows no tabs for a suite that never loaded, and keeps its tabs and
  any open dialog if a later refresh fails.

### Cases list

- Now the suite's Cases tab. Name, Scenario, Tags and the case's own scorers
  stay. The scorer count became the scorer names.
- New columns: a one-line input, and Red team with the attack type.
- New: Import cases from JSON, JSONL or CSV, with the count imported.
- Pagination is dropped. It was fake: every case rendered and Next
  reloaded the same list.
- Edit and Delete moved to the case page.

### Case form

- Name, Scenario type, Input and Expected output stay, in a dialog.
- Tags and scorers are editable now. The old form could not touch them.
  Scorers are a list of rows, each a registered scorer with its config.
  Context still is not editable; the case page shows it.
- A red-team case's leakage substring is never shown, only its length, and
  editing the case without retyping it keeps the stored one. The substring is
  the system prompt the case checks for.
- The old create form posted flat fields where the API expected a list, and
  the old edit form had no route at all. Both now work through the contract.

### Case detail

- Scenario, Tags, Input, Expected output and Context stay. Suite ID and Case
  ID give way to a link back to the suite, with created and updated times.
- Metadata is not shown. The contract still sends it, so this is a gap we
  have not closed yet, not a decision.
- Scorers show each one's config as text, with a withheld substring shown as
  "The substring, N characters".
- Delete is new, and Edit now works.

### Prompt versions list

- Now the suite's Prompts tab. Version, Changelog, Current and Created stay,
  with Runs (how many runs used the version).
- Pass rate and Avg score, averaged over every run of the version, become the
  latest completed run's pass rate. Runs of one version can use different
  scorers and thresholds, and an average over them mixes unlike things.
- New: Make current, behind a confirm. The API always had it and nothing
  called it.
- New version opens a dialog. The old `?page=` links meant this list was
  effectively unreachable.

### Prompt version form

- System prompt and Changelog stay. "Set as current version" now works, as
  "Make it current", ticked by default. The old form posted to a path with no
  handler, and the API had no such field.
- The prompt starts as the one runs use today, so you edit it rather than
  retype it.

### Prompt version detail

- The prompt and changelog stay. The stat cards rendered their arguments in
  the wrong slots; the facts are plain now: number, created, runs, latest pass
  rate.
- New: a diff against the version before, loaded only when you open a
  version. The linked run is dropped; a version is used by many runs, and the
  Runs count says how many.

### Runs list

- Suite, State, Pass rate and Started stay, with the run id first.
- The "Cases" column is replaced. It showed passed over total and read like
  progress. Now there is "Cases scored" (X of Y, with a meter while running) and
  a separate Passed column.
- New columns: Errored and the cost the target reported. Model, Avg score
  and Tokens moved to the run page.
- The state and suite filters stay, and paging is now newer and older runs.
- The list refreshes every three seconds while a run on the page is running.
- The Report action folded into the run page.

### Run detail and run report

- The report page folds into run detail: its content was a subset, and its
  stat cards were in the wrong slots.
- New: a verdict band that answers first: regressed against which baseline,
  within threshold, no baseline yet, or why the run is not compared (cancelled,
  failed, another suite). It names the evidence: the pass rate change, regressed
  cases, average score and dimension drops, unmeasured dimensions, missing and
  new cases.
- Pass rate (with passed of scored under it), Avg score, Errored, Tokens and
  Cost stay as stats, from the run's own counters. Failed is the Fail count on
  the results filter. The old page read them from the results and
  showed "of 0 total" for a run that had not stored any yet.
- Latency moved to each result. An average latency across cases with very
  different inputs says little.
- Dimension scores keep their bars, now in a fixed order with the pass
  threshold drawn and unmeasured dimensions named. They used to come out in a
  random order on every render.
- New: change from baseline, each case's score against the baseline the run
  was compared with, worst first, with the stretch past the threshold shaded.
- New: View against. Compare a completed run against another of the
  suite's baselines, or at another threshold, for that view only.
- Results keep Case, Status, Score, Latency, Tokens and Cost, add Change vs
  baseline, and filter by status with a count on each.
- New: Save as baseline, Cancel run and Compare with… The old page had none
  of them.
- New: a red-team section with the bypass rate by attack type and the
  scorers that judged it.
- The run's error, settings (thresholds, concurrency, target, scorers, model,
  prompt version) and duration are on the page. A running run shows a progress
  meter and refreshes every three seconds; there is no verdict until it ends.
- The page used to panic when the result stats failed to load. It does not.

### Result detail

- Score, Latency, Tokens, Cost, the output and the scorer table stay.
- Scorer details are shown. The old page dropped them, and kept only each
  scorer's reason.
- The trace shows every step and tool call in full, not one CSS-truncated
  line per field.
- New: the case's input, read from the case, with a note if the case has
  been deleted since.
- A red-team result keeps its output, trace, tool calls, scorer reasons and
  error collapsed until you ask for them, per result. The output may repeat
  the system prompt or carry the attack.
- Every output, reason and trace field renders as text. Nothing is read as
  markup.

### Comparison (new)

Compare with… on a run opens the comparison, with the older run as A. Scores
come first as dumbbells on one scale (pass rate, average score and each shared
dimension), then latency and cost in words, then every case with its change and
a Changed only filter. Any case's outputs open as a diff; a red-team case's only
after you ask.

### Baselines list

- Name, Suite, Pass rate, Avg score and Cases stay, now across every suite
  and on each suite's Baselines tab. Current is a badge beside the name, and
  Created is called Saved.
- New: From run, a link to the run the baseline was saved from.
- Delete works. The API had no delete route, so the old dialog failed, and
  then could not close itself either. Deleting the current baseline now says
  plainly that nothing takes its place.
- Save is new, from a completed run's page.
- The suite filter is the suite's own tab now. The old filter only worked by
  accident.
- Pagination is dropped. It was fake here too.

### Baseline detail

- Name, Current, the per-case results with status and score, the run's
  dimension scores and the run it came from stay. The run is a link now.
- Each case's own dimension badges are dropped; the case's score stays.
- The stat cards rendered their arguments in the wrong slots; the facts are
  plain now.
- New: Delete, which then returns to the suite's Baselines tab.

### Scorers reference

- Replaced by Setup, which lists the scorers the engine actually has registered,
  with each one's dimension, whether it calls an LLM, and whether it needs
  config of its own. The old page was a hard-coded list of ten, one of which
  (`custom`) was never a registered scorer, and it did not mention the eleven
  programmatic scorers at all. Setup lists the nine built-ins plus whatever
  you register with `WithScorer`.

### Widgets and settings

- The two widgets are the Overview's counts and recent runs. The React shell has
  no widget slots for an extension, and the Overview shows more than both did.
- The settings panel is Setup, showing the effective config: default
  model, temperature, pass threshold, regression threshold and concurrency,
  plus the registered targets. The old panel always showed the defaults,
  because the config never reached the engine, and called itself "Configure
  Sentinel engine behavior" though it was read-only. Setup is read-only too and
  says how to register a target when there are none.

## Dropped, and why

| What | Why |
|---|---|
| Topbar "API Docs" action and sidebar footer link | The shell owns the chrome. |
| `searchable` capability | Declared and never implemented. |
| Suites search | It filtered only the current page and left the count wrong. |
| Pagination on suites, cases and baselines | Suites paged in memory; cases and baselines were fake. Each list comes back whole. Runs page for real. |
| Avg Pass Rate stat | An unweighted all-history mean across unrelated suites, "0.0%" with no data. |
| Run report page | Its content was a subset of run detail. |
| Run detail's average latency card | Latency is per result now. |
| The linked run on a prompt version | A version is used by many runs; the Runs count replaces it. |
| Widgets as widgets | No widget slots in the shell; the Overview covers both. |
| Avg score on the prompt versions list | It averaged runs scored with different settings. |
| Per-case dimension badges on a baseline | The run's dimension scores stay. |
| Case metadata on the case page | Not a decision: the contract sends it and the page does not show it yet. |

## What the templ dashboard got wrong

The inventory found these, and they are the reason some surfaces changed rather
than moved:

1. Every write would have failed. Forms set `json-enc` but the shell never
   loaded it, so bodies went out form-encoded, which the API's binder very
   likely refused. Case edit, case delete, prompt version create and baseline
   delete went to paths with no handler, and case create sent the wrong body
   shape.
2. There was no way to start a run, save a baseline, make a prompt current,
   compare runs, import cases or generate red-team cases.
3. The settings panel showed the engine's defaults, never your config.
4. Three pages passed stat card arguments in the wrong order.
5. Run detail panicked when result stats failed to load.
6. Delete dialogs called `.close()` on a div and never closed.
7. A running run looked like a 0% failure on the runs list, the overview and
   the run page.
8. Every `?page=` link re-rendered the current page, so the prompts list was
   unreachable and the baseline filter worked by accident.
9. Write paths were hard-coded to `/sentinel/v1` and broke under a custom
   `base_path`.
10. A raw `suite_id` from the query string went into inline JavaScript, a
    possible reflected XSS.
11. Dimension scores came out in random order, score bar colours ignored the
    pass threshold, and timestamps in lists and the overview had no year.
12. Store errors became empty lists, so a database failure read as "No suites
    found".
13. Text was truncated by bytes, which could split a character.

## Badges

| Badge | templ | Now | Why |
|---|---|---|---|
| Run state: completed | grey | outline | Most runs are complete; it recedes. |
| Run state: running | primary | primary | Worth a look while it runs. |
| Run state: cancelled | outline | grey | |
| Run state: failed | destructive | destructive | |
| Result: pass | primary | outline | A pass is the common case. |
| Result: fail | destructive | destructive | |
| Result: error | destructive | primary | It means the case could not be judged, which is different from failing. |
| Verdict | (none) | regressed destructive, within threshold outline, the rest grey | Only a regression is loud, and always with an icon and a word. |
| Scenario | colours per type, cognitive stress red | standard outline, the rest grey | A scenario is not a severity. |
| Current | primary | primary | A marker worth finding. |
| Red team, Calls an LLM | (none) | primary | Markers worth finding. |
| Needs config | (none) | grey | |
| Scorer verdict | (none) | passed outline, failed destructive | The same rule as a result. |

## What stays uncovered

The tests do not reach four things, and you should know which:

- The LLM scorers and the scenario and dataset generators need real models, so
  nothing here runs them.
- The Mongo store reads nested values in a case's context and metadata, and
  in scorer config, back as `bson.D`, not as Go maps.
- Postgres stores scores and costs in 4-byte `REAL` columns, so a score read
  back can differ from the one written in the last few digits.
