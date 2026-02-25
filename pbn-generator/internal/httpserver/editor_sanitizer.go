package httpserver

import (
	"encoding/json"
	"errors"
	"strings"
)

func normalizeEditorContextMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "manual":
		return "manual"
	case "hybrid":
		return "hybrid"
	default:
		return "auto"
	}
}

func stripMarkdownFences(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
		}
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
			lines = lines[:len(lines)-1]
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return trimmed
}

func parseJSONGeneratedFiles(raw string) (editorPageSuggestionPayload, error) {
	cleaned := stripMarkdownFences(raw)
	var payload editorPageSuggestionPayload
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return editorPageSuggestionPayload{}, err
	}
	if len(payload.Files) == 0 {
		return editorPageSuggestionPayload{}, errors.New("empty files array")
	}
	return payload, nil
}

func sanitizeAISuggestContent(raw string, currentContent string) (string, []string) {
	warnings := make([]string, 0)
	cleaned := stripMarkdownFences(raw)
	if cleaned == "" {
		return currentContent, []string{"AI вернул пустой ответ, применен no-op"}
	}

	var wrapped struct {
		Suggested string `json:"suggested_content"`
		Content   string `json:"content"`
		Files     []struct {
			Content string `json:"content"`
		} `json:"files"`
	}
	if json.Unmarshal([]byte(cleaned), &wrapped) == nil {
		switch {
		case strings.TrimSpace(wrapped.Suggested) != "":
			cleaned = strings.TrimSpace(wrapped.Suggested)
			warnings = append(warnings, "AI вернул JSON-wrapper, извлечено suggested_content")
		case strings.TrimSpace(wrapped.Content) != "":
			cleaned = strings.TrimSpace(wrapped.Content)
			warnings = append(warnings, "AI вернул JSON-wrapper, извлечено content")
		case len(wrapped.Files) > 0 && strings.TrimSpace(wrapped.Files[0].Content) != "":
			cleaned = strings.TrimSpace(wrapped.Files[0].Content)
			warnings = append(warnings, "AI вернул JSON-wrapper, извлечен первый files[].content")
		}
	}

	lowered := strings.ToLower(cleaned)
	if strings.Contains(lowered, "предоставьте содержимое файла") || strings.Contains(lowered, "provide file content") {
		return currentContent, []string{"AI не смог выполнить изменение без контекста файла, применен no-op"}
	}
	return cleaned, warnings
}

func normalizeEditorPath(raw string) (string, error) {
	return sanitizeFilePath(raw)
}

func normalizeEditorProtectedPath(raw string) (string, error) {
	cleanPath, err := normalizeEditorPath(raw)
	if err != nil {
		return "", err
	}
	if err := validateEditorPath(cleanPath); err != nil {
		return "", err
	}
	return cleanPath, nil
}

func normalizeEditorWritablePath(raw string) (string, error) {
	cleanPath, err := normalizeEditorProtectedPath(raw)
	if err != nil {
		return "", err
	}
	if err := validateUploadPathPolicy(cleanPath); err != nil {
		return "", err
	}
	return cleanPath, nil
}

func normalizeEditorWritablePathWithReason(raw string) (string, string, error) {
	cleanPath, err := normalizeEditorPath(raw)
	if err != nil {
		return "", "invalid", err
	}
	if err := validateEditorPath(cleanPath); err != nil {
		return cleanPath, "protected", err
	}
	if err := validateUploadPathPolicy(cleanPath); err != nil {
		return cleanPath, "blocked", err
	}
	return cleanPath, "", nil
}

func validateEditorMimeAndPayload(relPath, requestedMime string, content []byte) (string, error) {
	detected := detectMimeType(relPath, content)
	if err := validateMimeType(relPath, detected, strings.TrimSpace(requestedMime)); err != nil {
		return "", err
	}
	if err := validateUploadSecurity(relPath, detected, content); err != nil {
		return "", err
	}
	return detected, nil
}
