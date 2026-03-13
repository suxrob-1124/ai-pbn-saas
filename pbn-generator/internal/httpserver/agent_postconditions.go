package httpserver

import (
	"bytes"
	"regexp"
	"strings"
)

// postconditionKind identifies which postcondition to evaluate after a file mutation.
type postconditionKind uint8

const (
	// pcNone means no postcondition was detected — agent loop continues normally.
	pcNone postconditionKind = iota
	// pcNoHTMLComments: the file must contain no <!-- ... --> comment blocks.
	pcNoHTMLComments
	// pcNoCSSComments: the file must contain no /* ... */ comment blocks.
	pcNoCSSComments
	// pcSubstringAbsent: the file must not contain the Target substring.
	pcSubstringAbsent
)

// agentPostcondition carries the kind and optional target for evaluation.
type agentPostcondition struct {
	Kind   postconditionKind
	Target string // used only for pcSubstringAbsent
}

// compiled regexes — initialised once at startup.
var (
	htmlCommentRe = regexp.MustCompile(`<!--[\s\S]*?-->`)
	cssCommentRe  = regexp.MustCompile(`/\*[\s\S]*?\*/`)
)

// Explicit whitelists.  Intentionally narrow: only phrases that unambiguously
// describe a mechanical removal task that can be verified by reading the file.
var (
	removalWords = []string{
		"remove", "delete", "strip", "clean", "clear",
		"убери", "удали", "очисти", "вырезать", "убрать", "удалить",
	}
	htmlCommentKeywords = []string{
		"html comment", "html комментар", "html-комментар", "html коммент",
	}
	cssCommentKeywords = []string{
		"css comment", "css комментар", "css-комментар", "css коммент",
	}
)

// detectPostcondition examines the original user message and returns a postcondition
// if a narrow, safe mechanical task is unambiguously recognised.
// Any message not on the explicit whitelist returns pcNone, preserving normal behaviour.
func detectPostcondition(msg string) agentPostcondition {
	lower := strings.ToLower(msg)
	if !containsAny(lower, removalWords) {
		return agentPostcondition{}
	}
	if containsAny(lower, htmlCommentKeywords) {
		return agentPostcondition{Kind: pcNoHTMLComments}
	}
	if containsAny(lower, cssCommentKeywords) {
		return agentPostcondition{Kind: pcNoCSSComments}
	}
	return agentPostcondition{}
}

// checkPostcondition returns true when the postcondition is satisfied for content.
// Always returns false for pcNone.
func checkPostcondition(pc agentPostcondition, content []byte) bool {
	switch pc.Kind {
	case pcNoHTMLComments:
		return !htmlCommentRe.Match(content)
	case pcNoCSSComments:
		return !cssCommentRe.Match(content)
	case pcSubstringAbsent:
		return !bytes.Contains(content, []byte(pc.Target))
	}
	return false
}

// containsAny returns true if s contains at least one of the given substrings.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// uniquePaths returns a deduplicated list of paths preserving first-seen order.
func uniquePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}
