package sqlstore

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func TestUserStoreCreateAuthenticate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewUserStore(db)
	email := "user@example.com"
	password := "password1"

	mock.ExpectExec(`INSERT INTO users`).
		WithArgs(email, sqlmock.AnyArg(), sqlmock.AnyArg(), false, "", "", "manager", false).
		WillReturnResult(sqlmock.NewResult(0, 1))

	user, err := store.Create(context.Background(), email, password)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if user.Email != email {
		t.Fatalf("wrong email stored")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"email", "password_hash", "created_at", "verified", "name", "avatar_url", "role", "is_approved", "api_key_updated_at"}).
		AddRow(email, hash, now, true, "", "", "manager", true, nil)
	mock.ExpectQuery(`SELECT email, password_hash, created_at, verified, name, avatar_url, role, is_approved, api_key_updated_at FROM users WHERE email = \$1`).
		WithArgs(email).
		WillReturnRows(rows)

	if _, err := store.Authenticate(context.Background(), email, password); err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
