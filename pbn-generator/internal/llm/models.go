package llm

import "time"

// LLMRequest представляет один запрос к LLM API
type LLMRequest struct {
	Stage      string    `json:"stage"`           // "competitor_analysis", "technical_spec", "content", "images", etc.
	Prompt     string    `json:"prompt"`          // полный промпт (может быть большой)
	Response   string    `json:"response"`        // полный ответ LLM
	Model      string    `json:"model"`           // "gemini-2.5-pro", "gemini-2.5-flash", etc.
	TokensUsed int64     `json:"tokens_used"`     // количество использованных токенов
	Timestamp  time.Time `json:"timestamp"`       // время запроса
	Error      string    `json:"error,omitempty"` // ошибка, если была
}

// Config содержит конфигурацию для LLM клиента
type Config struct {
	APIKey          string        // Gemini API ключ
	DefaultModel    string        // Модель по умолчанию
	MaxRetries      int           // Максимальное количество повторов
	RetryDelay      time.Duration // Задержка между повторами
	RequestTimeout  time.Duration // Таймаут для запросов
	RateLimitPerMin int           // Лимит запросов в минуту
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig(apiKey string) Config {
	return Config{
		APIKey:          apiKey,
		DefaultModel:    "gemini-2.5-pro",
		MaxRetries:      3,
		RetryDelay:      time.Second * 2,
		RequestTimeout:  time.Minute * 5,
		RateLimitPerMin: 60,
	}
}
