package sqlstore

import (
	"context"
	"database/sql"
	"time"
)

type DeploymentAttempt struct {
	ID             string
	DomainID       string
	GenerationID   string
	Mode           string
	TargetPath     string
	OwnerBefore    sql.NullString
	OwnerAfter     sql.NullString
	Status         string
	ErrorMessage   sql.NullString
	FileCount      int
	TotalSizeBytes int64
	CreatedAt      time.Time
	FinishedAt     sql.NullTime
}

type DeploymentAttemptStore struct {
	db *sql.DB
}

func NewDeploymentAttemptStore(db *sql.DB) *DeploymentAttemptStore {
	return &DeploymentAttemptStore{db: db}
}

func (s *DeploymentAttemptStore) Create(ctx context.Context, item DeploymentAttempt) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO deployment_attempts(
		id, domain_id, generation_id, mode, target_path, owner_before, owner_after, status, error_message, file_count, total_size_bytes, created_at, finished_at
	) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),$12)`,
		item.ID,
		item.DomainID,
		item.GenerationID,
		item.Mode,
		item.TargetPath,
		nullableString(item.OwnerBefore),
		nullableString(item.OwnerAfter),
		item.Status,
		nullableString(item.ErrorMessage),
		item.FileCount,
		item.TotalSizeBytes,
		nullableTime(item.FinishedAt),
	)
	return err
}

func (s *DeploymentAttemptStore) Finish(ctx context.Context, id, status string, errMsg, ownerAfter *string, fileCount int, totalSizeBytes int64, finishedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE deployment_attempts
SET status=$1,
	error_message=$2,
	owner_after=$3,
	file_count=$4,
	total_size_bytes=$5,
	finished_at=$6
WHERE id=$7`,
		status,
		nullableString(nullString(errMsg)),
		nullableString(nullString(ownerAfter)),
		fileCount,
		totalSizeBytes,
		finishedAt,
		id,
	)
	return err
}

func (s *DeploymentAttemptStore) ListByDomain(ctx context.Context, domainID string, limit int) ([]DeploymentAttempt, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, generation_id, mode, target_path, owner_before, owner_after, status, error_message, file_count, total_size_bytes, created_at, finished_at
FROM deployment_attempts
WHERE domain_id=$1
ORDER BY created_at DESC
LIMIT $2`, domainID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []DeploymentAttempt
	for rows.Next() {
		var item DeploymentAttempt
		if err := rows.Scan(&item.ID, &item.DomainID, &item.GenerationID, &item.Mode, &item.TargetPath, &item.OwnerBefore, &item.OwnerAfter, &item.Status, &item.ErrorMessage, &item.FileCount, &item.TotalSizeBytes, &item.CreatedAt, &item.FinishedAt); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

