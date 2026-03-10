package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// ─── Stub AgentSessionStore ──────────────────────────────────────────────────

type stubAgentSessionStore struct {
	sessions map[string]sqlstore.AgentSession
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

	body := strings.NewReader(`{"message":"Hello agent"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/domains/dom-1/agent", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), currentUserContextKey, auth.User{
		Email: "user@example.com",
		Role:  "user",
	})
	s.handleDomainAgent(rec, req.WithContext(ctx), "dom-1")

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 (no API key), got %d: %s", rec.Code, rec.Body.String())
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
