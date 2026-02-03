package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"obzornik-pbn-generator/internal/config"
)

func NormalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

type RateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
		hits:   make(map[string][]time.Time),
	}
}

func (l *RateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	windowStart := now.Add(-l.window)
	h := l.hits[key]
	pruned := h[:0]
	for _, ts := range h {
		if ts.After(windowStart) {
			pruned = append(pruned, ts)
		}
	}
	if len(pruned) >= l.limit {
		l.hits[key] = pruned
		return false
	}
	l.hits[key] = append(pruned, now)
	return true
}

// SlidingLimiter — нересетящийся слайдинг-лимит для, например, капчи.
type SlidingLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func NewSlidingLimiter(limit int, window time.Duration) *SlidingLimiter {
	return &SlidingLimiter{
		limit:  limit,
		window: window,
		hits:   make(map[string][]time.Time),
	}
}

func (l *SlidingLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	windowStart := now.Add(-l.window)
	h := l.hits[key]
	pruned := h[:0]
	for _, ts := range h {
		if ts.After(windowStart) {
			pruned = append(pruned, ts)
		}
	}
	if len(pruned) >= l.limit {
		l.hits[key] = pruned
		return false
	}
	l.hits[key] = append(pruned, now)
	return true
}

type lockoutTracker struct {
	mu          sync.Mutex
	failures    map[string]failureInfo
	maxFailures int
	lockoutFor  time.Duration
}

type failureInfo struct {
	count        int
	blockedUntil time.Time
	lastAttempt  time.Time
}

func newLockoutTracker(maxFailures int, lockoutFor time.Duration) *lockoutTracker {
	return &lockoutTracker{
		failures:    make(map[string]failureInfo),
		maxFailures: maxFailures,
		lockoutFor:  lockoutFor,
	}
}

func (l *lockoutTracker) isBlocked(key string) (time.Time, bool) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	info := l.failures[key]
	if info.blockedUntil.After(now) {
		return info.blockedUntil, true
	}
	return time.Time{}, false
}

func (l *lockoutTracker) registerFailure(key string) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	info := l.failures[key]
	if now.Sub(info.lastAttempt) > l.lockoutFor {
		info.count = 0
	}
	info.lastAttempt = now
	info.count++

	if info.count >= l.maxFailures {
		info.blockedUntil = now.Add(l.lockoutFor)
		info.count = 0
	}
	l.failures[key] = info
}

func (l *lockoutTracker) reset(key string) {
	l.mu.Lock()
	delete(l.failures, key)
	l.mu.Unlock()
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type passwordPolicy struct {
	MinLength      int
	MaxLength      int
	RequireUpper   bool
	RequireLower   bool
	RequireDigit   bool
	RequireSpecial bool
}

func buildPasswordPolicy(cfg config.Config) passwordPolicy {
	min := cfg.PasswordMinLength
	if min <= 0 {
		min = 10
	}
	max := cfg.PasswordMaxLength
	if max <= 0 {
		max = 128
	}
	return passwordPolicy{
		MinLength:      min,
		MaxLength:      max,
		RequireUpper:   cfg.PasswordRequireUpper,
		RequireLower:   cfg.PasswordRequireLower,
		RequireDigit:   cfg.PasswordRequireDigit,
		RequireSpecial: cfg.PasswordRequireSpecial,
	}
}

func validatePassword(pw string, policy passwordPolicy) error {
	if policy.MinLength <= 0 {
		policy.MinLength = 10
	}
	if policy.MaxLength <= 0 {
		policy.MaxLength = 128
	}
	if len(pw) < policy.MinLength {
		return errors.New("password must be at least the minimum length")
	}
	if len(pw) > policy.MaxLength {
		return errors.New("password too long")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range pw {
		switch {
		case 'A' <= c && c <= 'Z':
			hasUpper = true
		case 'a' <= c && c <= 'z':
			hasLower = true
		case '0' <= c && c <= '9':
			hasDigit = true
		default:
			if strings.ContainsRune("!@#$%^&*()-_=+[]{};:',.<>/?`~|\\\"", c) || c > 127 {
				hasSpecial = true
			}
		}
	}
	if policy.RequireUpper && !hasUpper {
		return errors.New("password must contain uppercase")
	}
	if policy.RequireLower && !hasLower {
		return errors.New("password must contain lowercase")
	}
	if policy.RequireDigit && !hasDigit {
		return errors.New("password must contain digit")
	}
	if policy.RequireSpecial && !hasSpecial {
		return errors.New("password must contain special character")
	}
	return nil
}
