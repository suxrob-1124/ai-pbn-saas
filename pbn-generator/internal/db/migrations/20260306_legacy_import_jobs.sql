-- Legacy import job tracking tables.
-- Jobs represent a batch of legacy domain imports triggered from the web UI.
-- Items track per-domain progress within a job.

CREATE TABLE IF NOT EXISTS legacy_import_jobs (
    id              TEXT PRIMARY KEY,
    project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    requested_by    TEXT NOT NULL,
    force_overwrite BOOLEAN NOT NULL DEFAULT FALSE,
    status          TEXT NOT NULL DEFAULT 'pending',
    total_items     INT NOT NULL DEFAULT 0,
    completed_items INT NOT NULL DEFAULT 0,
    failed_items    INT NOT NULL DEFAULT 0,
    skipped_items   INT NOT NULL DEFAULT 0,
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_legacy_import_jobs_project_id ON legacy_import_jobs(project_id);
CREATE INDEX IF NOT EXISTS idx_legacy_import_jobs_status ON legacy_import_jobs(status);

CREATE TABLE IF NOT EXISTS legacy_import_items (
    id               TEXT PRIMARY KEY,
    job_id           TEXT NOT NULL REFERENCES legacy_import_jobs(id) ON DELETE CASCADE,
    domain_id        TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    domain_url       TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending',
    step             TEXT NOT NULL DEFAULT '',
    progress         INT NOT NULL DEFAULT 0,
    error            TEXT,
    actions          JSONB,
    warnings         JSONB,
    file_count       INT NOT NULL DEFAULT 0,
    total_size_bytes BIGINT NOT NULL DEFAULT 0,
    started_at       TIMESTAMPTZ,
    finished_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_legacy_import_items_job_id ON legacy_import_items(job_id);
CREATE INDEX IF NOT EXISTS idx_legacy_import_items_domain_id ON legacy_import_items(domain_id);
CREATE INDEX IF NOT EXISTS idx_legacy_import_items_status ON legacy_import_items(status);
