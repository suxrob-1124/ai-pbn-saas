package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// FileEdit описывает изменение файла.
type FileEdit struct {
	ID                string
	FileID            string
	EditedBy          string
	ContentBeforeHash sql.NullString
	ContentAfterHash  sql.NullString
	EditType          string
	EditDescription   sql.NullString
	CreatedAt         time.Time
}

// FileRevision описывает снапшот содержимого файла.
type FileRevision struct {
	ID          string
	FileID      string
	Version     int
	Content     []byte
	ContentHash string
	SizeBytes   int64
	MimeType    string
	Source      string
	Description sql.NullString
	EditedBy    string
	CreatedAt   time.Time
}

// FileEditStore определяет операции с логами изменений файлов.
type FileEditStore interface {
	Create(ctx context.Context, edit FileEdit) error
	ListByFile(ctx context.Context, fileID string, limit int) ([]FileEdit, error)
	ListByUser(ctx context.Context, userEmail string, limit int) ([]FileEdit, error)
	CreateRevision(ctx context.Context, rev FileRevision) error
	GetRevision(ctx context.Context, revisionID string) (*FileRevision, error)
	ListRevisionsByFile(ctx context.Context, fileID string, limit int) ([]FileRevision, error)
}

// FileEditSQLStore реализует FileEditStore поверх SQL БД.
type FileEditSQLStore struct {
	db *sql.DB
}

// NewFileEditStore создает новый FileEditSQLStore.
func NewFileEditStore(db *sql.DB) *FileEditSQLStore {
	return &FileEditSQLStore{db: db}
}

// Create создает запись о редактировании файла.
func (s *FileEditSQLStore) Create(ctx context.Context, edit FileEdit) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO file_edits(id, file_id, edited_by, content_before_hash, content_after_hash, edit_type, edit_description, created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,NOW())`,
		edit.ID,
		edit.FileID,
		edit.EditedBy,
		nullableString(edit.ContentBeforeHash),
		nullableString(edit.ContentAfterHash),
		edit.EditType,
		nullableString(edit.EditDescription),
	)
	if err != nil {
		return fmt.Errorf("failed to create file edit: %w", err)
	}
	return nil
}

// ListByFile возвращает историю изменений файла.
func (s *FileEditSQLStore) ListByFile(ctx context.Context, fileID string, limit int) ([]FileEdit, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, file_id, edited_by, content_before_hash, content_after_hash, edit_type, edit_description, created_at
		FROM file_edits WHERE file_id=$1 ORDER BY created_at DESC LIMIT $2`, fileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []FileEdit
	for rows.Next() {
		var e FileEdit
		if err := rows.Scan(&e.ID, &e.FileID, &e.EditedBy, &e.ContentBeforeHash, &e.ContentAfterHash, &e.EditType, &e.EditDescription, &e.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, e)
	}
	return res, rows.Err()
}

// ListByUser возвращает историю изменений пользователя.
func (s *FileEditSQLStore) ListByUser(ctx context.Context, userEmail string, limit int) ([]FileEdit, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, file_id, edited_by, content_before_hash, content_after_hash, edit_type, edit_description, created_at
		FROM file_edits WHERE edited_by=$1 ORDER BY created_at DESC LIMIT $2`, userEmail, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []FileEdit
	for rows.Next() {
		var e FileEdit
		if err := rows.Scan(&e.ID, &e.FileID, &e.EditedBy, &e.ContentBeforeHash, &e.ContentAfterHash, &e.EditType, &e.EditDescription, &e.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, e)
	}
	return res, rows.Err()
}

// CreateRevision сохраняет снапшот версии файла.
func (s *FileEditSQLStore) CreateRevision(ctx context.Context, rev FileRevision) error {
	if rev.Version <= 0 {
		return fmt.Errorf("failed to create file revision: invalid version")
	}
	if len(rev.Content) == 0 {
		rev.Content = []byte{}
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO file_revisions(id, file_id, version, content, content_hash, size_bytes, mime_type, source, description, edited_by, created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())
ON CONFLICT (file_id, version) DO NOTHING`,
		rev.ID,
		rev.FileID,
		rev.Version,
		rev.Content,
		rev.ContentHash,
		rev.SizeBytes,
		rev.MimeType,
		rev.Source,
		nullableString(rev.Description),
		rev.EditedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to create file revision: %w", err)
	}
	return nil
}

// GetRevision возвращает снапшот по ID.
func (s *FileEditSQLStore) GetRevision(ctx context.Context, revisionID string) (*FileRevision, error) {
	var rev FileRevision
	err := s.db.QueryRowContext(ctx, `SELECT id, file_id, version, content, content_hash, size_bytes, mime_type, source, description, edited_by, created_at
FROM file_revisions WHERE id=$1`,
		revisionID,
	).Scan(
		&rev.ID,
		&rev.FileID,
		&rev.Version,
		&rev.Content,
		&rev.ContentHash,
		&rev.SizeBytes,
		&rev.MimeType,
		&rev.Source,
		&rev.Description,
		&rev.EditedBy,
		&rev.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

// ListRevisionsByFile возвращает историю снапшотов файла.
func (s *FileEditSQLStore) ListRevisionsByFile(ctx context.Context, fileID string, limit int) ([]FileRevision, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, file_id, version, content, content_hash, size_bytes, mime_type, source, description, edited_by, created_at
FROM file_revisions
WHERE file_id=$1
ORDER BY version DESC, created_at DESC
LIMIT $2`, fileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]FileRevision, 0)
	for rows.Next() {
		var rev FileRevision
		if err := rows.Scan(
			&rev.ID,
			&rev.FileID,
			&rev.Version,
			&rev.Content,
			&rev.ContentHash,
			&rev.SizeBytes,
			&rev.MimeType,
			&rev.Source,
			&rev.Description,
			&rev.EditedBy,
			&rev.CreatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, rev)
	}
	return res, rows.Err()
}
