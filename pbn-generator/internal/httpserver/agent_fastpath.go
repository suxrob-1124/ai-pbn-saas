package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/google/uuid"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// ─── Op types ─────────────────────────────────────────────────────────────────

const (
	fpOpRemoveHTMLComments = "remove_html_comments"
	fpOpRemoveCSSComments  = "remove_css_comments"
	fpOpReplaceAll         = "replace_all"
	fpOpRegexReplace       = "regex_replace"
	fpOpUpdateYear         = "update_year"
)

// fastPathOp describes a recognised deterministic single-file operation.
type fastPathOp struct {
	Type string // one of fpOp* constants
	File string // target file path (always set)
	From string // replace_all: old text; regex_replace: pattern; update_year: old year
	To   string // replace_all: new text; regex_replace: replacement; update_year: new year
}

// fastPathExecResult holds the output of a deterministic transformation.
type fastPathExecResult struct {
	Content []byte
	Changed bool
	Summary string
	Count   int // items changed (removals / replacements)
}

// ─── Intent-matching regexes ──────────────────────────────────────────────────
// htmlCommentRe and cssCommentRe are defined in agent_postconditions.go.

var (
	removeVerbRe  = regexp.MustCompile(`(?i)(?:\b(?:remove|delete|strip)\b|удали|убери|очисти)`)
	commentKeyRe  = regexp.MustCompile(`(?i)(?:\bcomments?\b|коммент\S*)`)
	multiFileRe   = regexp.MustCompile(`(?i)(?:\b(?:all\s+files?|all\s+pages?|everywhere|across|every\s+file)\b|все[хмх]?\s+файл|каждый\s+файл|все\s+страниц)`)
	replaceAllRe  = regexp.MustCompile(`(?i)(?:replace\s+all|замени\s+все?)\s+["']?(.+?)["']?\s+(?:with|на|->)\s+["']?(.+?)["']?\s*$`)
	regexReplRe   = regexp.MustCompile(`(?i)(?:regex|regexp)\s+(?:replace|заменить)\s+[/"']?(.+?)[/"']?\s+(?:with|на|->)\s+[/"']?(.+?)[/"']?\s*$`)
	yearExplicitRe = regexp.MustCompile(`(?i)(?:update|обнови|замени|replace)\s+(?:the\s+)?years?\s+(?:from\s+)?(20\d\d)\s+(?:to|на|with)\s+(20\d\d)`)
	yearReplaceRe  = regexp.MustCompile(`(?i)(?:замени|replace)\s+(20\d\d)\s+(?:на|with|to|->)\s+(20\d\d)`)
	yearImplicitRe = regexp.MustCompile(`(?i)(?:update\s+(?:the\s+)?year|обнови\s+год|замени\s+год)\s*$`)
)

// ─── Matcher ──────────────────────────────────────────────────────────────────

// matchFastPath returns a fastPathOp if the message describes a supported
// deterministic single-file task and contextFile is known.
// Returns nil, false whenever the request is ambiguous or out of scope.
func matchFastPath(msg, contextFile string) (*fastPathOp, bool) {
	if contextFile == "" {
		return nil, false
	}

	lower := strings.ToLower(strings.TrimSpace(msg))

	// Block multi-file requests.
	if multiFileRe.MatchString(lower) {
		return nil, false
	}

	if matchesHTMLCommentIntent(lower, contextFile) {
		return &fastPathOp{Type: fpOpRemoveHTMLComments, File: contextFile}, true
	}

	if matchesCSSCommentIntent(lower, contextFile) {
		return &fastPathOp{Type: fpOpRemoveCSSComments, File: contextFile}, true
	}

	// update_year before replace_all (more specific).
	if op := matchUpdateYear(lower, contextFile); op != nil {
		return op, true
	}

	if op := matchReplaceAll(msg, contextFile); op != nil {
		return op, true
	}

	if op := matchRegexReplace(msg, contextFile); op != nil {
		return op, true
	}

	return nil, false
}

