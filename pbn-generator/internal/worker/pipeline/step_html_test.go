package pipeline

import (
	"context"
	"strings"
	"testing"
)

type fakeLLMForHTML struct {
	genCalled bool
}

func (f *fakeLLMForHTML) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	f.genCalled = true
	return `<html><head><title>Test</title></head><body><h1>Content</h1></body></html>`, nil
}

func (f *fakeLLMForHTML) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

type fakePromptManagerForHTML struct{}

func (f *fakePromptManagerForHTML) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "prompt-html-v2", "Generate HTML based on {{design_system}} {{content_markdown}} {{logo_svg}} {{favicon_tag}}", "gemini-2.5-pro", nil
}

func TestHTMLGenerationStep(t *testing.T) {
	llm := &fakeLLMForHTML{}
	step := &HTMLGenerationStep{}
	logs := make([]string, 0)

	state := &PipelineState{
		Artifacts: map[string]any{
			"design_system":    `{"colors": {"primary": "#000"}}`,
			"content_markdown": "# Title\nContent",
			"logo_svg":         "<svg><circle/></svg>",
			"favicon_tag":      `<link rel="icon" href="/logo.svg">`,
		},
		Context: map[string]any{
			"design_system":    `{"colors": {"primary": "#000"}}`,
			"content_markdown": "# Title\nContent",
			"logo_svg":         "<svg><circle/></svg>",
			"favicon_tag":      `<link rel="icon" href="/logo.svg">`,
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForHTML{},
		DefaultModel:  "gemini-2.5-pro",
		AppendLog: func(s string) {
			logs = append(logs, s)
		},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	// Проверяем, что LLM был вызван
	if !llm.genCalled {
		t.Fatalf("expected LLM Generate to be called")
	}

	// Проверяем наличие html_raw
	htmlRaw, ok := artifacts["html_raw"].(string)
	if !ok || htmlRaw == "" {
		t.Fatalf("expected html_raw, got %#v", artifacts["html_raw"])
	}

	// Проверяем, что это валидный HTML
	if !strings.Contains(htmlRaw, "<html") {
		t.Errorf("expected html_raw to contain <html, got: %s", htmlRaw[:min(50, len(htmlRaw))])
	}

	// Проверяем, что html_raw сохранен в контекст
	if state.Context["html_raw"] != htmlRaw {
		t.Errorf("expected html_raw to be saved in context")
	}
}

func TestHTMLGenerationStep_MissingRequired(t *testing.T) {
	step := &HTMLGenerationStep{}
	tests := []struct {
		name      string
		artifacts map[string]any
		context   map[string]any
		expected  string
	}{
		{
			name:      "missing design_system",
			artifacts: map[string]any{},
			context:   map[string]any{"content_markdown": "# Title", "logo_svg": "<svg/>", "favicon_tag": "<link/>"},
			expected:  "design_system is required",
		},
		{
			name:      "missing content_markdown",
			artifacts: map[string]any{"design_system": "{}"},
			context:   map[string]any{"logo_svg": "<svg/>", "favicon_tag": "<link/>"},
			expected:  "content_markdown is required",
		},
		{
			name:      "missing logo_svg",
			artifacts: map[string]any{"design_system": "{}"},
			context:   map[string]any{"content_markdown": "# Title", "favicon_tag": "<link/>"},
			expected:  "logo_svg is required",
		},
		{
			name:      "missing favicon_tag",
			artifacts: map[string]any{"design_system": "{}"},
			context:   map[string]any{"content_markdown": "# Title", "logo_svg": "<svg/>"},
			expected:  "favicon_tag is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &PipelineState{
				Artifacts:     tt.artifacts,
				Context:       tt.context,
				LLMClient:     &fakeLLMForHTML{},
				PromptManager: &fakePromptManagerForHTML{},
				AppendLog:     func(s string) {},
			}

			_, err := step.Execute(context.Background(), state)
			if err == nil {
				t.Fatalf("expected error for missing required artifact, got nil")
			}
			if !strings.Contains(err.Error(), tt.expected) {
				t.Errorf("expected error about %s, got: %v", tt.expected, err)
			}
		})
	}
}

func TestHTMLGenerationStep_FromArtifacts(t *testing.T) {
	llm := &fakeLLMForHTML{}
	step := &HTMLGenerationStep{}
	state := &PipelineState{
		Artifacts: map[string]any{
			"design_system": `{"colors": {"primary": "#000"}}`,
			"logo_svg":      "<svg><circle/></svg>",
			"favicon_tag":   `<link rel="icon" href="/logo.svg">`,
		},
		Context: map[string]any{
			// design_system берется из Artifacts, остальное должно быть в Context
			"content_markdown": "# Title\nContent",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForHTML{},
		AppendLog:     func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	// Проверяем, что шаг работает с данными из Artifacts для design_system
	htmlRaw, ok := artifacts["html_raw"].(string)
	if !ok || htmlRaw == "" {
		t.Fatalf("expected html_raw, got %#v", artifacts["html_raw"])
	}
}
