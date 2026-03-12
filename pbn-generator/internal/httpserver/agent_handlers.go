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
	agentMaxIterations = 20
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

	domain, err := s.domains.Get(r.Context(), domainID)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	restored, err := s.rollbackAgentSnapshot(r.Context(), domain, sess.SnapshotTag.String)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rollback failed")
		return
	}

	_ = s.agentSessions.Finish(r.Context(), sessionID, "rolled_back", "Rolled back by user", sess.FilesChanged, sess.MessageCount)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "rolled_back",
		"restored": restored,
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

	if strings.TrimSpace(s.cfg.AnthropicAPIKey) == "" {
		writeError(w, http.StatusServiceUnavailable, "AI agent is not configured (ANTHROPIC_API_KEY is missing)")
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
						case <-r.Context().Done():
							return
						}
					}
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
					case <-r.Context().Done():
						return // client disconnected, agent keeps running
					}
				}
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
		snapshotTag, _ = s.createAgentSnapshot(context.Background(), domain, sessionID)
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
			userMsg = fmt.Sprintf("[Current file: %s]\n```\n%s\n```\n\n%s",
				req.Context.CurrentFile, string(fileContent), userMsg)
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
					case <-r.Context().Done():
						return
					}
				}
			}
		}
		return
	}
	subID, ch, _ := hs.subscribe()
	defer hs.unsubscribe(subID)

	// Run agent loop in background goroutine (detached from HTTP request)
	go s.runAgentLoop(sessionID, domainID, domain, user.Email, messages, chatLog, snapshotTag, hs)

	// Forward hub events to SSE
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
		case <-r.Context().Done():
			return // client disconnected, agent keeps running in background
		}
	}
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
	hs *hubSession,
) {
	// Declare here so the panic-recovery defer can close over them.
	var allFilesChanged []string
	var totalMessages int

	defer func() {
		// Catch any panics to prevent goroutine from crashing the entire server
		if r := recover(); r != nil {
			s.logger.Errorf("agent loop panic for session %s: %v", sessionID, r)
			hs.publish("error", map[string]any{
				"type":               "error",
				"message":            "Внутренняя ошибка агента",
				"rollback_available": snapshotTag != "",
			})
			_ = s.agentSessions.Finish(context.Background(), sessionID, "error",
				fmt.Sprintf("panic: %v", r), allFilesChanged, totalMessages)
			// Always persist whatever chat log we have so history is not lost
			if cl, err := json.Marshal(chatLog); err == nil {
				if ml, err2 := json.Marshal(messages); err2 == nil {
					_ = s.agentSessions.SaveMessages(context.Background(), sessionID, ml, cl)
				}
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
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := anthropic.NewClient(option.WithAPIKey(s.cfg.AnthropicAPIKey))
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
		_ = s.agentSessions.Finish(context.Background(), sessionID, status, summary, allFilesChanged, totalMessages)
		// Save final chat log
		if cl, err := json.Marshal(chatLog); err == nil {
			if ml, err2 := json.Marshal(messages); err2 == nil {
				_ = s.agentSessions.SaveMessages(context.Background(), sessionID, ml, cl)
			}
		}
	}

	// continueSent guards against injecting the "continue" nudge more than once
	// to prevent infinite loops when Claude is genuinely done but produces only text.
	// lastIterHadTools is true when the previous user message was a tool_result turn,
	// meaning the agent just processed tool results and may legitimately need a nudge.
	continueSent := false
	lastIterHadTools := false

	for iteration := 0; iteration < agentMaxIterations; iteration++ {
		select {
		case <-ctx.Done():
			publish("stopped", map[string]string{"type": "stopped", "message": "Timeout or cancelled"})
			finishSession("stopped", "Timed out")
			return
		default:
		}

		// Trim large tool_result content from older iterations to stay within token rate limits.
		compressedMessages := trimOldToolResults(messages, 4000)

		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(s.cfg.AnthropicModel),
			MaxTokens: maxTokens,
			System:    []anthropic.TextBlockParam{{Type: "text", Text: s.buildAgentSystemPrompt(domain)}},
			Messages:  compressedMessages,
			Tools:     domainAgentTools(),
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

			stream := client.Messages.NewStreaming(ctx, params)
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
			if streamErr == nil {
				break retryLoop
			}
			errMsg := streamErr.Error()
			isRateLimit := strings.Contains(errMsg, "429") || strings.Contains(errMsg, "rate_limit") || strings.Contains(errMsg, "rate limit")
			if isRateLimit && apiRetry < maxAPIRetries-1 {
				waitSec := 60 * (apiRetry + 1)
				s.logger.Warnf("agent session %s: Anthropic 429 rate limit (attempt %d), waiting %ds", sessionID, apiRetry+1, waitSec)
				publish("text", map[string]string{
					"type":  "text",
					"delta": fmt.Sprintf("\n[Rate limit reached — waiting %ds before retry...]\n", waitSec),
				})
				select {
				case <-time.After(time.Duration(waitSec) * time.Second):
					continue retryLoop
				case <-ctx.Done():
					publish("stopped", map[string]string{"type": "stopped", "message": "Timeout during rate-limit wait"})
					finishSession("stopped", "Timed out during rate-limit wait")
					return
				}
			}
			break retryLoop
		}
		if streamErr != nil {
			errMsg := streamErr.Error()
			publish("error", map[string]any{
				"type":               "error",
				"message":            errMsg,
				"rollback_available": snapshotTag != "",
			})
			if currentAsstEntry != nil {
				currentAsstEntry.Status = "error"
				currentAsstEntry.Error = errMsg
				currentAsstEntry.Text = textBuf.String()
				chatLog = append(chatLog, *currentAsstEntry)
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
			publish("done", map[string]any{
				"type":          "done",
				"summary":       summary,
				"files_changed": allFilesChanged,
			})
			if currentAsstEntry != nil {
				currentAsstEntry.Status = "done"
				currentAsstEntry.Text = textBuf.String()
				currentAsstEntry.FilesChanged = allFilesChanged
				chatLog = append(chatLog, *currentAsstEntry)
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
					chatLog = append(chatLog, *currentAsstEntry)
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
			publish("done", map[string]any{
				"type":          "done",
				"summary":       summary,
				"files_changed": allFilesChanged,
			})
			if currentAsstEntry != nil {
				currentAsstEntry.Status = "done"
				currentAsstEntry.Text = textBuf.String()
				currentAsstEntry.FilesChanged = allFilesChanged
				chatLog = append(chatLog, *currentAsstEntry)
			}
			finishSession("done", summary)
			return
		}

		// Execute tools
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

			result := s.executeAgentTool(ctx, domain, userEmail, toolBlock.ID, toolBlock.Name, toolBlock.Input)

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

		// Finalize current assistant entry with tool calls
		if currentAsstEntry != nil {
			currentAsstEntry.Text = textBuf.String()
			chatLog = append(chatLog, *currentAsstEntry)
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

		// Save progress to DB after each round-trip
		if ml, err := json.Marshal(messages); err == nil {
			if cl, err2 := json.Marshal(chatLog); err2 == nil {
				_ = s.agentSessions.SaveMessages(context.Background(), sessionID, ml, cl)
			}
		}
	}

	// Max iterations reached
	publish("done", map[string]any{
		"type":          "done",
		"summary":       "Reached maximum iterations",
		"files_changed": allFilesChanged,
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
	if len(sess.ChatLogJSON) > 0 {
		dto.ChatLog = json.RawMessage(sess.ChatLogJSON)
	}
	_, dto.IsActive = globalAgentHub.lookup(sessionID)
	writeJSON(w, http.StatusOK, dto)
}

// trimOldToolResults reduces input token count by truncating large tool_result
// content blocks from messages older than the last 2 (current iteration's output).
// This prevents read_file results from accumulating across many iterations.
func trimOldToolResults(messages []anthropic.MessageParam, maxContentLen int) []anthropic.MessageParam {
	if len(messages) <= 4 {
		return messages
	}
	raw, err := json.Marshal(messages)
	if err != nil {
		return messages
	}
	var generic []map[string]interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return messages
	}
	for i := 0; i < len(generic)-2; i++ {
		msg := generic[i]
		if msg["role"] != "user" {
			continue
		}
		contentArr, _ := msg["content"].([]interface{})
		for _, item := range contentArr {
			block, _ := item.(map[string]interface{})
			if block["type"] != "tool_result" {
				continue
			}
			if content, ok := block["content"].(string); ok && len(content) > maxContentLen {
				block["content"] = content[:maxContentLen] +
					fmt.Sprintf("\n[...%d chars omitted to reduce context size]", len(content)-maxContentLen)
			}
		}
	}
	trimmedRaw, err := json.Marshal(generic)
	if err != nil {
		return messages
	}
	var result []anthropic.MessageParam
	if err := json.Unmarshal(trimmedRaw, &result); err != nil {
		return messages
	}
	return result
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
1. ALWAYS read files before modifying them (use read_file)
2. Use list_files to understand site structure before creating new files
3. Preserve the existing style, theme and language of the site
4. HTML must be valid
5. Images must always be saved in the assets/ folder
6. Do not create duplicate files
7. When generating images, write a detailed prompt in English
8. Respond in the user's language
9. If the task is unclear, ask for clarification before starting
10. Keep explanatory text brief — a single short sentence before each tool call is enough
11. IMPORTANT: When editing an existing file, ALWAYS use patch_file (not write_file) to make targeted changes. Only use write_file when creating a brand new file. patch_file is faster, safer, and avoids rewriting large files.
12. CRITICAL: Complete ALL requested tasks before stopping. Never end your turn with only planning text like "Now I will create..." — instead, immediately call the tool. If an image generation fails, skip it and continue with the remaining tasks. Only output a final summary AFTER all tools have been called.
13. When creating new HTML pages, keep them concise — reuse the existing CSS from the site (link to the same stylesheet), avoid long inline styles or large blocks of lorem-ipsum. A working page with real content is better than a verbose one that gets cut off.`,
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

	event := sqlstore.LLMUsageEvent{
		Provider:                "anthropic",
		Operation:               "agent",
		Model:                   model,
		Status:                  "success",
		RequesterEmail:          requesterEmail,
		ProjectID:               sql.NullString{String: projectID, Valid: projectID != ""},
		DomainID:                sql.NullString{String: domainID, Valid: domainID != ""},
		PromptTokens:            sql.NullInt64{Int64: inputTokens, Valid: true},
		CompletionTokens:        sql.NullInt64{Int64: outputTokens, Valid: true},
		TotalTokens:             sql.NullInt64{Int64: total, Valid: true},
		TokenSource:             "api",
		InputPriceUSDPerMillion: inputPrice,
		OutputPriceUSDPerMillion: outputPrice,
		EstimatedCostUSD:        estCost,
		CreatedAt:               time.Now().UTC(),
	}
	_ = s.llmUsage.CreateEvent(ctx, event)
}
