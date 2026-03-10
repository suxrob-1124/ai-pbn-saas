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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	domain, err := s.domains.Get(r.Context(), domainID)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	// SSE setup — after this point we write SSE events directly.
	setupSSE(w)

	timeout := time.Duration(s.cfg.AgentTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Session management
	sessionID := req.SessionID
	isNewSession := sessionID == ""
	if isNewSession {
		sessionID = uuid.New().String()
		_ = s.agentSessions.Create(ctx, sqlstore.AgentSession{
			ID:        sessionID,
			DomainID:  domainID,
			CreatedBy: user.Email,
			Status:    "running",
		})
	}

	// Snapshot on new session
	var snapshotTag string
	if isNewSession {
		snapshotTag, _ = s.createAgentSnapshot(ctx, domain, sessionID)
		if snapshotTag != "" {
			_ = s.agentSessions.SetSnapshotTag(ctx, sessionID, snapshotTag)
		}
	}

	sseWrite(w, "session_start", map[string]any{
		"type":        "session_start",
		"session_id":  sessionID,
		"snapshot_id": snapshotTag,
		"is_new":      isNewSession,
	})

	// Build system prompt
	systemPrompt := s.buildAgentSystemPrompt(domain)

	// Build initial messages
	userMsg := req.Message
	if req.Context != nil && req.Context.IncludeCurrentFile && req.Context.CurrentFile != "" {
		fileContent, err := s.readDomainFileBytesFromBackend(ctx, domain, req.Context.CurrentFile)
		if err == nil {
			userMsg = fmt.Sprintf("[Current file: %s]\n```\n%s\n```\n\n%s",
				req.Context.CurrentFile, string(fileContent), userMsg)
		}
	}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)),
	}

	// Agent loop
	client := anthropic.NewClient(option.WithAPIKey(s.cfg.AnthropicAPIKey))
	maxTokens := int64(s.cfg.AgentMaxTokens)
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	var allFilesChanged []string
	var totalMessages int

	for iteration := 0; iteration < agentMaxIterations; iteration++ {
		select {
		case <-ctx.Done():
			sseWrite(w, "stopped", map[string]string{"type": "stopped", "message": "Timeout or cancelled"})
			_ = s.agentSessions.Finish(context.Background(), sessionID, "stopped", "Timed out", allFilesChanged, totalMessages)
			return
		default:
		}

		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(s.cfg.AnthropicModel),
			MaxTokens: maxTokens,
			System:    []anthropic.TextBlockParam{{Type: "text", Text: systemPrompt}},
			Messages:  messages,
			Tools:     domainAgentTools(),
		}

		// Streaming response
		stream := client.Messages.NewStreaming(ctx, params)

		var msg anthropic.Message
		var textBuf strings.Builder
		var toolUses []anthropic.ToolUseBlock

		// Accumulate events from stream
		for stream.Next() {
			event := stream.Current()
			_ = msg.Accumulate(event)
			// Stream text deltas as SSE events
			if e, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
				if td, ok := e.Delta.AsAny().(anthropic.TextDelta); ok {
					textBuf.WriteString(td.Text)
					sseWrite(w, "text", map[string]string{"type": "text", "delta": td.Text})
				}
			}
		}
		if err := stream.Err(); err != nil {
			errMsg := err.Error()
			sseWrite(w, "error", map[string]any{
				"type":               "error",
				"message":            errMsg,
				"rollback_available": snapshotTag != "",
			})
			_ = s.agentSessions.Finish(context.Background(), sessionID, "error", errMsg, allFilesChanged, totalMessages)
			return
		}

		totalMessages++

		// Log LLM usage for this Anthropic call
		go s.logAnthropicUsage(context.Background(), user.Email, domain.ProjectID, domainID, s.cfg.AnthropicModel,
			msg.Usage.InputTokens, msg.Usage.OutputTokens)

		// Collect tool uses from message content
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
			sseWrite(w, "done", map[string]any{
				"type":          "done",
				"summary":       summary,
				"files_changed": allFilesChanged,
			})
			_ = s.agentSessions.Finish(context.Background(), sessionID, "done", "Max tokens reached: "+summary, allFilesChanged, totalMessages)
			return
		}

		// End turn
		if msg.StopReason == "end_turn" || len(toolUses) == 0 {
			summary := textBuf.String()
			if len(summary) > 200 {
				summary = summary[:200] + "..."
			}
			sseWrite(w, "done", map[string]any{
				"type":          "done",
				"summary":       summary,
				"files_changed": allFilesChanged,
			})
			_ = s.agentSessions.Finish(context.Background(), sessionID, "done", summary, allFilesChanged, totalMessages)
			return
		}

		// Execute tools
		var toolResultBlocks []anthropic.ContentBlockParamUnion
		for _, toolBlock := range toolUses {
			sseWrite(w, "tool_start", map[string]any{
				"type":  "tool_start",
				"id":    toolBlock.ID,
				"tool":  toolBlock.Name,
				"input": toolBlock.Input,
			})

			result := s.executeAgentTool(ctx, domain, user.Email, toolBlock.ID, toolBlock.Name, toolBlock.Input)

			preview := result.Content
			if len(preview) > 300 {
				preview = preview[:300] + "..."
			}
			sseWrite(w, "tool_done", map[string]any{
				"type":    "tool_done",
				"id":      toolBlock.ID,
				"tool":    toolBlock.Name,
				"preview": preview,
				"error":   result.IsError,
			})

			if result.FileChanged != nil {
				allFilesChanged = append(allFilesChanged, result.FileChanged.Path)
				sseWrite(w, "file_changed", map[string]any{
					"type":   "file_changed",
					"path":   result.FileChanged.Path,
					"action": result.FileChanged.Action,
				})
			}

			toolResultBlocks = append(toolResultBlocks, anthropic.NewToolResultBlock(toolBlock.ID, result.Content, result.IsError))
		}

		// Append assistant message + tool results to conversation
		messages = append(messages, msg.ToParam())
		messages = append(messages, anthropic.NewUserMessage(toolResultBlocks...))
	}

	// Max iterations reached
	sseWrite(w, "done", map[string]any{
		"type":          "done",
		"summary":       "Reached maximum iterations",
		"files_changed": allFilesChanged,
	})
	_ = s.agentSessions.Finish(context.Background(), sessionID, "done", "Reached max iterations", allFilesChanged, totalMessages)
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
10. Explain what you are doing while you work
11. IMPORTANT: When editing an existing file, ALWAYS use patch_file (not write_file) to make targeted changes. Only use write_file when creating a brand new file. patch_file is faster, safer, and avoids rewriting large files.`,
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
