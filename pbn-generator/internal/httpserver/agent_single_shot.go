package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/google/uuid"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// ─── Single-file detector ─────────────────────────────────────────────────────

// singleShotBlockRe matches messages that require site-wide traversal or research,
// which cannot be handled by a single LLM call without file-listing/search tools.
var singleShotBlockRe = regexp.MustCompile(
	`(?i)\b(?:search|find\s+all|audit|scan|analyze|analyse|review\s+all|list\s+all|check\s+all)\b`,
)

// matchSingleShot returns true when the request is appropriate for single-shot edit
// mode: there is an explicit context file, the task is not multi-file, and it does
// not require cross-file research. Conservative by design — when in doubt, returns
// false and the caller falls through to the full agent loop.
func matchSingleShot(msg, contextFile string) bool {
	if contextFile == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(msg))
	// Block requests that explicitly span multiple files (reuse fast-path regex).
	if multiFileRe.MatchString(lower) {
		return false
	}
	// Block requests that require site-wide search / audit / research tooling.
	if singleShotBlockRe.MatchString(lower) {
		return false
	}
	return true
}

// ─── Output contract ──────────────────────────────────────────────────────────

// singleShotPayload is the structured response the model must return via the
// edit_file tool. Path must match the target file; content is the full updated
// file content; summary is a short human-readable description.
type singleShotPayload struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Summary string `json:"summary"`
}

func validateSingleShotPayload(p singleShotPayload, expectedPath string) error {
	if p.Path != expectedPath {
		return fmt.Errorf("path mismatch: expected %q, got %q", expectedPath, p.Path)
	}
	if strings.TrimSpace(p.Content) == "" {
		return fmt.Errorf("content is empty")
	}
	if strings.TrimSpace(p.Summary) == "" {
		return fmt.Errorf("summary is empty")
	}
	return nil
}

// ─── edit_file tool definition ────────────────────────────────────────────────

func singleShotTool() anthropic.ToolParam {
	return anthropic.ToolParam{
		Name:        "edit_file",
		Description: anthropic.String("Replace the entire content of the specified file with the updated version"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The file path being edited (must exactly match the target file path)",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The complete updated file content (full file, not a diff or patch)",
				},
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "Brief description of what was changed (1-2 sentences)",
				},
			},
			Required: []string{"path", "content", "summary"},
		},
	}
}

// ─── Single-shot runner ───────────────────────────────────────────────────────

