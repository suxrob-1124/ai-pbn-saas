package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// LinkSchedule описывает расписание линкбилдинга.
type LinkSchedule struct {
	ID        string
	ProjectID string
	Name      string
	Config    json.RawMessage
	IsActive  bool
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
	LastRunAt sql.NullTime
	NextRunAt sql.NullTime
	Timezone  sql.NullString
}

// LinkScheduleUpdates описывает изменения расписания линкбилдинга.
type LinkScheduleUpdates struct {
	Name      *string
	Config    *json.RawMessage
	IsActive  *bool
	LastRunAt *sql.NullTime
	NextRunAt *sql.NullTime
	Timezone  *sql.NullString
}

// LinkScheduleStore предоставляет доступ к расписаниям линкбилдинга.
type LinkScheduleStore struct {
	db *sql.DB
}

// NewLinkScheduleStore создает новый LinkScheduleStore.
func NewLinkScheduleStore(db *sql.DB) *LinkScheduleStore {
	return &LinkScheduleStore{db: db}
}

// GetByProject возвращает расписание по проекту.
func (s *LinkScheduleStore) GetByProject(ctx context.Context, projectID string) (*LinkSchedule, error) {
	var sched LinkSchedule
	if err := s.db.QueryRowContext(ctx, `SELECT id, project_id, name, config, is_active, created_by, created_at, updated_at, last_run_at, next_run_at, timezone
		FROM link_schedules WHERE project_id=$1`, projectID).
		Scan(
			&sched.ID,
			&sched.ProjectID,
			&sched.Name,
			&sched.Config,
			&sched.IsActive,
			&sched.CreatedBy,
			&sched.CreatedAt,
			&sched.UpdatedAt,
			&sched.LastRunAt,
			&sched.NextRunAt,
			&sched.Timezone,
		); err != nil {
		return nil, err
	}
	return &sched, nil
}

// Upsert создает или обновляет расписание (одно на проект).
func (s *LinkScheduleStore) Upsert(ctx context.Context, schedule LinkSchedule) (*LinkSchedule, error) {
	var stored LinkSchedule
	if err := s.db.QueryRowContext(ctx, `INSERT INTO link_schedules(
			id, project_id, name, config, is_active, created_by, created_at, updated_at, next_run_at, timezone
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (project_id) DO UPDATE SET
			name=EXCLUDED.name,
			config=EXCLUDED.config,
			is_active=EXCLUDED.is_active,
			next_run_at=EXCLUDED.next_run_at,
			timezone=EXCLUDED.timezone,
			updated_at=EXCLUDED.updated_at
		RETURNING id, project_id, name, config, is_active, created_by, created_at, updated_at, last_run_at, next_run_at, timezone`,
		schedule.ID,
		schedule.ProjectID,
		schedule.Name,
		schedule.Config,
		schedule.IsActive,
		schedule.CreatedBy,
		schedule.CreatedAt,
		schedule.UpdatedAt,
		nullableTime(schedule.NextRunAt),
		nullableString(schedule.Timezone),
	).Scan(
		&stored.ID,
		&stored.ProjectID,
		&stored.Name,
		&stored.Config,
		&stored.IsActive,
		&stored.CreatedBy,
		&stored.CreatedAt,
		&stored.UpdatedAt,
		&stored.LastRunAt,
		&stored.NextRunAt,
		&stored.Timezone,
	); err != nil {
		return nil, fmt.Errorf("failed to upsert link schedule: %w", err)
	}
	return &stored, nil
}

// Update обновляет расписание по ID.
func (s *LinkScheduleStore) Update(ctx context.Context, scheduleID string, updates LinkScheduleUpdates) error {
	setClauses := make([]string, 0, 6)
	args := make([]interface{}, 0, 6)
	idx := 1

	if updates.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name=$%d", idx))
		args = append(args, *updates.Name)
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
		return nil
	}

	setClauses = append(setClauses, "updated_at=NOW()")
	args = append(args, scheduleID)
	query := fmt.Sprintf("UPDATE link_schedules SET %s WHERE id=$%d", strings.Join(setClauses, ", "), idx)
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to update link schedule: %w", err)
	}
	return nil
}

// DisableByProject отключает расписание по проекту.
func (s *LinkScheduleStore) DisableByProject(ctx context.Context, projectID string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE link_schedules SET is_active=FALSE, updated_at=NOW() WHERE project_id=$1`, projectID)
	if err != nil {
		return fmt.Errorf("failed to disable link schedule: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteByProject удаляет расписание по проекту.
func (s *LinkScheduleStore) DeleteByProject(ctx context.Context, projectID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM link_schedules WHERE project_id=$1`, projectID)
	if err != nil {
		return fmt.Errorf("failed to delete link schedule: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListActive возвращает активные расписания линкбилдинга.
func (s *LinkScheduleStore) ListActive(ctx context.Context) ([]LinkSchedule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, name, config, is_active, created_by, created_at, updated_at, last_run_at, next_run_at, timezone
		FROM link_schedules
		WHERE is_active=TRUE
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list link schedules: %w", err)
	}
	defer rows.Close()

	var res []LinkSchedule
	for rows.Next() {
		var sched LinkSchedule
		if err := rows.Scan(
			&sched.ID,
			&sched.ProjectID,
			&sched.Name,
			&sched.Config,
			&sched.IsActive,
			&sched.CreatedBy,
			&sched.CreatedAt,
			&sched.UpdatedAt,
			&sched.LastRunAt,
			&sched.NextRunAt,
			&sched.Timezone,
		); err != nil {
			return nil, err
		}
		res = append(res, sched)
	}
	return res, rows.Err()
}
