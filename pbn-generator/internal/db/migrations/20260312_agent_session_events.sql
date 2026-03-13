-- Append-only event log for agent sessions.
-- Replaces per-iteration full-blob SaveMessages with cheap delta appends,
-- reducing write amplification from O(N²) to O(N) across the session.
CREATE TABLE IF NOT EXISTS agent_session_events (
    id           TEXT PRIMARY KEY,
    session_id   TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    seq          INT  NOT NULL,
    event_type   TEXT NOT NULL,   -- e.g. 'chat_entry'
    payload_json JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (session_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_agent_session_events_session
    ON agent_session_events(session_id, seq);
