package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// QueueItem описывает элемент очереди генераций.
type QueueItem struct {
	ID           string
	DomainID     string
	ScheduleID   sql.NullString
	Priority     int
	ScheduledFor time.Time
	Status       string
	ErrorMessage sql.NullString
	CreatedAt    time.Time
	ProcessedAt  sql.NullTime
}

// GenQueueStore определяет операции над очередью генераций.
type GenQueueStore interface {
	Enqueue(ctx context.Context, item QueueItem) error
	GetPending(ctx context.Context, limit int) ([]QueueItem, error)
	MarkProcessed(ctx context.Context, itemID, status string, errMsg *string) error
	ListByDomain(ctx context.Context, domainID string) ([]QueueItem, error)
	Get(ctx context.Context, itemID string) (*QueueItem, error)
	ListByProject(ctx context.Context, projectID string) ([]QueueItem, error)
	ListByProjectPage(ctx context.Context, projectID string, limit, offset int, search string) ([]QueueItem, error)
	ListHistoryByProjectPage(ctx context.Context, projectID string, limit, offset int, search string, status *string, dateFrom *time.Time, dateTo *time.Time) ([]QueueItem, error)
	Delete(ctx context.Context, itemID string) error
}

// GenQueueSQLStore реализует GenQueueStore поверх SQL БД.
type GenQueueSQLStore struct {
	db *sql.DB
}

// NewGenQueueStore создает новый GenQueueSQLStore.
func NewGenQueueStore(db *sql.DB) *GenQueueSQLStore {
	return &GenQueueSQLStore{db: db}
}

// Enqueue добавляет элемент в очередь генераций.
func (s *GenQueueSQLStore) Enqueue(ctx context.Context, item QueueItem) error {
	status := item.Status
	if status == "" {
		status = "pending"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO generation_queue(
			id, domain_id, schedule_id, priority, scheduled_for, status, error_message, created_at, processed_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,NOW(),$8)`,
		item.ID,
		item.DomainID,
		nullableString(item.ScheduleID),
		item.Priority,
		item.ScheduledFor,
		status,
		nullableString(item.ErrorMessage),
		nullableTime(item.ProcessedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue generation: %w", err)
	}
	return nil
}

// GetPending возвращает готовые к обработке элементы очереди.
func (s *GenQueueSQLStore) GetPending(ctx context.Context, limit int) ([]QueueItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, schedule_id, priority, scheduled_for, status, error_message, created_at, processed_at
		FROM generation_queue
		WHERE status='pending' AND scheduled_for <= NOW()
		ORDER BY scheduled_for ASC, priority DESC, created_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []QueueItem
	for rows.Next() {
		var item QueueItem
		if err := rows.Scan(&item.ID, &item.DomainID, &item.ScheduleID, &item.Priority, &item.ScheduledFor, &item.Status, &item.ErrorMessage, &item.CreatedAt, &item.ProcessedAt); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

// MarkProcessed обновляет статус элемента очереди и сохраняет ошибку.
func (s *GenQueueSQLStore) MarkProcessed(ctx context.Context, itemID, status string, errMsg *string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE generation_queue
		SET status=$1, error_message=$2, processed_at=NOW()
		WHERE id=$3`, status, nullableString(nullString(errMsg)), itemID)
	if err != nil {
		return fmt.Errorf("failed to mark queue item processed: %w", err)
	}
	return nil
}

