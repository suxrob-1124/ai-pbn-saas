package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"obzornik-pbn-generator/internal/db"
)

func main() {
	dsn := env("DB_DSN", "postgres://auth:auth@localhost:5432/auth?sslmode=disable")
	driver := env("DB_DRIVER", "pgx")

	conn, err := sql.Open(driver, dsn)
	if err != nil {
		fatalf("failed to open database: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		fatalf("failed to ping database: %v", err)
	}

	if err := db.Migrate(conn); err != nil {
		fatalf("migration failed: %v", err)
	}

	if err := verifySchema(ctx, conn); err != nil {
		fatalf("schema verification failed: %v", err)
	}

	if err := verifyConstraints(ctx, conn); err != nil {
		fatalf("constraint verification failed: %v", err)
	}

	fmt.Println("OK: file storage migrations verified")
}

func verifySchema(ctx context.Context, conn *sql.DB) error {
	tables := []string{"site_files", "file_edits"}
	for _, table := range tables {
		exists, err := tableExists(ctx, conn, table)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("table %s does not exist", table)
		}
	}

	domainCols, err := columnsForTable(ctx, conn, "domains")
	if err != nil {
		return err
	}

	required := []string{"published_path", "file_count", "total_size_bytes"}
	for _, col := range required {
		if !domainCols[col] {
			return fmt.Errorf("domains.%s column is missing", col)
		}
	}
	return nil
}

func verifyConstraints(ctx context.Context, conn *sql.DB) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	seed := uuid.NewString()
	userEmail := fmt.Sprintf("verify_%s@example.com", seed)
	projectID := "proj_" + seed
	domainID := "dom_" + seed
	fileID := "file_" + seed
	editID := "edit_" + seed

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (email, password_hash, created_at, verified, role, is_approved)
		 VALUES ($1, $2, NOW(), TRUE, 'admin', TRUE)`,
		userEmail, []byte("verify")); err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO projects (id, user_email, name)
		 VALUES ($1, $2, $3)`,
		projectID, userEmail, "verify project"); err != nil {
		return fmt.Errorf("insert project: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO domains (id, project_id, url)
		 VALUES ($1, $2, $3)`,
		domainID, projectID, fmt.Sprintf("verify-%s.example.com", seed)); err != nil {
		return fmt.Errorf("insert domain: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO site_files (id, domain_id, path, content_hash, size_bytes, mime_type)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		fileID, domainID, "index.html", nil, int64(1), "text/html"); err != nil {
		return fmt.Errorf("insert site_files: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO file_edits (id, file_id, edited_by, content_before_hash, content_after_hash, edit_type)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		editID, fileID, userEmail, nil, nil, "manual"); err != nil {
		return fmt.Errorf("insert file_edits: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO site_files (id, domain_id, path, content_hash, size_bytes, mime_type)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		"dup_"+fileID, domainID, "index.html", nil, int64(1), "text/html"); err == nil {
		return fmt.Errorf("expected unique constraint violation on site_files(domain_id, path)")
	}

	return nil
}

func tableExists(ctx context.Context, conn *sql.DB, table string) (bool, error) {
	var name sql.NullString
	if err := conn.QueryRowContext(ctx, `SELECT to_regclass($1)`, "public."+table).Scan(&name); err != nil {
		return false, fmt.Errorf("check table %s: %w", table, err)
	}
	return name.Valid, nil
}

func columnsForTable(ctx context.Context, conn *sql.DB, table string) (map[string]bool, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1`, table)
	if err != nil {
		return nil, fmt.Errorf("list columns for %s: %w", table, err)
	}
	defer rows.Close()

	cols := make(map[string]bool)
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, fmt.Errorf("scan column for %s: %w", table, err)
		}
		cols[col] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error for %s: %w", table, err)
	}
	return cols, nil
}

func env(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
