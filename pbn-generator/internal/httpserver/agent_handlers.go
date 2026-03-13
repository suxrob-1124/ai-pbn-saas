package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/google/uuid"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

const (
	agentMaxIterations   = 20
	agentIterTimeoutSec  = 90 // per-Anthropic-call timeout (seconds); child of session timeout
	agentNoProgressLimit = 3  // stop early after N consecutive iterations with no file changes
	agentBudgetWarnAt    = 5  // inject budget warning when ≤N iterations remain
)

// ─── Request / Response types ─────────────────────────────────────────────────

type agentStartRequest struct {
	Message   string            `json:"message"`
	SessionID string            `json:"session_id,omitempty"`
	Context   *agentContextHint `json:"context,omitempty"`
}

type agentContextHint struct {
	CurrentFile        string `json:"current_file,omitempty"`
	IncludeCurrentFile bool   `json:"include_current_file,omitempty"`
}

// ─── SSE helpers ─────────────────────────────────────────────────────────────

func sseWrite(w http.ResponseWriter, eventType string, payload any) {
	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "data: %s\n\n", data)
	_ = eventType // kept for future named events
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// sseHeartbeat sends a comment line to keep the SSE connection alive.
func sseHeartbeat(w http.ResponseWriter) {
	fmt.Fprintf(w, ": heartbeat\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

const sseHeartbeatInterval = 15 * time.Second

// relayHubEventsSSE forwards hub events to the SSE response writer with periodic heartbeats.
// Returns when the session ends (done/error/stopped), the channel closes, or the client disconnects.
func relayHubEventsSSE(w http.ResponseWriter, ch chan agentHubEvent, reqCtx context.Context) {
	ticker := time.NewTicker(sseHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case ev, open := <-ch:
			if !open {
				return
			}
			sseWrite(w, ev.EventType, ev.Payload)
			if ev.EventType == "done" || ev.EventType == "error" || ev.EventType == "stopped" {
				return
			}
		case <-ticker.C:
			sseHeartbeat(w)
		case <-reqCtx.Done():
			return
		}
	}
}

func setupSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}

// ─── Route dispatcher ─────────────────────────────────────────────────────────

// handleAgentRoute is the main dispatcher for /api/domains/{id}/agent/...
// Called from handleDomainActions when the path segment is "agent".
func (s *Server) handleAgentRoute(w http.ResponseWriter, r *http.Request, domainID string) {
	// Path after /agent is: "" | "/sessions" | "/{session_id}/rollback" | "/{session_id}/stop"
	suffix := strings.TrimPrefix(r.URL.Path, "/api/domains/"+domainID+"/agent")
	suffix = strings.TrimPrefix(suffix, "/")

	switch {
	case suffix == "" || suffix == "/":
		s.handleDomainAgent(w, r, domainID)
	case suffix == "sessions":
		s.handleAgentSessions(w, r, domainID)
	default:
		parts := strings.SplitN(suffix, "/", 2)
		sessionID := parts[0]
		action := ""
		if len(parts) > 1 {
			action = parts[1]
		}
		switch action {
		case "rollback":
			s.handleAgentRollback(w, r, domainID, sessionID)
		case "stop":
			s.handleAgentStop(w, r, domainID, sessionID)
		case "":
			if r.Method == http.MethodGet {
				s.handleAgentSessionDetail(w, r, domainID, sessionID)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		default:
			writeError(w, http.StatusNotFound, "unknown agent action")
		}
	}
}

// ─── GET /api/domains/{id}/agent/sessions ────────────────────────────────────

func (s *Server) handleAgentSessions(w http.ResponseWriter, r *http.Request, domainID string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sessions, err := s.agentSessions.ListByDomain(r.Context(), domainID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list sessions")
		return
	}

	type sessionDTO struct {
		ID           string     `json:"id"`
		DomainID     string     `json:"domain_id"`
		CreatedBy    string     `json:"created_by"`
		CreatedAt    time.Time  `json:"created_at"`
		FinishedAt   *time.Time `json:"finished_at,omitempty"`
		Status       string     `json:"status"`
		Summary      string     `json:"summary,omitempty"`
		FilesChanged []string   `json:"files_changed,omitempty"`
		MessageCount int        `json:"message_count"`
		SnapshotTag  string     `json:"snapshot_tag,omitempty"`
	}

	result := make([]sessionDTO, 0, len(sessions))
	for _, sess := range sessions {
		dto := sessionDTO{
			ID:           sess.ID,
			DomainID:     sess.DomainID,
			CreatedBy:    sess.CreatedBy,
			CreatedAt:    sess.CreatedAt,
			Status:       sess.Status,
			FilesChanged: sess.FilesChanged,
			MessageCount: sess.MessageCount,
		}
		if sess.FinishedAt.Valid {
			dto.FinishedAt = &sess.FinishedAt.Time
		}
		if sess.Summary.Valid {
			dto.Summary = sess.Summary.String
		}
		if sess.SnapshotTag.Valid {
			dto.SnapshotTag = sess.SnapshotTag.String
		}
		result = append(result, dto)
	}
	writeJSON(w, http.StatusOK, result)
}

// ─── POST /api/domains/{id}/agent/{session_id}/rollback ──────────────────────

func (s *Server) handleAgentRollback(w http.ResponseWriter, r *http.Request, domainID, sessionID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sess, err := s.agentSessions.Get(r.Context(), sessionID)
	if err != nil || sess.ID == "" {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if sess.DomainID != domainID {
		writeError(w, http.StatusForbidden, "session does not belong to this domain")
		return
	}
	if !sess.SnapshotTag.Valid || sess.SnapshotTag.String == "" {
		writeError(w, http.StatusBadRequest, "no snapshot available for this session")
		return
	}

	// Reject rollback while the agent loop is still running — it could continue
	// writing files after the rollback, leaving the domain in an inconsistent state.
	if hs, ok := globalAgentHub.lookup(sessionID); ok && !hs.isDone() {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "Нельзя откатить активную сессию. Сначала остановите агента.",
		})
		return
	}

	domain, err := s.domains.Get(r.Context(), domainID)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	rb, err := s.rollbackAgentSnapshot(r.Context(), domain, sess)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rollback failed")
		return
	}

	if rb.RestoredExisting == 0 && rb.DeletedCreated == 0 {
		// Snapshot existed but nothing changed — agent made no mutations in this session.
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "nothing_to_restore",
			"restored": 0,
			"deleted":  0,
		})
		return
	}

	_ = s.agentSessions.Finish(r.Context(), sessionID, "rolled_back", "Rolled back by user", sess.FilesChanged, sess.MessageCount)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "rolled_back",
		"restored": rb.RestoredExisting,
		"deleted":  rb.DeletedCreated,
	})
}

