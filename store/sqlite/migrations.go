package sqlite

import (
	"context"

	"github.com/xraph/grove/migrate"
)

// Migrations is the grove migration group for the Sentinel SQLite store.
var Migrations = migrate.NewGroup("sentinel")

func init() {
	Migrations.MustRegister(
		&migrate.Migration{
			Name:    "create_suites",
			Version: "20240101000001",
			Up: func(ctx context.Context, exec migrate.Executor) error {
				_, err := exec.Exec(ctx, `
CREATE TABLE IF NOT EXISTS sentinel_suites (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    app_id          TEXT NOT NULL,
    system_prompt   TEXT NOT NULL DEFAULT '',
    model           TEXT NOT NULL,
    temperature     REAL NOT NULL DEFAULT 0,
    persona_ref     TEXT NOT NULL DEFAULT '',
    metadata        TEXT NOT NULL DEFAULT '{}',
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),

    UNIQUE(app_id, name)
);

CREATE INDEX IF NOT EXISTS idx_sentinel_suites_app ON sentinel_suites (app_id);
`)
				return err
			},
			Down: func(ctx context.Context, exec migrate.Executor) error {
				_, err := exec.Exec(ctx, `DROP TABLE IF EXISTS sentinel_suites`)
				return err
			},
		},
		&migrate.Migration{
			Name:    "create_cases",
			Version: "20240101000002",
			Up: func(ctx context.Context, exec migrate.Executor) error {
				_, err := exec.Exec(ctx, `
CREATE TABLE IF NOT EXISTS sentinel_cases (
    id              TEXT PRIMARY KEY,
    suite_id        TEXT NOT NULL REFERENCES sentinel_suites(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    input           TEXT NOT NULL,
    expected        TEXT NOT NULL DEFAULT '',
    scenario_type   TEXT NOT NULL DEFAULT 'standard',
    scorers         TEXT NOT NULL DEFAULT '[]',
    tags            TEXT NOT NULL DEFAULT '[]',
    context         TEXT NOT NULL DEFAULT '{}',
    metadata        TEXT NOT NULL DEFAULT '{}',
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sentinel_cases_suite ON sentinel_cases (suite_id);
CREATE INDEX IF NOT EXISTS idx_sentinel_cases_scenario ON sentinel_cases (suite_id, scenario_type);
`)
				return err
			},
			Down: func(ctx context.Context, exec migrate.Executor) error {
				_, err := exec.Exec(ctx, `DROP TABLE IF EXISTS sentinel_cases`)
				return err
			},
		},
		&migrate.Migration{
			Name:    "create_runs_and_results",
			Version: "20240101000003",
			Up: func(ctx context.Context, exec migrate.Executor) error {
				_, err := exec.Exec(ctx, `
CREATE TABLE IF NOT EXISTS sentinel_runs (
    id               TEXT PRIMARY KEY,
    suite_id         TEXT NOT NULL REFERENCES sentinel_suites(id) ON DELETE CASCADE,
    model            TEXT NOT NULL,
    system_prompt    TEXT NOT NULL DEFAULT '',
    temperature      REAL NOT NULL DEFAULT 0,
    total_cases      INTEGER NOT NULL DEFAULT 0,
    passed           INTEGER NOT NULL DEFAULT 0,
    failed           INTEGER NOT NULL DEFAULT 0,
    pass_rate        REAL NOT NULL DEFAULT 0,
    avg_score        REAL NOT NULL DEFAULT 0,
    avg_latency_ms   INTEGER NOT NULL DEFAULT 0,
    total_tokens     INTEGER NOT NULL DEFAULT 0,
    total_cost       REAL NOT NULL DEFAULT 0,
    app_id           TEXT NOT NULL,
    target_tenant_id TEXT NOT NULL DEFAULT '',
    persona_ref      TEXT NOT NULL DEFAULT '',
    config           TEXT NOT NULL DEFAULT '{}',
    state            TEXT NOT NULL DEFAULT 'running',
    error            TEXT NOT NULL DEFAULT '',
    completed_at     TEXT,
    dimension_scores TEXT NOT NULL DEFAULT '{}',
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sentinel_runs_suite ON sentinel_runs (suite_id, state);
CREATE INDEX IF NOT EXISTS idx_sentinel_runs_app ON sentinel_runs (app_id);

CREATE TABLE IF NOT EXISTS sentinel_results (
    id               TEXT PRIMARY KEY,
    run_id           TEXT NOT NULL REFERENCES sentinel_runs(id) ON DELETE CASCADE,
    case_id          TEXT NOT NULL,
    case_name        TEXT NOT NULL,
    status           TEXT NOT NULL,
    score            REAL NOT NULL DEFAULT 0,
    output           TEXT NOT NULL DEFAULT '',
    latency_ms       INTEGER NOT NULL DEFAULT 0,
    tokens_used      INTEGER NOT NULL DEFAULT 0,
    cost             REAL NOT NULL DEFAULT 0,
    scorer_results   TEXT NOT NULL DEFAULT '[]',
    error            TEXT NOT NULL DEFAULT '',
    dimension_scores TEXT NOT NULL DEFAULT '{}',
    run_trace        TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sentinel_results_run ON sentinel_results (run_id);
CREATE INDEX IF NOT EXISTS idx_sentinel_results_case ON sentinel_results (run_id, case_id);
`)
				return err
			},
			Down: func(ctx context.Context, exec migrate.Executor) error {
				_, err := exec.Exec(ctx, `
DROP TABLE IF EXISTS sentinel_results;
DROP TABLE IF EXISTS sentinel_runs;
`)
				return err
			},
		},
		&migrate.Migration{
			Name:    "create_baselines",
			Version: "20240101000004",
			Up: func(ctx context.Context, exec migrate.Executor) error {
				_, err := exec.Exec(ctx, `
CREATE TABLE IF NOT EXISTS sentinel_baselines (
    id               TEXT PRIMARY KEY,
    suite_id         TEXT NOT NULL REFERENCES sentinel_suites(id) ON DELETE CASCADE,
    run_id           TEXT NOT NULL REFERENCES sentinel_runs(id),
    name             TEXT NOT NULL,
    results          TEXT NOT NULL,
    pass_rate        REAL NOT NULL DEFAULT 0,
    avg_score        REAL NOT NULL DEFAULT 0,
    dimension_scores TEXT NOT NULL DEFAULT '{}',
    is_current       INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sentinel_baselines_suite ON sentinel_baselines (suite_id, is_current);
`)
				return err
			},
			Down: func(ctx context.Context, exec migrate.Executor) error {
				_, err := exec.Exec(ctx, `DROP TABLE IF EXISTS sentinel_baselines`)
				return err
			},
		},
		&migrate.Migration{
			Name:    "create_prompt_versions",
			Version: "20240101000005",
			Up: func(ctx context.Context, exec migrate.Executor) error {
				_, err := exec.Exec(ctx, `
CREATE TABLE IF NOT EXISTS sentinel_prompt_versions (
    id              TEXT PRIMARY KEY,
    suite_id        TEXT NOT NULL REFERENCES sentinel_suites(id) ON DELETE CASCADE,
    version         INTEGER NOT NULL,
    system_prompt   TEXT NOT NULL,
    changelog       TEXT NOT NULL DEFAULT '',
    is_current      INTEGER NOT NULL DEFAULT 0,
    run_id          TEXT NOT NULL DEFAULT '',
    pass_rate       REAL,
    avg_score       REAL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),

    UNIQUE(suite_id, version)
);

CREATE INDEX IF NOT EXISTS idx_sentinel_prompts_suite ON sentinel_prompt_versions (suite_id, is_current);
`)
				return err
			},
			Down: func(ctx context.Context, exec migrate.Executor) error {
				_, err := exec.Exec(ctx, `DROP TABLE IF EXISTS sentinel_prompt_versions`)
				return err
			},
		},
	)
}
