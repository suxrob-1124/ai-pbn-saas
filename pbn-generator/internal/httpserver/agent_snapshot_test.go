package httpserver

import (
	"context"
	"database/sql"
	"fmt"
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
	agentStore := newStubAgentSessionStore()

	// Register the file in siteFiles store
	fileStore.add(sqlstore.SiteFile{
		ID:        "file-1",
		DomainID:  "dom-1",
		Path:      "index.html",
		MimeType:  "text/html",
		SizeBytes: int64(len(originalContent)),
		Version:   1,
	})

	// Create agent session first (snapshot needs it for SavePreFileIDs)
	sessionID := "sess-test-1"
	editorEmail := "agent-user@example.com"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{
		ID:       sessionID,
		DomainID: "dom-1",
		Status:   "running",
	})

	srv := &Server{
		siteFiles:     fileStore,
		fileEdits:     editStore,
		agentSessions: agentStore,
	}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(filepath.Join(tempDir, "server")))

	ctx := context.Background()

	// Create snapshot
	snapshotTag, err := srv.createAgentSnapshot(ctx, domain, sessionID, editorEmail)
	if err != nil {
		t.Fatalf("createAgentSnapshot: %v", err)
	}
	if snapshotTag != agentSnapshotTag(sessionID) {
		t.Errorf("snapshotTag = %q, want %q", snapshotTag, agentSnapshotTag(sessionID))
	}

	// Verify pre-existing file IDs were saved
	sess, _ := agentStore.Get(ctx, sessionID)
	if len(sess.PreExistingFileIDs) != 1 || sess.PreExistingFileIDs[0] != "file-1" {
		t.Fatalf("PreExistingFileIDs = %v, want [file-1]", sess.PreExistingFileIDs)
	}

	// Verify NO content revisions were created at snapshot time (lazy baseline).
	revisions, err := editStore.ListRevisionsBySource(ctx, snapshotTag)
	if err != nil {
		t.Fatalf("ListRevisionsBySource: %v", err)
	}
	if len(revisions) != 0 {
		t.Fatalf("expected 0 snapshot revisions at snapshot time (lazy), got %d", len(revisions))
	}

	// Simulate agent about to mutate index.html — ensure baseline is created lazily.
	baselined := make(map[string]bool)
	srv.ensureAgentBaselineForPath(ctx, domain, sessionID, "index.html", editorEmail, baselined)

	// Verify baseline revision now exists.
	revisions, err = editStore.ListRevisionsBySource(ctx, snapshotTag)
	if err != nil {
		t.Fatalf("ListRevisionsBySource after baseline: %v", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("expected 1 baseline revision after ensureAgentBaselineForPath, got %d", len(revisions))
	}
	if revisions[0].EditedBy != editorEmail {
		t.Fatalf("baseline revision edited_by = %q, want %q", revisions[0].EditedBy, editorEmail)
	}

	// Simulate agent modifying the file
	modifiedContent := []byte("<html>modified by agent</html>")
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), modifiedContent, 0o644); err != nil {
		t.Fatalf("write modified file: %v", err)
	}

	// Update session with snapshot tag for rollback
	sess.SnapshotTag = sql.NullString{String: snapshotTag, Valid: true}

	// Rollback
	rb, err := srv.rollbackAgentSnapshot(ctx, domain, sess)
	if err != nil {
		t.Fatalf("rollbackAgentSnapshot: %v", err)
	}
	if rb.RestoredExisting != 1 {
		t.Errorf("RestoredExisting = %d, want 1", rb.RestoredExisting)
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

// TestRollbackDoesNotDeletePreExistingFiles verifies that rollback does not
// delete files that existed before the agent, even if their revision failed.
func TestRollbackDoesNotDeletePreExistingFiles(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(filepath.Join(domainDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write files
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), []byte("<html>home</html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "assets", "logo.png"), []byte("PNG-DATA"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}

	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	agentStore := newStubAgentSessionStore()

	fileStore.add(sqlstore.SiteFile{ID: "file-html", DomainID: "dom-1", Path: "index.html", MimeType: "text/html", Version: 1})
	fileStore.add(sqlstore.SiteFile{ID: "file-png", DomainID: "dom-1", Path: "assets/logo.png", MimeType: "image/png", Version: 1})

	sessionID := "sess-rollback-test"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{ID: sessionID, DomainID: "dom-1", Status: "running"})

	// Simulate: save pre_file_ids with BOTH files, but only HTML gets a revision
	_ = agentStore.SavePreFileIDs(context.Background(), sessionID, []string{"file-html", "file-png"})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore, agentSessions: agentStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	// Only create revision for HTML (simulating PNG revision failure)
	snapshotTag := agentSnapshotTag(sessionID)
	_ = editStore.CreateRevision(context.Background(), sqlstore.FileRevision{
		ID: "rev-1", FileID: "file-html", Version: 1, Content: []byte("<html>home</html>"),
		SizeBytes: 17, MimeType: "text/html", Source: snapshotTag, EditedBy: "test@example.com",
	})

	// Agent creates a new file
	newFilePath := filepath.Join(domainDir, "new-page.html")
	if err := os.WriteFile(newFilePath, []byte("<html>new</html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	fileStore.add(sqlstore.SiteFile{ID: "file-new", DomainID: "dom-1", Path: "new-page.html", MimeType: "text/html", Version: 1})

	sess, _ := agentStore.Get(context.Background(), sessionID)
	sess.SnapshotTag = sql.NullString{String: snapshotTag, Valid: true}

	// Rollback
	_, err := srv.rollbackAgentSnapshot(context.Background(), domain, sess)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// Verify: new-page.html was deleted (created by agent)
	if _, err := os.Stat(newFilePath); !os.IsNotExist(err) {
		t.Error("new-page.html should have been deleted by rollback")
	}

	// Verify: assets/logo.png still exists (pre-existing, no revision)
	if _, err := os.Stat(filepath.Join(domainDir, "assets", "logo.png")); err != nil {
		t.Errorf("assets/logo.png should NOT have been deleted: %v", err)
	}

	// Verify: index.html is restored
	content, _ := os.ReadFile(filepath.Join(domainDir, "index.html"))
	if string(content) != "<html>home</html>" {
		t.Errorf("index.html not restored: got %q", string(content))
	}
}

// TestEnsureAgentBaselineForPath_CreatesRevision verifies that calling ensureAgentBaselineForPath
// for an existing file creates a content revision tagged with the session's snapshot tag.
func TestEnsureAgentBaselineForPath_CreatesRevision(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	originalContent := []byte("<html>original</html>")
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), originalContent, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	agentStore := newStubAgentSessionStore()

	fileStore.add(sqlstore.SiteFile{ID: "file-1", DomainID: "dom-1", Path: "index.html", MimeType: "text/html", Version: 1})

	sessionID := "sess-baseline-1"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{ID: sessionID, DomainID: "dom-1", Status: "running"})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore, agentSessions: agentStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	ctx := context.Background()
	baselined := make(map[string]bool)

	srv.ensureAgentBaselineForPath(ctx, domain, sessionID, "index.html", "test@example.com", baselined)

	revisions, err := editStore.ListRevisionsBySource(ctx, agentSnapshotTag(sessionID))
	if err != nil {
		t.Fatalf("ListRevisionsBySource: %v", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("expected 1 revision, got %d", len(revisions))
	}
	if string(revisions[0].Content) != string(originalContent) {
		t.Errorf("revision content = %q, want %q", revisions[0].Content, originalContent)
	}
}

// TestEnsureAgentBaselineForPath_Idempotent verifies that calling ensureAgentBaselineForPath
// twice for the same file only creates one revision (dedup via in-memory baselined map).
func TestEnsureAgentBaselineForPath_Idempotent(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), []byte("<html>v1</html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	agentStore := newStubAgentSessionStore()

	fileStore.add(sqlstore.SiteFile{ID: "file-1", DomainID: "dom-1", Path: "index.html", MimeType: "text/html", Version: 1})
	sessionID := "sess-idem-1"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{ID: sessionID, DomainID: "dom-1", Status: "running"})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore, agentSessions: agentStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	ctx := context.Background()
	baselined := make(map[string]bool)

	// Call twice
	srv.ensureAgentBaselineForPath(ctx, domain, sessionID, "index.html", "test@example.com", baselined)
	srv.ensureAgentBaselineForPath(ctx, domain, sessionID, "index.html", "test@example.com", baselined)

	revisions, _ := editStore.ListRevisionsBySource(ctx, agentSnapshotTag(sessionID))
	if len(revisions) != 1 {
		t.Errorf("expected exactly 1 revision (idempotent), got %d", len(revisions))
	}
}

// TestRollback_DeleteAndRecreateAtSamePath verifies rollback when the agent deletes
// a pre-existing file and recreates it at the same path within one session.
//
// Expected outcome:
//   - The newly-created file (different ID, same path) is deleted by rollback.
//   - The original content IS restored: rollback recovers the path from
//     rev.Description (set by ensureAgentBaselineForPath) and recreates the
//     SiteFile record under the original file ID.
func TestRollback_DeleteAndRecreateAtSamePath(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	originalContent := []byte("<html>original</html>")
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), originalContent, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	agentStore := newStubAgentSessionStore()

	fileStore.add(sqlstore.SiteFile{ID: "file-orig", DomainID: "dom-1", Path: "index.html", MimeType: "text/html", Version: 1})

	sessionID := "sess-del-recreate"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{ID: sessionID, DomainID: "dom-1", Status: "running"})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore, agentSessions: agentStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	ctx := context.Background()

	// Step 1: snapshot (saves PreExistingFileIDs = ["file-orig"])
	snapshotTag, err := srv.createAgentSnapshot(ctx, domain, sessionID, "user@example.com")
	if err != nil {
		t.Fatalf("createAgentSnapshot: %v", err)
	}
	_ = agentStore.SetSnapshotTag(ctx, sessionID, snapshotTag)

	// Step 2: lazily baseline index.html before the agent deletes it
	baselined := make(map[string]bool)
	srv.ensureAgentBaselineForPath(ctx, domain, sessionID, "index.html", "user@example.com", baselined)

	// Step 3: simulate agent deleting index.html — remove from disk and store
	if err := os.Remove(filepath.Join(domainDir, "index.html")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	_ = fileStore.Delete(ctx, "file-orig")

	// Step 4: simulate agent recreating index.html with a new file record (new ID)
	recreatedContent := []byte("<html>recreated by agent</html>")
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), recreatedContent, 0o644); err != nil {
		t.Fatalf("write recreated: %v", err)
	}
	fileStore.add(sqlstore.SiteFile{ID: "file-new", DomainID: "dom-1", Path: "index.html", MimeType: "text/html", Version: 1})

	// Step 5: rollback
	sess, _ := agentStore.Get(ctx, sessionID)
	rb, err := srv.rollbackAgentSnapshot(ctx, domain, sess)
	if err != nil {
		t.Fatalf("rollbackAgentSnapshot: %v", err)
	}

	// Rollback should delete file-new and restore original via path stored in revision.Description.
	if rb.RestoredExisting != 1 {
		t.Errorf("RestoredExisting = %d, want 1 (path recovered from revision description)", rb.RestoredExisting)
	}

	// Verify: original content is restored to disk.
	content, err := os.ReadFile(filepath.Join(domainDir, "index.html"))
	if err != nil {
		t.Fatalf("read file after rollback: %v", err)
	}
	if string(content) != string(originalContent) {
		t.Errorf("content after rollback = %q, want %q", string(content), string(originalContent))
	}

	// Verify: file-new is removed from store.
	if f, _ := fileStore.Get(ctx, "file-new"); f != nil {
		t.Error("file-new should have been deleted from store by rollback")
	}

	// Verify: original SiteFile record is recreated under its original ID.
	if f, _ := fileStore.Get(ctx, "file-orig"); f == nil {
		t.Error("original SiteFile record (file-orig) should be recreated after rollback")
	}
}

