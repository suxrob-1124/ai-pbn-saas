//go:build integration

package pipeline

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"obzornik-pbn-generator/internal/llm"
)

// StaticPromptManager возвращает заранее заданный промпт для image_prompt_generation.
type staticPromptManager struct {
	body  string
	model string
}

func (s *staticPromptManager) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "prompt-image-generator-v2", s.body, s.model, nil
}

// TestImageGeneration_Integration выполняет реальный запрос к Gemini (нужен GEMINI_API_KEY).
func TestImageGeneration_Integration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if strings.TrimSpace(apiKey) == "" {
		t.Skip("GEMINI_API_KEY is not set; skipping integration test")
	}

	// Реальный клиент Gemini
	llmClient := llm.NewClient(llm.DefaultConfig(apiKey))

	// Используем актуальный промпт (упрощенный вариант из seed.sql с запретами на текст/логотипы)
	promptBody := `
# РОЛЬ
Ты — AI Арт-директор. Твоя задача — написать идеальные промпты для генерации изображений (Text-to-Image).

# ВХОДНЫЕ ДАННЫЕ
1.  image_style_prompt: Глобальный стиль из дизайн-системы.
2.  Список задач на изображения из ТЗ.

# ПРАВИЛА ДЛЯ ПРОМПТОВ
Для каждой картинки создай промпт на английском языке, следуя формуле:
[Subject/Action] + [Environment/Background] + [Lighting/Mood] + [Style Modifiers]

КРИТИЧЕСКИЕ ЗАПРЕТЫ:
1.  NO TEXT: Никогда не проси нейросеть написать текст, буквы, цифры или логотипы внутри картинки.
2.  NO COMPLEX UI: Не проси рисовать сложные интерфейсы сайтов.
3.  Если нужен "Логотип бренда", пиши: "Minimalist abstract logo symbol representing [Concept], vector style, white background".

# ФОРМАТ ВЫВОДА
JSON массив: [{"slug": "...", "prompt": "..."}]

---
### Design Plan:
{{ design_system }}

### Image Tasks:
{{ technical_spec }}
`

	pm := &staticPromptManager{
		body:  promptBody,
		model: "gemini-2.5-pro",
	}

	logs := make([]string, 0)
	state := &PipelineState{
		Artifacts: map[string]any{
			"design_system":  `{"pfx":"x9k3","style_name":"Cyberpunk neon","image_style_prompt":"Cinematic cyberpunk, neon rain, high contrast"}`,
			"technical_spec": `Image 1: "hero-cat" — A futuristic robot cat sitting on a rain-slicked roof, looking at neon city.`,
		},
		LLMClient:     NewLLMClient(llmClient),
		PromptManager: pm,
		DefaultModel:  "gemini-2.5-pro",
		AppendLog: func(s string) {
			logs = append(logs, s)
			fmt.Println(s)
		},
	}

	step := &ImageGenerationStep{}
	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	// Печатаем сгенерированный промпт из истории клиента
	for _, r := range llmClient.GetRequests() {
		if r.Stage == "image_prompt_generation" {
			fmt.Printf("generated prompt:\n%s\n", r.Prompt)
		}
	}

	// Разбираем generated_files
	var files []GeneratedFile
	switch v := artifacts["generated_files"].(type) {
	case []GeneratedFile:
		files = v
	default:
		b, _ := json.Marshal(v)
		_ = json.Unmarshal(b, &files)
	}

	if len(files) == 0 {
		t.Fatalf("no generated files found")
	}

	// Используем временную директорию вместо хардкода
	outputDir, err := os.MkdirTemp("", "pbn-generator-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	// Автоматическая очистка после теста
	t.Cleanup(func() {
		os.RemoveAll(outputDir)
	})

	for _, f := range files {
		data, err := base64.StdEncoding.DecodeString(f.ContentBase64)
		if err != nil {
			t.Fatalf("failed to decode base64 for %s: %v", f.Path, err)
		}
		fmt.Printf("image size for %s: %d bytes\n", f.Path, len(data))
		target := filepath.Join(outputDir, f.Path)
		if err := os.WriteFile(target, data, 0o644); err != nil {
			t.Fatalf("failed to write file %s: %v", target, err)
		}
		fmt.Printf("saved file to %s\n", target)
	}
}
