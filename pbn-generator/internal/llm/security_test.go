package llm

import (
	"errors"
	"testing"
)

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "[empty]",
		},
		{
			name:     "very short key (1-4 chars)",
			input:    "abc",
			expected: "****",
		},
		{
			name:     "short key (5-8 chars)",
			input:    "abcdefgh",
			expected: "abcd****",
		},
		{
			name:     "normal key",
			input:    "AIzaSyD1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "AIza...wxyz",
		},
		{
			name:     "long key",
			input:    "AIzaSyD1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			expected: "AIza...WXYZ",
		},
		{
			name:     "exactly 8 chars",
			input:    "12345678",
			expected: "1234****",
		},
		{
			name:     "exactly 9 chars",
			input:    "123456789",
			expected: "1234...6789", // Первые 4 и последние 4 символа
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAPIKey(tt.input)
			if result != tt.expected {
				t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected string
		contains string // Альтернативная проверка - должна содержать эту строку
	}{
		{
			name:     "nil error",
			input:    nil,
			expected: "",
		},
		{
			name:     "error without key",
			input:    errors.New("some error message"),
			expected: "some error message",
		},
		{
			name:     "error with key in URL",
			input:    errors.New("gemini API returned status 400: request failed key=AIzaSyD1234567890abcdefghijklmnopqrstuvwxyz"),
			contains: "key=AIza...wxyz",
		},
		{
			name:     "error with key and query params",
			input:    errors.New("failed: key=AIzaSyD1234567890&model=test"),
			contains: "key=AIza...7890",
		},
		{
			name:     "error with key at end",
			input:    errors.New("error key=AIzaSyD1234567890"),
			contains: "key=AIza...7890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeError(tt.input)

			if tt.input == nil {
				if result != nil {
					t.Errorf("SanitizeError(nil) = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Errorf("SanitizeError(%v) = nil, want error", tt.input)
				return
			}

			resultMsg := result.Error()

			if tt.expected != "" {
				if resultMsg != tt.expected {
					t.Errorf("SanitizeError(%q) = %q, want %q", tt.input.Error(), resultMsg, tt.expected)
				}
			} else if tt.contains != "" {
				if !contains(resultMsg, tt.contains) {
					t.Errorf("SanitizeError(%q) = %q, should contain %q", tt.input.Error(), resultMsg, tt.contains)
				}
			}

			// Проверяем, что результат не содержит полный ключ (если был ключ в исходной ошибке)
			if contains(tt.input.Error(), "key=") {
				// Проверяем, что маскированный ключ не равен исходному
				if contains(resultMsg, "AIzaSyD1234567890abcdefghijklmnopqrstuvwxyz") {
					t.Errorf("SanitizeError should mask full key, but found full key in: %q", resultMsg)
				}
			}
		})
	}
}

// contains проверяет, содержит ли строка подстроку (case-sensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[:len(substr)] == substr ||
			(len(s) > len(substr) && containsHelper(s, substr)))))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
