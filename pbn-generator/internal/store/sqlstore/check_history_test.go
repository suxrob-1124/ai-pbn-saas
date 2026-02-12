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

func TestCheckHistoryStoreCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewCheckHistoryStore(db)
	ctx := context.Background()

	history := CheckHistory{
		ID:            "hist-1",
		CheckID:       "check-1",
		AttemptNumber: 1,
		Result:        sql.NullString{String: "success", Valid: true},
		ResponseData:  []byte(`{"ok":true}`),
		ErrorMessage:  sql.NullString{},
		DurationMS:    sql.NullInt64{Int64: 1200, Valid: true},
	}

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO index_check_history(")).
			WithArgs(
				history.ID,
				history.CheckID,
				history.AttemptNumber,
				history.Result.String,
				history.ResponseData,
				nil,
				history.DurationMS.Int64,
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := store.Create(ctx, history); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO index_check_history(")).
			WithArgs(
				history.ID,
				history.CheckID,
				history.AttemptNumber,
				history.Result.String,
				history.ResponseData,
				nil,
				history.DurationMS.Int64,
			).
			WillReturnError(errors.New("boom"))

		if err := store.Create(ctx, history); err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestCheckHistoryStoreListByCheck(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewCheckHistoryStore(db)
	ctx := context.Background()

	createdAt := time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id",
		"check_id",
		"attempt_number",
		"result",
		"response_data",
		"error_message",
		"duration_ms",
		"created_at",
	}).AddRow(
		"hist-1",
		"check-1",
		1,
		sql.NullString{String: "success", Valid: true},
		[]byte(`{"ok":true}`),
		sql.NullString{},
		sql.NullInt64{Int64: 800, Valid: true},
		createdAt,
	)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, check_id, attempt_number, result, response_data, error_message, duration_ms, created_at FROM index_check_history WHERE check_id=$1 ORDER BY attempt_number ASC, created_at ASC LIMIT $2")).
			WithArgs("check-1", 10).
			WillReturnRows(rows)

		list, err := store.ListByCheck(ctx, "check-1", 10)
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 result, got %d", len(list))
		}
		if list[0].ID != "hist-1" {
			t.Fatalf("unexpected id: %s", list[0].ID)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, check_id, attempt_number, result, response_data, error_message, duration_ms, created_at FROM index_check_history WHERE check_id=$1 ORDER BY attempt_number ASC, created_at ASC LIMIT $2")).
			WithArgs("check-2", 5).
			WillReturnError(errors.New("boom"))

		if _, err := store.ListByCheck(ctx, "check-2", 5); err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}