// TestEnsureAgentBaselineForPath_NewFileSkipped verifies that ensureAgentBaselineForPath
// is a no-op for paths that don't exist yet (new files created by the agent).
func TestEnsureAgentBaselineForPath_NewFileSkipped(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	if err := os.MkdirAll(filepath.Join(serverDir, "example.com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}
	fileStore := newMemorySiteFileStore() // empty — file doesn't exist
	editStore := newMemoryFileEditStore()
	agentStore := newStubAgentSessionStore()

	sessionID := "sess-newfile-1"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{ID: sessionID, DomainID: "dom-1", Status: "running"})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore, agentSessions: agentStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	ctx := context.Background()
	baselined := make(map[string]bool)

	// Should not panic or create any revision
	srv.ensureAgentBaselineForPath(ctx, domain, sessionID, "new-page.html", "test@example.com", baselined)

	revisions, _ := editStore.ListRevisionsBySource(ctx, agentSnapshotTag(sessionID))
	if len(revisions) != 0 {
		t.Errorf("expected 0 revisions for new file, got %d", len(revisions))
	}
}

// TestEnsureAgentBaselineForPath_FailureAllowsRetry verifies the mark-after-persist behaviour:
// a transient read failure (file in store but not on disk) does NOT mark the file in
// baselined, so the next mutation attempt will retry the baseline.
// The file is only marked after both the read and CreateRevision succeed.
func TestEnsureAgentBaselineForPath_FailureAllowsRetry(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	// Create domain dir but do NOT write index.html to disk — read will fail.
	if err := os.MkdirAll(filepath.Join(serverDir, "example.com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	agentStore := newStubAgentSessionStore()

	// File exists in store (GetByPath succeeds) but absent from disk (read fails).
	fileStore.add(sqlstore.SiteFile{ID: "file-1", DomainID: "dom-1", Path: "index.html", MimeType: "text/html", Version: 1})

	sessionID := "sess-fail-retry"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{ID: sessionID, DomainID: "dom-1", Status: "running"})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore, agentSessions: agentStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	ctx := context.Background()
	baselined := make(map[string]bool)

	// First call: read fails — file must NOT be marked (mark-after-read).
	srv.ensureAgentBaselineForPath(ctx, domain, sessionID, "index.html", "user@example.com", baselined)

	if baselined["file-1"] {
		t.Error("baselined[file-1] should be false after a failed read (mark-after-read)")
	}
	revisions, _ := editStore.ListRevisionsBySource(ctx, agentSnapshotTag(sessionID))
	if len(revisions) != 0 {
		t.Errorf("expected 0 revisions after failed read, got %d", len(revisions))
	}

	// Second call: file now exists on disk — retry should succeed and create a revision.
	fileContent := []byte("<html>now available</html>")
	if err := os.WriteFile(filepath.Join(serverDir, "example.com", "index.html"), fileContent, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv.ensureAgentBaselineForPath(ctx, domain, sessionID, "index.html", "user@example.com", baselined)

	if !baselined["file-1"] {
		t.Error("baselined[file-1] should be true after successful read")
	}
	revisions, _ = editStore.ListRevisionsBySource(ctx, agentSnapshotTag(sessionID))
	if len(revisions) != 1 {
		t.Errorf("expected 1 revision after successful retry, got %d", len(revisions))
	}
	if string(revisions[0].Content) != string(fileContent) {
		t.Errorf("revision content = %q, want %q", revisions[0].Content, fileContent)
	}

	// Third call: already marked — no duplicate revision.
	srv.ensureAgentBaselineForPath(ctx, domain, sessionID, "index.html", "user@example.com", baselined)
	revisions, _ = editStore.ListRevisionsBySource(ctx, agentSnapshotTag(sessionID))
	if len(revisions) != 1 {
		t.Errorf("expected still 1 revision after third call (no duplicate), got %d", len(revisions))
	}
}

// TestRollback_DeleteOnlySession verifies that when the agent only created new files
// (no existing files were modified), rollback correctly deletes those files and returns
// DeletedCreated > 0, RestoredExisting == 0.
// The endpoint must NOT return "nothing_to_restore" in this case.
func TestRollback_DeleteOnlySession(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Pre-existing file (index.html) — agent did NOT modify it.
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), []byte("<html>home</html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-del-only", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	agentStore := newStubAgentSessionStore()

	fileStore.add(sqlstore.SiteFile{ID: "pre-existing-1", DomainID: "dom-del-only", Path: "index.html", MimeType: "text/html", Version: 1})

	sessionID := "sess-del-only"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{
		ID: sessionID, DomainID: "dom-del-only", Status: "running", CreatedBy: "user@example.com",
	})
	_ = agentStore.SavePreFileIDs(context.Background(), sessionID, []string{"pre-existing-1"})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore, agentSessions: agentStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	// Agent creates two new files (no snapshot baseline for them — agent never mutated existing files).
	if err := os.WriteFile(filepath.Join(domainDir, "new1.html"), []byte("<html>new1</html>"), 0o644); err != nil {
		t.Fatalf("write new1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "new2.html"), []byte("<html>new2</html>"), 0o644); err != nil {
		t.Fatalf("write new2: %v", err)
	}
	fileStore.add(sqlstore.SiteFile{ID: "agent-new-1", DomainID: "dom-del-only", Path: "new1.html", MimeType: "text/html", Version: 1})
	fileStore.add(sqlstore.SiteFile{ID: "agent-new-2", DomainID: "dom-del-only", Path: "new2.html", MimeType: "text/html", Version: 1})

	snapshotTag := agentSnapshotTag(sessionID)
	sess, _ := agentStore.Get(context.Background(), sessionID)
	sess.SnapshotTag = sql.NullString{String: snapshotTag, Valid: true}

	ctx := context.Background()
	rb, err := srv.rollbackAgentSnapshot(ctx, domain, sess)
	if err != nil {
		t.Fatalf("rollbackAgentSnapshot: %v", err)
	}

	// No existing files were modified → RestoredExisting == 0.
	if rb.RestoredExisting != 0 {
		t.Errorf("RestoredExisting = %d, want 0", rb.RestoredExisting)
	}
	// Two agent-created files were deleted → DeletedCreated == 2.
	if rb.DeletedCreated != 2 {
		t.Errorf("DeletedCreated = %d, want 2", rb.DeletedCreated)
	}

	// Agent-created files must be gone from disk.
	if _, err := os.Stat(filepath.Join(domainDir, "new1.html")); !os.IsNotExist(err) {
		t.Error("new1.html should have been deleted by rollback")
	}
	if _, err := os.Stat(filepath.Join(domainDir, "new2.html")); !os.IsNotExist(err) {
		t.Error("new2.html should have been deleted by rollback")
	}

	// Pre-existing file must be untouched.
	if _, err := os.Stat(filepath.Join(domainDir, "index.html")); err != nil {
		t.Errorf("index.html should still exist: %v", err)
	}
}

// TestRollback_MixedSession verifies that when the agent both modified an existing file
// AND created a new one, both counters are correct.
func TestRollback_MixedSession(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	originalContent := []byte("<html>original</html>")
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), originalContent, 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-mixed", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	agentStore := newStubAgentSessionStore()

	fileStore.add(sqlstore.SiteFile{ID: "mix-pre-1", DomainID: "dom-mixed", Path: "index.html", MimeType: "text/html", Version: 1})

	sessionID := "sess-mixed"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{
		ID: sessionID, DomainID: "dom-mixed", Status: "running", CreatedBy: "user@example.com",
	})
	_ = agentStore.SavePreFileIDs(context.Background(), sessionID, []string{"mix-pre-1"})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore, agentSessions: agentStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	ctx := context.Background()
	snapshotTag := agentSnapshotTag(sessionID)

	// Snapshot baseline for index.html.
	_ = editStore.CreateRevision(ctx, sqlstore.FileRevision{
		ID: "mix-snap-1", FileID: "mix-pre-1", Version: 1,
		Content: originalContent, SizeBytes: int64(len(originalContent)),
		MimeType: "text/html", Source: snapshotTag, EditedBy: "user@example.com",
		Description: sql.NullString{String: "index.html", Valid: true},
	})

	// Agent modifies index.html.
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), []byte("<html>modified</html>"), 0o644); err != nil {
		t.Fatalf("write modified: %v", err)
	}

	// Agent creates a brand-new file.
	if err := os.WriteFile(filepath.Join(domainDir, "extra.html"), []byte("<html>extra</html>"), 0o644); err != nil {
		t.Fatalf("write extra: %v", err)
	}
	fileStore.add(sqlstore.SiteFile{ID: "mix-new-1", DomainID: "dom-mixed", Path: "extra.html", MimeType: "text/html", Version: 1})

	sess, _ := agentStore.Get(ctx, sessionID)
	sess.SnapshotTag = sql.NullString{String: snapshotTag, Valid: true}

	rb, err := srv.rollbackAgentSnapshot(ctx, domain, sess)
	if err != nil {
		t.Fatalf("rollbackAgentSnapshot: %v", err)
	}

	if rb.RestoredExisting != 1 {
		t.Errorf("RestoredExisting = %d, want 1", rb.RestoredExisting)
	}
	if rb.DeletedCreated != 1 {
		t.Errorf("DeletedCreated = %d, want 1", rb.DeletedCreated)
	}

	// index.html restored to original.
	content, err := os.ReadFile(filepath.Join(domainDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if string(content) != string(originalContent) {
		t.Errorf("index.html = %q, want %q", content, originalContent)
	}

	// extra.html deleted.
	if _, err := os.Stat(filepath.Join(domainDir, "extra.html")); !os.IsNotExist(err) {
		t.Error("extra.html should have been deleted by rollback")
	}
}

// failOnceFileEditStore wraps memoryFileEditStore and returns an error on the
// first CreateRevision call, then delegates normally.
type failOnceFileEditStore struct {
	*memoryFileEditStore
	failed bool
}

func (f *failOnceFileEditStore) CreateRevision(ctx context.Context, rev sqlstore.FileRevision) error {
	if !f.failed {
		f.failed = true
		return fmt.Errorf("simulated DB error")
	}
	return f.memoryFileEditStore.CreateRevision(ctx, rev)
}

// TestEnsureAgentBaselineForPath_CreateRevisionErrorAllowsRetry verifies that a real
// DB error from CreateRevision (not a UNIQUE conflict) leaves baselined[file.ID]=false,
// so the next mutation attempt can retry baseline creation.
func TestEnsureAgentBaselineForPath_CreateRevisionErrorAllowsRetry(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fileContent := []byte("<html>content</html>")
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), fileContent, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := &failOnceFileEditStore{memoryFileEditStore: newMemoryFileEditStore()}
	agentStore := newStubAgentSessionStore()

	fileStore.add(sqlstore.SiteFile{ID: "file-1", DomainID: "dom-1", Path: "index.html", MimeType: "text/html", Version: 1})

	sessionID := "sess-create-err"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{ID: sessionID, DomainID: "dom-1", Status: "running"})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore, agentSessions: agentStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	ctx := context.Background()
	baselined := make(map[string]bool)

	// First call: read succeeds but CreateRevision returns an error.
	// File must NOT be marked — retry must be allowed.
	srv.ensureAgentBaselineForPath(ctx, domain, sessionID, "index.html", "user@example.com", baselined)

	if baselined["file-1"] {
		t.Error("baselined[file-1] should be false after CreateRevision error (mark-after-persist)")
	}
	revisions, _ := editStore.ListRevisionsBySource(ctx, agentSnapshotTag(sessionID))
	if len(revisions) != 0 {
		t.Errorf("expected 0 revisions after failed CreateRevision, got %d", len(revisions))
	}

	// Second call: CreateRevision now succeeds — baseline is created and file is marked.
	srv.ensureAgentBaselineForPath(ctx, domain, sessionID, "index.html", "user@example.com", baselined)

	if !baselined["file-1"] {
		t.Error("baselined[file-1] should be true after successful retry")
	}
	revisions, _ = editStore.ListRevisionsBySource(ctx, agentSnapshotTag(sessionID))
	if len(revisions) != 1 {
		t.Errorf("expected 1 revision after retry, got %d", len(revisions))
	}
}

