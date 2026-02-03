package pipeline

import (
	"context"
	"fmt"
	"math/rand"

	"obzornik-pbn-generator/internal/content"
	"obzornik-pbn-generator/internal/llm"
)

// DesignArchitectureStep реализует шаг разработки дизайн-системы
type DesignArchitectureStep struct{}

func (s *DesignArchitectureStep) Name() string {
	return StepDesignArchitecture
}

func (s *DesignArchitectureStep) ArtifactKey() string {
	return "design_system" // Если есть design_system, значит шаг выполнен
}

func (s *DesignArchitectureStep) Progress() int {
	return 80 // Прогресс после выполнения этого шага
}

func (s *DesignArchitectureStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало разработки дизайн-системы")

	// Проверяем, что есть content_markdown
	contentMarkdown, ok := state.Context["content_markdown"].(string)
	if !ok || contentMarkdown == "" {
		return nil, fmt.Errorf("content_markdown not found in context, cannot generate design system")
	}

	// Генерируем случайные ID для дизайна
	const (
		NUM_STYLES            = 10
		NUM_COLOR_VARIATIONS  = 10
		NUM_FONT_VARIATIONS   = 10
		NUM_LAYOUT_VARIATIONS = 10
	)

	styleID := rand.Intn(NUM_STYLES) + 1
	colorID := rand.Intn(NUM_COLOR_VARIATIONS) + 1
	fontID := rand.Intn(NUM_FONT_VARIATIONS) + 1
	layoutID := rand.Intn(NUM_LAYOUT_VARIATIONS) + 1

	state.AppendLog(fmt.Sprintf("Сгенерированы ID: style_id=%d, color_id=%d, font_id=%d, layout_id=%d", styleID, colorID, fontID, layoutID))

	// Получаем промпт
	designPromptID, designSystemPrompt, designModel, err := state.PromptManager.GetPromptByStage(ctx, "design_architecture")
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt for design_architecture stage: %w", err)
	}

	state.AppendLog(fmt.Sprintf("Используется промпт: %s для этапа design_architecture", designPromptID))
	if designModel != "" {
		state.AppendLog(fmt.Sprintf("Используется модель LLM: %s", designModel))
	}

	// Подготавливаем переменные
	designVariables := map[string]string{
		"style_id":         fmt.Sprintf("%d", styleID),
		"color_id":         fmt.Sprintf("%d", colorID),
		"font_id":          fmt.Sprintf("%d", fontID),
		"layout_id":        fmt.Sprintf("%d", layoutID),
		"content_markdown": contentMarkdown,
	}

	// Строим промпт
	designPrompt := llm.BuildPrompt(designSystemPrompt, "", designVariables)

	// Логируем первые 500 символов промпта
	designPromptPreview := designPrompt
	if len(designPromptPreview) > 500 {
		designPromptPreview = designPromptPreview[:500] + "..."
	}
	state.AppendLog(fmt.Sprintf("Промпт для дизайн-системы (первые 500 символов): %s", designPromptPreview))

	// Определяем модель
	designModelToUse := designModel
	if designModelToUse == "" {
		designModelToUse = state.DefaultModel
	}

	// Выполняем запрос к LLM
	designSystemRaw, err := state.LLMClient.Generate(ctx, "design_architecture", designPrompt, designModelToUse)
	if err != nil {
		return nil, fmt.Errorf("design architecture generation failed: %w", err)
	}

	state.AppendLog("Генерация дизайн-системы завершена")

	// Парсим JSON и генерируем PFX
	designSystem, err := content.ParseDesignSystem(designSystemRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse design system JSON: %w", err)
	}

	// Конвертируем обратно в JSON с PFX
	designSystemBytes, err := designSystem.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize design system: %w", err)
	}

	designSystemJSON := string(designSystemBytes)
	state.AppendLog(fmt.Sprintf("PFX сгенерирован: %s", designSystem.PFX))

	// Сохраняем результаты
	artifacts := make(map[string]any)
	if designSystemJSON != "" {
		artifacts["design_architecture_prompt"] = formatPromptForArtifact(designPrompt)
		artifacts["design_system"] = designSystemJSON

		// Сохраняем в context для следующих шагов
		state.Context["design_system"] = designSystemJSON
	}

	return artifacts, nil
}
