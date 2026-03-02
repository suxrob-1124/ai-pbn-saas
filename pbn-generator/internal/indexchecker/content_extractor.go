package indexchecker

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

const (
	contentExtractMinLen  = 80
	contentExtractMaxLen  = 200
	contentExtractMinWords = 8
	contentFetchTimeout   = 15 * time.Second
)

// ExtractSiteQuote загружает главную страницу сайта и извлекает подходящий абзац
// для проверки индексации через intext:. Возвращает пустую строку (без ошибки),
// если подходящий абзац не найден.
func ExtractSiteQuote(ctx context.Context, siteURL string) (string, error) {
	siteURL = strings.TrimSpace(siteURL)
	if !strings.Contains(siteURL, "://") {
		siteURL = "https://" + siteURL
	}
	client := &http.Client{Timeout: contentFetchTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, siteURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; IndexChecker/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch site: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("site returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	var quote string
	doc.Find("p").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := normalizeQuoteText(s.Text())
		if isGoodQuote(text) {
			quote = text
			return false
		}
		return true
	})
	return quote, nil
}

// isGoodQuote возвращает true, если текст подходит для поиска intext:.
func isGoodQuote(text string) bool {
	if len(text) < contentExtractMinLen || len(text) > contentExtractMaxLen {
		return false
	}
	// Минимум 8 слов
	if len(strings.Fields(text)) < contentExtractMinWords {
		return false
	}
	// Не должно содержать кавычек, обратных слешей, переносов — они сломают запрос
	for _, r := range text {
		if r == '"' || r == '\'' || r == '`' || r == '\\' ||
			r == '\n' || r == '\r' || r == '\t' {
			return false
		}
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// normalizeQuoteText убирает лишние пробелы и нормализует текст.
func normalizeQuoteText(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace && b.Len() > 0 {
				b.WriteRune(' ')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}
