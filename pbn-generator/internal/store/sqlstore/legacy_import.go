package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// LegacyImportJob represents a batch legacy import operation.
type LegacyImportJob struct {
	ID             string
	ProjectID      string
	RequestedBy    string
	ForceOverwrite bool
	Status         string // pending, running, completed, failed
	TotalItems     int
	CompletedItems int
	FailedItems    int
	SkippedItems   int
	Error          sql.NullString
	CreatedAt      time.Time
	StartedAt      sql.NullTime
	FinishedAt     sql.NullTime
	UpdatedAt      time.Time
}

// LegacyImportItem represents a single domain within a legacy import job.
type LegacyImportItem struct {
	ID             string
	JobID          string
	DomainID       string
	DomainURL      string
	Status         string // pending, running, success, failed, skipped
	Step           string // ssh_probe, file_sync, link_decode, link_baseline, artifacts, inventory_update
	Progress       int
	Error          sql.NullString
	Actions        []byte // JSONB
	Warnings       []byte // JSONB
	FileCount      int
	TotalSizeBytes int64
	StartedAt      sql.NullTime
	FinishedAt     sql.NullTime
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// LegacyImportStore handles persistence for legacy import jobs and items.
type LegacyImportStore struct {
	db *sql.DB
}

// NewLegacyImportStore creates a new LegacyImportStore.
func NewLegacyImportStore(db *sql.DB) *LegacyImportStore {
	return &LegacyImportStore{db: db}
}

// --- Job methods ---

func (s *LegacyImportStore) CreateJob(ctx context.Context, job LegacyImportJob) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO legacy_import_jobs(
		id, project_id, requested_by, force_overwrite, status, total_items, created_at, updated_at
	) VALUES($1,$2,$3,$4,$5,$6,NOW(),NOW())`,
		job.ID, job.ProjectID, job.RequestedBy, job.ForceOverwrite, job.Status, job.TotalItems,
	)
	if err != nil {
		return fmt.Errorf("create legacy import job: %w", err)
	}
	return nil
}

func (s *LegacyImportStore) GetJob(ctx context.Context, id string) (LegacyImportJob, error) {
	var j LegacyImportJob
	err := s.db.QueryRowContext(ctx, `SELECT
		id, project_id, requested_by, force_overwrite, status,
		total_items, completed_items, failed_items, skipped_items,
		error, created_at, started_at, finished_at, updated_at
		FROM legacy_import_jobs WHERE id=$1`, id).Scan(
		&j.ID, &j.ProjectID, &j.RequestedBy, &j.ForceOverwrite, &j.Status,
		&j.TotalItems, &j.CompletedItems, &j.FailedItems, &j.SkippedItems,
		&j.Error, &j.CreatedAt, &j.StartedAt, &j.FinishedAt, &j.UpdatedAt,
	)
	if err != nil {
		return LegacyImportJob{}, fmt.Errorf("get legacy import job: %w", err)
	}
	return j, nil
}

func (s *LegacyImportStore) ListJobsByProject(ctx context.Context, projectID string) ([]LegacyImportJob, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, project_id, requested_by, force_overwrite, status,
		total_items, completed_items, failed_items, skipped_items,
		error, created_at, started_at, finished_at, updated_at
		FROM legacy_import_jobs WHERE project_id=$1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list legacy import jobs: %w", err)
	}
	defer rows.Close()
	var out []LegacyImportJob
	for rows.Next() {
		var j LegacyImportJob
		if err := rows.Scan(
			&j.ID, &j.ProjectID, &j.RequestedBy, &j.ForceOverwrite, &j.Status,
			&j.TotalItems, &j.CompletedItems, &j.FailedItems, &j.SkippedItems,
			&j.Error, &j.CreatedAt, &j.StartedAt, &j.FinishedAt, &j.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan legacy import job: %w", err)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *LegacyImportStore) UpdateJobStarted(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE legacy_import_jobs
		SET status='running', started_at=NOW(), updated_at=NOW()
		WHERE id=$1 AND status='pending'`, id)
	if err != nil {
		return fmt.Errorf("update job started: %w", err)
	}
	return nil
}

func (s *LegacyImportStore) UpdateJobFinished(ctx context.Context, id, status string, errMsg *string) error {
	var errVal interface{}
	if errMsg != nil {
		errVal = *errMsg
	}
	_, err := s.db.ExecContext(ctx, `UPDATE legacy_import_jobs
		SET status=$1, error=$2, finished_at=NOW(), updated_at=NOW()
		WHERE id=$3`, status, errVal, id)
	if err != nil {
		return fmt.Errorf("update job finished: %w", err)
	}
	return nil
}

