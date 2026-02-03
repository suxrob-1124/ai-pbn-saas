package sqlstore

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"obzornik-pbn-generator/internal/auth"
)

func TestSessionStoreLifecycle(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewSessionStore(db)
	now := time.Now()
	sess := auth.Session{JTI: "jti", Email: "user@example.com", ExpiresAt: now.Add(time.Hour)}

	mock.ExpectExec(`INSERT INTO sessions`).
		WithArgs(sess.JTI, sess.Email, sess.ExpiresAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Create(context.Background(), sess); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	rows := sqlmock.NewRows([]string{"token", "email", "expires_at"}).AddRow(sess.JTI, sess.Email, sess.ExpiresAt)
	mock.ExpectQuery(`SELECT token, email, expires_at FROM sessions WHERE token = \$1`).
		WithArgs(sess.JTI).
		WillReturnRows(rows)

	loaded, err := store.Get(context.Background(), sess.JTI)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if loaded.Email != sess.Email || !loaded.ExpiresAt.Equal(sess.ExpiresAt) {
		t.Fatalf("loaded session mismatch: %+v", loaded)
	}

	mock.ExpectExec(`DELETE FROM sessions WHERE token = \$1`).
		WithArgs(sess.JTI).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Delete(context.Background(), sess.JTI); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
