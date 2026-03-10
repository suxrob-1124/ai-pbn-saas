package httpserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// TestAgentSnapshotTag verifies the snapshot tag format.
func TestAgentSnapshotTag(t *testing.T) {
	tag := agentSnapshotTag("sess-abc-123")
	expected := "agent_snapshot:sess-abc-123"
	if tag != expected {
		t.Errorf("agentSnapshotTag() = %q, want %q", tag, expected)
	}
}

// TestCreateAndRollbackAgentSnapshot creates a snapshot of domain files and verifies rollback restores them.
func TestCreateAndRollbackAgentSnapshot(t *testing.T) {
	// Set up temp dir as content backend (server/<domain-url>/)
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write initial file
	originalContent := []byte("<html>original</html>")
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), originalContent, 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}

	domain := sqlstore.Domain{
		ID:  "dom-1",
		URL: "example.com",
	}

	// Set up in-memory stores
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()

	// Register the file in siteFiles store
	fileStore.add(sqlstore.SiteFile{
		ID:        "file-1",
		DomainID:  "dom-1",
		Path:      "index.html",
		MimeType:  "text/html",
		SizeBytes: int64(len(originalContent)),
	})

	srv := &Server{
		siteFiles: fileStore,
		fileEdits: editStore,
	}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(filepath.Join(tempDir, "server")))

	ctx := context.Background()
	sessionID := "sess-test-1"

	// Create snapshot
	snapshotTag, err := srv.createAgentSnapshot(ctx, domain, sessionID)
	if err != nil {
		t.Fatalf("createAgentSnapshot: %v", err)
	}
	if snapshotTag != agentSnapshotTag(sessionID) {
		t.Errorf("snapshotTag = %q, want %q", snapshotTag, agentSnapshotTag(sessionID))
	}

	// Simulate agent modifying the file
	modifiedContent := []byte("<html>modified by agent</html>")
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), modifiedContent, 0o644); err != nil {
		t.Fatalf("write modified file: %v", err)
	}

	// Rollback
	restored, err := srv.rollbackAgentSnapshot(ctx, domain, snapshotTag)
	if err != nil {
		t.Fatalf("rollbackAgentSnapshot: %v", err)
	}
	if restored != 1 {
		t.Errorf("restored = %d, want 1", restored)
	}

	// Verify file is restored
	content, err := os.ReadFile(filepath.Join(domainDir, "index.html"))
	if err != nil {
		t.Fatalf("read file after rollback: %v", err)
	}
	if string(content) != string(originalContent) {
		t.Errorf("after rollback: got %q, want %q", string(content), string(originalContent))
	}
}
