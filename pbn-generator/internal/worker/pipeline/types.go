package pipeline

import (
	"context"

	"obzornik-pbn-generator/internal/analyzer"
	"obzornik-pbn-generator/internal/llm"
)

// LLMClient интерфейс для работы с LLM
type LLMClient interface {
	Generate(ctx context.Context, stage string, prompt string, model string) (string, error)
	GenerateImage(ctx context.Context, prompt string, model string) ([]byte, error)
}

// PromptManager интерфейс для работы с промптами
type PromptManager interface {
	GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) // promptID, promptBody, model, error
}

// Analyzer интерфейс для анализа SERP
// analyzer.Analyze возвращает *analyzer.Result, поэтому интерфейс должен работать с этим
type Analyzer interface {
	Analyze(ctx context.Context, query string, country string, lang string, excludeDomains []string, config analyzer.Config) (*analyzer.Result, error)
}

// GeneratedFile описывает файл, который нужно сохранить на диск после генерации
type GeneratedFile struct {
	Path          string `json:"path"`
	Content       string `json:"content,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
}

// Реализации интерфейсов

type llmClientImpl struct {
	client *llm.Client
}

func (c *llmClientImpl) Generate(ctx context.Context, stage string, prompt string, model string) (string, error) {
	return c.client.Generate(ctx, stage, prompt, model)
}

func (c *llmClientImpl) GenerateImage(ctx context.Context, prompt string, model string) ([]byte, error) {
	return c.client.GenerateImage(ctx, prompt, model)
}

type promptManagerImpl struct {
	manager *llm.PromptManager
}

func (p *promptManagerImpl) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	return p.manager.GetPromptByStage(ctx, stage)
}

type analyzerImpl struct{}

func (a *analyzerImpl) Analyze(ctx context.Context, query string, country string, lang string, excludeDomains []string, config analyzer.Config) (*analyzer.Result, error) {
	return analyzer.Analyze(ctx, query, country, lang, excludeDomains, config)
}

// NewLLMClient создает новый LLM клиент
func NewLLMClient(client *llm.Client) LLMClient {
	return &llmClientImpl{client: client}
}

// NewPromptManager создает новый менеджер промптов
func NewPromptManager(manager *llm.PromptManager) PromptManager {
	return &promptManagerImpl{manager: manager}
}

// NewAnalyzer создает новый анализатор
func NewAnalyzer() Analyzer {
	return &analyzerImpl{}
}
