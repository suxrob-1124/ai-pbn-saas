package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client представляет клиент для работы с Gemini API
type Client struct {
	config   Config
	client   *http.Client
	requests []LLMRequest
	mu       sync.Mutex
}

// NewClient создает новый LLM клиент
func NewClient(cfg Config) *Client {
	return &Client{
		config:   cfg,
		client:   &http.Client{Timeout: cfg.RequestTimeout},
		requests: make([]LLMRequest, 0),
	}
}

// Generate выполняет запрос к Gemini API и возвращает ответ
func (c *Client) Generate(ctx context.Context, stage, prompt, model string) (string, error) {
	if model == "" {
		model = c.config.DefaultModel
	}

	start := time.Now()

	// Выполняем запрос с retry логикой
	var response string
	var tokensUsed int64
	var err error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(c.config.RetryDelay * time.Duration(attempt)):
			}
		}

		response, tokensUsed, err = c.callGemini(ctx, prompt, model)
		if err == nil {
			break
		}
	}

	// Сохраняем запрос
	c.mu.Lock()
	c.requests = append(c.requests, LLMRequest{
		Stage:      stage,
		Prompt:     prompt,
		Response:   response,
		Model:      model,
		TokensUsed: tokensUsed,
		Timestamp:  start,
		Error:      errString(err),
	})
	c.mu.Unlock()

	if err != nil {
		return "", fmt.Errorf("gemini API error after %d attempts: %w", c.config.MaxRetries+1, err)
	}

	return response, nil
}

func (c *Client) GenerateImage(ctx context.Context, prompt, model string) ([]byte, error) {
	if model == "" {
		model = c.config.DefaultModel
	}

	var (
		imgBytes []byte
		err      error
	)

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryDelay * time.Duration(attempt)):
			}
		}

		imgBytes, err = c.callGeminiImage(ctx, prompt, model)
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, fmt.Errorf("gemini image API error after %d attempts: %w", c.config.MaxRetries+1, err)
	}

	return imgBytes, nil
}

// callGemini выполняет один запрос к Gemini API
func (c *Client) callGemini(ctx context.Context, prompt, model string) (string, int64, error) {
	// Формируем URL для Gemini API
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, c.config.APIKey)

	// Формируем тело запроса
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": prompt,
					},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := c.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Маскируем потенциальные API ключи в URL ошибки
		err := fmt.Errorf("gemini API returned status %d: %s", resp.StatusCode, string(body))
		return "", 0, SanitizeError(err)
	}

	// Парсим ответ
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int64 `json:"promptTokenCount"`
			CandidatesTokenCount int64 `json:"candidatesTokenCount"`
			TotalTokenCount      int64 `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return "", 0, fmt.Errorf("no candidates in response")
	}

	// Извлекаем текст из ответа
	var textParts []string
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		textParts = append(textParts, part.Text)
	}
	response := strings.Join(textParts, "\n")

	// Получаем количество токенов
	tokensUsed := geminiResp.UsageMetadata.TotalTokenCount

	return response, tokensUsed, nil
}

// callGeminiImage выполняет запрос генерации изображения и возвращает байты (inlineData base64).
func (c *Client) callGeminiImage(ctx context.Context, prompt, model string) ([]byte, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, c.config.APIKey)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal image request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create image request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute image request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Маскируем потенциальные API ключи в URL ошибки
		err := fmt.Errorf("gemini image API returned status %d: %s", resp.StatusCode, string(body))
		return nil, SanitizeError(err)
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode image response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in image response")
	}
	parts := geminiResp.Candidates[0].Content.Parts
	for _, p := range parts {
		if p.InlineData.Data != "" {
			raw, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to decode inlineData: %w", err)
			}
			return raw, nil
		}
	}

	return nil, fmt.Errorf("no inlineData in image response")
}

// GetRequests возвращает все накопленные запросы
func (c *Client) GetRequests() []LLMRequest {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Возвращаем копию, чтобы избежать race conditions
	requests := make([]LLMRequest, len(c.requests))
	copy(requests, c.requests)
	return requests
}

// Reset очищает накопленные запросы
func (c *Client) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = nil
}

// errString возвращает строковое представление ошибки или пустую строку
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
