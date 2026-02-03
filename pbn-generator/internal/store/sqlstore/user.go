package sqlstore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"obzornik-pbn-generator/internal/auth"
)

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) Create(ctx context.Context, email, password string) (auth.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return auth.User{}, fmt.Errorf("failed to hash password: %w", err)
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `INSERT INTO users(email, password_hash, created_at, verified, name, avatar_url, role, is_approved) VALUES($1, $2, $3, $4, $5, $6, $7, $8)`,
		email, hash, now, false, "", "", "manager", false)
	if err != nil {
		if isUnique(err) {
			return auth.User{}, errors.New("user already exists")
		}
		return auth.User{}, fmt.Errorf("failed to insert user: %w", err)
	}
	return auth.User{Email: email, PasswordHash: hash, CreatedAt: now, Verified: false, Role: "manager", IsApproved: false}, nil
}

func (s *UserStore) Authenticate(ctx context.Context, email, password string) (auth.User, error) {
	var u auth.User
	var keyUpdated sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT email, password_hash, created_at, verified, name, avatar_url, role, is_approved, api_key_updated_at FROM users WHERE email = $1`, email).
		Scan(&u.Email, &u.PasswordHash, &u.CreatedAt, &u.Verified, &u.Name, &u.AvatarURL, &u.Role, &u.IsApproved, &keyUpdated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, auth.ErrInvalidCredentials
		}
		return auth.User{}, fmt.Errorf("failed to load user: %w", err)
	}
	if keyUpdated.Valid {
		u.APIKeySetAt = &keyUpdated.Time
	}
	hash := bytes.TrimRight(u.PasswordHash, "\x00")
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		return auth.User{}, auth.ErrInvalidCredentials
	}
	return u, nil
}

func (s *UserStore) UpdatePassword(ctx context.Context, email, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = $1 WHERE email = $2`, hash, email)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *UserStore) SetVerified(ctx context.Context, email string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET verified = true WHERE email = $1`, email)
	if err != nil {
		return fmt.Errorf("failed to update verification: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *UserStore) Get(ctx context.Context, email string) (auth.User, error) {
	var u auth.User
	var keyUpdated sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT email, password_hash, created_at, verified, name, avatar_url, role, is_approved, api_key_updated_at FROM users WHERE email = $1`, email).
		Scan(&u.Email, &u.PasswordHash, &u.CreatedAt, &u.Verified, &u.Name, &u.AvatarURL, &u.Role, &u.IsApproved, &keyUpdated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, errors.New("user not found")
		}
		return auth.User{}, fmt.Errorf("failed to load user: %w", err)
	}
	if keyUpdated.Valid {
		u.APIKeySetAt = &keyUpdated.Time
	}
	return u, nil
}

func (s *UserStore) UpdateProfile(ctx context.Context, email, name, avatarURL string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET name = $1, avatar_url = $2 WHERE email = $3`, name, avatarURL, email)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}
	return nil
}

func (s *UserStore) ChangeEmail(ctx context.Context, oldEmail, newEmail string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM email_verifications WHERE email = $1`, oldEmail); err != nil {
		return fmt.Errorf("failed to cleanup email verifications: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM password_resets WHERE email = $1`, oldEmail); err != nil {
		return fmt.Errorf("failed to cleanup password resets: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM email_changes WHERE email = $1`, oldEmail); err != nil {
		return fmt.Errorf("failed to cleanup email changes: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET email = $1 WHERE email = $2`, newEmail, oldEmail); err != nil {
		return fmt.Errorf("failed to update sessions email: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET email = $1 WHERE email = $2`, newEmail, oldEmail); err != nil {
		return fmt.Errorf("failed to update email: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit email change: %w", err)
	}
	return nil
}

func isUnique(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}

func (s *UserStore) List(ctx context.Context) ([]auth.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT email, password_hash, created_at, verified, name, avatar_url, role, is_approved, api_key_updated_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []auth.User
	for rows.Next() {
		var u auth.User
		var keyUpdated sql.NullTime
		if err := rows.Scan(&u.Email, &u.PasswordHash, &u.CreatedAt, &u.Verified, &u.Name, &u.AvatarURL, &u.Role, &u.IsApproved, &keyUpdated); err != nil {
			return nil, err
		}
		if keyUpdated.Valid {
			u.APIKeySetAt = &keyUpdated.Time
		}
		u.PasswordHash = nil
		res = append(res, u)
	}
	return res, rows.Err()
}

func (s *UserStore) UpdateRole(ctx context.Context, email, role string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET role=$1 WHERE email=$2`, role, email)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *UserStore) SetApproved(ctx context.Context, email string, approved bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET is_approved=$1 WHERE email=$2`, approved, email)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *UserStore) SetAPIKey(ctx context.Context, email string, ciphertext []byte, updatedAt time.Time) error {
	if len(ciphertext) == 0 {
		return fmt.Errorf("ciphertext is empty")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET api_key_enc=$1, api_key_updated_at=$2 WHERE email=$3`, ciphertext, updatedAt, email)
	if err != nil {
		return fmt.Errorf("failed to update api key: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *UserStore) ClearAPIKey(ctx context.Context, email string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET api_key_enc=NULL, api_key_updated_at=NULL WHERE email=$1`, email)
	if err != nil {
		return fmt.Errorf("failed to clear api key: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *UserStore) GetAPIKey(ctx context.Context, email string) ([]byte, *time.Time, error) {
	var enc []byte
	var updated sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT api_key_enc, api_key_updated_at FROM users WHERE email=$1`, email).Scan(&enc, &updated)
	if err != nil {
		return nil, nil, err
	}
	var ts *time.Time
	if updated.Valid {
		ts = &updated.Time
	}
	return enc, ts, nil
}
