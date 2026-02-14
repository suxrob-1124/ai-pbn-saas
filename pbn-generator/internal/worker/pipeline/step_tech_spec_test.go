package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

type fakeLLMForTechSpec struct {
	genCalled bool
	calls     int
	responses []string
}

func (f *fakeLLMForTechSpec) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	f.genCalled = true
	f.calls++
	if len(f.responses) > 0 {
		idx := f.calls - 1
		if idx >= len(f.responses) {
			idx = len(f.responses) - 1
		}
		return f.responses[idx], nil
	}
	return `# Technical Specification

## Requirements
- Generate high-quality content
- Use SEO best practices
`, nil
}

func (f *fakeLLMForTechSpec) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	return nil, nil
}

type fakePromptManagerForTechSpec struct{}

func (f *fakePromptManagerForTechSpec) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "prompt-tech-spec-v2", "Generate technical spec based on {{llm_analysis}} for keyword {{keyword}}", "gemini-2.5-pro", nil
}

func TestTechnicalSpecStep(t *testing.T) {
	llm := &fakeLLMForTechSpec{}
	step := &TechnicalSpecStep{}
	logs := make([]string, 0)

	state := &PipelineState{
		Context: map[string]any{
			"llm_analysis": "Competitor analysis shows that top sites use structured content.",
		},
		Domain: &sqlstore.Domain{
			MainKeyword:    "test keyword",
			TargetCountry:  "US",
			TargetLanguage: "en",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForTechSpec{},
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

	// Проверяем наличие technical_spec
	technicalSpec, ok := artifacts["technical_spec"].(string)
	if !ok || technicalSpec == "" {
		t.Fatalf("expected technical_spec, got %#v", artifacts["technical_spec"])
	}

	// Проверяем, что technical_spec сохранен в контекст
	if state.Context["technical_spec"] != technicalSpec {
		t.Errorf("expected technical_spec to be saved in context")
	}
}

func TestTechnicalSpecStep_MissingLLMAnalysis(t *testing.T) {
	step := &TechnicalSpecStep{}
	state := &PipelineState{
		Context:       map[string]any{},
		Domain:        &sqlstore.Domain{MainKeyword: "test"},
		LLMClient:     &fakeLLMForTechSpec{},
		PromptManager: &fakePromptManagerForTechSpec{},
		AppendLog:     func(s string) {},
	}

	_, err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatalf("expected error for missing llm_analysis, got nil")
	}
	if !strings.Contains(err.Error(), "llm_analysis not found") {
		t.Errorf("expected error about llm_analysis, got: %v", err)
	}
}

func TestTechnicalSpecStep_FallbackPrompt(t *testing.T) {
	llm := &fakeLLMForTechSpec{}
	step := &TechnicalSpecStep{}
	state := &PipelineState{
		Context: map[string]any{
			"llm_analysis": "Analysis data",
		},
		Domain: &sqlstore.Domain{
			MainKeyword:   "test",
			TargetCountry: "US",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForTechSpecError{},
		AppendLog:     func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	// Проверяем, что шаг работает даже при ошибке загрузки промпта (использует дефолтный)
	technicalSpec, ok := artifacts["technical_spec"].(string)
	if !ok || technicalSpec == "" {
		t.Fatalf("expected technical_spec even with fallback prompt, got %#v", artifacts["technical_spec"])
	}
}

func TestTechnicalSpecStep_BrandGuard_CorrectiveRegeneration(t *testing.T) {
	llm := &fakeLLMForTechSpec{
		responses: []string{
			"# ТЗ\n\nH1: Регистрация в ZenitBet",
			"# ТЗ\n\nH1: Регистрация в 1хБет",
		},
	}
	step := &TechnicalSpecStep{}
	state := &PipelineState{
		Context: map[string]any{
			"llm_analysis": "Анализ конкурентов по 1хБет",
		},
		Domain: &sqlstore.Domain{
			MainKeyword:    "регистрация в 1хБет",
			TargetCountry:  "RU",
			TargetLanguage: "ru",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForTechSpec{},
		AppendLog:     func(string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if llm.calls != 2 {
		t.Fatalf("expected corrective regeneration with 2 calls, got %d", llm.calls)
	}
	if got := artifacts["technical_spec"]; !strings.Contains(fmt.Sprintf("%v", got), "1хБет") {
		t.Fatalf("expected corrected technical spec with 1хБет, got %v", got)
	}
}

func TestTechnicalSpecStep_BrandGuard_SecondViolationFails(t *testing.T) {
	llm := &fakeLLMForTechSpec{
		responses: []string{
			"# ТЗ\n\nH1: Регистрация в ZenitBet",
			"# ТЗ\n\nH1: Регистрация в ProStavki",
		},
	}
	step := &TechnicalSpecStep{}
	state := &PipelineState{
		Context: map[string]any{
			"llm_analysis": "Анализ конкурентов по 1хБет",
		},
		Domain: &sqlstore.Domain{
			MainKeyword:    "регистрация в 1хБет",
			TargetCountry:  "RU",
			TargetLanguage: "ru",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForTechSpec{},
		AppendLog:     func(string) {},
	}

	_, err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatalf("expected brand policy error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "brand policy violation") {
		t.Fatalf("expected brand policy violation error, got %v", err)
	}
}

func TestTechnicalSpecStep_BrandGuard_GenericSecondViolationSoftPass(t *testing.T) {
	llm := &fakeLLMForTechSpec{
		responses: []string{
			"# ТЗ\n\nСравнение бонусов Myt1BetX",
			"# ТЗ\n\nСравнение бонусов Auto2BetY",
		},
	}
	step := &TechnicalSpecStep{}
	state := &PipelineState{
		Context: map[string]any{
			"llm_analysis": "Анализ без брендированного запроса",
		},
		Domain: &sqlstore.Domain{
			MainKeyword:    "платежные методы в онлайн казино",
			TargetCountry:  "SE",
			TargetLanguage: "sv",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForTechSpec{},
		AppendLog:     func(string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("expected no error for generic soft-fail, got %v", err)
	}
	if llm.calls != 2 {
		t.Fatalf("expected corrective regeneration with 2 calls, got %d", llm.calls)
	}
	if got := artifacts["technical_spec"]; got == nil {
		t.Fatalf("expected technical_spec artifact")
	}
}

func TestTechnicalSpecStep_BrandGuard_GenericCasinoKeyword_NoFalsePositive(t *testing.T) {
	llm := &fakeLLMForTechSpec{
		responses: []string{
			"# ТЗ\n\nUtbetalningstid. Betalningsmetoder. Betalningsalternativ. HowTo för uttag.",
		},
	}
	step := &TechnicalSpecStep{}
	state := &PipelineState{
		Context: map[string]any{
			"llm_analysis": "Анализ без конкретного бренда",
		},
		Domain: &sqlstore.Domain{
			MainKeyword:    "insättning och uttag på utländska casinon",
			TargetCountry:  "SE",
			TargetLanguage: "sv",
		},
		LLMClient:     llm,
		PromptManager: &fakePromptManagerForTechSpec{},
		AppendLog:     func(string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("expected no error for generic casino keyword, got %v", err)
	}
	if llm.calls != 1 {
		t.Fatalf("expected single LLM call without corrective regeneration, got %d", llm.calls)
	}
	if got := artifacts["technical_spec"]; got == nil {
		t.Fatalf("expected technical_spec artifact")
	}
}

type fakePromptManagerForTechSpecError struct{}

func (f *fakePromptManagerForTechSpecError) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return "", "", "", fmt.Errorf("prompt not found")
}
