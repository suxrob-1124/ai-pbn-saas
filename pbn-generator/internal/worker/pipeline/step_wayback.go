package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// WaybackFetchStep fetches archived snapshots from the Wayback Machine CDX API,
// downloads the HTML content, handles frame extraction, and cleans the text
// using goquery. This is the first step for webarchive_single generation.
type WaybackFetchStep struct{}

func (s *WaybackFetchStep) Name() string       { return StepWaybackFetch }
func (s *WaybackFetchStep) ArtifactKey() string { return "wayback_data" }
func (s *WaybackFetchStep) Progress() int       { return 3 }

const (
	waybackCDXURL        = "https://web.archive.org/cdx/search/cdx"
	waybackWebURL        = "https://web.archive.org/web"
	waybackCDXTimeout    = 60 * time.Second
	waybackDownTimeout   = 30 * time.Second
	waybackMinLength     = 500
	waybackMaxSnapshots  = 5
	waybackCDXRetries    = 2
	waybackDownRetries   = 3
	waybackRetryDelay    = 5 * time.Second
	waybackUserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
	waybackMaxCleanChars = 200000 // Limit combined clean text sent to LLM
)

type waybackSnapshot struct {
	Timestamp string `json:"timestamp"`
	Original  string `json:"original"`
	Length    int    `json:"length"`
}

func (s *WaybackFetchStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	domainURL := state.Domain.URL
	state.AppendLog(fmt.Sprintf("Начало загрузки архивных снэпшотов для %s", domainURL))

	// 1. Query CDX API
	snapshots, err := queryCDX(ctx, domainURL, state)
	if err != nil {
		return nil, fmt.Errorf("wayback CDX query failed for %s: %w", domainURL, err)
	}
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no archived snapshots found for domain %s", domainURL)
	}
	state.AppendLog(fmt.Sprintf("Найдено %d подходящих снэпшотов", len(snapshots)))

	// 2. Download and clean HTML for each snapshot
	var cleanTexts []string
	var snapshotsMeta []map[string]any
	for _, snap := range snapshots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		downloadURL := fmt.Sprintf("%s/%sid_/%s", waybackWebURL, snap.Timestamp, snap.Original)
		state.AppendLog(fmt.Sprintf("Скачивание снэпшота: %s", snap.Timestamp))

		htmlBody, err := downloadSnapshot(ctx, downloadURL, state)
		if err != nil {
			state.AppendLog(fmt.Sprintf("Предупреждение: не удалось скачать снэпшот %s: %v", snap.Timestamp, err))
			continue
		}

		// Handle frame extraction
		finalHTML := htmlBody
		if frameURL := extractFrameURL(htmlBody, downloadURL); frameURL != "" {
			state.AppendLog(fmt.Sprintf("Обнаружен фрейм, скачиваем: %s", frameURL))
			frameHTML, err := downloadSnapshot(ctx, frameURL, state)
			if err != nil {
				state.AppendLog(fmt.Sprintf("Предупреждение: не удалось скачать фрейм: %v", err))
			} else {
				finalHTML = frameHTML
			}
		}

		// Clean HTML
		cleanText := cleanHTMLText(finalHTML)
		if cleanText == "" {
			state.AppendLog(fmt.Sprintf("Предупреждение: снэпшот %s не содержит полезного текста", snap.Timestamp))
			continue
		}

		cleanTexts = append(cleanTexts, cleanText)
		snapshotsMeta = append(snapshotsMeta, map[string]any{
			"timestamp": snap.Timestamp,
			"url":       snap.Original,
			"length":    snap.Length,
		})
	}

	if len(cleanTexts) == 0 {
		return nil, fmt.Errorf("failed to download any archived content for domain %s", domainURL)
	}

	// Combine all clean texts
	combinedText := strings.Join(cleanTexts, "\n\n---\n\n")
	// Truncate if too long
	combinedText, truncated := truncateRunes(combinedText, waybackMaxCleanChars)
	if truncated {
		state.AppendLog("combined_text усечён до лимита")
	}

	state.AppendLog(fmt.Sprintf("Успешно извлечён текст из %d снэпшотов (%d символов)", len(cleanTexts), len([]rune(combinedText))))

	// Build artifacts
	waybackData := map[string]any{
		"domain":        domainURL,
		"snapshots":     snapshotsMeta,
		"clean_texts":   cleanTexts,
		"combined_text": combinedText,
	}

	artifacts := map[string]any{
		"wayback_data": waybackData,
	}
	state.Context["wayback_data"] = waybackData

	return artifacts, nil
}