// ─── POST /api/domains/{id}/agent/{session_id}/stop ──────────────────────────

func (s *Server) handleAgentStop(w http.ResponseWriter, r *http.Request, domainID, sessionID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sess, err := s.agentSessions.Get(r.Context(), sessionID)
	if err != nil || sess.ID == "" {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	_ = s.agentSessions.Finish(r.Context(), sessionID, "stopped", "Stopped by user", sess.FilesChanged, sess.MessageCount)
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// ─── POST /api/domains/{id}/agent (SSE streaming) ────────────────────────────
// Also handles reconnect: if session_id provided and message is empty, reconnects to active session.

// chatLogEntry is the UI-serializable chat message saved to DB.
type chatLogEntry struct {
	ID           string            `json:"id"`
	Role         string            `json:"role"`
	Text         string            `json:"text,omitempty"`
	ToolCalls    []chatLogToolCall `json:"tool_calls,omitempty"`
	Status       string            `json:"status,omitempty"`
	FilesChanged []string          `json:"files_changed,omitempty"`
	Error        string            `json:"error,omitempty"`
}

type chatLogToolCall struct {
	ID      string                 `json:"id"`
	Tool    string                 `json:"tool"`
	Input   map[string]interface{} `json:"input"`
	Preview string                 `json:"preview,omitempty"`
	Done    bool                   `json:"done"`
	IsError bool                   `json:"is_error"`
}

func (s *Server) handleDomainAgent(w http.ResponseWriter, r *http.Request, domainID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req agentStartRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	sessionID := req.SessionID
	isReconnect := sessionID != "" && strings.TrimSpace(req.Message) == ""
	isNewSession := sessionID == ""

	if !isReconnect && strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	domain, err := s.domains.Get(r.Context(), domainID)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	// SSE setup
	setupSSE(w)

	// If this domain already has an active agent session, subscribe to it instead of starting a new one.
	// This handles the case where the user refreshes and sends a new message while agent is still running.
	if isNewSession {
		if activeID, ok := globalAgentHub.getActiveDomainSession(domainID); ok {
			if hs, ok := globalAgentHub.lookup(activeID); ok {
				subID, ch, buffered := hs.subscribe()
				defer hs.unsubscribe(subID)
				for _, ev := range buffered {
					sseWrite(w, ev.EventType, ev.Payload)
				}
				if !hs.isDone() {
					relayHubEventsSSE(w, ch, r.Context())
				}
				return
			}
		}
	}

	// If session is active in hub by session_id, just subscribe and relay events (reconnect case)
	if sessionID != "" {
		if hs, ok := globalAgentHub.lookup(sessionID); ok {
			subID, ch, buffered := hs.subscribe()
			defer hs.unsubscribe(subID)
			// Replay buffered events
			for _, ev := range buffered {
				sseWrite(w, ev.EventType, ev.Payload)
			}
			if !hs.isDone() {
				relayHubEventsSSE(w, ch, r.Context())
			}
			return
		}
	}

	if isReconnect {
		// Session is not in hub (completed/stopped), nothing to reconnect to
		writeError(w, http.StatusGone, "session is no longer active")
		return
	}

	// Load previous messages if resuming a completed session
	var initialMessages []anthropic.MessageParam
	var chatLog []chatLogEntry
	if !isNewSession {
		if sess, err := s.agentSessions.Get(r.Context(), sessionID); err == nil {
			if len(sess.MessagesJSON) > 0 {
				_ = json.Unmarshal(sess.MessagesJSON, &initialMessages)
			}
			if len(sess.ChatLogJSON) > 0 {
				_ = json.Unmarshal(sess.ChatLogJSON, &chatLog)
			}
		}
	}

	// New session
	if isNewSession {
		sessionID = uuid.New().String()
		_ = s.agentSessions.Create(context.Background(), sqlstore.AgentSession{
			ID:        sessionID,
			DomainID:  domainID,
			CreatedBy: user.Email,
			Status:    "running",
		})
	}

	// Snapshot on first user message in this session
	var snapshotTag string
	if isNewSession {
		var snapErr error
		snapshotTag, snapErr = s.createAgentSnapshot(context.Background(), domain, sessionID, user.Email)
		if snapErr != nil && s.logger != nil {
			s.logger.Warnf("failed to create agent snapshot for domain %s session %s: %v", domainID, sessionID, snapErr)
		}
		if snapshotTag != "" {
			_ = s.agentSessions.SetSnapshotTag(context.Background(), sessionID, snapshotTag)
		}
	}

	sseWrite(w, "session_start", map[string]any{
		"type":        "session_start",
		"session_id":  sessionID,
		"snapshot_id": snapshotTag,
		"is_new":      isNewSession,
	})

	// Build user message
	userMsg := req.Message
	if req.Context != nil && req.Context.IncludeCurrentFile && req.Context.CurrentFile != "" {
		fileContent, err := s.readDomainFileBytesFromBackend(context.Background(), domain, req.Context.CurrentFile)
		if err == nil {
			userMsg = fmt.Sprintf(
				"[Current file: %s]\n```\n%s\n```\n\n[NOTE: The file content above is already loaded — do NOT call read_file for '%s' before your first edit. Edit it directly with patch_file or write_file.]\n\n%s",
				req.Context.CurrentFile, string(fileContent), req.Context.CurrentFile, userMsg)
		}
	}

	// Append user message to messages and chat log
	messages := append(initialMessages, anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)))
	chatLog = append(chatLog, chatLogEntry{
		ID:   uuid.New().String(),
		Role: "user",
		Text: req.Message, // store original (without file context)
	})

	// Check global concurrency limit
	if !globalAgentHub.acquireSem() {
		sseWrite(w, "error", map[string]any{
			"type":    "error",
			"message": "Сервер перегружен: слишком много активных агентов. Попробуйте через минуту.",
		})
		return
	}

	// Register hub session for this domain (ensures only 1 active per domain)
	hs, ok := globalAgentHub.registerForDomain(sessionID, domainID)
	if !ok {
		globalAgentHub.releaseSem()
		// Race condition: another goroutine registered just before us — subscribe to it
		if activeID, exists := globalAgentHub.getActiveDomainSession(domainID); exists {
			if hs2, ok2 := globalAgentHub.lookup(activeID); ok2 {
				subID, ch, buffered := hs2.subscribe()
				defer hs2.unsubscribe(subID)
				for _, ev := range buffered {
					sseWrite(w, ev.EventType, ev.Payload)
				}
				relayHubEventsSSE(w, ch, r.Context())
			}
		}
		return
	}
	subID, ch, _ := hs.subscribe()
	defer hs.unsubscribe(subID)

	// Detect a postcondition from the original user message so the agent loop can
	// skip the final LLM round-trip if the task is verifiably complete server-side.
	pc := detectPostcondition(req.Message)
	contextFile := ""
	if req.Context != nil {
		contextFile = req.Context.CurrentFile
	}

	// For resumed sessions, initialise eventSeq from MAX(seq)+1 so new events
	// never collide with existing ones even when there are gaps in the prior run.
	initialEventSeq := 0
	if !isNewSession {
		if n, err := s.agentSessions.NextEventSeq(context.Background(), sessionID); err == nil {
			initialEventSeq = n
		}
	}

	// Routing order:
	//   1. Fast path  — deterministic single-file, no LLM needed
	//   2. Single-shot — non-deterministic but still single-file, 1 LLM call
	//   3. Full agent loop — everything else
	//
	// Both single-shot and agent loop require Anthropic; check the key once here.
	if fpOp, ok := matchFastPath(req.Message, contextFile); ok {
		// Fast path: execute locally without calling Anthropic (no API key required).
		// Pass `messages` so runFastPath can append the synthetic assistant response.
		go s.runFastPath(sessionID, domainID, domain, user.Email, fpOp, chatLog, snapshotTag, initialEventSeq, hs, messages)
	} else if strings.TrimSpace(s.cfg.AnthropicAPIKey) == "" {
		// No API key and fast path didn't match — cannot proceed.
		_ = s.agentSessions.Finish(context.Background(), sessionID, "error", "AI agent not configured", nil, 0)
		hs.publish("error", map[string]any{"type": "error", "message": "AI agent is not configured (ANTHROPIC_API_KEY is missing)"})
		globalAgentHub.releaseSem()
		hs.markDone()
		go func() { time.Sleep(30 * time.Second); globalAgentHub.unregister(sessionID, domainID) }()
	} else if matchSingleShot(req.Message, contextFile) {
		// Single-shot: one LLM call, one write, no tool loop.
		go s.runSingleShot(sessionID, domainID, domain, user.Email, contextFile, req.Message, pc, messages, chatLog, snapshotTag, initialEventSeq, hs)
	} else {
		// Full agent loop: multi-step tool use for complex or multi-file tasks.
		go s.runAgentLoop(sessionID, domainID, domain, user.Email, messages, chatLog, snapshotTag, pc, contextFile, initialEventSeq, hs)
	}

	// Forward hub events to SSE with heartbeats
	relayHubEventsSSE(w, ch, r.Context())
}

