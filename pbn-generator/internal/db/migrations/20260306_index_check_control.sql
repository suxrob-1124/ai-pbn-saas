-- Index Checker Control: global, project-level, and domain-level pause/enable.

-- Global app settings (key-value store).
CREATE TABLE IF NOT EXISTS app_settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Default: index checker enabled globally.
INSERT INTO app_settings (key, value) VALUES ('index_check_enabled', 'true')
ON CONFLICT (key) DO NOTHING;

-- Project-level toggle.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS index_check_enabled BOOLEAN NOT NULL DEFAULT TRUE;

-- Domain-level toggle.
ALTER TABLE domains ADD COLUMN IF NOT EXISTS index_check_enabled BOOLEAN NOT NULL DEFAULT TRUE;
