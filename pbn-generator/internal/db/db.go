package db

import (
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(cfg config.Config, logger *zap.SugaredLogger) *sql.DB {
	if cfg.DBDriver == "" {
		logger.Fatalf("DB_DRIVER is required (pgx for Postgres)")
	}
	db, err := sql.Open(cfg.DBDriver, cfg.DSN)
	if err != nil {
		logger.Fatalf("failed to open database: %v", err)
	}

	if err := pingWithRetry(db, cfg.DBConnectRetries, cfg.DBConnectInterval); err != nil {
		logger.Fatalf("failed to ping database: %v", err)
	}

	if cfg.MigrateOnStart {
		if err := Migrate(db); err != nil {
			logger.Fatalf("failed to run migrations: %v", err)
		}
	}
	return db
}

func Migrate(db *sql.DB) error {
	for _, stmt := range migrationStatements() {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}

func migrationStatements() []string {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			email TEXT PRIMARY KEY,
			password_hash BYTEA NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			verified BOOLEAN NOT NULL DEFAULT FALSE,
			name TEXT,
			avatar_url TEXT,
			role TEXT NOT NULL DEFAULT 'manager',
			is_approved BOOLEAN NOT NULL DEFAULT FALSE,
			api_key_enc BYTEA,
			api_key_updated_at TIMESTAMPTZ
		);`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS verified BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS name TEXT;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'manager';`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS is_approved BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS api_key_enc BYTEA;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS api_key_updated_at TIMESTAMPTZ;`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT fk_sessions_user FOREIGN KEY (email) REFERENCES users(email) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_email ON sessions(email);`,
		`CREATE TABLE IF NOT EXISTS email_verifications (
			token_hash TEXT PRIMARY KEY,
			email TEXT NOT NULL REFERENCES users(email) ON DELETE CASCADE,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ
		);`,
		`ALTER TABLE email_verifications ADD COLUMN IF NOT EXISTS used_at TIMESTAMPTZ;`,
		`CREATE INDEX IF NOT EXISTS idx_email_verifications_email ON email_verifications(email);`,
		`CREATE TABLE IF NOT EXISTS password_resets (
			token_hash TEXT PRIMARY KEY,
			email TEXT NOT NULL REFERENCES users(email) ON DELETE CASCADE,
			expires_at TIMESTAMPTZ NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_password_resets_email ON password_resets(email);`,
		`CREATE TABLE IF NOT EXISTS email_changes (
			token_hash TEXT PRIMARY KEY,
			email TEXT NOT NULL REFERENCES users(email) ON DELETE CASCADE,
			new_email TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ
		);`,
		`ALTER TABLE email_changes ADD COLUMN IF NOT EXISTS used_at TIMESTAMPTZ;`,
		`CREATE INDEX IF NOT EXISTS idx_email_changes_email ON email_changes(email);`,
		`CREATE TABLE IF NOT EXISTS captchas (
			id TEXT PRIMARY KEY,
			answer_hash TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			ip TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_captchas_expires ON captchas(expires_at);`,
	}
	// Projects/domains/generations
	projectStmts := []string{
		`CREATE TABLE IF NOT EXISTS servers (
			id TEXT PRIMARY KEY,
			user_email TEXT NOT NULL REFERENCES users(email) ON DELETE CASCADE,
			name TEXT NOT NULL,
			ip TEXT,
			ssh_user TEXT,
			ssh_key TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			user_email TEXT NOT NULL REFERENCES users(email) ON DELETE CASCADE,
			name TEXT NOT NULL,
			target_country TEXT,
			target_language TEXT,
			timezone TEXT,
			global_blacklist JSONB,
			default_server_id TEXT REFERENCES servers(id),
			status TEXT NOT NULL DEFAULT 'draft',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`ALTER TABLE projects ADD COLUMN IF NOT EXISTS timezone TEXT;`,
		`CREATE INDEX IF NOT EXISTS idx_projects_user ON projects(user_email);`,
		`CREATE TABLE IF NOT EXISTS domains (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			server_id TEXT REFERENCES servers(id),
			url TEXT NOT NULL,
			main_keyword TEXT,
			target_country TEXT,
			target_language TEXT,
			exclude_domains TEXT,
			specific_blacklist JSONB,
			status TEXT NOT NULL DEFAULT 'waiting',
			last_generation_id TEXT,
			published_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS target_country TEXT;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS target_language TEXT;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS exclude_domains TEXT;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS published_path TEXT;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS file_count INT DEFAULT 0;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS total_size_bytes BIGINT DEFAULT 0;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_anchor_text TEXT;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_acceptor_url TEXT;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_status TEXT;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_updated_at TIMESTAMPTZ;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_last_task_id TEXT;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_file_path TEXT;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_anchor_snapshot TEXT;`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_ready_at TIMESTAMPTZ;`,
		`CREATE TABLE IF NOT EXISTS site_files (
			id TEXT PRIMARY KEY,
			domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			content_hash TEXT,
			size_bytes BIGINT NOT NULL,
			mime_type TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(domain_id, path)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_site_files_domain ON site_files(domain_id);`,
		`CREATE TABLE IF NOT EXISTS file_edits (
			id TEXT PRIMARY KEY,
			file_id TEXT NOT NULL REFERENCES site_files(id) ON DELETE CASCADE,
			edited_by TEXT NOT NULL REFERENCES users(email),
			content_before_hash TEXT,
			content_after_hash TEXT,
			edit_type TEXT NOT NULL DEFAULT 'manual',
			edit_description TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_file_edits_file ON file_edits(file_id);`,
		`CREATE INDEX IF NOT EXISTS idx_file_edits_user ON file_edits(edited_by);`,
		`CREATE INDEX IF NOT EXISTS idx_domains_project ON domains(project_id);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_domains_url ON domains(url);`,
		`CREATE TABLE IF NOT EXISTS system_prompts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			body TEXT NOT NULL,
			stage TEXT, -- Этап генерации: competitor_analysis, technical_spec, content_generation, image_generation, css_generation, js_generation, html_generation, svg_generation, 404_page
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`ALTER TABLE system_prompts ADD COLUMN IF NOT EXISTS stage TEXT;`,
		`ALTER TABLE system_prompts ADD COLUMN IF NOT EXISTS model TEXT;`, // Модель LLM: gemini-2.5-pro, gemini-2.5-flash, etc.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_system_prompts_stage_active 
			ON system_prompts(stage) 
			WHERE is_active = TRUE AND stage IS NOT NULL`,
		`CREATE TABLE IF NOT EXISTS project_members (
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			user_email TEXT NOT NULL REFERENCES users(email) ON DELETE CASCADE,
			role TEXT NOT NULL DEFAULT 'editor',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (project_id, user_email)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_email);`,
		`CREATE TABLE IF NOT EXISTS generations (
			id TEXT PRIMARY KEY,
			domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
			requested_by TEXT,
			status TEXT NOT NULL,
			progress INT NOT NULL DEFAULT 0,
			error TEXT,
			logs JSONB,
			artifacts JSONB,
			checkpoint_data JSONB,
			started_at TIMESTAMPTZ,
			finished_at TIMESTAMPTZ,
			prompt_id TEXT REFERENCES system_prompts(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`ALTER TABLE generations ADD COLUMN IF NOT EXISTS requested_by TEXT;`,
		`ALTER TABLE generations ADD COLUMN IF NOT EXISTS checkpoint_data JSONB;`,
		`ALTER TABLE generations ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0;`,
		`ALTER TABLE generations ADD COLUMN IF NOT EXISTS retryable BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE generations ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ;`,
		`ALTER TABLE generations ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMPTZ;`,
		`CREATE INDEX IF NOT EXISTS idx_generations_domain ON generations(domain_id);`,
		`CREATE TABLE IF NOT EXISTS generation_schedules (
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
		);`,
		`CREATE INDEX IF NOT EXISTS idx_gen_schedules_project ON generation_schedules(project_id);`,
		`CREATE INDEX IF NOT EXISTS idx_gen_schedules_active ON generation_schedules(is_active);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uniq_gen_schedules_project ON generation_schedules(project_id);`,
		`ALTER TABLE generation_schedules ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;`,
		`ALTER TABLE generation_schedules ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;`,
		`ALTER TABLE generation_schedules ADD COLUMN IF NOT EXISTS timezone TEXT;`,
		`CREATE TABLE IF NOT EXISTS generation_queue (
			id TEXT PRIMARY KEY,
			domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
			schedule_id TEXT REFERENCES generation_schedules(id) ON DELETE SET NULL,
			priority INT NOT NULL DEFAULT 0,
			scheduled_for TIMESTAMPTZ NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			error_message TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			processed_at TIMESTAMPTZ
		);`,
		`CREATE INDEX IF NOT EXISTS idx_gen_queue_scheduled ON generation_queue(scheduled_for, status);`,
		`CREATE INDEX IF NOT EXISTS idx_gen_queue_domain ON generation_queue(domain_id);`,
		`CREATE INDEX IF NOT EXISTS idx_gen_queue_status ON generation_queue(status);`,
		`CREATE TABLE IF NOT EXISTS link_tasks (
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
		);`,
		`ALTER TABLE link_tasks ADD COLUMN IF NOT EXISTS action TEXT NOT NULL DEFAULT 'insert';`,
		`ALTER TABLE link_tasks ADD COLUMN IF NOT EXISTS log_lines JSONB;`,
		`CREATE INDEX IF NOT EXISTS idx_link_tasks_domain ON link_tasks(domain_id);`,
		`CREATE INDEX IF NOT EXISTS idx_link_tasks_status ON link_tasks(status);`,
		`CREATE INDEX IF NOT EXISTS idx_link_tasks_scheduled ON link_tasks(scheduled_for, status);`,
		`CREATE TABLE IF NOT EXISTS link_schedules (
		  id TEXT PRIMARY KEY,
		  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		  name TEXT NOT NULL,
		  config JSONB NOT NULL,
		  is_active BOOLEAN NOT NULL DEFAULT TRUE,
		  created_by TEXT NOT NULL REFERENCES users(email),
		  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;`,
		`ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;`,
		`ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS timezone TEXT;`,
		`ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uniq_link_schedules_project ON link_schedules(project_id);`,
		`CREATE TABLE IF NOT EXISTS domain_index_checks (
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
		);`,
		`CREATE INDEX IF NOT EXISTS idx_index_checks_domain ON domain_index_checks(domain_id);`,
		`CREATE INDEX IF NOT EXISTS idx_index_checks_date ON domain_index_checks(check_date);`,
		`CREATE INDEX IF NOT EXISTS idx_index_checks_status ON domain_index_checks(status);`,
		`CREATE INDEX IF NOT EXISTS idx_index_checks_retry ON domain_index_checks(next_retry_at) WHERE status = 'checking';`,
		`CREATE TABLE IF NOT EXISTS index_check_history (
			id TEXT PRIMARY KEY,
			check_id TEXT NOT NULL REFERENCES domain_index_checks(id) ON DELETE CASCADE,
			attempt_number INT NOT NULL,
			result TEXT,
			response_data JSONB,
			error_message TEXT,
			duration_ms INT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_check_history_check ON index_check_history(check_id);`,
		`CREATE TABLE IF NOT EXISTS audit_rules (
			code TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT,
			severity TEXT NOT NULL DEFAULT 'warn',
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS audit_findings (
			id TEXT PRIMARY KEY,
			generation_id TEXT NOT NULL REFERENCES generations(id) ON DELETE CASCADE,
			domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
			rule_code TEXT NOT NULL REFERENCES audit_rules(code),
			severity TEXT NOT NULL DEFAULT 'warn',
			file_path TEXT,
			message TEXT NOT NULL,
			details JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_findings_generation ON audit_findings(generation_id);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_findings_domain ON audit_findings(domain_id);`,
		`CREATE TABLE IF NOT EXISTS api_usage (
			id TEXT PRIMARY KEY,
			user_email TEXT NOT NULL REFERENCES users(email) ON DELETE CASCADE,
			generation_id TEXT NOT NULL REFERENCES generations(id) ON DELETE CASCADE,
			tokens_used BIGINT,
			cost NUMERIC(12,6),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS api_key_usage_log (
			id TEXT PRIMARY KEY,
			user_email TEXT NOT NULL REFERENCES users(email) ON DELETE CASCADE,
			generation_id TEXT NOT NULL REFERENCES generations(id) ON DELETE CASCADE,
			key_type TEXT NOT NULL, -- 'user' или 'global'
			requests_count INT NOT NULL DEFAULT 0,
			total_tokens BIGINT NOT NULL DEFAULT 0,
			first_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_api_key_usage_user ON api_key_usage_log(user_email);`,
		`CREATE INDEX IF NOT EXISTS idx_api_key_usage_generation ON api_key_usage_log(generation_id);`,
		`CREATE INDEX IF NOT EXISTS idx_api_key_usage_date ON api_key_usage_log(first_used_at);`,
	}
	return append(stmts, projectStmts...)
}

func pingWithRetry(db *sql.DB, retries int, interval time.Duration) error {
	var lastErr error
	for i := 0; i < retries; i++ {
		if err := db.Ping(); err != nil {
			lastErr = err
			time.Sleep(interval)
			continue
		}
		return nil
	}
	return fmt.Errorf("ping failed after %d retries: %w", retries, lastErr)
}
