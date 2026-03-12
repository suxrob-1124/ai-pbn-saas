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
	return 18 // Прогресс после выполнения этого шага
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

	// Определяем модель
	modelToUse := promptModel
	if modelToUse == "" {
		modelToUse = state.DefaultModel
	}

	// Порог для переключения в multi-turn режим (chars)
	multiTurnThreshold := envInt("COMPETITOR_MULTITURN_THRESHOLD", 80000)
	totalContentSize := len([]rune(analysisCSV)) + len([]rune(contentsTxt))

	var llmAnalysis string
	var usedPrompt string

	if totalContentSize > multiTurnThreshold && contentsTxt != "" {
		// ── Multi-turn режим ─────────────────────────────────────────────────────
		state.AppendLog(fmt.Sprintf("Контент большой (%d chars), используется multi-turn режим", totalContentSize))

		// System instruction = промпт с заменой метаданных, но БЕЗ данных SERP.
		// Данные будут переданы по частям в turns.
		sysInstruction := buildMultiTurnSystemInstruction(systemPrompt, keyword, country, lang)

		// Разбиваем contentsTxt на чанки по страницам
		pagesPerChunk := envInt("COMPETITOR_MULTITURN_PAGES_PER_CHUNK", 5)
		pageChunks := splitIntoPageChunks(contentsTxt, pagesPerChunk)
		totalChunks := len(pageChunks) + 1 // +1 для CSV чанка

		state.AppendLog(fmt.Sprintf("Multi-turn: %d чанков контента по ~%d страниц", len(pageChunks), pagesPerChunk))

		turns := make([]string, 0, totalChunks+1)

		// Turn 1: CSV метаданные + первый чанк страниц
		var firstTurn strings.Builder
		firstTurn.WriteString(fmt.Sprintf("**Часть 1/%d — Метаданные SERP (CSV)**\n\n", totalChunks))
		firstTurn.WriteString(analysisCSV)
		if len(pageChunks) > 0 {
			firstTurn.WriteString(fmt.Sprintf("\n\n**Часть 1/%d — Контент сайтов (группа 1)**\n\n", totalChunks))
			firstTurn.WriteString(pageChunks[0])
		}
		turns = append(turns, firstTurn.String())

		// Turns 2..N: оставшиеся чанки страниц
		for i, chunk := range pageChunks[1:] {
			turns = append(turns, fmt.Sprintf("**Часть %d/%d — Контент сайтов (группа %d)**\n\n%s",
				i+2, totalChunks, i+2, chunk))
		}

		// Финальный turn: запрос полного анализа
		turns = append(turns, "Все данные переданы. Составь полный структурированный конкурентный анализ, используя все предоставленные данные.")

		usedPrompt = fmt.Sprintf("Режим multi-turn (%d запросов к модели). Системный промпт: %d символов.", len(turns), len(sysInstruction))

		// Логируем первые 500 символов system instruction
		preview, _ := truncateRunes(sysInstruction, 500)
		state.AppendLog(fmt.Sprintf("System instruction (первые 500 символов): %s", preview))

		llmAnalysis, err = state.LLMClient.GenerateMultiTurn(ctx, "competitor_analysis", sysInstruction, turns, modelToUse)
		if err != nil {
			return nil, fmt.Errorf("LLM multi-turn анализ конкурентов не выполнен: %w", err)
		}
		state.AppendLog("LLM multi-turn анализ конкурентов завершен")
	} else {
		// ── Одиночный запрос (стандартный режим) ─────────────────────────────────
		variables := map[string]string{
			"keyword":       keyword,
			"analysis_data": analysisCSV,
			"contents_data": contentsTxt,
			"country":       country,
			"language":      lang,
		}
		analysisPrompt := llm.BuildPrompt(systemPrompt, "", variables)
		usedPrompt = analysisPrompt

		// Логируем первые 500 символов промпта
		promptPreview, _ := truncateRunes(analysisPrompt, 500)
		if len(promptPreview) < len(analysisPrompt) {
			promptPreview += "..."
		}
		state.AppendLog(fmt.Sprintf("Промпт для анализа (первые 500 символов): %s", promptPreview))

		llmAnalysis, err = state.LLMClient.Generate(ctx, "competitor_analysis", analysisPrompt, modelToUse)
		if err != nil {
			return nil, fmt.Errorf("LLM анализ конкурентов не выполнен: %w", err)
		}
		state.AppendLog("LLM анализ конкурентов завершен")
	}

	// Сохраняем результаты
	artifacts := make(map[string]any)
	if llmAnalysis != "" {
		artifacts["llm_analysis_prompt"] = formatPromptForArtifact(usedPrompt)
		artifacts["competitor_analysis"] = llmAnalysis
		// для обратной совместимости
		artifacts["llm_analysis"] = llmAnalysis
		resolution := resolveBrandResolution(state, analysisCSV, contentsTxt)
		artifacts["brand_resolution"] = resolution
		logBrandResolution(state, resolution)

		// Сохраняем в context для следующих шагов
		state.Context["competitor_analysis"] = llmAnalysis
		state.Context["llm_analysis"] = llmAnalysis
		state.Context["brand_resolution"] = resolution
	}

	return artifacts, nil
}

