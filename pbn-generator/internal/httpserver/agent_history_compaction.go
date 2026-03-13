package httpserver

// agent_history_compaction.go — compact old tool_use and tool_result messages
// to reduce token usage across long agent sessions without losing conversation semantics.
//
// Design constraints:
//   - NEVER mutates the canonical messages slice (used for DB storage and resume).
//     Always works on a deep copy returned only to the Anthropic call.
//   - Tool-aware: only read_file tool_results are aggressively compacted with a
//     file-content placeholder.  Results from other tools (search, list) use a
//     conservative fallback threshold to avoid misleading the model.
//   - Hash-based dedup: identical file content (same hash) produces the same
//     placeholder, collapsing N full copies of the same file into one reference.
//   - Everything before the last keepRecentTurns*2 messages is "old history".

import (
	"crypto/md5"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
)

const (
	// compactMaxContentLen is the threshold (bytes) above which read_file tool_result
	// text is replaced with a placeholder.
	compactMaxContentLen = 4000
	// compactUnknownToolThreshold is a conservative threshold for tool_results whose
	// originating tool cannot be identified (e.g. very old history without ID match).
	compactUnknownToolThreshold = 20000
	// compactToolUseThreshold is the minimum field size that triggers tool_use compaction.
	compactToolUseThreshold = 500
	// compactKeepRecentTurns is the number of most-recent message pairs left untouched.
	compactKeepRecentTurns = 2
)

// compactAgentHistory returns a new, compacted copy of messages for use in the
// Anthropic API call.  The original slice is never modified.
//
// keepRecentTurns message pairs (assistant+user) at the tail are left untouched.
func compactAgentHistory(messages []anthropic.MessageParam, maxContentLen, keepRecentTurns int) []anthropic.MessageParam {
	if len(messages) <= keepRecentTurns*2 {
		return messages
	}

	// Deep copy so DB-stored history is never mutated.
	// If copy fails, skip compaction entirely rather than risk mutating canonical history.
	working := deepCopyMessages(messages)
	if working == nil {
		return messages
	}
	cutoff := len(working) - keepRecentTurns*2

	// Build toolUseID → toolName map from old history to enable tool-aware compaction.
	toolNames := buildToolNameMap(working[:cutoff])

	// Shared hash → placeholder for dedup across the entire old history.
	hashes := make(map[string]string)

	for i := 0; i < cutoff; i++ {
		msg := &working[i]
		switch msg.Role {
		case anthropic.MessageParamRoleUser:
			compactToolResultBlocks(msg, maxContentLen, hashes, toolNames)
		case anthropic.MessageParamRoleAssistant:
			compactToolUseInputs(msg, hashes)
		}
	}
	return working
}

