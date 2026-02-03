package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ValidateAPIKey проверяет валидность API ключа, делая тестовый запрос к Gemini API
func ValidateAPIKey(ctx context.Context, apiKey, model string) error {
	if apiKey == "" {
		return fmt.Errorf("api key is empty")
	}
	// Всегда используем легкую модель для валидации (быстрее и дешевле)
	validationModel := "gemini-2.5-flash"

	// Создаем HTTP клиент с увеличенным таймаутом для валидации
	// Учитываем возможные задержки сети
	client := &http.Client{Timeout: 30 * time.Second}

	// Формируем URL для Gemini API (используем легкую модель для валидации)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", validationModel, apiKey)

	// Формируем минимальный тестовый запрос
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": "test",
					},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := client.Do(req)
	if err != nil {
		// Проверяем, является ли ошибка таймаутом или проблемой сети
		if strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "deadline exceeded") ||
			strings.Contains(err.Error(), "context deadline exceeded") {
			return fmt.Errorf("request timeout: API key validation took too long. This might be a network issue. Please try again or check your connection")
		}
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		// Парсим ошибку от Gemini API
		var geminiError struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &geminiError); err == nil && geminiError.Error.Message != "" {
			// Проверяем на типичные ошибки невалидного ключа
			if strings.Contains(geminiError.Error.Message, "API_KEY_INVALID") ||
				strings.Contains(geminiError.Error.Message, "API key not valid") ||
				strings.Contains(geminiError.Error.Message, "invalid API key") {
				return fmt.Errorf("invalid API key: %s", geminiError.Error.Message)
			}
			return fmt.Errorf("gemini API error: %s", geminiError.Error.Message)
		}
		// Маскируем потенциальные API ключи в URL ошибки
		err := fmt.Errorf("gemini API returned status %d: %s", resp.StatusCode, string(body))
		return SanitizeError(err)
	}

	// Проверяем, что ответ валидный
	var geminiResp struct {
		Candidates []interface{} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return fmt.Errorf("no candidates in response")
	}

	return nil
}
