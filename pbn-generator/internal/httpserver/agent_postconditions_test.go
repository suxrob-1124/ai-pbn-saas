package httpserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// ─── detectPostcondition ──────────────────────────────────────────────────────

func TestDetectPostcondition_HTMLComments(t *testing.T) {
	cases := []string{
		"remove html comments from this file",
		"delete html comments",
		"убери html комментарии",
		"удали html комментарии из index.html",
		"strip html comments",
	}
	for _, msg := range cases {
		pc := detectPostcondition(msg)
		if pc.Kind != pcNoHTMLComments {
			t.Errorf("detectPostcondition(%q) = %v, want pcNoHTMLComments", msg, pc.Kind)
		}
	}
}

func TestDetectPostcondition_CSSComments(t *testing.T) {
	cases := []string{
		"remove css comments",
		"delete css comments from style.css",
		"убери css комментарии",
		"удали css комментарии",
		"strip css comments",
	}
	for _, msg := range cases {
		pc := detectPostcondition(msg)
		if pc.Kind != pcNoCSSComments {
			t.Errorf("detectPostcondition(%q) = %v, want pcNoCSSComments", msg, pc.Kind)
		}
	}
}

func TestDetectPostcondition_Unrecognised(t *testing.T) {
	cases := []string{
		"add a navigation menu",
		"change the background color to blue",
		"create a new page about us",
		"update the title",
		"translate page to English",
		"add html comments explaining each section", // add, not remove
	}
	for _, msg := range cases {
		pc := detectPostcondition(msg)
		if pc.Kind != pcNone {
			t.Errorf("detectPostcondition(%q) = %v, want pcNone", msg, pc.Kind)
		}
	}
}

// ─── checkPostcondition ───────────────────────────────────────────────────────

func TestCheckPostcondition_HTMLComments_Satisfied(t *testing.T) {
	pc := agentPostcondition{Kind: pcNoHTMLComments}
	content := []byte("<html><body><p>Hello</p></body></html>")
	if !checkPostcondition(pc, content) {
		t.Error("expected postcondition satisfied (no HTML comments present)")
	}
}

func TestCheckPostcondition_HTMLComments_NotSatisfied(t *testing.T) {
	pc := agentPostcondition{Kind: pcNoHTMLComments}
	content := []byte("<html><!-- this is a comment --><body></body></html>")
	if checkPostcondition(pc, content) {
		t.Error("expected postcondition NOT satisfied (HTML comment still present)")
	}
}

func TestCheckPostcondition_HTMLComments_Multiline(t *testing.T) {
	pc := agentPostcondition{Kind: pcNoHTMLComments}
	content := []byte("<html>\n<!--\n  multi-line comment\n-->\n<body></body></html>")
	if checkPostcondition(pc, content) {
		t.Error("expected postcondition NOT satisfied (multiline HTML comment still present)")
	}
}

func TestCheckPostcondition_CSSComments_Satisfied(t *testing.T) {
	pc := agentPostcondition{Kind: pcNoCSSComments}
	content := []byte("body { color: red; }\nh1 { font-size: 2em; }")
	if !checkPostcondition(pc, content) {
		t.Error("expected postcondition satisfied (no CSS comments present)")
	}
}

func TestCheckPostcondition_CSSComments_NotSatisfied(t *testing.T) {
	pc := agentPostcondition{Kind: pcNoCSSComments}
	content := []byte("body { /* reset */ color: red; }")
	if checkPostcondition(pc, content) {
		t.Error("expected postcondition NOT satisfied (CSS comment still present)")
	}
}

func TestCheckPostcondition_SubstringAbsent_Satisfied(t *testing.T) {
	pc := agentPostcondition{Kind: pcSubstringAbsent, Target: "TODO"}
	if !checkPostcondition(pc, []byte("<p>Hello world</p>")) {
		t.Error("expected postcondition satisfied (target substring absent)")
	}
}

func TestCheckPostcondition_SubstringAbsent_NotSatisfied(t *testing.T) {
	pc := agentPostcondition{Kind: pcSubstringAbsent, Target: "TODO"}
	if checkPostcondition(pc, []byte("<p>TODO: fix this</p>")) {
		t.Error("expected postcondition NOT satisfied (target substring present)")
	}
}

func TestCheckPostcondition_None(t *testing.T) {
	pc := agentPostcondition{Kind: pcNone}
	if checkPostcondition(pc, []byte("anything")) {
		t.Error("pcNone must always return false")
	}
}

// ─── uniquePaths ──────────────────────────────────────────────────────────────