// runSingleShot executes a single-shot LLM edit:
//   - 1 LLM call (+ optional 1 repair attempt)
//   - 1 write via the safe existing path
//   - no tool loop, no read_file / patch_file / list_files
//
// If the model returns an invalid response and a single repair attempt also fails,
// runSingleShot falls back to the full agent loop. The fallback flag suppresses
// the defer cleanup so runAgentLoop's own defer handles semaphore and hub teardown.
func (s *Server) runSingleShot(
	sessionID, domainID string,
	domain sqlstore.Domain,
	userEmail string,
	contextFile string,
	userMessage string, // original req.Message (without file-content prefix)
	pc agentPostcondition,
	messages []anthropic.MessageParam, // full history including new user turn
	chatLog []chatLogEntry,
	snapshotTag string,
	initialEventSeq int,
	hs *hubSession,
) {
	baselined := make(map[string]bool)
	fallback := false // set to true when handing off to runAgentLoop

	defer func() {
		if !fallback {
			globalAgentHub.releaseSem()
			hs.markDone()
			go func() {
				time.Sleep(30 * time.Second)
				globalAgentHub.unregister(sessionID, domainID)
			}()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	publish := func(eventType string, payload any) {
		hs.publish(eventType, payload)
	}

	type diagShape struct {
		Mode          string `json:"mode"`
		Iterations    int    `json:"iterations"`
		CompactedBytes int   `json:"compacted_bytes"`
		SnapshotFiles  int   `json:"snapshot_files"`
		FastPathHit    bool  `json:"fast_path_hit"`
	}
	diag := diagShape{Mode: "single_shot"}

	saveDiag := func(iterations int) {
		diag.Iterations = iterations
		diag.SnapshotFiles = len(baselined)
		if b, err := json.Marshal(diag); err == nil {
			hs.setLiveDiag(b)
			_ = s.agentSessions.SaveDiagnostics(ctx, sessionID, b)
		}
	}

	finishErr := func(summary string) {
		_ = s.agentSessions.Finish(ctx, sessionID, "error", summary, nil, 1)
		saveDiag(1)
	}

	eventSeq := initialEventSeq

	// Persist user turn event (same as fast path and agent loop).
	if len(chatLog) > 0 {
		if b, err := json.Marshal(chatLog[len(chatLog)-1]); err == nil {
			_ = s.agentSessions.AppendEvent(ctx, sessionID, eventSeq, "chat_entry", b)
			eventSeq++
		}
	}

	// Read current file content — server-side, no LLM read_file call.
	fileContent, err := s.readDomainFileBytesFromBackend(ctx, domain, contextFile)
	if err != nil {
		publish("error", map[string]any{"type": "error", "message": fmt.Sprintf("Cannot read %q: %v", contextFile, err)})
		finishErr(fmt.Sprintf("single_shot: cannot read %q: %v", contextFile, err))
		return
	}

	opts := []option.RequestOption{option.WithAPIKey(s.cfg.AnthropicAPIKey)}
	if s.cfg.AnthropicBaseURL != "" {
		opts = append(opts, option.WithBaseURL(s.cfg.AnthropicBaseURL))
	}
	client := anthropic.NewClient(opts...)
	maxTokens := int64(s.cfg.AgentMaxTokens)
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	tool := singleShotTool()
	toolUnion := anthropic.ToolUnionParam{OfTool: &tool}

	systemPrompt := "You are a precise file editor. Apply the user's requested change to the provided file and return the complete updated content via the edit_file tool. Edit only the specified file. Do not add commentary outside the tool call."

	// Build the single-shot user message: task + current file content.
	userText := fmt.Sprintf(
		"Edit the file %q.\n\nCurrent content:\n```\n%s\n```\n\nTask: %s",
		contextFile, string(fileContent), userMessage,
	)
	singleShotMsgs := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userText)),
	}

	// callOnce makes one non-streaming Anthropic call and extracts the edit_file result.
	// Returns (payload, *Message, error). *Message is non-nil on any successful HTTP response.
	callOnce := func(msgs []anthropic.MessageParam) (singleShotPayload, *anthropic.Message, error) {
		iterCtx, iterCancel := context.WithTimeout(ctx, 90*time.Second)
		defer iterCancel()
		msg, err := client.Messages.New(iterCtx, anthropic.MessageNewParams{
			Model:      anthropic.Model(s.cfg.AnthropicModel),
			MaxTokens:  maxTokens,
			System:     []anthropic.TextBlockParam{{Type: "text", Text: systemPrompt}},
			Messages:   msgs,
			Tools:      []anthropic.ToolUnionParam{toolUnion},
			ToolChoice: anthropic.ToolChoiceParamOfTool("edit_file"),
		})
		if err != nil {
			return singleShotPayload{}, nil, fmt.Errorf("LLM call failed: %w", err)
		}
		go s.logAnthropicUsage(context.Background(), userEmail, domain.ProjectID, domainID,
			s.cfg.AnthropicModel, msg.Usage.InputTokens, msg.Usage.OutputTokens)

		for _, block := range msg.Content {
			if tu, ok := block.AsAny().(anthropic.ToolUseBlock); ok && tu.Name == "edit_file" {
				var p singleShotPayload
				if err := json.Unmarshal([]byte(tu.Input), &p); err != nil {
					return singleShotPayload{}, msg, fmt.Errorf("cannot parse tool input: %w", err)
				}
				return p, msg, nil
			}
		}
		return singleShotPayload{}, msg, fmt.Errorf("model did not call edit_file tool")
	}

	// First attempt.
	payload, assistantMsg, callErr := callOnce(singleShotMsgs)
	iterations := 1
	var validErr error
	if callErr == nil {
		validErr = validateSingleShotPayload(payload, contextFile)
	} else {
		validErr = callErr
	}

	// Repair attempt (at most one): only when we got a valid HTTP response but the
	// payload failed validation. Skip repair on network/API errors — fall back directly.
	if validErr != nil && callErr == nil && assistantMsg != nil {
		repairText := fmt.Sprintf(
			"Your previous response was invalid: %v. Call edit_file again with path=%q and the complete corrected file content.",
			validErr, contextFile,
		)
		repairMsgs := append(singleShotMsgs, assistantMsg.ToParam(),
			anthropic.NewUserMessage(anthropic.NewTextBlock(repairText)))
		payload, _, callErr = callOnce(repairMsgs)
		iterations = 2
		if callErr == nil {
			validErr = validateSingleShotPayload(payload, contextFile)
		} else {
			validErr = callErr
		}
	}

	// After max one repair, if still invalid → fall back to full agent loop.
	if validErr != nil {
		saveDiag(iterations)
		fallback = true
		go s.runAgentLoop(sessionID, domainID, domain, userEmail, messages, chatLog, snapshotTag, pc, contextFile, initialEventSeq, hs)
		return
	}

	// Apply the change if content actually differs.
	var filesChanged []string
	newContent := []byte(payload.Content)
	if !bytes.Equal(newContent, fileContent) {
		if !s.ensureAgentBaselineForPath(ctx, domain, sessionID, contextFile, userEmail, baselined) {
			publish("error", map[string]any{"type": "error", "message": fmt.Sprintf("Cannot write %q: failed to save rollback baseline", contextFile)})
			finishErr("single_shot: baseline failed for " + contextFile)
			return
		}
		if err := s.agentUpsertSiteFile(ctx, domain, contextFile, newContent, userEmail, false); err != nil {
			publish("error", map[string]any{"type": "error", "message": fmt.Sprintf("Write failed: %v", err)})
			finishErr("single_shot: write error: " + err.Error())
			return
		}
		filesChanged = []string{contextFile}
		publish("file_changed", map[string]any{
			"type":   "file_changed",
			"path":   contextFile,
			"action": "updated",
		})
	}

	// Build and persist the assistant chat log entry.
	asstEntry := chatLogEntry{
		ID:     uuid.New().String(),
		Role:   "assistant",
		Status: "done",
		Text:   payload.Summary,
	}
	chatLog = append(chatLog, asstEntry)
	if b, err := json.Marshal(asstEntry); err == nil {
		_ = s.agentSessions.AppendEvent(ctx, sessionID, eventSeq, "chat_entry", b)
	}

	// Stream the summary to SSE.
	publish("text", map[string]string{"type": "text", "delta": payload.Summary})

	// Diagnostics.
	saveDiag(iterations)

	// Finish session.
	_ = s.agentSessions.Finish(ctx, sessionID, "done", payload.Summary, filesChanged, 1)

	// Save messages_json: append synthetic assistant summary so a subsequent LLM
	// resume has full context of what this single-shot run accomplished.
	finalMessages := append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(payload.Summary)))
	msgsJSON, _ := json.Marshal(finalMessages)
	if cl, err := json.Marshal(chatLog); err == nil {
		_ = s.agentSessions.SaveMessages(ctx, sessionID, msgsJSON, cl)
	}

	// Done event.
	publish("done", map[string]any{
		"type":               "done",
		"summary":            payload.Summary,
		"files_changed":      filesChanged,
		"rollback_available": snapshotTag != "" && len(filesChanged) > 0,
	})
}
