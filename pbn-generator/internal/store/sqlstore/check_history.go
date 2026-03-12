package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CheckHistory описывает попытку проверки индексации.
type CheckHistory struct {
	ID            string
	CheckID       string
	AttemptNumber int
	Result        sql.NullString
	ResponseData  []byte
	ErrorMessage  sql.NullString
	DurationMS    sql.NullInt64
	CreatedAt     time.Time
}

// CheckHistoryStore определяет операции над историями проверок.
type CheckHistoryStore interface {
	Create(ctx context.Context, history CheckHistory) error
	ListByCheck(ctx context.Context, checkID string, limit int) ([]CheckHistory, error)
	// PrunePerCheck удаляет старые попытки, оставляя последние keepLast для каждого check_id.
	PrunePerCheck(ctx context.Context, keepLast int) (int64, error)
}

// CheckHistorySQLStore реализует CheckHistoryStore поверх SQL БД.
type CheckHistorySQLStore struct {
	db *sql.DB
}

// NewCheckHistoryStore создает новый CheckHistorySQLStore.
func NewCheckHistoryStore(db *sql.DB) *CheckHistorySQLStore {
	return &CheckHistorySQLStore{db: db}
}

// Create логирует попытку проверки индексации.
func (s *CheckHistorySQLStore) Create(ctx context.Context, history CheckHistory) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO index_check_history(
			id, check_id, attempt_number, result, response_data, error_message, duration_ms, created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,NOW())`,
		history.ID,
		history.CheckID,
		history.AttemptNumber,
		nullableString(history.Result),
		nullableBytes(history.ResponseData),
		nullableString(history.ErrorMessage),
		nullableInt64(history.DurationMS),
	)
	if err != nil {
		return fmt.Errorf("failed to create check history: %w", err)
	}
	return nil
}

// ListByCheck возвращает историю проверок по check_id.
func (s *CheckHistorySQLStore) ListByCheck(ctx context.Context, checkID string, limit int) ([]CheckHistory, error) {
	query := `SELECT id, check_id, attempt_number, result, response_data, error_message, duration_ms, created_at
		FROM index_check_history
		WHERE check_id=$1
		ORDER BY attempt_number ASC, created_at ASC`
	args := []interface{}{checkID}
	if limit > 0 {
		query += " LIMIT $2"
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanCheckHistory(rows)
}

func scanCheckHistory(rows *sql.Rows) ([]CheckHistory, error) {
	var res []CheckHistory
	for rows.Next() {
		var history CheckHistory
		if err := rows.Scan(
			&history.ID,
			&history.CheckID,
			&history.AttemptNumber,
			&history.Result,
			&history.ResponseData,
			&history.ErrorMessage,
			&history.DurationMS,
			&history.CreatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, history)
	}
	return res, rows.Err()
}

// PrunePerCheck удаляет старые попытки, оставляя последние keepLast для каждого check_id.
func (s *CheckHistorySQLStore) PrunePerCheck(ctx context.Context, keepLast int) (int64, error) {
	if keepLast <= 0 {
		keepLast = 5
	}
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM index_check_history
		WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY check_id ORDER BY created_at DESC) AS rn
				FROM index_check_history
			) ranked WHERE rn > $1
		)`, keepLast)
	if err != nil {
		return 0, fmt.Errorf("prune index_check_history: %w", err)
	}
	return res.RowsAffected()
}

func nullableInt64(n sql.NullInt64) interface{} {
	if n.Valid {
		return n.Int64
	}
	return nil
}
