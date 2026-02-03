package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type EmailChangeStore struct {
	db *sql.DB
}

func NewEmailChangeStore(db *sql.DB) *EmailChangeStore {
	return &EmailChangeStore{db: db}
}

func (s *EmailChangeStore) SaveChange(ctx context.Context, email, newEmail, tokenHash string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO email_changes(token_hash, email, new_email, expires_at, used_at) VALUES($1, $2, $3, $4, NULL) ON CONFLICT (token_hash) DO UPDATE SET email = excluded.email, new_email = excluded.new_email, expires_at = excluded.expires_at, used_at = NULL`, tokenHash, email, newEmail, expires)
	if err != nil {
		return fmt.Errorf("failed to save email change: %w", err)
	}
	return nil
}

func (s *EmailChangeStore) ConsumeChange(ctx context.Context, tokenHash string, now time.Time) (string, string, error) {
	var (
		email    string
		newEmail string
		expires  time.Time
		usedAt   sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, `SELECT email, new_email, expires_at, used_at FROM email_changes WHERE token_hash = $1`, tokenHash).Scan(&email, &newEmail, &expires, &usedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("token not found or expired")
		}
		return "", "", fmt.Errorf("failed to fetch email change token: %w", err)
	}
	if expires.Before(now) {
		return "", "", fmt.Errorf("token not found or expired")
	}
	if usedAt.Valid {
		return email, newEmail, nil
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE email_changes SET used_at = $1 WHERE token_hash = $2`, now, tokenHash); err != nil {
		return "", "", fmt.Errorf("failed to consume email change token: %w", err)
	}
	return email, newEmail, nil
}
