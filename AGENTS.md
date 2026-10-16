# AGENTS.md

Sentinel is a Go library for evaluating AI agents. You define suites of cases, run them against a target, score the outputs with deterministic, LLM-judge and persona-aware scorers, and compare runs against a baseline to catch regressions. It ships an HTTP API, a Forge extension and a Forge dashboard contributor. The module path is `github.com/xraph/sentinel`.

It's one module. There is no `cmd/` and no binary.

## Commands

Run everything from the repo root. CI has no `go.work`, so build with `GOWORK=off` and you get exactly what `go.mod` pins.

| What | Command |
|---|---|
| Build | `GOWORK=off go build ./...` |
| Test, as CI runs it | `make test` (`go test -v ./...`) |
| Race tests, as release runs them | `go test -race -count=1 ./...` (or `make test-race`) |
| Postgres and mongo store tests | `go test -tags integration -count=1 -v ./store/...` (see below) |
| Vet | `make vet` |
| Lint | `make lint`, or the uncapped form below |
| Format | `make fmt` (`gofmt -s -w .`, then `goimports -w -local github.com/xraph/sentinel .`) |
| Format, vet and lint together | `make check` |
| Coverage | `make coverage` |
| Docs site | `make docs` or `make docs-build` (pnpm, inside `docs/`) |

`make build`, `make run`, `make dev`, `make install` and `make all` all reach for `./cmd/sentinel`, which does not exist. They fail.

`.github/workflows/ci.yml` calls the shared `xraph/workflows/.github/workflows/go-ci.yml@v1` on Go 1.26 (setup-go picks the newest patch) and adds its own Format and Docs jobs. Between them CI runs `make test`, a race-enabled coverage run, golangci-lint at the version go-ci.yml pins, `gofmt -l .`, `make vet`, a `go mod tidy` diff check on `go.mod` and `go.sum`, gosec (`-exclude=G115`) and govulncheck (both fail on findings), `goimports -l -local github.com/xraph/sentinel .`, and `pnpm types:check`, `pnpm lint` and `pnpm build` in `docs/`.

## Store tests

Every backend runs the same conformance suite, `storetest.Run` in `store/storetest`. Memory and SQLite (pure Go, through modernc) run it on a plain `go test ./...`.

Postgres and mongo don't. Their tests live in `conformance_integration_test.go` behind `//go:build integration`, so without the tag they aren't compiled at all. You won't even see a `SKIP`. CI never passes the tag. When you change anything under `store/`, run them yourself:

```sh
GOWORK=off go test -tags integration -count=1 -v ./store/...
```

With Docker running and no env vars set, testcontainers starts a `postgres:16-alpine` and a `mongo:7` for you, once per package. If Docker isn't reachable, both suites call `t.Skipf` ("postgres unavailable", "mongo unavailable") and the run still exits green, so read the output. To use servers you already run, set:

- `SENTINEL_TEST_DSN`, an admin DSN for postgres. Each subtest creates its own `sentinel_test_<hex>` database and drops it `WITH (FORCE)` afterwards, so the role needs `CREATEDB`.
- `SENTINEL_TEST_MONGO_URI`. Each subtest uses its own `sentinel_test_<hex>` database and drops it, so the URI does not need a database name.

Because the integration files sit behind a tag, `make vet` and golangci-lint skip them too. After you edit one, check it explicitly with `go vet -tags integration ./store/...` and `golangci-lint run --build-tags integration ./store/...`.

## Lint

`.golangci.yml` is golangci-lint v2 config. golangci-lint shows only three identical messages by default, which hides hits locally and in CI. Run it uncapped before you call a lint fix finished:

```sh
golangci-lint run --max-same-issues=0 --max-issues-per-linter=0
```

What bites here:

