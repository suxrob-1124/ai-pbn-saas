package sqlstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// SiteFile описывает файл, опубликованный для домена.
type SiteFile struct {
	ID           string
	DomainID     string
	Path         string
	ContentHash  sql.NullString
	SizeBytes    int64
	MimeType     string
	Version      int
	LastEditedBy sql.NullString
	DeletedAt    sql.NullTime
	DeletedBy    sql.NullString
	DeleteReason sql.NullString
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SiteFileStore определяет операции над файлами домена.
type SiteFileStore interface {
	Create(ctx context.Context, file SiteFile) error
	Get(ctx context.Context, fileID string) (*SiteFile, error)
	GetByPath(ctx context.Context, domainID, path string) (*SiteFile, error)
	GetByPathAny(ctx context.Context, domainID, path string) (*SiteFile, error)
	List(ctx context.Context, domainID string) ([]SiteFile, error)
	ListDeleted(ctx context.Context, domainID string) ([]SiteFile, error)
	Update(ctx context.Context, fileID string, content []byte) error
	Delete(ctx context.Context, fileID string) error
	SoftDelete(ctx context.Context, fileID string, deletedBy sql.NullString, reason sql.NullString) error
	Restore(ctx context.Context, fileID string) error
	Move(ctx context.Context, fileID, newPath string) error
	SetLastEditedBy(ctx context.Context, fileID string, editedBy sql.NullString) error
	UpdateHash(ctx context.Context, fileID, hash string) error
	ClearHistory(ctx context.Context, domainID string) error
}

// SiteFileSQLStore реализует SiteFileStore поверх SQL БД.
type SiteFileSQLStore struct {
	db *sql.DB
}

// NewSiteFileStore создает новый SiteFileSQLStore.
func NewSiteFileStore(db *sql.DB) *SiteFileSQLStore {
	return &SiteFileSQLStore{db: db}
}

// Create создает запись о файле домена.
func (s *SiteFileSQLStore) Create(ctx context.Context, file SiteFile) error {
	version := file.Version
	if version <= 0 {
		version = 1
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO site_files(id, domain_id, path, content_hash, size_bytes, mime_type, version, last_edited_by, deleted_at, deleted_by, delete_reason, created_at, updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULL,NULL,NULL,NOW(),NOW())`,
		file.ID,
		file.DomainID,
		file.Path,
		nullableString(file.ContentHash),
		file.SizeBytes,
		file.MimeType,
		version,
		nullableString(file.LastEditedBy),
	)
	if err != nil {
		return fmt.Errorf("failed to create site file: %w", err)
	}
	return nil
}

// Get возвращает файл по ID.
func (s *SiteFileSQLStore) Get(ctx context.Context, fileID string) (*SiteFile, error) {
	var f SiteFile
	err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, path, content_hash, size_bytes, mime_type, version, last_edited_by, deleted_at, deleted_by, delete_reason, created_at, updated_at
		FROM site_files WHERE id=$1`, fileID).
		Scan(&f.ID, &f.DomainID, &f.Path, &f.ContentHash, &f.SizeBytes, &f.MimeType, &f.Version, &f.LastEditedBy, &f.DeletedAt, &f.DeletedBy, &f.DeleteReason, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// GetByPath возвращает файл по домену и пути.
func (s *SiteFileSQLStore) GetByPath(ctx context.Context, domainID, path string) (*SiteFile, error) {
	var f SiteFile
	err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, path, content_hash, size_bytes, mime_type, version, last_edited_by, deleted_at, deleted_by, delete_reason, created_at, updated_at
		FROM site_files WHERE domain_id=$1 AND path=$2 AND deleted_at IS NULL`, domainID, path).
		Scan(&f.ID, &f.DomainID, &f.Path, &f.ContentHash, &f.SizeBytes, &f.MimeType, &f.Version, &f.LastEditedBy, &f.DeletedAt, &f.DeletedBy, &f.DeleteReason, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// GetByPathAny возвращает файл по домену и пути вне зависимости от deleted_at.
func (s *SiteFileSQLStore) GetByPathAny(ctx context.Context, domainID, path string) (*SiteFile, error) {
	var f SiteFile
	err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, path, content_hash, size_bytes, mime_type, version, last_edited_by, deleted_at, deleted_by, delete_reason, created_at, updated_at
		FROM site_files WHERE domain_id=$1 AND path=$2`, domainID, path).
		Scan(&f.ID, &f.DomainID, &f.Path, &f.ContentHash, &f.SizeBytes, &f.MimeType, &f.Version, &f.LastEditedBy, &f.DeletedAt, &f.DeletedBy, &f.DeleteReason, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// List возвращает список файлов домена.
func (s *SiteFileSQLStore) List(ctx context.Context, domainID string) ([]SiteFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, path, content_hash, size_bytes, mime_type, version, last_edited_by, deleted_at, deleted_by, delete_reason, created_at, updated_at
		FROM site_files WHERE domain_id=$1 AND deleted_at IS NULL ORDER BY path`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []SiteFile
	for rows.Next() {
		var f SiteFile
		if err := rows.Scan(&f.ID, &f.DomainID, &f.Path, &f.ContentHash, &f.SizeBytes, &f.MimeType, &f.Version, &f.LastEditedBy, &f.DeletedAt, &f.DeletedBy, &f.DeleteReason, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, f)
	}
	return res, rows.Err()
}

// ListDeleted возвращает список soft-deleted файлов домена.
func (s *SiteFileSQLStore) ListDeleted(ctx context.Context, domainID string) ([]SiteFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, path, content_hash, size_bytes, mime_type, version, last_edited_by, deleted_at, deleted_by, delete_reason, created_at, updated_at
		FROM site_files WHERE domain_id=$1 AND deleted_at IS NOT NULL ORDER BY deleted_at DESC, path`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []SiteFile
	for rows.Next() {
		var f SiteFile
		if err := rows.Scan(&f.ID, &f.DomainID, &f.Path, &f.ContentHash, &f.SizeBytes, &f.MimeType, &f.Version, &f.LastEditedBy, &f.DeletedAt, &f.DeletedBy, &f.DeleteReason, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, f)
	}
	return res, rows.Err()
}

// Update обновляет хэш, размер и mime типа файла.
func (s *SiteFileSQLStore) Update(ctx context.Context, fileID string, content []byte) error {
	file, err := s.Get(ctx, fileID)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])
	mimeType := detectMimeType(file.Path, content)
	_, err = s.db.ExecContext(ctx, `UPDATE site_files
		SET content_hash=$1, size_bytes=$2, mime_type=$3, version = GREATEST(1, version + 1), updated_at=NOW()
		WHERE id=$4`, hashStr, int64(len(content)), mimeType, fileID)
	if err != nil {
		return fmt.Errorf("failed to update site file: %w", err)
	}
	return nil
}

// Delete удаляет запись о файле.
func (s *SiteFileSQLStore) Delete(ctx context.Context, fileID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM site_files WHERE id=$1`, fileID)
	return err
}

// SoftDelete помечает файл удаленным.
func (s *SiteFileSQLStore) SoftDelete(ctx context.Context, fileID string, deletedBy sql.NullString, reason sql.NullString) error {
	_, err := s.db.ExecContext(ctx, `UPDATE site_files
		SET deleted_at=NOW(), deleted_by=$1, delete_reason=$2, updated_at=NOW()
		WHERE id=$3`, nullableString(deletedBy), nullableString(reason), fileID)
	if err != nil {
		return fmt.Errorf("failed to soft-delete site file: %w", err)
	}
	return nil
}

// Restore снимает soft-delete с файла.
func (s *SiteFileSQLStore) Restore(ctx context.Context, fileID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE site_files
		SET deleted_at=NULL, deleted_by=NULL, delete_reason=NULL, updated_at=NOW()
		WHERE id=$1`, fileID)
	if err != nil {
		return fmt.Errorf("failed to restore site file: %w", err)
	}
	return nil
}

// Move обновляет путь файла.
func (s *SiteFileSQLStore) Move(ctx context.Context, fileID, newPath string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE site_files SET path=$1, updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`, newPath, fileID)
	if err != nil {
		return fmt.Errorf("failed to move site file: %w", err)
	}
	return nil
}

// SetLastEditedBy обновляет автора последнего изменения файла.
func (s *SiteFileSQLStore) SetLastEditedBy(ctx context.Context, fileID string, editedBy sql.NullString) error {
	_, err := s.db.ExecContext(ctx, `UPDATE site_files SET last_edited_by=$1, updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`, nullableString(editedBy), fileID)
	if err != nil {
		return fmt.Errorf("failed to set file editor: %w", err)
	}
	return nil
}

// UpdateHash обновляет только хэш файла.
func (s *SiteFileSQLStore) UpdateHash(ctx context.Context, fileID, hash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE site_files SET content_hash=$1, updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`, hash, fileID)
	return err
}