// runAgentLoop runs the agent loop in a background goroutine.
// It uses its own timeout context (not tied to the HTTP request).
func (s *Server) runAgentLoop(
	sessionID, domainID string,
	domain sqlstore.Domain,
	userEmail string,
	messages []anthropic.MessageParam,
	chatLog []chatLogEntry,
	snapshotTag string,
	pc agentPostcondition,
	currentFile string,
	initialEventSeq int,
	hs *hubSession,
) {
	// Declare here so the panic-recovery defer can close over them.
	var allFilesChanged []string
	var totalMessages int
	// baselined tracks which files have had a content revision created this session.
	// Must be declared before the defer so the panic path can read len(baselined).
	baselined := make(map[string]bool)
	// revisionsFinalized guards against double finalization (finishSession + panic defer).
	var revisionsFinalized bool

	// agentDiagnostics accumulates per-session telemetry (PR5).
	// Declared before the defer so both the panic-recovery path and finishSession can read it.
	type agentDiagnostics struct {
		Mode           string `json:"mode"`
		Iterations     int    `json:"iterations"`
		CompactedBytes int    `json:"compacted_bytes"`
		SnapshotFiles  int    `json:"snapshot_files"`
		FastPathHit    bool   `json:"fast_path_hit"`
	}
	diag := agentDiagnostics{Mode: "agent_loop"}

	dedupChanged := func() []string {
		return uniquePaths(allFilesChanged)
	}

	defer func() {
		// Catch any panics to prevent goroutine from crashing the entire server
		if r := recover(); r != nil {
			filesChanged := dedupChanged()
			s.logger.Errorf("agent loop panic for session %s: %v", sessionID, r)
			hs.publish("error", map[string]any{
				"type":               "error",
				"message":            "Внутренняя ошибка агента",
				"rollback_available": snapshotTag != "" && len(filesChanged) > 0,
			})
			// Create partial revisions for any files changed before the panic.
			if !revisionsFinalized && len(filesChanged) > 0 {
				revisionsFinalized = true
				s.finalizeAgentRevisions(context.Background(), domain, filesChanged, sessionID, userEmail, "agent_partial")
			}
			_ = s.agentSessions.Finish(context.Background(), sessionID, "error",
				fmt.Sprintf("panic: %v", r), filesChanged, totalMessages)
			// Always persist whatever chat log we have so history is not lost
			if cl, err := json.Marshal(chatLog); err == nil {
				if ml, err2 := json.Marshal(messages); err2 == nil {
					_ = s.agentSessions.SaveMessages(context.Background(), sessionID, ml, cl)
				}
			}
			// Persist whatever diagnostics were accumulated before the panic.
			diag.SnapshotFiles = len(baselined)
			if b, err := json.Marshal(diag); err == nil {
				_ = s.agentSessions.SaveDiagnostics(context.Background(), sessionID, b)
			}
		}
		globalAgentHub.releaseSem()
		hs.markDone()
		// Clean up hub after a delay to allow late reconnects to see the final event
		go func() {
			time.Sleep(30 * time.Second)
			globalAgentHub.unregister(sessionID, domainID)
		}()
	}()

	timeout := time.Duration(s.cfg.AgentTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	opts := []option.RequestOption{option.WithAPIKey(s.cfg.AnthropicAPIKey)}
	if s.cfg.AnthropicBaseURL != "" {
		opts = append(opts, option.WithBaseURL(s.cfg.AnthropicBaseURL))
	}
	client := anthropic.NewClient(opts...)
	maxTokens := int64(s.cfg.AgentMaxTokens)
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	// Current assistant chat log entry (built per iteration)
	var currentAsstEntry *chatLogEntry

	publish := func(eventType string, payload any) {
		hs.publish(eventType, payload)
	}

	finishSession := func(status, summary string) {
		// Create one visible revision per changed file so user-facing history
		// reflects what the agent did.  Source depends on outcome:
		//   "done"  → "agent"         (successful session)
		//   other   → "agent_partial" (interrupted / failed session)
		if !revisionsFinalized {
			revisionsFinalized = true
			changed := dedupChanged()
			if status == "done" {
				s.finalizeAgentRevisions(context.Background(), domain, changed, sessionID, userEmail, "agent")
			} else if len(changed) > 0 {
				s.finalizeAgentRevisions(context.Background(), domain, changed, sessionID, userEmail, "agent_partial")
			}
		}
		_ = s.agentSessions.Finish(context.Background(), sessionID, status, summary, dedupChanged(), totalMessages)
		// Save final chat log
		if cl, err := json.Marshal(chatLog); err == nil {
			if ml, err2 := json.Marshal(messages); err2 == nil {
				_ = s.agentSessions.SaveMessages(context.Background(), sessionID, ml, cl)
			}
		}
		// Persist diagnostics
		diag.SnapshotFiles = len(baselined)
		if b, err := json.Marshal(diag); err == nil {
			_ = s.agentSessions.SaveDiagnostics(context.Background(), sessionID, b)
		}
	}

	// continueSent guards against injecting the "continue" nudge more than once
	// to prevent infinite loops when Claude is genuinely done but produces only text.
	// lastIterHadTools is true when the previous user message was a tool_result turn,
	// meaning the agent just processed tool results and may legitimately need a nudge.
	continueSent := false
	lastIterHadTools := false

	// noProgressCount tracks consecutive iterations where no file was changed.
	// If the agent keeps calling read/list/search without writing, we stop early.
	noProgressCount := 0
	// budgetWarnSent ensures we inject the budget warning only once.
	budgetWarnSent := false

	// eventSeq is a monotonically increasing counter for agent_session_events rows.
	// Initialised from existing event count so resumed sessions never collide on seq.
	eventSeq := initialEventSeq
	// appendChatEvent persists a single chatLogEntry as a delta event so the
	// detail endpoint can reconstruct the chat log without a full-blob read.
	appendChatEvent := func(entry chatLogEntry) {
		b, err := json.Marshal(entry)
		if err != nil {
			return
		}
		_ = s.agentSessions.AppendEvent(context.Background(), sessionID, eventSeq, "chat_entry", b)
		eventSeq++
	}

	// Persist the user message that triggered this run.
	// The new user message is always the last entry in chatLog at this point —
	// it was just appended in handleDomainAgent before launching the goroutine.
	// Using len-1 is robust to old sessions that have no event log yet (where
	// initialEventSeq=0 but chatLog already contains checkpoint entries).
	if len(chatLog) > 0 {
		appendChatEvent(chatLog[len(chatLog)-1])
	}

	for iteration := 0; iteration < agentMaxIterations; iteration++ {
		select {
		case <-ctx.Done():
			publish("error", map[string]any{
				"type":               "error",
				"message":            fmt.Sprintf("Ошибка: время выполнения агента истекло (%d мин). Попробуйте разбить задачу на более мелкие шаги.", int(timeout.Minutes())),
				"rollback_available": snapshotTag != "" && len(allFilesChanged) > 0,
			})
			finishSession("error", "Timed out")
			return
		default:
		}

		// Compact old history: replace large file content in tool_result and tool_use blocks
		// with short hash placeholders. Keeps last 2 turns intact.
		compressedMessages := compactAgentHistory(messages, compactMaxContentLen, compactKeepRecentTurns)

		// Diagnostics: track iterations and compacted bytes.
		diag.Iterations++
		if beforeB, err := json.Marshal(messages); err == nil {
			if afterB, err := json.Marshal(compressedMessages); err == nil {
				if delta := len(beforeB) - len(afterB); delta > 0 {
					diag.CompactedBytes += delta
				}
			}
		}
		// Push live diagnostics to the hub so the detail endpoint reflects
		// current metrics without waiting for finishSession.
		diag.SnapshotFiles = len(baselined)
		if b, err := json.Marshal(diag); err == nil {
			hs.setLiveDiag(b)
		}

		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(s.cfg.AnthropicModel),
			MaxTokens: maxTokens,
			// Cache breakpoint on system prompt: Anthropic will cache this stable
			// prefix across iterations, reducing input token cost for long sessions.
			System: []anthropic.TextBlockParam{{
				Type:         "text",
				Text:         s.buildAgentSystemPrompt(domain),
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			}},
			Messages: compressedMessages,
			Tools:    domainAgentTools(),
		}

		var msg anthropic.Message
		var textBuf strings.Builder
		var toolUses []anthropic.ToolUseBlock

		// Start new assistant chat log entry
		asstEntryID := uuid.New().String()
		currentAsstEntry = &chatLogEntry{
			ID:     asstEntryID,
			Role:   "assistant",
			Status: "running",
		}

		// Call Anthropic with retry on 429 rate-limit errors.
		const maxAPIRetries = 3
		var streamErr error
	retryLoop:
		for apiRetry := 0; apiRetry < maxAPIRetries; apiRetry++ {
			if apiRetry > 0 {
				// Reset accumulators before retry
				textBuf.Reset()
				toolUses = toolUses[:0]
				msg = anthropic.Message{}
			}

			// Per-iteration timeout: each Anthropic call gets its own deadline,
			// independent of (but bounded by) the overall session timeout.
			iterCtx, iterCancel := context.WithTimeout(ctx, agentIterTimeoutSec*time.Second)
			stream := client.Messages.NewStreaming(iterCtx, params)
			for stream.Next() {
				event := stream.Current()
				_ = msg.Accumulate(event)
				if e, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
					if td, ok := e.Delta.AsAny().(anthropic.TextDelta); ok {
						textBuf.WriteString(td.Text)
						publish("text", map[string]string{"type": "text", "delta": td.Text})
					}
				}
			}
			streamErr = stream.Err()
			iterCancel()
			if streamErr == nil {
				break retryLoop
			}
			errMsg := streamErr.Error()
			isRateLimit := strings.Contains(errMsg, "429") || strings.Contains(errMsg, "rate_limit") || strings.Contains(errMsg, "rate limit")
			if isRateLimit && apiRetry < maxAPIRetries-1 {
				waitSec := 60 * (apiRetry + 1)
				s.logger.Warnf("agent session %s: Anthropic 429 rate limit (attempt %d), waiting %ds", sessionID, apiRetry+1, waitSec)
				publish("rate_limit", map[string]any{
					"type":     "rate_limit",
					"wait_sec": waitSec,
					"attempt":  apiRetry + 1,
				})
				// Wait with periodic progress updates so the UI stays responsive.
				deadline := time.After(time.Duration(waitSec) * time.Second)
				progressTicker := time.NewTicker(5 * time.Second)
				remaining := waitSec
			waitLoop:
				for {
					select {
					case <-deadline:
						progressTicker.Stop()
						break waitLoop
					case <-progressTicker.C:
						remaining -= 5
						if remaining < 0 {
							remaining = 0
						}
						publish("rate_limit", map[string]any{
							"type":          "rate_limit",
							"remaining_sec": remaining,
						})
					case <-ctx.Done():
						progressTicker.Stop()
						publish("stopped", map[string]string{"type": "stopped", "message": "Timeout during rate-limit wait"})
						finishSession("stopped", "Timed out during rate-limit wait")
						return
					}
				}
				continue retryLoop
			}
			break retryLoop
		}
		if streamErr != nil {
			errMsg := streamErr.Error()
			s.logger.Warnf("agent session %s: stream error on iteration %d: %v (ctx.Err=%v)", sessionID, iteration, streamErr, ctx.Err())
			// Detect context deadline / cancellation and provide a user-friendly message.
			if ctx.Err() != nil {
				errMsg = fmt.Sprintf("Ошибка: время выполнения агента истекло (%d мин). Попробуйте разбить задачу на более мелкие шаги.", int(timeout.Minutes()))
			}
			hasChanges := len(allFilesChanged) > 0
			publish("error", map[string]any{
				"type":               "error",
				"message":            errMsg,
				"rollback_available": snapshotTag != "" && hasChanges,
			})
			if currentAsstEntry != nil {
				currentAsstEntry.Status = "error"
				currentAsstEntry.Error = errMsg
				currentAsstEntry.Text = textBuf.String()
				entry := *currentAsstEntry
				chatLog = append(chatLog, entry)
				appendChatEvent(entry)
			}
			finishSession("error", errMsg)
			return
		}

		totalMessages++
		go s.logAnthropicUsage(context.Background(), userEmail, domain.ProjectID, domainID, s.cfg.AnthropicModel,
			msg.Usage.InputTokens, msg.Usage.OutputTokens)

		for _, block := range msg.Content {
			if tu, ok := block.AsAny().(anthropic.ToolUseBlock); ok {
				toolUses = append(toolUses, tu)
			}
		}

		// If output was truncated (max_tokens), stop to avoid infinite loops
		if msg.StopReason == "max_tokens" {
			summary := textBuf.String()
			if len(summary) > 200 {
				summary = summary[:200] + "..."
			}
			filesChanged := dedupChanged()
			publish("done", map[string]any{
				"type":               "done",
				"summary":            summary,
				"files_changed":      filesChanged,
				"rollback_available": snapshotTag != "" && len(filesChanged) > 0,
			})
			if currentAsstEntry != nil {
				currentAsstEntry.Status = "done"
				currentAsstEntry.Text = textBuf.String()
				currentAsstEntry.FilesChanged = filesChanged
				entry := *currentAsstEntry
				chatLog = append(chatLog, entry)
				appendChatEvent(entry)
			}
			finishSession("done", "Max tokens reached: "+summary)
			return
		}

		// End turn — but if Claude just received tool results and produced only text,
		// it may have written a "planning sentence" without calling the next tool.
		// Inject one "continue" nudge so it actually completes the task.
		if msg.StopReason == "end_turn" || len(toolUses) == 0 {
			if lastIterHadTools && !continueSent && textBuf.Len() > 0 {
				continueSent = true
				lastIterHadTools = false // nudge is a plain user text, not a tool-result
				// Append the planning text as an assistant message, then nudge
				if currentAsstEntry != nil {
					currentAsstEntry.Text = textBuf.String()
					entry := *currentAsstEntry
					chatLog = append(chatLog, entry)
					appendChatEvent(entry)
					currentAsstEntry = nil
				}
				messages = append(messages, msg.ToParam())
				messages = append(messages, anthropic.NewUserMessage(
					anthropic.NewTextBlock("Continue and complete all remaining tasks now. Call the necessary tools immediately."),
				))
				continue
			}

			summary := textBuf.String()
			if len(summary) > 200 {
				summary = summary[:200] + "..."
			}
			filesChanged := dedupChanged()
			publish("done", map[string]any{
				"type":               "done",
				"summary":            summary,
				"files_changed":      filesChanged,
				"rollback_available": snapshotTag != "" && len(filesChanged) > 0,
			})
			if currentAsstEntry != nil {
				currentAsstEntry.Status = "done"
				currentAsstEntry.Text = textBuf.String()
				currentAsstEntry.FilesChanged = filesChanged
				entry := *currentAsstEntry
				chatLog = append(chatLog, entry)
				appendChatEvent(entry)
			}
			finishSession("done", summary)
			return
		}

		// Execute tools
		filesChangedBefore := len(allFilesChanged)
		var toolResultBlocks []anthropic.ContentBlockParamUnion
		for _, toolBlock := range toolUses {
			var inputMap map[string]interface{}
			_ = json.Unmarshal(toolBlock.Input, &inputMap)

			publish("tool_start", map[string]any{
				"type":  "tool_start",
				"id":    toolBlock.ID,
				"tool":  toolBlock.Name,
				"input": toolBlock.Input,
			})

			result := s.executeAgentTool(ctx, domain, userEmail, toolBlock.ID, toolBlock.Name, toolBlock.Input, sessionID, baselined)

			preview := result.Content
			if len(preview) > 300 {
				preview = preview[:300] + "..."
			}
			publish("tool_done", map[string]any{
				"type":    "tool_done",
				"id":      toolBlock.ID,
				"tool":    toolBlock.Name,
				"preview": preview,
				"error":   result.IsError,
			})

			if result.FileChanged != nil {
				allFilesChanged = append(allFilesChanged, result.FileChanged.Path)
				publish("file_changed", map[string]any{
					"type":   "file_changed",
					"path":   result.FileChanged.Path,
					"action": result.FileChanged.Action,
				})
			}

			// Track in current assistant entry
			if currentAsstEntry != nil {
				currentAsstEntry.ToolCalls = append(currentAsstEntry.ToolCalls, chatLogToolCall{
					ID:      toolBlock.ID,
					Tool:    toolBlock.Name,
					Input:   inputMap,
					Preview: preview,
					Done:    true,
					IsError: result.IsError,
				})
			}

			toolResultBlocks = append(toolResultBlocks, anthropic.NewToolResultBlock(toolBlock.ID, result.Content, result.IsError))
		}

		// Postcondition auto-stop: if a narrow mechanical task was recognised and the
		// target file was just changed, verify the postcondition locally.  If it passes,
		// finish the session without another Anthropic round-trip.
		if pc.Kind != pcNone && len(allFilesChanged) > filesChangedBefore {
			fileToCheck := currentFile
			if fileToCheck == "" {
				// Only auto-stop when a single unique file has been changed in the session
				// (unambiguous target).
				if unique := uniquePaths(allFilesChanged); len(unique) == 1 {
					fileToCheck = unique[0]
				}
			}
			if fileToCheck != "" {
				changedTarget := false
				for _, p := range allFilesChanged[filesChangedBefore:] {
					if p == fileToCheck {
						changedTarget = true
						break
					}
				}
				if changedTarget {
					if content, err := s.readDomainFileBytesFromBackend(ctx, domain, fileToCheck); err == nil {
						if checkPostcondition(pc, content) {
							filesChanged := dedupChanged()
							if currentAsstEntry != nil {
								currentAsstEntry.Status = "done"
								currentAsstEntry.Text = textBuf.String()
								currentAsstEntry.FilesChanged = filesChanged
								entry := *currentAsstEntry
								chatLog = append(chatLog, entry)
								appendChatEvent(entry)
							}
							publish("done", map[string]any{
								"type":               "done",
								"summary":            "Task completed: server-side verification passed.",
								"files_changed":      filesChanged,
								"rollback_available": snapshotTag != "" && len(filesChanged) > 0,
							})
							finishSession("done", "Postcondition verified — auto-stop.")
							return
						}
					}
				}
			}
		}

		// No-progress tracking: count consecutive iterations without file changes,
		// but only AFTER the first file has been modified in this session.
		// Before the first file change, the agent may legitimately need several
		// read/search/list iterations to gather context — penalising that causes
		// false-positive early termination on read-heavy tasks.
		if len(allFilesChanged) > filesChangedBefore {
			noProgressCount = 0
		} else if len(allFilesChanged) > 0 {
			// At least one file was changed earlier; now the agent is stalled.
			noProgressCount++
			if noProgressCount >= agentNoProgressLimit {
				filesChanged := dedupChanged()
				if currentAsstEntry != nil {
					currentAsstEntry.Status = "done"
					currentAsstEntry.Text = textBuf.String()
					currentAsstEntry.FilesChanged = filesChanged
					entry := *currentAsstEntry
					chatLog = append(chatLog, entry)
					appendChatEvent(entry)
				}
				publish("done", map[string]any{
					"type":               "done",
					"summary":            "No further file changes after several iterations — task appears complete.",
					"files_changed":      filesChanged,
					"rollback_available": snapshotTag != "" && len(filesChanged) > 0,
				})
				finishSession("done", "No-progress stop: no file changes in last iterations")
				return
			}
		}

		// Budget governor: when few iterations remain, inject a one-time warning so the
		// agent wraps up instead of starting new sub-tasks.
		if !budgetWarnSent && iteration >= agentMaxIterations-agentBudgetWarnAt {
			budgetWarnSent = true
			toolResultBlocks = append(toolResultBlocks, anthropic.NewTextBlock(
				fmt.Sprintf("[SYSTEM BUDGET WARNING: You have used %d of %d allowed iterations. "+
					"Complete your current task now. If a file needs updating, write_file it immediately and stop. "+
					"Do not start new sub-tasks.]", iteration+1, agentMaxIterations),
			))
		}

		// Finalize current assistant entry with tool calls
		if currentAsstEntry != nil {
			currentAsstEntry.Text = textBuf.String()
			entry := *currentAsstEntry
			chatLog = append(chatLog, entry)
			// Append a cheap delta event instead of rewriting the full blob.
			appendChatEvent(entry)
			currentAsstEntry = nil
		}

		// Tools were executed — reset the continue-nudge guard so future planning-only
		// turns still get a nudge (e.g. "I'll create images" → tools → "I'll create pages" → tools).
		// Mark that the NEXT iteration follows a tool-result user message.
		continueSent = false
		lastIterHadTools = true

		// Append to messages
		messages = append(messages, msg.ToParam())
		messages = append(messages, anthropic.NewUserMessage(toolResultBlocks...))
	}

	// Max iterations reached
	filesChanged := dedupChanged()
	publish("done", map[string]any{
		"type":               "done",
		"summary":            "Reached maximum iterations",
		"files_changed":      filesChanged,
		"rollback_available": snapshotTag != "" && len(filesChanged) > 0,
	})
	finishSession("done", "Reached max iterations")
}

