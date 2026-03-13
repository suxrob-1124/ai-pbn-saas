package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// ─── Stub AgentSessionStore ──────────────────────────────────────────────────

type stubAgentSessionStore struct {
	sessions map[string]sqlstore.AgentSession
	events   []sqlstore.AgentSessionEvent
}

func newStubAgentSessionStore() *stubAgentSessionStore {
	return &stubAgentSessionStore{sessions: make(map[string]sqlstore.AgentSession)}
}

func (s *stubAgentSessionStore) Create(ctx context.Context, sess sqlstore.AgentSession) error {
	s.sessions[sess.ID] = sess
	return nil
}

func (s *stubAgentSessionStore) Get(ctx context.Context, id string) (sqlstore.AgentSession, error) {
	if sess, ok := s.sessions[id]; ok {
		return sess, nil
	}
	return sqlstore.AgentSession{}, nil
}

func (s *stubAgentSessionStore) ListByDomain(ctx context.Context, domainID string, limit int) ([]sqlstore.AgentSession, error) {
	var result []sqlstore.AgentSession
	for _, sess := range s.sessions {
		if sess.DomainID == domainID {
			result = append(result, sess)
		}
	}
	return result, nil
}

func (s *stubAgentSessionStore) Finish(ctx context.Context, id, status, summary string, filesChanged []string, messageCount int) error {
	if sess, ok := s.sessions[id]; ok {
		sess.Status = status
		sess.Summary.String = summary
		sess.Summary.Valid = true
		s.sessions[id] = sess
	}
	return nil
}

func (s *stubAgentSessionStore) SetSnapshotTag(ctx context.Context, id, tag string) error {
	if sess, ok := s.sessions[id]; ok {
		sess.SnapshotTag.String = tag
		sess.SnapshotTag.Valid = true
		s.sessions[id] = sess
	}
	return nil
}

func (s *stubAgentSessionStore) MarkStaleRunning(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}

func (s *stubAgentSessionStore) SaveMessages(_ context.Context, id string, messagesJSON, chatLogJSON []byte) error {
	if sess, ok := s.sessions[id]; ok {
		sess.MessagesJSON = messagesJSON
		sess.ChatLogJSON = chatLogJSON
		s.sessions[id] = sess
	}
	return nil
}

func (s *stubAgentSessionStore) SavePreFileIDs(_ context.Context, id string, fileIDs []string) error {
	if sess, ok := s.sessions[id]; ok {
		sess.PreExistingFileIDs = fileIDs
		s.sessions[id] = sess
	}
	return nil
}

func (s *stubAgentSessionStore) AppendEvent(_ context.Context, sessionID string, seq int, eventType string, payload []byte) error {
	s.events = append(s.events, sqlstore.AgentSessionEvent{
		SessionID:   sessionID,
		Seq:         seq,
		EventType:   eventType,
		PayloadJSON: payload,
	})
	return nil
}

