CREATE TABLE IF NOT EXISTS sentinel_baselines (
    id               TEXT PRIMARY KEY,
    suite_id         TEXT NOT NULL REFERENCES sentinel_suites(id) ON DELETE CASCADE,
    run_id           TEXT NOT NULL REFERENCES sentinel_runs(id),
    name             TEXT NOT NULL,
    results          JSONB NOT NULL,
    pass_rate        REAL NOT NULL DEFAULT 0,
    avg_score        REAL NOT NULL DEFAULT 0,
    dimension_scores JSONB NOT NULL DEFAULT '{}',
    is_current       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sentinel_baselines_suite ON sentinel_baselines (suite_id, is_current);
