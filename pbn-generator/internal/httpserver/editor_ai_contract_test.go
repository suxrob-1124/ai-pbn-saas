package httpserver

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseJSONGeneratedFilesWithFence(t *testing.T) {
	raw := "```json\n{\"files\":[{\"path\":\"about.html\",\"content\":\"<h1>About</h1>\",\"mime_type\":\"text/html\"}],\"assets\":[{\"path\":\"about/hero.webp\",\"alt\":\"hero\",\"prompt\":\"casino hero\",\"mime_type\":\"image/webp\"}],\"warnings\":[\"ok\"]}\n```"
	payload, err := parseJSONGeneratedFiles(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(payload.Files))
	}
	if payload.Files[0].Path != "about.html" {
		t.Fatalf("unexpected path: %s", payload.Files[0].Path)
	}
	if payload.Files[0].MimeType != "text/html" {
		t.Fatalf("unexpected mime_type: %s", payload.Files[0].MimeType)
	}
	if len(payload.Assets) != 1 || payload.Assets[0].Path != "about/hero.webp" {
		t.Fatalf("unexpected assets: %#v", payload.Assets)
	}
	if len(payload.Warnings) != 1 || payload.Warnings[0] != "ok" {
		t.Fatalf("unexpected warnings: %#v", payload.Warnings)
	}
}

func TestParseJSONGeneratedFilesInvalid(t *testing.T) {
	raw := "{\"files\":[{\"path\":\"about.html\"}]"
	if _, err := parseJSONGeneratedFiles(raw); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestSanitizeAISuggestContentExtractsWrapper(t *testing.T) {
	raw := "```json\n{\"suggested_content\":\"<h1>Updated</h1>\"}\n```"
	got, warnings := sanitizeAISuggestContent(raw, "<h1>Old</h1>")
	if got != "<h1>Updated</h1>" {
		t.Fatalf("unexpected content: %q", got)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected wrapper warning")
	}
}

func TestSanitizeAISuggestContentNoopOnServiceReply(t *testing.T) {
	current := "<h1>Original</h1>"
	raw := "Готов. Предоставьте содержимое файла и инструкции по его редактированию."
	got, warnings := sanitizeAISuggestContent(raw, current)
	if got != current {
		t.Fatalf("expected current content unchanged")
	}
	if len(warnings) == 0 {
		t.Fatalf("expected no-op warning")
	}
}

func TestNormalizeEditorContextMode(t *testing.T) {
	cases := map[string]string{
		"":        "manual",
		"unknown": "manual",
		"manual":  "manual",
		"auto":    "auto",
		"hybrid":  "hybrid",
	}
	for input, expected := range cases {
		if got := normalizeEditorContextMode(input); got != expected {
			t.Fatalf("input=%q expected=%q got=%q", input, expected, got)
		}
	}
}

func TestValidateImagePayloadRejectsBrokenWebp(t *testing.T) {
	err := validateImagePayload("about/hero.webp", "image/webp", []byte("not-a-webp"))
	if err == nil {
		t.Fatalf("expected invalid webp payload error")
	}
}

func TestValidateImagePayloadAcceptsValidPNG(t *testing.T) {
	// 1x1 transparent PNG
	raw, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jw2kAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode base64 png: %v", err)
	}
	if err := validateImagePayload("about/pixel.png", "image/png", raw); err != nil {
		t.Fatalf("expected valid png payload, got %v", err)
	}
}

func TestNormalizeEditorPageSuggestionPayloadFiltersInvalidEntries(t *testing.T) {
	payload := editorPageSuggestionPayload{
		Files: []editorGeneratedFile{
			{Path: "about.html", Content: "<h1>About</h1>", MimeType: "text/html"},
			{Path: "assets/hero.webp", Content: "RIFFxxxxWEBP", MimeType: "image/webp"},
			{Path: "about.php", Content: "<?php echo 1;", MimeType: "text/plain"},
			{Path: "../escape.html", Content: "<h1>Bad</h1>", MimeType: "text/html"},
			{Path: "empty.html", Content: "   ", MimeType: "text/html"},
		},
		Assets: []editorGeneratedAsset{
			{Path: "assets/hero.webp", Alt: "hero", Prompt: "hero image", MimeType: "image/webp"},
			{Path: "docs/readme.txt", Alt: "doc", Prompt: "not image", MimeType: "text/plain"},
			{Path: "../escape.webp", Alt: "bad", Prompt: "bad", MimeType: "image/webp"},
		},
		Warnings: []string{"from-model"},
	}

	files, assets, warnings := normalizeEditorPageSuggestionPayload(payload)
	if len(files) != 1 {
		t.Fatalf("expected only 1 valid file, got %d (%#v)", len(files), files)
	}
	if files[0]["path"] != "about.html" {
		t.Fatalf("unexpected output file path: %#v", files[0])
	}
	if len(assets) != 1 {
		t.Fatalf("expected only 1 valid asset, got %d (%#v)", len(assets), assets)
	}
	if assets[0]["path"] != "assets/hero.webp" {
		t.Fatalf("unexpected output asset path: %#v", assets[0])
	}

	joined := strings.Join(warnings, " | ")
	for _, needle := range []string{
		"from-model",
		"binary asset skipped from files",
		"blocked file type skipped: about.php",
		"invalid path skipped: ../escape.html",
		"empty content skipped: empty.html",
		"asset skipped (not an image): docs/readme.txt",
		"invalid asset path skipped: ../escape.webp",
	} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("expected warning %q in %q", needle, joined)
		}
	}
}
