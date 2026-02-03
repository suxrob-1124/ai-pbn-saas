package pipeline

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

type fakeLLM struct {
	genCalled bool
	imgCalled bool
}

func (f *fakeLLM) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	f.genCalled = true
	// Возвращаем один таск
	return `[{"slug":"hero-image","prompt":"minimal logo"}]`, nil
}

func (f *fakeLLM) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	f.imgCalled = true
	// Фейковые байты картинки
	return []byte{0x89, 0x50, 0x4E, 0x47}, nil
}

type fakePromptManager struct{}

func (f *fakePromptManager) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "prompt-image-generator-v2", "prompt body {{design_system}} {{technical_spec}}", "gemini-2.5-pro", nil
}

// Проверяем, что шаг корректно парсит JSON задач, вызывает LLM для картинки
// и возвращает артефакты с промптами и файлами.
func TestImageGenerationStep(t *testing.T) {
	llm := &fakeLLM{}
	step := &ImageGenerationStep{}
	logs := make([]string, 0)

	state := &PipelineState{
		Artifacts: map[string]any{
			"design_system":  `{"pfx":"abc"}`,
			"technical_spec": "image spec",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManager{},
		DefaultModel:  "gemini-2.5-pro",
		AppendLog: func(s string) {
			logs = append(logs, s)
		},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	// Проверяем, что LLM вызван
	if !llm.genCalled || !llm.imgCalled {
		t.Fatalf("expected llm Generate and GenerateImage to be called, got gen=%v img=%v", llm.genCalled, llm.imgCalled)
	}

	// Проверяем артефакты
	tasks, ok := artifacts["image_prompts"].([]ImageTask)
	if !ok || len(tasks) != 1 {
		t.Fatalf("expected one image task, got %#v", artifacts["image_prompts"])
	}
	if tasks[0].Slug != "hero-image" {
		t.Errorf("unexpected slug: %s", tasks[0].Slug)
	}

	files, ok := artifacts["generated_files"].([]GeneratedFile)
	if !ok || len(files) == 0 {
		t.Fatalf("expected generated_files slice, got %#v", artifacts["generated_files"])
	}

	var pngFound, webpFound bool
	for _, f := range files {
		if f.ContentBase64 == "" {
			t.Fatalf("expected base64 content, got empty for %s", f.Path)
		}
		if _, err := base64.StdEncoding.DecodeString(f.ContentBase64); err != nil {
			t.Fatalf("base64 decode failed for %s: %v", f.Path, err)
		}
		if strings.Contains(f.Path, "hero-image") && strings.HasSuffix(f.Path, ".png") {
			pngFound = true
		}
		if strings.Contains(f.Path, "hero-image") && strings.HasSuffix(f.Path, ".webp") {
			webpFound = true
		}
	}
	if !pngFound {
		t.Fatalf("expected png file for hero-image, got %#v", files)
	}
	// WebP может отсутствовать при сборке без CGO. Если нет — это не фатально для юнит-теста.
	_ = webpFound
}

// TestSanitizeSlug проверяет защиту от path traversal и корректную очистку slug
func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal slug",
			input:    "hero-image",
			expected: "hero-image",
		},
		{
			name:     "path traversal attack",
			input:    "../../etc/passwd",
			expected: "etcpasswd",
		},
		{
			name:     "path traversal with dots",
			input:    "../../../sensitive",
			expected: "sensitive",
		},
		{
			name:     "windows path traversal",
			input:    "..\\..\\windows\\system32",
			expected: "windowssystem32",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "image",
		},
		{
			name:     "only dangerous chars",
			input:    "../../../",
			expected: "image",
		},
		{
			name:     "spaces and special chars",
			input:    "my image 123!@#",
			expected: "myimage123", // Пробелы удаляются до замены на дефисы
		},
		{
			name:     "very long string",
			input:    strings.Repeat("a", 150),
			expected: strings.Repeat("a", 100),
		},
		{
			name:     "mixed case and numbers",
			input:    "Hero-Image_123",
			expected: "Hero-Image_123",
		},
		{
			name:     "multiple dashes",
			input:    "hero---image",
			expected: "hero-image",
		},
		{
			name:     "leading and trailing dashes",
			input:    "-hero-image-",
			expected: "hero-image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeSlug(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeSlug(%q) = %q, want %q", tt.input, result, tt.expected)
			}
			// Дополнительная проверка: результат не должен содержать опасные символы
			if strings.Contains(result, "../") || strings.Contains(result, "./") ||
				strings.Contains(result, "/") || strings.Contains(result, "\\") {
				t.Errorf("sanitizeSlug(%q) contains dangerous characters: %q", tt.input, result)
			}
		})
	}
}
