package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"obzornik-pbn-generator/internal/llm"
)

// KeywordGenerationStep analyzes archived website content via LLM to extract
// a theme and generate relevant keywords. It randomly selects one keyword
// and sets it as the domain's MainKeyword for subsequent SERP analysis.
// This is the second step for webarchive_single generation.
type KeywordGenerationStep struct{}

func (s *KeywordGenerationStep) Name() string       { return StepKeywordGeneration }
func (s *KeywordGenerationStep) ArtifactKey() string { return "generated_keyword" }
func (s *KeywordGenerationStep) Progress() int       { return 6 }

const (
	defaultThemePrompt = `Ты — SEO-аналитик. Проанализируй текст, извлечённый из архивной версии сайта.
Извлеки основную тематику и создай краткое описание.

Текст: {{clean_text}}
Язык: {{language}}

Ответь строго в формате JSON:
{"topic": "1-2 слова описывающие тематику", "summary": "2-3 предложения описывающие содержание сайта"}`

	defaultKeywordPrompt = `Сгенерируй 15 популярных поисковых запросов (ключевых фраз) для тематики "{{topic}}" на языке {{language}} для рынка {{country}}.

ВАЖНО:
- НЕ заголовки статей, а именно поисковые запросы (2-5 слов)
- Фокус на информационный и коммерческий интент
- Реальные запросы, которые пользователи вводят в Google

Ответь строго в формате JSON:
{"keywords": ["запрос 1", "запрос 2", "запрос 3", "запрос 4", "запрос 5", "запрос 6", "запрос 7", "запрос 8", "запрос 9", "запрос 10", "запрос 11", "запрос 12", "запрос 13", "запрос 14", "запрос 15"]}`

	keywordGenMaxCleanText = 150000 // Max chars of clean text to send to LLM for theme extraction
)

func (s *KeywordGenerationStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало генерации ключевого слова из архивного контента")

	// 1. Get clean text from wayback data
	combinedText := ""
	if wd, ok := state.Context["wayback_data"].(map[string]any); ok {
		if ct, ok := wd["combined_text"].(string); ok {
			combinedText = ct
		}
	}
	if combinedText == "" {
		return nil, fmt.Errorf("no wayback_data combined_text available for keyword generation")
	}

	// Truncate clean text for LLM
	cleanTextForLLM, truncated := truncateRunes(combinedText, keywordGenMaxCleanText)
	if truncated {
		state.AppendLog("clean_text усечён до лимита для LLM")
	}

	lang := state.Domain.TargetLanguage
	if lang == "" {
		lang = "sv"
	}
	country := state.Domain.TargetCountry
	if country == "" {
		country = "se"
	}

	// 2. LLM call 1: theme extraction
	topic, summary, themePromptFull, err := s.extractTheme(ctx, state, cleanTextForLLM, lang)
	if err != nil {
		return nil, fmt.Errorf("theme extraction failed: %w", err)
	}
	state.AppendLog(fmt.Sprintf("Извлечена тема: %s", topic))

	// 3. LLM call 2: keyword generation
	keywords, kwPromptFull, err := s.generateKeywords(ctx, state, topic, lang, country)
	if err != nil {
		return nil, fmt.Errorf("keyword generation failed: %w", err)
	}
	if len(keywords) == 0 {
		return nil, fmt.Errorf("LLM returned empty keywords list")
	}
	state.AppendLog(fmt.Sprintf("Сгенерировано %d ключевых запросов", len(keywords)))

	// 4. Select random keyword
	selected := keywords[rand.Intn(len(keywords))]
	state.AppendLog(fmt.Sprintf("Выбран ключевой запрос: %s", selected))

	// 5. Update domain keyword in memory and DB
	state.Domain.MainKeyword = selected
	if err := state.DomainStore.UpdateKeyword(ctx, state.DomainID, selected); err != nil {
		state.AppendLog(fmt.Sprintf("Предупреждение: не удалось обновить keyword в БД: %v", err))
	}

	// 6. Build artifacts
	keywordData := map[string]any{
		"topic":              topic,
		"summary":            summary,
		"keyword_candidates": keywords,
		"selected_keyword":   selected,
	}

	artifacts := map[string]any{
		"generated_keyword":      keywordData,
		"wayback_theme_prompt":   formatPromptForArtifact(themePromptFull),
		"wayback_keyword_prompt": formatPromptForArtifact(kwPromptFull),
	}
	state.Context["generated_keyword"] = keywordData

	return artifacts, nil
}

