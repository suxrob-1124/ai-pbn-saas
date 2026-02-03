package pipeline

import (
	"context"
	"fmt"
	"strings"

	"obzornik-pbn-generator/internal/llm"
)

// LogoGenerationStep реализует шаг генерации логотипа (SVG)
type LogoGenerationStep struct{}

func (s *LogoGenerationStep) Name() string {
	return StepLogoGeneration
}

func (s *LogoGenerationStep) ArtifactKey() string {
	return "logo_svg"
}

func (s *LogoGenerationStep) Progress() int {
	return 90
}

func (s *LogoGenerationStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало генерации логотипа (SVG)")

	designSystem, ok := state.Context["design_system"].(string)
	if !ok || strings.TrimSpace(designSystem) == "" {
		// попробуем взять из артефактов напрямую
		if ds, ok := state.Artifacts["design_system"].(string); ok && strings.TrimSpace(ds) != "" {
			designSystem = ds
		}
	}
	if strings.TrimSpace(designSystem) == "" {
		return nil, fmt.Errorf("design_system is required for logo generation")
	}

	// Получаем промпт
	promptID, promptBody, promptModel, err := state.PromptManager.GetPromptByStage(ctx, "logo_generation")
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt for logo_generation: %w", err)
	}
	state.AppendLog(fmt.Sprintf("Используется промпт: %s для этапа logo_generation", promptID))
	if promptModel != "" {
		state.AppendLog(fmt.Sprintf("Используется модель LLM: %s", promptModel))
	}

	variables := map[string]string{
		"design_system": designSystem,
	}
	logoPrompt := llm.BuildPrompt(promptBody, "", variables)

	preview := logoPrompt
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	state.AppendLog(fmt.Sprintf("Промпт для логотипа (первые 500 символов): %s", preview))

	modelToUse := promptModel
	if modelToUse == "" {
		modelToUse = state.DefaultModel
	}

	rawLogo, err := state.LLMClient.Generate(ctx, "logo_generation", logoPrompt, modelToUse)
	if err != nil {
		return nil, fmt.Errorf("logo generation failed: %w", err)
	}

	// Очистка от ```svg ... ```
	clean := strings.TrimSpace(rawLogo)
	clean = strings.TrimPrefix(clean, "```svg")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	if !strings.HasPrefix(clean, "<svg") {
		state.AppendLog("Предупреждение: LLM ответ не начинается с <svg")
	}

	// Подготовка списка файлов
	files := mergeGeneratedFiles(state.Artifacts["generated_files"], []GeneratedFile{
		{
			Path:    "logo.svg",
			Content: clean,
		},
	})

	artifacts := map[string]any{
		"logo_svg":        clean,
		"favicon_tag":     `<link rel="icon" type="image/svg+xml" href="/logo.svg">`,
		"generated_files": files,
		"logo_prompt":     formatPromptForArtifact(logoPrompt),
	}

	// Сохраняем в контекст
	state.Context["logo_svg"] = clean

	state.AppendLog("Генерация логотипа завершена")
	return artifacts, nil
}
