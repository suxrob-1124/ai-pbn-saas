ALTER TABLE site_files
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE site_files
  ADD COLUMN IF NOT EXISTS deleted_by TEXT REFERENCES users(email);

ALTER TABLE site_files
  ADD COLUMN IF NOT EXISTS delete_reason TEXT;

ALTER TABLE site_files
  DROP CONSTRAINT IF EXISTS site_files_domain_id_path_key;

CREATE INDEX IF NOT EXISTS idx_site_files_deleted
  ON site_files(domain_id, deleted_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_site_files_domain_path_active
  ON site_files(domain_id, path)
  WHERE deleted_at IS NULL;
