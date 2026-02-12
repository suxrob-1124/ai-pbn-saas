-- Index check history

CREATE TABLE IF NOT EXISTS index_check_history (
  id TEXT PRIMARY KEY,
  check_id TEXT NOT NULL REFERENCES domain_index_checks(id) ON DELETE CASCADE,
  attempt_number INT NOT NULL,
  result TEXT,
  response_data JSONB,
  error_message TEXT,
  duration_ms INT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_check_history_check ON index_check_history(check_id);
