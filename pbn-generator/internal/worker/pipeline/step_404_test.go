package pipeline

import (
	"context"
	"strings"
	"testing"
)

type fakeLLMFor404 struct {
	genCalled bool
}

func (f *fakeLLMFor404) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	f.genCalled = true
	return `<html><head><title>404</title></head><body><h1>Page not found</h1></body></html>`, nil
}

func (f *fakeLLMFor404) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

type fakeLLMFor404WithResponse struct {
	response string
}

func (f *fakeLLMFor404WithResponse) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	if f.response != "" {
		return f.response, nil
	}
	return `<html><head><title>404</title></head><body><h1>Page not found</h1></body></html>`, nil
}

func (f *fakeLLMFor404WithResponse) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

type fakePromptManagerFor404 struct{}

func (f *fakePromptManagerFor404) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "prompt-404-v2", "Generate 404 page based on {{design_system}} {{html_raw}} {{css_content}} {{js_content}}", "gemini-2.5-pro", nil
}

func TestPage404GenerationStep(t *testing.T) {
	llm := &fakeLLMFor404{}
	step := &Page404GenerationStep{}
	logs := make([]string, 0)

	state := &PipelineState{
		Artifacts: map[string]any{
			"design_system": `{"colors": {"primary": "#000"}}`,
			"html_raw":      "<html><body>Main page</body></html>",
			"css_content":   "body { color: red; }",
			"js_content":    "console.log('test');",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerFor404{},
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

	// Проверяем наличие 404_html
	html404, ok := artifacts["404_html"].(string)
	if !ok || html404 == "" {
		t.Fatalf("expected 404_html, got %#v", artifacts["404_html"])
	}

	// Проверяем, что это валидный HTML
	if !strings.Contains(html404, "<html") {
		t.Errorf("expected 404_html to contain <html, got: %s", html404[:min(50, len(html404))])
	}

	// Проверяем generated_files
	files, ok := artifacts["generated_files"].([]GeneratedFile)
	if !ok || len(files) == 0 {
		t.Fatalf("expected generated_files slice, got %#v", artifacts["generated_files"])
	}

	has404File := false
	for _, f := range files {
		if f.Path == "404.html" {
			has404File = true
			if f.Content != html404 {
				t.Errorf("404.html content doesn't match 404_html artifact")
			}
			break
		}
	}
	if !has404File {
		t.Errorf("expected 404.html in generated_files")
	}
}

func TestPage404GenerationStep_MissingRequired(t *testing.T) {
	step := &Page404GenerationStep{}
	tests := []struct {
		name      string
		artifacts map[string]any
	}{
		{
			name:      "missing design_system",
			artifacts: map[string]any{"html_raw": "<html></html>", "css_content": "body{}", "js_content": "console.log();"},
		},
		{
			name:      "missing html_raw",
			artifacts: map[string]any{"design_system": "{}", "css_content": "body{}", "js_content": "console.log();"},
		},
		{
			name:      "missing css_content",
			artifacts: map[string]any{"design_system": "{}", "html_raw": "<html></html>", "js_content": "console.log();"},
		},
		{
			name:      "missing js_content",
			artifacts: map[string]any{"design_system": "{}", "html_raw": "<html></html>", "css_content": "body{}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &PipelineState{
				Artifacts:     tt.artifacts,
				LLMClient:     &fakeLLMFor404{},
				PromptManager: &fakePromptManagerFor404{},
				AppendLog:     func(s string) {},
			}

			_, err := step.Execute(context.Background(), state)
			if err == nil {
				t.Fatalf("expected error for missing required artifact, got nil")
			}
			if !strings.Contains(err.Error(), "обязательны для 404 страницы") {
				t.Errorf("expected error about required artifacts, got: %v", err)
			}
		})
	}
}

func TestPage404GenerationStep_HTMLWithCodeBlocks(t *testing.T) {
	llm := &fakeLLMFor404WithResponse{response: "```html\n<html><body>404</body></html>\n```"}
	step := &Page404GenerationStep{}
	state := &PipelineState{
		Artifacts: map[string]any{
			"design_system": `{"colors": {"primary": "#000"}}`,
			"html_raw":      "<html><body>Main</body></html>",
			"css_content":   "body{}",
			"js_content":    "console.log();",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerFor404{},
		AppendLog:     func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	html404 := artifacts["404_html"].(string)
	// Проверяем, что код-блоки удалены
	if strings.Contains(html404, "```") {
		t.Errorf("expected code blocks to be removed from 404_html, got: %s", html404)
	}
}