// ClearHistory удаляет все file_revisions, file_edits и soft-deleted записи для домена,
// сбрасывает версии активных файлов. Вызывается при перегенерации сайта — новый сайт
// означает, что корзина предыдущей версии уже неактуальна.
func (s *SiteFileSQLStore) ClearHistory(ctx context.Context, domainID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM file_revisions WHERE file_id IN (SELECT id FROM site_files WHERE domain_id = $1)`, domainID)
	if err != nil {
		return fmt.Errorf("failed to clear file revisions for domain %s: %w", domainID, err)
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM file_edits WHERE file_id IN (SELECT id FROM site_files WHERE domain_id = $1)`, domainID)
	if err != nil {
		return fmt.Errorf("failed to clear file edits for domain %s: %w", domainID, err)
	}
	// Удаляем записи в корзине — перегенерация заменяет весь сайт, старые удалённые файлы неактуальны.
	_, err = s.db.ExecContext(ctx, `DELETE FROM site_files WHERE domain_id = $1 AND deleted_at IS NOT NULL`, domainID)
	if err != nil {
		return fmt.Errorf("failed to clear soft-deleted site files for domain %s: %w", domainID, err)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE site_files SET version = 1, updated_at = NOW() WHERE domain_id = $1`, domainID)
	if err != nil {
		return fmt.Errorf("failed to reset file versions for domain %s: %w", domainID, err)
	}
	return nil
}

// PurgeDeletedOlderThan жёстко удаляет soft-deleted файлы старше retentionDays дней.
// Используется периодической очисткой для предотвращения раздувания таблицы.
func (s *SiteFileSQLStore) PurgeDeletedOlderThan(ctx context.Context, retentionDays int) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM site_files WHERE deleted_at IS NOT NULL AND deleted_at < NOW() - make_interval(days => $1)`,
		retentionDays)
	if err != nil {
		return 0, fmt.Errorf("purge deleted site files: %w", err)
	}
	return res.RowsAffected()
}

func detectMimeType(path string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}
	if len(content) > 0 {
		return http.DetectContentType(content)
	}
	return "application/octet-stream"
}