// ─── Baseline gate return-value tests ────────────────────────────────────────

// TestEnsureAgentBaselineForPath_ReturnsTrueOnSuccess verifies the new bool
// return: true when baseline is successfully saved for an existing file.
func TestEnsureAgentBaselineForPath_ReturnsTrueOnSuccess(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), []byte("<html>ok</html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	fileStore.add(sqlstore.SiteFile{ID: "file-1", DomainID: "dom-1", Path: "index.html", MimeType: "text/html", Version: 1})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	baselined := make(map[string]bool)
	ok := srv.ensureAgentBaselineForPath(context.Background(), domain, "sess-ok", "index.html", "user@example.com", baselined)
	if !ok {
		t.Error("expected true for successful baseline")
	}
	if !baselined["file-1"] {
		t.Error("expected file-1 in baselined map")
	}
}

// TestEnsureAgentBaselineForPath_ReturnsTrueForNewFile verifies the new bool
// return: true for a file that doesn't exist yet (new file — no baseline needed).
func TestEnsureAgentBaselineForPath_ReturnsTrueForNewFile(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	if err := os.MkdirAll(filepath.Join(serverDir, "example.com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()

	srv := &Server{siteFiles: fileStore, fileEdits: editStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	baselined := make(map[string]bool)
	ok := srv.ensureAgentBaselineForPath(context.Background(), domain, "sess-new", "new-page.html", "user@example.com", baselined)
	if !ok {
		t.Error("expected true for new file (no baseline needed)")
	}
}

// TestEnsureAgentBaselineForPath_ReturnsFalseOnReadFailure verifies the new bool
// return: false when the file exists in store but content cannot be read from backend.
func TestEnsureAgentBaselineForPath_ReturnsFalseOnReadFailure(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	// Create domain dir but do NOT write the file — read will fail.
	if err := os.MkdirAll(filepath.Join(serverDir, "example.com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	fileStore.add(sqlstore.SiteFile{ID: "file-1", DomainID: "dom-1", Path: "index.html", MimeType: "text/html", Version: 1})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	baselined := make(map[string]bool)
	ok := srv.ensureAgentBaselineForPath(context.Background(), domain, "sess-fail", "index.html", "user@example.com", baselined)
	if ok {
		t.Error("expected false when file cannot be read from backend")
	}
	if baselined["file-1"] {
		t.Error("file should not be marked as baselined on read failure")
	}
}

// TestEnsureAgentBaselineForPath_ReturnsTrueOnIdempotentCall verifies that
// a second call returns true (baseline already in map) without re-reading.
func TestEnsureAgentBaselineForPath_ReturnsTrueOnIdempotentCall(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), []byte("<html>v1</html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	fileStore.add(sqlstore.SiteFile{ID: "file-1", DomainID: "dom-1", Path: "index.html", MimeType: "text/html", Version: 1})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	baselined := make(map[string]bool)
	ok1 := srv.ensureAgentBaselineForPath(context.Background(), domain, "sess-idem", "index.html", "user@example.com", baselined)
	if !ok1 {
		t.Fatal("first call should succeed")
	}

	// Remove file from disk — second call should still return true (from map, no re-read)
	_ = os.Remove(filepath.Join(domainDir, "index.html"))
	ok2 := srv.ensureAgentBaselineForPath(context.Background(), domain, "sess-idem", "index.html", "user@example.com", baselined)
	if !ok2 {
		t.Error("second call should return true from baselined map even if file is gone")
	}
}
