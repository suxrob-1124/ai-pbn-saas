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
	deleted   []string
	existing  map[string]*sqlstore.SiteFile
	createErr error
	updateErr error
	listErr   error
	deleteErr error
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

func (s *stubSiteFileStore) List(ctx context.Context, domainID string) ([]sqlstore.SiteFile, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	res := make([]sqlstore.SiteFile, 0, len(s.existing))
	for _, file := range s.existing {
		res = append(res, *file)
	}
	return res, nil
}

func (s *stubSiteFileStore) Delete(ctx context.Context, fileID string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, fileID)
	for p, file := range s.existing {
		if file.ID == fileID {
			delete(s.existing, p)
			break
		}
	}
	return nil
}

func (s *stubSiteFileStore) ClearHistory(ctx context.Context, domainID string) error {
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
		"old.html":   {ID: "file-old", DomainID: "domain-1", Path: "old.html"},
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
	if len(store.deleted) != 1 {
		t.Fatalf("expected 1 deleted file, got %d", len(store.deleted))
	}
	if store.deleted[0] != "file-old" {
		t.Fatalf("unexpected deleted file id: %s", store.deleted[0])
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
