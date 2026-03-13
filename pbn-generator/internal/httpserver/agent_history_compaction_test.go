package httpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

// makeReadFileToolUseMsg builds an assistant message that called read_file.
func makeReadFileToolUseMsg(id, path string) anthropic.MessageParam {
	input, _ := json.Marshal(map[string]interface{}{"path": path})
	return anthropic.MessageParam{
		Role: anthropic.MessageParamRoleAssistant,
		Content: []anthropic.ContentBlockParamUnion{
			{OfToolUse: &anthropic.ToolUseBlockParam{
				ID:    id,
				Name:  "read_file",
				Input: json.RawMessage(input),
			}},
		},
	}
}

// makeToolResultMsgWithID builds a user message with a tool_result tied to a specific tool_use ID.
func makeToolResultMsgWithID(toolUseID, content string) anthropic.MessageParam {
	return anthropic.NewUserMessage(
		anthropic.NewToolResultBlock(toolUseID, content, false),
	)
}

// makeToolResultMsg builds a user message with a tool_result (generic tool_use id).
func makeToolResultMsg(content string) anthropic.MessageParam {
	return makeToolResultMsgWithID("tool-id", content)
}

func makeWriteFileToolUseMsg(path, content string) anthropic.MessageParam {
	input, _ := json.Marshal(map[string]interface{}{
		"path":    path,
		"content": content,
	})
	return anthropic.MessageParam{
		Role: anthropic.MessageParamRoleAssistant,
		Content: []anthropic.ContentBlockParamUnion{
			{OfToolUse: &anthropic.ToolUseBlockParam{
				ID:    "tu-1",
				Name:  "write_file",
				Input: json.RawMessage(input),
			}},
		},
	}
}

func makePatchFileToolUseMsg(path, oldText, newText string) anthropic.MessageParam {
	input, _ := json.Marshal(map[string]interface{}{
		"path":     path,
		"old_text": oldText,
		"new_text": newText,
	})
	return anthropic.MessageParam{
		Role: anthropic.MessageParamRoleAssistant,
		Content: []anthropic.ContentBlockParamUnion{
			{OfToolUse: &anthropic.ToolUseBlockParam{
				ID:    "tu-2",
				Name:  "patch_file",
				Input: json.RawMessage(input),
			}},
		},
	}
}

// ─── Fix 1: deep copy — original must not be mutated ─────────────────────────

// TestCompactAgentHistory_DoesNotMutateOriginal verifies that compactAgentHistory
// returns a copy and leaves the original messages slice unchanged.
func TestCompactAgentHistory_DoesNotMutateOriginal(t *testing.T) {
	large := strings.Repeat("x", 10000)
	// Build: read_file tool_use + matching tool_result (old), then 4 recent.
	msgs := []anthropic.MessageParam{
		makeReadFileToolUseMsg("rf-1", "index.html"),  // old assistant
		makeToolResultMsgWithID("rf-1", large),        // old user
		makeToolResultMsg("tiny1"),                     // recent
		makeToolResultMsg("tiny2"),                     // recent
		makeToolResultMsg("tiny3"),                     // recent
		makeToolResultMsg("tiny4"),                     // recent
	}

	// Make a copy of the original text before compaction.
	originalText := msgs[1].Content[0].OfToolResult.Content[0].OfText.Text

	out := compactAgentHistory(msgs, 100, 2)

	// Original must be unchanged.
	afterText := msgs[1].Content[0].OfToolResult.Content[0].OfText.Text
	if afterText != originalText {
		t.Errorf("original messages were mutated: len changed from %d to %d", len(originalText), len(afterText))
	}

	// Returned copy must be compacted.
	outText := out[1].Content[0].OfToolResult.Content[0].OfText.Text
	if len(outText) >= len(large) {
		t.Errorf("compacted copy was not compacted: len=%d", len(outText))
	}
}

// ─── Fix 3: tool-aware compaction ─────────────────────────────────────────────

