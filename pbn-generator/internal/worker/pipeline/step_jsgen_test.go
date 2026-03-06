package pipeline

import (
	"context"
	"strings"
	"testing"
)

type fakeLLMForJS struct {
	genCalled bool
}

func (f *fakeLLMForJS) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	f.genCalled = true
	return `console.log('Hello World');`, nil
}

func (f *fakeLLMForJS) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

type fakePromptManagerForJS struct{}

func (f *fakePromptManagerForJS) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "prompt-js-v2", "Generate JS based on {{design_system}} {{html_raw}}", "gemini-2.5-pro", nil
}

func TestJSGenerationStep(t *testing.T) {
	llm := &fakeLLMForJS{}
	step := &JSGenerationStep{}
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
		PromptManager: &fakePromptManagerForJS{},
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

	// Проверяем наличие js_content
	jsContent, ok := artifacts["js_content"].(string)
	if !ok || jsContent == "" {
		t.Fatalf("expected js_content, got %#v", artifacts["js_content"])
	}

	// Проверяем, что код-блоки удалены
	if strings.Contains(jsContent, "```") {
		t.Errorf("expected code blocks to be removed from js_content, got: %s", jsContent)
	}

	// Проверяем generated_files (script.js больше не добавляется — JS инлайнится в assembly)
	files, ok := artifacts["generated_files"].([]GeneratedFile)
	if !ok {
		t.Fatalf("expected generated_files slice, got %#v", artifacts["generated_files"])
	}

	for _, f := range files {
		if f.Path == "script.js" {
			t.Errorf("script.js should not be in generated_files (JS is inlined in assembly)")
		}
	}

	// Проверяем, что js_content сохранен в контекст
	if state.Context["js_content"] != jsContent {
		t.Errorf("expected js_content to be saved in context")
	}
}

func TestJSGenerationStep_MissingRequired(t *testing.T) {
	step := &JSGenerationStep{}
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
				LLMClient:     &fakeLLMForJS{},
				PromptManager: &fakePromptManagerForJS{},
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

func TestJSGenerationStep_JSWithCodeBlocks(t *testing.T) {
	llm := &fakeLLMForJSWithResponse{response: "```javascript\nconsole.log('test');\n```"}
	step := &JSGenerationStep{}
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
		PromptManager: &fakePromptManagerForJS{},
		AppendLog:     func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	jsContent := artifacts["js_content"].(string)
	// Проверяем, что код-блоки удалены
	if strings.Contains(jsContent, "```") {
		t.Errorf("expected code blocks to be removed from js_content, got: %s", jsContent)
	}
}

type fakeLLMForJSWithResponse struct {
	response string
}

func (f *fakeLLMForJSWithResponse) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	if f.response != "" {
		return f.response, nil
	}
	return `console.log('test');`, nil
}

func (f *fakeLLMForJSWithResponse) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}
