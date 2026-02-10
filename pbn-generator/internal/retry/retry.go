package retry

import (
	"fmt"
	"time"
)

const (
	// MaxRetries defines how many retry attempts are allowed.
	MaxRetries = 7
	// RetryWindow limits retries to a 24 hour window since creation.
	RetryWindow = 24 * time.Hour
)

var retryBackoffs = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
	6 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
}

// NextRetryAt returns the next retry time for a 1-based attempt number.
func NextRetryAt(attempt int, createdAt, now time.Time) (time.Time, error) {
	if attempt <= 0 {
		return time.Time{}, fmt.Errorf("attempt must be >= 1")
	}
	if attempt > len(retryBackoffs) {
		return time.Time{}, fmt.Errorf("max retries exceeded")
	}
	if now.Sub(createdAt) > RetryWindow {
		return time.Time{}, fmt.Errorf("retry window exceeded")
	}
	return now.Add(retryBackoffs[attempt-1]), nil
}

