package pipeline

import (
	"context"
	"fmt"
	"time"

	"obzornik-pbn-generator/internal/llm"
)

// ContentGenerationStep реализует шаг генерации контента
type ContentGenerationStep struct{}

func (s *ContentGenerationStep) Name() string {
	return StepContentGeneration
}

func (s *ContentGenerationStep) ArtifactKey() string {
	return "content_markdown" // Если есть content_markdown, значит шаг выполнен
}

func (s *ContentGenerationStep) Progress() int {
	return 70 // Прогресс после выполнения этого шага
}

func (s *ContentGenerationStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало генерации контента")

	// Проверяем, что есть ТЗ
	technicalSpec, ok := state.Context["technical_spec"].(string)
	if !ok || technicalSpec == "" {
		return nil, fmt.Errorf("technical_spec not found in context, cannot generate content")
	}

	domain := state.Domain.URL
	lang := state.Domain.TargetLanguage
	if lang == "" {
		lang = "sv"
	}

	// Получаем промпт
	contentPromptID, contentSystemPrompt, contentModel, err := state.PromptManager.GetPromptByStage(ctx, "content_generation")
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt for content_generation stage: %w", err)
	}

	state.AppendLog(fmt.Sprintf("Используется промпт: %s для этапа content_generation", contentPromptID))
	if contentModel != "" {
		state.AppendLog(fmt.Sprintf("Используется модель LLM: %s", contentModel))
	}

	// Подготавливаем переменные
	currentDate := time.Now().Format("2006-01-02")
	contentVariables := map[string]string{
		"domain":         domain,
		"lang":           lang,
		"date":           currentDate,
		"technical_spec": technicalSpec,
	}

	// Строим промпт
	contentPrompt := llm.BuildPrompt(contentSystemPrompt, "", contentVariables)

	// Логируем первые 500 символов промпта
	contentPromptPreview := contentPrompt
	if len(contentPromptPreview) > 500 {
		contentPromptPreview = contentPromptPreview[:500] + "..."
	}
	state.AppendLog(fmt.Sprintf("Промпт для генерации контента (первые 500 символов): %s", contentPromptPreview))

	// Определяем модель
	contentModelToUse := contentModel
	if contentModelToUse == "" {
		contentModelToUse = state.DefaultModel
	}

	// Выполняем запрос к LLM
	contentMarkdown, err := state.LLMClient.Generate(ctx, "content_generation", contentPrompt, contentModelToUse)
	if err != nil {
		return nil, fmt.Errorf("content generation failed: %w", err)
	}

	state.AppendLog("Генерация контента завершена")

	// Сохраняем результаты
	artifacts := make(map[string]any)
	if contentMarkdown != "" {
		artifacts["content_generation_prompt"] = formatPromptForArtifact(contentPrompt)
		artifacts["content_markdown"] = contentMarkdown

		// Сохраняем в context для следующих шагов
		state.Context["content_markdown"] = contentMarkdown
	}

	return artifacts, nil
}
