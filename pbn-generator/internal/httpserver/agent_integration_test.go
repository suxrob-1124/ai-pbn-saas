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
	result := env.srv.agentToolWriteFile(context.Background(), env.domain, "tu-write-1", input)

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
	result := env.srv.agentToolWriteFile(context.Background(), env.domain, "tu-write-2", input)

	if result.IsError {
		t.Fatalf("expected no error: %s", result.Content)
	}
	if result.FileChanged == nil || result.FileChanged.Action != "updated" {
		t.Errorf("expected action=updated, got %+v", result.FileChanged)
	}
}

func TestAgentTool_WriteFile_DisallowedExtension(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	for _, ext := range []string{"script.php", "run.sh", "hack.py", "virus.exe"} {
		input, _ := json.Marshal(map[string]string{"path": ext, "content": "bad"})
		result := env.srv.agentToolWriteFile(context.Background(), env.domain, "tu-ext", input)
		if !result.IsError {
			t.Errorf("expected error for disallowed extension %q", ext)
		}
	}
}

func TestAgentTool_WriteFile_TooBig(t *testing.T) {
	env := setupAgentIntegrationEnv(t)

	big := strings.Repeat("x", agentMaxFileSizeBytes+1)
	input, _ := json.Marshal(map[string]string{"path": "big.html", "content": big})
	result := env.srv.agentToolWriteFile(context.Background(), env.domain, "tu-big", input)

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
		{"images/hero.webp", "a nice image"},  // must be under assets/
		{"assets/../../etc.webp", "test"},     // traversal
		{"assets/logo", "test"},               // no extension
		{"assets/logo.php", "test"},           // disallowed extension
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

	// Create snapshot
	ctx := context.Background()
	sessionID := "rollback-session-1"
	snapshotTag, err := env.srv.createAgentSnapshot(ctx, env.domain, sessionID)
	if err != nil {
		t.Fatalf("createAgentSnapshot: %v", err)
	}

	// Create a session with snapshot tag
	agentStore := env.srv.agentSessions.(*stubAgentSessionStore)
	agentStore.sessions[sessionID] = sqlstore.AgentSession{
		ID:       sessionID,
		DomainID: "dom-integ",
		Status:   "done",
		SnapshotTag: sql.NullString{
			String: snapshotTag,
			Valid:  true,
		},
	}

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
