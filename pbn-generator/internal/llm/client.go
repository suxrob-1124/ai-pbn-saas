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
	"unicode/utf8"
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
	var usage usageTotals
	var err error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.config.RetryDelay * time.Duration(attempt)
			// 429 rate-limit: wait much longer (Gemini free tier resets in ~60s)
			if err != nil && (strings.Contains(err.Error(), "status 429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED")) {
				delay = 65 * time.Second
			}
			// unexpected EOF / connection reset: Gemini dropped connection under load, wait before retry
			if err != nil && (strings.Contains(err.Error(), "unexpected EOF") || strings.Contains(err.Error(), "connection reset")) {
				delay = 30 * time.Second
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		response, usage, err = c.callGemini(ctx, prompt, model)
		if err == nil {
			break
		}
	}

	// Сохраняем запрос
	c.mu.Lock()
	c.requests = append(c.requests, LLMRequest{
		Stage:            stage,
		Prompt:           prompt,
		Response:         response,
		Model:            model,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		TokensUsed:       usage.TotalTokens,
		TokenSource:      usage.TokenSource,
		Timestamp:        start,
		Error:            errString(err),
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
		usage    usageTotals
		err      error
	)

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.config.RetryDelay * time.Duration(attempt)
			// 429 rate-limit: wait much longer (Gemini free tier resets in ~60s)
			if err != nil && (strings.Contains(err.Error(), "status 429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED")) {
				delay = 65 * time.Second
			}
			// unexpected EOF / connection reset: Gemini dropped connection under load, wait before retry
			if err != nil && (strings.Contains(err.Error(), "unexpected EOF") || strings.Contains(err.Error(), "connection reset")) {
				delay = 30 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		imgBytes, usage, err = c.callGeminiImage(ctx, prompt, model)
		if err == nil {
			break
		}
	}

	start := time.Now()
	c.mu.Lock()
	c.requests = append(c.requests, LLMRequest{
		Stage:            "image_generation",
		Prompt:           prompt,
		Response:         fmt.Sprintf("[image:%d bytes]", len(imgBytes)),
		Model:            model,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		TokensUsed:       usage.TotalTokens,
		TokenSource:      usage.TokenSource,
		Timestamp:        start,
		Error:            errString(err),
	})
	c.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("gemini image API error after %d attempts: %w", c.config.MaxRetries+1, err)
	}

	return imgBytes, nil
}

