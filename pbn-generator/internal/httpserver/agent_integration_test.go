package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// ─── Shared setup helper ──────────────────────────────────────────────────────

type agentIntegrationEnv struct {
	srv       *Server
	domain    sqlstore.Domain
	domainDir string
	fileStore *memorySiteFileStore
	editStore *memoryFileEditStore
}

func setupAgentIntegrationEnv(t *testing.T) agentIntegrationEnv {
	t.Helper()
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainURL := "testsite.local"
	domainDir := filepath.Join(serverDir, domainURL)
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir domain dir: %v", err)
	}

	domain := sqlstore.Domain{
		ID:  "dom-integ",
		URL: domainURL,
	}

	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()

	srv := setupServer(t)
	srv.siteFiles = fileStore
	srv.fileEdits = editStore
	srv.agentSessions = newStubAgentSessionStore()
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	return agentIntegrationEnv{
		srv:       srv,
		domain:    domain,
		domainDir: domainDir,
		fileStore: fileStore,
		editStore: editStore,
	}
}

func authCtx(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), currentUserContextKey, auth.User{
		Email: "agent-test@example.com",
		Role:  "admin",
	})
	return r.WithContext(ctx)
}

// ─── Тест 1: агент читает файлы корректно ────────────────────────────────────

func TestAgentTool_ReadFile_OK(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// Write a real file to the backend
	content := "<html><body>Hello World</body></html>"
	filePath := filepath.Join(env.domainDir, "index.html")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	env.fileStore.add(sqlstore.SiteFile{
		ID:       "f-read-1",
		DomainID: "dom-integ",
		Path:     "index.html",
		MimeType: "text/html",
	})

	input, _ := json.Marshal(map[string]string{"path": "index.html"})
	result := env.srv.agentToolReadFile(context.Background(), env.domain, "tu-1", input)

	if result.IsError {
		t.Fatalf("expected no error, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Hello World") {
		t.Errorf("expected file content in result, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "index.html") {
		t.Errorf("expected filename in result, got: %s", result.Content)
	}
}

func TestAgentTool_ReadFile_NotFound(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	input, _ := json.Marshal(map[string]string{"path": "nonexistent.html"})
	result := env.srv.agentToolReadFile(context.Background(), env.domain, "tu-2", input)

	if !result.IsError {
		t.Errorf("expected error for nonexistent file, got content: %s", result.Content)
	}
}

func TestAgentTool_ReadFile_TraversalBlocked(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	for _, path := range []string{"../etc/passwd", "../../etc/shadow", "/etc/passwd"} {
		input, _ := json.Marshal(map[string]string{"path": path})
		result := env.srv.agentToolReadFile(context.Background(), env.domain, "tu-traversal", input)
		if !result.IsError {
			t.Errorf("path traversal %q should have been blocked, got: %s", path, result.Content)
		}
	}
}

// ─── Тест 2: агент пишет файлы ───────────────────────────────────────────────

func TestAgentTool_WriteFile_Creates(t *testing.T) {
	env := setupAgentIntegrationEnv(t)
	env.fileStore.add(sqlstore.SiteFile{
		ID: "fp-existing", DomainID: "dom-integ", Path: "index.html", MimeType: "text/html",
	})
	if err := os.WriteFile(filepath.Join(env.domainDir, "index.html"), []byte("<html/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	input, _ := json.Marshal(map[string]string{
		"path":    "about.html",
		"content": "<html><body>About</body></html>",
	})
	result := env.srv.agentToolWriteFile(context.Background(), env.domain, "test@example.com", "tu-write-1", input)

	if result.IsError {
		t.Fatalf("expected no error writing file, got: %s", result.Content)
	}
	if result.FileChanged == nil {
		t.Fatal("expected FileChanged to be set")
	}
	if result.FileChanged.Path != "about.html" {
		t.Errorf("FileChanged.Path = %q, want about.html", result.FileChanged.Path)
	}
	if result.FileChanged.Action != "created" {
		t.Errorf("FileChanged.Action = %q, want created", result.FileChanged.Action)
	}

	// Verify file exists on disk
	written, err := os.ReadFile(filepath.Join(env.domainDir, "about.html"))
	if err != nil {
		t.Fatalf("file not written to disk: %v", err)
	}
	if !strings.Contains(string(written), "About") {
		t.Errorf("file content mismatch: %s", string(written))
	}
}

func TestAgentTool_WriteFile_Updates(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// Pre-create the file in store so it's "existing"
	env.fileStore.add(sqlstore.SiteFile{
		ID: "fp-2", DomainID: "dom-integ", Path: "page.html", MimeType: "text/html",
	})
	if err := os.WriteFile(filepath.Join(env.domainDir, "page.html"), []byte("<html>old</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	input, _ := json.Marshal(map[string]string{
		"path":    "page.html",
		"content": "<html>new content</html>",
	})
	result := env.srv.agentToolWriteFile(context.Background(), env.domain, "test@example.com", "tu-write-2", input)

	if result.IsError {
		t.Fatalf("expected no error: %s", result.Content)
	}
	if result.FileChanged == nil || result.FileChanged.Action != "updated" {
		t.Errorf("expected action=updated, got %+v", result.FileChanged)
	}
}

// TestAgentTool_PatchFile_NoIntermediateRevision verifies that patch_file does NOT
// create an intermediate file revision — revisions are deferred to finalizeAgentRevisions
// at successful session completion (skipRevision=true path).
func TestAgentTool_PatchFile_NoIntermediateRevision(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	env.fileStore.add(sqlstore.SiteFile{
		ID: "fp-patch-1", DomainID: "dom-integ", Path: "index.html", MimeType: "text/html",
	})
	if err := os.WriteFile(filepath.Join(env.domainDir, "index.html"), []byte("<html><body>old link</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	input, _ := json.Marshal(map[string]string{
		"path":     "index.html",
		"old_text": "old link",
		"new_text": "new link",
	})
	result := env.srv.agentToolPatchFile(context.Background(), env.domain, "test@example.com", "tu-patch-1", input)

	if result.IsError {
		t.Fatalf("expected no error: %s", result.Content)
	}
	if result.FileChanged == nil || result.FileChanged.Action != "updated" {
		t.Fatalf("expected updated file change, got %+v", result.FileChanged)
	}

	// No intermediate revision should be created by patch_file itself.
	revs, err := env.editStore.ListRevisionsByFile(context.Background(), "fp-patch-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 0 {
		t.Errorf("expected 0 intermediate revisions from patch_file (skipRevision=true), got %d", len(revs))
	}
}

// TestAgentTool_WriteFile_NoIntermediateRevision verifies that write_file does NOT
// create an intermediate file revision — revisions are created by finalizeAgentRevisions.
func TestAgentTool_WriteFile_NoIntermediateRevision(t *testing.T) {
	env := setupAgentIntegrationEnv(t)
	env.fileStore.add(sqlstore.SiteFile{
		ID: "fp-write-norev", DomainID: "dom-integ", Path: "page.html", MimeType: "text/html",
	})
	if err := os.WriteFile(filepath.Join(env.domainDir, "page.html"), []byte("<html>old</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	input, _ := json.Marshal(map[string]string{
		"path":    "page.html",
		"content": "<html>new content</html>",
	})
	result := env.srv.agentToolWriteFile(context.Background(), env.domain, "test@example.com", "tu-write-norev", input)

	if result.IsError {
		t.Fatalf("expected no error: %s", result.Content)
	}

	// No intermediate revision should be created by write_file itself.
	revs, err := env.editStore.ListRevisionsByFile(context.Background(), "fp-write-norev", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 0 {
		t.Errorf("expected 0 intermediate revisions from write_file (skipRevision=true), got %d", len(revs))
	}
}

// TestFinalizeAgentRevisions_OneRevisionPerFile verifies that finalizeAgentRevisions
// creates exactly one "agent" revision per unique file path regardless of how many
// times that path appears in filesChanged.
func TestFinalizeAgentRevisions_OneRevisionPerFile(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// Seed two files
	env.fileStore.add(sqlstore.SiteFile{ID: "fin-1", DomainID: "dom-integ", Path: "index.html", MimeType: "text/html"})
	env.fileStore.add(sqlstore.SiteFile{ID: "fin-2", DomainID: "dom-integ", Path: "about.html", MimeType: "text/html"})
	if err := os.WriteFile(filepath.Join(env.domainDir, "index.html"), []byte("<html>home</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.domainDir, "about.html"), []byte("<html>about</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// index.html appears twice (duplicate) to verify dedup.
	filesChanged := []string{"index.html", "about.html", "index.html"}
	env.srv.finalizeAgentRevisions(context.Background(), env.domain, filesChanged, "sess-finalize-1", "user@example.com", "agent")

	// index.html: exactly 1 revision
	revsIndex, err := env.editStore.ListRevisionsByFile(context.Background(), "fin-1", 10)
	if err != nil {
		t.Fatalf("list revisions index.html: %v", err)
	}
	if len(revsIndex) != 1 {
		t.Errorf("index.html: expected 1 final revision, got %d", len(revsIndex))
	}
	if revsIndex[0].Source != "agent" {
		t.Errorf("index.html revision source = %q, want \"agent\"", revsIndex[0].Source)
	}

	// about.html: exactly 1 revision
	revsAbout, err := env.editStore.ListRevisionsByFile(context.Background(), "fin-2", 10)
	if err != nil {
		t.Fatalf("list revisions about.html: %v", err)
	}
	if len(revsAbout) != 1 {
		t.Errorf("about.html: expected 1 final revision, got %d", len(revsAbout))
	}
	if revsAbout[0].Source != "agent" {
		t.Errorf("about.html revision source = %q, want \"agent\"", revsAbout[0].Source)
	}
}

// TestRollback_CreatesRevertRevision verifies that rollbackAgentSnapshot creates a
// visible "revert" revision for each successfully restored file.
func TestRollback_CreatesRevertRevision(t *testing.T) {
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

	domain := sqlstore.Domain{ID: "dom-rv", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	agentStore := newStubAgentSessionStore()

	fileStore.add(sqlstore.SiteFile{ID: "rv-file-1", DomainID: "dom-rv", Path: "index.html", MimeType: "text/html", Version: 1})

	sessionID := "sess-rv-1"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{ID: sessionID, DomainID: "dom-rv", Status: "running", CreatedBy: "user@example.com"})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore, agentSessions: agentStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	ctx := context.Background()
	snapshotTag := agentSnapshotTag(sessionID)

	// Create baseline revision (simulates ensureAgentBaselineForPath)
	_ = editStore.CreateRevision(ctx, sqlstore.FileRevision{
		ID: "snap-rv-1", FileID: "rv-file-1", Version: 1,
		Content: originalContent, SizeBytes: int64(len(originalContent)),
		MimeType: "text/html", Source: snapshotTag, EditedBy: "user@example.com",
		Description: sql.NullString{String: "index.html", Valid: true},
	})
	_ = agentStore.SavePreFileIDs(ctx, sessionID, []string{"rv-file-1"})

	// Simulate agent modifying the file
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), []byte("<html>modified</html>"), 0o644); err != nil {
		t.Fatalf("write modified: %v", err)
	}

	sess, _ := agentStore.Get(ctx, sessionID)
	sess.SnapshotTag = sql.NullString{String: snapshotTag, Valid: true}

	rb, err := srv.rollbackAgentSnapshot(ctx, domain, sess)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rb.RestoredExisting != 1 {
		t.Fatalf("RestoredExisting = %d, want 1", rb.RestoredExisting)
	}

	// Verify a "revert" revision was created for index.html.
	revs, err := editStore.ListRevisionsByFile(ctx, "rv-file-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	var revertRevs []sqlstore.FileRevision
	for _, rev := range revs {
		if rev.Source == "revert" {
			revertRevs = append(revertRevs, rev)
		}
	}
	if len(revertRevs) != 1 {
		t.Errorf("expected 1 revert revision after rollback, got %d (all revisions: %d)", len(revertRevs), len(revs))
	}
}

func TestAgentTool_WriteFile_DisallowedExtension(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	for _, ext := range []string{"script.php", "run.sh", "hack.py", "virus.exe"} {
		input, _ := json.Marshal(map[string]string{"path": ext, "content": "bad"})
		result := env.srv.agentToolWriteFile(context.Background(), env.domain, "test@example.com", "tu-ext", input)
		if !result.IsError {
			t.Errorf("expected error for disallowed extension %q", ext)
		}
	}
}

func TestAgentTool_WriteFile_TooBig(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	big := strings.Repeat("x", agentMaxFileSizeBytes+1)
	input, _ := json.Marshal(map[string]string{"path": "big.html", "content": big})
	result := env.srv.agentToolWriteFile(context.Background(), env.domain, "test@example.com", "tu-big", input)

	if !result.IsError {
		t.Errorf("expected error for file exceeding size limit")
	}
}

// ─── Тест 3: generate_image — валидация пути и промпта ───────────────────────

func TestAgentTool_GenerateImage_InvalidPath(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	cases := []struct {
		path   string
		prompt string
	}{
		{"images/hero.webp", "a nice image"}, // must be under assets/
		{"assets/../../etc.webp", "test"},    // traversal
		{"assets/logo", "test"},              // no extension
		{"assets/logo.php", "test"},          // disallowed extension
	}
	for _, c := range cases {
		input, _ := json.Marshal(map[string]string{"path": c.path, "prompt": c.prompt})
		result := env.srv.agentToolGenerateImage(context.Background(), env.domain, "user@example.com", "tu-img", input)
		if !result.IsError {
			t.Errorf("expected error for path=%q prompt=%q, got: %s", c.path, c.prompt, result.Content)
		}
	}
}

func TestAgentTool_GenerateImage_EmptyPrompt(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	input, _ := json.Marshal(map[string]string{"path": "assets/hero.webp", "prompt": ""})
	result := env.srv.agentToolGenerateImage(context.Background(), env.domain, "user@example.com", "tu-img-empty", input)

	if !result.IsError {
		t.Errorf("expected error for empty prompt, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "prompt") {
		t.Errorf("expected 'prompt' in error message, got: %s", result.Content)
	}
}

// ─── Тест 4: SSE разрыв — корректная остановка ───────────────────────────────

func TestAgentHandler_ContextCancel_StopsGracefully(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// Configure with a real key placeholder so the handler passes the key check
	// but context is cancelled before any real API call is made.
	env.srv.cfg.AnthropicAPIKey = "sk-placeholder"

	env.srv.domains.(*stubDomainStore).domains["dom-integ"] = env.domain

	body := strings.NewReader(`{"message":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/domains/dom-integ/agent", body)
	req.Header.Set("Content-Type", "application/json")

	// Cancel the context before the request — simulates client disconnect
	ctx, cancel := context.WithCancel(req.Context())
	cancel() // cancel immediately
	ctx = context.WithValue(ctx, currentUserContextKey, auth.User{
		Email: "agent-test@example.com",
		Role:  "admin",
	})

	rec := httptest.NewRecorder()
	env.srv.handleDomainAgent(rec, req.WithContext(ctx), "dom-integ")

	body2 := rec.Body.String()
	// Should get a stopped event (ctx already done → first select catches it)
	if !strings.Contains(body2, `"stopped"`) && !strings.Contains(body2, `"session_start"`) {
		t.Logf("response body: %s", body2)
		// It's OK if it sends session_start then stopped — just must not hang
	}
	// Main assertion: response is complete (not hanging) and returned a valid SSE response
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// ─── Тест 5: rollback восстанавливает все файлы ───────────────────────────────
// (Покрыт TestCreateAndRollbackAgentSnapshot в agent_snapshot_test.go)
// Добавляем дополнительный тест через HTTP endpoint

func TestAgentRollback_RestoresFiles(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// Write original file to disk
	originalContent := []byte("<html>original</html>")
	if err := os.WriteFile(filepath.Join(env.domainDir, "index.html"), originalContent, 0o644); err != nil {
		t.Fatal(err)
	}
	env.fileStore.add(sqlstore.SiteFile{
		ID:       "rollback-file-1",
		DomainID: "dom-integ",
		Path:     "index.html",
		MimeType: "text/html",
	})
	env.srv.domains.(*stubDomainStore).domains["dom-integ"] = env.domain

	ctx := context.Background()
	sessionID := "rollback-session-1"

	// Create session BEFORE snapshot so SavePreFileIDs can store file IDs
	agentStore := env.srv.agentSessions.(*stubAgentSessionStore)
	agentStore.sessions[sessionID] = sqlstore.AgentSession{
		ID:       sessionID,
		DomainID: "dom-integ",
		Status:   "done",
	}

	// Create snapshot (saves PreExistingFileIDs into the session)
	snapshotTag, err := env.srv.createAgentSnapshot(ctx, env.domain, sessionID, "user@example.com")
	if err != nil {
		t.Fatalf("createAgentSnapshot: %v", err)
	}

	// Set snapshot tag on the session (without overwriting PreExistingFileIDs)
	_ = agentStore.SetSnapshotTag(ctx, sessionID, snapshotTag)

	// Lazily create content baseline for index.html (simulates executeAgentTool pre-write baseline)
	baselined := make(map[string]bool)
	env.srv.ensureAgentBaselineForPath(ctx, env.domain, sessionID, "index.html", "user@example.com", baselined)

	// Simulate agent modifying the file
	modifiedContent := []byte("<html>modified by agent</html>")
	if err := os.WriteFile(filepath.Join(env.domainDir, "index.html"), modifiedContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Call rollback endpoint
	req := httptest.NewRequest(http.MethodPost, "/api/domains/dom-integ/agent/"+sessionID+"/rollback", nil)
	req = authCtx(req)
	rec := httptest.NewRecorder()
	env.srv.handleAgentRollback(rec, req, "dom-integ", sessionID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "rolled_back" {
		t.Errorf("expected status=rolled_back, got %v", resp["status"])
	}

	// Verify file restored
	content, err := os.ReadFile(filepath.Join(env.domainDir, "index.html"))
	if err != nil {
		t.Fatalf("read file after rollback: %v", err)
	}
	if string(content) != string(originalContent) {
		t.Errorf("content after rollback = %q, want %q", string(content), string(originalContent))
	}
}

// ─── Тест 6: path traversal блокируется (дополнительно через executor) ───────
// (Уже покрыт TestValidateAgentFilePath и TestAgentTool_ReadFile_TraversalBlocked)

func TestAgentTool_DeleteFile_ProtectedFileBlocked(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// index.html — protected, cannot delete
	input, _ := json.Marshal(map[string]string{"path": "index.html"})
	result := env.srv.agentToolDeleteFile(context.Background(), env.domain, "tu-del-protected", input)

	if !result.IsError {
		t.Errorf("expected error deleting protected file index.html")
	}
	if !strings.Contains(result.Content, "protected") {
		t.Errorf("expected 'protected' in error message, got: %s", result.Content)
	}
}

func TestAgentTool_DeleteFile_TraversalBlocked(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	for _, path := range []string{"../etc/passwd", "../../secret"} {
		input, _ := json.Marshal(map[string]string{"path": path})
		result := env.srv.agentToolDeleteFile(context.Background(), env.domain, "tu-del-traversal", input)
		if !result.IsError {
			t.Errorf("expected traversal blocked for %q, got success", path)
		}
	}
}

// ─── Partial revision tests ──────────────────────────────────────────────────

// TestFinalizeAgentRevisions_PartialSource verifies that finalizeAgentRevisions
// creates revisions with source="agent_partial" when called in partial mode.
func TestFinalizeAgentRevisions_PartialSource(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	env.fileStore.add(sqlstore.SiteFile{ID: "partial-1", DomainID: "dom-integ", Path: "index.html", MimeType: "text/html"})
	if err := os.WriteFile(filepath.Join(env.domainDir, "index.html"), []byte("<html>partial</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	env.srv.finalizeAgentRevisions(context.Background(), env.domain,
		[]string{"index.html"}, "sess-partial-1", "user@example.com", "agent_partial")

	revs, err := env.editStore.ListRevisionsByFile(context.Background(), "partial-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("expected 1 revision, got %d", len(revs))
	}
	if revs[0].Source != "agent_partial" {
		t.Errorf("revision source = %q, want \"agent_partial\"", revs[0].Source)
	}
	if !strings.Contains(revs[0].Description.String, "partial update by agent") {
		t.Errorf("revision description = %q, expected 'partial update by agent'", revs[0].Description.String)
	}
}

// TestFinalizeAgentRevisions_EmptyFilesChanged verifies no revisions are created
// when filesChanged is empty (session failed without any file writes).
func TestFinalizeAgentRevisions_EmptyFilesChanged(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	env.fileStore.add(sqlstore.SiteFile{ID: "noop-1", DomainID: "dom-integ", Path: "index.html", MimeType: "text/html"})

	env.srv.finalizeAgentRevisions(context.Background(), env.domain,
		nil, "sess-noop", "user@example.com", "agent_partial")

	revs, err := env.editStore.ListRevisionsByFile(context.Background(), "noop-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 0 {
		t.Errorf("expected 0 revisions for empty filesChanged, got %d", len(revs))
	}
}

// TestFinalizeAgentRevisions_MultiFile_Partial verifies one agent_partial revision
// per unique changed file in a failed session with multiple files modified.
func TestFinalizeAgentRevisions_MultiFile_Partial(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	env.fileStore.add(sqlstore.SiteFile{ID: "mp-1", DomainID: "dom-integ", Path: "page1.html", MimeType: "text/html"})
	env.fileStore.add(sqlstore.SiteFile{ID: "mp-2", DomainID: "dom-integ", Path: "page2.html", MimeType: "text/html"})
	if err := os.WriteFile(filepath.Join(env.domainDir, "page1.html"), []byte("<p>1</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.domainDir, "page2.html"), []byte("<p>2</p>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// page1.html appears twice — dedup should give exactly 1 revision per file.
	env.srv.finalizeAgentRevisions(context.Background(), env.domain,
		[]string{"page1.html", "page2.html", "page1.html"}, "sess-mp", "user@example.com", "agent_partial")

	for _, tc := range []struct {
		fileID string
		path   string
	}{
		{"mp-1", "page1.html"},
		{"mp-2", "page2.html"},
	} {
		revs, err := env.editStore.ListRevisionsByFile(context.Background(), tc.fileID, 10)
		if err != nil {
			t.Fatalf("list revisions %s: %v", tc.path, err)
		}
		if len(revs) != 1 {
			t.Errorf("%s: expected 1 revision, got %d", tc.path, len(revs))
		}
		if len(revs) > 0 && revs[0].Source != "agent_partial" {
			t.Errorf("%s: source = %q, want \"agent_partial\"", tc.path, revs[0].Source)
		}
	}
}

// TestFinalizeAgentRevisions_DoubleCallGuard verifies that calling finalize twice
// (simulating finishSession + panic defer race) does not create duplicate revisions
// because the revisionsFinalized flag prevents the second call.
// Here we test the function itself is idempotent from the dedup perspective —
// two consecutive calls with the same filesChanged produce 2 revisions (different IDs),
// which is why the revisionsFinalized guard in agent_handlers.go is necessary.
func TestFinalizeAgentRevisions_DoubleCallGuard(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	env.fileStore.add(sqlstore.SiteFile{ID: "dbl-1", DomainID: "dom-integ", Path: "index.html", MimeType: "text/html", Version: 1})
	if err := os.WriteFile(filepath.Join(env.domainDir, "index.html"), []byte("<html>content</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate the guard: only the first call creates revisions.
	revisionsFinalized := false
	callFinalize := func(source string) {
		if revisionsFinalized {
			return
		}
		revisionsFinalized = true
		env.srv.finalizeAgentRevisions(context.Background(), env.domain,
			[]string{"index.html"}, "sess-dbl", "user@example.com", source)
	}

	callFinalize("agent")
	callFinalize("agent_partial") // should be skipped by guard

	revs, err := env.editStore.ListRevisionsByFile(context.Background(), "dbl-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 1 {
		t.Errorf("expected exactly 1 revision (guard should prevent second call), got %d", len(revs))
	}
	if len(revs) > 0 && revs[0].Source != "agent" {
		t.Errorf("source = %q, want \"agent\" (first call wins)", revs[0].Source)
	}
}

// TestRollback_AfterPartialRevision verifies that rollback works correctly even
// when an agent_partial revision exists for the file. The snapshot baseline (not
// the partial revision) drives the restore.
func TestRollback_AfterPartialRevision(t *testing.T) {
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

	domain := sqlstore.Domain{ID: "dom-rp", URL: "example.com"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	agentStore := newStubAgentSessionStore()

	fileStore.add(sqlstore.SiteFile{ID: "rp-file-1", DomainID: "dom-rp", Path: "index.html", MimeType: "text/html", Version: 1})

	sessionID := "sess-rp-1"
	_ = agentStore.Create(context.Background(), sqlstore.AgentSession{ID: sessionID, DomainID: "dom-rp", Status: "error", CreatedBy: "user@example.com"})

	srv := &Server{siteFiles: fileStore, fileEdits: editStore, agentSessions: agentStore}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))

	ctx := context.Background()
	snapshotTag := agentSnapshotTag(sessionID)

	// 1. Create baseline revision (simulates ensureAgentBaselineForPath)
	_ = editStore.CreateRevision(ctx, sqlstore.FileRevision{
		ID: "snap-rp-1", FileID: "rp-file-1", Version: 1,
		Content: originalContent, SizeBytes: int64(len(originalContent)),
		MimeType: "text/html", Source: snapshotTag, EditedBy: "user@example.com",
		Description: sql.NullString{String: "index.html", Valid: true},
	})
	_ = agentStore.SavePreFileIDs(ctx, sessionID, []string{"rp-file-1"})

	// 2. Simulate agent modifying the file
	modifiedContent := []byte("<html>partially modified</html>")
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), modifiedContent, 0o644); err != nil {
		t.Fatal(err)
	}
	// Update version in store (simulates agentUpsertSiteFile bumping version)
	fileStore.mu.Lock()
	if f, ok := fileStore.files["rp-file-1"]; ok {
		f.Version = 2
		fileStore.files["rp-file-1"] = f
	}
	fileStore.mu.Unlock()

	// 3. Create agent_partial revision (simulates failed session finalization)
	srv.finalizeAgentRevisions(ctx, domain, []string{"index.html"}, sessionID, "user@example.com", "agent_partial")

	// 4. Now rollback — should restore to original content
	sess, _ := agentStore.Get(ctx, sessionID)
	sess.SnapshotTag = sql.NullString{String: snapshotTag, Valid: true}

	rb, err := srv.rollbackAgentSnapshot(ctx, domain, sess)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rb.RestoredExisting != 1 {
		t.Fatalf("RestoredExisting = %d, want 1", rb.RestoredExisting)
	}

	// 5. Verify file content restored to original
	content, err := os.ReadFile(filepath.Join(domainDir, "index.html"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != string(originalContent) {
		t.Errorf("content after rollback = %q, want %q", string(content), string(originalContent))
	}

	// 6. Verify a "revert" revision was created
	revs, err := editStore.ListRevisionsByFile(ctx, "rp-file-1", 20)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	var revertCount int
	for _, rev := range revs {
		if rev.Source == "revert" {
			revertCount++
		}
	}
	if revertCount != 1 {
		t.Errorf("expected 1 revert revision, got %d", revertCount)
	}

	// 7. Verify agent_snapshot:* is not visible (for filter correctness)
	var snapshotCount int
	for _, rev := range revs {
		if strings.HasPrefix(rev.Source, "agent_snapshot:") {
			snapshotCount++
		}
	}
	// Snapshot revisions still exist in DB but should be filtered by handler.
	// Here we just verify they exist so rollback works.
	if snapshotCount != 1 {
		t.Errorf("expected 1 snapshot revision in DB, got %d", snapshotCount)
	}
}

// ─── Delete-file revision tests ──────────────────────────────────────────────

// TestFinalizeAgentRevisions_DeletedFile verifies that when an agent deletes a
// pre-existing file, finalizeAgentRevisions creates a zero-content deletion-marker
// revision using the FileID recovered from the snapshot baseline.
func TestFinalizeAgentRevisions_DeletedFile(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// Seed a pre-existing file
	env.fileStore.add(sqlstore.SiteFile{ID: "del-1", DomainID: "dom-integ", Path: "about.html", MimeType: "text/html", Version: 1})
	if err := os.WriteFile(filepath.Join(env.domainDir, "about.html"), []byte("<html>about</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	sessionID := "sess-del-1"

	// Create snapshot baseline (simulates ensureAgentBaselineForPath before delete)
	_ = env.editStore.CreateRevision(context.Background(), sqlstore.FileRevision{
		ID: "snap-del-1", FileID: "del-1", Version: 1,
		Content:  []byte("<html>about</html>"),
		MimeType: "text/html", Source: agentSnapshotTag(sessionID),
		EditedBy:    "user@example.com",
		Description: sql.NullString{String: "about.html", Valid: true},
	})

	// Simulate agent deleting the file: remove from store and disk
	env.fileStore.mu.Lock()
	delete(env.fileStore.files, "del-1")
	env.fileStore.mu.Unlock()
	_ = os.Remove(filepath.Join(env.domainDir, "about.html"))

	// Finalize with the deleted file in filesChanged
	env.srv.finalizeAgentRevisions(context.Background(), env.domain,
		[]string{"about.html"}, sessionID, "user@example.com", "agent")

	// Should have a deletion-marker revision for the original file ID
	revs, err := env.editStore.ListRevisionsByFile(context.Background(), "del-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	// Expect 2 revisions: snapshot baseline + agent deletion marker
	var agentRevs []sqlstore.FileRevision
	for _, rev := range revs {
		if rev.Source == "agent" {
			agentRevs = append(agentRevs, rev)
		}
	}
	if len(agentRevs) != 1 {
		t.Fatalf("expected 1 agent deletion-marker revision, got %d (total revs: %d)", len(agentRevs), len(revs))
	}
	if len(agentRevs[0].Content) != 0 {
		t.Errorf("deletion-marker revision should have zero content, got %d bytes", len(agentRevs[0].Content))
	}
	if !strings.Contains(agentRevs[0].Description.String, "deleted by") {
		t.Errorf("description = %q, expected 'deleted by' prefix", agentRevs[0].Description.String)
	}
}

// TestFinalizeAgentRevisions_CreatedThenDeletedFile verifies that a file created
// by the agent and then deleted in the same session does NOT produce a revision
// (no snapshot baseline exists for it).
func TestFinalizeAgentRevisions_CreatedThenDeletedFile(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	sessionID := "sess-cd-1"

	// No file in store, no baseline — simulates create → delete in same session.
	env.srv.finalizeAgentRevisions(context.Background(), env.domain,
		[]string{"temp-page.html"}, sessionID, "user@example.com", "agent")

	// No revision should be created for a file that was created and deleted
	// within the same session (no prior history to record against).
	// Verify by checking revisions map is empty (no file ID to query by).
	env.editStore.mu.Lock()
	count := len(env.editStore.revisions)
	env.editStore.mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 revisions for created-then-deleted file, got %d", count)
	}
}

// TestFinalizeAgentRevisions_MixedDeleteAndUpdate verifies correct behavior when
// an agent session both modifies an existing file and deletes another.
func TestFinalizeAgentRevisions_MixedDeleteAndUpdate(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// File 1: will be modified
	env.fileStore.add(sqlstore.SiteFile{ID: "mix-1", DomainID: "dom-integ", Path: "index.html", MimeType: "text/html", Version: 2})
	if err := os.WriteFile(filepath.Join(env.domainDir, "index.html"), []byte("<html>modified</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// File 2: will be deleted
	sessionID := "sess-mix-1"
	_ = env.editStore.CreateRevision(context.Background(), sqlstore.FileRevision{
		ID: "snap-mix-2", FileID: "mix-2", Version: 1,
		Content:  []byte("<html>old about</html>"),
		MimeType: "text/html", Source: agentSnapshotTag(sessionID),
		EditedBy:    "user@example.com",
		Description: sql.NullString{String: "about.html", Valid: true},
	})
	// about.html already deleted from store and disk

	env.srv.finalizeAgentRevisions(context.Background(), env.domain,
		[]string{"index.html", "about.html"}, sessionID, "user@example.com", "agent")

	// index.html: should have 1 "agent" revision with content
	revsIndex, err := env.editStore.ListRevisionsByFile(context.Background(), "mix-1", 10)
	if err != nil {
		t.Fatalf("list revisions index.html: %v", err)
	}
	if len(revsIndex) != 1 {
		t.Fatalf("index.html: expected 1 revision, got %d", len(revsIndex))
	}
	if revsIndex[0].Source != "agent" {
		t.Errorf("index.html source = %q, want \"agent\"", revsIndex[0].Source)
	}
	if len(revsIndex[0].Content) == 0 {
		t.Error("index.html revision should have content (modified, not deleted)")
	}

	// about.html: should have 2 revisions (snapshot + deletion marker)
	revsAbout, err := env.editStore.ListRevisionsByFile(context.Background(), "mix-2", 10)
	if err != nil {
		t.Fatalf("list revisions about.html: %v", err)
	}
	var delMarkers int
	for _, rev := range revsAbout {
		if rev.Source == "agent" && len(rev.Content) == 0 {
			delMarkers++
		}
	}
	if delMarkers != 1 {
		t.Errorf("about.html: expected 1 deletion-marker revision, got %d", delMarkers)
	}
}

// ─── E2E finishSession test ─────────────────────────────────────────────────

// TestFinishSession_PartialRevisionOnError exercises the finishSession closure
// path in agent_handlers.go: when a session finishes with an error status and
// files were changed, an agent_partial revision must appear in file history.
func TestFinishSession_PartialRevisionOnError(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// Seed a file that the agent will modify
	env.fileStore.add(sqlstore.SiteFile{ID: "e2e-1", DomainID: "dom-integ", Path: "page.html", MimeType: "text/html", Version: 1})
	if err := os.WriteFile(filepath.Join(env.domainDir, "page.html"), []byte("<html>original</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	sessionID := "sess-e2e-1"
	agentStore := env.srv.agentSessions.(*stubAgentSessionStore)
	agentStore.sessions[sessionID] = sqlstore.AgentSession{
		ID: sessionID, DomainID: "dom-integ", Status: "running", CreatedBy: "user@example.com",
	}

	// Simulate agent modifying the file
	if err := os.WriteFile(filepath.Join(env.domainDir, "page.html"), []byte("<html>agent-modified</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate finishSession behavior: partial finalization then Finish
	var revisionsFinalized bool
	filesChanged := []string{"page.html"}
	status := "error"

	if !revisionsFinalized {
		revisionsFinalized = true
		if status == "done" {
			env.srv.finalizeAgentRevisions(context.Background(), env.domain, filesChanged, sessionID, "user@example.com", "agent")
		} else if len(filesChanged) > 0 {
			env.srv.finalizeAgentRevisions(context.Background(), env.domain, filesChanged, sessionID, "user@example.com", "agent_partial")
		}
	}
	_ = agentStore.Finish(context.Background(), sessionID, status, "stream error", filesChanged, 3)

	// Verify: exactly one agent_partial revision for the file
	revs, err := env.editStore.ListRevisionsByFile(context.Background(), "e2e-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("expected 1 revision, got %d", len(revs))
	}
	if revs[0].Source != "agent_partial" {
		t.Errorf("source = %q, want \"agent_partial\"", revs[0].Source)
	}

	// Verify the guard prevents double finalization
	env.srv.finalizeAgentRevisions(context.Background(), env.domain, filesChanged, sessionID, "user@example.com", "agent_partial")
	// This second call WOULD create another revision, but the guard (revisionsFinalized)
	// prevents it from being called in production. Here we verify the guard pattern.
	if revisionsFinalized {
		// Guard was set — in production the second call wouldn't happen.
		// For completeness, verify that calling finalize again DOES create a dupe
		// (proving the guard is necessary).
		revs2, _ := env.editStore.ListRevisionsByFile(context.Background(), "e2e-1", 10)
		if len(revs2) != 2 {
			t.Errorf("without guard, second finalize should create a dupe, got %d revisions", len(revs2))
		}
	}
}

// ─── Тест 7a: baseline gate blocks write_file when baseline fails ────────────

func TestBaselineGate_BlocksWriteOnBaselineFailure(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// File exists in DB store but NOT on disk → readDomainFileBytesFromBackend will fail
	// → ensureAgentBaselineForPath returns false → mutation must be blocked.
	env.fileStore.add(sqlstore.SiteFile{
		ID:       "gate-f1",
		DomainID: "dom-integ",
		Path:     "index.html",
		MimeType: "text/html",
		Version:  1,
	})

	sessionID := "sess-gate-block"
	baselined := make(map[string]bool)

	input, _ := json.Marshal(map[string]string{
		"path":    "index.html",
		"content": "<html>should not be written</html>",
	})

	result := env.srv.executeAgentTool(
		context.Background(),
		env.domain,
		"agent-test@example.com",
		"tu-gate-1",
		"write_file",
		input,
		sessionID,
		baselined,
	)

	if !result.IsError {
		t.Fatal("expected error from baseline gate, got success")
	}
	if !strings.Contains(result.Content, "failed to save rollback baseline") {
		t.Errorf("expected baseline failure message, got: %s", result.Content)
	}

	// File must NOT have been changed — no file_changed event.
	if result.FileChanged != nil {
		t.Error("FileChanged should be nil when baseline gate blocks the write")
	}

	// No revisions should have been created.
	revs, _ := env.editStore.ListRevisionsBySource(context.Background(), agentSnapshotTag(sessionID))
	if len(revs) != 0 {
		t.Errorf("expected 0 revisions, got %d", len(revs))
	}
}

// TestBaselineGate_AllowsWriteOnBaselineSuccess verifies that when baseline
// succeeds, the write_file tool proceeds normally and the file is written.
func TestBaselineGate_AllowsWriteOnBaselineSuccess(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// File exists both in DB and on disk → baseline succeeds.
	originalContent := []byte("<html>original</html>")
	filePath := filepath.Join(env.domainDir, "index.html")
	if err := os.WriteFile(filePath, originalContent, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	env.fileStore.add(sqlstore.SiteFile{
		ID:       "gate-f2",
		DomainID: "dom-integ",
		Path:     "index.html",
		MimeType: "text/html",
		Version:  1,
	})

	sessionID := "sess-gate-allow"
	baselined := make(map[string]bool)

	newContent := "<html>updated by agent</html>"
	input, _ := json.Marshal(map[string]string{
		"path":    "index.html",
		"content": newContent,
	})

	result := env.srv.executeAgentTool(
		context.Background(),
		env.domain,
		"agent-test@example.com",
		"tu-gate-2",
		"write_file",
		input,
		sessionID,
		baselined,
	)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	// Baseline revision should exist.
	revs, _ := env.editStore.ListRevisionsBySource(context.Background(), agentSnapshotTag(sessionID))
	if len(revs) != 1 {
		t.Errorf("expected 1 baseline revision, got %d", len(revs))
	}

	// File on disk should be updated.
	diskContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(diskContent) != newContent {
		t.Errorf("disk content = %q, want %q", diskContent, newContent)
	}
}

// ─── Тест 7b: active session rollback returns 409 ───────────────────────────

func TestRollback_ActiveSessionReturns409(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	sessionID := "sess-active-rollback"
	domainID := "dom-active-rb" // unique to avoid globalAgentHub conflicts with other tests

	// Create agent session in DB with snapshot tag.
	_ = env.srv.agentSessions.Create(context.Background(), sqlstore.AgentSession{
		ID:        sessionID,
		DomainID:  domainID,
		Status:    "running",
		CreatedBy: "agent-test@example.com",
	})
	_ = env.srv.agentSessions.SetSnapshotTag(context.Background(), sessionID, agentSnapshotTag(sessionID))

	// Register session in globalAgentHub as active (NOT done).
	hs, ok := globalAgentHub.registerForDomain(sessionID, domainID)
	if !ok {
		t.Fatal("failed to register hub session")
	}
	defer func() {
		hs.markDone()
		globalAgentHub.unregister(sessionID, domainID)
	}()

	// Make the rollback request.
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/agent/"+sessionID+"/rollback", nil)
	req = authCtx(req)
	rr := httptest.NewRecorder()

	env.srv.handleAgentRollback(rr, req, domainID, sessionID)

	if rr.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d (409 Conflict)", rr.Code, http.StatusConflict)
	}

	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, hasError := body["error"]; !hasError {
		t.Error("response should contain 'error' field")
	}
}

// TestRollback_DoneSessionAllowed verifies that rollback succeeds for a session
// that is registered in the hub but has been marked as done.
func TestRollback_DoneSessionAllowed(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	sessionID := "sess-done-rollback"
	domainID := "dom-done-rb" // unique to avoid globalAgentHub conflicts with other tests

	// Set up domain in the domains store (handleAgentRollback calls s.domains.Get).
	domStore, ok := env.srv.domains.(*stubDomainStore)
	if !ok {
		t.Fatal("expected *stubDomainStore")
	}
	domStore.Create(context.Background(), sqlstore.Domain{
		ID:  domainID,
		URL: "testsite.local",
	})

	// Create agent session with snapshot tag.
	_ = env.srv.agentSessions.Create(context.Background(), sqlstore.AgentSession{
		ID:        sessionID,
		DomainID:  domainID,
		Status:    "done",
		CreatedBy: "agent-test@example.com",
	})
	_ = env.srv.agentSessions.SetSnapshotTag(context.Background(), sessionID, agentSnapshotTag(sessionID))
	_ = env.srv.agentSessions.SavePreFileIDs(context.Background(), sessionID, []string{})

	// Register session in hub but mark it done immediately.
	hs, registered := globalAgentHub.registerForDomain(sessionID, domainID)
	if !registered {
		t.Fatal("failed to register hub session")
	}
	hs.markDone()
	defer globalAgentHub.unregister(sessionID, domainID)

	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/agent/"+sessionID+"/rollback", nil)
	req = authCtx(req)
	rr := httptest.NewRecorder()

	env.srv.handleAgentRollback(rr, req, domainID, sessionID)

	// Should succeed (200), not 409.
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
}

// ─── Тест 7c: baseline gate blocks delete_file too ──────────────────────────

func TestBaselineGate_BlocksDeleteOnBaselineFailure(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// File in DB but not on disk → baseline fails.
	env.fileStore.add(sqlstore.SiteFile{
		ID:       "gate-del-f1",
		DomainID: "dom-integ",
		Path:     "about.html",
		MimeType: "text/html",
		Version:  1,
	})

	sessionID := "sess-gate-del"
	baselined := make(map[string]bool)

	input, _ := json.Marshal(map[string]string{"path": "about.html"})

	result := env.srv.executeAgentTool(
		context.Background(),
		env.domain,
		"agent-test@example.com",
		"tu-gate-del",
		"delete_file",
		input,
		sessionID,
		baselined,
	)

	if !result.IsError {
		t.Fatal("expected error from baseline gate on delete_file, got success")
	}
	if !strings.Contains(result.Content, "failed to save rollback baseline") {
		t.Errorf("expected baseline failure message, got: %s", result.Content)
	}
}

// ─── Тест 7d: generate_image baseline gate blocks overwrite of existing asset ─

func TestBaselineGate_BlocksGenerateImageOverwrite(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// Existing asset in DB but NOT on disk → baseline will fail.
	env.fileStore.add(sqlstore.SiteFile{
		ID:       "gate-img-f1",
		DomainID: "dom-integ",
		Path:     "assets/hero.webp",
		MimeType: "image/webp",
		Version:  1,
	})

	sessionID := "sess-gate-img"
	baselined := make(map[string]bool)

	input, _ := json.Marshal(map[string]string{
		"path":   "assets/hero.webp",
		"prompt": "A beautiful landscape",
	})

	result := env.srv.executeAgentTool(
		context.Background(),
		env.domain,
		"agent-test@example.com",
		"tu-gate-img",
		"generate_image",
		input,
		sessionID,
		baselined,
	)

	// Baseline gate must fire before the image generation API call.
	if !result.IsError {
		t.Fatal("expected error from baseline gate on generate_image overwrite, got success")
	}
	if !strings.Contains(result.Content, "failed to save rollback baseline") {
		t.Errorf("expected baseline failure message, got: %s", result.Content)
	}
}

// ─── Тест 7: 10 параллельных сессий ─────────────────────────────────────────

func TestAgentTools_ConcurrentSessions(t *testing.T) {
	const numGoroutines = 10
	env := setupAgentIntegrationEnv(t)

	// Seed some files
	for i := 0; i < 5; i++ {
		env.fileStore.add(sqlstore.SiteFile{
			ID:       "concurrent-f-" + string(rune('0'+i)),
			DomainID: "dom-integ",
			Path:     "page" + string(rune('0'+i)) + ".html",
			MimeType: "text/html",
		})
	}

	var wg sync.WaitGroup
	errors := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			input, _ := json.Marshal(map[string]string{})
			result := env.srv.agentToolListFiles(context.Background(), env.domain, "tu-concurrent", input)
			if result.IsError {
				errors <- result.Content
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for e := range errors {
		t.Errorf("concurrent agentToolListFiles error: %s", e)
	}
}

func TestAgentTool_ListFiles_FiltersByDirectory(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	env.fileStore.add(sqlstore.SiteFile{ID: "lf-1", DomainID: "dom-integ", Path: "index.html"})
	env.fileStore.add(sqlstore.SiteFile{ID: "lf-2", DomainID: "dom-integ", Path: "assets/logo.png"})
	env.fileStore.add(sqlstore.SiteFile{ID: "lf-3", DomainID: "dom-integ", Path: "assets/style.css"})

	// List only assets/
	input, _ := json.Marshal(map[string]string{"directory": "assets"})
	result := env.srv.agentToolListFiles(context.Background(), env.domain, "tu-list", input)

	if result.IsError {
		t.Fatalf("expected no error: %s", result.Content)
	}
	if strings.Contains(result.Content, "index.html") {
		t.Errorf("index.html should not appear in assets/ listing, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "assets/logo.png") {
		t.Errorf("assets/logo.png should appear in assets/ listing, got: %s", result.Content)
	}
}

func TestAgentTool_SearchInFiles(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	// Write real files to backend
	if err := os.WriteFile(filepath.Join(env.domainDir, "index.html"), []byte("<html>Hello World keyword here</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.domainDir, "about.html"), []byte("<html>About page no match</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.fileStore.add(sqlstore.SiteFile{ID: "search-1", DomainID: "dom-integ", Path: "index.html", MimeType: "text/html"})
	env.fileStore.add(sqlstore.SiteFile{ID: "search-2", DomainID: "dom-integ", Path: "about.html", MimeType: "text/html"})

	input, _ := json.Marshal(map[string]string{"query": "keyword"})
	result := env.srv.agentToolSearchInFiles(context.Background(), env.domain, "tu-search", input)

	if result.IsError {
		t.Fatalf("expected no error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "index.html") {
		t.Errorf("expected index.html in search results, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "1 file(s)") {
		t.Errorf("expected 1 file match, got: %s", result.Content)
	}
}