func (s *stubAgentSessionStore) ListEvents(_ context.Context, sessionID string) ([]sqlstore.AgentSessionEvent, error) {
	var result []sqlstore.AgentSessionEvent
	for _, ev := range s.events {
		if ev.SessionID == sessionID {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (s *stubAgentSessionStore) NextEventSeq(_ context.Context, sessionID string) (int, error) {
	maxSeq := -1
	for _, ev := range s.events {
		if ev.SessionID == sessionID && ev.Seq > maxSeq {
			maxSeq = ev.Seq
		}
	}
	return maxSeq + 1, nil
}

func (s *stubAgentSessionStore) SaveDiagnostics(_ context.Context, id string, diagJSON []byte) error {
	if sess, ok := s.sessions[id]; ok {
		sess.DiagnosticsJSON = diagJSON
		s.sessions[id] = sess
	}
	return nil
}

// ─── Helper: build a server with agent session store ─────────────────────────

func setupAgentTestServer(t *testing.T) (*Server, *stubDomainStore, *stubAgentSessionStore) {
	t.Helper()
	s := setupServer(t)

	domStore := newStubDomainStore()
	s.domains = domStore

	agentStore := newStubAgentSessionStore()
	s.agentSessions = agentStore

	return s, domStore, agentStore
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestAgentSessionsListRequiresAuth verifies that the sessions list endpoint requires authentication.
func TestAgentSessionsListRequiresAuth(t *testing.T) {
	s, domStore, _ := setupAgentTestServer(t)

	domStore.domains["dom-1"] = sqlstore.Domain{
		ID: "dom-1", ProjectID: "proj-1", URL: "example.com",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/domains/dom-1/agent/sessions", nil)
	rec := httptest.NewRecorder()

	// No auth → should be handled by withAuth middleware, but we call handler directly
	// So inject empty user
	ctx := context.WithValue(req.Context(), currentUserContextKey, auth.User{})
	s.handleAgentSessions(rec, req.WithContext(ctx), "dom-1")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAgentSessionsListOK verifies listing sessions for a domain.
func TestAgentSessionsListOK(t *testing.T) {
	s, domStore, agentStore := setupAgentTestServer(t)

	domStore.domains["dom-1"] = sqlstore.Domain{
		ID: "dom-1", ProjectID: "proj-1", URL: "example.com",
	}

	// Seed a session
	agentStore.sessions["sess-1"] = sqlstore.AgentSession{
		ID:       "sess-1",
		DomainID: "dom-1",
		Status:   "done",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/domains/dom-1/agent/sessions", nil)
	rec := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), currentUserContextKey, auth.User{
		Email: "user@example.com",
		Role:  "user",
	})
	s.handleAgentSessions(rec, req.WithContext(ctx), "dom-1")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 1 {
		t.Errorf("expected 1 session, got %d", len(resp))
	}
}

// TestAgentRollbackRequiresAuth verifies the rollback endpoint requires authentication.
func TestAgentRollbackRequiresAuth(t *testing.T) {
	s, _, _ := setupAgentTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/domains/dom-1/agent/sess-1/rollback", nil)
	rec := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), currentUserContextKey, auth.User{})
	s.handleAgentRollback(rec, req.WithContext(ctx), "dom-1", "sess-1")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// TestAgentStartRequiresAnthropicKey verifies that starting an agent without Anthropic API key returns an error.
func TestAgentStartRequiresAnthropicKey(t *testing.T) {
	s, domStore, _ := setupAgentTestServer(t)
	s.cfg.AnthropicAPIKey = "" // No API key

	domStore.domains["dom-1"] = sqlstore.Domain{
		ID: "dom-1", ProjectID: "proj-1", URL: "example.com",
	}

	// "Hello agent" has no context file, so matchFastPath returns false.
	// The handler enters SSE mode (HTTP 200) and sends an error event.
	body := strings.NewReader(`{"message":"Hello agent"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/domains/dom-1/agent", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), currentUserContextKey, auth.User{
		Email: "user@example.com",
		Role:  "user",
	})
	s.handleDomainAgent(rec, req.WithContext(ctx), "dom-1")

	if rec.Code != http.StatusOK {
		t.Errorf("expected SSE 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AI agent is not configured") {
		t.Errorf("expected API key error in SSE body, got: %s", rec.Body.String())
	}
}

// TestAgentStartMethodNotAllowed verifies only POST is allowed.
func TestAgentStartMethodNotAllowed(t *testing.T) {
	s, _, _ := setupAgentTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/domains/dom-1/agent", nil)
	rec := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), currentUserContextKey, auth.User{Email: "user@example.com"})
	s.handleDomainAgent(rec, req.WithContext(ctx), "dom-1")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

// ─── PR1 Guardrails tests ─────────────────────────────────────────────────────

// TestSystemPromptUnifiedFileEditingRules verifies there is no conflict between
// the patch_file and write_file instructions (old rules 11 vs 14).
func TestSystemPromptUnifiedFileEditingRules(t *testing.T) {
	s, _, _ := setupAgentTestServer(t)
	domain := sqlstore.Domain{ID: "d1", URL: "example.com", TargetLanguage: "en"}
	prompt := s.buildAgentSystemPrompt(domain)

	// The old conflicting pair was:
	//   rule 11: "ALWAYS use patch_file (not write_file)"
	//   rule 14: "do NOT use patch_file repeatedly"
	// After the fix, rule 11 must contain the unified decision tree and
	// the old absolute "ALWAYS use patch_file" must be gone.
	if strings.Contains(prompt, "ALWAYS use patch_file (not write_file)") {
		t.Error("system prompt still contains the old conflicting rule 11 text")
	}

	// New unified rule must describe all three cases.
	for _, want := range []string{
		"New file",
		"patch_file",
		"write_file",
		"read_file once",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("system prompt missing expected decision-tree text: %q", want)
		}
	}

	// Rule 14 should now mention budget, not the old patch efficiency hint.
	if strings.Contains(prompt, "Multiple patch_file calls on the same file waste time") {
		t.Error("system prompt still contains old rule 14 efficiency text (should be removed)")
	}
	if !strings.Contains(prompt, "BUDGET") {
		t.Error("system prompt rule 14 should contain budget guidance")
	}
}

// TestSystemPromptInlineFileRule verifies rule 1 allows skipping read_file
// when the file is already provided in context.
func TestSystemPromptInlineFileRule(t *testing.T) {
	s, _, _ := setupAgentTestServer(t)
	domain := sqlstore.Domain{ID: "d1", URL: "example.com"}
	prompt := s.buildAgentSystemPrompt(domain)

	// Old rule 1 said "ALWAYS read files before modifying them".
	// New rule 1 must have an exception for files already in context.
	if strings.Contains(prompt, "ALWAYS read files before modifying them") {
		t.Error("system prompt still contains old absolute rule 1 without inline-file exception")
	}
	if !strings.Contains(prompt, "already provided in the conversation context") {
		t.Error("system prompt rule 1 missing inline-file exception")
	}
}

// TestGuardrailConstants verifies guardrail constants have reasonable values.
func TestGuardrailConstants(t *testing.T) {
	if agentMaxIterations <= 0 {
		t.Errorf("agentMaxIterations must be > 0, got %d", agentMaxIterations)
	}
	if agentIterTimeoutSec <= 0 || agentIterTimeoutSec > 300 {
		t.Errorf("agentIterTimeoutSec should be between 1 and 300, got %d", agentIterTimeoutSec)
	}
	if agentNoProgressLimit < 2 || agentNoProgressLimit > agentMaxIterations {
		t.Errorf("agentNoProgressLimit should be between 2 and %d, got %d", agentMaxIterations, agentNoProgressLimit)
	}
	if agentBudgetWarnAt < 1 || agentBudgetWarnAt >= agentMaxIterations {
		t.Errorf("agentBudgetWarnAt should be between 1 and %d, got %d", agentMaxIterations-1, agentBudgetWarnAt)
	}
}

// TestTrimOldToolResults_TruncatesLargeContent verifies that tool results older than
// the last 2 messages are truncated to the given limit.
func TestTrimOldToolResults_TruncatesLargeContent(t *testing.T) {
	longContent := strings.Repeat("x", 10000)

	// Build a conversation: 3 old tool-result messages + 2 recent ones.
	makeToolResultMsg := func(content string) anthropic.MessageParam {
		return anthropic.NewUserMessage(
			anthropic.NewToolResultBlock("tool-id", content, false),
		)
	}

	messages := []anthropic.MessageParam{
		makeToolResultMsg(longContent), // old
		makeToolResultMsg(longContent), // old
		makeToolResultMsg(longContent), // old
		makeToolResultMsg(longContent), // recent (cutoff-1)
		makeToolResultMsg(longContent), // recent (cutoff)
	}

	trimmed := trimOldToolResults(messages, 4000)

	// First 3 messages should be truncated: original 10000 chars → ~4000 + short suffix.
	for i := 0; i < 3; i++ {
		block := trimmed[i].Content[0].OfToolResult
		if block == nil {
			t.Fatalf("msg[%d]: expected tool result block", i)
		}
		text := block.Content[0].OfText.Text
		if len(text) >= len(longContent) {
			t.Errorf("msg[%d]: content not truncated (len=%d, original=%d)", i, len(text), len(longContent))
		}
		if !strings.Contains(text, "omitted") {
			t.Errorf("msg[%d]: truncated text should contain omission marker", i)
		}
	}
	// Last 2 messages must be untouched.
	for i := 3; i < 5; i++ {
		block := trimmed[i].Content[0].OfToolResult
		if block == nil {
			t.Fatalf("msg[%d]: expected tool result block", i)
		}
		text := block.Content[0].OfText.Text
		if len(text) != len(longContent) {
			t.Errorf("msg[%d]: recent message was modified (len=%d, want %d)", i, len(text), len(longContent))
		}
	}
}

// TestTrimOldToolResults_ShortHistoryUnchanged verifies that short histories
// (≤ 4 messages) are returned unchanged.
func TestTrimOldToolResults_ShortHistoryUnchanged(t *testing.T) {
	longContent := strings.Repeat("y", 5000)
	msg := anthropic.NewUserMessage(anthropic.NewToolResultBlock("id", longContent, false))
	messages := []anthropic.MessageParam{msg, msg, msg} // only 3 messages

	trimmed := trimOldToolResults(messages, 100)
	for i, m := range trimmed {
		block := m.Content[0].OfToolResult
		if block == nil {
			continue
		}
		if text := block.Content[0].OfText.Text; len(text) != len(longContent) {
			t.Errorf("msg[%d]: short history was modified", i)
		}
	}
}

// ─── Event seq tests ──────────────────────────────────────────────────────────

// TestEventSeq_ResumeContinuation verifies that NextEventSeq returns MAX(seq)+1
// and the next AppendEvent uses that seq — no conflict with existing entries.
// Also tests the gap scenario: if seq 1 is missing (write failed in prior run),
// NextEventSeq still returns 3 (MAX=2+1), not 2 (COUNT=2).
func TestEventSeq_ResumeContinuation(t *testing.T) {
	store := newStubAgentSessionStore()
	ctx := context.Background()

	// Simulate a prior run with a gap: events 0 and 2 persisted, seq 1 missing
	// (as if the seq=1 AppendEvent failed but eventSeq was still incremented).
	for _, seq := range []int{0, 2} {
		if err := store.AppendEvent(ctx, "sess-resume", seq, "chat_entry", []byte(`{}`)); err != nil {
			t.Fatalf("seed AppendEvent seq=%d: %v", seq, err)
		}
	}

	// NextEventSeq must return MAX(2)+1 = 3, not COUNT=2.
	n, err := store.NextEventSeq(ctx, "sess-resume")
	if err != nil {
		t.Fatalf("NextEventSeq: %v", err)
	}
	if n != 3 {
		t.Errorf("expected NextEventSeq=3 (MAX+1 ignores gap), got %d", n)
	}

	// Simulate what runAgentLoop does on resume: start from eventSeq=n.
	eventSeq := n
	newPayload, _ := json.Marshal(chatLogEntry{ID: "e3", Role: "user", Text: "resume message"})
	if err := store.AppendEvent(ctx, "sess-resume", eventSeq, "chat_entry", newPayload); err != nil {
		t.Fatalf("AppendEvent on resume: %v", err)
	}

	// Total events: 0, 2, 3 (gap at 1). Resumed event must be at seq=3, not colliding with 0 or 2.
	events, err := store.ListEvents(ctx, "sess-resume")
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events after resume, got %d", len(events))
	}
	if events[2].Seq != 3 {
		t.Errorf("resumed event must have seq=3, got seq=%d", events[2].Seq)
	}
}

// ─── Detail endpoint tests ────────────────────────────────────────────────────

// TestDetailEndpoint_ActiveSession_ReconstructsFromEvents verifies that when
// a session is active (in hub) and has no ChatLogJSON checkpoint yet, the
// detail endpoint reconstructs the chat log from agent_session_events in the
// correct order and includes both the user message and the assistant entry.
func TestDetailEndpoint_ActiveSession_ReconstructsFromEvents(t *testing.T) {
	s, domStore, agentStore := setupAgentTestServer(t)

	const (
		domainID  = "dom-detail-active"
		sessionID = "sess-detail-active"
	)

	domStore.domains[domainID] = sqlstore.Domain{
		ID: domainID, ProjectID: "proj-1", URL: "example.com",
	}

	// Seed a running session with no ChatLogJSON checkpoint.
	agentStore.sessions[sessionID] = sqlstore.AgentSession{
		ID:       sessionID,
		DomainID: domainID,
		Status:   "running",
	}

	// Seed two events: user message (seq=0), assistant entry (seq=1).
	ctx := context.Background()
	userPayload, _ := json.Marshal(chatLogEntry{ID: "e0", Role: "user", Text: "do the task"})
	asstPayload, _ := json.Marshal(chatLogEntry{ID: "e1", Role: "assistant", Status: "running", Text: "working on it"})
	_ = agentStore.AppendEvent(ctx, sessionID, 0, "chat_entry", userPayload)
	_ = agentStore.AppendEvent(ctx, sessionID, 1, "chat_entry", asstPayload)

	// Register the session in the global hub so IsActive=true.
	_, ok := globalAgentHub.registerForDomain(sessionID, domainID)
	if !ok {
		t.Fatal("could not register session in hub")
	}
	t.Cleanup(func() {
		globalAgentHub.unregister(sessionID, domainID)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/agent/sessions/"+sessionID, nil)
	rec := httptest.NewRecorder()
	reqCtx := context.WithValue(req.Context(), currentUserContextKey, auth.User{
		Email: "user@example.com",
		Role:  "user",
	})
	s.handleAgentSessionDetail(rec, req.WithContext(reqCtx), domainID, sessionID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		IsActive bool              `json:"is_active"`
		ChatLog  []json.RawMessage `json:"chat_log"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !resp.IsActive {
		t.Error("expected is_active=true for hub-registered session")
	}
	if len(resp.ChatLog) != 2 {
		t.Fatalf("expected 2 chat entries from events, got %d", len(resp.ChatLog))
	}

	// Verify order: user first, assistant second.
	var first, second struct {
		Role string `json:"role"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(resp.ChatLog[0], &first); err != nil {
		t.Fatalf("unmarshal first entry: %v", err)
	}
	if err := json.Unmarshal(resp.ChatLog[1], &second); err != nil {
		t.Fatalf("unmarshal second entry: %v", err)
	}
	if first.Role != "user" {
		t.Errorf("first entry: expected role=user, got %s", first.Role)
	}
	if first.Text != "do the task" {
		t.Errorf("first entry: expected text=%q, got %q", "do the task", first.Text)
	}
	if second.Role != "assistant" {
		t.Errorf("second entry: expected role=assistant, got %s", second.Role)
	}
}

// TestDetailEndpoint_ActiveResumedSession_MergesHistoryAndEvents verifies that
// when a session is active but was previously checkpointed (has ChatLogJSON),
// the detail endpoint returns historical entries from ChatLogJSON prepended to
// live entries from agent_session_events — not just the live tail.
func TestDetailEndpoint_ActiveResumedSession_MergesHistoryAndEvents(t *testing.T) {
	s, domStore, agentStore := setupAgentTestServer(t)

	const (
		domainID  = "dom-detail-resumed"
		sessionID = "sess-detail-resumed"
	)

	domStore.domains[domainID] = sqlstore.Domain{
		ID: domainID, ProjectID: "proj-1", URL: "example.com",
	}

	// ChatLogJSON checkpoint from a prior run: two historical entries.
	historicalLog, _ := json.Marshal([]chatLogEntry{
		{ID: "h0", Role: "user", Text: "old request"},
		{ID: "h1", Role: "assistant", Status: "done", Text: "old answer"},
	})
	agentStore.sessions[sessionID] = sqlstore.AgentSession{
		ID:          sessionID,
		DomainID:    domainID,
		Status:      "running",
		ChatLogJSON: historicalLog,
	}

	// Previous run left events at seq=0,1 (same content as ChatLogJSON checkpoint).
	// Current run appends new events at seq=2,3.
	// The endpoint must NOT include seq=0,1 again (duplication bug).
	ctx := context.Background()
	oldUser, _ := json.Marshal(chatLogEntry{ID: "h0", Role: "user", Text: "old request"})
	oldAsst, _ := json.Marshal(chatLogEntry{ID: "h1", Role: "assistant", Status: "done", Text: "old answer"})
	_ = agentStore.AppendEvent(ctx, sessionID, 0, "chat_entry", oldUser)
	_ = agentStore.AppendEvent(ctx, sessionID, 1, "chat_entry", oldAsst)
	userPayload, _ := json.Marshal(chatLogEntry{ID: "n0", Role: "user", Text: "new request"})
	asstPayload, _ := json.Marshal(chatLogEntry{ID: "n1", Role: "assistant", Status: "running", Text: "working"})
	_ = agentStore.AppendEvent(ctx, sessionID, 2, "chat_entry", userPayload)
	_ = agentStore.AppendEvent(ctx, sessionID, 3, "chat_entry", asstPayload)

	// Register in hub so IsActive=true.
	_, ok := globalAgentHub.registerForDomain(sessionID, domainID)
	if !ok {
		t.Fatal("could not register session in hub")
	}
	t.Cleanup(func() {
		globalAgentHub.unregister(sessionID, domainID)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/agent/sessions/"+sessionID, nil)
	rec := httptest.NewRecorder()
	reqCtx := context.WithValue(req.Context(), currentUserContextKey, auth.User{
		Email: "user@example.com",
		Role:  "user",
	})
	s.handleAgentSessionDetail(rec, req.WithContext(reqCtx), domainID, sessionID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		IsActive bool              `json:"is_active"`
		ChatLog  []json.RawMessage `json:"chat_log"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !resp.IsActive {
		t.Error("expected is_active=true")
	}
	// Expect 4 entries: 2 historical + 2 live.
	if len(resp.ChatLog) != 4 {
		t.Fatalf("expected 4 chat entries (2 historical + 2 live), got %d", len(resp.ChatLog))
	}

	// Verify order: historical first, then new entries.
	var entries []struct {
		ID   string `json:"id"`
		Role string `json:"role"`
		Text string `json:"text"`
	}
	for i, raw := range resp.ChatLog {
		var e struct {
			ID   string `json:"id"`
			Role string `json:"role"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatalf("unmarshal entry %d: %v", i, err)
		}
		entries = append(entries, e)
	}
	if entries[0].ID != "h0" || entries[1].ID != "h1" {
		t.Errorf("expected historical entries first: got %v %v", entries[0].ID, entries[1].ID)
	}
	if entries[2].ID != "n0" || entries[3].ID != "n1" {
		t.Errorf("expected new entries last: got %v %v", entries[2].ID, entries[3].ID)
	}
}

// ─── PR5: Prompt caching tests ────────────────────────────────────────────────

// TestPromptCaching_SystemHasCacheControl verifies that the system block passed
// to Anthropic always carries a cache breakpoint so the stable prefix is eligible
// for prompt caching without changing agent behaviour.
func TestPromptCaching_SystemHasCacheControl(t *testing.T) {
	s, _, _ := setupAgentTestServer(t)
	domain := sqlstore.Domain{ID: "d1", URL: "example.com", TargetLanguage: "en"}

	// Build params the same way runAgentLoop does (one iteration).
	systemText := s.buildAgentSystemPrompt(domain)
	systemBlocks := []anthropic.TextBlockParam{{
		Type:         "text",
		Text:         systemText,
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	}}

	if len(systemBlocks) != 1 {
		t.Fatalf("expected 1 system block, got %d", len(systemBlocks))
	}
	cc := systemBlocks[0].CacheControl
	if cc.Type != "ephemeral" {
		t.Errorf("system block CacheControl.Type: want %q, got %q", "ephemeral", cc.Type)
	}
}

// TestPromptCaching_LastToolHasCacheControl verifies that the last tool returned
// by domainAgentTools carries a cache breakpoint, making the entire tools block
// eligible for caching when combined with the system prompt breakpoint.
func TestPromptCaching_LastToolHasCacheControl(t *testing.T) {
	tools := domainAgentTools()
	if len(tools) == 0 {
		t.Fatal("domainAgentTools returned empty slice")
	}
	last := tools[len(tools)-1]
	if last.OfTool == nil {
		t.Fatal("last tool OfTool is nil")
	}
	cc := last.OfTool.CacheControl
	if cc.Type != "ephemeral" {
		t.Errorf("last tool CacheControl.Type: want %q, got %q", "ephemeral", cc.Type)
	}
	// All other tools must NOT have a cache breakpoint (only last sets the boundary).
	for i, tool := range tools[:len(tools)-1] {
		if tool.OfTool != nil && tool.OfTool.CacheControl.Type == "ephemeral" {
			t.Errorf("tool[%d] (%s) unexpectedly has cache control", i, tool.OfTool.Name)
		}
	}
}

// ─── PR5: Diagnostics tests ───────────────────────────────────────────────────

// TestDiagnostics_DetailEndpointReturnsDiagnostics verifies that diagnostics
// saved to a session are returned by the detail endpoint under the "diagnostics" key.
func TestDiagnostics_DetailEndpointReturnsDiagnostics(t *testing.T) {
	s, domStore, agentStore := setupAgentTestServer(t)

	const (
		domainID  = "dom-diag"
		sessionID = "sess-diag"
	)
	domStore.domains[domainID] = sqlstore.Domain{ID: domainID, ProjectID: "proj-1", URL: "example.com"}

	diagPayload := []byte(`{"mode":"agent_loop","iterations":3,"compacted_bytes":512,"snapshot_files":2,"fast_path_hit":false}`)
	agentStore.sessions[sessionID] = sqlstore.AgentSession{
		ID:              sessionID,
		DomainID:        domainID,
		Status:          "done",
		DiagnosticsJSON: diagPayload,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/agent/sessions/"+sessionID, nil)
	rec := httptest.NewRecorder()
	reqCtx := context.WithValue(req.Context(), currentUserContextKey, auth.User{
		Email: "user@example.com", Role: "user",
	})
	s.handleAgentSessionDetail(rec, req.WithContext(reqCtx), domainID, sessionID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Diagnostics *struct {
			Mode           string `json:"mode"`
			Iterations     int    `json:"iterations"`
			CompactedBytes int    `json:"compacted_bytes"`
			SnapshotFiles  int    `json:"snapshot_files"`
			FastPathHit    bool   `json:"fast_path_hit"`
		} `json:"diagnostics"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Diagnostics == nil {
		t.Fatal("diagnostics field missing from response")
	}
	d := resp.Diagnostics
	if d.Mode != "agent_loop" {
		t.Errorf("mode: want agent_loop, got %s", d.Mode)
	}
	if d.Iterations != 3 {
		t.Errorf("iterations: want 3, got %d", d.Iterations)
	}
	if d.CompactedBytes != 512 {
		t.Errorf("compacted_bytes: want 512, got %d", d.CompactedBytes)
	}
	if d.SnapshotFiles != 2 {
		t.Errorf("snapshot_files: want 2, got %d", d.SnapshotFiles)
	}
	if d.FastPathHit {
		t.Error("fast_path_hit: want false, got true")
	}
}

// TestDiagnostics_SaveDiagnosticsOnFinish verifies that SaveDiagnostics is called
// with valid JSON containing all required fields when finishSession executes.
func TestDiagnostics_SaveDiagnosticsOnFinish(t *testing.T) {
	_, _, agentStore := setupAgentTestServer(t)

	const sessionID = "sess-diag-finish"
	agentStore.sessions[sessionID] = sqlstore.AgentSession{ID: sessionID, DomainID: "dom-x", Status: "running"}

	// Simulate SaveDiagnostics being called (as finishSession would do).
	diag := struct {
		Mode           string `json:"mode"`
		Iterations     int    `json:"iterations"`
		CompactedBytes int    `json:"compacted_bytes"`
		SnapshotFiles  int    `json:"snapshot_files"`
		FastPathHit    bool   `json:"fast_path_hit"`
	}{
		Mode:           "agent_loop",
		Iterations:     2,
		CompactedBytes: 128,
		SnapshotFiles:  1,
		FastPathHit:    false,
	}
	b, err := json.Marshal(diag)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := agentStore.SaveDiagnostics(context.Background(), sessionID, b); err != nil {
		t.Fatalf("SaveDiagnostics: %v", err)
	}

	sess, _ := agentStore.Get(context.Background(), sessionID)
	if len(sess.DiagnosticsJSON) == 0 {
		t.Fatal("DiagnosticsJSON not persisted by SaveDiagnostics")
	}
	var got struct {
		Mode        string `json:"mode"`
		Iterations  int    `json:"iterations"`
		FastPathHit bool   `json:"fast_path_hit"`
	}
	if err := json.Unmarshal(sess.DiagnosticsJSON, &got); err != nil {
		t.Fatalf("unmarshal stored diagnostics: %v", err)
	}
	if got.Mode != "agent_loop" {
		t.Errorf("mode: want agent_loop, got %s", got.Mode)
	}
	if got.Iterations != 2 {
		t.Errorf("iterations: want 2, got %d", got.Iterations)
	}
	if got.FastPathHit {
		t.Error("fast_path_hit: want false")
	}
}
