package publisher

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

// SiteFileStore описывает операции, необходимые для синхронизации файлов.
type SiteFileStore interface {
	Create(ctx context.Context, file sqlstore.SiteFile) error
	GetByPath(ctx context.Context, domainID, path string) (*sqlstore.SiteFile, error)
	List(ctx context.Context, domainID string) ([]sqlstore.SiteFile, error)
	Update(ctx context.Context, fileID string, content []byte) error
	Delete(ctx context.Context, fileID string) error
}

// SyncPublishedFilesFromMap синхронизирует файлы домена в БД по уже собранной map.
func SyncPublishedFilesFromMap(ctx context.Context, domainID string, files map[string][]byte, store SiteFileStore) (int, int64, error) {
	if store == nil {
		return 0, 0, errors.New("site file store is nil")
	}
	if ctx.Err() != nil {
		return 0, 0, ctx.Err()
	}
	if len(files) == 0 {
		return 0, 0, errors.New("sync files: no files provided")
	}

	fileCount := 0
	var totalSize int64
	seenPaths := map[string]struct{}{}

	for rawPath, content := range files {
		if ctx.Err() != nil {
			return 0, 0, ctx.Err()
		}
		clean, err := sanitizeFilePath(rawPath)
		if err != nil {
			return 0, 0, fmt.Errorf("sync files: invalid file path %q: %w", rawPath, err)
		}
		seenPaths[clean] = struct{}{}
		hash := sha256.Sum256(content)
		hashStr := hex.EncodeToString(hash[:])
		mimeType := detectMimeType(clean, content)
		size := int64(len(content))

		existing, err := store.GetByPath(ctx, domainID, clean)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return 0, 0, err
			}
			file := sqlstore.SiteFile{
				ID:          uuid.NewString(),
				DomainID:    domainID,
				Path:        clean,
				ContentHash: sql.NullString{String: hashStr, Valid: true},
				SizeBytes:   size,
				MimeType:    mimeType,
			}
			if err := store.Create(ctx, file); err != nil {
				return 0, 0, err
			}
		} else {
			if err := store.Update(ctx, existing.ID, content); err != nil {
				return 0, 0, err
			}
		}

		fileCount++
		totalSize += size
	}

	existingFiles, err := store.List(ctx, domainID)
	if err != nil {
		return 0, 0, err
	}
	for _, file := range existingFiles {
		if _, ok := seenPaths[file.Path]; ok {
			continue
		}
		if err := store.Delete(ctx, file.ID); err != nil {
			return 0, 0, err
		}
	}

	if fileCount == 0 {
		return 0, 0, errors.New("sync files: no files found")
	}
	return fileCount, totalSize, nil
}

// SyncPublishedFiles сканирует опубликованные файлы и синхронизирует их с БД.
func SyncPublishedFiles(ctx context.Context, baseDir, domainName, domainID string, store SiteFileStore) (int, int64, error) {
	if store == nil {
		return 0, 0, errors.New("site file store is nil")
	}
	if ctx.Err() != nil {
		return 0, 0, ctx.Err()
	}
	root := filepath.Join(baseDir, domainName)
	info, err := os.Stat(root)
	if err != nil {
		return 0, 0, fmt.Errorf("sync files: %w", err)
	}
	if !info.IsDir() {
		return 0, 0, fmt.Errorf("sync files: %s is not a directory", root)
	}

	fileCount := 0
	var totalSize int64
	seenPaths := map[string]struct{}{}

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") || rel == ".." {
			return fmt.Errorf("sync files: invalid path %s", rel)
		}
		seenPaths[rel] = struct{}{}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(content)
		hashStr := hex.EncodeToString(hash[:])
		mimeType := detectMimeType(rel, content)
		size := int64(len(content))

		existing, err := store.GetByPath(ctx, domainID, rel)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			file := sqlstore.SiteFile{
				ID:          uuid.NewString(),
				DomainID:    domainID,
				Path:        rel,
				ContentHash: sql.NullString{String: hashStr, Valid: true},
				SizeBytes:   size,
				MimeType:    mimeType,
			}
			if err := store.Create(ctx, file); err != nil {
				return err
			}
		} else {
			if err := store.Update(ctx, existing.ID, content); err != nil {
				return err
			}
		}

		fileCount++
		totalSize += size
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	existingFiles, err := store.List(ctx, domainID)
	if err != nil {
		return 0, 0, err
	}
	for _, file := range existingFiles {
		if _, ok := seenPaths[file.Path]; ok {
			continue
		}
		if err := store.Delete(ctx, file.ID); err != nil {
			return 0, 0, err
		}
	}

	if fileCount == 0 {
		return 0, 0, errors.New("sync files: no files found")
	}
	return fileCount, totalSize, nil
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