- govet runs `enable-all` minus `fieldalignment`, so shadow is on, and an inner `if err := f()` under an outer `err` is flagged. gocritic's `sloppyReassign` rejects `if err = f(); err != nil { return err }` in non-test code. Give the inner error its own name, as the tree already does: `recErr`, `getErr`, `runErr`. `store/storetest/*.go` counts as non-test code, since none of those files end in `_test.go`. Test files can reuse `err`, because gocritic skips them.
- Only gosec, errcheck and gocritic are excluded for `_test.go`. revive and staticcheck still run on test files, so a `//nolint:revive` or `//nolint:staticcheck` there is legitimate. A `//nolint` for gosec, errcheck or gocritic in a test file is dead, and nolintlint reports it.
- errcheck sets `check-blank: true` and `check-type-assertions: true`, and also exempts forge's `Router.GET/POST/PUT/DELETE/PATCH`. A deliberate `_ = f()` needs `//nolint:errcheck // <reason>`. Every `//nolint` in the tree names its linter and gives a reason.
- The postgres and sqlite stores roll back with one line:

  ```go
  defer func() { _ = tx.Rollback() }() //nolint:errcheck // rollback after a commit returns ErrTxDone
  ```

- errorlint wants `errors.Is`, `errors.As` and `%w`. errname wants `Err...` sentinels and `...Error` types.
- goimports puts `github.com/xraph/sentinel` imports in the last group.

For govulncheck, use CI's Go. CI always resolves the newest 1.26 patch, and an older local toolchain reports stdlib advisories CI never sees. Run `GOTOOLCHAIN=go1.26.N govulncheck ./...` with N set to that patch.

## Layout

| Path | What lives there |
|---|---|
| `sentinel.go`, `config.go`, `errors.go`, `scope.go`, `entity.go`, `id.go` | Package docs, `Config`/`DefaultConfig`, sentinel errors, `WithTenant`/`WithApp` and their readers, the `Entity` timestamps, the `ID` re-export |
| `id/` | TypeID-based `id.ID` with a prefix per entity (`suite`, `tcase`, `erun`, `eres`, `base`, `rteam`, `pver`) |
| `engine/` | `engine.Engine`, the coordinator: runs, scoring, regression checks, red team, imports. `telemetry.go` and `metrics.go` hold its spans and metrics, and `internal/telemetry` the error helper every span shares |
| `suite/`, `testcase/`, `evalrun/`, `baseline/`, `promptversion/` | Domain models, each with its own `Store` interface |
| `store/store.go` | `store.Store`, which embeds those five and adds `Migrate`, `Ping`, `Close` |
| `store/memory`, `store/sqlite`, `store/postgres`, `store/mongo` | The backends. The grove ones keep their migrations as Go in `migrations.go` |
| `store/storetest/` | The conformance suite |
| `scorer/` | The `Scorer` interface, the built-in scorers and the registry |
| `target/` | The `Target` interface plus LLM, agent and function targets |
| `redteam/`, `scenario/`, `dataset/`, `comparison/`, `report/` | Adversarial case generators, persona scenario generators, loading and sampling cases, side-by-side model comparison, and report writers (terminal, JSON, HTML, CI) |
| `plugin/` | Lifecycle hook interfaces (the base one is `plugin.Extension`) and the registry |
| `audit_hook/` (package `audithook`), `observability/` | Plugins for an audit trail and for unlabelled lifecycle counters |
| `api/` | Forge HTTP handlers, mounted under `BasePath` (default `/sentinel`) |
| `extension/` | The Forge extension, its `Config` and `ExtOption`s |
| `extension/contract/` | The `sentinel` dashboard contributor: `manifest.yaml` and `handlers_*.go` |
| `sentineltest/` | `Assert` helpers for tests that check run results |
| `docs/` | The Fumadocs site |
| `MIGRATION.md` | How the templ dashboard moved to the React shell |

`store/postgres/migrations/*.sql` is not loaded by anything. The postgres migrations that run are the Go strings in `store/postgres/migrations.go`, so change those.

## Conventions

Errors are sentinels in `errors.go`, prefixed `sentinel:`. Stores return the specific one for a missing row (`sentinel.ErrSuiteNotFound`, `sentinel.ErrRunNotFound`) and wrap everything else as `fmt.Errorf("sentinel: get suite: %w", err)`. Callers use `errors.Is`.

IDs come from the `id` package, never from string building.

