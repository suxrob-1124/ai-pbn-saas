package pipeline

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"obzornik-pbn-generator/internal/llm"
)

// TechnicalSpecStep реализует шаг разработки технического задания
type TechnicalSpecStep struct{}

func (s *TechnicalSpecStep) Name() string {
	return StepTechnicalSpec
}

func (s *TechnicalSpecStep) ArtifactKey() string {
	return "technical_spec" // Если есть technical_spec, значит шаг выполнен
}

func (s *TechnicalSpecStep) Progress() int {
	return 60 // Прогресс после выполнения этого шага
}

func (s *TechnicalSpecStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало разработки технического задания")

	// Проверяем, что есть LLM анализ
	llmAnalysis, ok := state.Context["llm_analysis"].(string)
	if !ok || llmAnalysis == "" {
		return nil, fmt.Errorf("llm_analysis not found in context, cannot generate technical spec")
	}

	keyword := state.Domain.MainKeyword
	country := state.Domain.TargetCountry
	lang := state.Domain.TargetLanguage
	resolution := resolveBrandResolution(state, "", "")
	logBrandResolution(state, resolution)

	// Получаем промпт
	specPromptID, specSystemPrompt, specModel, err := state.PromptManager.GetPromptByStage(ctx, "technical_spec")
	if err != nil {
		state.AppendLog(fmt.Sprintf("Предупреждение: не удалось загрузить промпт для этапа technical_spec: %v", err))
		// Используем дефолтный промпт
		specSystemPrompt = "Ты — SEO-аналитик. На основе анализа конкурентов создай техническое задание для копирайтера."
		specPromptID = ""
		specModel = ""
	} else {
		state.AppendLog(fmt.Sprintf("Используется промпт: %s для этапа technical_spec", specPromptID))
		if specModel != "" {
			state.AppendLog(fmt.Sprintf("Используется модель LLM: %s", specModel))
		}
	}

	// Генерируем случайные ID для выбора вариантов (как в n8n)
	rand.Seed(time.Now().UnixNano())
	randomID := func(max int) int {
		return rand.Intn(max) + 1
	}

	archetypeID := randomID(4)
	imageStyleID := randomID(4)
	headerElementID := randomID(4)
	footerVariantID := randomID(4)

	state.AppendLog(fmt.Sprintf("Сгенерированы случайные ID: archetype=%d, image_style=%d, header_element=%d, footer_variant=%d",
		archetypeID, imageStyleID, headerElementID, footerVariantID))

	// Подготавливаем переменные
	specVariables := map[string]string{
		"llm_analysis":      llmAnalysis,
		"keyword":           keyword,
		"country":           country,
		"language":          lang,
		"archetype_id":      fmt.Sprintf("%d", archetypeID),
		"image_style_id":    fmt.Sprintf("%d", imageStyleID),
		"header_element_id": fmt.Sprintf("%d", headerElementID),
		"footer_variant_id": fmt.Sprintf("%d", footerVariantID),
	}
	for k, v := range buildBrandPromptVars(resolution) {
		specVariables[k] = v
	}

	// Строим промпт
	specPrompt := llm.BuildPrompt(specSystemPrompt, "", specVariables)

	// Логируем первые 500 символов промпта
	specPromptPreview := specPrompt
	if len(specPromptPreview) > 500 {
		specPromptPreview = specPromptPreview[:500] + "..."
	}
	state.AppendLog(fmt.Sprintf("Промпт для ТЗ (первые 500 символов): %s", specPromptPreview))

	// Определяем модель
	specModelToUse := specModel
	if specModelToUse == "" {
		specModelToUse = state.DefaultModel
	}

	// Выполняем запрос к LLM + валидация brand-policy
	technicalSpec, validation, err := generateWithBrandGuard(ctx, state, "technical_spec", specPrompt, specModelToUse, resolution)
	if err != nil {
		return nil, fmt.Errorf("technical spec generation failed: %w", err)
	}

	state.AppendLog("Техническое задание разработано")

	// Сохраняем результаты
	artifacts := make(map[string]any)
	if technicalSpec != "" {
		artifacts["technical_spec_prompt"] = formatPromptForArtifact(specPrompt)
		artifacts["technical_spec"] = technicalSpec
		artifacts["brand_resolution"] = resolution
		artifacts["brand_validation"] = mergeBrandValidation(state.Artifacts["brand_validation"], "technical_spec", validation)

		// Сохраняем в context для следующих шагов
		state.Context["technical_spec"] = technicalSpec
		state.Context["brand_resolution"] = resolution
	}

	return artifacts, nil
}
