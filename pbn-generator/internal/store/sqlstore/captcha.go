package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type CaptchaStore struct {
	db *sql.DB
}

func NewCaptchaStore(db *sql.DB) *CaptchaStore {
	return &CaptchaStore{db: db}
}

func (s *CaptchaStore) Save(ctx context.Context, id, answerHash string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO captchas(id, answer_hash, expires_at, created_at) VALUES($1, $2, $3, $4) ON CONFLICT (id) DO UPDATE SET answer_hash = excluded.answer_hash, expires_at = excluded.expires_at`, id, answerHash, expires, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to save captcha: %w", err)
	}
	return nil
}

func (s *CaptchaStore) Consume(ctx context.Context, id, answerHash string, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM captchas WHERE id = $1 AND answer_hash = $2 AND expires_at > $3`, id, answerHash, now)
	if err != nil {
		return fmt.Errorf("failed to consume captcha: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("captcha not found or expired")
	}
	return nil
}
