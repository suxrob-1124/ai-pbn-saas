package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// LinkTask описывает задачу линкбилдинга.
type LinkTask struct {
	ID               string
	DomainID         string
	AnchorText       string
	TargetURL        string
	ScheduledFor     time.Time
	Action           string
	Status           string
	FoundLocation    sql.NullString
	GeneratedContent sql.NullString
	ErrorMessage     sql.NullString
	Attempts         int
	CreatedBy        string
	CreatedAt        time.Time
	CompletedAt      sql.NullTime
	LogLines         []string
}

// LinkTaskFilters определяет фильтры для выборки задач.
type LinkTaskFilters struct {
	Status          *string
	ScheduledAfter  *time.Time
	ScheduledBefore *time.Time
	Limit           int
	Offset          int
	Search          *string
}

// LinkTaskUpdates описывает изменения задачи.
type LinkTaskUpdates struct {
	AnchorText       *string
	TargetURL        *string
	Action           *string
	Status           *string
	FoundLocation    *sql.NullString
	GeneratedContent *sql.NullString
	ErrorMessage     *sql.NullString
	Attempts         *int
	ScheduledFor     *time.Time
	CompletedAt      *sql.NullTime
	LogLines         *[]string
}

// LinkTaskStore определяет операции над задачами линкбилдинга.
type LinkTaskStore interface {
	Create(ctx context.Context, task LinkTask) error
	Get(ctx context.Context, taskID string) (*LinkTask, error)
	ListByDomain(ctx context.Context, domainID string, filters LinkTaskFilters) ([]LinkTask, error)
	ListByProject(ctx context.Context, projectID string, filters LinkTaskFilters) ([]LinkTask, error)
	ListAll(ctx context.Context, filters LinkTaskFilters) ([]LinkTask, error)
	ListPending(ctx context.Context, limit int) ([]LinkTask, error)
	Update(ctx context.Context, taskID string, updates LinkTaskUpdates) error
	Delete(ctx context.Context, taskID string) error
}

// LinkTaskSQLStore реализует LinkTaskStore поверх SQL БД.
type LinkTaskSQLStore struct {
	db *sql.DB
}

// NewLinkTaskStore создает новый LinkTaskSQLStore.
func NewLinkTaskStore(db *sql.DB) *LinkTaskSQLStore {
	return &LinkTaskSQLStore{db: db}
}

// Create создает задачу линкбилдинга.
func (s *LinkTaskSQLStore) Create(ctx context.Context, task LinkTask) error {
	logLines, err := marshalLogLines(task.LogLines)
	if err != nil {
		return fmt.Errorf("failed to encode link task logs: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO link_tasks(
			id, domain_id, anchor_text, target_url, scheduled_for, action, status, found_location, generated_content, error_message, attempts, created_by, created_at, completed_at, log_lines
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW(),$13,$14)`,
		task.ID,
		task.DomainID,
		task.AnchorText,
		task.TargetURL,
		task.ScheduledFor,
		normalizeLinkTaskAction(task.Action),
		task.Status,
		nullableString(task.FoundLocation),
		nullableString(task.GeneratedContent),
		nullableString(task.ErrorMessage),
		task.Attempts,
		task.CreatedBy,
		nullableTime(task.CompletedAt),
		logLines,
	)
	if err != nil {
		return fmt.Errorf("failed to create link task: %w", err)
	}
	return nil
}

// Get возвращает задачу по ID.
func (s *LinkTaskSQLStore) Get(ctx context.Context, taskID string) (*LinkTask, error) {
	var task LinkTask
	var logLinesRaw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, anchor_text, target_url, scheduled_for, action, status, found_location, generated_content, error_message, attempts, created_by, created_at, completed_at, log_lines
		FROM link_tasks WHERE id=$1`, taskID).
		Scan(
			&task.ID,
			&task.DomainID,
			&task.AnchorText,
			&task.TargetURL,
			&task.ScheduledFor,
			&task.Action,
			&task.Status,
			&task.FoundLocation,
			&task.GeneratedContent,
			&task.ErrorMessage,
			&task.Attempts,
			&task.CreatedBy,
			&task.CreatedAt,
			&task.CompletedAt,
			&logLinesRaw,
		); err != nil {
		return nil, err
	}
	if err := decodeLogLines(logLinesRaw, &task.LogLines); err != nil {
		return nil, err
	}
	return &task, nil
}

