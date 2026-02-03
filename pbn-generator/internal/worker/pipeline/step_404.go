package pipeline

import (
	"context"
	"fmt"
	"strings"

	"obzornik-pbn-generator/internal/llm"
)

// Page404GenerationStep генерирует полноценный 404.html в стиле сайта
// Требует: design_system, html_raw, css_content, js_content
// Артефакты: 404_html, generated_files (добавляет 404.html)
type Page404GenerationStep struct{}

func (s *Page404GenerationStep) Name() string { return Step404Page }

func (s *Page404GenerationStep) ArtifactKey() string { return "404_html" }

func (s *Page404GenerationStep) Progress() int { return 97 }

func (s *Page404GenerationStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало генерации 404 страницы")

	designSystem, _ := state.Artifacts["design_system"].(string)
	htmlRaw, _ := state.Artifacts["html_raw"].(string)
	cssContent, _ := state.Artifacts["css_content"].(string)
	jsContent, _ := state.Artifacts["js_content"].(string)

	if strings.TrimSpace(designSystem) == "" || strings.TrimSpace(htmlRaw) == "" || strings.TrimSpace(cssContent) == "" || strings.TrimSpace(jsContent) == "" {
		return nil, fmt.Errorf("design_system, html_raw, css_content и js_content обязательны для 404 страницы")
	}

	promptID, promptBody, promptModel, err := state.PromptManager.GetPromptByStage(ctx, Step404Page)
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt for 404_page: %w", err)
	}
	if promptID != "" {
		state.AppendLog(fmt.Sprintf("Используется промпт: %s для этапа 404_page", promptID))
	}
	if promptModel != "" {
		state.AppendLog(fmt.Sprintf("Используется модель LLM: %s", promptModel))
	}

	variables := map[string]string{
		"design_system": designSystem,
		"html_raw":      htmlRaw,
		"css_content":   cssContent,
		"js_content":    jsContent,
	}
	prompt := llm.BuildPrompt(promptBody, "", variables)
	preview := prompt
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	state.AppendLog(fmt.Sprintf("Промпт для 404 (первые 500 символов): %s", preview))

	model := promptModel
	if model == "" {
		model = state.DefaultModel
	}

	raw, err := state.LLMClient.Generate(ctx, Step404Page, prompt, model)
	if err != nil {
		return nil, fmt.Errorf("404 generation failed: %w", err)
	}

	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```html")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	files := mergeGeneratedFiles(state.Artifacts["generated_files"], []GeneratedFile{
		{Path: "404.html", Content: clean},
	})

	return map[string]any{
		"404_html":        clean,
		"generated_files": files,
	}, nil
}
