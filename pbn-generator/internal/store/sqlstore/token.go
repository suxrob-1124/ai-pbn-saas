package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type VerificationTokenStore struct {
	db *sql.DB
}

func NewVerificationTokenStore(db *sql.DB) *VerificationTokenStore {
	return &VerificationTokenStore{db: db}
}

func (s *VerificationTokenStore) SaveVerification(ctx context.Context, email, tokenHash string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO email_verifications(token_hash, email, expires_at, used_at) VALUES($1, $2, $3, NULL) ON CONFLICT (token_hash) DO UPDATE SET email = excluded.email, expires_at = excluded.expires_at, used_at = NULL`, tokenHash, email, expires)
	if err != nil {
		return fmt.Errorf("failed to save verification token: %w", err)
	}
	return nil
}

func (s *VerificationTokenStore) ConsumeVerification(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	var (
		email   string
		expires time.Time
		usedAt  sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, `SELECT email, expires_at, used_at FROM email_verifications WHERE token_hash = $1`, tokenHash).Scan(&email, &expires, &usedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("token not found or expired")
		}
		return "", fmt.Errorf("failed to fetch verification token: %w", err)
	}
	if expires.Before(now) {
		return "", fmt.Errorf("token not found or expired")
	}
	if usedAt.Valid {
		// токен уже применён — возвращаем email для идемпотентности
		return email, nil
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE email_verifications SET used_at = $1 WHERE token_hash = $2`, now, tokenHash); err != nil {
		return "", fmt.Errorf("failed to consume verification token: %w", err)
	}
	return email, nil
}

type ResetTokenStore struct {
	db *sql.DB
}

func NewResetTokenStore(db *sql.DB) *ResetTokenStore {
	return &ResetTokenStore{db: db}
}

func (s *ResetTokenStore) SaveReset(ctx context.Context, email, tokenHash string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO password_resets(token_hash, email, expires_at) VALUES($1, $2, $3) ON CONFLICT (token_hash) DO UPDATE SET email = excluded.email, expires_at = excluded.expires_at`, tokenHash, email, expires)
	if err != nil {
		return fmt.Errorf("failed to save reset token: %w", err)
	}
	return nil
}

func (s *ResetTokenStore) ConsumeReset(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	var email string
	err := s.db.QueryRowContext(ctx, `DELETE FROM password_resets WHERE token_hash = $1 AND expires_at > $2 RETURNING email`, tokenHash, now).Scan(&email)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("token not found or expired")
		}
		return "", fmt.Errorf("failed to consume reset token: %w", err)
	}
	return email, nil
}