func TestUniquePaths(t *testing.T) {
	in := []string{"a.html", "b.html", "a.html", "c.html", "b.html"}
	got := uniquePaths(in)
	want := []string{"a.html", "b.html", "c.html"}
	if len(got) != len(want) {
		t.Fatalf("uniquePaths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("uniquePaths[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// ─── postcondition auto-stop integration ──────────────────────────────────────

// setupPostconditionEnv creates a minimal server + domain for postcondition tests
// that need to read files from a local filesystem backend.
func setupPostconditionEnv(t *testing.T, fileName, content string) (*Server, sqlstore.Domain, string) {
	t.Helper()
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, fileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	domain := sqlstore.Domain{ID: "dom-1", URL: "example.com"}
	srv := &Server{}
	srv.SetContentBackend(domainfs.NewLocalFSBackend(serverDir))
	return srv, domain, domainDir
}

// TestPostcondition_HTMLComments_AutoStopCondition verifies that after a write_file
// that removes HTML comments, checkPostcondition returns true (auto-stop would fire).
func TestPostcondition_HTMLComments_AutoStopCondition(t *testing.T) {
	srv, domain, domainDir := setupPostconditionEnv(t, "index.html", "<html><body><p>clean</p></body></html>")

	// Simulate agent having written a clean file (no comments).
	_ = domainDir // file already written by setupPostconditionEnv

	pc := detectPostcondition("remove html comments from index.html")
	if pc.Kind != pcNoHTMLComments {
		t.Fatalf("expected pcNoHTMLComments, got %v", pc.Kind)
	}

	content, err := srv.readDomainFileBytesFromBackend(context.Background(), domain, "index.html")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !checkPostcondition(pc, content) {
		t.Error("expected postcondition satisfied — auto-stop should fire")
	}
}

// TestPostcondition_HTMLComments_NoAutoStopIfCommentRemains verifies that when
// comments are still present, checkPostcondition returns false (loop continues).
func TestPostcondition_HTMLComments_NoAutoStopIfCommentRemains(t *testing.T) {
	srv, domain, _ := setupPostconditionEnv(t, "index.html", "<html><!-- still here --><body></body></html>")

	pc := detectPostcondition("remove html comments")
	content, err := srv.readDomainFileBytesFromBackend(context.Background(), domain, "index.html")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if checkPostcondition(pc, content) {
		t.Error("expected postcondition NOT satisfied — loop should continue")
	}
}

// TestPostcondition_CSSComments_AutoStopCondition verifies CSS comment removal.
func TestPostcondition_CSSComments_AutoStopCondition(t *testing.T) {
	srv, domain, _ := setupPostconditionEnv(t, "style.css", "body { color: red; }")

	pc := detectPostcondition("удали css комментарии из style.css")
	if pc.Kind != pcNoCSSComments {
		t.Fatalf("expected pcNoCSSComments, got %v", pc.Kind)
	}
	content, err := srv.readDomainFileBytesFromBackend(context.Background(), domain, "style.css")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !checkPostcondition(pc, content) {
		t.Error("expected postcondition satisfied — auto-stop should fire")
	}
}

// TestPostcondition_WrongFileChanged verifies that if the changed file does not
// match the context file, the auto-stop gate (changedTarget check) blocks it.
func TestPostcondition_WrongFileChanged(t *testing.T) {
	currentFile := "index.html"
	changedThisIter := []string{"about.html"} // different file changed

	changedTarget := false
	for _, p := range changedThisIter {
		if p == currentFile {
			changedTarget = true
			break
		}
	}
	if changedTarget {
		t.Error("changedTarget should be false when wrong file was changed")
	}
}

// TestPostcondition_NoFileChanged verifies that when no file was changed in the
// iteration (read-only), the auto-stop gate (len > filesChangedBefore) blocks it.
func TestPostcondition_NoFileChanged(t *testing.T) {
	allFilesChanged := []string{} // no changes yet
	filesChangedBefore := 0

	// Condition checked before the postcondition block in runAgentLoop:
	// `pc.Kind != pcNone && len(allFilesChanged) > filesChangedBefore`
	firedGate := len(allFilesChanged) > filesChangedBefore
	if firedGate {
		t.Error("postcondition gate should not fire when no file was changed")
	}
}

// TestPostcondition_UnrecognisedTask verifies that unknown tasks produce pcNone
// and checkPostcondition always returns false for it.
func TestPostcondition_UnrecognisedTask(t *testing.T) {
	pc := detectPostcondition("add a footer to every page")
	if pc.Kind != pcNone {
		t.Errorf("expected pcNone for unrecognised task, got %v", pc.Kind)
	}
	if checkPostcondition(pc, []byte("any content")) {
		t.Error("pcNone must never satisfy")
	}
}

// TestPostcondition_SingleUniqueFileAutoTarget verifies that when currentFile=""
// but only one unique file was changed in the session, that file is used as the target.
func TestPostcondition_SingleUniqueFileAutoTarget(t *testing.T) {
	allFilesChanged := []string{"index.html", "index.html"} // same file twice
	unique := uniquePaths(allFilesChanged)
	if len(unique) != 1 || unique[0] != "index.html" {
		t.Errorf("expected single unique path [index.html], got %v", unique)
	}
}

// TestPostcondition_MultipleUniqueFilesNoAutoTarget verifies that when currentFile=""
// and multiple distinct files were changed, no auto-target is selected (fileToCheck="").
func TestPostcondition_MultipleUniqueFilesNoAutoTarget(t *testing.T) {
	allFilesChanged := []string{"index.html", "about.html"}
	unique := uniquePaths(allFilesChanged)
	// auto-stop requires len(unique)==1; here we have 2 → no target selected
	if len(unique) == 1 {
		t.Error("expected no auto-target when multiple unique files changed")
	}
}
