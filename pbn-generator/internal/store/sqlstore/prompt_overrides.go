package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	PromptScopeProject = "project"
	PromptScopeDomain  = "domain"
)

var GenerationPromptStages = []string{
	"competitor_analysis",
	"technical_spec",
	"content_generation",
	"design_architecture",
	"logo_generation",
	"html_generation",
	"css_generation",
	"js_generation",
	"image_prompt_generation",
	"404_page",
	"editor_file_edit",
	"editor_page_create",
	"editor_asset_regenerate",
}

type PromptOverride struct {
	ID            string
	ScopeType     string
	ScopeID       string
	Stage         string
	Body          string
	Model         sql.NullString
	BasedOnPrompt sql.NullString
	UpdatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ResolvedPrompt struct {
	Stage           string
	Source          string // domain|project|global
	PromptID        sql.NullString
	OverrideID      sql.NullString
	Body            string
	Model           sql.NullString
	BasedOnPromptID sql.NullString
}

type PromptOverrideStore struct {
	db *sql.DB
}

func NewPromptOverrideStore(db *sql.DB) *PromptOverrideStore {
	return &PromptOverrideStore{db: db}
}

func (s *PromptOverrideStore) Upsert(ctx context.Context, item PromptOverride) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO prompt_overrides(id, scope_type, scope_id, stage, body, model, based_on_prompt_id, updated_by, created_at, updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
ON CONFLICT (scope_type, scope_id, stage)
DO UPDATE SET body=EXCLUDED.body, model=EXCLUDED.model, based_on_prompt_id=EXCLUDED.based_on_prompt_id, updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
		item.ID,
		item.ScopeType,
		item.ScopeID,
		item.Stage,
		item.Body,
		nullableString(item.Model),
		nullableString(item.BasedOnPrompt),
		item.UpdatedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert prompt override: %w", err)
	}
	return nil
}

func (s *PromptOverrideStore) Delete(ctx context.Context, scopeType, scopeID, stage string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM prompt_overrides WHERE scope_type=$1 AND scope_id=$2 AND stage=$3`, scopeType, scopeID, stage)
	return err
}

func (s *PromptOverrideStore) ListByScope(ctx context.Context, scopeType, scopeID string) ([]PromptOverride, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, scope_type, scope_id, stage, body, model, based_on_prompt_id, updated_by, created_at, updated_at
FROM prompt_overrides
WHERE scope_type=$1 AND scope_id=$2
ORDER BY stage ASC`, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []PromptOverride
	for rows.Next() {
		var item PromptOverride
		if err := rows.Scan(&item.ID, &item.ScopeType, &item.ScopeID, &item.Stage, &item.Body, &item.Model, &item.BasedOnPrompt, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

func (s *PromptOverrideStore) GetByScopeStage(ctx context.Context, scopeType, scopeID, stage string) (PromptOverride, error) {
	var item PromptOverride
	err := s.db.QueryRowContext(ctx, `SELECT id, scope_type, scope_id, stage, body, model, based_on_prompt_id, updated_by, created_at, updated_at
FROM prompt_overrides
WHERE scope_type=$1 AND scope_id=$2 AND stage=$3`, scopeType, scopeID, stage).
		Scan(&item.ID, &item.ScopeType, &item.ScopeID, &item.Stage, &item.Body, &item.Model, &item.BasedOnPrompt, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return PromptOverride{}, err
	}
	return item, nil
}

func (s *PromptOverrideStore) ResolveForDomainStage(ctx context.Context, domainID, projectID, stage string) (ResolvedPrompt, error) {
	stage = strings.TrimSpace(stage)
	if stage == "" {
		return ResolvedPrompt{}, fmt.Errorf("stage is required")
	}

	// 1) domain override
	if domainID != "" {
		if item, err := s.GetByScopeStage(ctx, PromptScopeDomain, domainID, stage); err == nil {
			return ResolvedPrompt{
				Stage:           stage,
				Source:          PromptScopeDomain,
				OverrideID:      NullableString(item.ID),
				Body:            item.Body,
				Model:           item.Model,
				BasedOnPromptID: item.BasedOnPrompt,
			}, nil
		} else if err != nil && err != sql.ErrNoRows {
			return ResolvedPrompt{}, err
		}
	}

	// 2) project override
	if projectID != "" {
		if item, err := s.GetByScopeStage(ctx, PromptScopeProject, projectID, stage); err == nil {
			return ResolvedPrompt{
				Stage:           stage,
				Source:          PromptScopeProject,
				OverrideID:      NullableString(item.ID),
				Body:            item.Body,
				Model:           item.Model,
				BasedOnPromptID: item.BasedOnPrompt,
			}, nil
		} else if err != nil && err != sql.ErrNoRows {
			return ResolvedPrompt{}, err
		}
	}

	// 3) global active prompt by stage
	var p SystemPrompt
	err := s.db.QueryRowContext(ctx, `SELECT id, name, description, body, stage, model, is_active, created_at, updated_at
FROM system_prompts
WHERE stage=$1 AND is_active=TRUE
ORDER BY updated_at DESC
LIMIT 1`, stage).Scan(&p.ID, &p.Name, &p.Description, &p.Body, &p.Stage, &p.Model, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return ResolvedPrompt{}, err
	}
	return ResolvedPrompt{
		Stage:    stage,
		Source:   "global",
		PromptID: NullableString(p.ID),
		Body:     p.Body,
		Model:    p.Model,
	}, nil
}
