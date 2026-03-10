CREATE TABLE IF NOT EXISTS file_locks (
    domain_id TEXT NOT NULL,
    file_path TEXT NOT NULL,
    locked_by TEXT NOT NULL,
    locked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (domain_id, file_path)
);
CREATE INDEX IF NOT EXISTS idx_file_locks_expires ON file_locks(expires_at);