// ListByDomain возвращает элементы очереди по домену.
func (s *GenQueueSQLStore) ListByDomain(ctx context.Context, domainID string) ([]QueueItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, schedule_id, priority, scheduled_for, status, error_message, created_at, processed_at
		FROM generation_queue
		WHERE domain_id=$1
		ORDER BY created_at DESC`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []QueueItem
	for rows.Next() {
		var item QueueItem
		if err := rows.Scan(&item.ID, &item.DomainID, &item.ScheduleID, &item.Priority, &item.ScheduledFor, &item.Status, &item.ErrorMessage, &item.CreatedAt, &item.ProcessedAt); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

// Get возвращает элемент очереди по ID.
func (s *GenQueueSQLStore) Get(ctx context.Context, itemID string) (*QueueItem, error) {
	var item QueueItem
	err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, schedule_id, priority, scheduled_for, status, error_message, created_at, processed_at
		FROM generation_queue WHERE id=$1`, itemID).
		Scan(&item.ID, &item.DomainID, &item.ScheduleID, &item.Priority, &item.ScheduledFor, &item.Status, &item.ErrorMessage, &item.CreatedAt, &item.ProcessedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ListByProject возвращает элементы очереди по проекту.
func (s *GenQueueSQLStore) ListByProject(ctx context.Context, projectID string) ([]QueueItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT q.id, q.domain_id, q.schedule_id, q.priority, q.scheduled_for, q.status, q.error_message, q.created_at, q.processed_at
		FROM generation_queue q
		JOIN domains d ON d.id = q.domain_id
		WHERE d.project_id=$1 AND d.deleted_at IS NULL
		ORDER BY q.created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []QueueItem
	for rows.Next() {
		var item QueueItem
		if err := rows.Scan(&item.ID, &item.DomainID, &item.ScheduleID, &item.Priority, &item.ScheduledFor, &item.Status, &item.ErrorMessage, &item.CreatedAt, &item.ProcessedAt); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

// ListByProjectPage возвращает элементы очереди по проекту с пагинацией и фильтром активных доменов.
func (s *GenQueueSQLStore) ListByProjectPage(ctx context.Context, projectID string, limit, offset int, search string) ([]QueueItem, error) {
	query := `SELECT q.id, q.domain_id, q.schedule_id, q.priority, q.scheduled_for, q.status, q.error_message, q.created_at, q.processed_at
		FROM generation_queue q
		JOIN domains d ON d.id = q.domain_id
		WHERE d.project_id=$1 AND d.deleted_at IS NULL
			AND q.status IN ('pending','queued')`
	args := []interface{}{projectID}
	if term := strings.TrimSpace(search); term != "" {
		query += fmt.Sprintf(" AND (LOWER(COALESCE(d.url, '')) LIKE $%d OR LOWER(q.domain_id) LIKE $%d)", len(args)+1, len(args)+1)
		args = append(args, "%"+strings.ToLower(term)+"%")
	}
	query += " ORDER BY q.created_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", len(args)+1)
		args = append(args, limit)
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", len(args)+1)
		args = append(args, offset)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []QueueItem
	for rows.Next() {
		var item QueueItem
		if err := rows.Scan(&item.ID, &item.DomainID, &item.ScheduleID, &item.Priority, &item.ScheduledFor, &item.Status, &item.ErrorMessage, &item.CreatedAt, &item.ProcessedAt); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

// ListHistoryByProjectPage возвращает завершенные/ошибочные элементы очереди по проекту.
func (s *GenQueueSQLStore) ListHistoryByProjectPage(
	ctx context.Context,
	projectID string,
	limit, offset int,
	search string,
	status *string,
	dateFrom *time.Time,
	dateTo *time.Time,
) ([]QueueItem, error) {
	query := `SELECT q.id, q.domain_id, q.schedule_id, q.priority, q.scheduled_for, q.status, q.error_message, q.created_at, q.processed_at
		FROM generation_queue q
		JOIN domains d ON d.id = q.domain_id
		WHERE d.project_id=$1 AND d.deleted_at IS NULL
			AND q.status IN ('completed','failed')`
	args := []interface{}{projectID}
	if status != nil {
		query += fmt.Sprintf(" AND q.status=$%d", len(args)+1)
		args = append(args, strings.ToLower(strings.TrimSpace(*status)))
	}
	if dateFrom != nil {
		query += fmt.Sprintf(" AND q.scheduled_for >= $%d", len(args)+1)
		args = append(args, *dateFrom)
	}
	if dateTo != nil {
		query += fmt.Sprintf(" AND q.scheduled_for <= $%d", len(args)+1)
		args = append(args, *dateTo)
	}
	if term := strings.TrimSpace(search); term != "" {
		query += fmt.Sprintf(" AND (LOWER(COALESCE(d.url, '')) LIKE $%d OR LOWER(q.domain_id) LIKE $%d)", len(args)+1, len(args)+1)
		args = append(args, "%"+strings.ToLower(term)+"%")
	}
	query += " ORDER BY COALESCE(q.processed_at, q.created_at) DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", len(args)+1)
		args = append(args, limit)
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", len(args)+1)
		args = append(args, offset)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []QueueItem
	for rows.Next() {
		var item QueueItem
		if err := rows.Scan(&item.ID, &item.DomainID, &item.ScheduleID, &item.Priority, &item.ScheduledFor, &item.Status, &item.ErrorMessage, &item.CreatedAt, &item.ProcessedAt); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

// Delete удаляет элемент очереди по ID.
func (s *GenQueueSQLStore) Delete(ctx context.Context, itemID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM generation_queue WHERE id=$1`, itemID)
	return err
}
