package pipeline

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"obzornik-pbn-generator/internal/llm"
)

// CompetitorAnalysisStep реализует шаг анализа конкурентов
type CompetitorAnalysisStep struct{}

func (s *CompetitorAnalysisStep) Name() string {
	return StepCompetitorAnalysis
}

func (s *CompetitorAnalysisStep) ArtifactKey() string {
	return "competitor_analysis" // Если есть competitor_analysis, значит шаг выполнен
}

func (s *CompetitorAnalysisStep) Progress() int {
	return 50 // Прогресс после выполнения этого шага
}

func (s *CompetitorAnalysisStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало LLM анализа конкурентов")

	// Получаем данные из предыдущих шагов
	analysisCSV := ""
	contentsTxt := ""
	// Пытаемся взять из нового ключа serp_data
	if serpData, ok := state.Context["serp_data"].(map[string]any); ok {
		if v, ok := serpData["analysis_csv"].(string); ok {
			analysisCSV = v
		}
		if v, ok := serpData["contents_txt"].(string); ok {
			contentsTxt = v
		}
	}
	// Фолбэк на старые ключи
	if analysisCSV == "" {
		if v, ok := state.Context["analysis_csv"].(string); ok {
			analysisCSV = v
		}
	}
	if contentsTxt == "" {
		if v, ok := state.Context["contents_txt"].(string); ok {
			contentsTxt = v
		}
	}
	if analysisCSV == "" {
		state.AppendLog("Предупреждение: analysis_csv не найден")
	}
	if contentsTxt == "" {
		state.AppendLog("Предупреждение: contents_txt не найден")
	}

	analysisCSV, csvTruncated := truncateRunes(analysisCSV, envInt("COMPETITOR_ANALYSIS_MAX_CSV_CHARS", 120000))
	if csvTruncated {
		state.AppendLog("analysis_csv truncated to limit")
	}
	contentsTxt, contentsTruncated := truncateRunes(contentsTxt, envInt("COMPETITOR_ANALYSIS_MAX_CONTENT_CHARS", 200000))
	if contentsTruncated {
		state.AppendLog("contents_txt truncated to limit")
	}

	keyword := state.Domain.MainKeyword
	country := state.Domain.TargetCountry
	lang := state.Domain.TargetLanguage

	// Получаем промпт
	promptID, systemPrompt, promptModel, err := state.PromptManager.GetPromptByStage(ctx, "competitor_analysis")
	if err != nil {
		state.AppendLog(fmt.Sprintf("Предупреждение: не удалось загрузить промпт для этапа competitor_analysis: %v, используем дефолтный", err))
		systemPrompt = "Ты — SEO-аналитик. Проанализируй данные о конкурентах и предоставь структурированные выводы."
		promptID = ""
		promptModel = ""
	} else {
		state.AppendLog(fmt.Sprintf("Используется промпт: %s для этапа competitor_analysis", promptID))
		if promptModel != "" {
			state.AppendLog(fmt.Sprintf("Используется модель LLM: %s", promptModel))
		}
	}

	// Подготавливаем переменные
	variables := map[string]string{
		"keyword":       keyword,
		"analysis_data": analysisCSV,
		"contents_data": contentsTxt,
		"country":       country,
		"language":      lang,
	}

	// Строим промпт
	analysisPrompt := llm.BuildPrompt(systemPrompt, "", variables)

	// Логируем первые 500 символов промпта
	promptPreview, _ := truncateRunes(analysisPrompt, 500)
	if len(promptPreview) < len(analysisPrompt) {
		promptPreview += "..."
	}
	state.AppendLog(fmt.Sprintf("Промпт для анализа (первые 500 символов): %s", promptPreview))

	// Определяем модель
	modelToUse := promptModel
	if modelToUse == "" {
		modelToUse = state.DefaultModel
	}

	// Выполняем запрос к LLM
	llmAnalysis, err := state.LLMClient.Generate(ctx, "competitor_analysis", analysisPrompt, modelToUse)
	if err != nil {
		state.AppendLog(fmt.Sprintf("Предупреждение: LLM анализ конкурентов не выполнен: %v", err))
		return map[string]any{}, nil
	}

	state.AppendLog("LLM анализ конкурентов завершен")

	// Сохраняем результаты
	artifacts := make(map[string]any)
	if llmAnalysis != "" {
		artifacts["llm_analysis_prompt"] = formatPromptForArtifact(analysisPrompt)
		artifacts["competitor_analysis"] = llmAnalysis
		// для обратной совместимости
		artifacts["llm_analysis"] = llmAnalysis

		// Сохраняем в context для следующих шагов
		state.Context["competitor_analysis"] = llmAnalysis
		state.Context["llm_analysis"] = llmAnalysis
	}

	return artifacts, nil
}

func envInt(key string, fallback int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	if n, err := strconv.Atoi(val); err == nil && n > 0 {
		return n
	}
	return fallback
}

func truncateRunes(s string, max int) (string, bool) {
	if max <= 0 || s == "" {
		return s, false
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s, false
	}
	return string(runes[:max]), true
}
