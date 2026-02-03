package pipeline

import (
	"context"
	"fmt"
	"strings"

	"obzornik-pbn-generator/internal/llm"
)

// CSSGenerationStep реализует генерацию итоговых стилей на основе дизайн-системы и HTML
type CSSGenerationStep struct{}

func (s *CSSGenerationStep) Name() string {
	return StepCSSGeneration
}

func (s *CSSGenerationStep) ArtifactKey() string {
	return "css_content"
}

func (s *CSSGenerationStep) Progress() int {
	return 97
}

func (s *CSSGenerationStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало генерации CSS")

	designSystem, ok := state.Context["design_system"].(string)
	if !ok || strings.TrimSpace(designSystem) == "" {
		if v, ok := state.Artifacts["design_system"].(string); ok {
			designSystem = v
		}
	}
	htmlRaw, ok := state.Context["html_raw"].(string)
	if !ok || strings.TrimSpace(htmlRaw) == "" {
		if v, ok := state.Artifacts["html_raw"].(string); ok {
			htmlRaw = v
		}
	}

	if strings.TrimSpace(designSystem) == "" {
		return nil, fmt.Errorf("design_system is required for css generation")
	}
	if strings.TrimSpace(htmlRaw) == "" {
		return nil, fmt.Errorf("html_raw is required for css generation")
	}

	promptID, promptBody, promptModel, err := state.PromptManager.GetPromptByStage(ctx, "css_generation")
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt for css_generation: %w", err)
	}
	state.AppendLog(fmt.Sprintf("Используется промпт: %s для этапа css_generation", promptID))
	if promptModel != "" {
		state.AppendLog(fmt.Sprintf("Используется модель LLM: %s", promptModel))
	}

	variables := map[string]string{
		"design_system": designSystem,
		"html_raw":      htmlRaw,
	}
	cssPrompt := llm.BuildPrompt(promptBody, "", variables)

	preview := cssPrompt
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	state.AppendLog(fmt.Sprintf("Промпт для CSS (первые 500 символов): %s", preview))

	modelToUse := promptModel
	if modelToUse == "" {
		modelToUse = state.DefaultModel
	}

	rawCSS, err := state.LLMClient.Generate(ctx, "css_generation", cssPrompt, modelToUse)
	if err != nil {
		return nil, fmt.Errorf("css generation failed: %w", err)
	}

	clean := strings.TrimSpace(rawCSS)
	clean = strings.TrimPrefix(clean, "```css")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	files := mergeGeneratedFiles(state.Artifacts["generated_files"], []GeneratedFile{
		{Path: "style.css", Content: clean},
	})

	artifacts := map[string]any{
		"css_content":           clean,
		"generated_files":       files,
		"css_generation_prompt": formatPromptForArtifact(cssPrompt),
	}

	state.Context["css_content"] = clean
	state.AppendLog("Генерация CSS завершена")

	return artifacts, nil
}
