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
	CountByDomain(ctx context.Context, domainID string, filters IndexCheckFilters) (int, error)
	CountByProject(ctx context.Context, projectID string, filters IndexCheckFilters) (int, error)
	CountAll(ctx context.Context, filters IndexCheckFilters) (int, error)
	CountFailed(ctx context.Context, filters IndexCheckFilters) (int, error)
	ListPendingRetries(ctx context.Context) ([]IndexCheck, error)
	UpdateStatus(ctx context.Context, checkID string, status string, isIndexed *bool, errMsg *string) error
	IncrementAttempts(ctx context.Context, checkID string) error
	SetNextRetry(ctx context.Context, checkID string, nextRetry time.Time) error
	ResetForManual(ctx context.Context, checkID string, nextRetry time.Time) error
	AggregateStats(ctx context.Context, filters IndexCheckFilters) (IndexCheckStats, error)
	AggregateStatsByDomain(ctx context.Context, domainID string, filters IndexCheckFilters) (IndexCheckStats, error)
	AggregateStatsByProject(ctx context.Context, projectID string, filters IndexCheckFilters) (IndexCheckStats, error)
	AggregateDaily(ctx context.Context, filters IndexCheckFilters) ([]IndexCheckDailySummary, error)
	AggregateDailyByDomain(ctx context.Context, domainID string, filters IndexCheckFilters) ([]IndexCheckDailySummary, error)
	AggregateDailyByProject(ctx context.Context, projectID string, filters IndexCheckFilters) ([]IndexCheckDailySummary, error)
}

// IndexCheckFilters описывает фильтры для выборки проверок индексации.
type IndexCheckFilters struct {
  Statuses  []string
  Search    *string
  Limit     int
  Offset    int
  SortBy    string
  SortDir   string
  IsIndexed *bool
  From      *time.Time
  To        *time.Time
  DomainID  *string
}

// IndexCheckDailySummary описывает агрегаты по дням.
type IndexCheckDailySummary struct {
	Date                time.Time
	Total               int
	IndexedTrue         int
	IndexedFalse        int
	Pending             int
	Checking            int
	FailedInvestigation int
	Success             int
}

// IndexCheckStats описывает агрегированную статистику.
type IndexCheckStats struct {
	TotalChecks          int
	TotalResolved        int
	IndexedTrue          int
	AvgAttemptsToSuccess float64
	FailedInvestigation  int
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
	filters.Statuses = []string{"failed_investigation"}
	return s.listIndexChecks(ctx, filters)
}

// CountByDomain возвращает количество проверок по домену.
func (s *IndexCheckSQLStore) CountByDomain(ctx context.Context, domainID string, filters IndexCheckFilters) (int, error) {
	return s.countIndexChecksWithBase(ctx,
		"domain_index_checks c",
		[]string{"c.domain_id=$1"},
		[]interface{}{domainID},
		filters,
		true,
		false,
	)
}

// CountByProject возвращает количество проверок по проекту.
func (s *IndexCheckSQLStore) CountByProject(ctx context.Context, projectID string, filters IndexCheckFilters) (int, error) {
	return s.countIndexChecksWithBase(ctx,
		"domain_index_checks c JOIN domains d ON d.id = c.domain_id",
		[]string{"d.project_id=$1"},
		[]interface{}{projectID},
		filters,
		true,
		true,
	)
}

// CountAll возвращает количество проверок по всем доменам.
func (s *IndexCheckSQLStore) CountAll(ctx context.Context, filters IndexCheckFilters) (int, error) {
	return s.countIndexChecksWithBase(ctx, "domain_index_checks c", nil, nil, filters, true, false)
}

