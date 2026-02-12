package indexchecker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/idna"
)

const defaultSerpBaseURL = "https://alfasearchspy.alfasearch.ru/api/v1/ranking/parse"

// IndexChecker определяет контракт проверки индексации домена.
type IndexChecker interface {
	Check(ctx context.Context, domain string, geo string) (indexed bool, err error)
}

// MockChecker используется в тестах и всегда возвращает false.
type MockChecker struct{}

// Check всегда возвращает false без ошибки.
func (m MockChecker) Check(ctx context.Context, domain string, geo string) (bool, error) {
	return false, nil
}

// SerpChecker проверяет индексацию через SERP API.
type SerpChecker struct {
	BaseURL string
	Client  *http.Client
}

// Check выполняет запрос site:{domain} к SERP API и возвращает факт индексации.
func (s *SerpChecker) Check(ctx context.Context, domain string, geo string) (bool, error) {
	if ctx == nil {
		return false, errors.New("context is required")
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return false, errors.New("domain is required")
	}

	geo = strings.ToLower(strings.TrimSpace(geo))
	if geo == "" {
		geo = "se"
	}

	puny, err := normalizeDomain(domain)
	if err != nil {
		return false, fmt.Errorf("normalize domain: %w", err)
	}

	base := strings.TrimSpace(s.BaseURL)
	if base == "" {
		base = defaultSerpBaseURL
	}

	params := url.Values{}
	params.Set("keyword", "site:"+puny)
	params.Set("geo", geo)
	params.Set("group_by", "10")
	params.Set("device", "MOBILE")
	params.Set("real_time", "true")

	timeout := serpTimeout()
	retries := serpRetries()
	client := serpClient(s.Client, timeout)

	var raw map[string]any
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+params.Encode(), nil)
		if err != nil {
			return false, err
		}
		req.Header.Set("accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("serp request failed for domain %q: %v", puny, err)
			if ue, ok := err.(*url.Error); ok && ue.Err != nil {
				lastErr = fmt.Errorf("serp request failed for domain %q: %v", puny, ue.Err)
			}
			if isTimeoutErr(err) && attempt < retries {
				backoff := time.Duration(2*(attempt+1)) * time.Second
				select {
				case <-time.After(backoff):
					continue
				case <-ctx.Done():
					return false, ctx.Err()
				}
			}
			return false, lastErr
		}

		if resp.Body != nil {
			defer resp.Body.Close()
		}
		if resp.StatusCode >= 400 {
			return false, fmt.Errorf("serp status %d for domain %q", resp.StatusCode, puny)
		}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return false, err
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		return false, lastErr
	}

	if positions, ok := raw["positions"].([]any); ok && len(positions) > 0 {
		return true, nil
	}
	return false, nil
}

func normalizeDomain(input string) (string, error) {
	target := strings.TrimSpace(input)
	if target == "" {
		return "", errors.New("domain is empty")
	}
	if !strings.Contains(target, "://") {
		target = "http://" + target
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return "", err
	}
	host := parsed.Hostname()
	if host == "" {
		return "", errors.New("domain is empty")
	}
	puny, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", err
	}
	return puny, nil
}

func serpClient(client *http.Client, timeout time.Duration) *http.Client {
	if client == nil {
		return &http.Client{Timeout: timeout}
	}
	if client.Timeout == 0 {
		cloned := *client
		cloned.Timeout = timeout
		return &cloned
	}
	return client
}

func serpTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("SERP_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 30 {
			return time.Duration(n) * time.Second
		}
	}
	return 180 * time.Second
}

func serpRetries() int {
	if v := strings.TrimSpace(os.Getenv("SERP_RETRIES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			if n > 3 {
				return 3
			}
			return n
		}
	}
	return 1
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "Client.Timeout exceeded")
}