// deepCopyMessages returns an independent deep copy via JSON round-trip.
// Returns nil on error so the caller can detect failure and skip compaction
// entirely rather than accidentally mutating the canonical history.
func deepCopyMessages(messages []anthropic.MessageParam) []anthropic.MessageParam {
	if len(messages) == 0 {
		return messages
	}
	data, err := json.Marshal(messages)
	if err != nil {
		return nil // caller must handle nil → skip compaction
	}
	var out []anthropic.MessageParam
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

// buildToolNameMap scans messages and returns a map of toolUseID → toolName.
// Used to identify what tool produced each tool_result block.
func buildToolNameMap(messages []anthropic.MessageParam) map[string]string {
	names := make(map[string]string)
	for _, msg := range messages {
		if msg.Role != anthropic.MessageParamRoleAssistant {
			continue
		}
		for _, block := range msg.Content {
			if block.OfToolUse != nil {
				names[block.OfToolUse.ID] = block.OfToolUse.Name
			}
		}
	}
	return names
}

// compactToolResultBlocks replaces large text inside tool_result blocks.
// Only read_file results are aggressively compacted; other tool results use
// a much more conservative threshold to avoid misleading the model.
func compactToolResultBlocks(msg *anthropic.MessageParam, maxContentLen int, hashes map[string]string, toolNames map[string]string) {
	for j := range msg.Content {
		block := &msg.Content[j]
		if block.OfToolResult == nil {
			continue
		}

		toolName := toolNames[block.OfToolResult.ToolUseID]
		// Determine effective threshold for this tool result.
		threshold := compactUnknownToolThreshold // conservative default
		if toolName == "read_file" {
			threshold = maxContentLen
		} else if toolName != "" {
			// Known non-file tools (write_file confirmation, search_in_files,
			// list_files, delete_file, generate_image) produce small outputs;
			// their results are never large enough to compact, but if somehow
			// they are, use the conservative threshold.
			threshold = compactUnknownToolThreshold
		}

		for k := range block.OfToolResult.Content {
			tb := block.OfToolResult.Content[k].OfText
			if tb == nil || len(tb.Text) <= threshold {
				continue
			}
			hash := contentHash(tb.Text)
			label := "content"
			if toolName == "read_file" {
				label = "file content"
			}
			if placeholder, seen := hashes[hash]; seen {
				tb.Text = placeholder
			} else {
				placeholder = fmt.Sprintf("[%s omitted: %d bytes, ref:%s]", label, len(tb.Text), hash)
				hashes[hash] = placeholder
				tb.Text = placeholder
			}
		}
	}
}

// compactToolUseInputs replaces large content fields inside old tool_use blocks:
//   - write_file:  "content" → compact summary  (also registers hash for tool_result dedup)
//   - patch_file:  "old_text" and "new_text" → compact summaries
//   - read_file:   input only has "path", nothing to compact
func compactToolUseInputs(msg *anthropic.MessageParam, hashes map[string]string) {
	for j := range msg.Content {
		block := &msg.Content[j]
		if block.OfToolUse == nil {
			continue
		}
		name := block.OfToolUse.Name

		// Input can be json.RawMessage (from msg.ToParam()) or map[string]interface{}
		// (after a JSON round-trip deep copy).  Handle both.
		var inputMap map[string]interface{}
		switch v := block.OfToolUse.Input.(type) {
		case json.RawMessage:
			if err := json.Unmarshal(v, &inputMap); err != nil {
				continue
			}
		case map[string]interface{}:
			inputMap = v
		default:
			continue
		}

		changed := false
		switch name {
		case "write_file":
			if content, ok := inputMap["content"].(string); ok && len(content) > compactToolUseThreshold {
				path, _ := inputMap["path"].(string)
				hash := contentHash(content)
				// Register hash so matching tool_result reads get same dedup placeholder.
				if _, seen := hashes[hash]; !seen {
					hashes[hash] = fmt.Sprintf("[file content omitted: %d bytes, ref:%s]", len(content), hash)
				}
				inputMap["content"] = fmt.Sprintf("[compacted write_file: %d bytes → %s, ref:%s]", len(content), path, hash)
				changed = true
			}

		case "patch_file":
			if oldText, ok := inputMap["old_text"].(string); ok && len(oldText) > compactToolUseThreshold {
				inputMap["old_text"] = fmt.Sprintf("[compacted old_text: %d bytes]", len(oldText))
				changed = true
			}
			if newText, ok := inputMap["new_text"].(string); ok && len(newText) > compactToolUseThreshold {
				path, _ := inputMap["path"].(string)
				inputMap["new_text"] = fmt.Sprintf("[compacted new_text: %d bytes → %s]", len(newText), path)
				changed = true
			}
		}

		if changed {
			if reEncoded, err := json.Marshal(inputMap); err == nil {
				block.OfToolUse.Input = json.RawMessage(reEncoded)
			}
		}
	}
}

// contentHash returns an 8-character hex prefix of the MD5 of s — enough for
// identification and dedup; not meant to be cryptographically strong.
func contentHash(s string) string {
	h := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", h[:4])
}

// trimOldToolResults is the legacy compaction function, kept for backwards
// compatibility with existing tests.  New code should call compactAgentHistory.
func trimOldToolResults(messages []anthropic.MessageParam, maxContentLen int) []anthropic.MessageParam {
	if len(messages) <= 4 {
		return messages
	}
	cutoff := len(messages) - 2
	for i := 0; i < cutoff; i++ {
		if messages[i].Role != anthropic.MessageParamRoleUser {
			continue
		}
		for j := range messages[i].Content {
			block := &messages[i].Content[j]
			if block.OfToolResult == nil {
				continue
			}
			for k := range block.OfToolResult.Content {
				tb := block.OfToolResult.Content[k].OfText
				if tb == nil {
					continue
				}
				if len(tb.Text) > maxContentLen {
					tb.Text = tb.Text[:maxContentLen] +
						fmt.Sprintf("\n[...%d chars omitted to reduce context size]", len(tb.Text)-maxContentLen)
				}
			}
		}
	}
	return messages
}
