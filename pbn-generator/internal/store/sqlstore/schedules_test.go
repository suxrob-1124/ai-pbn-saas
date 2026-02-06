package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestScheduleStoreCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewScheduleStore(db)
	ctx := context.Background()

	cfg := json.RawMessage(`{"cron":"0 9 * * *"}`)
	lastRun := time.Date(2026, 2, 3, 9, 0, 0, 0, time.UTC)
	nextRun := time.Date(2026, 2, 4, 9, 0, 0, 0, time.UTC)
	sched := Schedule{
		ID:          "sched-1",
		ProjectID:   "proj-1",
		Name:        "Daily",
		Description: sql.NullString{String: "desc", Valid: true},
		Strategy:    "daily",
		Config:      cfg,
		IsActive:    true,
		CreatedBy:   "owner@example.com",
		LastRunAt:   sql.NullTime{Time: lastRun, Valid: true},
		NextRunAt:   sql.NullTime{Time: nextRun, Valid: true},
		Timezone:    sql.NullString{String: "Europe/Moscow", Valid: true},
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO generation_schedules(")).
		WithArgs(
			sched.ID,
			sched.ProjectID,
			sched.Name,
			sched.Description.String,
			sched.Strategy,
			sched.Config,
			sched.IsActive,
			sched.CreatedBy,
			lastRun,
			nextRun,
			sched.Timezone.String,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Create(ctx, sched); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestScheduleStoreGet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewScheduleStore(db)
	ctx := context.Background()

	created := time.Date(2026, 2, 4, 9, 0, 0, 0, time.UTC)
	updated := created.Add(2 * time.Hour)
	cfg := []byte(`{"cron":"0 9 * * *"}`)

	rows := sqlmock.NewRows([]string{
		"id", "project_id", "name", "description", "strategy", "config", "is_active", "created_by", "last_run_at", "next_run_at", "timezone", "created_at", "updated_at",
	}).AddRow(
		"sched-1",
		"proj-1",
		"Daily",
		sql.NullString{String: "desc", Valid: true},
		"daily",
		cfg,
		true,
		"owner@example.com",
		created,
		updated,
		sql.NullString{String: "UTC", Valid: true},
		created,
		updated,
	)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, name, description, strategy, config, is_active, created_by, last_run_at, next_run_at, timezone, created_at, updated_at FROM generation_schedules WHERE id=$1")).
		WithArgs("sched-1").
		WillReturnRows(rows)

	sched, err := store.Get(ctx, "sched-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if sched.ID != "sched-1" {
		t.Fatalf("unexpected id: %s", sched.ID)
	}
	if string(sched.Config) != string(cfg) {
		t.Fatalf("unexpected config: %s", string(sched.Config))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestScheduleStoreList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewScheduleStore(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{
		"id", "project_id", "name", "description", "strategy", "config", "is_active", "created_by", "last_run_at", "next_run_at", "timezone", "created_at", "updated_at",
	}).AddRow(
		"sched-1",
		"proj-1",
		"Daily",
		sql.NullString{},
		"daily",
		[]byte(`{"cron":"0 9 * * *"}`),
		true,
		"owner@example.com",
		sql.NullTime{},
		sql.NullTime{},
		sql.NullString{},
		time.Now(),
		time.Now(),
	).AddRow(
		"sched-2",
		"proj-1",
		"Weekly",
		sql.NullString{},
		"weekly",
		[]byte(`{"cron":"0 9 * * 1"}`),
		false,
		"owner@example.com",
		sql.NullTime{},
		sql.NullTime{},
		sql.NullString{},
		time.Now(),
		time.Now(),
	)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, name, description, strategy, config, is_active, created_by, last_run_at, next_run_at, timezone, created_at, updated_at FROM generation_schedules WHERE project_id=$1 ORDER BY updated_at DESC")).
		WithArgs("proj-1").
		WillReturnRows(rows)

	list, err := store.List(ctx, "proj-1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 schedules, got %d", len(list))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestScheduleStoreUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewScheduleStore(db)
	ctx := context.Background()

	t.Run("no updates", func(t *testing.T) {
		if err := store.Update(ctx, "sched-1", ScheduleUpdates{}); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		name := "New"
		active := true
		updates := ScheduleUpdates{Name: &name, IsActive: &active}

		mock.ExpectExec(`UPDATE generation_schedules SET name=\$1, is_active=\$2, updated_at=NOW\(\) WHERE id=\$3`).
			WithArgs(name, active, "sched-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := store.Update(ctx, "sched-1", updates); err != nil {
			t.Fatalf("update failed: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		strategy := "daily"
		updates := ScheduleUpdates{Strategy: &strategy}

		mock.ExpectExec(`UPDATE generation_schedules SET strategy=\$1, updated_at=NOW\(\) WHERE id=\$2`).
			WithArgs(strategy, "sched-2").
			WillReturnError(errors.New("boom"))

		if err := store.Update(ctx, "sched-2", updates); err == nil {
			t.Fatalf("expected error")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestScheduleStoreDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewScheduleStore(db)
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM generation_schedules WHERE id=$1")).
		WithArgs("sched-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Delete(ctx, "sched-1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestScheduleStoreListActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewScheduleStore(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{
		"id", "project_id", "name", "description", "strategy", "config", "is_active", "created_by", "last_run_at", "next_run_at", "timezone", "created_at", "updated_at",
	}).AddRow(
		"sched-1",
		"proj-1",
		"Daily",
		sql.NullString{},
		"daily",
		[]byte(`{"cron":"0 9 * * *"}`),
		true,
		"owner@example.com",
		sql.NullTime{},
		sql.NullTime{},
		sql.NullString{},
		time.Now(),
		time.Now(),
	)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, name, description, strategy, config, is_active, created_by, last_run_at, next_run_at, timezone, created_at, updated_at FROM generation_schedules WHERE is_active=TRUE ORDER BY updated_at DESC")).
		WillReturnRows(rows)

	list, err := store.ListActive(ctx)
	if err != nil {
		t.Fatalf("list active failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(list))
	}
	if !list[0].IsActive {
		t.Fatalf("expected active schedule")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
