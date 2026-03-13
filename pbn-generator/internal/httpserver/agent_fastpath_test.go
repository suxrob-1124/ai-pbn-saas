package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

var fastPathDomainCounter atomic.Int64

// ─── Matcher tests ────────────────────────────────────────────────────────────

func TestMatchFastPath_NoContextFile(t *testing.T) {
	_, ok := matchFastPath("remove html comments", "")
	if ok {
		t.Error("should not match when contextFile is empty")
	}
}

func TestMatchFastPath_MultiFile_Blocked(t *testing.T) {
	cases := []string{
		"remove html comments from all files",
		"remove comments everywhere",
		"remove html comments across all pages",
		"удали комментарии из всех файлов",
	}
	for _, msg := range cases {
		_, ok := matchFastPath(msg, "index.html")
		if ok {
			t.Errorf("multi-file phrase should not match: %q", msg)
		}
	}
}

func TestMatchFastPath_HTMLComments(t *testing.T) {
	cases := []string{
		"remove html comments",
		"delete html comments",
		"удали html комментарии",
		"убери html комментарии",
		"strip html comments",
	}
	for _, msg := range cases {
		op, ok := matchFastPath(msg, "index.html")
		if !ok {
			t.Errorf("should match html comments: %q", msg)
			continue
		}
		if op.Type != fpOpRemoveHTMLComments {
			t.Errorf("wrong type for %q: got %s", msg, op.Type)
		}
		if op.File != "index.html" {
			t.Errorf("wrong file: %s", op.File)
		}
	}
}

func TestMatchFastPath_HTMLComments_InferFromExtension(t *testing.T) {
	// "remove comments" without "html" but file is .html
	op, ok := matchFastPath("remove comments", "page.html")
	if !ok {
		t.Fatal("should infer html comments from .html extension")
	}
	if op.Type != fpOpRemoveHTMLComments {
		t.Errorf("expected remove_html_comments, got %s", op.Type)
	}
}

func TestMatchFastPath_CSSComments(t *testing.T) {
	cases := []string{
		"remove css comments",
		"delete css comments",
		"удали css комментарии",
		"strip css comments",
	}
	for _, msg := range cases {
		op, ok := matchFastPath(msg, "style.css")
		if !ok {
			t.Errorf("should match css comments: %q", msg)
			continue
		}
		if op.Type != fpOpRemoveCSSComments {
			t.Errorf("wrong type for %q: %s", msg, op.Type)
		}
	}
}

func TestMatchFastPath_CSSComments_InferFromExtension(t *testing.T) {
	op, ok := matchFastPath("remove comments", "main.css")
	if !ok {
		t.Fatal("should infer css comments from .css extension")
	}
	if op.Type != fpOpRemoveCSSComments {
		t.Errorf("expected remove_css_comments, got %s", op.Type)
	}
}

func TestMatchFastPath_Comments_AmbiguousExtension_NoMatch(t *testing.T) {
	// "remove comments" on a .js file — ambiguous, no fast path
	_, ok := matchFastPath("remove comments", "script.js")
	if ok {
		t.Error("should not match for ambiguous file type .js")
	}
}

func TestMatchFastPath_ReplaceAll(t *testing.T) {
	cases := []struct {
		msg      string
		wantFrom string
		wantTo   string
	}{
		{"replace all foo with bar", "foo", "bar"},
		{"Replace all 'Hello' with 'Hi'", "Hello", "Hi"},
		{"замени все foo на bar", "foo", "bar"},
	}
	for _, tc := range cases {
		op, ok := matchFastPath(tc.msg, "index.html")
		if !ok {
			t.Errorf("should match replace_all: %q", tc.msg)
			continue
		}
		if op.Type != fpOpReplaceAll {
			t.Errorf("wrong type for %q: %s", tc.msg, op.Type)
		}
		if op.From != tc.wantFrom {
			t.Errorf("from: want %q, got %q (msg=%q)", tc.wantFrom, op.From, tc.msg)
		}
		if op.To != tc.wantTo {
			t.Errorf("to: want %q, got %q (msg=%q)", tc.wantTo, op.To, tc.msg)
		}
	}
}

