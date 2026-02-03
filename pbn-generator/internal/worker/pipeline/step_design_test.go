package pipeline

import (
	"context"
	"strings"
	"testing"
)

type fakeLLMForDesign struct {
	genCalled bool
	response  string
}

func (f *fakeLLMForDesign) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	f.genCalled = true
	if f.response != "" {
		return f.response, nil
	}
	// Возвращаем валидный JSON дизайн-системы
	return `{
		"pfx": "abc",
		"style_name": "minimal",
		"layout": "centered",
		"color_palette": {"primary": "#000", "secondary": "#fff"},
		"font_palette": {"heading": "Arial", "body": "Arial"},
		"element_style": {},
		"image_style_prompt": "minimal style",
		"logo_concept": "simple logo",
		"title": "Test Site",
		"canonical": "https://example.com",
		"design_seed": "test-seed-123"
	}`, nil
}

func (f *fakeLLMForDesign) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

type fakePromptManagerForDesign struct{}

func (f *fakePromptManagerForDesign) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "prompt-design-v2", "Generate design system with style_id={{style_id}} color_id={{color_id}}", "gemini-2.5-pro", nil
}

func TestDesignArchitectureStep(t *testing.T) {
	llm := &fakeLLMForDesign{}
	step := &DesignArchitectureStep{}
	logs := make([]string, 0)

	state := &PipelineState{
		Context: map[string]any{
			"content_markdown": "# Title\n\nContent here.",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForDesign{},
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

	// Проверяем наличие design_system
	designSystem, ok := artifacts["design_system"].(string)
	if !ok || designSystem == "" {
		t.Fatalf("expected design_system, got %#v", artifacts["design_system"])
	}

	// Проверяем, что это валидный JSON
	if !strings.Contains(designSystem, `"pfx"`) {
		t.Errorf("expected design_system to contain pfx, got: %s", designSystem[:min(100, len(designSystem))])
	}

	// Проверяем, что design_system сохранен в контекст
	if state.Context["design_system"] != designSystem {
		t.Errorf("expected design_system to be saved in context")
	}
}

func TestDesignArchitectureStep_MissingContentMarkdown(t *testing.T) {
	step := &DesignArchitectureStep{}
	state := &PipelineState{
		Context:       map[string]any{},
		LLMClient:     &fakeLLMForDesign{},
		PromptManager: &fakePromptManagerForDesign{},
		AppendLog:     func(s string) {},
	}

	_, err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatalf("expected error for missing content_markdown, got nil")
	}
	if !strings.Contains(err.Error(), "content_markdown not found") {
		t.Errorf("expected error about content_markdown, got: %v", err)
	}
}

func TestDesignArchitectureStep_InvalidJSON(t *testing.T) {
	llm := &fakeLLMForDesign{
		response: "invalid json",
	}
	step := &DesignArchitectureStep{}
	state := &PipelineState{
		Context: map[string]any{
			"content_markdown": "# Title",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForDesign{},
		AppendLog:     func(s string) {},
	}

	_, err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatalf("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse design system") {
		t.Errorf("expected error about parsing design system, got: %v", err)
	}
}
