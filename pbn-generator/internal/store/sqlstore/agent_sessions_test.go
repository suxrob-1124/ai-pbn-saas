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
		"message_count", "snapshot_tag", "messages_json", "chat_log_json",
		"pre_file_ids", "diagnostics_json",
	}).AddRow(
		"sess-1", "dom-1", "user@example.com", now,
		nil, "done", "Created 2 files", filesChanged,
		3, "agent_snapshot:sess-1", nil, nil,
		nil, nil,
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
		"message_count", "snapshot_tag", "messages_json", "chat_log_json",
		"pre_file_ids", "diagnostics_json",
	}).AddRow(
		"sess-1", "dom-1", "user@example.com", now,
		nil, "running", nil, nil, 1, nil, nil, nil,
		nil, nil,
	).AddRow(
		"sess-2", "dom-1", "user@example.com", now.Add(-time.Hour),
		&now, "done", "Done", nil, 2, "agent_snapshot:sess-2", nil, nil,
		nil, nil,
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

func TestAgentSessionStoreAppendEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewAgentSessionStore(db)
	ctx := context.Background()

	payload := []byte(`{"id":"e1","role":"assistant"}`)
	// First arg is the Go-generated UUID — match with AnyArg().
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO agent_session_events`)).
		WithArgs(sqlmock.AnyArg(), "sess-1", 0, "chat_entry", payload).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.AppendEvent(ctx, "sess-1", 0, "chat_entry", payload); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

func TestAgentSessionStoreAppendEvent_DuplicateSeqIgnored(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewAgentSessionStore(db)
	ctx := context.Background()

	payload := []byte(`{}`)
	// ON CONFLICT DO NOTHING → 0 rows affected, no error
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO agent_session_events`)).
		WithArgs(sqlmock.AnyArg(), "sess-1", 0, "chat_entry", payload).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := store.AppendEvent(ctx, "sess-1", 0, "chat_entry", payload); err != nil {
		t.Fatalf("AppendEvent duplicate must not error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

func TestAgentSessionStoreListEvents(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewAgentSessionStore(db)
	ctx := context.Background()
	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id", "session_id", "seq", "event_type", "payload_json", "created_at",
	}).
		AddRow("ev-1", "sess-1", 0, "chat_entry", []byte(`{"role":"assistant"}`), now).
		AddRow("ev-2", "sess-1", 1, "chat_entry", []byte(`{"role":"assistant"}`), now.Add(time.Second))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
		WithArgs("sess-1").
		WillReturnRows(rows)

	events, err := store.ListEvents(ctx, "sess-1")
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Seq != 0 || events[1].Seq != 1 {
		t.Errorf("events not in seq order: %v %v", events[0].Seq, events[1].Seq)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

func TestAgentSessionStoreNextEventSeq(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewAgentSessionStore(db)
	ctx := context.Background()

	// MAX(seq)+1 = 5 when the highest persisted seq is 4.
	rows := sqlmock.NewRows([]string{"coalesce"}).AddRow(5)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(MAX(seq)+1, 0)`)).
		WithArgs("sess-1").
		WillReturnRows(rows)

	n, err := store.NextEventSeq(ctx, "sess-1")
	if err != nil {
		t.Fatalf("NextEventSeq: %v", err)
	}
	if n != 5 {
		t.Errorf("expected NextEventSeq=5, got %d", n)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

func TestAgentSessionStoreNextEventSeq_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewAgentSessionStore(db)
	ctx := context.Background()

	// COALESCE returns 0 when no events exist.
	rows := sqlmock.NewRows([]string{"coalesce"}).AddRow(0)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(MAX(seq)+1, 0)`)).
		WithArgs("sess-1").
		WillReturnRows(rows)

	n, err := store.NextEventSeq(ctx, "sess-1")
	if err != nil {
		t.Fatalf("NextEventSeq (empty): %v", err)
	}
	if n != 0 {
		t.Errorf("expected NextEventSeq=0 for empty session, got %d", n)
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