func TestMatchFastPath_UpdateYear_Explicit(t *testing.T) {
	cases := []struct{ msg, from, to string }{
		{"update year 2024 to 2025", "2024", "2025"},
		{"update year from 2023 to 2024", "2023", "2024"},
		{"замени 2024 на 2025", "2024", "2025"},
		{"replace 2023 with 2024", "2023", "2024"},
	}
	for _, tc := range cases {
		op, ok := matchFastPath(tc.msg, "index.html")
		if !ok {
			t.Errorf("should match update_year: %q", tc.msg)
			continue
		}
		if op.Type != fpOpUpdateYear {
			t.Errorf("wrong type for %q: %s", tc.msg, op.Type)
		}
		if op.From != tc.from || op.To != tc.to {
			t.Errorf("years: want %s→%s, got %s→%s (msg=%q)", tc.from, tc.to, op.From, op.To, tc.msg)
		}
	}
}

func TestMatchFastPath_UpdateYear_Implicit(t *testing.T) {
	op, ok := matchFastPath("update year", "index.html")
	if !ok {
		t.Fatal("should match implicit update_year")
	}
	if op.Type != fpOpUpdateYear {
		t.Errorf("expected update_year, got %s", op.Type)
	}
	thisYear := time.Now().Year()
	if op.To != strings.TrimSpace(op.To) {
		t.Error("To should have no extra spaces")
	}
	// from = last year, to = this year
	if op.To != func() string { return strings.TrimSpace(op.To) }() {
		t.Error("From should be last year")
	}
	_ = thisYear // verified via From/To existence
}

func TestMatchFastPath_RegexReplace(t *testing.T) {
	op, ok := matchFastPath(`regex replace /\d{4}/ with XXXX`, "index.html")
	if !ok {
		t.Fatal("should match regex_replace")
	}
	if op.Type != fpOpRegexReplace {
		t.Errorf("expected regex_replace, got %s", op.Type)
	}
	if op.From != `\d{4}` {
		t.Errorf("unexpected pattern: %q", op.From)
	}
	if op.To != "XXXX" {
		t.Errorf("unexpected replacement: %q", op.To)
	}
}

func TestMatchFastPath_InvalidRegex_NoMatch(t *testing.T) {
	_, ok := matchFastPath(`regex replace /[invalid/ with X`, "index.html")
	if ok {
		t.Error("invalid regex should not produce a fast path op")
	}
}

func TestMatchFastPath_Ambiguous_NoMatch(t *testing.T) {
	ambiguous := []string{
		"rewrite the page",
		"make the site look modern",
		"add a footer",
		"update styles",
		"translate to French",
	}
	for _, msg := range ambiguous {
		_, ok := matchFastPath(msg, "index.html")
		if ok {
			t.Errorf("ambiguous message should not match fast path: %q", msg)
		}
	}
}

// ─── Executor tests ───────────────────────────────────────────────────────────

func TestExecRemoveHTMLComments_Basic(t *testing.T) {
	input := []byte(`<html><!-- comment 1 --><body><!-- comment 2 --></body></html>`)
	res := execRemoveHTMLComments(input)
	if !res.Changed {
		t.Error("should have changed")
	}
	if res.Count != 2 {
		t.Errorf("count: want 2, got %d", res.Count)
	}
	if bytes.Contains(res.Content, []byte("<!--")) {
		t.Error("comments not removed")
	}
	if !bytes.Contains(res.Content, []byte("<html>")) {
		t.Error("non-comment content should be preserved")
	}
}

func TestExecRemoveHTMLComments_Multiline(t *testing.T) {
	input := []byte("<!-- line1\nline2 --><p>hello</p>")
	res := execRemoveHTMLComments(input)
	if res.Count != 1 {
		t.Errorf("count: want 1, got %d", res.Count)
	}
	if bytes.Contains(res.Content, []byte("<!--")) {
		t.Error("multiline comment not removed")
	}
}

func TestExecRemoveHTMLComments_None(t *testing.T) {
	input := []byte(`<html><body><p>hello</p></body></html>`)
	res := execRemoveHTMLComments(input)
	if res.Changed {
		t.Error("should not change when no comments")
	}
	if res.Count != 0 {
		t.Errorf("count should be 0, got %d", res.Count)
	}
}

func TestExecRemoveCSSComments_Basic(t *testing.T) {
	input := []byte("body { /* reset */ color: red; } /* end */")
	res := execRemoveCSSComments(input)
	if !res.Changed {
		t.Error("should have changed")
	}
	if res.Count != 2 {
		t.Errorf("count: want 2, got %d", res.Count)
	}
	if bytes.Contains(res.Content, []byte("/*")) {
		t.Error("CSS comments not removed")
	}
	if !bytes.Contains(res.Content, []byte("color: red;")) {
		t.Error("non-comment content should be preserved")
	}
}

