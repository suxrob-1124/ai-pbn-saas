package sqlstore

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestBuildLLMUsageWhere(t *testing.T) {
	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 19, 23, 59, 59, 0, time.UTC)
	user := "owner@example.com"
	project := "project-1"
	domain := "domain-1"
	model := "gemini-2.5-pro"
	operation := "editor_ai_suggest"
	status := "success"

	where, args := buildLLMUsageWhere(LLMUsageFilters{
		From:      &from,
		To:        &to,
		UserEmail: &user,
		ProjectID: &project,
		DomainID:  &domain,
		Model:     &model,
		Operation: &operation,
		Status:    &status,
	})

	if !strings.Contains(where, "created_at >=") || !strings.Contains(where, "created_at <=") {
		t.Fatalf("expected time bounds in where, got: %s", where)
	}
	if !strings.Contains(where, "requester_email =") || !strings.Contains(where, "project_id =") {
		t.Fatalf("expected requester/project filters in where, got: %s", where)
	}
	if len(args) != 8 {
		t.Fatalf("expected 8 args, got %d", len(args))
	}
}

func TestLLMUsageStoreListEvents(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := NewLLMUsageStore(db)
	now := time.Date(2026, 2, 19, 14, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id", "provider", "operation", "stage", "model", "status", "requester_email", "key_owner_email", "key_type",
		"project_id", "domain_id", "generation_id", "link_task_id", "file_path",
		"prompt_tokens", "completion_tokens", "total_tokens", "token_source",
		"input_price_usd_per_million", "output_price_usd_per_million", "estimated_cost_usd", "error_message", "created_at",
	}).AddRow(
		"evt-1", "gemini", "editor_ai_suggest", "editor_file_edit", "gemini-2.5-pro", "success", "owner@example.com", nil, nil,
		"project-1", "domain-1", nil, nil, "index.html",
		10, 20, 30, "estimated",
		1.25, 5.0, 0.000113, nil, now,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT
	id, provider, operation, stage, model, status, requester_email, key_owner_email, key_type, project_id, domain_id, generation_id, link_task_id, file_path,
	prompt_tokens, completion_tokens, total_tokens, token_source, input_price_usd_per_million, output_price_usd_per_million, estimated_cost_usd, error_message, created_at
FROM llm_usage_events
ORDER BY created_at DESC
LIMIT $1 OFFSET $2`)).
		WithArgs(50, 0).
		WillReturnRows(rows)

	items, err := store.ListEvents(context.Background(), LLMUsageFilters{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "evt-1" || items[0].TotalTokens.Int64 != 30 {
		t.Fatalf("unexpected item: %+v", items[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
