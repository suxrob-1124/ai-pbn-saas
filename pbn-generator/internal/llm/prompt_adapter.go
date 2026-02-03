package llm

import (
	"context"
	
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// PromptAdapter адаптирует sqlstore.PromptStore для использования в llm.PromptManager
type PromptAdapter struct {
	store interface {
		Get(ctx context.Context, id string) (sqlstore.SystemPrompt, error)
		GetActive(ctx context.Context) (sqlstore.SystemPrompt, error)
		GetByStage(ctx context.Context, stage string) (sqlstore.SystemPrompt, error)
	}
}

// NewPromptAdapter создает новый адаптер
func NewPromptAdapter(store interface {
	Get(ctx context.Context, id string) (sqlstore.SystemPrompt, error)
	GetActive(ctx context.Context) (sqlstore.SystemPrompt, error)
	GetByStage(ctx context.Context, stage string) (sqlstore.SystemPrompt, error)
}) *PromptAdapter {
	return &PromptAdapter{store: store}
}

// Get получает промпт по ID
func (a *PromptAdapter) Get(ctx context.Context, id string) (Prompt, error) {
	p, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &promptWrapper{prompt: p}, nil
}

// GetActive получает активный промпт
func (a *PromptAdapter) GetActive(ctx context.Context) (Prompt, error) {
	p, err := a.store.GetActive(ctx)
	if err != nil {
		return nil, err
	}
	return &promptWrapper{prompt: p}, nil
}

// GetByStage получает активный промпт для конкретного этапа
func (a *PromptAdapter) GetByStage(ctx context.Context, stage string) (Prompt, error) {
	p, err := a.store.GetByStage(ctx, stage)
	if err != nil {
		return nil, err
	}
	return &promptWrapper{prompt: p}, nil
}

// promptWrapper оборачивает sqlstore.SystemPrompt для реализации интерфейса Prompt
type promptWrapper struct {
	prompt sqlstore.SystemPrompt
}

func (w *promptWrapper) GetID() string {
	return w.prompt.ID
}

func (w *promptWrapper) GetBody() string {
	return w.prompt.Body
}

func (w *promptWrapper) GetName() string {
	return w.prompt.Name
}

func (w *promptWrapper) GetModel() string {
	if w.prompt.Model.Valid {
		return w.prompt.Model.String
	}
	return ""
}