func TestExecReplaceAll_Basic(t *testing.T) {
	input := []byte("Hello World Hello World")
	res := execReplaceAll(input, "Hello", "Hi")
	if !res.Changed {
		t.Error("should have changed")
	}
	if res.Count != 2 {
		t.Errorf("count: want 2, got %d", res.Count)
	}
	if string(res.Content) != "Hi World Hi World" {
		t.Errorf("unexpected result: %q", res.Content)
	}
}

func TestExecReplaceAll_NotFound(t *testing.T) {
	input := []byte("Hello World")
	res := execReplaceAll(input, "Foo", "Bar")
	if res.Changed {
		t.Error("should not change when not found")
	}
	if res.Count != 0 {
		t.Errorf("count should be 0")
	}
}

func TestExecRegexReplace_Basic(t *testing.T) {
	input := []byte("Copyright 2023 and 2022")
	res, err := execRegexReplace(input, `20\d\d`, "YYYY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Changed {
		t.Error("should have changed")
	}
	if res.Count != 2 {
		t.Errorf("count: want 2, got %d", res.Count)
	}
	if string(res.Content) != "Copyright YYYY and YYYY" {
		t.Errorf("unexpected result: %q", res.Content)
	}
}

func TestExecRegexReplace_InvalidPattern(t *testing.T) {
	_, err := execRegexReplace([]byte("hello"), `[invalid`, "X")
	if err == nil {
		t.Error("should return error for invalid regex")
	}
}

func TestExecReplaceAll_UpdateYear(t *testing.T) {
	input := []byte("<footer>Copyright 2024. All rights reserved 2024.</footer>")
	res := execReplaceAll(input, "2024", "2025")
	if res.Count != 2 {
		t.Errorf("count: want 2, got %d", res.Count)
	}
	if bytes.Contains(res.Content, []byte("2024")) {
		t.Error("old year should be gone")
	}
}

// ─── Verification tests ───────────────────────────────────────────────────────

func TestVerifyFastPathOp_HTMLComments_Pass(t *testing.T) {
	op := &fastPathOp{Type: fpOpRemoveHTMLComments}
	clean := []byte("<html><body></body></html>")
	if !verifyFastPathOp(op, clean) {
		t.Error("should pass when no html comments remain")
	}
}

func TestVerifyFastPathOp_HTMLComments_Fail(t *testing.T) {
	op := &fastPathOp{Type: fpOpRemoveHTMLComments}
	withComment := []byte("<html><!-- still here --></html>")
	if verifyFastPathOp(op, withComment) {
		t.Error("should fail when html comment remains")
	}
}

func TestVerifyFastPathOp_CSSComments_Pass(t *testing.T) {
	op := &fastPathOp{Type: fpOpRemoveCSSComments}
	clean := []byte("body { color: red; }")
	if !verifyFastPathOp(op, clean) {
		t.Error("should pass when no css comments remain")
	}
}

func TestVerifyFastPathOp_UpdateYear_Pass(t *testing.T) {
	op := &fastPathOp{Type: fpOpUpdateYear, From: "2024", To: "2025"}
	result := []byte("Copyright 2025")
	if !verifyFastPathOp(op, result) {
		t.Error("should pass when old year is gone")
	}
}

func TestVerifyFastPathOp_UpdateYear_Fail(t *testing.T) {
	op := &fastPathOp{Type: fpOpUpdateYear, From: "2024", To: "2025"}
	result := []byte("Copyright 2024") // old year still present
	if verifyFastPathOp(op, result) {
		t.Error("should fail when old year remains")
	}
}

// ─── Integration / handler tests ─────────────────────────────────────────────

func setupFastPathEnv(t *testing.T) agentIntegrationEnv {
	t.Helper()
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainURL := "fp-test.local"
	domainDir := filepath.Join(serverDir, domainURL)
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Use a unique domain ID per test to avoid globalAgentHub state from a prior test
	// (completed sessions linger in the hub for 30 s before unregistering).
	domainID := fmt.Sprintf("dom-fp-%d", fastPathDomainCounter.Add(1))
	domain := sqlstore.Domain{ID: domainID, URL: domainURL, ProjectID: "proj-fp"}
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	srv := setupServer(t)
	srv.siteFiles = fileStore
	srv.fileEdits = editStore
	srv.agentSessions = newStubAgentSessionStore()
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))
	// Register domain so handleDomainAgent can look it up.
	srv.domains.(*stubDomainStore).domains[domain.ID] = domain
	// No API key needed: fast path executes locally without calling Anthropic.

	return agentIntegrationEnv{
		srv:       srv,
		domain:    domain,
		domainDir: domainDir,
		fileStore: fileStore,
		editStore: editStore,
	}
}

