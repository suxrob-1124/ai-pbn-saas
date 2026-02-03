package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// APIKeyUsageLog представляет запись об использовании API ключа
type APIKeyUsageLog struct {
	ID            string
	UserEmail     string
	GenerationID  string
	KeyType       string // "user" или "global"
	RequestsCount int
	TotalTokens   int64
	FirstUsedAt   time.Time
	LastUsedAt    time.Time
}

// APIKeyUsageStore управляет логами использования API ключей
type APIKeyUsageStore struct {
	db *sql.DB
}

// NewAPIKeyUsageStore создает новый APIKeyUsageStore
func NewAPIKeyUsageStore(db *sql.DB) *APIKeyUsageStore {
	return &APIKeyUsageStore{db: db}
}

// LogUsage логирует использование API ключа для генерации
func (s *APIKeyUsageStore) LogUsage(ctx context.Context, userEmail, generationID, keyType string, requestsCount int, totalTokens int64) error {
	if userEmail == "" || generationID == "" || keyType == "" {
		return errors.New("user_email, generation_id and key_type are required")
	}
	if keyType != "user" && keyType != "global" {
		return fmt.Errorf("invalid key_type: %s (allowed: user, global)", keyType)
	}

	now := time.Now().UTC()
	
	// Проверяем, есть ли уже запись для этой генерации
	var existingID string
	err := s.db.QueryRowContext(ctx, 
		`SELECT id FROM api_key_usage_log WHERE generation_id=$1 AND user_email=$2`,
		generationID, userEmail).Scan(&existingID)
	
	if err == nil && existingID != "" {
		// Обновляем существующую запись
		_, err = s.db.ExecContext(ctx, `
			UPDATE api_key_usage_log 
			SET requests_count = requests_count + $1,
			    total_tokens = total_tokens + $2,
			    last_used_at = $3
			WHERE id = $4`,
			requestsCount, totalTokens, now, existingID)
		return err
	}
	
	// Создаем новую запись
	id := uuid.NewString()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO api_key_usage_log (id, user_email, generation_id, key_type, requests_count, total_tokens, first_used_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, userEmail, generationID, keyType, requestsCount, totalTokens, now, now)
	return err
}

// GetUsageByUser возвращает статистику использования API ключей пользователя
func (s *APIKeyUsageStore) GetUsageByUser(ctx context.Context, userEmail string, limit int) ([]APIKeyUsageLog, error) {
	if userEmail == "" {
		return nil, errors.New("user_email is required")
	}
	if limit <= 0 {
		limit = 100
	}
	
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_email, generation_id, key_type, requests_count, total_tokens, first_used_at, last_used_at
		FROM api_key_usage_log
		WHERE user_email = $1
		ORDER BY first_used_at DESC
		LIMIT $2`,
		userEmail, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage by user: %w", err)
	}
	defer rows.Close()
	
	var logs []APIKeyUsageLog
	for rows.Next() {
		var log APIKeyUsageLog
		if err := rows.Scan(&log.ID, &log.UserEmail, &log.GenerationID, &log.KeyType, 
			&log.RequestsCount, &log.TotalTokens, &log.FirstUsedAt, &log.LastUsedAt); err != nil {
			return nil, fmt.Errorf("failed to scan usage log: %w", err)
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

// GetUsageByGeneration возвращает информацию об использовании API ключа для конкретной генерации
func (s *APIKeyUsageStore) GetUsageByGeneration(ctx context.Context, generationID string) (*APIKeyUsageLog, error) {
	if generationID == "" {
		return nil, errors.New("generation_id is required")
	}
	
	var log APIKeyUsageLog
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_email, generation_id, key_type, requests_count, total_tokens, first_used_at, last_used_at
		FROM api_key_usage_log
		WHERE generation_id = $1`,
		generationID).Scan(&log.ID, &log.UserEmail, &log.GenerationID, &log.KeyType,
		&log.RequestsCount, &log.TotalTokens, &log.FirstUsedAt, &log.LastUsedAt)
	
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("usage log not found")
		}
		return nil, fmt.Errorf("failed to get usage by generation: %w", err)
	}
	return &log, nil
}

// GetTotalUsageByUser возвращает общую статистику использования API ключей пользователя
func (s *APIKeyUsageStore) GetTotalUsageByUser(ctx context.Context, userEmail string) (totalRequests int, totalTokens int64, err error) {
	if userEmail == "" {
		return 0, 0, errors.New("user_email is required")
	}
	
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(requests_count), 0), COALESCE(SUM(total_tokens), 0)
		FROM api_key_usage_log
		WHERE user_email = $1`,
		userEmail).Scan(&totalRequests, &totalTokens)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get total usage: %w", err)
	}
	return totalRequests, totalTokens, nil
}

