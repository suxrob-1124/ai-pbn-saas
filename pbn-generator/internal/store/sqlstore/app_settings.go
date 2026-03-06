package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// AppSettingsStore provides access to the app_settings key-value table.
type AppSettingsStore struct {
	db *sql.DB
}

func NewAppSettingsStore(db *sql.DB) *AppSettingsStore {
	return &AppSettingsStore{db: db}
}

// Get returns the value for the given key.
func (s *AppSettingsStore) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key=$1`, key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("app_settings get %q: %w", key, err)
	}
	return value, nil
}

// Set upserts the value for the given key.
func (s *AppSettingsStore) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO app_settings (key, value, updated_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (key) DO UPDATE SET value=$2, updated_at=NOW()`, key, value)
	return err
}

// GetBool returns the boolean value for the given key ("true"/"false").
func (s *AppSettingsStore) GetBool(ctx context.Context, key string) (bool, error) {
	val, err := s.Get(ctx, key)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(val), "true"), nil
}

// SetBool stores a boolean value as "true" or "false".
func (s *AppSettingsStore) SetBool(ctx context.Context, key string, val bool) error {
	v := "false"
	if val {
		v = "true"
	}
	return s.Set(ctx, key, v)
}