// ListByDomain возвращает список задач по домену с фильтрами.
func (s *LinkTaskSQLStore) ListByDomain(ctx context.Context, domainID string, filters LinkTaskFilters) ([]LinkTask, error) {
	query, args := buildLinkTaskQuery("link_tasks", "domain_id=$1", []interface{}{domainID}, filters)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLinkTasks(rows)
}

// ListByProject возвращает задачи для проекта.
func (s *LinkTaskSQLStore) ListByProject(ctx context.Context, projectID string, filters LinkTaskFilters) ([]LinkTask, error) {
	base := `link_tasks lt JOIN domains d ON d.id = lt.domain_id`
	query, args := buildLinkTaskQuery(base, "d.project_id=$1", []interface{}{projectID}, filters)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLinkTasks(rows)
}

// ListAll возвращает задачи с фильтрами без ограничений по домену.
func (s *LinkTaskSQLStore) ListAll(ctx context.Context, filters LinkTaskFilters) ([]LinkTask, error) {
	query, args := buildLinkTaskQuery("link_tasks", "", nil, filters)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLinkTasks(rows)
}

// ListPending возвращает ожидающие задачи.
func (s *LinkTaskSQLStore) ListPending(ctx context.Context, limit int) ([]LinkTask, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, anchor_text, target_url, scheduled_for, action, status, found_location, generated_content, error_message, attempts, created_by, created_at, completed_at, log_lines
		FROM link_tasks
		WHERE status='pending' AND scheduled_for <= NOW()
		ORDER BY scheduled_for ASC, created_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []LinkTask
	for rows.Next() {
		var task LinkTask
		var logLinesRaw []byte
		if err := rows.Scan(
			&task.ID,
			&task.DomainID,
			&task.AnchorText,
			&task.TargetURL,
			&task.ScheduledFor,
			&task.Action,
			&task.Status,
			&task.FoundLocation,
			&task.GeneratedContent,
			&task.ErrorMessage,
			&task.Attempts,
			&task.CreatedBy,
			&task.CreatedAt,
			&task.CompletedAt,
			&logLinesRaw,
		); err != nil {
			return nil, err
		}
		if err := decodeLogLines(logLinesRaw, &task.LogLines); err != nil {
			return nil, err
		}
		task.Action = normalizeLinkTaskAction(task.Action)
		res = append(res, task)
	}
	return res, rows.Err()
}

// Update обновляет задачу линкбилдинга.
func (s *LinkTaskSQLStore) Update(ctx context.Context, taskID string, updates LinkTaskUpdates) error {
	setClauses := make([]string, 0, 6)
	args := make([]interface{}, 0, 6)
	idx := 1

	if updates.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status=$%d", idx))
		args = append(args, *updates.Status)
		idx++
	}
	if updates.AnchorText != nil {
		setClauses = append(setClauses, fmt.Sprintf("anchor_text=$%d", idx))
		args = append(args, *updates.AnchorText)
		idx++
	}
	if updates.TargetURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("target_url=$%d", idx))
		args = append(args, *updates.TargetURL)
		idx++
	}
	if updates.Action != nil {
		setClauses = append(setClauses, fmt.Sprintf("action=$%d", idx))
		args = append(args, normalizeLinkTaskAction(*updates.Action))
		idx++
	}
	if updates.FoundLocation != nil {
		setClauses = append(setClauses, fmt.Sprintf("found_location=$%d", idx))
		args = append(args, nullableString(*updates.FoundLocation))
		idx++
	}
	if updates.GeneratedContent != nil {
		setClauses = append(setClauses, fmt.Sprintf("generated_content=$%d", idx))
		args = append(args, nullableString(*updates.GeneratedContent))
		idx++
	}
	if updates.ErrorMessage != nil {
		setClauses = append(setClauses, fmt.Sprintf("error_message=$%d", idx))
		args = append(args, nullableString(*updates.ErrorMessage))
		idx++
	}
	if updates.Attempts != nil {
		setClauses = append(setClauses, fmt.Sprintf("attempts=$%d", idx))
		args = append(args, *updates.Attempts)
		idx++
	}
	if updates.ScheduledFor != nil {
		setClauses = append(setClauses, fmt.Sprintf("scheduled_for=$%d", idx))
		args = append(args, *updates.ScheduledFor)
		idx++
	}
	if updates.CompletedAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("completed_at=$%d", idx))
		args = append(args, nullableTime(*updates.CompletedAt))
		idx++
	}
	if updates.LogLines != nil {
		encoded, err := marshalLogLines(*updates.LogLines)
		if err != nil {
			return fmt.Errorf("failed to encode link task logs: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("log_lines=$%d", idx))
		args = append(args, encoded)
		idx++
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no link task updates provided")
	}

	query := fmt.Sprintf("UPDATE link_tasks SET %s WHERE id=$%d", strings.Join(setClauses, ", "), idx)
	args = append(args, taskID)

	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to update link task: %w", err)
	}
	return nil
}

