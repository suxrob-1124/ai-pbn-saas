package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// IndexCheck описывает проверку индексации домена.
type IndexCheck struct {
	ID            string
	DomainID      string
	CheckDate     time.Time
	Status        string
	IsIndexed     sql.NullBool
	Attempts      int
	LastAttemptAt sql.NullTime
	NextRetryAt   sql.NullTime
	ErrorMessage  sql.NullString
	CompletedAt   sql.NullTime
	CreatedAt     time.Time
}

// IndexCheckStore определяет операции над проверками индексации.
type IndexCheckStore interface {
	Create(ctx context.Context, check IndexCheck) error
	Get(ctx context.Context, checkID string) (*IndexCheck, error)
	GetByDomainAndDate(ctx context.Context, domainID string, date time.Time) (*IndexCheck, error)
	ListByDomain(ctx context.Context, domainID string, filters IndexCheckFilters) ([]IndexCheck, error)
	ListByProject(ctx context.Context, projectID string, filters IndexCheckFilters) ([]IndexCheck, error)
	ListAll(ctx context.Context, filters IndexCheckFilters) ([]IndexCheck, error)
	ListFailed(ctx context.Context, filters IndexCheckFilters) ([]IndexCheck, error)
	ListPendingRetries(ctx context.Context) ([]IndexCheck, error)
	UpdateStatus(ctx context.Context, checkID string, status string, isIndexed *bool, errMsg *string) error
	IncrementAttempts(ctx context.Context, checkID string) error
	SetNextRetry(ctx context.Context, checkID string, nextRetry time.Time) error
	ResetForManual(ctx context.Context, checkID string, nextRetry time.Time) error
}

// IndexCheckFilters описывает фильтры для выборки проверок индексации.
type IndexCheckFilters struct {
	Status *string
	Search *string
	Limit  int
	Offset int
}

// IndexCheckSQLStore реализует IndexCheckStore поверх SQL БД.
type IndexCheckSQLStore struct {
	db *sql.DB
}

// NewIndexCheckStore создает новый IndexCheckSQLStore.
func NewIndexCheckStore(db *sql.DB) *IndexCheckSQLStore {
	return &IndexCheckSQLStore{db: db}
}

// Create создает запись проверки индексации.
func (s *IndexCheckSQLStore) Create(ctx context.Context, check IndexCheck) error {
	status := check.Status
	if status == "" {
		status = "pending"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO domain_index_checks(
			id, domain_id, check_date, status, is_indexed, attempts, last_attempt_at, next_retry_at, error_message, completed_at, created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())`,
		check.ID,
		check.DomainID,
		check.CheckDate,
		status,
		nullableBool(check.IsIndexed),
		check.Attempts,
		nullableTime(check.LastAttemptAt),
		nullableTime(check.NextRetryAt),
		nullableString(check.ErrorMessage),
		nullableTime(check.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create index check: %w", err)
	}
	return nil
}

// Get возвращает проверку по ID.
func (s *IndexCheckSQLStore) Get(ctx context.Context, checkID string) (*IndexCheck, error) {
	var check IndexCheck
	if err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, check_date, status, is_indexed, attempts, last_attempt_at, next_retry_at, error_message, completed_at, created_at
		FROM domain_index_checks WHERE id=$1`, checkID).
		Scan(
			&check.ID,
			&check.DomainID,
			&check.CheckDate,
			&check.Status,
			&check.IsIndexed,
			&check.Attempts,
			&check.LastAttemptAt,
			&check.NextRetryAt,
			&check.ErrorMessage,
			&check.CompletedAt,
			&check.CreatedAt,
		); err != nil {
		return nil, err
	}
	return &check, nil
}

// GetByDomainAndDate возвращает проверку по домену и дате.
func (s *IndexCheckSQLStore) GetByDomainAndDate(ctx context.Context, domainID string, date time.Time) (*IndexCheck, error) {
	var check IndexCheck
	if err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, check_date, status, is_indexed, attempts, last_attempt_at, next_retry_at, error_message, completed_at, created_at
		FROM domain_index_checks WHERE domain_id=$1 AND check_date=$2`, domainID, date).
		Scan(
			&check.ID,
			&check.DomainID,
			&check.CheckDate,
			&check.Status,
			&check.IsIndexed,
			&check.Attempts,
			&check.LastAttemptAt,
			&check.NextRetryAt,
			&check.ErrorMessage,
			&check.CompletedAt,
			&check.CreatedAt,
		); err != nil {
		return nil, err
	}
	return &check, nil
}

// ListByDomain возвращает проверки по домену (последние по дате).
func (s *IndexCheckSQLStore) ListByDomain(ctx context.Context, domainID string, filters IndexCheckFilters) ([]IndexCheck, error) {
	return s.listIndexChecksWithBase(ctx,
		"domain_index_checks c",
		[]string{"c.domain_id=$1"},
		[]interface{}{domainID},
		filters,
		true,
		false,
	)
}

// ListByProject возвращает проверки по проекту.
func (s *IndexCheckSQLStore) ListByProject(ctx context.Context, projectID string, filters IndexCheckFilters) ([]IndexCheck, error) {
	return s.listIndexChecksWithBase(ctx,
		"domain_index_checks c JOIN domains d ON d.id = c.domain_id",
		[]string{"d.project_id=$1"},
		[]interface{}{projectID},
		filters,
		true,
		true,
	)
}

// ListAll возвращает проверки без ограничения по проекту.
func (s *IndexCheckSQLStore) ListAll(ctx context.Context, filters IndexCheckFilters) ([]IndexCheck, error) {
	return s.listIndexChecks(ctx, filters)
}

// ListFailed возвращает проблемные проверки.
func (s *IndexCheckSQLStore) ListFailed(ctx context.Context, filters IndexCheckFilters) ([]IndexCheck, error) {
	status := "failed_investigation"
	filters.Status = &status
	return s.listIndexChecks(ctx, filters)
}

// ListPendingRetries возвращает проверки, готовые к повторной попытке.
func (s *IndexCheckSQLStore) ListPendingRetries(ctx context.Context) ([]IndexCheck, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, check_date, status, is_indexed, attempts, last_attempt_at, next_retry_at, error_message, completed_at, created_at
		FROM domain_index_checks
		WHERE status='checking' AND next_retry_at IS NOT NULL AND next_retry_at <= NOW()
		ORDER BY next_retry_at ASC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIndexChecks(rows)
}

// UpdateStatus обновляет статус проверки и связанные поля.
func (s *IndexCheckSQLStore) UpdateStatus(ctx context.Context, checkID string, status string, isIndexed *bool, errMsg *string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domain_index_checks
		SET status=$1,
			is_indexed=$2,
			error_message=$3,
			completed_at=CASE WHEN $1 IN ('success','failed_investigation') THEN NOW() ELSE NULL END
		WHERE id=$4`,
		status,
		nullableBool(nullBool(isIndexed)),
		nullableString(nullString(errMsg)),
		checkID,
	)
	if err != nil {
		return fmt.Errorf("failed to update index check status: %w", err)
	}
	return nil
}

