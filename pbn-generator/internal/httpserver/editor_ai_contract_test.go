package httpserver

import (
	"encoding/base64"
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
		"":        "auto",
		"unknown": "auto",
		"manual":  "manual",
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
