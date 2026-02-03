package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type SystemPrompt struct {
	ID          string
	Name        string
	Description sql.NullString
	Body        string
	Stage       sql.NullString // Этап генерации: competitor_analysis, technical_spec, content_generation, etc.
	Model       sql.NullString // Модель LLM: gemini-2.5-pro, gemini-2.5-flash, etc.
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PromptStore struct {
	db *sql.DB
}

func NewPromptStore(db *sql.DB) *PromptStore {
	return &PromptStore{db: db}
}

func (s *PromptStore) Create(ctx context.Context, p SystemPrompt) error {
	// Если промпт активен и имеет этап, деактивируем другие промпты этого этапа
	if p.IsActive && p.Stage.Valid && p.Stage.String != "" {
		_, err := s.db.ExecContext(ctx, `UPDATE system_prompts SET is_active = FALSE, updated_at = NOW() 
			WHERE stage = $1 AND is_active = TRUE AND id != $2`, p.Stage.String, p.ID)
		if err != nil {
			return fmt.Errorf("failed to deactivate other prompts: %w", err)
		}
	}
	
	_, err := s.db.ExecContext(ctx, `INSERT INTO system_prompts (id, name, description, body, stage, model, is_active, created_at, updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`, p.ID, p.Name, nullableString(p.Description), p.Body, nullableString(p.Stage), nullableString(p.Model), p.IsActive)
	if err != nil {
		// Проверяем, является ли ошибка нарушением уникального индекса
		if strings.Contains(err.Error(), "idx_system_prompts_stage_active") || 
		   strings.Contains(err.Error(), "unique constraint") {
			return fmt.Errorf("only one active prompt allowed per stage")
		}
		return fmt.Errorf("failed to create prompt: %w", err)
	}
	return nil
}

func (s *PromptStore) Update(ctx context.Context, p SystemPrompt) error {
	// Если промпт активируется и имеет этап, деактивируем другие промпты этого этапа
	if p.IsActive && p.Stage.Valid && p.Stage.String != "" {
		_, err := s.db.ExecContext(ctx, `UPDATE system_prompts SET is_active = FALSE, updated_at = NOW() 
			WHERE stage = $1 AND is_active = TRUE AND id != $2`, p.Stage.String, p.ID)
		if err != nil {
			return fmt.Errorf("failed to deactivate other prompts: %w", err)
		}
	}
	
	_, err := s.db.ExecContext(ctx, `UPDATE system_prompts SET name=$1, description=$2, body=$3, stage=$4, model=$5, is_active=$6, updated_at=NOW() WHERE id=$7`,
		p.Name, nullableString(p.Description), p.Body, nullableString(p.Stage), nullableString(p.Model), p.IsActive, p.ID)
	if err != nil {
		// Проверяем, является ли ошибка нарушением уникального индекса
		if strings.Contains(err.Error(), "idx_system_prompts_stage_active") || 
		   strings.Contains(err.Error(), "unique constraint") {
			return fmt.Errorf("only one active prompt allowed per stage")
		}
		return fmt.Errorf("failed to update prompt: %w", err)
	}
	return nil
}

func (s *PromptStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM system_prompts WHERE id=$1`, id)
	return err
}

func (s *PromptStore) List(ctx context.Context) ([]SystemPrompt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, description, body, stage, model, is_active, created_at, updated_at FROM system_prompts ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []SystemPrompt
	for rows.Next() {
		var p SystemPrompt
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Body, &p.Stage, &p.Model, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, p)
	}
	return res, rows.Err()
}

func (s *PromptStore) Get(ctx context.Context, id string) (SystemPrompt, error) {
	var p SystemPrompt
	err := s.db.QueryRowContext(ctx, `SELECT id, name, description, body, stage, model, is_active, created_at, updated_at FROM system_prompts WHERE id=$1`, id).
		Scan(&p.ID, &p.Name, &p.Description, &p.Body, &p.Stage, &p.Model, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s *PromptStore) GetActive(ctx context.Context) (SystemPrompt, error) {
	var p SystemPrompt
	err := s.db.QueryRowContext(ctx, `SELECT id, name, description, body, stage, model, is_active, created_at, updated_at FROM system_prompts WHERE is_active = TRUE ORDER BY updated_at DESC LIMIT 1`).
		Scan(&p.ID, &p.Name, &p.Description, &p.Body, &p.Stage, &p.Model, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// GetByStage получает активный промпт для конкретного этапа генерации
func (s *PromptStore) GetByStage(ctx context.Context, stage string) (SystemPrompt, error) {
	var p SystemPrompt
	err := s.db.QueryRowContext(ctx, `SELECT id, name, description, body, stage, model, is_active, created_at, updated_at FROM system_prompts WHERE stage=$1 AND is_active = TRUE ORDER BY updated_at DESC LIMIT 1`, stage).
		Scan(&p.ID, &p.Name, &p.Description, &p.Body, &p.Stage, &p.Model, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
