package llm

import (
	"context"
	"fmt"
	"strings"
)

// PromptManager управляет загрузкой и использованием промптов из БД
type PromptManager struct {
	promptStore PromptStore
	cache       map[string]string // кэш промптов по ID
}

// PromptStore интерфейс для работы с промптами (из sqlstore)
type PromptStore interface {
	Get(ctx context.Context, id string) (Prompt, error)
	GetActive(ctx context.Context) (Prompt, error)
	GetByStage(ctx context.Context, stage string) (Prompt, error)
}

// Prompt представляет промпт из БД
type Prompt interface {
	GetID() string
	GetBody() string
	GetName() string
	GetModel() string // Возвращает модель LLM или пустую строку
}

// NewPromptManager создает новый менеджер промптов
func NewPromptManager(store PromptStore) *PromptManager {
	return &PromptManager{
		promptStore: store,
		cache:       make(map[string]string),
	}
}

// GetPrompt загружает промпт по ID (с кэшированием)
func (pm *PromptManager) GetPrompt(ctx context.Context, promptID string) (string, error) {
	// Проверяем кэш
	if cached, ok := pm.cache[promptID]; ok {
		return cached, nil
	}
	
	// Загружаем из БД
	prompt, err := pm.promptStore.Get(ctx, promptID)
	if err != nil {
		return "", fmt.Errorf("failed to get prompt %s: %w", promptID, err)
	}
	
	body := prompt.GetBody()
	
	// Сохраняем в кэш
	pm.cache[promptID] = body
	
	return body, nil
}

// GetActivePrompt загружает активный промпт (deprecated: используйте GetPromptByStage)
func (pm *PromptManager) GetActivePrompt(ctx context.Context) (string, string, error) {
	prompt, err := pm.promptStore.GetActive(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get active prompt: %w", err)
	}
	
	body := prompt.GetBody()
	id := prompt.GetID()
	
	// Сохраняем в кэш
	pm.cache[id] = body
	
	return id, body, nil
}

// GetPromptByStage загружает активный промпт для конкретного этапа генерации
// Возвращает: promptID, promptBody, model, error
func (pm *PromptManager) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	// Проверяем кэш по ключу stage
	cacheKey := fmt.Sprintf("stage:%s", stage)
	if cached, ok := pm.cache[cacheKey]; ok {
		// Нужно найти ID промпта для этого этапа
		prompt, err := pm.promptStore.GetByStage(ctx, stage)
		if err == nil {
			return prompt.GetID(), cached, prompt.GetModel(), nil
		}
	}
	
	// Загружаем из БД
	prompt, err := pm.promptStore.GetByStage(ctx, stage)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get prompt for stage %s: %w", stage, err)
	}
	
	body := prompt.GetBody()
	id := prompt.GetID()
	model := prompt.GetModel()
	
	// Сохраняем в кэш
	pm.cache[cacheKey] = body
	pm.cache[id] = body
	
	return id, body, model, nil
}

// BuildPrompt строит полный промпт из системного промпта и пользовательского контента
func BuildPrompt(systemPrompt, userContent string, variables map[string]string) string {
	// Заменяем переменные в системном промпте
	prompt := systemPrompt
	for key, value := range variables {
		// Поддерживаем оба формата: {{key}} и {{ key }} (с пробелами)
		placeholder1 := fmt.Sprintf("{{%s}}", key)
		placeholder2 := fmt.Sprintf("{{ %s }}", key)
		prompt = strings.ReplaceAll(prompt, placeholder1, value)
		prompt = strings.ReplaceAll(prompt, placeholder2, value)
	}
	
	// Заменяем переменные в пользовательском контенте (если он есть)
	processedUserContent := userContent
	for key, value := range variables {
		// Поддерживаем оба формата: {{key}} и {{ key }} (с пробелами)
		placeholder1 := fmt.Sprintf("{{%s}}", key)
		placeholder2 := fmt.Sprintf("{{ %s }}", key)
		processedUserContent = strings.ReplaceAll(processedUserContent, placeholder1, value)
		processedUserContent = strings.ReplaceAll(processedUserContent, placeholder2, value)
	}
	
	// Добавляем пользовательский контент
	if processedUserContent != "" {
		prompt += "\n\n" + processedUserContent
	}
	
	return prompt
}

// ClearCache очищает кэш промптов
func (pm *PromptManager) ClearCache() {
	pm.cache = make(map[string]string)
}