// TestCompactAgentHistory_ReadFileResultCompacted verifies that read_file results
// are aggressively compacted at the normal threshold.
func TestCompactAgentHistory_ReadFileResultCompacted(t *testing.T) {
	large := strings.Repeat("a", 5000)
	msgs := []anthropic.MessageParam{
		makeReadFileToolUseMsg("rf-1", "index.html"), // old assistant
		makeToolResultMsgWithID("rf-1", large),       // old user
		makeToolResultMsg("recent1"),                  // recent
		makeToolResultMsg("recent2"),                  // recent
		makeToolResultMsg("recent3"),                  // recent
		makeToolResultMsg("recent4"),                  // recent
	}
	out := compactAgentHistory(msgs, 4000, 2) // cutoff=2

	resultText := out[1].Content[0].OfToolResult.Content[0].OfText.Text
	if len(resultText) >= len(large) {
		t.Errorf("read_file result should be compacted, got len=%d", len(resultText))
	}
	if !strings.Contains(resultText, "file content omitted") {
		t.Errorf("read_file placeholder should say 'file content omitted', got: %q", resultText)
	}
}

// TestCompactAgentHistory_NonReadFileResultNotAggressivelyCompacted verifies that
// tool_results from non-read_file tools (e.g. search_in_files) are only compacted
// at the conservative threshold, not at the aggressive read_file threshold.
func TestCompactAgentHistory_NonReadFileResultNotAggressivelyCompacted(t *testing.T) {
	// 8000 chars: above aggressive threshold (4000) but below conservative (20000).
	mediumContent := strings.Repeat("s", 8000)

	// Simulate a search_in_files tool_use (id="sf-1") followed by its result.
	searchToolUseInput, _ := json.Marshal(map[string]interface{}{"query": "foo"})
	msgs := []anthropic.MessageParam{
		{
			Role: anthropic.MessageParamRoleAssistant,
			Content: []anthropic.ContentBlockParamUnion{
				{OfToolUse: &anthropic.ToolUseBlockParam{
					ID:    "sf-1",
					Name:  "search_in_files",
					Input: json.RawMessage(searchToolUseInput),
				}},
			},
		},
		makeToolResultMsgWithID("sf-1", mediumContent), // old user
		makeToolResultMsg("recent1"),                    // recent
		makeToolResultMsg("recent2"),                    // recent
		makeToolResultMsg("recent3"),                    // recent
		makeToolResultMsg("recent4"),                    // recent
	}
	out := compactAgentHistory(msgs, 4000, 2) // cutoff=2

	resultText := out[1].Content[0].OfToolResult.Content[0].OfText.Text
	// 8000 chars is below the conservative threshold (20000), so should be UNTOUCHED.
	if len(resultText) != len(mediumContent) {
		t.Errorf("search_in_files result (8000 chars) should not be compacted at aggressive threshold, got len=%d", len(resultText))
	}
}

// ─── compactAgentHistory general ─────────────────────────────────────────────

// TestCompactAgentHistory_ShortHistoryUnchanged verifies that a history shorter
// than keepRecentTurns*2 is returned untouched.
func TestCompactAgentHistory_ShortHistoryUnchanged(t *testing.T) {
	large := strings.Repeat("x", 10000)
	msgs := []anthropic.MessageParam{
		makeToolResultMsg(large),
		makeToolResultMsg(large),
	}
	out := compactAgentHistory(msgs, 100, 2)
	block := out[0].Content[0].OfToolResult
	if block == nil {
		t.Fatal("expected tool_result block")
	}
	if text := block.Content[0].OfText.Text; len(text) != len(large) {
		t.Errorf("short history should be untouched, got len=%d", len(text))
	}
}

// TestCompactAgentHistory_OldToolResultCompacted verifies old tool_result blocks
// (with unknown/generic tool_use IDs) use the conservative threshold.
func TestCompactAgentHistory_OldToolResultCompacted(t *testing.T) {
	// Use content above conservative threshold (>20000) to confirm it gets compacted.
	veryLarge := strings.Repeat("y", 25000)
	msgs := []anthropic.MessageParam{
		makeToolResultMsg(veryLarge), // old (unknown tool_use ID "tool-id")
		makeToolResultMsg("recent1"), // recent
		makeToolResultMsg("recent2"), // recent
		makeToolResultMsg("recent3"), // recent
		makeToolResultMsg("recent4"), // recent
	}
	out := compactAgentHistory(msgs, 4000, 2) // cutoff=1

	block := out[0].Content[0].OfToolResult
	if block == nil {
		t.Fatal("expected tool_result block")
	}
	text := block.Content[0].OfText.Text
	if len(text) >= len(veryLarge) {
		t.Errorf("very large unknown-tool result should be compacted, got len=%d", len(text))
	}
}

