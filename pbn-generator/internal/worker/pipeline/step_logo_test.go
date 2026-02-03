package pipeline

import (
	"context"
	"strings"
	"testing"
)

type fakeLLMForLogo struct {
	genCalled bool
	response  string
}

func (f *fakeLLMForLogo) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	f.genCalled = true
	if f.response != "" {
		return f.response, nil
	}
	return `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40"/></svg>`, nil
}

func (f *fakeLLMForLogo) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

type fakePromptManagerForLogo struct{}

func (f *fakePromptManagerForLogo) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "prompt-logo-v2", "Generate SVG logo based on {{design_system}}", "gemini-2.5-pro", nil
}

func TestLogoGenerationStep(t *testing.T) {
	llm := &fakeLLMForLogo{}
	step := &LogoGenerationStep{}
	logs := make([]string, 0)

	state := &PipelineState{
		Artifacts: map[string]any{
			"design_system": `{"colors": {"primary": "#000"}}`,
		},
		Context: map[string]any{
			"design_system": `{"colors": {"primary": "#000"}}`,
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForLogo{},
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

	// Проверяем наличие logo_svg
	logoSVG, ok := artifacts["logo_svg"].(string)
	if !ok || logoSVG == "" {
		t.Fatalf("expected logo_svg, got %#v", artifacts["logo_svg"])
	}

	// Проверяем, что это валидный SVG
	if !strings.HasPrefix(logoSVG, "<svg") {
		t.Errorf("expected logo_svg to start with <svg, got: %s", logoSVG[:min(50, len(logoSVG))])
	}

	// Проверяем наличие favicon_tag
	faviconTag, ok := artifacts["favicon_tag"].(string)
	if !ok || faviconTag == "" {
		t.Fatalf("expected favicon_tag, got %#v", artifacts["favicon_tag"])
	}
	if !strings.Contains(faviconTag, "logo.svg") {
		t.Errorf("expected favicon_tag to contain logo.svg, got: %s", faviconTag)
	}

	// Проверяем generated_files
	files, ok := artifacts["generated_files"].([]GeneratedFile)
	if !ok || len(files) == 0 {
		t.Fatalf("expected generated_files slice, got %#v", artifacts["generated_files"])
	}

	hasLogoFile := false
	for _, f := range files {
		if f.Path == "logo.svg" {
			hasLogoFile = true
			if f.Content != logoSVG {
				t.Errorf("logo.svg content doesn't match logo_svg artifact")
			}
			break
		}
	}
	if !hasLogoFile {
		t.Errorf("expected logo.svg in generated_files")
	}

	// Проверяем, что logo_svg сохранен в контекст
	if state.Context["logo_svg"] != logoSVG {
		t.Errorf("expected logo_svg to be saved in context")
	}
}

func TestLogoGenerationStep_NoDesignSystem(t *testing.T) {
	step := &LogoGenerationStep{}
	state := &PipelineState{
		Artifacts:     map[string]any{},
		Context:       map[string]any{},
		LLMClient:     &fakeLLMForLogo{},
		PromptManager: &fakePromptManagerForLogo{},
		AppendLog:     func(s string) {},
	}

	_, err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatalf("expected error for missing design_system, got nil")
	}
	if !strings.Contains(err.Error(), "design_system is required") {
		t.Errorf("expected error about design_system, got: %v", err)
	}
}

func TestLogoGenerationStep_SVGWithCodeBlocks(t *testing.T) {
	llm := &fakeLLMForLogo{
		response: "```svg\n<svg><rect/></svg>\n```",
	}
	step := &LogoGenerationStep{}
	state := &PipelineState{
		Artifacts: map[string]any{
			"design_system": `{"colors": {"primary": "#000"}}`,
		},
		Context:       map[string]any{},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForLogo{},
		AppendLog:     func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	logoSVG := artifacts["logo_svg"].(string)
	// Проверяем, что код-блоки удалены
	if strings.Contains(logoSVG, "```") {
		t.Errorf("expected code blocks to be removed from logo_svg, got: %s", logoSVG)
	}
	if !strings.HasPrefix(logoSVG, "<svg") {
		t.Errorf("expected logo_svg to start with <svg after cleaning, got: %s", logoSVG)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
