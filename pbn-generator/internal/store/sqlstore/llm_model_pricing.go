package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type LLMModelPricing struct {
	ID                string
	Provider          string
	Model             string
	InputUSDPerMillion  float64
	OutputUSDPerMillion float64
	ActiveFrom        time.Time
	ActiveTo          sql.NullTime
	IsActive          bool
	UpdatedBy         string
	UpdatedAt         time.Time
}

type ModelPricingStore struct {
	db *sql.DB
}

func NewModelPricingStore(db *sql.DB) *ModelPricingStore {
	return &ModelPricingStore{db: db}
}

func (s *ModelPricingStore) GetActiveByModel(ctx context.Context, provider, model string, at time.Time) (*LLMModelPricing, error) {
	var item LLMModelPricing
	err := s.db.QueryRowContext(ctx, `SELECT id, provider, model, input_usd_per_million, output_usd_per_million, active_from, active_to, is_active, updated_by, updated_at
FROM llm_model_pricing
WHERE provider=$1 AND model=$2
  AND active_from <= $3
  AND (active_to IS NULL OR active_to > $3)
  AND is_active = TRUE
ORDER BY active_from DESC
LIMIT 1`, provider, model, at.UTC()).
		Scan(
			&item.ID,
			&item.Provider,
			&item.Model,
			&item.InputUSDPerMillion,
			&item.OutputUSDPerMillion,
			&item.ActiveFrom,
			&item.ActiveTo,
			&item.IsActive,
			&item.UpdatedBy,
			&item.UpdatedAt,
		)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *ModelPricingStore) ListActive(ctx context.Context) ([]LLMModelPricing, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, provider, model, input_usd_per_million, output_usd_per_million, active_from, active_to, is_active, updated_by, updated_at
FROM llm_model_pricing
WHERE is_active = TRUE
ORDER BY provider ASC, model ASC, active_from DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list llm pricing: %w", err)
	}
	defer rows.Close()

	result := make([]LLMModelPricing, 0)
	for rows.Next() {
		var item LLMModelPricing
		if err := rows.Scan(
			&item.ID,
			&item.Provider,
			&item.Model,
			&item.InputUSDPerMillion,
			&item.OutputUSDPerMillion,
			&item.ActiveFrom,
			&item.ActiveTo,
			&item.IsActive,
			&item.UpdatedBy,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan llm pricing: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *ModelPricingStore) UpsertActive(ctx context.Context, provider, model string, inputUSDPerMillion, outputUSDPerMillion float64, updatedBy string, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin pricing tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := at.UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE llm_model_pricing
SET is_active=FALSE, active_to=$3, updated_at=$3, updated_by=$4
WHERE provider=$1 AND model=$2 AND is_active=TRUE`, provider, model, now, updatedBy); err != nil {
		return fmt.Errorf("failed to deactivate previous pricing: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO llm_model_pricing(
	id, provider, model, input_usd_per_million, output_usd_per_million, active_from, active_to, is_active, updated_by, updated_at
) VALUES($1,$2,$3,$4,$5,$6,NULL,TRUE,$7,$6)`,
		uuid.NewString(),
		provider,
		model,
		inputUSDPerMillion,
		outputUSDPerMillion,
		now,
		updatedBy,
	); err != nil {
		return fmt.Errorf("failed to insert active pricing: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit pricing upsert: %w", err)
	}
	return nil
}
