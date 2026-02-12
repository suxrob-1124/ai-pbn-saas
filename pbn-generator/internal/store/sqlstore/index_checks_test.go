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

func TestIndexCheckStoreCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewIndexCheckStore(db)
	ctx := context.Background()

	checkDate := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)
	check := IndexCheck{
		ID:        "check-1",
		DomainID:  "domain-1",
		CheckDate: checkDate,
		Status:    "pending",
		Attempts:  0,
	}

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_index_checks(")).
			WithArgs(
				check.ID,
				check.DomainID,
				check.CheckDate,
				check.Status,
				nil,
				check.Attempts,
				nil,
				nil,
				nil,
				nil,
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := store.Create(ctx, check); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_index_checks(")).
			WithArgs(
				check.ID,
				check.DomainID,
				check.CheckDate,
				check.Status,
				nil,
				check.Attempts,
				nil,
				nil,
				nil,
				nil,
			).
			WillReturnError(errors.New("boom"))

		if err := store.Create(ctx, check); err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestIndexCheckStoreGet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewIndexCheckStore(db)
	ctx := context.Background()

	checkDate := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)
	createdAt := checkDate.Add(2 * time.Hour)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"check_date",
		"status",
		"is_indexed",
		"attempts",
		"last_attempt_at",
		"next_retry_at",
		"error_message",
		"completed_at",
		"created_at",
	}).AddRow(
		"check-1",
		"domain-1",
		checkDate,
		"success",
		sql.NullBool{Bool: true, Valid: true},
		1,
		sql.NullTime{},
		sql.NullTime{},
		sql.NullString{},
		sql.NullTime{},
		createdAt,
	)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, check_date, status, is_indexed, attempts, last_attempt_at, next_retry_at, error_message, completed_at, created_at FROM domain_index_checks WHERE id=$1")).
			WithArgs("check-1").
			WillReturnRows(rows)

		got, err := store.Get(ctx, "check-1")
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if got.ID != "check-1" || got.DomainID != "domain-1" {
			t.Fatalf("unexpected result: %#v", got)
		}
		if !got.IsIndexed.Valid || !got.IsIndexed.Bool {
			t.Fatalf("expected IsIndexed=true, got %#v", got.IsIndexed)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, check_date, status, is_indexed, attempts, last_attempt_at, next_retry_at, error_message, completed_at, created_at FROM domain_index_checks WHERE id=$1")).
			WithArgs("check-2").
			WillReturnError(errors.New("boom"))

		if _, err := store.Get(ctx, "check-2"); err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestIndexCheckStoreGetByDomainAndDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewIndexCheckStore(db)
	ctx := context.Background()

	checkDate := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)
	createdAt := checkDate.Add(time.Hour)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"check_date",
		"status",
		"is_indexed",
		"attempts",
		"last_attempt_at",
		"next_retry_at",
		"error_message",
		"completed_at",
		"created_at",
	}).AddRow(
		"check-1",
		"domain-1",
		checkDate,
		"pending",
		sql.NullBool{},
		0,
		sql.NullTime{},
		sql.NullTime{},
		sql.NullString{},
		sql.NullTime{},
		createdAt,
	)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, check_date, status, is_indexed, attempts, last_attempt_at, next_retry_at, error_message, completed_at, created_at FROM domain_index_checks WHERE domain_id=$1 AND check_date=$2")).
			WithArgs("domain-1", checkDate).
			WillReturnRows(rows)

		got, err := store.GetByDomainAndDate(ctx, "domain-1", checkDate)
		if err != nil {
			t.Fatalf("get by domain/date failed: %v", err)
		}
		if got.ID != "check-1" {
			t.Fatalf("unexpected id: %s", got.ID)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, check_date, status, is_indexed, attempts, last_attempt_at, next_retry_at, error_message, completed_at, created_at FROM domain_index_checks WHERE domain_id=$1 AND check_date=$2")).
			WithArgs("domain-2", checkDate).
			WillReturnError(errors.New("boom"))

		if _, err := store.GetByDomainAndDate(ctx, "domain-2", checkDate); err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestIndexCheckStoreListByDomain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewIndexCheckStore(db)
	ctx := context.Background()

	checkDate := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)
	createdAt := checkDate.Add(time.Hour)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"check_date",
		"status",
		"is_indexed",
		"attempts",
		"last_attempt_at",
		"next_retry_at",
		"error_message",
		"completed_at",
		"created_at",
	}).AddRow(
		"check-1",
		"domain-1",
		checkDate,
		"pending",
		sql.NullBool{},
		0,
		sql.NullTime{},
		sql.NullTime{},
		sql.NullString{},
		sql.NullTime{},
		createdAt,
	)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT c.id, c.domain_id, c.check_date, c.status, c.is_indexed, c.attempts, c.last_attempt_at, c.next_retry_at, c.error_message, c.completed_at, c.created_at FROM domain_index_checks c WHERE c.domain_id=$1 ORDER BY c.check_date DESC, c.created_at DESC LIMIT $2")).
			WithArgs("domain-1", 10).
			WillReturnRows(rows)

		got, err := store.ListByDomain(ctx, "domain-1", IndexCheckFilters{Limit: 10})
		if err != nil {
			t.Fatalf("list by domain failed: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 result, got %d", len(got))
		}
		if got[0].ID != "check-1" {
			t.Fatalf("unexpected id: %s", got[0].ID)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("sorted by domain", func(t *testing.T) {
		rowsSorted := sqlmock.NewRows([]string{
			"id",
			"domain_id",
			"check_date",
			"status",
			"is_indexed",
			"attempts",
			"last_attempt_at",
			"next_retry_at",
			"error_message",
			"completed_at",
			"created_at",
		}).AddRow(
			"check-1",
			"domain-1",
			checkDate,
			"pending",
			sql.NullBool{},
			0,
			sql.NullTime{},
			sql.NullTime{},
			sql.NullString{},
			sql.NullTime{},
			createdAt,
		)

		mock.ExpectQuery(regexp.QuoteMeta("SELECT c.id, c.domain_id, c.check_date, c.status, c.is_indexed, c.attempts, c.last_attempt_at, c.next_retry_at, c.error_message, c.completed_at, c.created_at FROM domain_index_checks c JOIN domains d ON d.id = c.domain_id WHERE c.domain_id=$1 ORDER BY COALESCE(d.url, c.domain_id) ASC, c.check_date DESC, c.created_at DESC LIMIT $2")).
			WithArgs("domain-1", 10).
			WillReturnRows(rowsSorted)

		got, err := store.ListByDomain(ctx, "domain-1", IndexCheckFilters{
			Limit:   10,
			SortBy:  "domain",
			SortDir: "asc",
		})
		if err != nil {
			t.Fatalf("list by domain failed: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 result, got %d", len(got))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT c.id, c.domain_id, c.check_date, c.status, c.is_indexed, c.attempts, c.last_attempt_at, c.next_retry_at, c.error_message, c.completed_at, c.created_at FROM domain_index_checks c WHERE c.domain_id=$1 ORDER BY c.check_date DESC, c.created_at DESC LIMIT $2")).
			WithArgs("domain-2", 5).
			WillReturnError(errors.New("boom"))

		if _, err := store.ListByDomain(ctx, "domain-2", IndexCheckFilters{Limit: 5}); err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestIndexCheckStoreListAllFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewIndexCheckStore(db)
	ctx := context.Background()

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"check_date",
		"status",
		"is_indexed",
		"attempts",
		"last_attempt_at",
		"next_retry_at",
		"error_message",
		"completed_at",
		"created_at",
	}).AddRow(
		"check-1",
		"domain-1",
		from,
		"success",
		sql.NullBool{Bool: true, Valid: true},
		1,
		sql.NullTime{},
		sql.NullTime{},
		sql.NullString{},
		sql.NullTime{},
		from.Add(time.Hour),
	)

	search := "Example.COM"
	isIndexed := true
	filters := IndexCheckFilters{
		Statuses:  []string{"success", "checking"},
		Search:    &search,
		IsIndexed: &isIndexed,
		From:      &from,
		To:        &to,
		Limit:     10,
	}

	mock.ExpectQuery(regexp.QuoteMeta("SELECT c.id, c.domain_id, c.check_date, c.status, c.is_indexed, c.attempts, c.last_attempt_at, c.next_retry_at, c.error_message, c.completed_at, c.created_at FROM domain_index_checks c JOIN domains d ON d.id = c.domain_id WHERE c.status IN ($1,$2) AND (LOWER(COALESCE(d.url, '')) LIKE $3 OR LOWER(c.domain_id) LIKE $3) AND c.is_indexed=$4 AND c.check_date >= $5 AND c.check_date <= $6 ORDER BY c.check_date DESC, c.created_at DESC LIMIT $7")).
		WithArgs("success", "checking", "%example.com%", true, from, to, 10).
		WillReturnRows(rows)

	got, err := store.ListAll(ctx, filters)
	if err != nil {
		t.Fatalf("list all failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].ID != "check-1" {
		t.Fatalf("unexpected id: %s", got[0].ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestIndexCheckStoreListPendingRetries(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewIndexCheckStore(db)
	ctx := context.Background()

	checkDate := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)
	nextRetry := checkDate.Add(30 * time.Minute)
	rows := sqlmock.NewRows([]string{
		"id",
		"domain_id",
		"check_date",
		"status",
		"is_indexed",
		"attempts",
		"last_attempt_at",
		"next_retry_at",
		"error_message",
		"completed_at",
		"created_at",
	}).AddRow(
		"check-1",
		"domain-1",
		checkDate,
		"checking",
		sql.NullBool{},
		1,
		sql.NullTime{},
		sql.NullTime{Time: nextRetry, Valid: true},
		sql.NullString{},
		sql.NullTime{},
		checkDate,
	)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, check_date, status, is_indexed, attempts, last_attempt_at, next_retry_at, error_message, completed_at, created_at FROM domain_index_checks WHERE status='checking' AND next_retry_at IS NOT NULL AND next_retry_at <= NOW() ORDER BY next_retry_at ASC, created_at ASC")).
			WillReturnRows(rows)

		got, err := store.ListPendingRetries(ctx)
		if err != nil {
			t.Fatalf("list pending retries failed: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 result, got %d", len(got))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, domain_id, check_date, status, is_indexed, attempts, last_attempt_at, next_retry_at, error_message, completed_at, created_at FROM domain_index_checks WHERE status='checking' AND next_retry_at IS NOT NULL AND next_retry_at <= NOW() ORDER BY next_retry_at ASC, created_at ASC")).
			WillReturnError(errors.New("boom"))

		if _, err := store.ListPendingRetries(ctx); err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestIndexCheckStoreUpdateStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewIndexCheckStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		indexed := true
		errMsg := "ok"
		mock.ExpectExec(regexp.QuoteMeta("UPDATE domain_index_checks")).
			WithArgs("success", indexed, errMsg, "check-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := store.UpdateStatus(ctx, "check-1", "success", &indexed, &errMsg); err != nil {
			t.Fatalf("update status failed: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE domain_index_checks")).
			WithArgs("failed_investigation", nil, nil, "check-2").
			WillReturnError(errors.New("boom"))

		if err := store.UpdateStatus(ctx, "check-2", "failed_investigation", nil, nil); err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestIndexCheckStoreIncrementAttempts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewIndexCheckStore(db)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE domain_index_checks")).
			WithArgs("check-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := store.IncrementAttempts(ctx, "check-1"); err != nil {
			t.Fatalf("increment attempts failed: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE domain_index_checks")).
			WithArgs("check-2").
			WillReturnError(errors.New("boom"))

		if err := store.IncrementAttempts(ctx, "check-2"); err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestIndexCheckStoreSetNextRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewIndexCheckStore(db)
	ctx := context.Background()

	nextRetry := time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC)

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE domain_index_checks")).
			WithArgs(nextRetry, "check-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := store.SetNextRetry(ctx, "check-1", nextRetry); err != nil {
			t.Fatalf("set next retry failed: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE domain_index_checks")).
			WithArgs(nextRetry, "check-2").
			WillReturnError(errors.New("boom"))

		if err := store.SetNextRetry(ctx, "check-2", nextRetry); err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}
