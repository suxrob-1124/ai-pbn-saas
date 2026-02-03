package db

import (
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

func expectContains(t *testing.T, stmts []string, needle string) {
	t.Helper()
	for _, stmt := range stmts {
		if strings.Contains(stmt, needle) {
			return
		}
	}
	t.Fatalf("expected migration statement to contain: %s", needle)
}
