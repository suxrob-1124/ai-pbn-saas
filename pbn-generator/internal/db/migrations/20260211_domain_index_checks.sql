-- Domain index checks

CREATE TABLE IF NOT EXISTS domain_index_checks (
  id TEXT PRIMARY KEY,
  domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  check_date DATE NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  is_indexed BOOLEAN,
  attempts INT NOT NULL DEFAULT 0,
  last_attempt_at TIMESTAMPTZ,
  next_retry_at TIMESTAMPTZ,
  error_message TEXT,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(domain_id, check_date)
);

CREATE INDEX IF NOT EXISTS idx_index_checks_domain ON domain_index_checks(domain_id);
CREATE INDEX IF NOT EXISTS idx_index_checks_date ON domain_index_checks(check_date);
CREATE INDEX IF NOT EXISTS idx_index_checks_status ON domain_index_checks(status);
CREATE INDEX IF NOT EXISTS idx_index_checks_retry ON domain_index_checks(next_retry_at) WHERE status = 'checking';
