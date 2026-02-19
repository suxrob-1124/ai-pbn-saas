CREATE TABLE IF NOT EXISTS llm_model_pricing (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  input_usd_per_million NUMERIC(14,6) NOT NULL,
  output_usd_per_million NUMERIC(14,6) NOT NULL,
  active_from TIMESTAMPTZ NOT NULL,
  active_to TIMESTAMPTZ NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  updated_by TEXT NOT NULL DEFAULT 'system',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_llm_model_pricing_model
  ON llm_model_pricing(provider, model, active_from DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_model_pricing_active
  ON llm_model_pricing(provider, model)
  WHERE is_active = TRUE;

CREATE TABLE IF NOT EXISTS llm_usage_events (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL,
  operation TEXT NOT NULL,
  stage TEXT NULL,
  model TEXT NOT NULL,
  status TEXT NOT NULL,
  requester_email TEXT NOT NULL REFERENCES users(email) ON DELETE CASCADE,
  key_owner_email TEXT REFERENCES users(email) ON DELETE SET NULL,
  key_type TEXT NULL,
  project_id TEXT REFERENCES projects(id) ON DELETE SET NULL,
  domain_id TEXT REFERENCES domains(id) ON DELETE SET NULL,
  generation_id TEXT REFERENCES generations(id) ON DELETE SET NULL,
  link_task_id TEXT REFERENCES link_tasks(id) ON DELETE SET NULL,
  file_path TEXT NULL,
  prompt_tokens BIGINT NULL,
  completion_tokens BIGINT NULL,
  total_tokens BIGINT NULL,
  token_source TEXT NOT NULL,
  input_price_usd_per_million NUMERIC(14,6) NULL,
  output_price_usd_per_million NUMERIC(14,6) NULL,
  estimated_cost_usd NUMERIC(18,8) NULL,
  error_message TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_llm_usage_events_created_at
  ON llm_usage_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_llm_usage_events_requester
  ON llm_usage_events(requester_email);
CREATE INDEX IF NOT EXISTS idx_llm_usage_events_project
  ON llm_usage_events(project_id);
CREATE INDEX IF NOT EXISTS idx_llm_usage_events_domain
  ON llm_usage_events(domain_id);
CREATE INDEX IF NOT EXISTS idx_llm_usage_events_generation
  ON llm_usage_events(generation_id);
CREATE INDEX IF NOT EXISTS idx_llm_usage_events_model
  ON llm_usage_events(model);
CREATE INDEX IF NOT EXISTS idx_llm_usage_events_operation
  ON llm_usage_events(operation);

INSERT INTO llm_model_pricing(id, provider, model, input_usd_per_million, output_usd_per_million, active_from, active_to, is_active, updated_by, updated_at)
VALUES
  ('seed-gemini-2.5-pro', 'gemini', 'gemini-2.5-pro', 1.250000, 5.000000, NOW(), NULL, TRUE, 'system', NOW()),
  ('seed-gemini-2.5-flash', 'gemini', 'gemini-2.5-flash', 0.300000, 0.600000, NOW(), NULL, TRUE, 'system', NOW()),
  ('seed-gemini-2.5-flash-image', 'gemini', 'gemini-2.5-flash-image', 0.300000, 0.600000, NOW(), NULL, TRUE, 'system', NOW())
ON CONFLICT (id) DO NOTHING;
