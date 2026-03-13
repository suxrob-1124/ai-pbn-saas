package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// ─── matchSingleShot tests ────────────────────────────────────────────────────

func TestMatchSingleShot_RequiresContextFile(t *testing.T) {
	cases := []string{"translate the heading to French", "update the copyright year"}
	for _, msg := range cases {
		if matchSingleShot(msg, "") {
			t.Errorf("should not match when contextFile is empty: %q", msg)
		}
	}
}

func TestMatchSingleShot_BasicSingleFile(t *testing.T) {
	cases := []string{
		"translate the heading to French",
		"add a meta description",
		"fix the typo in the footer",
		"make the button text shorter",
		"update copyright to 2026",
	}
	for _, msg := range cases {
		if !matchSingleShot(msg, "index.html") {
			t.Errorf("should match single-file task: %q", msg)
		}
	}
}

func TestMatchSingleShot_FastPathHasPriority(t *testing.T) {
	// Fast path must be checked before single-shot in the routing layer; here we
	// just verify that matchSingleShot itself does not reject fast-path messages —
	// the routing order in handleDomainAgent is the actual guard.
	// This test exists to document the expected priority behaviour.
	fp, fpOk := matchFastPath("remove html comments", "index.html")
	ss := matchSingleShot("remove html comments", "index.html")
	if !fpOk {
		t.Error("fast path should match 'remove html comments'")
	}
	if fp != nil && !ss {
		// single-shot would also match, but fast path wins via routing order — that is fine.
	}
}

func TestMatchSingleShot_MultiFileBlocked(t *testing.T) {
	cases := []string{
		"remove html comments from all files",
		"update every file",
		"make changes across the site",
		"edit all pages",
	}
	for _, msg := range cases {
		if matchSingleShot(msg, "index.html") {
			t.Errorf("should NOT match multi-file phrase: %q", msg)
		}
	}
}

func TestMatchSingleShot_ResearchTasksBlocked(t *testing.T) {
	cases := []string{
		"search for all broken links",
		"find all instances of the old logo",
		"audit the entire site",
		"scan for SEO issues",
		"analyze page performance",
		"list all images",
	}
	for _, msg := range cases {
		if matchSingleShot(msg, "index.html") {
			t.Errorf("should NOT match research/site-wide task: %q", msg)
		}
	}
}

// ─── validateSingleShotPayload tests ─────────────────────────────────────────

