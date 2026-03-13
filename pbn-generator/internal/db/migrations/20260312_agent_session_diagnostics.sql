ALTER TABLE agent_sessions ADD COLUMN IF NOT EXISTS diagnostics_json JSONB;
