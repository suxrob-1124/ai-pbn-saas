package pipeline

import (
	"context"
	"strings"
	"testing"
)

type fakeLLMForCSS struct {
	genCalled bool
}

func (f *fakeLLMForCSS) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	f.genCalled = true
	return `body { color: #000; margin: 0; padding: 0; }`, nil
}

func (f *fakeLLMForCSS) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

func (f *fakeLLMForCSS) GenerateMultiTurn(ctx context.Context, stage, systemInstruction string, turns []string, model string) (string, error) {
	return "", nil
}

type fakePromptManagerForCSS struct{}

func (f *fakePromptManagerForCSS) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "prompt-css-v2", "Generate CSS based on {{design_system}} {{html_raw}}", "gemini-2.5-pro", nil
}

func TestCSSGenerationStep(t *testing.T) {
	llm := &fakeLLMForCSS{}
	step := &CSSGenerationStep{}
	logs := make([]string, 0)

	state := &PipelineState{
		Artifacts: map[string]any{
			"design_system": `{"colors": {"primary": "#000"}}`,
			"html_raw":      "<html><body>Test</body></html>",
		},
		Context: map[string]any{
			"design_system": `{"colors": {"primary": "#000"}}`,
			"html_raw":      "<html><body>Test</body></html>",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForCSS{},
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

	// Проверяем наличие css_content
	cssContent, ok := artifacts["css_content"].(string)
	if !ok || cssContent == "" {
		t.Fatalf("expected css_content, got %#v", artifacts["css_content"])
	}

	// Проверяем, что код-блоки удалены
	if strings.Contains(cssContent, "```") {
		t.Errorf("expected code blocks to be removed from css_content, got: %s", cssContent)
	}

	// Проверяем generated_files (style.css больше не добавляется — CSS инлайнится в assembly)
	files, ok := artifacts["generated_files"].([]GeneratedFile)
	if !ok {
		t.Fatalf("expected generated_files slice, got %#v", artifacts["generated_files"])
	}

	for _, f := range files {
		if f.Path == "style.css" {
			t.Errorf("style.css should not be in generated_files (CSS is inlined in assembly)")
		}
	}

	// Проверяем, что css_content сохранен в контекст
	if state.Context["css_content"] != cssContent {
		t.Errorf("expected css_content to be saved in context")
	}
}

func TestCSSGenerationStep_MissingRequired(t *testing.T) {
	step := &CSSGenerationStep{}
	tests := []struct {
		name      string
		artifacts map[string]any
		context   map[string]any
		expected  string
	}{
		{
			name:      "missing design_system",
			artifacts: map[string]any{"html_raw": "<html></html>"},
			context:   map[string]any{"html_raw": "<html></html>"},
			expected:  "design_system is required",
		},
		{
			name:      "missing html_raw",
			artifacts: map[string]any{"design_system": "{}"},
			context:   map[string]any{"design_system": "{}"},
			expected:  "html_raw is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &PipelineState{
				Artifacts:     tt.artifacts,
				Context:       tt.context,
				LLMClient:     &fakeLLMForCSS{},
				PromptManager: &fakePromptManagerForCSS{},
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

func TestCSSGenerationStep_CSSWithCodeBlocks(t *testing.T) {
	llm := &fakeLLMForCSSWithResponse{response: "```css\nbody { color: red; }\n```"}
	step := &CSSGenerationStep{}
	state := &PipelineState{
		Artifacts: map[string]any{
			"design_system": `{"colors": {"primary": "#000"}}`,
			"html_raw":      "<html><body>Test</body></html>",
		},
		Context: map[string]any{
			"design_system": `{"colors": {"primary": "#000"}}`,
			"html_raw":      "<html><body>Test</body></html>",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForCSS{},
		AppendLog:     func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	cssContent := artifacts["css_content"].(string)
	// Проверяем, что код-блоки удалены
	if strings.Contains(cssContent, "```") {
		t.Errorf("expected code blocks to be removed from css_content, got: %s", cssContent)
	}
}

type fakeLLMForCSSWithResponse struct {
	response string
}

func (f *fakeLLMForCSSWithResponse) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	if f.response != "" {
		return f.response, nil
	}
	return `body { color: #000; }`, nil
}

func (f *fakeLLMForCSSWithResponse) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

func (f *fakeLLMForCSSWithResponse) GenerateMultiTurn(ctx context.Context, stage, systemInstruction string, turns []string, model string) (string, error) {
	return "", nil
}
