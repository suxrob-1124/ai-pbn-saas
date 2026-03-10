package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"obzornik-pbn-generator/internal/auth"
)

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) Create(ctx context.Context, session auth.Session) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sessions(token, email, expires_at) VALUES($1, $2, $3)`, session.JTI, session.Email, session.ExpiresAt); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

func (s *SessionStore) CleanupExpiredForEmail(ctx context.Context, email string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE email = $1 OR expires_at <= $2`, email, time.Now()); err != nil {
		return fmt.Errorf("failed to cleanup sessions: %w", err)
	}
	return nil
}

func (s *SessionStore) CleanupExpired(ctx context.Context, now time.Time) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now); err != nil {
		return fmt.Errorf("failed to clean expired sessions: %w", err)
	}
	return nil
}

func (s *SessionStore) Get(ctx context.Context, jti string) (auth.Session, error) {
	var sess auth.Session
	err := s.db.QueryRowContext(ctx, `SELECT token, email, expires_at FROM sessions WHERE token = $1`, jti).
		Scan(&sess.JTI, &sess.Email, &sess.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.Session{}, fmt.Errorf("not found")
		}
		return auth.Session{}, fmt.Errorf("failed to load session: %w", err)
	}
	return sess, nil
}

func (s *SessionStore) Renew(ctx context.Context, jti string, expiresAt time.Time) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE sessions SET expires_at = $1 WHERE token = $2`, expiresAt, jti); err != nil {
		return fmt.Errorf("failed to renew session: %w", err)
	}
	return nil
}

func (s *SessionStore) Delete(ctx context.Context, jti string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = $1`, jti); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

func (s *SessionStore) RevokeByEmail(ctx context.Context, email string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE email = $1`, email); err != nil {
		return fmt.Errorf("failed to revoke sessions: %w", err)
	}
	return nil
}

func (s *SessionStore) RevokeAll(ctx context.Context, email string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE email = $1`, email)
	if err != nil {
		return fmt.Errorf("failed to revoke all sessions: %w", err)
	}
	return nil
}

func (s *SessionStore) RevokeAllExcept(ctx context.Context, email, keepJTI string) error {
	if keepJTI == "" {
		return s.RevokeAll(ctx, email)
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE email = $1 AND token <> $2`, email, keepJTI)
	if err != nil {
		return fmt.Errorf("failed to revoke sessions except current: %w", err)
	}
	return nil
}

func (s *SessionStore) UpdateEmail(ctx context.Context, oldEmail, newEmail string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET email = $1 WHERE email = $2`, newEmail, oldEmail)
	if err != nil {
		return fmt.Errorf("failed to update sessions email: %w", err)
	}
	return nil
}
