package retry

import (
	"testing"
	"time"
)

func TestNextRetryAtSuccess(t *testing.T) {
	now := time.Date(2026, 2, 6, 10, 0, 0, 0, time.UTC)
	createdAt := now.Add(-time.Hour)

	got, err := NextRetryAt(1, createdAt, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := now.Add(1 * time.Minute)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestNextRetryAtErrors(t *testing.T) {
	now := time.Date(2026, 2, 6, 10, 0, 0, 0, time.UTC)
	createdAt := now.Add(-48 * time.Hour)

	if _, err := NextRetryAt(0, now, now); err == nil {
		t.Fatalf("expected error for invalid attempt")
	}
	if _, err := NextRetryAt(MaxRetries+1, now, now); err == nil {
		t.Fatalf("expected error for exceeding max retries")
	}
	if _, err := NextRetryAt(1, createdAt, now); err == nil {
		t.Fatalf("expected error for retry window exceeded")
	}
}

