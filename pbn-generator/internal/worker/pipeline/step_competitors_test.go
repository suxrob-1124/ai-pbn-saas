package pipeline

import (
	"context"
	"fmt"
	"testing"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

type fakeLLMForCompetitors struct {
	genCalled bool
}

func (f *fakeLLMForCompetitors) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	f.genCalled = true
	return `# Competitor Analysis

## Key Findings
- Top sites use structured content
- Average word count: 2000 words
`, nil
}

func (f *fakeLLMForCompetitors) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

type fakePromptManagerForCompetitors struct{}

func (f *fakePromptManagerForCompetitors) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "prompt-competitors-v2", "Analyze competitors for {{keyword}} in {{country}}", "gemini-2.5-pro", nil
}

func TestCompetitorAnalysisStep(t *testing.T) {
	llm := &fakeLLMForCompetitors{}
	step := &CompetitorAnalysisStep{}
	logs := make([]string, 0)

	state := &PipelineState{
		Context: map[string]any{
			"serp_data": map[string]any{
				"analysis_csv": "url,word_count\nsite1.com,2000",
				"contents_txt": "Site 1 content...",
			},
		},
		Domain: &sqlstore.Domain{
			MainKeyword:    "test keyword",
			TargetCountry:  "US",
			TargetLanguage: "en",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForCompetitors{},
		DefaultModel:  "gemini-2.5-pro",
		AppendLog: func(s string) {
			logs = append(logs, s)
		},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	// Проверяем, что LLM был вызван
	if !llm.genCalled {
		t.Fatalf("expected LLM Generate to be called")
	}

	// Проверяем наличие competitor_analysis
	competitorAnalysis, ok := artifacts["competitor_analysis"].(string)
	if !ok || competitorAnalysis == "" {
		t.Fatalf("expected competitor_analysis, got %#v", artifacts["competitor_analysis"])
	}

	// Проверяем наличие llm_analysis (для обратной совместимости)
	llmAnalysis, ok := artifacts["llm_analysis"].(string)
	if !ok || llmAnalysis == "" {
		t.Fatalf("expected llm_analysis, got %#v", artifacts["llm_analysis"])
	}

	// Проверяем, что они одинаковые
	if competitorAnalysis != llmAnalysis {
		t.Errorf("expected competitor_analysis and llm_analysis to be the same")
	}

	// Проверяем, что результаты сохранены в контекст
	if state.Context["competitor_analysis"] != competitorAnalysis {
		t.Errorf("expected competitor_analysis to be saved in context")
	}
	if state.Context["llm_analysis"] != llmAnalysis {
		t.Errorf("expected llm_analysis to be saved in context")
	}
}

func TestCompetitorAnalysisStep_WithOldKeys(t *testing.T) {
	llm := &fakeLLMForCompetitors{}
	step := &CompetitorAnalysisStep{}
	state := &PipelineState{
		Context: map[string]any{
			"analysis_csv": "url,word_count\nsite1.com,2000",
			"contents_txt": "Site 1 content...",
		},
		Domain: &sqlstore.Domain{
			MainKeyword:   "test keyword",
			TargetCountry: "US",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForCompetitors{},
		AppendLog:     func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	// Проверяем, что шаг работает со старыми ключами
	competitorAnalysis, ok := artifacts["competitor_analysis"].(string)
	if !ok || competitorAnalysis == "" {
		t.Fatalf("expected competitor_analysis, got %#v", artifacts["competitor_analysis"])
	}
}

func TestCompetitorAnalysisStep_EmptyData(t *testing.T) {
	llm := &fakeLLMForCompetitors{}
	step := &CompetitorAnalysisStep{}
	state := &PipelineState{
		Context: map[string]any{},
		Domain: &sqlstore.Domain{
			MainKeyword:   "test keyword",
			TargetCountry: "US",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForCompetitors{},
		AppendLog:     func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	// Шаг не должен падать, даже если данных нет (только предупреждение)
	if err != nil {
		t.Fatalf("expected no error even with empty data, got: %v", err)
	}

	// Проверяем, что LLM все равно был вызван
	if !llm.genCalled {
		t.Fatalf("expected LLM Generate to be called even with empty data")
	}

	// Проверяем наличие результатов
	competitorAnalysis, ok := artifacts["competitor_analysis"].(string)
	if !ok || competitorAnalysis == "" {
		t.Fatalf("expected competitor_analysis even with empty input, got %#v", artifacts["competitor_analysis"])
	}
}

func TestCompetitorAnalysisStep_LLMError(t *testing.T) {
	llm := &fakeLLMForCompetitorsError{}
	step := &CompetitorAnalysisStep{}
	state := &PipelineState{
		Context: map[string]any{
			"analysis_csv": "url,word_count\nsite1.com,2000",
		},
		Domain: &sqlstore.Domain{
			MainKeyword: "test keyword",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForCompetitors{},
		AppendLog:     func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	// Шаг не должен падать при ошибке LLM (возвращает пустой результат)
	if err != nil {
		t.Fatalf("expected no error even with LLM error, got: %v", err)
	}

	// Проверяем, что возвращен пустой результат
	if len(artifacts) != 0 {
		t.Errorf("expected empty artifacts on LLM error, got: %#v", artifacts)
	}
}

func TestCompetitorAnalysisStep_FallbackPrompt(t *testing.T) {
	llm := &fakeLLMForCompetitors{}
	step := &CompetitorAnalysisStep{}
	state := &PipelineState{
		Context: map[string]any{
			"analysis_csv": "url,word_count\nsite1.com,2000",
		},
		Domain: &sqlstore.Domain{
			MainKeyword: "test keyword",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForCompetitorsError{},
		AppendLog:     func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	// Проверяем, что шаг работает с дефолтным промптом
	competitorAnalysis, ok := artifacts["competitor_analysis"].(string)
	if !ok || competitorAnalysis == "" {
		t.Fatalf("expected competitor_analysis with fallback prompt, got %#v", artifacts["competitor_analysis"])
	}
}

type fakeLLMForCompetitorsError struct{}

func (f *fakeLLMForCompetitorsError) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	return "", fmt.Errorf("LLM API error")
}

func (f *fakeLLMForCompetitorsError) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

type fakePromptManagerForCompetitorsError struct{}

func (f *fakePromptManagerForCompetitorsError) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "", "", "", fmt.Errorf("prompt not found")
}