// TestCompactAgentHistory_HashDedup verifies identical content produces same placeholder.
func TestCompactAgentHistory_HashDedup(t *testing.T) {
	large := strings.Repeat("z", 5000)
	// 3 identical read_file results, then 4 recent.
	msgs := []anthropic.MessageParam{
		makeReadFileToolUseMsg("rf-1", "a.html"),
		makeToolResultMsgWithID("rf-1", large),
		makeReadFileToolUseMsg("rf-2", "a.html"),
		makeToolResultMsgWithID("rf-2", large),
		makeReadFileToolUseMsg("rf-3", "a.html"),
		makeToolResultMsgWithID("rf-3", large),
		makeToolResultMsg("recent1"),
		makeToolResultMsg("recent2"),
		makeToolResultMsg("recent3"),
		makeToolResultMsg("recent4"),
	}
	out := compactAgentHistory(msgs, 100, 2) // cutoff=6

	placeholders := make([]string, 3)
	for i, idx := range []int{1, 3, 5} {
		block := out[idx].Content[0].OfToolResult
		if block == nil {
			t.Fatalf("msg[%d]: expected tool_result block", idx)
		}
		placeholders[i] = block.Content[0].OfText.Text
	}
	if placeholders[0] != placeholders[1] || placeholders[1] != placeholders[2] {
		t.Errorf("identical content should produce same placeholder, got: %v", placeholders)
	}
	if !strings.Contains(placeholders[0], "ref:") {
		t.Errorf("placeholder should contain hash ref: %q", placeholders[0])
	}
}

// TestCompactAgentHistory_WriteFileInputCompacted verifies write_file tool_use
// inputs have their content field replaced.
func TestCompactAgentHistory_WriteFileInputCompacted(t *testing.T) {
	largeContent := strings.Repeat("w", 3000)
	msgs := []anthropic.MessageParam{
		makeWriteFileToolUseMsg("index.html", largeContent), // old
		makeToolResultMsg("ok"),                             // recent
		makeToolResultMsg("ok"),                             // recent
		makeToolResultMsg("ok"),                             // recent
		makeToolResultMsg("ok"),                             // recent
	}
	out := compactAgentHistory(msgs, 4000, 2) // cutoff=1

	toolUse := out[0].Content[0].OfToolUse
	if toolUse == nil {
		t.Fatal("expected tool_use block")
	}
	raw, ok := toolUse.Input.(json.RawMessage)
	if !ok {
		t.Fatal("expected json.RawMessage input")
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	content, _ := m["content"].(string)
	if content == largeContent {
		t.Error("write_file content should have been compacted")
	}
	if !strings.Contains(content, "compacted write_file") {
		t.Errorf("write_file content placeholder unexpected: %q", content)
	}
	if m["path"] != "index.html" {
		t.Errorf("path should be preserved, got: %v", m["path"])
	}
}

// TestCompactAgentHistory_PatchFileInputCompacted verifies patch_file tool_use
// inputs have old_text and new_text replaced.
func TestCompactAgentHistory_PatchFileInputCompacted(t *testing.T) {
	largeOld := strings.Repeat("o", 1000)
	largeNew := strings.Repeat("n", 1000)
	msgs := []anthropic.MessageParam{
		makePatchFileToolUseMsg("style.css", largeOld, largeNew), // old
		makeToolResultMsg("ok"),                                   // recent
		makeToolResultMsg("ok"),                                   // recent
		makeToolResultMsg("ok"),                                   // recent
		makeToolResultMsg("ok"),                                   // recent
	}
	out := compactAgentHistory(msgs, 4000, 2) // cutoff=1

	toolUse := out[0].Content[0].OfToolUse
	if toolUse == nil {
		t.Fatal("expected tool_use block")
	}
	raw, _ := toolUse.Input.(json.RawMessage)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)

	if m["old_text"] == largeOld {
		t.Error("old_text should have been compacted")
	}
	if m["new_text"] == largeNew {
		t.Error("new_text should have been compacted")
	}
	if !strings.Contains(m["old_text"].(string), "compacted old_text") {
		t.Errorf("old_text placeholder unexpected: %v", m["old_text"])
	}
	if !strings.Contains(m["new_text"].(string), "compacted new_text") {
		t.Errorf("new_text placeholder unexpected: %v", m["new_text"])
	}
}