func TestValidateSingleShotPayload_Valid(t *testing.T) {
	p := singleShotPayload{Path: "index.html", Content: "<html></html>", Summary: "Updated heading"}
	if err := validateSingleShotPayload(p, "index.html"); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateSingleShotPayload_WrongPath(t *testing.T) {
	p := singleShotPayload{Path: "other.html", Content: "<html></html>", Summary: "done"}
	if err := validateSingleShotPayload(p, "index.html"); err == nil {
		t.Error("expected path mismatch error")
	}
}

func TestValidateSingleShotPayload_EmptyContent(t *testing.T) {
	p := singleShotPayload{Path: "index.html", Content: "   ", Summary: "done"}
	if err := validateSingleShotPayload(p, "index.html"); err == nil {
		t.Error("expected empty content error")
	}
}

func TestValidateSingleShotPayload_EmptySummary(t *testing.T) {
	p := singleShotPayload{Path: "index.html", Content: "<html></html>", Summary: ""}
	if err := validateSingleShotPayload(p, "index.html"); err == nil {
		t.Error("expected empty summary error")
	}
}

// ─── Routing tests (handler level) ───────────────────────────────────────────

// TestRouting_FastPathBeforeSingleShot verifies that deterministic fast-path
// operations are NOT routed through single-shot even when matchSingleShot would
// also return true. We confirm the session diagnostics show mode=fast_path.
func TestRouting_FastPathBeforeSingleShot(t *testing.T) {
	env := setupSingleShotEnv(t)

	initial := `<html><!-- drop this --><body>hi</body></html>`
	writeEnvFile(t, env, "index.html", initial)
	registerEnvFile(t, env, "file-fp-prio", "index.html")

	body, _ := json.Marshal(map[string]any{
		"message": "remove html comments",
		"context": map[string]any{"current_file": "index.html"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+env.domain.ID+"/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authCtx(req)
	env.srv.handleDomainAgent(httptest.NewRecorder(), req, env.domain.ID)

	store := env.srv.agentSessions.(*stubAgentSessionStore)
	var sess *sqlstore.AgentSession
	for _, s := range store.sessions {
		if s.DomainID == env.domain.ID {
			cp := s
			sess = &cp
			break
		}
	}
	if sess == nil {
		t.Fatal("no session found")
	}
	var diag struct {
		Mode string `json:"mode"`
	}
	_ = json.Unmarshal(sess.DiagnosticsJSON, &diag)
	if diag.Mode != "fast_path" {
		t.Errorf("expected mode=fast_path (fast path takes priority), got %q", diag.Mode)
	}
}

// TestRouting_AmbiguousGoesToAgentLoop verifies that a task with no context file
// is routed to the full agent loop (which requires an API key), not single-shot.
// We verify this by checking there is no single-shot in play (session never starts
// because API key is empty — the handler publishes an error instead).
func TestRouting_NoContextFile_NotSingleShot(t *testing.T) {
	env := setupSingleShotEnv(t)
	env.srv.cfg.AnthropicAPIKey = "" // will trigger the "not configured" path

	body, _ := json.Marshal(map[string]any{
		"message": "make the page look nicer", // no context file
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+env.domain.ID+"/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authCtx(req)
	rec := httptest.NewRecorder()
	env.srv.handleDomainAgent(rec, req, env.domain.ID)

	// Handler should send SSE error (API key check fires, not single-shot).
	if !strings.Contains(rec.Body.String(), "AI agent is not configured") {
		t.Errorf("expected API key error, got: %s", rec.Body.String())
	}
}

// ─── Integration: single-shot actually edits a file ──────────────────────────

// singleShotEnv is an agentIntegrationEnv plus a stubbed Anthropic interceptor.
// Because real Anthropic calls are not available in tests we use a fake HTTP
// server that returns a canned valid edit_file tool use response.
type singleShotEnv struct {
	agentIntegrationEnv
	anthropicServer *httptest.Server
}

func setupSingleShotEnv(t *testing.T) singleShotEnv {
	t.Helper()
	base := setupFastPathEnv(t) // reuse fast-path env (unique domain, registered, no API key set)
	return singleShotEnv{agentIntegrationEnv: base}
}

func writeEnvFile(t *testing.T, env singleShotEnv, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(env.domainDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func registerEnvFile(t *testing.T, env singleShotEnv, id, path string) {
	t.Helper()
	if err := env.fileStore.Create(context.Background(), sqlstore.SiteFile{
		ID: id, DomainID: env.domain.ID, Path: path,
	}); err != nil {
		t.Fatalf("register %s: %v", path, err)
	}
}

// startFakeAnthropic starts an httptest.Server that always returns a valid
// edit_file tool-use response for the given path and content. The returned URL
// should be used as the Anthropic base URL override.
func startFakeAnthropic(t *testing.T, path, content, summary string) *httptest.Server {
	t.Helper()
	toolUseBlock := map[string]any{
		"type":  "tool_use",
		"id":    "tu_123",
		"name":  "edit_file",
		"input": map[string]any{"path": path, "content": content, "summary": summary},
	}
	resp := map[string]any{
		"id":           "msg_test",
		"type":         "message",
		"role":         "assistant",
		"content":      []any{toolUseBlock},
		"model":        "claude-test",
		"stop_reason":  "tool_use",
		"stop_sequence": nil,
		"usage":        map[string]any{"input_tokens": 10, "output_tokens": 20},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestSingleShot_EditFile_E2E verifies the complete single-shot pipeline:
// - file is read server-side
// - model is called exactly once (via fake server)
// - file is updated on disk
// - session completes with mode=single_shot, iterations=1
// - rollback baseline is created
// - messages_json contains user turn + assistant summary
func TestSingleShot_EditFile_E2E(t *testing.T) {
	env := setupSingleShotEnv(t)

	original := `<html><body><h1>Hello</h1></body></html>`
	updated := `<html><body><h1>Bonjour</h1></body></html>`
	summary := "Translated heading to French"

	writeEnvFile(t, env, "index.html", original)
	registerEnvFile(t, env, "file-ss-e2e", "index.html")

	fakeSrv := startFakeAnthropic(t, "index.html", updated, summary)

	// Point the server's Anthropic client at the fake.
	env.srv.cfg.AnthropicAPIKey = "sk-test"
	env.srv.cfg.AnthropicBaseURL = fakeSrv.URL

	body, _ := json.Marshal(map[string]any{
		"message": "translate the heading to French",
		"context": map[string]any{"current_file": "index.html"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+env.domain.ID+"/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authCtx(req)
	rec := httptest.NewRecorder()
	env.srv.handleDomainAgent(rec, req, env.domain.ID)

	// HTTP 200 SSE response.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// File content updated on disk.
	got, _ := os.ReadFile(filepath.Join(env.domainDir, "index.html"))
	if !bytes.Equal(got, []byte(updated)) {
		t.Errorf("file not updated: got %q", got)
	}

	// Session diagnostics: mode=single_shot, iterations=1.
	store := env.srv.agentSessions.(*stubAgentSessionStore)
	var sess *sqlstore.AgentSession
	for _, s := range store.sessions {
		if s.DomainID == env.domain.ID {
			cp := s
			sess = &cp
			break
		}
	}
	if sess == nil {
		t.Fatal("session not found")
	}
	if sess.Status != "done" {
		t.Errorf("status: want done, got %s", sess.Status)
	}
	var diag struct {
		Mode       string `json:"mode"`
		Iterations int    `json:"iterations"`
		FastPath   bool   `json:"fast_path_hit"`
	}
	if err := json.Unmarshal(sess.DiagnosticsJSON, &diag); err != nil {
		t.Fatalf("unmarshal diag: %v", err)
	}
	if diag.Mode != "single_shot" {
		t.Errorf("mode: want single_shot, got %s", diag.Mode)
	}
	if diag.Iterations != 1 {
		t.Errorf("iterations: want 1, got %d", diag.Iterations)
	}
	if diag.FastPath {
		t.Error("fast_path_hit should be false")
	}

	// Rollback baseline created.
	revs, _ := env.editStore.ListRevisionsByFile(context.Background(), "file-ss-e2e", 10)
	if len(revs) == 0 {
		t.Error("no revision created — rollback path broken")
	}
}

// TestSingleShot_MessagesJSON_Continuity verifies that after a successful single-shot
// run the messages_json preserves prior history + user turn + synthetic assistant.
func TestSingleShot_MessagesJSON_Continuity(t *testing.T) {
	env := setupSingleShotEnv(t)

	original := `<html><body><h1>Hello</h1></body></html>`
	updated := `<html><body><h1>Hola</h1></body></html>`
	summary := "Translated heading to Spanish"

	writeEnvFile(t, env, "page.html", original)
	registerEnvFile(t, env, "file-ss-cont", "page.html")

	fakeSrv := startFakeAnthropic(t, "page.html", updated, summary)
	env.srv.cfg.AnthropicAPIKey = "sk-test"
	env.srv.cfg.AnthropicBaseURL = fakeSrv.URL

	// Seed prior LLM history in the session.
	sessionID := "ss-cont-session"
	priorMsgs := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("add a nav bar")),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("Added navigation.")),
	}
	priorMsgsJSON, _ := json.Marshal(priorMsgs)
	priorChatLog := []chatLogEntry{
		{ID: "p1", Role: "user", Text: "add a nav bar"},
		{ID: "p2", Role: "assistant", Status: "done", Text: "Added navigation."},
	}
	priorChatLogJSON, _ := json.Marshal(priorChatLog)
	store := env.srv.agentSessions.(*stubAgentSessionStore)
	store.sessions[sessionID] = sqlstore.AgentSession{
		ID:           sessionID,
		DomainID:     env.domain.ID,
		Status:       "done",
		MessagesJSON: priorMsgsJSON,
		ChatLogJSON:  priorChatLogJSON,
	}

	body, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"message":    "translate the heading to Spanish",
		"context":    map[string]any{"current_file": "page.html"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+env.domain.ID+"/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authCtx(req)
	env.srv.handleDomainAgent(httptest.NewRecorder(), req, env.domain.ID)

	saved := store.sessions[sessionID]
	if len(saved.MessagesJSON) == 0 {
		t.Fatal("messages_json not saved")
	}
	var msgs []anthropic.MessageParam
	if err := json.Unmarshal(saved.MessagesJSON, &msgs); err != nil {
		t.Fatalf("unmarshal messages_json: %v", err)
	}
	// 2 prior + 1 user + 1 synthetic assistant = 4
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages (2 prior + user + assistant), got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Error("prior history not preserved")
	}
	if msgs[2].Role != "user" {
		t.Errorf("[2] want user, got %s", msgs[2].Role)
	}
	if msgs[3].Role != "assistant" {
		t.Errorf("[3] want assistant, got %s", msgs[3].Role)
	}
}

// TestSingleShot_InvalidPath_Fallback verifies that when the model returns a wrong
// path and the repair also fails, the session falls back to the full agent loop.
// We confirm this by checking diagnostics and that the single-shot didn't write the file.
func TestSingleShot_InvalidPath_Fallback(t *testing.T) {
	env := setupSingleShotEnv(t)

	writeEnvFile(t, env, "target.html", `<html><body>original</body></html>`)
	registerEnvFile(t, env, "file-ss-fb", "target.html")

	// Fake Anthropic always returns wrong path → validation fails → repair → fails again.
	fakeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id":   "msg_test",
			"type": "message",
			"role": "assistant",
			"content": []any{map[string]any{
				"type":  "tool_use",
				"id":    "tu_bad",
				"name":  "edit_file",
				"input": map[string]any{"path": "wrong.html", "content": "bad", "summary": "oops"},
			}},
			"model":       "claude-test",
			"stop_reason": "tool_use",
			"usage":       map[string]any{"input_tokens": 5, "output_tokens": 5},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer fakeSrv.Close()

	env.srv.cfg.AnthropicAPIKey = "sk-test"
	env.srv.cfg.AnthropicBaseURL = fakeSrv.URL

	// Inject a fake runAgentLoop by pointing to an Anthropic that returns end_turn immediately.
	// In practice, the agent loop will fail at the API call, which is fine — we just need
	// to confirm that the single-shot fallback path is triggered (diagnostics are saved).
	body, _ := json.Marshal(map[string]any{
		"message": "improve the headline copy",
		"context": map[string]any{"current_file": "target.html"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+env.domain.ID+"/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authCtx(req)
	env.srv.handleDomainAgent(httptest.NewRecorder(), req, env.domain.ID)

	// File must not have been written with wrong-path content.
	got, _ := os.ReadFile(filepath.Join(env.domainDir, "target.html"))
	if !bytes.Equal(got, []byte(`<html><body>original</body></html>`)) {
		t.Errorf("file should be unchanged after failed single-shot, got: %s", got)
	}

	// After fallback, runAgentLoop runs and overwrites diagnostics with mode=agent_loop.
	// Verify the fallback happened by checking the final mode is agent_loop (not single_shot,
	// which would mean single-shot succeeded without fallback). The file-unchanged assertion
	// above already confirms single-shot didn't write anything.
	store := env.srv.agentSessions.(*stubAgentSessionStore)
	for _, s := range store.sessions {
		if s.DomainID == env.domain.ID && len(s.DiagnosticsJSON) > 0 {
			var diag struct {
				Mode string `json:"mode"`
			}
			_ = json.Unmarshal(s.DiagnosticsJSON, &diag)
			if diag.Mode == "agent_loop" {
				return // fallback confirmed: agent loop ran
			}
		}
	}
	t.Error("expected agent_loop diagnostics after single-shot fallback")
}

// ─── Setup helpers (extend setupFastPathEnv for single-shot needs) ────────────

// setupSingleShotEnv reuses the fast-path env; no API key is set by default
// so tests that need it must set env.srv.cfg.AnthropicAPIKey themselves.
func setupSingleShotEnv2(t *testing.T) singleShotEnv {
	t.Helper()
	return setupSingleShotEnv(t)
}

// domainfs import is used indirectly via setupFastPathEnv which calls
// domainfs.NewLocalFSBackend; keep the import used.
var _ = domainfs.NewLocalFSBackend
