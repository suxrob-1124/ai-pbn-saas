package publisher

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

type stubDomainStore struct {
	domain           sqlstore.Domain
	updateCalled     bool
	publishedPath    string
	fileCount        int
	totalSizeBytes   int64
	updateShouldFail bool
}

func (s *stubDomainStore) Get(ctx context.Context, id string) (sqlstore.Domain, error) {
	return s.domain, nil
}

func (s *stubDomainStore) UpdatePublishState(ctx context.Context, id, publishedPath string, fileCount int, totalSizeBytes int64) error {
	if s.updateShouldFail {
		return os.ErrPermission
	}
	s.updateCalled = true
	s.publishedPath = publishedPath
	s.fileCount = fileCount
	s.totalSizeBytes = totalSizeBytes
	return nil
}

func TestLocalPublisherPublishSuccess(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	serverDir := filepath.Join(baseDir, "server")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatalf("failed to create server dir: %v", err)
	}

	store := &stubDomainStore{domain: sqlstore.Domain{ID: "domain-1", URL: "example.com"}}
	pub := NewLocalPublisher(serverDir, store, nil)

	files := map[string][]byte{
		"index.html":     []byte("hello"),
		"css/main.css":   []byte("body{}"),
		"img/logo.svg":   []byte("<svg></svg>"),
		"nested/a/b.txt": []byte("ok"),
	}

	if err := pub.Publish(ctx, "domain-1", files); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	path := filepath.Join(serverDir, "example.com", "index.html")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	if !store.updateCalled {
		t.Fatalf("expected UpdatePublishState to be called")
	}
	if store.publishedPath != "/server/example.com/" {
		t.Fatalf("unexpected published path: %s", store.publishedPath)
	}
	if store.fileCount != 4 {
		t.Fatalf("unexpected file count: %d", store.fileCount)
	}
	if store.totalSizeBytes <= 0 {
		t.Fatalf("expected total size > 0")
	}
}

func TestLocalPublisherPublishFailsWhenServerMissing(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	serverDir := filepath.Join(baseDir, "server")

	store := &stubDomainStore{domain: sqlstore.Domain{ID: "domain-1", URL: "example.com"}}
	pub := NewLocalPublisher(serverDir, store, nil)

	files := map[string][]byte{
		"index.html": []byte("hello"),
	}

	if err := pub.Publish(ctx, "domain-1", files); err == nil {
		t.Fatalf("expected error when server dir is missing")
	}
}

func TestLocalPublisherPublishRejectsTraversal(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	serverDir := filepath.Join(baseDir, "server")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatalf("failed to create server dir: %v", err)
	}

	store := &stubDomainStore{domain: sqlstore.Domain{ID: "domain-1", URL: "example.com"}}
	pub := NewLocalPublisher(serverDir, store, nil)

	files := map[string][]byte{
		"../secrets.txt": []byte("nope"),
	}

	if err := pub.Publish(ctx, "domain-1", files); err == nil {
		t.Fatalf("expected error for path traversal")
	}
}