// buildMultiTurnSystemInstruction строит system instruction для multi-turn сессии:
// заменяет метаданные (keyword/country/language), убирает плейсхолдеры данных
// и добавляет инструкцию о поэтапной передаче данных.
func buildMultiTurnSystemInstruction(systemPrompt, keyword, country, lang string) string {
	inst := systemPrompt

	// Заменяем метаданные
	for _, kv := range [][2]string{
		{"keyword", keyword},
		{"country", country},
		{"language", lang},
	} {
		inst = strings.ReplaceAll(inst, "{{"+kv[0]+"}}", kv[1])
		inst = strings.ReplaceAll(inst, "{{ "+kv[0]+" }}", kv[1])
	}

	// Убираем плейсхолдеры данных — данные придут в turns
	for _, placeholder := range []string{"analysis_data", "contents_data"} {
		inst = strings.ReplaceAll(inst, "{{"+placeholder+"}}", "")
		inst = strings.ReplaceAll(inst, "{{ "+placeholder+" }}", "")
	}
	inst = strings.TrimSpace(inst)

	// Добавляем инструкцию о поэтапной передаче
	inst += "\n\n---\nДанные SERP анализа будут переданы по частям в следующих сообщениях. " +
		"После каждой части кратко (1-2 предложения) фиксируй ключевые наблюдения. " +
		"После получения всех частей составь полный структурированный анализ."

	return inst
}

// splitIntoPageChunks разбивает contentsTxt (с разделителями «--- САЙТ N ---») на чанки по pagesPerChunk страниц.
func splitIntoPageChunks(contents string, pagesPerChunk int) []string {
	if pagesPerChunk <= 0 {
		pagesPerChunk = 5
	}
	const sep = "--- САЙТ "
	parts := strings.Split(contents, sep)
	if len(parts) <= 1 {
		// Нет разделителей — возвращаем как один чанк
		return []string{contents}
	}

	header := parts[0] // «# Текстовое содержимое Top-20\n...»
	pages := parts[1:] // каждый элемент начинается с «N (URL: ...) ---\n...»

	var chunks []string
	for start := 0; start < len(pages); start += pagesPerChunk {
		end := start + pagesPerChunk
		if end > len(pages) {
			end = len(pages)
		}
		var chunk strings.Builder
		if start == 0 {
			chunk.WriteString(header)
		}
		for _, page := range pages[start:end] {
			chunk.WriteString(sep)
			chunk.WriteString(page)
		}
		chunks = append(chunks, chunk.String())
	}
	return chunks
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
