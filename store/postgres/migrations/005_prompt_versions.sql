CREATE TABLE IF NOT EXISTS sentinel_prompt_versions (
    id              TEXT PRIMARY KEY,
    suite_id        TEXT NOT NULL REFERENCES sentinel_suites(id) ON DELETE CASCADE,
    version         INT NOT NULL,
    system_prompt   TEXT NOT NULL,
    changelog       TEXT NOT NULL DEFAULT '',
    is_current      BOOLEAN NOT NULL DEFAULT FALSE,
    run_id          TEXT NOT NULL DEFAULT '',
    pass_rate       REAL,
    avg_score       REAL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(suite_id, version)
);

CREATE INDEX IF NOT EXISTS idx_sentinel_prompts_suite ON sentinel_prompt_versions (suite_id, is_current);
