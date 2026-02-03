package sqlstore

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDeleteOldGenerations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewGenerationStore(db)
	ctx := context.Background()

	t.Run("delete with default statuses", func(t *testing.T) {
		// Тест с дефолтными статусами (cancelled, error)
		mock.ExpectExec(`DELETE FROM generations WHERE created_at < NOW\(\) - INTERVAL '30 days' AND status IN \(\$1,\$2\)`).
			WithArgs("cancelled", "error").
			WillReturnResult(sqlmock.NewResult(0, 5))

		count, err := store.DeleteOldGenerations(ctx, 30, nil)
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}
		if count != 5 {
			t.Errorf("expected 5 deleted, got %d", count)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("delete with custom statuses", func(t *testing.T) {
		// Тест с кастомными статусами
		mock.ExpectExec(`DELETE FROM generations WHERE created_at < NOW\(\) - INTERVAL '60 days' AND status IN \(\$1,\$2,\$3\)`).
			WithArgs("cancelled", "error", "failed").
			WillReturnResult(sqlmock.NewResult(0, 10))

		count, err := store.DeleteOldGenerations(ctx, 60, []string{"cancelled", "error", "failed"})
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}
		if count != 10 {
			t.Errorf("expected 10 deleted, got %d", count)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("delete with empty statuses uses default", func(t *testing.T) {
		// Тест с пустым массивом статусов (должен использовать дефолтные)
		mock.ExpectExec(`DELETE FROM generations WHERE created_at < NOW\(\) - INTERVAL '15 days' AND status IN \(\$1,\$2\)`).
			WithArgs("cancelled", "error").
			WillReturnResult(sqlmock.NewResult(0, 0))

		count, err := store.DeleteOldGenerations(ctx, 15, []string{})
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 deleted, got %d", count)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("delete with single status", func(t *testing.T) {
		// Тест с одним статусом
		mock.ExpectExec(`DELETE FROM generations WHERE created_at < NOW\(\) - INTERVAL '7 days' AND status IN \(\$1\)`).
			WithArgs("cancelled").
			WillReturnResult(sqlmock.NewResult(0, 3))

		count, err := store.DeleteOldGenerations(ctx, 7, []string{"cancelled"})
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}
		if count != 3 {
			t.Errorf("expected 3 deleted, got %d", count)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}