// queryCDX queries the Wayback Machine CDX API and returns filtered snapshots.
func queryCDX(ctx context.Context, domain string, state *PipelineState) ([]waybackSnapshot, error) {
	params := url.Values{}
	params.Set("url", domain)
	params.Set("output", "json")
	params.Set("fl", "timestamp,original,mimetype,statuscode,length")
	params.Set("collapse", "digest")
	params.Set("filter", "statuscode:200")
	params.Set("filter", "mimetype:text/html")
	params.Set("limit", "-5") // Все результаты

	reqURL := waybackCDXURL + "?" + params.Encode()
	// Fix: url.Values encodes duplicate keys weirdly. Build manually.
	reqURL = fmt.Sprintf("%s?url=%s&output=json&fl=timestamp,original,mimetype,statuscode,length&collapse=digest&filter=statuscode:200&filter=mimetype:text/html&limit=-5",
		waybackCDXURL, url.QueryEscape(domain))

	client := &http.Client{Timeout: waybackCDXTimeout}

	var lastErr error
	for attempt := 0; attempt <= waybackCDXRetries; attempt++ {
		if attempt > 0 {
			state.AppendLog(fmt.Sprintf("CDX API попытка %d/%d", attempt+1, waybackCDXRetries+1))
			select {
			case <-time.After(waybackRetryDelay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create CDX request: %w", err)
		}
		req.Header.Set("User-Agent", waybackUserAgent)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("CDX API returned status %d", resp.StatusCode)
			continue
		}

		// Parse JSON response: array of arrays, first row is headers
		var raw [][]any
		if err := json.Unmarshal(body, &raw); err != nil {
			// CDX may return empty response or non-JSON
			if len(bytes.TrimSpace(body)) == 0 {
				return nil, nil // No snapshots
			}
			lastErr = fmt.Errorf("failed to parse CDX response: %w", err)
			continue
		}

		if len(raw) <= 1 {
			return nil, nil // Only headers, no data
		}

		// Parse rows (skip header row)
		var snapshots []waybackSnapshot
		for _, row := range raw[1:] {
			if len(row) < 5 {
				continue
			}
			timestamp := fmt.Sprintf("%v", row[0])
			original := fmt.Sprintf("%v", row[1])
			lengthStr := fmt.Sprintf("%v", row[4])
			length, _ := strconv.Atoi(lengthStr)

			if length < waybackMinLength {
				continue
			}

			snapshots = append(snapshots, waybackSnapshot{
				Timestamp: timestamp,
				Original:  original,
				Length:    length,
			})
		}

		// Sort by timestamp descending (newest first)
		sort.Slice(snapshots, func(i, j int) bool {
			return snapshots[i].Timestamp > snapshots[j].Timestamp
		})

		// Limit to max snapshots
		if len(snapshots) > waybackMaxSnapshots {
			snapshots = snapshots[:waybackMaxSnapshots]
		}

		return snapshots, nil
	}

	return nil, fmt.Errorf("CDX API failed after %d attempts: %w", waybackCDXRetries+1, lastErr)
}

// downloadSnapshot downloads HTML content from a Wayback Machine URL with retries.
func downloadSnapshot(ctx context.Context, rawURL string, state *PipelineState) (string, error) {
	client := &http.Client{Timeout: waybackDownTimeout}

	var lastErr error
	for attempt := 0; attempt <= waybackDownRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(waybackRetryDelay):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create download request: %w", err)
		}
		req.Header.Set("User-Agent", waybackUserAgent)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("download returned status %d", resp.StatusCode)
			continue
		}

		return string(body), nil
	}

	return "", fmt.Errorf("download failed after %d attempts: %w", waybackDownRetries+1, lastErr)
}

// extractFrameURL checks if the HTML is a Wayback Machine frame wrapper
// and extracts the actual content URL from iframe/frame elements.
func extractFrameURL(html, baseURL string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	// Check for <iframe id="playback">
	if src, exists := doc.Find(`iframe#playback`).Attr("src"); exists && src != "" {
		return resolveWaybackURL(src, baseURL)
	}

	// Check for <frame name="main">
	if src, exists := doc.Find(`frame[name="main"]`).Attr("src"); exists && src != "" {
		return resolveWaybackURL(src, baseURL)
	}

	return ""
}

// resolveWaybackURL resolves a potentially relative URL against the base.
func resolveWaybackURL(src, base string) string {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return src
	}
	if strings.HasPrefix(src, "//") {
		return "https:" + src
	}
	if strings.HasPrefix(src, "/") {
		return "https://web.archive.org" + src
	}
	// Relative URL — resolve against base
	if baseURL, err := url.Parse(base); err == nil {
		if ref, err := url.Parse(src); err == nil {
			return baseURL.ResolveReference(ref).String()
		}
	}
	return src
}

// cleanHTMLText removes scripts, styles, navigation, and Wayback Machine artifacts,
// then extracts clean visible text.
func cleanHTMLText(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	// Remove unwanted elements
	doc.Find("script, style, noscript, nav, footer").Remove()

	// Remove Wayback Machine-specific elements
	doc.Find("#wm-ipp-base, #wm-ipp, #wm-ipp-print, .wb-autocomplete-suggestions, #donato, #__wb_panel").Remove()
	doc.Find("[id^='wm-'], [class^='wm-']").Remove()

	// Extract and normalize text
	text := doc.Find("body").Text()
	text = strings.Join(strings.Fields(text), " ")
	text = strings.TrimSpace(text)

	return text
}
