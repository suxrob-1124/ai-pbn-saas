package httpserver

import (
	"testing"
)

func TestParseJSONGeneratedFilesWithFence(t *testing.T) {
	raw := "```json\n{\"files\":[{\"path\":\"about.html\",\"content\":\"<h1>About</h1>\",\"mime_type\":\"text/html\"}],\"warnings\":[\"ok\"]}\n```"
	files, warnings, err := parseJSONGeneratedFiles(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "about.html" {
		t.Fatalf("unexpected path: %s", files[0].Path)
	}
	if files[0].MimeType != "text/html" {
		t.Fatalf("unexpected mime_type: %s", files[0].MimeType)
	}
	if len(warnings) != 1 || warnings[0] != "ok" {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
}

func TestParseJSONGeneratedFilesInvalid(t *testing.T) {
	raw := "{\"files\":[{\"path\":\"about.html\"}]"
	if _, _, err := parseJSONGeneratedFiles(raw); err == nil {
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