func matchesHTMLCommentIntent(lower, file string) bool {
	if !removeVerbRe.MatchString(lower) || !commentKeyRe.MatchString(lower) {
		return false
	}
	if strings.Contains(lower, "html") {
		return true
	}
	// Infer from file extension when unambiguous.
	return strings.ToLower(path.Ext(file)) == ".html" && !strings.Contains(lower, "css")
}

func matchesCSSCommentIntent(lower, file string) bool {
	if !removeVerbRe.MatchString(lower) || !commentKeyRe.MatchString(lower) {
		return false
	}
	if strings.Contains(lower, "css") {
		return true
	}
	return strings.ToLower(path.Ext(file)) == ".css" && !strings.Contains(lower, "html")
}

func matchReplaceAll(msg, file string) *fastPathOp {
	m := replaceAllRe.FindStringSubmatch(msg)
	if m == nil {
		return nil
	}
	from := strings.Trim(m[1], `"' `)
	to := strings.Trim(m[2], `"' `)
	if from == "" {
		return nil
	}
	return &fastPathOp{Type: fpOpReplaceAll, File: file, From: from, To: to}
}

func matchRegexReplace(msg, file string) *fastPathOp {
	m := regexReplRe.FindStringSubmatch(msg)
	if m == nil {
		return nil
	}
	pattern := strings.Trim(m[1], `"'/ `)
	repl := strings.Trim(m[2], `"'/ `)
	if pattern == "" {
		return nil
	}
	// Reject if the pattern doesn't compile.
	if _, err := regexp.Compile(pattern); err != nil {
		return nil
	}
	return &fastPathOp{Type: fpOpRegexReplace, File: file, From: pattern, To: repl}
}

func matchUpdateYear(lower, file string) *fastPathOp {
	if m := yearExplicitRe.FindStringSubmatch(lower); m != nil {
		return &fastPathOp{Type: fpOpUpdateYear, File: file, From: m[1], To: m[2]}
	}
	if m := yearReplaceRe.FindStringSubmatch(lower); m != nil {
		return &fastPathOp{Type: fpOpUpdateYear, File: file, From: m[1], To: m[2]}
	}
	if yearImplicitRe.MatchString(lower) {
		thisYear := strconv.Itoa(time.Now().Year())
		lastYear := strconv.Itoa(time.Now().Year() - 1)
		return &fastPathOp{Type: fpOpUpdateYear, File: file, From: lastYear, To: thisYear}
	}
	return nil
}

// ─── Executors ────────────────────────────────────────────────────────────────

// executeFastPathOp applies the deterministic transformation described by op.
func executeFastPathOp(op *fastPathOp, content []byte) (fastPathExecResult, error) {
	switch op.Type {
	case fpOpRemoveHTMLComments:
		return execRemoveHTMLComments(content), nil
	case fpOpRemoveCSSComments:
		return execRemoveCSSComments(content), nil
	case fpOpReplaceAll, fpOpUpdateYear:
		return execReplaceAll(content, op.From, op.To), nil
	case fpOpRegexReplace:
		return execRegexReplace(content, op.From, op.To)
	default:
		return fastPathExecResult{}, fmt.Errorf("unknown fast path op: %s", op.Type)
	}
}

func execRemoveHTMLComments(content []byte) fastPathExecResult {
	count := len(htmlCommentRe.FindAll(content, -1))
	if count == 0 {
		return fastPathExecResult{Content: content, Changed: false, Summary: "No HTML comments found.", Count: 0}
	}
	result := htmlCommentRe.ReplaceAll(content, nil)
	return fastPathExecResult{
		Content: result,
		Changed: !bytes.Equal(result, content),
		Summary: fmt.Sprintf("Removed %d HTML comment(s) from %q.", count, "file"),
		Count:   count,
	}
}

func execRemoveCSSComments(content []byte) fastPathExecResult {
	count := len(cssCommentRe.FindAll(content, -1))
	if count == 0 {
		return fastPathExecResult{Content: content, Changed: false, Summary: "No CSS comments found.", Count: 0}
	}
	result := cssCommentRe.ReplaceAll(content, nil)
	return fastPathExecResult{
		Content: result,
		Changed: !bytes.Equal(result, content),
		Summary: fmt.Sprintf("Removed %d CSS comment(s) from %q.", count, "file"),
		Count:   count,
	}
}