// Delete удаляет задачу.
func (s *LinkTaskSQLStore) Delete(ctx context.Context, taskID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM link_tasks WHERE id=$1`, taskID)
	return err
}

func buildLinkTaskQuery(table string, baseClause string, baseArgs []interface{}, filters LinkTaskFilters) (string, []interface{}) {
	clauses := []string{}
	args := []interface{}{}
	idx := 1
	colPrefix := ""
	selectPrefix := ""
	fromTable := table
	if strings.Contains(fromTable, "link_tasks lt") {
		colPrefix = "lt."
		selectPrefix = "lt."
	}

	if baseClause != "" {
		clauses = append(clauses, baseClause)
		args = append(args, baseArgs...)
		idx += len(baseArgs)
	}
	if filters.Search != nil {
		if term := strings.TrimSpace(*filters.Search); term != "" {
			if !strings.Contains(fromTable, "JOIN domains") {
				fromTable = "link_tasks lt JOIN domains d ON d.id = lt.domain_id"
				colPrefix = "lt."
				selectPrefix = "lt."
			}
			clauses = append(clauses, fmt.Sprintf("(LOWER(COALESCE(d.url, '')) LIKE $%d OR LOWER(%sdomain_id) LIKE $%d)", idx, colPrefix, idx))
			args = append(args, "%"+strings.ToLower(term)+"%")
			idx++
		}
	}
	if filters.Status != nil {
		clauses = append(clauses, fmt.Sprintf("%sstatus=$%d", colPrefix, idx))
		args = append(args, *filters.Status)
		idx++
	}
	if filters.ScheduledAfter != nil {
		clauses = append(clauses, fmt.Sprintf("%sscheduled_for >= $%d", colPrefix, idx))
		args = append(args, *filters.ScheduledAfter)
		idx++
	}
	if filters.ScheduledBefore != nil {
		clauses = append(clauses, fmt.Sprintf("%sscheduled_for <= $%d", colPrefix, idx))
		args = append(args, *filters.ScheduledBefore)
		idx++
	}

	query := fmt.Sprintf(`SELECT %sid, %sdomain_id, %sanchor_text, %starget_url, %sscheduled_for, %saction, %sstatus, %sfound_location, %sgenerated_content, %serror_message, %sattempts, %screated_by, %screated_at, %scompleted_at, %slog_lines
		FROM %s`,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		selectPrefix,
		fromTable,
	)
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY %sscheduled_for ASC, %screated_at ASC", selectPrefix, selectPrefix)
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", idx)
		args = append(args, filters.Limit)
		idx++
	}
	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", idx)
		args = append(args, filters.Offset)
	}

	return query, args
}

func scanLinkTasks(rows *sql.Rows) ([]LinkTask, error) {
	var res []LinkTask
	for rows.Next() {
		var task LinkTask
		var logLinesRaw []byte
		if err := rows.Scan(
			&task.ID,
			&task.DomainID,
			&task.AnchorText,
			&task.TargetURL,
			&task.ScheduledFor,
			&task.Action,
			&task.Status,
			&task.FoundLocation,
			&task.GeneratedContent,
			&task.ErrorMessage,
			&task.Attempts,
			&task.CreatedBy,
			&task.CreatedAt,
			&task.CompletedAt,
			&logLinesRaw,
		); err != nil {
			return nil, err
		}
		if err := decodeLogLines(logLinesRaw, &task.LogLines); err != nil {
			return nil, err
		}
		task.Action = normalizeLinkTaskAction(task.Action)
		res = append(res, task)
	}
	return res, rows.Err()
}

func normalizeLinkTaskAction(action string) string {
	action = strings.TrimSpace(strings.ToLower(action))
	if action == "" {
		return "insert"
	}
	return action
}

func marshalLogLines(lines []string) ([]byte, error) {
	if len(lines) == 0 {
		return nil, nil
	}
	return json.Marshal(lines)
}

func decodeLogLines(raw []byte, target *[]string) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}
