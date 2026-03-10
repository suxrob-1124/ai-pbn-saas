package sqlstore

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAgentSessionStoreCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewAgentSessionStore(db)
	ctx := context.Background()

	sess := AgentSession{
		ID:        "sess-1",
		DomainID:  "dom-1",
		CreatedBy: "user@example.com",
		Status:    "running",
	}

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO agent_sessions`)).
		WithArgs(sess.ID, sess.DomainID, sess.CreatedBy, sess.Status).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

func TestAgentSessionStoreGet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewAgentSessionStore(db)
	ctx := context.Background()

	now := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	filesChanged, _ := json.Marshal([]string{"index.html"})

	rows := sqlmock.NewRows([]string{
		"id", "domain_id", "created_by", "created_at",
		"finished_at", "status", "summary", "files_changed",
		"message_count", "snapshot_tag",
	}).AddRow(
		"sess-1", "dom-1", "user@example.com", now,
		nil, "done", "Created 2 files", filesChanged,
		3, "agent_snapshot:sess-1",
	)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
		WithArgs("sess-1").
		WillReturnRows(rows)

	sess, err := store.Get(ctx, "sess-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sess.ID != "sess-1" {
		t.Errorf("expected ID sess-1, got %s", sess.ID)
	}
	if sess.Status != "done" {
		t.Errorf("expected status done, got %s", sess.Status)
	}
	if len(sess.FilesChanged) != 1 || sess.FilesChanged[0] != "index.html" {
		t.Errorf("unexpected files_changed: %v", sess.FilesChanged)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

func TestAgentSessionStoreListByDomain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewAgentSessionStore(db)
	ctx := context.Background()

	now := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id", "domain_id", "created_by", "created_at",
		"finished_at", "status", "summary", "files_changed",
		"message_count", "snapshot_tag",
	}).AddRow(
		"sess-1", "dom-1", "user@example.com", now,
		nil, "running", nil, nil, 1, nil,
	).AddRow(
		"sess-2", "dom-1", "user@example.com", now.Add(-time.Hour),
		&now, "done", "Done", nil, 2, "agent_snapshot:sess-2",
	)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
		WithArgs("dom-1", 20).
		WillReturnRows(rows)

	sessions, err := store.ListByDomain(ctx, "dom-1", 20)
	if err != nil {
		t.Fatalf("ListByDomain: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

func TestAgentSessionStoreFinish(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewAgentSessionStore(db)
	ctx := context.Background()

	filesChanged := []string{"index.html", "about.html"}
	filesJSON, _ := json.Marshal(filesChanged)

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agent_sessions`)).
		WithArgs("done", "Completed", filesJSON, 5, "sess-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.Finish(ctx, "sess-1", "done", "Completed", filesChanged, 5); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

func TestAgentSessionStoreSetSnapshotTag(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewAgentSessionStore(db)
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agent_sessions SET snapshot_tag`)).
		WithArgs("agent_snapshot:sess-1", "sess-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.SetSnapshotTag(ctx, "sess-1", "agent_snapshot:sess-1"); err != nil {
		t.Fatalf("SetSnapshotTag: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}
