package indexchecker

import (
	"time"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

const maxRetryAttemptsPerDay = 8

// CalculateNextRetry возвращает интервал до следующей попытки по номеру попытки.
func CalculateNextRetry(attempts int) time.Duration {
	switch {
	case attempts <= 1:
		return 30 * time.Minute
	case attempts == 2:
		return time.Hour
	case attempts == 3:
		return 2 * time.Hour
	case attempts == 4:
		return 4 * time.Hour
	default:
		return 4 * time.Hour
	}
}

// ShouldRetry определяет, можно ли делать повторную попытку.
func ShouldRetry(check sqlstore.IndexCheck) bool {
	if check.Attempts >= maxRetryAttemptsPerDay {
		return false
	}
	if check.CreatedAt.IsZero() {
		return false
	}
	return time.Since(check.CreatedAt) < 24*time.Hour
}