// handleAgentSessionDetail returns session detail including chat log.
func (s *Server) handleAgentSessionDetail(w http.ResponseWriter, r *http.Request, domainID, sessionID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sess, err := s.agentSessions.Get(r.Context(), sessionID)
	if err != nil || sess.ID == "" {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if sess.DomainID != domainID {
		writeError(w, http.StatusForbidden, "session does not belong to this domain")
		return
	}

	type detailDTO struct {
		ID           string          `json:"id"`
		DomainID     string          `json:"domain_id"`
		CreatedBy    string          `json:"created_by"`
		CreatedAt    time.Time       `json:"created_at"`
		FinishedAt   *time.Time      `json:"finished_at,omitempty"`
		Status       string          `json:"status"`
		Summary      string          `json:"summary,omitempty"`
		FilesChanged []string        `json:"files_changed,omitempty"`
		MessageCount int             `json:"message_count"`
		SnapshotTag  string          `json:"snapshot_tag,omitempty"`
		ChatLog      json.RawMessage `json:"chat_log,omitempty"`
		IsActive     bool            `json:"is_active"`
		Diagnostics  json.RawMessage `json:"diagnostics,omitempty"`
	}
	dto := detailDTO{
		ID:           sess.ID,
		DomainID:     sess.DomainID,
		CreatedBy:    sess.CreatedBy,
		CreatedAt:    sess.CreatedAt,
		Status:       sess.Status,
		FilesChanged: sess.FilesChanged,
		MessageCount: sess.MessageCount,
	}
	if sess.FinishedAt.Valid {
		dto.FinishedAt = &sess.FinishedAt.Time
	}
	if sess.Summary.Valid {
		dto.Summary = sess.Summary.String
	}
	if sess.SnapshotTag.Valid {
		dto.SnapshotTag = sess.SnapshotTag.String
	}
	hs, found := globalAgentHub.lookup(sessionID)
	isActive := found && !hs.isDone()
	dto.IsActive = isActive
	if isActive {
		// Prefer live diagnostics from the hub (updated each iteration).
		// Fall back to DB value for sessions that haven't completed their first iteration.
		if live := hs.getLiveDiag(); len(live) > 0 {
			dto.Diagnostics = json.RawMessage(live)
		} else if len(sess.DiagnosticsJSON) > 0 {
			dto.Diagnostics = json.RawMessage(sess.DiagnosticsJSON)
		}
	} else if len(sess.DiagnosticsJSON) > 0 {
		dto.Diagnostics = json.RawMessage(sess.DiagnosticsJSON)
	}

	// For active sessions, always reconstruct from the event log so the UI sees
	// live progress rather than a stale checkpoint from a prior run.
	// For finished sessions, use the ChatLogJSON checkpoint (cheaper — already in
	// the row — and complete, including any terminal entries).
	if dto.IsActive {
		// Start with the historical checkpoint (may be empty for new sessions).
		var entries []chatLogEntry
		if len(sess.ChatLogJSON) > 0 {
			_ = json.Unmarshal(sess.ChatLogJSON, &entries)
		}
		// Events from previous runs (seq < len(entries)) are already captured in
		// ChatLogJSON. Only append events from the current run to avoid duplicates.
		currentRunStartSeq := len(entries)
		events, evErr := s.agentSessions.ListEvents(r.Context(), sessionID)
		if evErr == nil {
			for _, ev := range events {
				if ev.EventType != "chat_entry" || ev.Seq < currentRunStartSeq {
					continue
				}
				var entry chatLogEntry
				if json.Unmarshal(ev.PayloadJSON, &entry) == nil {
					entries = append(entries, entry)
				}
			}
		}
		if b, err := json.Marshal(entries); err == nil {
			dto.ChatLog = json.RawMessage(b)
		}
	} else if len(sess.ChatLogJSON) > 0 {
		dto.ChatLog = json.RawMessage(sess.ChatLogJSON)
	} else {
		// Fallback: session finished before checkpoint was written (crash/hard kill).
		// Reconstruct from events if available.
		events, evErr := s.agentSessions.ListEvents(r.Context(), sessionID)
		if evErr == nil && len(events) > 0 {
			var entries []chatLogEntry
			for _, ev := range events {
				if ev.EventType != "chat_entry" {
					continue
				}
				var entry chatLogEntry
				if json.Unmarshal(ev.PayloadJSON, &entry) == nil {
					entries = append(entries, entry)
				}
			}
			if b, err := json.Marshal(entries); err == nil {
				dto.ChatLog = json.RawMessage(b)
			}
		}
	}
	writeJSON(w, http.StatusOK, dto)
}

// ─── System prompt ────────────────────────────────────────────────────────────

func (s *Server) buildAgentSystemPrompt(domain sqlstore.Domain) string {
	lang := domain.TargetLanguage
	if lang == "" {
		lang = "en"
	}
	keyword := domain.MainKeyword
	genType := domain.GenerationType

	return fmt.Sprintf(`You are an AI assistant for a site editor. You have access to all files of the site %s.

Site information:
- URL: %s
- Language: %s
- Main keyword: %s
- Generation type: %s

RULES:
1. Read files before modifying them (use read_file) — unless the file content is already provided in the conversation context (look for "[Current file: ...]" in the message).
2. Use list_files to understand site structure before creating new files
3. Preserve the existing style, theme and language of the site
4. HTML must be valid
5. Images must always be saved in the assets/ folder
6. Do not create duplicate files
7. When generating images, write a detailed prompt in English
8. Respond in the user's language
9. If the task is unclear, ask for clarification before starting
10. Keep explanatory text brief — a single short sentence before each tool call is enough
11. FILE EDITING — pick the right tool based on scope:
    • New file → write_file directly (no read_file needed).
    • 1–2 small targeted edits in an existing file → patch_file (faster, no full read needed).
    • 3 or more edits to the same file, OR large restructuring (e.g. removing all comments, replacing many values) → read_file once, then ONE write_file with the fully-updated content.
    Never call patch_file on the same file more than twice in one turn — use write_file instead.
    Never use write_file on an existing file without reading it first (unless the file is already in context or you are explicitly asked to overwrite).
12. CRITICAL: Complete ALL requested tasks before stopping. Never end your turn with only planning text like "Now I will create..." — instead, immediately call the tool. If an image generation fails, skip it and continue with the remaining tasks. Only output a final summary AFTER all tools have been called.
13. When creating new HTML pages, keep them concise — reuse the existing CSS from the site (link to the same stylesheet), avoid long inline styles or large blocks of lorem-ipsum. A working page with real content is better than a verbose one that gets cut off.
14. BUDGET: You have a limited number of iterations. Do not make unnecessary read_file calls for files you do not intend to modify. If you receive a budget warning, finalize your work immediately with a write_file and stop.`,
		domain.URL, domain.URL, lang, keyword, genType)
}

// ─── LLM usage logging ────────────────────────────────────────────────────────

func (s *Server) logAnthropicUsage(ctx context.Context, requesterEmail, projectID, domainID, model string, inputTokens, outputTokens int64) {
	if requesterEmail == "" {
		return
	}
	total := inputTokens + outputTokens

	var inputPrice, outputPrice sql.NullFloat64
	var estCost sql.NullFloat64
	if s.modelPricing != nil {
		if pricing, err := s.modelPricing.GetActiveByModel(ctx, "anthropic", model, time.Now()); err == nil {
			inputPrice = sql.NullFloat64{Float64: pricing.InputUSDPerMillion, Valid: true}
			outputPrice = sql.NullFloat64{Float64: pricing.OutputUSDPerMillion, Valid: true}
			cost := (float64(inputTokens)/1_000_000)*pricing.InputUSDPerMillion +
				(float64(outputTokens)/1_000_000)*pricing.OutputUSDPerMillion
			estCost = sql.NullFloat64{Float64: cost, Valid: true}
		} else {
			// Fallback: hardcoded pricing for known Anthropic models (USD per million tokens)
			var defIn, defOut float64
			switch {
			case strings.HasPrefix(model, "claude-opus-4") || strings.Contains(model, "claude-3-opus"):
				defIn, defOut = 15.0, 75.0
			case strings.HasPrefix(model, "claude-sonnet-4") || strings.Contains(model, "claude-3-5-sonnet") || strings.Contains(model, "claude-3-sonnet"):
				defIn, defOut = 3.0, 15.0
			case strings.HasPrefix(model, "claude-haiku-4") || strings.Contains(model, "claude-3-haiku"):
				defIn, defOut = 0.25, 1.25
			}
			if defIn > 0 {
				inputPrice = sql.NullFloat64{Float64: defIn, Valid: true}
				outputPrice = sql.NullFloat64{Float64: defOut, Valid: true}
				cost := (float64(inputTokens)/1_000_000)*defIn + (float64(outputTokens)/1_000_000)*defOut
				estCost = sql.NullFloat64{Float64: cost, Valid: true}
			}
		}
	}

	event := sqlstore.LLMUsageEvent{
		Provider:                 "anthropic",
		Operation:                "agent",
		Model:                    model,
		Status:                   "success",
		RequesterEmail:           requesterEmail,
		ProjectID:                sql.NullString{String: projectID, Valid: projectID != ""},
		DomainID:                 sql.NullString{String: domainID, Valid: domainID != ""},
		PromptTokens:             sql.NullInt64{Int64: inputTokens, Valid: true},
		CompletionTokens:         sql.NullInt64{Int64: outputTokens, Valid: true},
		TotalTokens:              sql.NullInt64{Int64: total, Valid: true},
		TokenSource:              "api",
		InputPriceUSDPerMillion:  inputPrice,
		OutputPriceUSDPerMillion: outputPrice,
		EstimatedCostUSD:         estCost,
		CreatedAt:                time.Now().UTC(),
	}
	if s.llmUsage != nil {
		_ = s.llmUsage.CreateEvent(ctx, event)
	}
}
