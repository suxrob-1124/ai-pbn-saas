package sqlstore

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestModelPricingStoreGetActiveByModel(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := NewModelPricingStore(db)
	now := time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id", "provider", "model", "input_usd_per_million", "output_usd_per_million",
		"active_from", "active_to", "is_active", "updated_by", "updated_at",
	}).AddRow(
		"pricing-1", "gemini", "gemini-2.5-pro", 1.25, 5.0,
		now.Add(-time.Hour), nil, true, "admin@example.com", now,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, provider, model, input_usd_per_million, output_usd_per_million, active_from, active_to, is_active, updated_by, updated_at
FROM llm_model_pricing
WHERE provider=$1 AND model=$2
  AND active_from <= $3
  AND (active_to IS NULL OR active_to > $3)
  AND is_active = TRUE
ORDER BY active_from DESC
LIMIT 1`)).
		WithArgs("gemini", "gemini-2.5-pro", now).
		WillReturnRows(rows)

	item, err := store.GetActiveByModel(context.Background(), "gemini", "gemini-2.5-pro", now)
	if err != nil {
		t.Fatalf("GetActiveByModel: %v", err)
	}
	if item == nil || item.ID != "pricing-1" {
		t.Fatalf("unexpected item: %+v", item)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestModelPricingStoreUpsertActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := NewModelPricingStore(db)
	now := time.Date(2026, 2, 19, 12, 30, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE llm_model_pricing
SET is_active=FALSE, active_to=$3, updated_at=$3, updated_by=$4
WHERE provider=$1 AND model=$2 AND is_active=TRUE`)).
		WithArgs("gemini", "gemini-2.5-pro", now, "admin@example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO llm_model_pricing(
	id, provider, model, input_usd_per_million, output_usd_per_million, active_from, active_to, is_active, updated_by, updated_at
) VALUES($1,$2,$3,$4,$5,$6,NULL,TRUE,$7,$6)`)).
		WithArgs(sqlmock.AnyArg(), "gemini", "gemini-2.5-pro", 1.25, 5.0, now, "admin@example.com").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := store.UpsertActive(context.Background(), "gemini", "gemini-2.5-pro", 1.25, 5.0, "admin@example.com", now); err != nil {
		t.Fatalf("UpsertActive: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
