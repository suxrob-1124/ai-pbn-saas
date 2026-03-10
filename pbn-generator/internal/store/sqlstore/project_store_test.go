package sqlstore

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestProjectStoreListByNameExact(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewProjectStore(db)
	ctx := context.Background()
	now := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id",
		"user_email",
		"name",
		"target_country",
		"target_language",
		"timezone",
		"global_blacklist",
		"default_server_id",
		"status",
		"index_check_enabled",
		"created_at",
		"updated_at",
		"deleted_at",
		"deleted_by",
	}).AddRow(
		"proj-1",
		"owner@example.com",
		"proj-name",
		"se",
		"sv",
		nil,
		nil,
		nil,
		"draft",
		true,
		now,
		now,
		nil,
		nil,
	)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_email, name, target_country, target_language, timezone, global_blacklist, default_server_id, status, index_check_enabled, created_at, updated_at, deleted_at, deleted_by FROM projects WHERE name=$1 AND deleted_at IS NULL ORDER BY updated_at DESC")).
		WithArgs("proj-name").
		WillReturnRows(rows)

	list, err := store.ListByNameExact(ctx, "proj-name")
	if err != nil {
		t.Fatalf("ListByNameExact failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 project, got %d", len(list))
	}
	if list[0].ID != "proj-1" {
		t.Fatalf("unexpected project id: %s", list[0].ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
