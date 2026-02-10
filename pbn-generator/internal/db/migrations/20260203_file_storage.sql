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
ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_anchor_text TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_acceptor_url TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_status TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_updated_at TIMESTAMPTZ;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_last_task_id TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_file_path TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_anchor_snapshot TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_ready_at TIMESTAMPTZ;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS timezone TEXT;

CREATE TABLE IF NOT EXISTS generation_schedules (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT,
  strategy TEXT NOT NULL,
  config JSONB NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_by TEXT NOT NULL REFERENCES users(email),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_gen_schedules_project ON generation_schedules(project_id);
CREATE INDEX IF NOT EXISTS idx_gen_schedules_active ON generation_schedules(is_active);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_gen_schedules_project ON generation_schedules(project_id);
ALTER TABLE generation_schedules ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;
ALTER TABLE generation_schedules ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;
ALTER TABLE generation_schedules ADD COLUMN IF NOT EXISTS timezone TEXT;

CREATE TABLE IF NOT EXISTS generation_queue (
  id TEXT PRIMARY KEY,
  domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  schedule_id TEXT REFERENCES generation_schedules(id) ON DELETE SET NULL,
  priority INT NOT NULL DEFAULT 0,
  scheduled_for TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  processed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_gen_queue_scheduled ON generation_queue(scheduled_for, status);
CREATE INDEX IF NOT EXISTS idx_gen_queue_domain ON generation_queue(domain_id);
CREATE INDEX IF NOT EXISTS idx_gen_queue_status ON generation_queue(status);

CREATE TABLE IF NOT EXISTS link_tasks (
  id TEXT PRIMARY KEY,
  domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  anchor_text TEXT NOT NULL,
  target_url TEXT NOT NULL,
  scheduled_for TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  action TEXT NOT NULL DEFAULT 'insert',
  status TEXT NOT NULL DEFAULT 'pending',
  found_location TEXT,
  generated_content TEXT,
  error_message TEXT,
  attempts INT NOT NULL DEFAULT 0,
  created_by TEXT NOT NULL REFERENCES users(email),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at TIMESTAMPTZ
);
ALTER TABLE link_tasks ADD COLUMN IF NOT EXISTS action TEXT NOT NULL DEFAULT 'insert';
ALTER TABLE link_tasks ADD COLUMN IF NOT EXISTS log_lines JSONB;
CREATE INDEX IF NOT EXISTS idx_link_tasks_domain ON link_tasks(domain_id);
CREATE INDEX IF NOT EXISTS idx_link_tasks_status ON link_tasks(status);
CREATE INDEX IF NOT EXISTS idx_link_tasks_scheduled ON link_tasks(scheduled_for, status);

CREATE TABLE IF NOT EXISTS link_schedules (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  config JSONB NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_by TEXT NOT NULL REFERENCES users(email),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;
ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;
ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS timezone TEXT;
ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
CREATE UNIQUE INDEX IF NOT EXISTS uniq_link_schedules_project ON link_schedules(project_id);

CREATE TABLE IF NOT EXISTS audit_rules (
	code TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT,
	severity TEXT NOT NULL DEFAULT 'warn',
	is_active BOOLEAN NOT NULL DEFAULT TRUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_findings (
	id TEXT PRIMARY KEY,
	generation_id TEXT NOT NULL REFERENCES generations(id) ON DELETE CASCADE,
	domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
	rule_code TEXT NOT NULL REFERENCES audit_rules(code),
	severity TEXT NOT NULL DEFAULT 'warn',
	file_path TEXT,
	message TEXT NOT NULL,
	details JSONB,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_findings_generation ON audit_findings(generation_id);
CREATE INDEX IF NOT EXISTS idx_audit_findings_domain ON audit_findings(domain_id);
