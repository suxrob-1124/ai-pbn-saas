package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type AuditRule struct {
	Code        string
	Title       string
	Description string
	Severity    string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AuditFinding struct {
	ID           string
	GenerationID string
	DomainID     string
	RuleCode     string
	Severity     string
	FilePath     sql.NullString
	Message      string
	Details      []byte
	CreatedAt    time.Time
}

type AuditStore struct {
	db *sql.DB
}

func NewAuditStore(db *sql.DB) *AuditStore {
	return &AuditStore{db: db}
}

func (s *AuditStore) ListRules(ctx context.Context) ([]AuditRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT code, title, description, severity, is_active, created_at, updated_at
FROM audit_rules ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []AuditRule
	for rows.Next() {
		var r AuditRule
		var desc sql.NullString
		if err := rows.Scan(&r.Code, &r.Title, &desc, &r.Severity, &r.IsActive, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		if desc.Valid {
			r.Description = desc.String
		}
		res = append(res, r)
	}
	return res, rows.Err()
}

func (s *AuditStore) GetRule(ctx context.Context, code string) (AuditRule, error) {
	var r AuditRule
	var desc sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT code, title, description, severity, is_active, created_at, updated_at
FROM audit_rules WHERE code=$1`, code).
		Scan(&r.Code, &r.Title, &desc, &r.Severity, &r.IsActive, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return AuditRule{}, err
	}
	if desc.Valid {
		r.Description = desc.String
	}
	return r, nil
}

func (s *AuditStore) CreateRule(ctx context.Context, r AuditRule) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_rules(code, title, description, severity, is_active, created_at, updated_at)
		VALUES($1,$2,$3,$4,$5,NOW(),NOW())`, r.Code, r.Title, nullableString(sql.NullString{String: r.Description, Valid: r.Description != ""}), r.Severity, r.IsActive)
	if err != nil {
		return fmt.Errorf("failed to create audit rule: %w", err)
	}
	return nil
}

func (s *AuditStore) UpdateRule(ctx context.Context, r AuditRule) error {
	_, err := s.db.ExecContext(ctx, `UPDATE audit_rules
		SET title=$1, description=$2, severity=$3, is_active=$4, updated_at=NOW()
		WHERE code=$5`, r.Title, nullableString(sql.NullString{String: r.Description, Valid: r.Description != ""}), r.Severity, r.IsActive, r.Code)
	if err != nil {
		return fmt.Errorf("failed to update audit rule: %w", err)
	}
	return nil
}

func (s *AuditStore) DeleteRule(ctx context.Context, code string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM audit_rules WHERE code=$1`, code)
	if err != nil {
		return fmt.Errorf("failed to delete audit rule: %w", err)
	}
	return nil
}

func (s *AuditStore) UpsertRules(ctx context.Context, rules []AuditRule) error {
	if len(rules) == 0 {
		return nil
	}
	for _, r := range rules {
		_, err := s.db.ExecContext(ctx, `INSERT INTO audit_rules(code, title, description, severity, is_active, created_at, updated_at)
			VALUES($1,$2,$3,$4,$5,NOW(),NOW())
			ON CONFLICT (code) DO UPDATE
			SET title=EXCLUDED.title,
				description=EXCLUDED.description,
				severity=EXCLUDED.severity,
				is_active=EXCLUDED.is_active,
				updated_at=NOW()`,
			r.Code, r.Title, r.Description, r.Severity, r.IsActive)
		if err != nil {
			return fmt.Errorf("upsert audit rule %s: %w", r.Code, err)
		}
	}
	return nil
}

func (s *AuditStore) ReplaceFindings(ctx context.Context, generationID string, findings []AuditFinding) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM audit_findings WHERE generation_id=$1`, generationID); err != nil {
		return fmt.Errorf("delete findings: %w", err)
	}

	for _, f := range findings {
		details := f.Details
		if len(details) == 0 {
			details, _ = json.Marshal(map[string]any{})
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO audit_findings(id, generation_id, domain_id, rule_code, severity, file_path, message, details, created_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOW())`,
			f.ID, f.GenerationID, f.DomainID, f.RuleCode, f.Severity, nullableString(f.FilePath), f.Message, details)
		if err != nil {
			return fmt.Errorf("insert finding %s: %w", f.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit findings: %w", err)
	}
	return nil
}
