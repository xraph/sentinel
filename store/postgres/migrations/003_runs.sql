CREATE TABLE IF NOT EXISTS sentinel_runs (
    id               TEXT PRIMARY KEY,
    suite_id         TEXT NOT NULL REFERENCES sentinel_suites(id) ON DELETE CASCADE,
    model            TEXT NOT NULL,
    system_prompt    TEXT NOT NULL DEFAULT '',
    temperature      REAL NOT NULL DEFAULT 0,
    total_cases      INT NOT NULL DEFAULT 0,
    passed           INT NOT NULL DEFAULT 0,
    failed           INT NOT NULL DEFAULT 0,
    pass_rate        REAL NOT NULL DEFAULT 0,
    avg_score        REAL NOT NULL DEFAULT 0,
    avg_latency_ms   INT NOT NULL DEFAULT 0,
    total_tokens     INT NOT NULL DEFAULT 0,
    total_cost       REAL NOT NULL DEFAULT 0,
    app_id           TEXT NOT NULL,
    target_tenant_id TEXT NOT NULL DEFAULT '',
    persona_ref      TEXT NOT NULL DEFAULT '',
    config           JSONB NOT NULL DEFAULT '{}',
    state            TEXT NOT NULL DEFAULT 'running',
    error            TEXT NOT NULL DEFAULT '',
    completed_at     TIMESTAMPTZ,
    dimension_scores JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
    latency_ms       INT NOT NULL DEFAULT 0,
    tokens_used      INT NOT NULL DEFAULT 0,
    cost             REAL NOT NULL DEFAULT 0,
    scorer_results   JSONB NOT NULL DEFAULT '[]',
    error            TEXT NOT NULL DEFAULT '',
    dimension_scores JSONB NOT NULL DEFAULT '{}',
    run_trace        JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sentinel_results_run ON sentinel_results (run_id);
CREATE INDEX IF NOT EXISTS idx_sentinel_results_case ON sentinel_results (run_id, case_id);
