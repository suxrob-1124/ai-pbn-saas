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

func TestGenQueueStoreEnqueue(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewGenQueueStore(db)
	ctx := context.Background()

	scheduledFor := time.Date(2026, 2, 4, 12, 0, 0, 0, time.UTC)
	item := QueueItem{
		ID:           "queue-1",
		DomainID:     "domain-1",
		ScheduleID:   sql.NullString{String: "sched-1", Valid: true},
		Priority:     2,
		ScheduledFor: scheduledFor,
		Status:       "pending",
		ErrorMessage: sql.NullString{},
		ProcessedAt:  sql.NullTime{},
	}

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO generation_queue(")).
			WithArgs(
				item.ID,
				item.DomainID,
				item.ScheduleID.String,
				item.Priority,
				item.ScheduledFor,
				item.Status,
				nil,
				nil,
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := store.Enqueue(ctx, item); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO generation_queue(")).
			WithArgs(
				item.ID,
				item.DomainID,
				item.ScheduleID.String,
				item.Priority,
				item.ScheduledFor,
				item.Status,
				nil,
				nil,
			).
			WillReturnError(errors.New("boom"))

		if err := store.Enqueue(ctx, item); err == nil {
			t.Fatalf("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestGenQueueStoreGetPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewGenQueueStore(db)
	ctx := context.Background()

	scheduledFor := time.Date(2026, 2, 4, 12, 0, 0, 0, time.UTC)
	createdAt := scheduledFor.Add(-time.Minute)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"schedule_id",
		"priority",
		"scheduled_for",
		"status",
		"error_message",
		"created_at",
		"processed_at",
	}).AddRow(
		"queue-1",
		"domain-1",
		sql.NullString{String: "sched-1", Valid: true},
		1,
		scheduledFor,
		"pending",
		sql.NullString{},
		createdAt,
		sql.NullTime{},
	)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, schedule_id, priority, scheduled_for, status, error_message, created_at, processed_at FROM generation_queue")).
			WithArgs(5).
			WillReturnRows(rows)

		items, err := store.GetPending(ctx, 5)
		if err != nil {
			t.Fatalf("get pending failed: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
		if items[0].ID != "queue-1" {
			t.Fatalf("unexpected item id: %s", items[0].ID)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, schedule_id, priority, scheduled_for, status, error_message, created_at, processed_at FROM generation_queue")).
			WithArgs(1).
			WillReturnError(errors.New("boom"))

		if _, err := store.GetPending(ctx, 1); err == nil {
			t.Fatalf("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestGenQueueStoreMarkProcessed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewGenQueueStore(db)
	ctx := context.Background()

	errMsg := "failed"

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE generation_queue")).
			WithArgs("failed", errMsg, "queue-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := store.MarkProcessed(ctx, "queue-1", "failed", &errMsg); err != nil {
			t.Fatalf("mark processed failed: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE generation_queue")).
			WithArgs("completed", nil, "queue-2").
			WillReturnError(errors.New("boom"))

		if err := store.MarkProcessed(ctx, "queue-2", "completed", nil); err == nil {
			t.Fatalf("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestGenQueueStoreListByDomain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewGenQueueStore(db)
	ctx := context.Background()

	createdAt := time.Date(2026, 2, 4, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"schedule_id",
		"priority",
		"scheduled_for",
		"status",
		"error_message",
		"created_at",
		"processed_at",
	}).AddRow(
		"queue-1",
		"domain-1",
		sql.NullString{},
		0,
		createdAt,
		"pending",
		sql.NullString{},
		createdAt,
		sql.NullTime{},
	)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, schedule_id, priority, scheduled_for, status, error_message, created_at, processed_at FROM generation_queue")).
			WithArgs("domain-1").
			WillReturnRows(rows)

		items, err := store.ListByDomain(ctx, "domain-1")
		if err != nil {
			t.Fatalf("list by domain failed: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, schedule_id, priority, scheduled_for, status, error_message, created_at, processed_at FROM generation_queue")).
			WithArgs("domain-2").
			WillReturnError(errors.New("boom"))

		if _, err := store.ListByDomain(ctx, "domain-2"); err == nil {
			t.Fatalf("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestGenQueueStoreGet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewGenQueueStore(db)
	ctx := context.Background()

	scheduledFor := time.Date(2026, 2, 4, 12, 0, 0, 0, time.UTC)
	createdAt := scheduledFor.Add(-time.Minute)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"schedule_id",
		"priority",
		"scheduled_for",
		"status",
		"error_message",
		"created_at",
		"processed_at",
	}).AddRow(
		"queue-1",
		"domain-1",
		sql.NullString{String: "sched-1", Valid: true},
		1,
		scheduledFor,
		"pending",
		sql.NullString{},
		createdAt,
		sql.NullTime{},
	)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, schedule_id, priority, scheduled_for, status, error_message, created_at, processed_at FROM generation_queue")).
			WithArgs("queue-1").
			WillReturnRows(rows)

		item, err := store.Get(ctx, "queue-1")
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if item.ID != "queue-1" {
			t.Fatalf("unexpected id: %s", item.ID)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, schedule_id, priority, scheduled_for, status, error_message, created_at, processed_at FROM generation_queue")).
			WithArgs("queue-2").
			WillReturnError(errors.New("boom"))

		if _, err := store.Get(ctx, "queue-2"); err == nil {
			t.Fatalf("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestGenQueueStoreListByProject(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewGenQueueStore(db)
	ctx := context.Background()

	scheduledFor := time.Date(2026, 2, 4, 12, 0, 0, 0, time.UTC)
	createdAt := scheduledFor.Add(-time.Minute)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"schedule_id",
		"priority",
		"scheduled_for",
		"status",
		"error_message",
		"created_at",
		"processed_at",
	}).AddRow(
		"queue-1",
		"domain-1",
		sql.NullString{String: "sched-1", Valid: true},
		1,
		scheduledFor,
		"pending",
		sql.NullString{},
		createdAt,
		sql.NullTime{},
	)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery("FROM generation_queue").
			WithArgs("project-1").
			WillReturnRows(rows)

		items, err := store.ListByProject(ctx, "project-1")
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery("FROM generation_queue").
			WithArgs("project-2").
			WillReturnError(errors.New("boom"))

		if _, err := store.ListByProject(ctx, "project-2"); err == nil {
			t.Fatalf("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestGenQueueStoreListHistoryByProjectPage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewGenQueueStore(db)
	ctx := context.Background()

	scheduledFor := time.Date(2026, 2, 4, 12, 0, 0, 0, time.UTC)
	createdAt := scheduledFor.Add(-time.Minute)
	processedAt := sql.NullTime{Time: scheduledFor.Add(time.Minute), Valid: true}
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"schedule_id",
		"priority",
		"scheduled_for",
		"status",
		"error_message",
		"created_at",
		"processed_at",
	}).AddRow(
		"queue-1",
		"domain-1",
		sql.NullString{String: "sched-1", Valid: true},
		1,
		scheduledFor,
		"failed",
		sql.NullString{String: "boom", Valid: true},
		createdAt,
		processedAt,
	)

	status := "failed"
	dateFrom := scheduledFor.Add(-time.Hour)
	dateTo := scheduledFor.Add(time.Hour)
	expected := `(?s)SELECT q.id, q.domain_id, q.schedule_id, q.priority, q.scheduled_for, q.status, q.error_message, q.created_at, q.processed_at\s+FROM generation_queue q\s+JOIN domains d ON d.id = q.domain_id\s+WHERE d.project_id=\$1\s+AND q.status IN \('completed','failed'\)\s+AND q.status=\$2\s+AND q.scheduled_for >= \$3\s+AND q.scheduled_for <= \$4\s+AND \(LOWER\(COALESCE\(d.url, ''\)\) LIKE \$5 OR LOWER\(q.domain_id\) LIKE \$5\)\s+ORDER BY`
	mock.ExpectQuery(expected).
		WithArgs("project-1", status, dateFrom, dateTo, "%domain%").
		WillReturnRows(rows)

	items, err := store.ListHistoryByProjectPage(ctx, "project-1", 0, 0, "domain", &status, &dateFrom, &dateTo)
	if err != nil {
		t.Fatalf("list history failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Status != "failed" {
		t.Fatalf("expected failed status, got %s", items[0].Status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGenQueueStoreDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewGenQueueStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM generation_queue WHERE id=$1")).
			WithArgs("queue-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := store.Delete(ctx, "queue-1"); err != nil {
			t.Fatalf("delete failed: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM generation_queue WHERE id=$1")).
			WithArgs("queue-2").
			WillReturnError(errors.New("boom"))

		if err := store.Delete(ctx, "queue-2"); err == nil {
			t.Fatalf("expected error, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}
