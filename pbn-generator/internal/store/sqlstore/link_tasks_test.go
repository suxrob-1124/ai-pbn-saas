package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLinkTaskStoreCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewLinkTaskStore(db)
	ctx := context.Background()

	item := LinkTask{
		ID:           "task-1",
		DomainID:     "domain-1",
		AnchorText:   "anchor",
		TargetURL:    "https://example.com",
		ScheduledFor: time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC),
		Action:       "insert",
		Status:       "pending",
		Attempts:     0,
		CreatedBy:    "admin@example.com",
	}

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO link_tasks(")).
			WithArgs(
				item.ID,
				item.DomainID,
				item.AnchorText,
				item.TargetURL,
				item.ScheduledFor,
				item.Action,
				item.Status,
				nil,
				nil,
				nil,
				item.Attempts,
				item.CreatedBy,
				nil,
				sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := store.Create(ctx, item); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO link_tasks(")).
			WithArgs(
				item.ID,
				item.DomainID,
				item.AnchorText,
				item.TargetURL,
				item.ScheduledFor,
				item.Action,
				item.Status,
				nil,
				nil,
				nil,
				item.Attempts,
				item.CreatedBy,
				nil,
				sqlmock.AnyArg(),
			).
			WillReturnError(errors.New("boom"))

		if err := store.Create(ctx, item); err == nil {
			t.Fatalf("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestLinkTaskStoreGet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewLinkTaskStore(db)
	ctx := context.Background()

	scheduled := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	created := scheduled.Add(-time.Minute)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"anchor_text",
		"target_url",
		"scheduled_for",
		"action",
		"status",
		"found_location",
		"generated_content",
		"error_message",
		"attempts",
		"created_by",
		"created_at",
		"completed_at",
		"log_lines",
	}).AddRow(
		"task-1",
		"domain-1",
		"anchor",
		"https://example.com",
		scheduled,
		"insert",
		"pending",
		sql.NullString{},
		sql.NullString{},
		sql.NullString{},
		0,
		"admin@example.com",
		created,
		sql.NullTime{},
		nil,
	)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, anchor_text, target_url, scheduled_for, action, status, found_location, generated_content, error_message, attempts, created_by, created_at, completed_at, log_lines FROM link_tasks WHERE id=$1")).
			WithArgs("task-1").
			WillReturnRows(rows)

		item, err := store.Get(ctx, "task-1")
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if item.ID != "task-1" {
			t.Fatalf("unexpected id: %s", item.ID)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, anchor_text, target_url, scheduled_for, action, status, found_location, generated_content, error_message, attempts, created_by, created_at, completed_at, log_lines FROM link_tasks WHERE id=$1")).
			WithArgs("task-2").
			WillReturnError(errors.New("boom"))

		if _, err := store.Get(ctx, "task-2"); err == nil {
			t.Fatalf("expected error")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestLinkTaskStoreListByDomain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewLinkTaskStore(db)
	ctx := context.Background()

	scheduled := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	created := scheduled.Add(-time.Minute)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"anchor_text",
		"target_url",
		"scheduled_for",
		"action",
		"status",
		"found_location",
		"generated_content",
		"error_message",
		"attempts",
		"created_by",
		"created_at",
		"completed_at",
		"log_lines",
	}).AddRow(
		"task-1",
		"domain-1",
		"anchor",
		"https://example.com",
		scheduled,
		"insert",
		"pending",
		sql.NullString{},
		sql.NullString{},
		sql.NullString{},
		0,
		"admin@example.com",
		created,
		sql.NullTime{},
		nil,
	)

	status := "pending"
	filters := LinkTaskFilters{Status: &status, Limit: 10}

	mock.ExpectQuery("FROM link_tasks").
		WithArgs("domain-1", status, 10).
		WillReturnRows(rows)

	list, err := store.ListByDomain(ctx, "domain-1", filters)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestLinkTaskStoreListByProject(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewLinkTaskStore(db)
	ctx := context.Background()

	scheduled := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	created := scheduled.Add(-time.Minute)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"anchor_text",
		"target_url",
		"scheduled_for",
		"action",
		"status",
		"found_location",
		"generated_content",
		"error_message",
		"attempts",
		"created_by",
		"created_at",
		"completed_at",
		"log_lines",
	}).AddRow(
		"task-1",
		"domain-1",
		"anchor",
		"https://example.com",
		scheduled,
		"insert",
		"pending",
		sql.NullString{},
		sql.NullString{},
		sql.NullString{},
		0,
		"admin@example.com",
		created,
		sql.NullTime{},
		nil,
	)

	status := "pending"
	filters := LinkTaskFilters{Status: &status, Limit: 5}
	expected := `(?s)SELECT lt.id, lt.domain_id, lt.anchor_text, lt.target_url, lt.scheduled_for, lt.action, lt.status, lt.found_location, lt.generated_content, lt.error_message, lt.attempts, lt.created_by, lt.created_at, lt.completed_at, lt.log_lines\s+FROM link_tasks lt JOIN domains d ON d.id = lt.domain_id\s+WHERE d.project_id=\$1 AND lt.status=\$2\s+ORDER BY lt.scheduled_for ASC, lt.created_at ASC\s+LIMIT \$3`

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(expected).
			WithArgs("project-1", status, 5).
			WillReturnRows(rows)

		list, err := store.ListByProject(ctx, "project-1", filters)
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 item, got %d", len(list))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(expected).
			WithArgs("project-2", status, 5).
			WillReturnError(errors.New("boom"))

		if _, err := store.ListByProject(ctx, "project-2", filters); err == nil {
			t.Fatalf("expected error")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestLinkTaskStoreListPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewLinkTaskStore(db)
	ctx := context.Background()

	scheduled := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	created := scheduled.Add(-time.Minute)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"anchor_text",
		"target_url",
		"scheduled_for",
		"action",
		"status",
		"found_location",
		"generated_content",
		"error_message",
		"attempts",
		"created_by",
		"created_at",
		"completed_at",
		"log_lines",
	}).AddRow(
		"task-1",
		"domain-1",
		"anchor",
		"https://example.com",
		scheduled,
		"insert",
		"pending",
		sql.NullString{},
		sql.NullString{},
		sql.NullString{},
		0,
		"admin@example.com",
		created,
		sql.NullTime{},
		nil,
	)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, anchor_text, target_url, scheduled_for, action, status, found_location, generated_content, error_message, attempts, created_by, created_at, completed_at, log_lines FROM link_tasks")).
		WithArgs(5).
		WillReturnRows(rows)

	list, err := store.ListPending(ctx, 5)
	if err != nil {
		t.Fatalf("list pending failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestLinkTaskStoreUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewLinkTaskStore(db)
	ctx := context.Background()

	t.Run("no updates", func(t *testing.T) {
		if err := store.Update(ctx, "task-1", LinkTaskUpdates{}); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		status := "inserted"
		attempts := 2
		updates := LinkTaskUpdates{Status: &status, Attempts: &attempts}

		mock.ExpectExec(`UPDATE link_tasks SET status=\$1, attempts=\$2 WHERE id=\$3`).
			WithArgs(status, attempts, "task-2").
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := store.Update(ctx, "task-2", updates); err != nil {
			t.Fatalf("update failed: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		status := "failed"
		updates := LinkTaskUpdates{Status: &status}

		mock.ExpectExec(`UPDATE link_tasks SET status=\$1 WHERE id=\$2`).
			WithArgs(status, "task-3").
			WillReturnError(errors.New("boom"))

		if err := store.Update(ctx, "task-3", updates); err == nil {
			t.Fatalf("expected error")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestLinkTaskStoreDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewLinkTaskStore(db)
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM link_tasks WHERE id=$1")).
		WithArgs("task-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Delete(ctx, "task-1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
