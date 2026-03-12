package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SkipDetail describes why a specific domain was skipped during a schedule run.
type SkipDetail struct {
	DomainID  string `json:"domain_id"`
	DomainURL string `json:"domain_url"`
	Reason    string `json:"reason"`
}

// ScheduleRunLog records a single execution of a schedule.
type ScheduleRunLog struct {
	ID            string
	ScheduleID    string
	ScheduleType  string // "generation" or "link"
	ProjectID     string
	RunAt         time.Time
	TotalDomains  int
	EligibleCount int
	EnqueuedCount int
	SkippedCount  int
	SkipDetails   []SkipDetail
	ErrorMessage  sql.NullString
	NextRunAt     sql.NullTime
	CreatedAt     time.Time
}

// ScheduleRunLogStore defines operations on schedule run audit logs.
type ScheduleRunLogStore interface {
	Create(ctx context.Context, log ScheduleRunLog) error
	ListByProject(ctx context.Context, projectID string, limit, offset int) ([]ScheduleRunLog, error)
	ListBySchedule(ctx context.Context, scheduleID string, limit, offset int) ([]ScheduleRunLog, error)
	PurgeOld(ctx context.Context, retentionDays int) (int64, error)
}

// ScheduleRunLogSQLStore implements ScheduleRunLogStore on top of SQL DB.
type ScheduleRunLogSQLStore struct {
	db *sql.DB
}

// NewScheduleRunLogStore creates a new ScheduleRunLogSQLStore.
func NewScheduleRunLogStore(db *sql.DB) *ScheduleRunLogSQLStore {
	return &ScheduleRunLogSQLStore{db: db}
}

// Create inserts a schedule run log record.
func (s *ScheduleRunLogSQLStore) Create(ctx context.Context, log ScheduleRunLog) error {
	var skipJSON []byte
	if len(log.SkipDetails) > 0 {
		var err error
		skipJSON, err = json.Marshal(log.SkipDetails)
		if err != nil {
			return fmt.Errorf("marshal skip_details: %w", err)
		}
	}

	_, err := s.db.ExecContext(ctx, `INSERT INTO schedule_run_logs(
			id, schedule_id, schedule_type, project_id, run_at,
			total_domains, eligible_count, enqueued_count, skipped_count,
			skip_details, error_message, next_run_at, created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW())`,
		log.ID,
		log.ScheduleID,
		log.ScheduleType,
		log.ProjectID,
		log.RunAt,
		log.TotalDomains,
		log.EligibleCount,
		log.EnqueuedCount,
		log.SkippedCount,
		skipJSON,
		nullableString(log.ErrorMessage),
		nullableTime(log.NextRunAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create schedule_run_log: %w", err)
	}
	return nil
}

const scheduleRunLogCols = `id, schedule_id, schedule_type, project_id, run_at,
	total_domains, eligible_count, enqueued_count, skipped_count,
	skip_details, error_message, next_run_at, created_at`

func scanScheduleRunLog(row interface{ Scan(...interface{}) error }) (*ScheduleRunLog, error) {
	var log ScheduleRunLog
	var skipJSON []byte
	if err := row.Scan(
		&log.ID, &log.ScheduleID, &log.ScheduleType, &log.ProjectID, &log.RunAt,
		&log.TotalDomains, &log.EligibleCount, &log.EnqueuedCount, &log.SkippedCount,
		&skipJSON, &log.ErrorMessage, &log.NextRunAt, &log.CreatedAt,
	); err != nil {
		return nil, err
	}
	if len(skipJSON) > 0 {
		if err := json.Unmarshal(skipJSON, &log.SkipDetails); err != nil {
			return nil, fmt.Errorf("unmarshal skip_details: %w", err)
		}
	}
	return &log, nil
}

// ListByProject returns schedule run logs for a project ordered by newest first.
func (s *ScheduleRunLogSQLStore) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]ScheduleRunLog, error) {
	query := fmt.Sprintf(`SELECT %s FROM schedule_run_logs WHERE project_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, scheduleRunLogCols)
	rows, err := s.db.QueryContext(ctx, query, projectID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []ScheduleRunLog
	for rows.Next() {
		log, err := scanScheduleRunLog(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *log)
	}
	return res, rows.Err()
}

// ListBySchedule returns run logs for a specific schedule.
func (s *ScheduleRunLogSQLStore) ListBySchedule(ctx context.Context, scheduleID string, limit, offset int) ([]ScheduleRunLog, error) {
	query := fmt.Sprintf(`SELECT %s FROM schedule_run_logs WHERE schedule_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, scheduleRunLogCols)
	rows, err := s.db.QueryContext(ctx, query, scheduleID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []ScheduleRunLog
	for rows.Next() {
		log, err := scanScheduleRunLog(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *log)
	}
	return res, rows.Err()
}

// PurgeOld deletes schedule run logs older than retentionDays.
func (s *ScheduleRunLogSQLStore) PurgeOld(ctx context.Context, retentionDays int) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM schedule_run_logs WHERE created_at < NOW() - make_interval(days => $1)`,
		retentionDays)
	if err != nil {
		return 0, fmt.Errorf("purge schedule_run_logs: %w", err)
	}
	return res.RowsAffected()
}