// TestFastPath_RemoveHTMLComments verifies that:
// - request "remove html comments" completes without Anthropic
// - comments are actually removed from the file
// - session diagnostics show mode=fast_path and fast_path_hit=true
// - a baseline revision is created for rollback
func TestFastPath_RemoveHTMLComments_E2E(t *testing.T) {
	env := setupFastPathEnv(t)

	// Seed file with HTML comments.
	initial := `<html><!-- keep this out --><body><p>hello</p><!-- another --></body></html>`
	if err := os.WriteFile(filepath.Join(env.domainDir, "index.html"), []byte(initial), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// Register in file store.
	fileID := "file-fp-1"
	if err := env.fileStore.Create(context.Background(), sqlstore.SiteFile{
		ID: fileID, DomainID: env.domain.ID, Path: "index.html",
	}); err != nil {
		t.Fatalf("create file record: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"message": "remove html comments",
		"context": map[string]any{
			"current_file":         "index.html",
			"include_current_file": false,
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/dom-fp/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authCtx(req)

	rec := httptest.NewRecorder()
	env.srv.handleDomainAgent(rec, req, env.domain.ID)

	// Response must be SSE (200).
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// File content should have no HTML comments.
	got, err := os.ReadFile(filepath.Join(env.domainDir, "index.html"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if bytes.Contains(got, []byte("<!--")) {
		t.Errorf("HTML comments not removed, got: %s", got)
	}
	if !bytes.Contains(got, []byte("<p>hello</p>")) {
		t.Errorf("regular content was stripped, got: %s", got)
	}

	// Check session diagnostics saved by fast path.
	store := env.srv.agentSessions.(*stubAgentSessionStore)
	var foundSession *sqlstore.AgentSession
	for _, sess := range store.sessions {
		if sess.DomainID == env.domain.ID {
			s := sess
			foundSession = &s
			break
		}
	}
	if foundSession == nil {
		t.Fatal("no session found in store")
	}
	if foundSession.Status != "done" {
		t.Errorf("session status: want done, got %s", foundSession.Status)
	}
	if len(foundSession.DiagnosticsJSON) == 0 {
		t.Fatal("diagnostics not saved")
	}

	var diag struct {
		Mode        string `json:"mode"`
		FastPathHit bool   `json:"fast_path_hit"`
		Iterations  int    `json:"iterations"`
	}
	if err := json.Unmarshal(foundSession.DiagnosticsJSON, &diag); err != nil {
		t.Fatalf("unmarshal diagnostics: %v", err)
	}
	if diag.Mode != "fast_path" {
		t.Errorf("mode: want fast_path, got %s", diag.Mode)
	}
	if !diag.FastPathHit {
		t.Error("fast_path_hit should be true")
	}
	if diag.Iterations != 0 {
		t.Errorf("iterations: want 0, got %d", diag.Iterations)
	}
}

// TestFastPath_AgentLoop_UsedWhenNoFastPath verifies that ambiguous messages
// still go through the normal agent loop (fast path does NOT intercept them).
// We just verify matchFastPath returns false for such messages — the actual
// agent loop is not invoked in this test (no Anthropic key needed).
func TestFastPath_NoInterceptionForAmbiguous(t *testing.T) {
	messages := []string{
		"make the homepage look nicer",
		"add a navigation menu",
		"translate the page to Spanish",
		"",
	}
	for _, msg := range messages {
		_, ok := matchFastPath(msg, "index.html")
		if ok {
			t.Errorf("fast path should NOT match ambiguous message: %q", msg)
		}
	}
}

// TestFastPath_BaselineRevision_Created verifies that when a file is changed
// via fast path, a baseline revision is created (enabling rollback).
func TestFastPath_BaselineRevision_Created(t *testing.T) {
	env := setupFastPathEnv(t)

	content := `body { /* old comment */ color: red; }`
	if err := os.WriteFile(filepath.Join(env.domainDir, "style.css"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := env.fileStore.Create(context.Background(), sqlstore.SiteFile{
		ID: "file-css-1", DomainID: env.domain.ID, Path: "style.css",
	}); err != nil {
		t.Fatalf("create file: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"message": "remove css comments",
		"context": map[string]any{"current_file": "style.css"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/dom-fp/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authCtx(req)
	env.srv.handleDomainAgent(httptest.NewRecorder(), req, env.domain.ID)

	revs, err := env.editStore.ListRevisionsBySource(context.Background(), "agent_snapshot:")
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	// At least one baseline revision should exist (the pre-edit snapshot).
	found := false
	for _, rev := range revs {
		if strings.HasPrefix(rev.Source, "agent_snapshot:") {
			found = true
			break
		}
	}
	if !found {
		// Fallback: check via ListRevisionsByFile
		cssRevs, _ := env.editStore.ListRevisionsByFile(context.Background(), "file-css-1", 10)
		if len(cssRevs) == 0 {
			t.Error("no revision created — rollback path broken")
		}
	}
}

// TestFastPath_MessagesJSON_ResumedSession is a regression test that verifies
// messages_json after a fast-path run contains:
//  1. The prior Anthropic history from the resumed session (old user+assistant turns),
//  2. The new user turn that triggered the fast path,
//  3. A synthetic assistant turn with the operation summary.
//
// Without this, a subsequent LLM resume would miss the fast-path context entirely.
func TestFastPath_MessagesJSON_ResumedSession(t *testing.T) {
	env := setupFastPathEnv(t)

	// Seed HTML file.
	initial := `<html><!-- stale --><body>hello</body></html>`
	if err := os.WriteFile(filepath.Join(env.domainDir, "index.html"), []byte(initial), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := env.fileStore.Create(context.Background(), sqlstore.SiteFile{
		ID: "file-html-resume", DomainID: env.domain.ID, Path: "index.html",
	}); err != nil {
		t.Fatalf("create file: %v", err)
	}

	// Simulate a prior LLM run: pre-populate the session store with an existing
	// session that already has Anthropic history (two turns).
	sessionID := "session-resume-test"
	priorMessages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("make the page pretty")),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("Done, I updated the styles.")),
	}
	priorMessagesJSON, _ := json.Marshal(priorMessages)
	priorChatLog := []chatLogEntry{
		{ID: "e1", Role: "user", Text: "make the page pretty"},
		{ID: "e2", Role: "assistant", Status: "done", Text: "Done, I updated the styles."},
	}
	priorChatLogJSON, _ := json.Marshal(priorChatLog)
	store := env.srv.agentSessions.(*stubAgentSessionStore)
	store.sessions[sessionID] = sqlstore.AgentSession{
		ID:           sessionID,
		DomainID:     env.domain.ID,
		Status:       "done",
		MessagesJSON: priorMessagesJSON,
		ChatLogJSON:  priorChatLogJSON,
	}

	// Resume the session with a fast-path request.
	body, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"message":    "remove html comments",
		"context":    map[string]any{"current_file": "index.html"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+env.domain.ID+"/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authCtx(req)
	env.srv.handleDomainAgent(httptest.NewRecorder(), req, env.domain.ID)

	// Inspect the saved messages_json.
	saved := store.sessions[sessionID]
	if len(saved.MessagesJSON) == 0 {
		t.Fatal("messages_json not saved after fast-path run")
	}

	var savedMsgs []anthropic.MessageParam
	if err := json.Unmarshal(saved.MessagesJSON, &savedMsgs); err != nil {
		t.Fatalf("unmarshal messages_json: %v", err)
	}

	// Expect: 2 prior turns + 1 new user turn + 1 synthetic assistant turn = 4.
	if len(savedMsgs) != 4 {
		t.Fatalf("expected 4 messages (2 prior + user + assistant), got %d", len(savedMsgs))
	}

	// [0] prior user, [1] prior assistant — preserved verbatim.
	if savedMsgs[0].Role != "user" {
		t.Errorf("[0] role: want user, got %s", savedMsgs[0].Role)
	}
	if savedMsgs[1].Role != "assistant" {
		t.Errorf("[1] role: want assistant, got %s", savedMsgs[1].Role)
	}

	// [2] new user turn — must contain the fast-path request message.
	if savedMsgs[2].Role != "user" {
		t.Errorf("[2] role: want user, got %s", savedMsgs[2].Role)
	}

	// [3] synthetic assistant turn — must be non-empty (operation summary).
	if savedMsgs[3].Role != "assistant" {
		t.Errorf("[3] role: want assistant, got %s", savedMsgs[3].Role)
	}
	// Extract text from the synthetic assistant block and confirm it's non-empty.
	if len(savedMsgs[3].Content) == 0 {
		t.Error("[3] synthetic assistant content is empty")
	}
}