`store.Store` is the contract for all four backends. If you change one of the embedded store interfaces, you implement it in memory, sqlite, postgres and mongo, add a check to `store/storetest`, and run the integration suites. Each backend numbers its migrations separately (`Version: "20240101000005"` and so on), so when you add one, give it the next version in that backend's file. An empty app id in a list filter matches every app on all four backends, which `store/storetest/tenancy.go` pins. That's why `resolveApp` in `extension/contract/tenancy.go` never returns an empty string. Read its comment before you touch it.

Options come in two shapes. `engine.New(opts...)` takes `type Option func(*Engine) error` and returns `(*Engine, error)`. `extension.New(opts...)` takes `type ExtOption func(*Extension)`. The extension reads YAML under `extensions.sentinel`, then `sentinel`, and merges it with the programmatic options. For a store it uses `WithStore`, a named grove DB (`WithGroveDatabase`), or the default `*grove.DB` it finds in the container, picking the backend from the driver name (`pg`, `sqlite`, `mongo`). It provides the `*engine.Engine` with `vessel.Provide`.

Logging goes through `github.com/xraph/go-utils/log`. The engine starts with `log.NewNoopLogger()`, and the extension passes in the Forge logger.

Telemetry lives in the engine. Spans go to the global tracer provider under `engine.InstrumentationName`, and metrics go to whatever `engine.WithMetrics` was given (the extension passes `e.Metrics()`; nil records nothing). Never record through `otel.Meter()`: Forge installs no global MeterProvider, so those series are never exported. Spans and labels take IDs, counts and fixed kinds only. Prompts, inputs, outputs, judge reasons, error text and user-chosen scorer or target names stay off them, which is why `scorerKind` and `targetKind` switch on Go types and never read `Name()`. IDs can go on spans but never on labels. `TestSpansCarryNoContent` in `engine/telemetry_test.go` pins the rule, and `docs/content/docs/infrastructure/observability.mdx` lists every span and metric, so change both with the code.

The dashboard intents are pinned in `TestIntentSet` in `extension/contract/contract_test.go`. If you add or remove one, you change `manifest.yaml`, the handler, that test's `want` map, and `packages/plugin-sentinel` in forge-dashboard, all in step. Every command has to list what it invalidates.

## Dependencies

Sentinel depends on grove (with `drivers/mongodriver`, `drivers/pgdriver` and `drivers/sqlitedriver`) and forge, plus `go-utils` and `vessel`. In cortex, `cortex/sentinel` requires it directly and `cortex/extension` pulls it in through that. See `go.mod` for versions.

Grove's drivers are tagged in lockstep with grove, so move all four together:

```sh
GOWORK=off go get github.com/xraph/grove@vX.Y.Z \
  github.com/xraph/grove/drivers/mongodriver@vX.Y.Z \
  github.com/xraph/grove/drivers/pgdriver@vX.Y.Z \
  github.com/xraph/grove/drivers/sqlitedriver@vX.Y.Z
GOWORK=off go get github.com/xraph/forge@vX.Y.Z
GOWORK=off go mod tidy
```

Don't commit a `go.work` or a `replace` that points at a sibling checkout. A local replace is fine while you try an unreleased grove or forge, but it has to go before you push: CI can't resolve the path, and cortex ignores replaces in its dependencies anyway.

## Releasing

Releases go through the workflow. Never push a tag by hand.

```sh
gh run list --workflow ci.yml --branch main --limit 1   # wait for green
gh workflow run release.yml --ref main -f tag=vX.Y.Z
```

`release.yml` tags the dispatched commit and pushes the tag first. Then it runs `go build ./...` and `go test -race -count=1 ./...`, builds notes from the commits since the previous tag, and creates the GitHub release. A failed test leaves the tag in place with no release, and re-dispatching the same tag fails at `git tag`. Fix forward with the next patch.

Release only after the grove and forge versions sentinel needs are out. Once it's tagged, `cortex/sentinel` re-pins with `go get github.com/xraph/sentinel@vX.Y.Z`.

## Branches and commits

`main` has one ruleset that blocks deletion and force-pushes. Nothing else: no required checks, reviews or signatures, and we push to it directly. Commit subjects are conventional commits, often scoped, like `feat(contract): ...`, `test(contract): ...` or `docs: ...`.