func execReplaceAll(content []byte, from, to string) fastPathExecResult {
	if from == "" {
		return fastPathExecResult{Content: content, Changed: false, Summary: "Empty search string — nothing to replace.", Count: 0}
	}
	count := bytes.Count(content, []byte(from))
	if count == 0 {
		return fastPathExecResult{Content: content, Changed: false, Summary: fmt.Sprintf("No occurrences of %q found.", from), Count: 0}
	}
	result := bytes.ReplaceAll(content, []byte(from), []byte(to))
	return fastPathExecResult{
		Content: result,
		Changed: true,
		Summary: fmt.Sprintf("Replaced %d occurrence(s) of %q with %q.", count, from, to),
		Count:   count,
	}
}

func execRegexReplace(content []byte, pattern, repl string) (fastPathExecResult, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fastPathExecResult{}, fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	count := len(re.FindAll(content, -1))
	if count == 0 {
		return fastPathExecResult{Content: content, Changed: false, Summary: fmt.Sprintf("Pattern /%s/ matched nothing.", pattern), Count: 0}, nil
	}
	result := re.ReplaceAll(content, []byte(repl))
	return fastPathExecResult{
		Content: result,
		Changed: true,
		Summary: fmt.Sprintf("Regex replaced %d match(es) of /%s/ with %q.", count, pattern, repl),
		Count:   count,
	}, nil
}

// ─── Verification ─────────────────────────────────────────────────────────────

// verifyFastPathOp checks that the postcondition holds after transformation.
// For remove operations: no matches remain. For replacements: trusted to be correct.
func verifyFastPathOp(op *fastPathOp, result []byte) bool {
	switch op.Type {
	case fpOpRemoveHTMLComments:
		return !htmlCommentRe.Match(result)
	case fpOpRemoveCSSComments:
		return !cssCommentRe.Match(result)
	case fpOpUpdateYear:
		if op.From == "" || op.From == op.To {
			return true
		}
		return !bytes.Contains(result, []byte(op.From))
	default:
		// replace_all, regex_replace: ReplaceAll/ReplaceAll are deterministic;
		// from may appear in `to` (e.g. replace "a" with "aa"), so we don't
		// re-check for absence. Trust the executor.
		return true
	}
}

// ─── Fast path runner ─────────────────────────────────────────────────────────

