package publisher

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"

	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

type stubBackend struct {
	deleteAllCalls []string
	ensureDirCalls []string
	writeCalls     []string
}

func (s *stubBackend) ReadFile(context.Context, domainfs.DomainFSContext, string) ([]byte, error) {
	return nil, fs.ErrNotExist
}

func (s *stubBackend) WriteFile(_ context.Context, _ domainfs.DomainFSContext, filePath string, _ []byte) error {
	s.writeCalls = append(s.writeCalls, filePath)
	return nil
}

func (s *stubBackend) EnsureDir(_ context.Context, _ domainfs.DomainFSContext, dirPath string) error {
	s.ensureDirCalls = append(s.ensureDirCalls, dirPath)
	return nil
}

func (s *stubBackend) Stat(context.Context, domainfs.DomainFSContext, string) (domainfs.FileInfo, error) {
	return domainfs.FileInfo{}, fs.ErrNotExist
}

func (s *stubBackend) ReadDir(context.Context, domainfs.DomainFSContext, string) ([]domainfs.EntryInfo, error) {
	return nil, nil
}

func (s *stubBackend) Move(context.Context, domainfs.DomainFSContext, string, string) error {
	return nil
}

func (s *stubBackend) Delete(context.Context, domainfs.DomainFSContext, string) error {
	return nil
}

func (s *stubBackend) DeleteAll(_ context.Context, _ domainfs.DomainFSContext, dirPath string) error {
	s.deleteAllCalls = append(s.deleteAllCalls, dirPath)
	return nil
}

func (s *stubBackend) ListTree(context.Context, domainfs.DomainFSContext, string) ([]domainfs.FileInfo, error) {
	return nil, nil
}

func TestSSHPublisherPublishSuccess(t *testing.T) {
	ctx := context.Background()
	store := &stubDomainStore{
		domain: sqlstore.Domain{
			ID:             "d-1",
			URL:            "example.com",
			DeploymentMode: sql.NullString{String: "ssh_remote", Valid: true},
			PublishedPath:  sql.NullString{String: "/var/www/example.com", Valid: true},
			ServerID:       sql.NullString{String: "media1", Valid: true},
			SiteOwner:      sql.NullString{String: "u123:u123", Valid: true},
		},
	}
	backend := &stubBackend{}
	pub := NewSSHPublisher(backend, store, nil)

	err := pub.Publish(ctx, "d-1", map[string][]byte{
		"index.html":      []byte("ok"),
		"assets/logo.svg": []byte("<svg/>"),
	})
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if len(backend.deleteAllCalls) != 1 || backend.deleteAllCalls[0] != "" {
		t.Fatalf("expected root DeleteAll call, got: %#v", backend.deleteAllCalls)
	}
	if len(backend.ensureDirCalls) != 1 || backend.ensureDirCalls[0] != "" {
		t.Fatalf("expected root EnsureDir call, got: %#v", backend.ensureDirCalls)
	}
	if len(backend.writeCalls) != 2 {
		t.Fatalf("expected 2 WriteFile calls, got %d", len(backend.writeCalls))
	}
	if !store.updateCalled {
		t.Fatalf("expected UpdatePublishState to be called")
	}
	if store.publishedPath != "/var/www/example.com/" {
		t.Fatalf("unexpected published path: %s", store.publishedPath)
	}
	if store.fileCount != 2 {
		t.Fatalf("unexpected file count: %d", store.fileCount)
	}
}

func TestSSHPublisherPublishRequiresPublishedPath(t *testing.T) {
	ctx := context.Background()
	store := &stubDomainStore{
		domain: sqlstore.Domain{
			ID:             "d-1",
			URL:            "example.com",
			DeploymentMode: sql.NullString{String: "ssh_remote", Valid: true},
		},
	}
	backend := &stubBackend{}
	pub := NewSSHPublisher(backend, store, nil)
	err := pub.Publish(ctx, "d-1", map[string][]byte{"index.html": []byte("ok")})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(backend.writeCalls) != 0 {
		t.Fatalf("expected no writes on validation failure")
	}
}

func TestSSHPublisherGetPublishedPath(t *testing.T) {
	store := &stubDomainStore{
		domain: sqlstore.Domain{
			ID:             "d-1",
			URL:            "example.com",
			DeploymentMode: sql.NullString{String: "ssh_remote", Valid: true},
			PublishedPath:  sql.NullString{String: "/var/www/example.com", Valid: true},
		},
	}
	pub := NewSSHPublisher(&stubBackend{}, store, nil)
	if got := pub.GetPublishedPath("d-1"); got != "/var/www/example.com/" {
		t.Fatalf("unexpected published path: %s", got)
	}
}
