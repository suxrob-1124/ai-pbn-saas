package indexchecker

import (
	"testing"
	"time"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

func TestCalculateNextRetry(t *testing.T) {
	tests := []struct {
		name     string
		attempts int
		expect   time.Duration
	}{
		{name: "attempt 1", attempts: 1, expect: 30 * time.Minute},
		{name: "attempt 2", attempts: 2, expect: time.Hour},
		{name: "attempt 3", attempts: 3, expect: 2 * time.Hour},
		{name: "attempt 4", attempts: 4, expect: 4 * time.Hour},
		{name: "attempt 5 capped", attempts: 5, expect: 4 * time.Hour},
		{name: "attempt 0 treated as first", attempts: 0, expect: 30 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateNextRetry(tt.attempts)
			if got != tt.expect {
				t.Fatalf("CalculateNextRetry(%d) = %v, want %v", tt.attempts, got, tt.expect)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	now := time.Now()

	t.Run("retry allowed", func(t *testing.T) {
		check := sqlstore.IndexCheck{
			Attempts:  3,
			CreatedAt: now.Add(-2 * time.Hour),
		}
		if !ShouldRetry(check) {
			t.Fatal("expected retry to be allowed")
		}
	})

	t.Run("retry blocked by attempts", func(t *testing.T) {
		check := sqlstore.IndexCheck{
			Attempts:  8,
			CreatedAt: now.Add(-2 * time.Hour),
		}
		if ShouldRetry(check) {
			t.Fatal("expected retry to be blocked when attempts limit reached")
		}
	})

	t.Run("retry blocked by age", func(t *testing.T) {
		check := sqlstore.IndexCheck{
			Attempts:  1,
			CreatedAt: now.Add(-25 * time.Hour),
		}
		if ShouldRetry(check) {
			t.Fatal("expected retry to be blocked when older than 24h")
		}
	})
}
