CREATE TABLE IF NOT EXISTS sentinel_cases (
    id              TEXT PRIMARY KEY,
    suite_id        TEXT NOT NULL REFERENCES sentinel_suites(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    input           TEXT NOT NULL,
    expected        TEXT NOT NULL DEFAULT '',
    scenario_type   TEXT NOT NULL DEFAULT 'standard',
    scorers         JSONB NOT NULL DEFAULT '[]',
    tags            JSONB NOT NULL DEFAULT '[]',
    context         JSONB NOT NULL DEFAULT '{}',
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sentinel_cases_suite ON sentinel_cases (suite_id);
CREATE INDEX IF NOT EXISTS idx_sentinel_cases_scenario ON sentinel_cases (suite_id, scenario_type);
