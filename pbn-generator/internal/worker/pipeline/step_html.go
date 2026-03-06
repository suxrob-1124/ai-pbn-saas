package pipeline

import (
	"context"
	"fmt"
	"strings"

	"obzornik-pbn-generator/internal/llm"
)

// HTMLGenerationStep отвечает за сборку HTML-каркаса
type HTMLGenerationStep struct{}

func (s *HTMLGenerationStep) Name() string {
	return StepHTMLGeneration
}

func (s *HTMLGenerationStep) ArtifactKey() string {
	return "html_raw"
}

func (s *HTMLGenerationStep) Progress() int {
	return 68
}

func (s *HTMLGenerationStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало генерации HTML-каркаса")

	designSystem, ok := state.Context["design_system"].(string)
	if !ok || strings.TrimSpace(designSystem) == "" {
		if ds, ok := state.Artifacts["design_system"].(string); ok {
			designSystem = ds
		}
	}
	contentMarkdown, _ := state.Context["content_markdown"].(string)
	logoSVG, _ := state.Context["logo_svg"].(string)
	if logoSVG == "" {
		if v, ok := state.Artifacts["logo_svg"].(string); ok {
			logoSVG = v
		}
	}
	faviconTag, _ := state.Context["favicon_tag"].(string)
	if faviconTag == "" {
		if v, ok := state.Artifacts["favicon_tag"].(string); ok {
			faviconTag = v
		}
	}

	if strings.TrimSpace(designSystem) == "" {
		return nil, fmt.Errorf("design_system is required")
	}
	if strings.TrimSpace(contentMarkdown) == "" {
		return nil, fmt.Errorf("content_markdown is required")
	}
	if strings.TrimSpace(logoSVG) == "" {
		return nil, fmt.Errorf("logo_svg is required")
	}
	if strings.TrimSpace(faviconTag) == "" {
		return nil, fmt.Errorf("favicon_tag is required")
	}

	promptID, promptBody, promptModel, err := state.PromptManager.GetPromptByStage(ctx, "html_generation")
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt for html_generation: %w", err)
	}
	state.AppendLog(fmt.Sprintf("Используется промпт: %s для этапа html_generation", promptID))
	if promptModel != "" {
		state.AppendLog(fmt.Sprintf("Используется модель LLM: %s", promptModel))
	}

	variables := map[string]string{
		"design_system":   designSystem,
		"content_markdown": contentMarkdown,
		"logo_svg":        logoSVG,
		"favicon_tag":     faviconTag,
	}

	htmlPrompt := llm.BuildPrompt(promptBody, "", variables)
	preview := htmlPrompt
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	state.AppendLog(fmt.Sprintf("Промпт для HTML (первые 500 символов): %s", preview))

	modelToUse := promptModel
	if modelToUse == "" {
		modelToUse = state.DefaultModel
	}

	rawHTML, err := state.LLMClient.Generate(ctx, "html_generation", htmlPrompt, modelToUse)
	if err != nil {
		return nil, fmt.Errorf("html generation failed: %w", err)
	}

	clean := strings.TrimSpace(rawHTML)
	clean = strings.TrimPrefix(clean, "```html")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	artifacts := map[string]any{
		"html_raw":          clean,
		"html_generation_prompt": formatPromptForArtifact(htmlPrompt),
	}

	state.Context["html_raw"] = clean
	state.AppendLog("Генерация HTML-каркаса завершена")

	return artifacts, nil
}
