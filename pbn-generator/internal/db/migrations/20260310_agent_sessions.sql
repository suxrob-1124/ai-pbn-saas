CREATE TABLE IF NOT EXISTS agent_sessions (
    id            TEXT PRIMARY KEY,
    domain_id     TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    created_by    TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at   TIMESTAMPTZ,
    status        TEXT NOT NULL DEFAULT 'running',
    summary       TEXT,
    files_changed JSONB,
    message_count INT NOT NULL DEFAULT 0,
    snapshot_tag  TEXT
);

CREATE INDEX IF NOT EXISTS idx_agent_sessions_domain ON agent_sessions(domain_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_snapshot_tag ON agent_sessions(snapshot_tag) WHERE snapshot_tag IS NOT NULL;