// IncrementAttempts увеличивает счетчик попыток и обновляет время последней попытки.
func (s *IndexCheckSQLStore) IncrementAttempts(ctx context.Context, checkID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domain_index_checks
		SET attempts=attempts+1, last_attempt_at=NOW()
		WHERE id=$1`, checkID)
	if err != nil {
		return fmt.Errorf("failed to increment attempts: %w", err)
	}
	return nil
}

// SetNextRetry обновляет время следующей попытки.
func (s *IndexCheckSQLStore) SetNextRetry(ctx context.Context, checkID string, nextRetry time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domain_index_checks
		SET next_retry_at=$1
		WHERE id=$2`, nextRetry, checkID)
	if err != nil {
		return fmt.Errorf("failed to set next retry: %w", err)
	}
	return nil
}

// ResetForManual сбрасывает проверку для ручного запуска.
func (s *IndexCheckSQLStore) ResetForManual(ctx context.Context, checkID string, nextRetry time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domain_index_checks
		SET status='pending',
			is_indexed=NULL,
			attempts=0,
			last_attempt_at=NULL,
			next_retry_at=$1,
			error_message=NULL,
			completed_at=NULL
		WHERE id=$2`, nextRetry, checkID)
	if err != nil {
		return fmt.Errorf("failed to reset index check: %w", err)
	}
	return nil
}

func (s *IndexCheckSQLStore) listIndexChecks(ctx context.Context, filters IndexCheckFilters) ([]IndexCheck, error) {
	return s.listIndexChecksWithBase(ctx,
		"domain_index_checks c",
		nil,
		nil,
		filters,
		true,
		false,
	)
}

func (s *IndexCheckSQLStore) listIndexChecksWithBase(
	ctx context.Context,
	baseFrom string,
	baseClauses []string,
	baseArgs []interface{},
	filters IndexCheckFilters,
	allowSearch bool,
	hasDomainJoin bool,
) ([]IndexCheck, error) {
	fromTable := baseFrom
	clauses := append([]string(nil), baseClauses...)
	args := append([]interface{}(nil), baseArgs...)
	idx := len(args) + 1

	if filters.Status != nil {
		status := strings.TrimSpace(*filters.Status)
		if status != "" {
			clauses = append(clauses, fmt.Sprintf("c.status=$%d", idx))
			args = append(args, status)
			idx++
		}
	}
	if allowSearch && filters.Search != nil {
		term := strings.TrimSpace(*filters.Search)
		if term != "" {
			if !hasDomainJoin {
				fromTable = "domain_index_checks c JOIN domains d ON d.id = c.domain_id"
				hasDomainJoin = true
			}
			clauses = append(clauses, fmt.Sprintf("(LOWER(COALESCE(d.url, '')) LIKE $%d OR LOWER(c.domain_id) LIKE $%d)", idx, idx))
			args = append(args, "%"+strings.ToLower(term)+"%")
			idx++
		}
	}

	query := fmt.Sprintf(`SELECT c.id, c.domain_id, c.check_date, c.status, c.is_indexed, c.attempts, c.last_attempt_at, c.next_retry_at, c.error_message, c.completed_at, c.created_at
		FROM %s`, fromTable)
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY c.check_date DESC, c.created_at DESC"
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", idx)
		args = append(args, filters.Limit)
		idx++
	}
	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", idx)
		args = append(args, filters.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIndexChecks(rows)
}

func scanIndexChecks(rows *sql.Rows) ([]IndexCheck, error) {
	var res []IndexCheck
	for rows.Next() {
		var check IndexCheck
		if err := rows.Scan(
			&check.ID,
			&check.DomainID,
			&check.CheckDate,
			&check.Status,
			&check.IsIndexed,
			&check.Attempts,
			&check.LastAttemptAt,
			&check.NextRetryAt,
			&check.ErrorMessage,
			&check.CompletedAt,
			&check.CreatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, check)
	}
	return res, rows.Err()
}

func nullBool(ptr *bool) sql.NullBool {
	if ptr == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *ptr, Valid: true}
}

func nullableBool(nb sql.NullBool) interface{} {
	if nb.Valid {
		return nb.Bool
	}
	return nil
}
