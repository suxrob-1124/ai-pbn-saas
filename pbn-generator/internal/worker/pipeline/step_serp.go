package pipeline

import (
	"context"
	"fmt"

	"obzornik-pbn-generator/internal/analyzer"
)

// SERPAnalysisStep реализует шаг SERP анализа
type SERPAnalysisStep struct{}

func (s *SERPAnalysisStep) Name() string {
	return StepSERPAnalysis
}

func (s *SERPAnalysisStep) ArtifactKey() string {
	return "serp_data" // Основной ключ артефакта SERP анализа
}

func (s *SERPAnalysisStep) Progress() int {
	return 8 // Прогресс после выполнения этого шага
}

func (s *SERPAnalysisStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало SERP анализа")

	keyword := state.Domain.MainKeyword
	country := state.Domain.TargetCountry
	lang := state.Domain.TargetLanguage

	if keyword == "" {
		return nil, fmt.Errorf("keyword is empty")
	}
	if country == "" {
		country = "se"
	}
	if lang == "" {
		lang = "sv"
	}

	excludes := splitExcludeList(state.Domain.ExcludeDomains.String)

	// Выполняем анализ
	result, err := state.Analyzer.Analyze(ctx, keyword, country, lang, excludes, analyzer.Config{})
	if err != nil {
		return nil, fmt.Errorf("analyzer error: %w", err)
	}

	// Добавляем логи из анализатора
	for _, logLine := range result.Logs {
		state.AppendLog(logLine)
	}

	state.AppendLog("SERP анализ завершен")

	// Сохраняем результаты в artifacts
	artifacts := make(map[string]any)
	serpPayload := map[string]any{
		"analysis_csv":  result.CSV,
		"contents_txt":  result.Contents,
		"serp_raw":      result.RawSerp,
		"keyword":       keyword,
		"country":       country,
		"language":      lang,
	}
	artifacts["serp_data"] = serpPayload
	// дублируем старые ключи для обратной совместимости
	artifacts["analysis_csv"] = result.CSV
	artifacts["contents_txt"] = result.Contents
	if result.RawSerp != nil {
		artifacts["serp_raw"] = result.RawSerp
	}

	// Сохраняем в context для следующих шагов
	state.Context["analysis_csv"] = result.CSV
	state.Context["contents_txt"] = result.Contents
	state.Context["serp_data"] = serpPayload

	return artifacts, nil
}
