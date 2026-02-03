-- File storage tables and domain stats

CREATE TABLE IF NOT EXISTS site_files (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    content_hash TEXT,
    size_bytes BIGINT NOT NULL,
    mime_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(domain_id, path)
);

CREATE INDEX IF NOT EXISTS idx_site_files_domain ON site_files(domain_id);

CREATE TABLE IF NOT EXISTS file_edits (
    id TEXT PRIMARY KEY,
    file_id TEXT NOT NULL REFERENCES site_files(id) ON DELETE CASCADE,
    edited_by TEXT NOT NULL REFERENCES users(email),
    content_before_hash TEXT,
    content_after_hash TEXT,
    edit_type TEXT NOT NULL DEFAULT 'manual',
    edit_description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_file_edits_file ON file_edits(file_id);
CREATE INDEX IF NOT EXISTS idx_file_edits_user ON file_edits(edited_by);

ALTER TABLE domains ADD COLUMN IF NOT EXISTS published_path TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS file_count INT DEFAULT 0;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS total_size_bytes BIGINT DEFAULT 0;