func (s *LegacyImportStore) IncrementJobCompleted(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE legacy_import_jobs
		SET completed_items=completed_items+1, updated_at=NOW()
		WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("increment job completed: %w", err)
	}
	return nil
}

func (s *LegacyImportStore) IncrementJobFailed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE legacy_import_jobs
		SET failed_items=failed_items+1, updated_at=NOW()
		WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("increment job failed: %w", err)
	}
	return nil
}

func (s *LegacyImportStore) IncrementJobSkipped(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE legacy_import_jobs
		SET skipped_items=skipped_items+1, updated_at=NOW()
		WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("increment job skipped: %w", err)
	}
	return nil
}

// --- Item methods ---

func (s *LegacyImportStore) CreateItem(ctx context.Context, item LegacyImportItem) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO legacy_import_items(
		id, job_id, domain_id, domain_url, status, created_at, updated_at
	) VALUES($1,$2,$3,$4,'pending',NOW(),NOW())`,
		item.ID, item.JobID, item.DomainID, item.DomainURL,
	)
	if err != nil {
		return fmt.Errorf("create legacy import item: %w", err)
	}
	return nil
}

func (s *LegacyImportStore) GetItem(ctx context.Context, id string) (LegacyImportItem, error) {
	var it LegacyImportItem
	err := s.db.QueryRowContext(ctx, `SELECT
		id, job_id, domain_id, domain_url, status, step, progress,
		error, actions, warnings, file_count, total_size_bytes,
		started_at, finished_at, created_at, updated_at
		FROM legacy_import_items WHERE id=$1`, id).Scan(
		&it.ID, &it.JobID, &it.DomainID, &it.DomainURL, &it.Status, &it.Step, &it.Progress,
		&it.Error, &it.Actions, &it.Warnings, &it.FileCount, &it.TotalSizeBytes,
		&it.StartedAt, &it.FinishedAt, &it.CreatedAt, &it.UpdatedAt,
	)
	if err != nil {
		return LegacyImportItem{}, fmt.Errorf("get legacy import item: %w", err)
	}
	return it, nil
}

func (s *LegacyImportStore) ListItemsByJob(ctx context.Context, jobID string) ([]LegacyImportItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, job_id, domain_id, domain_url, status, step, progress,
		error, actions, warnings, file_count, total_size_bytes,
		started_at, finished_at, created_at, updated_at
		FROM legacy_import_items WHERE job_id=$1 ORDER BY created_at ASC`, jobID)
	if err != nil {
		return nil, fmt.Errorf("list legacy import items: %w", err)
	}
	defer rows.Close()
	var out []LegacyImportItem
	for rows.Next() {
		var it LegacyImportItem
		if err := rows.Scan(
			&it.ID, &it.JobID, &it.DomainID, &it.DomainURL, &it.Status, &it.Step, &it.Progress,
			&it.Error, &it.Actions, &it.Warnings, &it.FileCount, &it.TotalSizeBytes,
			&it.StartedAt, &it.FinishedAt, &it.CreatedAt, &it.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan legacy import item: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *LegacyImportStore) UpdateItemStatus(ctx context.Context, id, status, step string, progress int, errMsg *string) error {
	var errVal interface{}
	if errMsg != nil {
		errVal = *errMsg
	}
	startedClause := ""
	if status == "running" {
		startedClause = ", started_at=COALESCE(started_at, NOW())"
	}
	query := fmt.Sprintf(`UPDATE legacy_import_items
		SET status=$1, step=$2, progress=$3, error=$4, updated_at=NOW()%s
		WHERE id=$5`, startedClause)
	_, err := s.db.ExecContext(ctx, query, status, step, progress, errVal, id)
	if err != nil {
		return fmt.Errorf("update item status: %w", err)
	}
	return nil
}

func (s *LegacyImportStore) UpdateItemResult(ctx context.Context, id, status string, actions, warnings []byte, fileCount int, totalSize int64, errMsg *string) error {
	var errVal interface{}
	if errMsg != nil {
		errVal = *errMsg
	}
	_, err := s.db.ExecContext(ctx, `UPDATE legacy_import_items
		SET status=$1, progress=100, error=$2, actions=$3, warnings=$4,
			file_count=$5, total_size_bytes=$6, finished_at=NOW(), updated_at=NOW()
		WHERE id=$7`,
		status, errVal, actions, warnings, fileCount, totalSize, id,
	)
	if err != nil {
		return fmt.Errorf("update item result: %w", err)
	}
	return nil
}