// TestCompactAgentHistory_SmallFieldsUntouched verifies that fields below
// compactToolUseThreshold are not modified.
func TestCompactAgentHistory_SmallFieldsUntouched(t *testing.T) {
	smallContent := "small content"
	msgs := []anthropic.MessageParam{
		makeWriteFileToolUseMsg("index.html", smallContent),
		makeToolResultMsg("ok"),
		makeToolResultMsg("ok"),
		makeToolResultMsg("ok"),
		makeToolResultMsg("ok"),
	}
	out := compactAgentHistory(msgs, 4000, 2)
	toolUse := out[0].Content[0].OfToolUse
	if toolUse == nil {
		t.Fatal("expected tool_use block")
	}
	m := toolUseInputToMap(t, toolUse)
	if m["content"] != smallContent {
		t.Errorf("small content should be untouched, got: %v", m["content"])
	}
}

// toolUseInputToMap converts a ToolUseBlockParam.Input (json.RawMessage or
// map[string]interface{}) to map[string]interface{} for test assertions.
// After deep copy, unmodified inputs may still be map[string]interface{};
// after compaction they are re-encoded as json.RawMessage.
func toolUseInputToMap(t *testing.T, toolUse *anthropic.ToolUseBlockParam) map[string]interface{} {
	t.Helper()
	switch v := toolUse.Input.(type) {
	case json.RawMessage:
		var m map[string]interface{}
		if err := json.Unmarshal(v, &m); err != nil {
			t.Fatalf("toolUseInputToMap: json.Unmarshal: %v", err)
		}
		return m
	case map[string]interface{}:
		return v
	default:
		t.Fatalf("toolUseInputToMap: unexpected Input type %T", toolUse.Input)
		return nil
	}
}

// ─── deepCopyMessages ────────────────────────────────────────────────────────

func TestDeepCopyMessages_Independence(t *testing.T) {
	large := strings.Repeat("q", 1000)
	msgs := []anthropic.MessageParam{makeToolResultMsg(large)}
	copied := deepCopyMessages(msgs)
	if &copied[0] == &msgs[0] {
		t.Error("deepCopyMessages must return new slice, not same pointer")
	}
	// Mutate copy, original must be unchanged.
	copied[0].Content[0].OfToolResult.Content[0].OfText.Text = "mutated"
	if msgs[0].Content[0].OfToolResult.Content[0].OfText.Text != large {
		t.Error("mutating copy should not affect original")
	}
}

// ─── buildToolNameMap ────────────────────────────────────────────────────────

func TestBuildToolNameMap(t *testing.T) {
	msgs := []anthropic.MessageParam{
		makeReadFileToolUseMsg("id-1", "index.html"),
		makeWriteFileToolUseMsg("out.html", "content"),
	}
	m := buildToolNameMap(msgs)
	if m["id-1"] != "read_file" {
		t.Errorf("expected read_file for id-1, got %q", m["id-1"])
	}
	if m["tu-1"] != "write_file" {
		t.Errorf("expected write_file for tu-1, got %q", m["tu-1"])
	}
}

// ─── contentHash ─────────────────────────────────────────────────────────────

func TestContentHash_Deterministic(t *testing.T) {
	h1 := contentHash("hello world")
	h2 := contentHash("hello world")
	if h1 != h2 {
		t.Errorf("hash should be deterministic: %q vs %q", h1, h2)
	}
	h3 := contentHash("different")
	if h1 == h3 {
		t.Error("different inputs should produce different hashes")
	}
}

func TestContentHash_Length(t *testing.T) {
	h := contentHash("test")
	if len(h) != 8 {
		t.Errorf("expected 8-char hex hash, got len=%d: %q", len(h), h)
	}
}
