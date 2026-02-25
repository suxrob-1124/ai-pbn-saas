-- Additive reporting indexes for llm_usage_events filters:
-- date/user/project/domain/model/operation/status.

CREATE INDEX IF NOT EXISTS idx_llm_usage_events_status
  ON llm_usage_events(status);

CREATE INDEX IF NOT EXISTS idx_llm_usage_events_operation_status_created
  ON llm_usage_events(operation, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_usage_events_requester_created
  ON llm_usage_events(requester_email, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_usage_events_project_created
  ON llm_usage_events(project_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_usage_events_domain_created
  ON llm_usage_events(domain_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_usage_events_model_created
  ON llm_usage_events(model, created_at DESC);
