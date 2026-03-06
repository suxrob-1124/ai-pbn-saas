package indexchecker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"obzornik-pbn-generator/internal/analyzer"

	"golang.org/x/net/idna"
)

const defaultSerpBaseURL = "https://alfasearchspy.alfasearch.ru/api/v1/ranking/parse"

// IndexChecker определяет контракт проверки индексации домена.
type IndexChecker interface {
	Check(ctx context.Context, domain string, geo string) (indexed bool, err error)
	// CheckWithQuote проверяет индексацию цитаты: site:{domain} intext:"{quote}".
	CheckWithQuote(ctx context.Context, domain string, quote string, geo string) (indexed bool, err error)
}

// MockChecker используется в тестах и всегда возвращает false.
type MockChecker struct{}

// Check всегда возвращает false без ошибки.
func (m MockChecker) Check(ctx context.Context, domain string, geo string) (bool, error) {
	return false, nil
}

// CheckWithQuote всегда возвращает false без ошибки.
func (m MockChecker) CheckWithQuote(ctx context.Context, domain string, quote string, geo string) (bool, error) {
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
	puny, err := normalizeDomain(domain)
	if err != nil {
		return false, fmt.Errorf("normalize domain: %w", err)
	}
	return s.querySerpIndexed(ctx, "site:"+puny, geo)
}

// CheckWithQuote выполняет запрос site:{domain} intext:"{quote}" к SERP API.
func (s *SerpChecker) CheckWithQuote(ctx context.Context, domain string, quote string, geo string) (bool, error) {
	if ctx == nil {
		return false, errors.New("context is required")
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return false, errors.New("domain is required")
	}
	quote = strings.TrimSpace(quote)
	if quote == "" {
		return false, errors.New("quote is required")
	}
	puny, err := normalizeDomain(domain)
	if err != nil {
		return false, fmt.Errorf("normalize domain: %w", err)
	}
	keyword := `site:` + puny + ` intext:"` + quote + `"`
	return s.querySerpIndexed(ctx, keyword, geo)
}

// querySerpIndexed выполняет SERP-запрос с заданным keyword и возвращает наличие позиций.
func (s *SerpChecker) querySerpIndexed(ctx context.Context, keyword string, geo string) (bool, error) {
	geo, _ = analyzer.NormalizeSerpGeoLang(geo, "")
	if geo == "" {
		geo = "se"
	}

	base := strings.TrimSpace(s.BaseURL)
	if base == "" {
		base = defaultSerpBaseURL
	}

	params := url.Values{}
	params.Set("keyword", keyword)
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
			lastErr = fmt.Errorf("serp request failed for keyword %q: %v", keyword, err)
			if ue, ok := err.(*url.Error); ok && ue.Err != nil {
				lastErr = fmt.Errorf("serp request failed for keyword %q: %v", keyword, ue.Err)
			}
			if isRetriableSerpErr(err) && attempt < retries {
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

		if resp.Body == nil {
			return false, fmt.Errorf("serp response body is empty for keyword %q", keyword)
		}
		if resp.StatusCode >= 400 {
			_ = resp.Body.Close()
			return false, fmt.Errorf("serp status %d for keyword %q", resp.StatusCode, keyword)
		}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("serp decode failed for keyword %q: %v", keyword, err)
			if isRetriableSerpErr(err) && attempt < retries {
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
		if err := resp.Body.Close(); err != nil {
			lastErr = fmt.Errorf("serp body close failed for keyword %q: %v", keyword, err)
			if isRetriableSerpErr(err) && attempt < retries {
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

func isRetriableSerpErr(err error) bool {
	if err == nil {
		return false
	}
	if isTimeoutErr(err) {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
		if temporaryErr, ok := any(netErr).(interface{ Temporary() bool }); ok && temporaryErr.Temporary() {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "http2: client connection lost")
}
