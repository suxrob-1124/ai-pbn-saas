package publisher

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

type stubSiteFileStore struct {
	created   []sqlstore.SiteFile
	updated   []string
	existing  map[string]*sqlstore.SiteFile
	createErr error
	updateErr error
}

func (s *stubSiteFileStore) Create(ctx context.Context, file sqlstore.SiteFile) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.created = append(s.created, file)
	s.existing[file.Path] = &file
	return nil
}

func (s *stubSiteFileStore) GetByPath(ctx context.Context, domainID, path string) (*sqlstore.SiteFile, error) {
	if file, ok := s.existing[path]; ok {
		return file, nil
	}
	return nil, sql.ErrNoRows
}

func (s *stubSiteFileStore) Update(ctx context.Context, fileID string, content []byte) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updated = append(s.updated, fileID)
	return nil
}

func TestSyncPublishedFilesSuccess(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	domain := "example.com"
	root := filepath.Join(baseDir, domain)
	if err := os.MkdirAll(filepath.Join(root, "css"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "css", "main.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	store := &stubSiteFileStore{existing: map[string]*sqlstore.SiteFile{
		"index.html": {ID: "file-1", DomainID: "domain-1", Path: "index.html"},
	}}

	count, size, err := SyncPublishedFiles(ctx, baseDir, domain, "domain-1", store)
	if err != nil {
		t.Fatalf("SyncPublishedFiles error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 files, got %d", count)
	}
	if size == 0 {
		t.Fatalf("expected non-zero size")
	}
	if len(store.created) != 1 {
		t.Fatalf("expected 1 created file, got %d", len(store.created))
	}
	if len(store.updated) != 1 {
		t.Fatalf("expected 1 updated file, got %d", len(store.updated))
	}
}

func TestSyncPublishedFilesMissingDir(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	store := &stubSiteFileStore{existing: map[string]*sqlstore.SiteFile{}}

	if _, _, err := SyncPublishedFiles(ctx, baseDir, "missing", "domain-1", store); err == nil {
		t.Fatalf("expected error when domain dir missing")
	}
}