// callGemini выполняет один запрос к Gemini API
func (c *Client) callGemini(ctx context.Context, prompt, model string) (string, usageTotals, error) {
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
		return "", usageTotals{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", usageTotals{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := c.client.Do(req)
	if err != nil {
		return "", usageTotals{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Маскируем потенциальные API ключи в URL ошибки
		err := fmt.Errorf("gemini API returned status %d: %s", resp.StatusCode, string(body))
		return "", usageTotals{}, SanitizeError(err)
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
		return "", usageTotals{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return "", usageTotals{}, fmt.Errorf("no candidates in response")
	}

	// Извлекаем текст из ответа
	var textParts []string
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		textParts = append(textParts, part.Text)
	}
	response := strings.Join(textParts, "\n")

	usage := normalizeUsage(
		prompt,
		response,
		geminiResp.UsageMetadata.PromptTokenCount,
		geminiResp.UsageMetadata.CandidatesTokenCount,
		geminiResp.UsageMetadata.TotalTokenCount,
	)

	return response, usage, nil
}

// callGeminiImage выполняет запрос генерации изображения и возвращает байты (inlineData base64).
func (c *Client) callGeminiImage(ctx context.Context, prompt, model string) ([]byte, usageTotals, error) {
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
		return nil, usageTotals{}, fmt.Errorf("failed to marshal image request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, usageTotals{}, fmt.Errorf("failed to create image request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, usageTotals{}, fmt.Errorf("failed to execute image request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Маскируем потенциальные API ключи в URL ошибки
		err := fmt.Errorf("gemini image API returned status %d: %s", resp.StatusCode, string(body))
		return nil, usageTotals{}, SanitizeError(err)
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
		UsageMetadata struct {
			PromptTokenCount     int64 `json:"promptTokenCount"`
			CandidatesTokenCount int64 `json:"candidatesTokenCount"`
			TotalTokenCount      int64 `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, usageTotals{}, fmt.Errorf("failed to decode image response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, usageTotals{}, fmt.Errorf("no candidates in image response")
	}

	usage := normalizeImageUsage(
		prompt,
		geminiResp.UsageMetadata.PromptTokenCount,
		geminiResp.UsageMetadata.CandidatesTokenCount,
		geminiResp.UsageMetadata.TotalTokenCount,
	)
	parts := geminiResp.Candidates[0].Content.Parts
	for _, p := range parts {
		if p.InlineData.Data != "" {
			raw, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
			if err != nil {
				return nil, usageTotals{}, fmt.Errorf("failed to decode inlineData: %w", err)
			}
			return raw, usage, nil
		}
	}

	return nil, usage, fmt.Errorf("no inlineData in image response")
}

// chatMessage представляет одно сообщение в multi-turn разговоре.
type chatMessage struct {
	Role  string                   `json:"role"`
	Parts []map[string]interface{} `json:"parts"`
}

// GenerateMultiTurn отправляет данные в несколько turns в рамках одной chat-сессии Gemini.
// systemInstruction задаётся как Gemini system_instruction (инструкции без данных).
// turns — последовательные пользовательские сообщения (данные по частям + финальный запрос).
// Возвращает ответ модели на последний turn.
func (c *Client) GenerateMultiTurn(ctx context.Context, stage, systemInstruction string, turns []string, model string) (string, error) {
	if model == "" {
		model = c.config.DefaultModel
	}
	if len(turns) == 0 {
		return "", fmt.Errorf("no turns provided")
	}
	// Один turn без system instruction — обычный Generate
	if len(turns) == 1 && systemInstruction == "" {
		return c.Generate(ctx, stage, turns[0], model)
	}

	start := time.Now()

	var history []chatMessage
	var finalResponse string
	var totalUsage usageTotals

	for i, userMsg := range turns {
		history = append(history, chatMessage{
			Role:  "user",
			Parts: []map[string]interface{}{{"text": userMsg}},
		})

		var resp string
		var usage usageTotals
		var err error

		for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
			if attempt > 0 {
				delay := c.config.RetryDelay * time.Duration(attempt)
				if err != nil && (strings.Contains(err.Error(), "status 429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED")) {
					delay = 65 * time.Second
				}
				if err != nil && (strings.Contains(err.Error(), "unexpected EOF") || strings.Contains(err.Error(), "connection reset")) {
					delay = 30 * time.Second
				}
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(delay):
				}
			}
			resp, usage, err = c.callGeminiChat(ctx, systemInstruction, history, model)
			if err == nil {
				break
			}
		}
		if err != nil {
			return "", fmt.Errorf("multi-turn turn %d/%d failed after %d attempts: %w", i+1, len(turns), c.config.MaxRetries+1, err)
		}

		history = append(history, chatMessage{
			Role:  "model",
			Parts: []map[string]interface{}{{"text": resp}},
		})
		totalUsage.PromptTokens += usage.PromptTokens
		totalUsage.CompletionTokens += usage.CompletionTokens
		totalUsage.TotalTokens += usage.TotalTokens
		finalResponse = resp
	}

	c.mu.Lock()
	c.requests = append(c.requests, LLMRequest{
		Stage:            stage,
		Prompt:           fmt.Sprintf("Режим multi-turn (%d запросов к модели). Системный промпт: %d символов.", len(turns), len(systemInstruction)),
		Response:         finalResponse,
		Model:            model,
		PromptTokens:     totalUsage.PromptTokens,
		CompletionTokens: totalUsage.CompletionTokens,
		TotalTokens:      totalUsage.TotalTokens,
		TokensUsed:       totalUsage.TotalTokens,
		TokenSource:      "provider",
		Timestamp:        start,
	})
	c.mu.Unlock()

	return finalResponse, nil
}

// callGeminiChat выполняет один запрос в рамках multi-turn сессии с переданной историей.
func (c *Client) callGeminiChat(ctx context.Context, systemInstruction string, history []chatMessage, model string) (string, usageTotals, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, c.config.APIKey)

	reqBody := map[string]interface{}{
		"contents": history,
	}
	if systemInstruction != "" {
		reqBody["system_instruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{{"text": systemInstruction}},
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", usageTotals{}, fmt.Errorf("failed to marshal chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", usageTotals{}, fmt.Errorf("failed to create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", usageTotals{}, fmt.Errorf("failed to execute chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", usageTotals{}, SanitizeError(fmt.Errorf("gemini API returned status %d: %s", resp.StatusCode, string(body)))
	}

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
		return "", usageTotals{}, fmt.Errorf("failed to decode chat response: %w", err)
	}
	if len(geminiResp.Candidates) == 0 {
		return "", usageTotals{}, fmt.Errorf("no candidates in chat response")
	}

	var textParts []string
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		textParts = append(textParts, part.Text)
	}
	response := strings.Join(textParts, "\n")

	usage := normalizeUsage(
		"",
		response,
		geminiResp.UsageMetadata.PromptTokenCount,
		geminiResp.UsageMetadata.CandidatesTokenCount,
		geminiResp.UsageMetadata.TotalTokenCount,
	)
	return response, usage, nil
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

type usageTotals struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	TokenSource      string
}

type usageNormalizationPolicy struct {
	EstimateCompletion bool
}

var (
	defaultUsagePolicy = usageNormalizationPolicy{EstimateCompletion: true}
	imageUsagePolicy   = usageNormalizationPolicy{EstimateCompletion: false}
)

func normalizeUsage(prompt, response string, providerPrompt, providerCompletion, providerTotal int64) usageTotals {
	return normalizeUsageWithPolicy(prompt, response, providerPrompt, providerCompletion, providerTotal, defaultUsagePolicy)
}

func normalizeImageUsage(prompt string, providerPrompt, providerCompletion, providerTotal int64) usageTotals {
	return normalizeUsageWithPolicy(prompt, "", providerPrompt, providerCompletion, providerTotal, imageUsagePolicy)
}

func normalizeUsageWithPolicy(
	prompt, response string,
	providerPrompt, providerCompletion, providerTotal int64,
	policy usageNormalizationPolicy,
) usageTotals {
	if providerPrompt < 0 {
		providerPrompt = 0
	}
	if providerCompletion < 0 {
		providerCompletion = 0
	}
	if providerTotal < 0 {
		providerTotal = 0
	}

	estimatedPrompt := estimateTokens(prompt)
	estimatedCompletion := int64(0)
	if policy.EstimateCompletion {
		estimatedCompletion = estimateTokens(response)
	}

	providerPresent := providerPrompt > 0 || providerCompletion > 0 || providerTotal > 0
	fullProvider := providerPrompt > 0 && providerCompletion > 0 && providerTotal > 0
	if !policy.EstimateCompletion {
		// Для image-cases completion может отсутствовать в provider usage легитимно.
		fullProvider = providerPrompt > 0 && providerTotal > 0
	}
	if fullProvider {
		total := providerTotal
		if total < providerPrompt+providerCompletion {
			total = providerPrompt + providerCompletion
		}
		return usageTotals{
			PromptTokens:     providerPrompt,
			CompletionTokens: providerCompletion,
			TotalTokens:      total,
			TokenSource:      "provider",
		}
	}

	promptTokens := providerPrompt
	completionTokens := providerCompletion
	totalTokens := providerTotal

	if policy.EstimateCompletion {
		if promptTokens == 0 && totalTokens > completionTokens && completionTokens > 0 {
			promptTokens = totalTokens - completionTokens
		}
		if completionTokens == 0 && totalTokens > promptTokens && promptTokens > 0 {
			completionTokens = totalTokens - promptTokens
		}
	} else {
		// image policy:
		// - если provider usage отсутствует, completion должен оставаться 0.
		// - если provider дал только total, считаем, что это prompt budget.
		if promptTokens == 0 && totalTokens > completionTokens && completionTokens > 0 {
			promptTokens = totalTokens - completionTokens
		}
		if promptTokens == 0 && totalTokens > 0 && completionTokens == 0 {
			promptTokens = totalTokens
		}
		if providerCompletion == 0 {
			completionTokens = 0
		}
	}
	if promptTokens == 0 {
		promptTokens = estimatedPrompt
	}
	if completionTokens == 0 {
		completionTokens = estimatedCompletion
	}
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}
	if totalTokens < promptTokens+completionTokens {
		totalTokens = promptTokens + completionTokens
	}

	source := "estimated"
	if providerPresent {
		source = "mixed"
	}
	return usageTotals{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		TokenSource:      source,
	}
}

func estimateTokens(text string) int64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runes := utf8.RuneCountInString(text)
	if runes <= 0 {
		return 0
	}
	return int64((runes + 3) / 4)
}