// extractTheme calls LLM to analyze archived content and extract a topic + summary.
func (s *KeywordGenerationStep) extractTheme(ctx context.Context, state *PipelineState, cleanText, lang string) (topic, summary, promptUsed string, err error) {
	// Load prompt from DB or use default
	_, promptBody, promptModel, promptErr := state.PromptManager.GetPromptByStage(ctx, "wayback_theme_extraction")
	if promptErr != nil {
		state.AppendLog(fmt.Sprintf("Используется дефолтный промпт для wayback_theme_extraction: %v", promptErr))
		promptBody = defaultThemePrompt
	}

	variables := map[string]string{
		"clean_text": cleanText,
		"language":   lang,
	}
	fullPrompt := llm.BuildPrompt(promptBody, "", variables)

	model := promptModel
	if model == "" {
		model = state.DefaultModel
	}

	response, err := state.LLMClient.Generate(ctx, "wayback_theme_extraction", fullPrompt, model)
	if err != nil {
		return "", "", fullPrompt, fmt.Errorf("LLM theme extraction error: %w", err)
	}

	// Parse JSON from response
	topic, summary, err = parseThemeResponse(response)
	if err != nil {
		return "", "", fullPrompt, fmt.Errorf("failed to parse theme response: %w", err)
	}

	return topic, summary, fullPrompt, nil
}

// generateKeywords calls LLM to generate search keywords from a topic.
func (s *KeywordGenerationStep) generateKeywords(ctx context.Context, state *PipelineState, topic, lang, country string) (keywords []string, promptUsed string, err error) {
	// Load prompt from DB or use default
	_, promptBody, promptModel, promptErr := state.PromptManager.GetPromptByStage(ctx, "wayback_keyword_generation")
	if promptErr != nil {
		state.AppendLog(fmt.Sprintf("Используется дефолтный промпт для wayback_keyword_generation: %v", promptErr))
		promptBody = defaultKeywordPrompt
	}

	variables := map[string]string{
		"topic":    topic,
		"language": lang,
		"country":  country,
	}
	fullPrompt := llm.BuildPrompt(promptBody, "", variables)

	model := promptModel
	if model == "" {
		model = state.DefaultModel
	}

	response, err := state.LLMClient.Generate(ctx, "wayback_keyword_generation", fullPrompt, model)
	if err != nil {
		return nil, fullPrompt, fmt.Errorf("LLM keyword generation error: %w", err)
	}

	// Parse JSON from response
	keywords, err = parseKeywordsResponse(response)
	if err != nil {
		return nil, fullPrompt, fmt.Errorf("failed to parse keywords response: %w", err)
	}

	return keywords, fullPrompt, nil
}

// parseThemeResponse extracts topic and summary from LLM JSON response.
func parseThemeResponse(response string) (topic, summary string, err error) {
	cleaned := stripCodeFence(response)

	var result struct {
		Topic   string `json:"topic"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return "", "", fmt.Errorf("invalid JSON: %w (raw: %.500s)", err, cleaned)
	}

	topic = strings.TrimSpace(result.Topic)
	summary = strings.TrimSpace(result.Summary)

	if topic == "" {
		return "", "", fmt.Errorf("empty topic in LLM response")
	}

	return topic, summary, nil
}

// parseKeywordsResponse extracts keywords array from LLM JSON response.
func parseKeywordsResponse(response string) ([]string, error) {
	cleaned := stripCodeFence(response)

	// Try {"keywords": [...]}
	var result struct {
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(cleaned), &result); err == nil && len(result.Keywords) > 0 {
		return filterNonEmpty(result.Keywords), nil
	}

	// Try {"topics": [...]} (n8n compatibility)
	var altResult struct {
		Topics []string `json:"topics"`
	}
	if err := json.Unmarshal([]byte(cleaned), &altResult); err == nil && len(altResult.Topics) > 0 {
		return filterNonEmpty(altResult.Topics), nil
	}

	// Try plain array [...]
	var arr []string
	if err := json.Unmarshal([]byte(cleaned), &arr); err == nil && len(arr) > 0 {
		return filterNonEmpty(arr), nil
	}

	return nil, fmt.Errorf("could not parse keywords from LLM response: %.500s", cleaned)
}

// stripCodeFence removes markdown code fences from LLM response.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// filterNonEmpty removes empty strings from a slice.
func filterNonEmpty(ss []string) []string {
	var out []string
	for _, s := range ss {
		if v := strings.TrimSpace(s); v != "" {
			out = append(out, v)
		}
	}
	return out
}
