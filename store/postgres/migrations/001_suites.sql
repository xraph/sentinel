CREATE TABLE IF NOT EXISTS sentinel_suites (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    app_id          TEXT NOT NULL,
    system_prompt   TEXT NOT NULL DEFAULT '',
    model           TEXT NOT NULL,
    temperature     REAL NOT NULL DEFAULT 0,
    persona_ref     TEXT NOT NULL DEFAULT '',
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(app_id, name)
);

CREATE INDEX IF NOT EXISTS idx_sentinel_suites_app ON sentinel_suites (app_id);