// runFastPath executes a deterministic single-file operation without calling
// Anthropic. It mirrors runAgentLoop's lifecycle: same defer for semaphore
// release, hub markDone, session finish, diagnostics save.
func (s *Server) runFastPath(
	sessionID, domainID string,
	domain sqlstore.Domain,
	userEmail string,
	op *fastPathOp,
	chatLog []chatLogEntry,
	snapshotTag string,
	initialEventSeq int,
	hs *hubSession,
	messages []anthropic.MessageParam, // full history including the new user turn
) {
	baselined := make(map[string]bool)

	defer func() {
		globalAgentHub.releaseSem()
		hs.markDone()
		go func() {
			time.Sleep(30 * time.Second)
			globalAgentHub.unregister(sessionID, domainID)
		}()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	publish := func(eventType string, payload any) {
		hs.publish(eventType, payload)
	}

	saveDiag := func(hit bool, snapshotFiles int) {
		type diagShape struct {
			Mode          string `json:"mode"`
			Iterations    int    `json:"iterations"`
			CompactedBytes int   `json:"compacted_bytes"`
			SnapshotFiles int    `json:"snapshot_files"`
			FastPathHit   bool   `json:"fast_path_hit"`
		}
		d := diagShape{
			Mode:          "fast_path",
			Iterations:    0,
			CompactedBytes: 0,
			SnapshotFiles: snapshotFiles,
			FastPathHit:   hit,
		}
		if b, err := json.Marshal(d); err == nil {
			hs.setLiveDiag(b)
			_ = s.agentSessions.SaveDiagnostics(ctx, sessionID, b)
		}
	}

	finishErr := func(summary string) {
		_ = s.agentSessions.Finish(ctx, sessionID, "error", summary, nil, 1)
		saveDiag(false, len(baselined))
	}

	eventSeq := initialEventSeq

	// Persist user message as event (same position as runAgentLoop).
	if len(chatLog) > 0 {
		if b, err := json.Marshal(chatLog[len(chatLog)-1]); err == nil {
			_ = s.agentSessions.AppendEvent(ctx, sessionID, eventSeq, "chat_entry", b)
			eventSeq++
		}
	}

	// Read file.
	content, err := s.readDomainFileBytesFromBackend(ctx, domain, op.File)
	if err != nil {
		publish("error", map[string]any{
			"type":    "error",
			"message": fmt.Sprintf("Cannot read file %q: %v", op.File, err),
		})
		finishErr(fmt.Sprintf("fast path: cannot read %q", op.File))
		return
	}

	// Execute transformation.
	result, execErr := executeFastPathOp(op, content)
	if execErr != nil {
		publish("error", map[string]any{"type": "error", "message": execErr.Error()})
		finishErr("fast path: execution error: " + execErr.Error())
		return
	}

	// Verify postcondition.
	if !verifyFastPathOp(op, result.Content) {
		publish("error", map[string]any{
			"type":    "error",
			"message": "Fast path verification failed — task could not be completed deterministically. Please try again or describe the change in more detail.",
		})
		finishErr("fast path: postcondition not met")
		return
	}

	var filesChanged []string

	if result.Changed {
		// Baseline the file before mutating (supports rollback, same as agent tools).
		if !s.ensureAgentBaselineForPath(ctx, domain, sessionID, op.File, userEmail, baselined) {
			publish("error", map[string]any{"type": "error", "message": fmt.Sprintf("Cannot write %q: failed to save rollback baseline", op.File)})
			finishErr("fast path: baseline failed for " + op.File)
			return
		}

		// Write via the safe existing path (revision + DB + backend).
		if err := s.agentUpsertSiteFile(ctx, domain, op.File, result.Content, userEmail, false); err != nil {
			publish("error", map[string]any{"type": "error", "message": fmt.Sprintf("Write failed: %v", err)})
			finishErr("fast path: write error: " + err.Error())
			return
		}
		filesChanged = []string{op.File}
		publish("file_changed", map[string]any{
			"type":   "file_changed",
			"path":   op.File,
			"action": "updated",
		})
	}

	// Build assistant chat log entry.
	asstEntry := chatLogEntry{
		ID:     uuid.New().String(),
		Role:   "assistant",
		Status: "done",
		Text:   result.Summary,
	}
	chatLog = append(chatLog, asstEntry)

	// Persist assistant event.
	if b, err := json.Marshal(asstEntry); err == nil {
		_ = s.agentSessions.AppendEvent(ctx, sessionID, eventSeq, "chat_entry", b)
	}

	// Stream summary text.
	publish("text", map[string]string{"type": "text", "delta": result.Summary})

	// Save diagnostics.
	saveDiag(true, len(baselined))

	// Finish session.
	_ = s.agentSessions.Finish(ctx, sessionID, "done", result.Summary, filesChanged, 1)

	// Save chat log + messages checkpoint.
	// Append a synthetic assistant turn so a subsequent LLM resume has full context
	// of what the fast path did (user request + assistant summary).
	fullMessages := append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(result.Summary)))
	msgsJSON, _ := json.Marshal(fullMessages)
	if cl, err := json.Marshal(chatLog); err == nil {
		_ = s.agentSessions.SaveMessages(ctx, sessionID, msgsJSON, cl)
	}

	// Done event.
	publish("done", map[string]any{
		"type":               "done",
		"summary":            result.Summary,
		"files_changed":      filesChanged,
		"rollback_available": snapshotTag != "" && len(filesChanged) > 0,
	})
}
