package legacy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildLegacyArtifactsFullSet(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), `<html><head><link rel="icon" href="/favicon.ico"></head><body>Hello</body></html>`)
	mustWrite(t, filepath.Join(dir, "style.css"), `body{color:red}`)
	mustWrite(t, filepath.Join(dir, "script.js"), `console.log("ok")`)
	mustWrite(t, filepath.Join(dir, "404.html"), `<html>404</html>`)
	mustWrite(t, filepath.Join(dir, "logo.svg"), `<svg></svg>`)
	mustWriteBytes(t, filepath.Join(dir, "img.png"), []byte{0x89, 0x50, 0x4e, 0x47})

	artifacts, meta, err := BuildLegacyArtifacts(dir, "example.com", "import_legacy")
	if err != nil {
		t.Fatalf("BuildLegacyArtifacts failed: %v", err)
	}
	if artifacts["final_html"] == nil {
		t.Fatalf("expected final_html")
	}
	if artifacts["css_content"] == nil {
		t.Fatalf("expected css_content")
	}
	if artifacts["js_content"] == nil {
		t.Fatalf("expected js_content")
	}
	if artifacts["404_html"] == nil {
		t.Fatalf("expected 404_html")
	}
	if artifacts["logo_svg"] == nil {
		t.Fatalf("expected logo_svg")
	}
	if artifacts["favicon_tag"] == nil {
		t.Fatalf("expected favicon_tag")
	}
	if artifacts["legacy_decode_meta"] == nil {
		t.Fatalf("expected legacy_decode_meta")
	}
	if meta.ArtifactHash == "" {
		t.Fatalf("expected artifact hash")
	}
	if meta.Source != "import_legacy" {
		t.Fatalf("unexpected source: %s", meta.Source)
	}
}

func TestBuildLegacyArtifactsWithoutOptionalFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), `<html><body>No optional</body></html>`)

	artifacts, meta, err := BuildLegacyArtifacts(dir, "example.com", "decode_backfill")
	if err != nil {
		t.Fatalf("BuildLegacyArtifacts failed: %v", err)
	}
	if artifacts["final_html"] == nil {
		t.Fatalf("expected final_html")
	}
	if artifacts["css_content"] != nil {
		t.Fatalf("did not expect css_content")
	}
	if artifacts["js_content"] != nil {
		t.Fatalf("did not expect js_content")
	}
	if meta.Source != "decode_backfill" {
		t.Fatalf("unexpected source: %s", meta.Source)
	}
}

func TestBuildLegacyArtifactsAppliesLimits(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), `<html><body>ok</body></html>`)
	large := strings.Repeat("A", maxTextEmbedBytesPerFile+128)
	mustWrite(t, filepath.Join(dir, "huge.txt"), large)

	artifacts, meta, err := BuildLegacyArtifacts(dir, "example.com", "import_legacy")
	if err != nil {
		t.Fatalf("BuildLegacyArtifacts failed: %v", err)
	}
	if artifacts["generated_files"] == nil {
		t.Fatalf("expected generated_files")
	}
	foundSkip := false
	for _, skip := range meta.Skipped {
		if skip.Path == "huge.txt" && skip.Reason == "exceeds_text_embed_limit" {
			foundSkip = true
			break
		}
	}
	if !foundSkip {
		t.Fatalf("expected skip reason for huge.txt")
	}
}

func TestBuildLegacyArtifactsInvalidHTML(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), `<html><head><link rel="icon" href="/f.ico"></head><body><a`)

	artifacts, _, err := BuildLegacyArtifacts(dir, "example.com", "import_legacy")
	if err != nil {
		t.Fatalf("BuildLegacyArtifacts failed: %v", err)
	}
	if artifacts["final_html"] == nil {
		t.Fatalf("expected final_html")
	}
}

func mustWrite(t *testing.T, path, value string) {
	t.Helper()
	mustWriteBytes(t, path, []byte(value))
}

func mustWriteBytes(t *testing.T, path string, value []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, value, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
