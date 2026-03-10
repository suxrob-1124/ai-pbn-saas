-- Soft-delete columns for projects
ALTER TABLE projects ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS deleted_by TEXT;

-- Soft-delete columns for domains
ALTER TABLE domains ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS deleted_by TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS delete_batch TEXT;

-- Indexes for soft-deleted lookups
CREATE INDEX IF NOT EXISTS idx_projects_deleted ON projects(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_domains_deleted ON domains(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_domains_delete_batch ON domains(delete_batch) WHERE delete_batch IS NOT NULL;

-- Unique URL constraint only for active (non-deleted) domains
DROP INDEX IF EXISTS idx_domains_url;
CREATE UNIQUE INDEX IF NOT EXISTS idx_domains_url_active ON domains(url) WHERE deleted_at IS NULL;
