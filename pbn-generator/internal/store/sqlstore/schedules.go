package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Schedule описывает расписание генерации.
type Schedule struct {
	ID          string
	ProjectID   string
	Name        string
	Description sql.NullString
	Strategy    string
	Config      json.RawMessage
	IsActive    bool
	CreatedBy   string
	LastRunAt   sql.NullTime
	NextRunAt   sql.NullTime
	Timezone    sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ScheduleUpdates описывает частичные изменения расписания.
type ScheduleUpdates struct {
	Name        *string
	Description *sql.NullString
	Strategy    *string
	Config      *json.RawMessage
	IsActive    *bool
	LastRunAt   *sql.NullTime
	NextRunAt   *sql.NullTime
	Timezone    *sql.NullString
}

// ScheduleStore определяет операции над расписаниями генерации.
type ScheduleStore interface {
	Create(ctx context.Context, schedule Schedule) error
	Get(ctx context.Context, scheduleID string) (*Schedule, error)
	List(ctx context.Context, projectID string) ([]Schedule, error)
	Update(ctx context.Context, scheduleID string, updates ScheduleUpdates) error
	Delete(ctx context.Context, scheduleID string) error
	ListActive(ctx context.Context) ([]Schedule, error)
}

// ScheduleSQLStore реализует ScheduleStore поверх SQL БД.
type ScheduleSQLStore struct {
	db *sql.DB
}

// NewScheduleStore создает новый ScheduleSQLStore.
func NewScheduleStore(db *sql.DB) *ScheduleSQLStore {
	return &ScheduleSQLStore{db: db}
}

// Create создает запись расписания генерации.
func (s *ScheduleSQLStore) Create(ctx context.Context, schedule Schedule) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO generation_schedules(
			id, project_id, name, description, strategy, config, is_active, created_by, last_run_at, next_run_at, timezone, created_at, updated_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),NOW())`,
		schedule.ID,
		schedule.ProjectID,
		schedule.Name,
		nullableString(schedule.Description),
		schedule.Strategy,
		schedule.Config,
		schedule.IsActive,
		schedule.CreatedBy,
		nullableTime(schedule.LastRunAt),
		nullableTime(schedule.NextRunAt),
		nullableString(schedule.Timezone),
	)
	if err != nil {
		return fmt.Errorf("failed to create schedule: %w", err)
	}
	return nil
}

// Get возвращает расписание по ID.
func (s *ScheduleSQLStore) Get(ctx context.Context, scheduleID string) (*Schedule, error) {
	var sched Schedule
	var config []byte
	if err := s.db.QueryRowContext(ctx, `SELECT id, project_id, name, description, strategy, config, is_active, created_by, last_run_at, next_run_at, timezone, created_at, updated_at
		FROM generation_schedules WHERE id=$1`, scheduleID).
		Scan(&sched.ID, &sched.ProjectID, &sched.Name, &sched.Description, &sched.Strategy, &config, &sched.IsActive, &sched.CreatedBy, &sched.LastRunAt, &sched.NextRunAt, &sched.Timezone, &sched.CreatedAt, &sched.UpdatedAt); err != nil {
		return nil, err
	}
	sched.Config = config
	return &sched, nil
}

// List возвращает список расписаний для проекта.
func (s *ScheduleSQLStore) List(ctx context.Context, projectID string) ([]Schedule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, name, description, strategy, config, is_active, created_by, last_run_at, next_run_at, timezone, created_at, updated_at
		FROM generation_schedules WHERE project_id=$1 ORDER BY updated_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Schedule
	for rows.Next() {
		var sched Schedule
		var config []byte
		if err := rows.Scan(&sched.ID, &sched.ProjectID, &sched.Name, &sched.Description, &sched.Strategy, &config, &sched.IsActive, &sched.CreatedBy, &sched.LastRunAt, &sched.NextRunAt, &sched.Timezone, &sched.CreatedAt, &sched.UpdatedAt); err != nil {
			return nil, err
		}
		sched.Config = config
		res = append(res, sched)
	}
	return res, rows.Err()
}

// Update применяет частичные изменения к расписанию.
func (s *ScheduleSQLStore) Update(ctx context.Context, scheduleID string, updates ScheduleUpdates) error {
	setClauses := make([]string, 0, 6)
	args := make([]interface{}, 0, 6)
	idx := 1

	if updates.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name=$%d", idx))
		args = append(args, *updates.Name)
		idx++
	}
	if updates.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description=$%d", idx))
		args = append(args, nullableString(*updates.Description))
		idx++
	}
	if updates.Strategy != nil {
		setClauses = append(setClauses, fmt.Sprintf("strategy=$%d", idx))
		args = append(args, *updates.Strategy)
		idx++
	}
	if updates.Config != nil {
		setClauses = append(setClauses, fmt.Sprintf("config=$%d", idx))
		args = append(args, *updates.Config)
		idx++
	}
	if updates.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active=$%d", idx))
		args = append(args, *updates.IsActive)
		idx++
	}
	if updates.LastRunAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("last_run_at=$%d", idx))
		args = append(args, nullableTime(*updates.LastRunAt))
		idx++
	}
	if updates.NextRunAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("next_run_at=$%d", idx))
		args = append(args, nullableTime(*updates.NextRunAt))
		idx++
	}
	if updates.Timezone != nil {
		setClauses = append(setClauses, fmt.Sprintf("timezone=$%d", idx))
		args = append(args, nullableString(*updates.Timezone))
		idx++
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no schedule updates provided")
	}

	setClauses = append(setClauses, "updated_at=NOW()")
	query := fmt.Sprintf("UPDATE generation_schedules SET %s WHERE id=$%d", strings.Join(setClauses, ", "), idx)
	args = append(args, scheduleID)

	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to update schedule: %w", err)
	}
	return nil
}

// Delete удаляет расписание по ID.
func (s *ScheduleSQLStore) Delete(ctx context.Context, scheduleID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM generation_schedules WHERE id=$1`, scheduleID)
	return err
}

// ListActive возвращает список активных расписаний.
func (s *ScheduleSQLStore) ListActive(ctx context.Context) ([]Schedule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, name, description, strategy, config, is_active, created_by, last_run_at, next_run_at, timezone, created_at, updated_at
		FROM generation_schedules WHERE is_active=TRUE ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Schedule
	for rows.Next() {
		var sched Schedule
		var config []byte
		if err := rows.Scan(&sched.ID, &sched.ProjectID, &sched.Name, &sched.Description, &sched.Strategy, &config, &sched.IsActive, &sched.CreatedBy, &sched.LastRunAt, &sched.NextRunAt, &sched.Timezone, &sched.CreatedAt, &sched.UpdatedAt); err != nil {
			return nil, err
		}
		sched.Config = config
		res = append(res, sched)
	}
	return res, rows.Err()
}
