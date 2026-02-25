ALTER TABLE site_files
  ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1;

ALTER TABLE site_files
  ADD COLUMN IF NOT EXISTS last_edited_by TEXT REFERENCES users(email);

CREATE TABLE IF NOT EXISTS file_revisions (
  id TEXT PRIMARY KEY,
  file_id TEXT NOT NULL REFERENCES site_files(id) ON DELETE CASCADE,
  version INT NOT NULL,
  content BYTEA NOT NULL,
  content_hash TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  mime_type TEXT NOT NULL,
  source TEXT NOT NULL,
  description TEXT,
  edited_by TEXT NOT NULL REFERENCES users(email),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(file_id, version)
);

CREATE INDEX IF NOT EXISTS idx_file_revisions_file_created_at
  ON file_revisions(file_id, created_at DESC);