// CountFailed возвращает количество проблемных проверок.
func (s *IndexCheckSQLStore) CountFailed(ctx context.Context, filters IndexCheckFilters) (int, error) {
	filters.Statuses = []string{"failed_investigation"}
	return s.countIndexChecksWithBase(ctx, "domain_index_checks c", nil, nil, filters, true, false)
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
	fromTable, clauses, args, idx := buildIndexCheckFilterClauses(
		baseFrom,
		baseClauses,
		baseArgs,
		filters,
		allowSearch,
		hasDomainJoin,
	)

	query := fmt.Sprintf(`SELECT c.id, c.domain_id, c.check_date, c.status, c.is_indexed, c.attempts, c.last_attempt_at, c.next_retry_at, c.error_message, c.completed_at, c.created_at
		FROM %s`, fromTable)
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY " + buildIndexCheckOrder(filters)
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

func (s *IndexCheckSQLStore) countIndexChecksWithBase(
	ctx context.Context,
	baseFrom string,
	baseClauses []string,
	baseArgs []interface{},
	filters IndexCheckFilters,
	allowSearch bool,
	hasDomainJoin bool,
) (int, error) {
	fromTable, clauses, args, _ := buildIndexCheckFilterClauses(
		baseFrom,
		baseClauses,
		baseArgs,
		filters,
		allowSearch,
		hasDomainJoin,
	)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", fromTable)
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// AggregateStats агрегирует статистику по всем проверкам.
func (s *IndexCheckSQLStore) AggregateStats(ctx context.Context, filters IndexCheckFilters) (IndexCheckStats, error) {
	return s.aggregateStatsWithBase(ctx, "domain_index_checks c", nil, nil, filters, true, false)
}

// AggregateStatsByDomain агрегирует статистику по домену.
func (s *IndexCheckSQLStore) AggregateStatsByDomain(ctx context.Context, domainID string, filters IndexCheckFilters) (IndexCheckStats, error) {
	return s.aggregateStatsWithBase(
		ctx,
		"domain_index_checks c",
		[]string{"c.domain_id=$1"},
		[]interface{}{domainID},
		filters,
		true,
		false,
	)
}

// AggregateStatsByProject агрегирует статистику по проекту.
func (s *IndexCheckSQLStore) AggregateStatsByProject(ctx context.Context, projectID string, filters IndexCheckFilters) (IndexCheckStats, error) {
	return s.aggregateStatsWithBase(
		ctx,
		"domain_index_checks c JOIN domains d ON d.id = c.domain_id",
		[]string{"d.project_id=$1"},
		[]interface{}{projectID},
		filters,
		true,
		true,
	)
}

// AggregateDaily агрегирует проверки по дням (global).
func (s *IndexCheckSQLStore) AggregateDaily(ctx context.Context, filters IndexCheckFilters) ([]IndexCheckDailySummary, error) {
	return s.aggregateDailyWithBase(ctx, "domain_index_checks c", nil, nil, filters, true, false)
}

// AggregateDailyByDomain агрегирует проверки по дням для домена.
func (s *IndexCheckSQLStore) AggregateDailyByDomain(ctx context.Context, domainID string, filters IndexCheckFilters) ([]IndexCheckDailySummary, error) {
	return s.aggregateDailyWithBase(
		ctx,
		"domain_index_checks c",
		[]string{"c.domain_id=$1"},
		[]interface{}{domainID},
		filters,
		true,
		false,
	)
}

// AggregateDailyByProject агрегирует проверки по дням для проекта.
func (s *IndexCheckSQLStore) AggregateDailyByProject(ctx context.Context, projectID string, filters IndexCheckFilters) ([]IndexCheckDailySummary, error) {
	return s.aggregateDailyWithBase(
		ctx,
		"domain_index_checks c JOIN domains d ON d.id = c.domain_id",
		[]string{"d.project_id=$1"},
		[]interface{}{projectID},
		filters,
		true,
		true,
	)
}

func (s *IndexCheckSQLStore) aggregateStatsWithBase(
	ctx context.Context,
	baseFrom string,
	baseClauses []string,
	baseArgs []interface{},
	filters IndexCheckFilters,
	allowSearch bool,
	hasDomainJoin bool,
) (IndexCheckStats, error) {
	fromTable, clauses, args, _ := buildIndexCheckFilterClauses(
		baseFrom,
		baseClauses,
		baseArgs,
		filters,
		allowSearch,
		hasDomainJoin,
	)
	query := fmt.Sprintf(`SELECT
		COUNT(*) AS total_checks,
		COUNT(*) FILTER (WHERE c.status='success' AND c.is_indexed IS NOT NULL) AS total_resolved,
		COUNT(*) FILTER (WHERE c.status='success' AND c.is_indexed = true) AS indexed_true,
		AVG(c.attempts) FILTER (WHERE c.status='success') AS avg_attempts,
		COUNT(*) FILTER (WHERE c.status='failed_investigation') AS failed_count
		FROM %s`, fromTable)
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	var stats IndexCheckStats
	var avg sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.TotalChecks,
		&stats.TotalResolved,
		&stats.IndexedTrue,
		&avg,
		&stats.FailedInvestigation,
	); err != nil {
		return IndexCheckStats{}, err
	}
	if avg.Valid {
		stats.AvgAttemptsToSuccess = avg.Float64
	}
	return stats, nil
}

func (s *IndexCheckSQLStore) aggregateDailyWithBase(
	ctx context.Context,
	baseFrom string,
	baseClauses []string,
	baseArgs []interface{},
	filters IndexCheckFilters,
	allowSearch bool,
	hasDomainJoin bool,
) ([]IndexCheckDailySummary, error) {
	fromTable, clauses, args, _ := buildIndexCheckFilterClauses(
		baseFrom,
		baseClauses,
		baseArgs,
		filters,
		allowSearch,
		hasDomainJoin,
	)
	query := fmt.Sprintf(`SELECT
		c.check_date,
		COUNT(*) AS total,
		COUNT(*) FILTER (WHERE c.status='success' AND c.is_indexed = true) AS indexed_true,
		COUNT(*) FILTER (WHERE c.status='success' AND c.is_indexed = false) AS indexed_false,
		COUNT(*) FILTER (WHERE c.status='pending') AS pending,
		COUNT(*) FILTER (WHERE c.status='checking') AS checking,
		COUNT(*) FILTER (WHERE c.status='failed_investigation') AS failed,
		COUNT(*) FILTER (WHERE c.status='success') AS success
		FROM %s`, fromTable)
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " GROUP BY c.check_date ORDER BY c.check_date ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []IndexCheckDailySummary
	for rows.Next() {
		var item IndexCheckDailySummary
		if err := rows.Scan(
			&item.Date,
			&item.Total,
			&item.IndexedTrue,
			&item.IndexedFalse,
			&item.Pending,
			&item.Checking,
			&item.FailedInvestigation,
			&item.Success,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
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

func buildIndexCheckOrder(filters IndexCheckFilters) string {
	key := strings.TrimSpace(filters.SortBy)
	dir := "DESC"
	if strings.EqualFold(strings.TrimSpace(filters.SortDir), "asc") {
		dir = "ASC"
	}
	switch key {
	case "domain":
		return fmt.Sprintf("COALESCE(d.url, c.domain_id) %s, c.check_date DESC, c.created_at DESC", dir)
	case "status":
		return fmt.Sprintf("c.status %s, c.check_date DESC, c.created_at DESC", dir)
	case "attempts":
    return fmt.Sprintf("c.attempts %s, c.check_date DESC, c.created_at DESC", dir)
  case "is_indexed":
    return fmt.Sprintf("c.is_indexed %s, c.check_date DESC, c.created_at DESC", dir)
  case "last_attempt_at":
    return fmt.Sprintf("c.last_attempt_at %s, c.check_date DESC, c.created_at DESC", dir)
  case "next_retry_at":
    return fmt.Sprintf("c.next_retry_at %s, c.check_date DESC, c.created_at DESC", dir)
  case "created_at":
    return fmt.Sprintf("c.created_at %s", dir)
  case "check_date":
    fallthrough
  default:
    return fmt.Sprintf("c.check_date %s, c.created_at %s", dir, dir)
  }
}

func requiresDomainJoinForSort(sortBy string) bool {
  return strings.TrimSpace(sortBy) == "domain"
}

func normalizeIndexCheckStatuses(list []string) []string {
  if len(list) == 0 {
    return nil
	}
	seen := make(map[string]struct{}, len(list))
	out := make([]string, 0, len(list))
	for _, raw := range list {
		status := strings.TrimSpace(raw)
		if status == "" {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		out = append(out, status)
	}
	return out
}

func buildIndexCheckFilterClauses(
	baseFrom string,
	baseClauses []string,
	baseArgs []interface{},
	filters IndexCheckFilters,
	allowSearch bool,
	hasDomainJoin bool,
) (string, []string, []interface{}, int) {
	fromTable := baseFrom
	clauses := append([]string(nil), baseClauses...)
	args := append([]interface{}(nil), baseArgs...)
	idx := len(args) + 1

	statuses := normalizeIndexCheckStatuses(filters.Statuses)
	if len(statuses) > 0 {
		placeholders := make([]string, 0, len(statuses))
		for _, status := range statuses {
			placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
			args = append(args, status)
			idx++
		}
		clauses = append(clauses, fmt.Sprintf("c.status IN (%s)", strings.Join(placeholders, ",")))
	}
	if requiresDomainJoinForSort(filters.SortBy) && !hasDomainJoin {
		fromTable = "domain_index_checks c JOIN domains d ON d.id = c.domain_id"
		hasDomainJoin = true
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
	if filters.DomainID != nil {
		domainID := strings.TrimSpace(*filters.DomainID)
		if domainID != "" {
			clauses = append(clauses, fmt.Sprintf("c.domain_id=$%d", idx))
			args = append(args, domainID)
			idx++
		}
	}
	if filters.IsIndexed != nil {
		clauses = append(clauses, fmt.Sprintf("c.is_indexed=$%d", idx))
		args = append(args, *filters.IsIndexed)
		idx++
	}
	if filters.From != nil {
		clauses = append(clauses, fmt.Sprintf("c.check_date >= $%d", idx))
		args = append(args, *filters.From)
		idx++
	}
	if filters.To != nil {
		clauses = append(clauses, fmt.Sprintf("c.check_date <= $%d", idx))
		args = append(args, *filters.To)
		idx++
	}
	return fromTable, clauses, args, idx
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
