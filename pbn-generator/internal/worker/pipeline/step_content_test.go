package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

type fakeLLMForContent struct {
	genCalled bool
	calls     int
	responses []string
}

func (f *fakeLLMForContent) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	f.genCalled = true
	f.calls++
	if len(f.responses) > 0 {
		idx := f.calls - 1
		if idx >= len(f.responses) {
			idx = len(f.responses) - 1
		}
		return f.responses[idx], nil
	}
	return `# Title\n\nContent here.`, nil
}

func (f *fakeLLMForContent) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

type fakePromptManagerForContent struct{}

func (f *fakePromptManagerForContent) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "prompt-content-v2", "Generate content based on {{technical_spec}} for domain {{domain}}", "gemini-2.5-pro", nil
}

func TestContentGenerationStep(t *testing.T) {
	llm := &fakeLLMForContent{}
	step := &ContentGenerationStep{}
	logs := make([]string, 0)

	state := &PipelineState{
		Context: map[string]any{
			"technical_spec": "# Technical Spec\n\nGenerate content about topic.",
		},
		Domain: &sqlstore.Domain{
			URL:            "example.com",
			TargetLanguage: "en",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForContent{},
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

	// Проверяем наличие content_markdown
	contentMarkdown, ok := artifacts["content_markdown"].(string)
	if !ok || contentMarkdown == "" {
		t.Fatalf("expected content_markdown, got %#v", artifacts["content_markdown"])
	}

	// Проверяем, что content_markdown сохранен в контекст
	if state.Context["content_markdown"] != contentMarkdown {
		t.Errorf("expected content_markdown to be saved in context")
	}
}

func TestContentGenerationStep_MissingTechnicalSpec(t *testing.T) {
	step := &ContentGenerationStep{}
	state := &PipelineState{
		Context:       map[string]any{},
		Domain:        &sqlstore.Domain{URL: "example.com"},
		LLMClient:     &fakeLLMForContent{},
		PromptManager: &fakePromptManagerForContent{},
		AppendLog:     func(s string) {},
	}

	_, err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatalf("expected error for missing technical_spec, got nil")
	}
	if !strings.Contains(err.Error(), "technical_spec not found") {
		t.Errorf("expected error about technical_spec, got: %v", err)
	}
}

func TestContentGenerationStep_DefaultLanguage(t *testing.T) {
	llm := &fakeLLMForContent{}
	step := &ContentGenerationStep{}
	state := &PipelineState{
		Context: map[string]any{
			"technical_spec": "# Technical Spec",
		},
		Domain: &sqlstore.Domain{
			URL:            "example.com",
			TargetLanguage: "", // Пустой язык, должен использоваться дефолт "sv"
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForContent{},
		AppendLog:     func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	// Проверяем, что контент сгенерирован
	contentMarkdown, ok := artifacts["content_markdown"].(string)
	if !ok || contentMarkdown == "" {
		t.Fatalf("expected content_markdown, got %#v", artifacts["content_markdown"])
	}
}

func TestContentGenerationStep_BrandGuard_CorrectiveRegeneration(t *testing.T) {
	llm := &fakeLLMForContent{
		responses: []string{
			"# Title\n\nОбзор ZenitBet.",
			"# Title\n\nГайд по регистрации в 1хБет.",
		},
	}
	step := &ContentGenerationStep{}
	state := &PipelineState{
		Context: map[string]any{
			"technical_spec": "# ТЗ\n\nБренд 1хБет",
		},
		Domain: &sqlstore.Domain{
			URL:            "example.com",
			MainKeyword:    "регистрация в 1хБет",
			TargetLanguage: "ru",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForContent{},
		AppendLog:     func(string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if llm.calls != 2 {
		t.Fatalf("expected corrective regeneration with 2 calls, got %d", llm.calls)
	}
	if got := artifacts["content_markdown"]; !strings.Contains(fmt.Sprintf("%v", got), "1хБет") {
		t.Fatalf("expected corrected content with 1хБет, got %v", got)
	}
}
