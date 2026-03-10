package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type FileLock struct {
	DomainID  string
	FilePath  string
	LockedBy  string
	LockedAt  time.Time
	ExpiresAt time.Time
}

type FileLockStore interface {
	// Acquire upserts a lock. Returns the resulting lock and whether the requesting
	// user owns it (isOwner=true means acquired or renewed; false means locked by someone else).
	Acquire(ctx context.Context, domainID, filePath, userEmail string, ttl time.Duration) (FileLock, bool, error)
	Release(ctx context.Context, domainID, filePath, userEmail string) error
	ListByDomain(ctx context.Context, domainID string) ([]FileLock, error)
	PurgeExpired(ctx context.Context) (int64, error)
}

type fileLockSQLStore struct{ db *sql.DB }

func NewFileLockStore(db *sql.DB) FileLockStore { return &fileLockSQLStore{db: db} }

func (s *fileLockSQLStore) Acquire(ctx context.Context, domainID, filePath, userEmail string, ttl time.Duration) (FileLock, bool, error) {
	expiresAt := time.Now().UTC().Add(ttl)
	// Upsert: insert or update only if no live lock from a different user.
	// Postgres: insert; on conflict update if lock expired OR belongs to same user.
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO file_locks(domain_id, file_path, locked_by, locked_at, expires_at)
		VALUES($1, $2, $3, NOW(), $4)
		ON CONFLICT(domain_id, file_path) DO UPDATE
		  SET locked_by = $3, locked_at = NOW(), expires_at = $4
		  WHERE file_locks.expires_at < NOW() OR file_locks.locked_by = $3`,
		domainID, filePath, userEmail, expiresAt)
	if err != nil {
		return FileLock{}, false, fmt.Errorf("acquire file lock: %w", err)
	}
	// Read current state to determine outcome.
	var lock FileLock
	err = s.db.QueryRowContext(ctx,
		`SELECT domain_id, file_path, locked_by, locked_at, expires_at
		 FROM file_locks WHERE domain_id=$1 AND file_path=$2`,
		domainID, filePath).
		Scan(&lock.DomainID, &lock.FilePath, &lock.LockedBy, &lock.LockedAt, &lock.ExpiresAt)
	if err != nil {
		return FileLock{}, false, fmt.Errorf("read file lock: %w", err)
	}
	return lock, lock.LockedBy == userEmail, nil
}

func (s *fileLockSQLStore) Release(ctx context.Context, domainID, filePath, userEmail string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM file_locks WHERE domain_id=$1 AND file_path=$2 AND locked_by=$3`,
		domainID, filePath, userEmail)
	if err != nil {
		return fmt.Errorf("release file lock: %w", err)
	}
	return nil
}

func (s *fileLockSQLStore) ListByDomain(ctx context.Context, domainID string) ([]FileLock, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT domain_id, file_path, locked_by, locked_at, expires_at
		 FROM file_locks
		 WHERE domain_id=$1 AND expires_at > NOW()
		 ORDER BY locked_at DESC`,
		domainID)
	if err != nil {
		return nil, fmt.Errorf("list file locks: %w", err)
	}
	defer rows.Close()
	var result []FileLock
	for rows.Next() {
		var l FileLock
		if err := rows.Scan(&l.DomainID, &l.FilePath, &l.LockedBy, &l.LockedAt, &l.ExpiresAt); err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func (s *fileLockSQLStore) PurgeExpired(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM file_locks WHERE expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("purge expired file locks: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
