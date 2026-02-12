package db

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMigrateExecutesAllStatements(t *testing.T) {
	t.Parallel()

	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer conn.Close()

	for _, stmt := range migrationStatements() {
		mock.ExpectExec(regexp.QuoteMeta(stmt)).
			WillReturnResult(sqlmock.NewResult(0, 0))
	}

	if err := Migrate(conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestMigrateReturnsError(t *testing.T) {
	t.Parallel()

	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer conn.Close()

	stmts := migrationStatements()
	if len(stmts) == 0 {
		t.Fatal("migrationStatements returned no statements")
	}

	mock.ExpectExec(regexp.QuoteMeta(stmts[0])).
		WillReturnError(errors.New("boom"))

	if err := Migrate(conn); err == nil {
		t.Fatal("expected Migrate to return error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestMigrationStatementsIncludeFileStorage(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "CREATE TABLE IF NOT EXISTS site_files")
	expectContains(t, stmts, "CREATE TABLE IF NOT EXISTS file_edits")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_site_files_domain")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_file_edits_file")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_file_edits_user")
	expectContains(t, stmts, "ALTER TABLE domains ADD COLUMN IF NOT EXISTS published_path TEXT;")
	expectContains(t, stmts, "ALTER TABLE domains ADD COLUMN IF NOT EXISTS file_count INT DEFAULT 0;")
	expectContains(t, stmts, "ALTER TABLE domains ADD COLUMN IF NOT EXISTS total_size_bytes BIGINT DEFAULT 0;")
}

func TestMigrationStatementsIncludeGenerationSchedules(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "CREATE TABLE IF NOT EXISTS generation_schedules")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_gen_schedules_project ON generation_schedules(project_id);")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_gen_schedules_active ON generation_schedules(is_active);")
}

func TestMigrationStatementsIncludeScheduleMetadata(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "ALTER TABLE generation_schedules ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;")
	expectContains(t, stmts, "ALTER TABLE generation_schedules ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;")
	expectContains(t, stmts, "ALTER TABLE generation_schedules ADD COLUMN IF NOT EXISTS timezone TEXT;")
}

func TestMigrationStatementsIncludeGenerationQueue(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "CREATE TABLE IF NOT EXISTS generation_queue")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_gen_queue_scheduled ON generation_queue(scheduled_for, status);")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_gen_queue_domain ON generation_queue(domain_id);")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_gen_queue_status ON generation_queue(status);")
}

func TestMigrationStatementsIncludeGenerationRetry(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "ALTER TABLE generations ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0;")
	expectContains(t, stmts, "ALTER TABLE generations ADD COLUMN IF NOT EXISTS retryable BOOLEAN NOT NULL DEFAULT FALSE;")
	expectContains(t, stmts, "ALTER TABLE generations ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ;")
	expectContains(t, stmts, "ALTER TABLE generations ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMPTZ;")
}

func TestMigrationStatementsIncludeLinkTasks(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "CREATE TABLE IF NOT EXISTS link_tasks")
	expectContains(t, stmts, "ALTER TABLE link_tasks ADD COLUMN IF NOT EXISTS action TEXT NOT NULL DEFAULT 'insert';")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_link_tasks_domain ON link_tasks(domain_id);")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_link_tasks_status ON link_tasks(status);")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_link_tasks_scheduled ON link_tasks(scheduled_for, status);")
}

func TestMigrationStatementsIncludeLinkSchedules(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "CREATE TABLE IF NOT EXISTS link_schedules")
}

func TestMigrationStatementsIncludeLinkSchedulesMetadata(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;")
	expectContains(t, stmts, "ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;")
	expectContains(t, stmts, "ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS timezone TEXT;")
	expectContains(t, stmts, "ALTER TABLE link_schedules ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();")
}

func TestMigrationStatementsIncludeScheduleUniq(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "CREATE UNIQUE INDEX IF NOT EXISTS uniq_gen_schedules_project ON generation_schedules(project_id);")
	expectContains(t, stmts, "CREATE UNIQUE INDEX IF NOT EXISTS uniq_link_schedules_project ON link_schedules(project_id);")
}

func TestMigrationStatementsIncludeDomainLinkFields(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_anchor_text TEXT;")
	expectContains(t, stmts, "ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_acceptor_url TEXT;")
	expectContains(t, stmts, "ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_status TEXT;")
	expectContains(t, stmts, "ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_updated_at TIMESTAMPTZ;")
	expectContains(t, stmts, "ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_last_task_id TEXT;")
	expectContains(t, stmts, "ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_file_path TEXT;")
	expectContains(t, stmts, "ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_anchor_snapshot TEXT;")
	expectContains(t, stmts, "ALTER TABLE domains ADD COLUMN IF NOT EXISTS link_ready_at TIMESTAMPTZ;")
}

func TestMigrationStatementsIncludeProjectTimezone(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "ALTER TABLE projects ADD COLUMN IF NOT EXISTS timezone TEXT;")
}

func TestMigrationStatementsIncludeDomainIndexChecks(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "CREATE TABLE IF NOT EXISTS domain_index_checks")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_index_checks_domain ON domain_index_checks(domain_id);")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_index_checks_date ON domain_index_checks(check_date);")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_index_checks_status ON domain_index_checks(status);")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_index_checks_retry ON domain_index_checks(next_retry_at) WHERE status = 'checking';")
}

func TestMigrationStatementsIncludeIndexCheckHistory(t *testing.T) {
	t.Parallel()

	stmts := migrationStatements()
	expectContains(t, stmts, "CREATE TABLE IF NOT EXISTS index_check_history")
	expectContains(t, stmts, "CREATE INDEX IF NOT EXISTS idx_check_history_check ON index_check_history(check_id);")
}

func expectContains(t *testing.T, stmts []string, needle string) {
	t.Helper()
	for _, stmt := range stmts {
		if strings.Contains(stmt, needle) {
			return
		}
	}
	t.Fatalf("expected migration statement to contain: %s", needle)
}
