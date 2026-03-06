package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

// Config управляет параметрами анализа.
type Config struct {
	MaxResults  int
	WorkerCount int
}

// Result содержит итог работы анализа выдачи/страниц.
type Result struct {
	SerpItems []SerpItem
	Pages     []PageRow
	RawSerp   map[string]any
	CSV       string
	Contents  string
	Logs      []string
}

type SerpItem struct {
	Position int
	Title    string
	Link     string
}

type PageRow struct {
	URL           string   `json:"url"`
	FinalURL      string   `json:"final_url"`
	StatusCode    int      `json:"status_code"`
	Domain        string   `json:"domain"`
	PageTitle     string   `json:"page_title"`
	PageTitleLen  int      `json:"page_title_len"`
	WordCount     int      `json:"word_count"`
	CharCount     int      `json:"char_count"`
	H1Count       int      `json:"h1_count"`
	H2Count       int      `json:"h2_count"`
	H1List        []string `json:"h1_list"`
	H2List        []string `json:"h2_list"`
	ContentText   string   `json:"content_text"`
	ContentHTML   string   `json:"content_html"`
	ReadingTime   float64  `json:"reading_time_min"`
	TfidfTopTerms []string `json:"tfidf_top_terms"`
	Position      int      `json:"position"`
}

// Analyze выполняет полный цикл: получает выдачу, скачивает страницы, считает tf-idf и готовит артефакты.
func Analyze(ctx context.Context, keyword, country, lang string, exclude []string, cfg Config) (*Result, error) {
	if cfg.MaxResults == 0 {
		cfg.MaxResults = 20
	}
	if cfg.WorkerCount == 0 {
		cfg.WorkerCount = 5
	}

	logs := []string{}
	logf := func(msg string) {
		logs = append(logs, time.Now().Format(time.RFC3339)+" "+msg)
	}

	serpItems, raw, err := fetchSerp(ctx, keyword, country, lang, cfg.MaxResults)
	if err != nil {
		return nil, err
	}
	logf(fmt.Sprintf("SERP получен, %d позиций", len(serpItems)))

	excludeSet := make(map[string]struct{})
	for _, host := range exclude {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		excludeSet[extractDomain(host)] = struct{}{}
	}

	pages := crawlPages(ctx, serpItems, lang, keyword, excludeSet, cfg.MaxResults, cfg.WorkerCount, logf)
	logf(fmt.Sprintf("проанализировано страниц: %d", len(pages)))
	computeTFIDF(pages, keyword, lang)
	logf("TF-IDF рассчитан")

	csv := buildCSV(keyword, country, lang, serpItems, pages)
	contents := buildContents(keyword, serpItems, pages)

	return &Result{
		SerpItems: serpItems,
		Pages:     pages,
		RawSerp:   raw,
		CSV:       csv,
		Contents:  contents,
		Logs:      logs,
	}, nil
}

func fetchSerp(ctx context.Context, keyword, country, lang string, limit int) ([]SerpItem, map[string]any, error) {
	country, lang = NormalizeSerpGeoLang(country, lang)

	base := "https://alfasearchspy.alfasearch.ru/api/v1/ranking/parse"
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("geo", country)
	params.Set("lang", lang)
	params.Set("group_by", strconv.Itoa(limit))
	params.Set("device", "MOBILE")
	params.Set("real_time", "true")

	timeout := serpTimeout()
	retries := serpRetries()
	var raw map[string]any
	var lastErr error
	client := &http.Client{Timeout: timeout}
	for attempt := 0; attempt <= retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+params.Encode(), nil)
		if err != nil {
			return nil, nil, err
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
					return nil, nil, ctx.Err()
				}
			}
			return nil, nil, lastErr
		}
		if resp.StatusCode >= 400 {
			_ = resp.Body.Close()
			return nil, nil, fmt.Errorf("serp status %d for keyword %q", resp.StatusCode, keyword)
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
					return nil, nil, ctx.Err()
				}
			}
			return nil, nil, lastErr
		}
		if err := resp.Body.Close(); err != nil {
			lastErr = fmt.Errorf("serp body close failed for keyword %q: %v", keyword, err)
			if isRetriableSerpErr(err) && attempt < retries {
				backoff := time.Duration(2*(attempt+1)) * time.Second
				select {
				case <-time.After(backoff):
					continue
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
			}
			return nil, nil, err
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		return nil, nil, lastErr
	}

	items := []SerpItem{}
	if positions, ok := raw["positions"].([]any); ok {
		for _, it := range positions {
			entry, _ := it.(map[string]any)
			var link string
			if uo, ok := entry["url"].(map[string]any); ok {
				if u, ok := uo["url"].(string); ok {
					link = u
				}
			}
			title, _ := entry["title"].(string)
			if link == "" || title == "" {
				continue
			}
			items = append(items, SerpItem{
				Position: toInt(entry["position"]),
				Title:    title,
				Link:     link,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Position < items[j].Position })
	return items, raw, nil
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

func crawlPages(ctx context.Context, items []SerpItem, lang, keyword string, excludes map[string]struct{}, limit, workers int, logf func(string)) []PageRow {
	type result struct {
		index int
		row   PageRow
	}
	results := make([]result, 0, limit)
	var mu sync.Mutex

	wg := sync.WaitGroup{}
	sem := make(chan struct{}, workers)

	addJob := 0
	for _, item := range items {
		if addJob >= limit {
			break
		}
		if _, skip := excludes[extractDomain(item.Link)]; skip {
			continue
		}
		addJob++
		pos := addJob
		wg.Add(1)
		sem <- struct{}{}
		go func(it SerpItem, position int) {
			defer wg.Done()
			defer func() { <-sem }()
			row := analyzePage(ctx, it.Link, lang, keyword)
			row.Position = position
			mu.Lock()
			results = append(results, result{index: position, row: row})
			mu.Unlock()
		}(item, pos)
	}

	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })
	rows := make([]PageRow, 0, len(results))
	for _, r := range results {
		rows = append(rows, r.row)
	}
	logf(fmt.Sprintf("скачано страниц: %d", len(rows)))
	return rows
}

func analyzePage(ctx context.Context, rawURL, lang, keyword string) PageRow {
	row := PageRow{
		URL:      rawURL,
		FinalURL: rawURL,
		Domain:   extractDomain(rawURL),
	}
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return row
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Accept-Language", lang+",en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return row
	}
	defer resp.Body.Close()
	row.StatusCode = resp.StatusCode
	row.FinalURL = resp.Request.URL.String()
	if resp.StatusCode >= 400 {
		return row
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return row
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err == nil {
		if row.PageTitle == "" {
			row.PageTitle = strings.TrimSpace(doc.Find("title").Text())
		}
		doc.Find("body script, body style, body noscript").Remove()
		if row.ContentText == "" {
			row.ContentText = strings.Join(strings.Fields(doc.Find("body").Text()), " ")
		}
		if row.ContentHTML == "" {
			if html, err := doc.Find("body").Html(); err == nil {
				row.ContentHTML = html
			}
		}
		doc.Find("h1").Each(func(_ int, s *goquery.Selection) {
			row.H1List = append(row.H1List, strings.TrimSpace(s.Text()))
		})
		doc.Find("h2").Each(func(_ int, s *goquery.Selection) {
			row.H2List = append(row.H2List, strings.TrimSpace(s.Text()))
		})
	}

	row.PageTitle = strings.TrimSpace(row.PageTitle)
	row.PageTitleLen = len(row.PageTitle)
	row.ContentText = strings.TrimSpace(row.ContentText)
	row.CharCount = len(row.ContentText)
	row.WordCount = len(tokenize(row.ContentText))
	if row.WordCount > 0 {
		row.ReadingTime = float64(row.WordCount) / 200.0
	}
	row.H1Count = len(row.H1List)
	row.H2Count = len(row.H2List)
	return row
}

func computeTFIDF(rows []PageRow, keyword, lang string) {
	if len(rows) == 0 {
		return
	}
	keywordTokens := make(map[string]struct{})
	for _, tok := range tokenize(keyword) {
		if tok != "" {
			keywordTokens[tok] = struct{}{}
		}
	}
	stop := stopwordSet(lang)

	type termData struct {
		tf    map[string]float64
		total float64
	}
	termStats := make([]termData, len(rows))
	docFreq := map[string]int{}
	validDocs := 0
	for i, row := range rows {
		tokens := tokenize(row.ContentText)
		if len(tokens) == 0 {
			continue
		}
		freq := map[string]float64{}
		for _, tok := range tokens {
			if len(tok) < 3 {
				continue
			}
			if _, banned := stop[tok]; banned {
				continue
			}
			if _, used := keywordTokens[tok]; used {
				continue
			}
			freq[tok]++
		}
		if len(freq) == 0 {
			continue
		}
		validDocs++
		termStats[i] = termData{tf: freq}
		for _, count := range freq {
			termStats[i].total += count
		}
		for tok := range freq {
			docFreq[tok]++
		}
	}
	if validDocs == 0 {
		return
	}
	for i := range rows {
		stats := termStats[i]
		if len(stats.tf) == 0 || stats.total == 0 {
			continue
		}
		type scored struct {
			term  string
			score float64
		}
		scores := make([]scored, 0, len(stats.tf))
		for term, count := range stats.tf {
			tf := count / stats.total
			idf := math.Log((float64(validDocs)+1)/(float64(docFreq[term])+1)) + 1
			scores = append(scores, scored{term: term, score: tf * idf})
		}
		sort.Slice(scores, func(a, b int) bool { return scores[a].score > scores[b].score })
		maxTerms := 20
		if len(scores) < maxTerms {
			maxTerms = len(scores)
		}
		terms := make([]string, 0, maxTerms)
		for idx := 0; idx < maxTerms; idx++ {
			terms = append(terms, scores[idx].term)
		}
		rows[i].TfidfTopTerms = terms
	}
}

func buildCSV(keyword, country, lang string, serp []SerpItem, pages []PageRow) string {
	var b strings.Builder
	b.WriteString("query,country,lang,position,title,link,final_url,status_code,domain,page_title,page_title_len,word_count,char_count,reading_time_min,h1_count,h2_count,h1_list,h2_list,tfidf_top_terms\n")
	posMap := make(map[string]int)
	for _, it := range serp {
		posMap[normalizeURL(it.Link)] = it.Position
	}

	wordCounts := make([]int, 0, len(pages))
	for _, p := range pages {
		wordCounts = append(wordCounts, p.WordCount)
		pos := posMap[normalizeURL(p.FinalURL)]
		writeCSVRow(&b, []string{
			keyword,
			country,
			lang,
			fmtInt(pos),
			quote(p.PageTitle),
			quote(p.URL),
			quote(p.FinalURL),
			fmtInt(p.StatusCode),
			p.Domain,
			quote(p.PageTitle),
			fmtInt(p.PageTitleLen),
			fmtInt(p.WordCount),
			fmtInt(p.CharCount),
			fmtFloat(p.ReadingTime),
			fmtInt(p.H1Count),
			fmtInt(p.H2Count),
			quote(strings.Join(p.H1List, " | ")),
			quote(strings.Join(p.H2List, " | ")),
			quote(strings.Join(p.TfidfTopTerms, " | ")),
		})
	}
	if len(wordCounts) > 0 {
		avg := average(wordCounts)
		median := median(wordCounts)
		max := maxValue(wordCounts)
		writeCSVRow(&b, []string{
			keyword,
			country,
			lang,
			"AGGREGATES",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			fmtFloat(avg),
			"",
			"",
			"",
			"",
			"",
			"median=" + fmtFloat(median),
			"max=" + fmtFloat(max),
			"",
		})
	}
	return b.String()
}

func buildContents(keyword string, serp []SerpItem, pages []PageRow) string {
	var b strings.Builder
	b.WriteString("# HTML-содержимое Top-20\n")
	b.WriteString("# Запрос: " + keyword + "\n")
	b.WriteString("# Дата: " + time.Now().Format("2006-01-02") + "\n\n")
	positions := make(map[string]int)
	for i, it := range serp {
		positions[normalizeURL(it.Link)] = i + 1
	}
	sort.Slice(pages, func(i, j int) bool {
		return positions[normalizeURL(pages[i].FinalURL)] < positions[normalizeURL(pages[j].FinalURL)]
	})
	for idx, p := range pages {
		if idx > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(fmt.Sprintf("--- САЙТ %d (URL: %s) ---\n\n", idx+1, p.FinalURL))
		if strings.TrimSpace(p.ContentHTML) == "" {
			b.WriteString("[ОСНОВНОЙ HTML-КОНТЕНТ НЕ ИЗВЛЕЧЕН]\n")
		} else {
			b.WriteString(p.ContentHTML)
		}
	}
	return b.String()
}

func writeCSVRow(b *strings.Builder, fields []string) {
	for i, f := range fields {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(f)
	}
	b.WriteString("\n")
}

func normalizeURL(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return strings.TrimSpace(u)
	}
	parsed.Fragment = ""
	return parsed.String()
}

func fmtInt(v int) string {
	if v == 0 {
		return "0"
	}
	return strconv.Itoa(v)
}

func fmtFloat(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}

func quote(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func extractDomain(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return strings.TrimPrefix(raw, "www.")
	}
	host := parsed.Hostname()
	return strings.TrimPrefix(host, "www.")
}

func tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
}

func stopwordSet(lang string) map[string]struct{} {
	data, ok := stopwords[strings.ToLower(lang)]
	if !ok {
		data = stopwords["default"]
	}
	res := make(map[string]struct{}, len(data))
	for _, w := range data {
		res[w] = struct{}{}
	}
	return res
}

// langToCountry maps ISO 639-1 language codes to ISO 3166-1 Alpha-2 country
// codes. The external SERP API expects country codes in both `geo` and `lang`
// parameters (despite the name). If a user accidentally passes a language code
// in `lang`, this table lets us silently fix it.
var langToCountry = map[string]string{
	"da": "dk", // Danish   → Denmark
	"sv": "se", // Swedish  → Sweden
	"nb": "no", // Norwegian Bokmål → Norway
	"nn": "no", // Norwegian Nynorsk → Norway
	"fi": "fi", // Finnish  → Finland (same code)
	"en": "us", // English  → USA (default)
	"de": "de", // German   → Germany (same code)
	"fr": "fr", // French   → France  (same code)
	"es": "es", // Spanish  → Spain   (same code)
	"it": "it", // Italian  → Italy   (same code)
	"pt": "pt", // Portuguese → Portugal (same code)
	"nl": "nl", // Dutch    → Netherlands (same code)
	"pl": "pl", // Polish   → Poland (same code)
	"ru": "ru", // Russian  → Russia (same code)
	"uk": "ua", // Ukrainian → Ukraine
	"cs": "cz", // Czech    → Czech Republic
	"el": "gr", // Greek    → Greece
	"ja": "jp", // Japanese → Japan
	"ko": "kr", // Korean   → South Korea
	"zh": "cn", // Chinese  → China
	"ar": "sa", // Arabic   → Saudi Arabia (default)
	"he": "il", // Hebrew   → Israel
	"hi": "in", // Hindi    → India
	"th": "th", // Thai     → Thailand (same code)
	"vi": "vn", // Vietnamese → Vietnam
	"tr": "tr", // Turkish  → Turkey (same code)
	"ro": "ro", // Romanian → Romania (same code)
	"et": "ee", // Estonian → Estonia
	"lv": "lv", // Latvian  → Latvia (same code)
	"lt": "lt", // Lithuanian → Lithuania (same code)
	"sl": "si", // Slovenian → Slovenia
	"hr": "hr", // Croatian → Croatia (same code)
	"sk": "sk", // Slovak   → Slovakia (same code)
	"bg": "bg", // Bulgarian → Bulgaria (same code)
	"sr": "rs", // Serbian  → Serbia
	"ka": "ge", // Georgian → Georgia
}

// validCountryCodes contains known ISO 3166-1 Alpha-2 country codes that the
// SERP API accepts. Used to detect if a value is already a valid country code.
var validCountryCodes = func() map[string]struct{} {
	codes := []string{
		"ad", "ae", "af", "ag", "al", "am", "ao", "ar", "at", "au",
		"az", "ba", "bb", "bd", "be", "bf", "bg", "bh", "bi", "bj",
		"bn", "bo", "br", "bs", "bt", "bw", "by", "bz", "ca", "cd",
		"cf", "cg", "ch", "ci", "cl", "cm", "cn", "co", "cr", "cu",
		"cy", "cz", "de", "dj", "dk", "dm", "do", "dz", "ec", "ee",
		"eg", "er", "es", "et", "fi", "fj", "fr", "ga", "gb", "ge",
		"gh", "gm", "gn", "gq", "gr", "gt", "gw", "gy", "hk", "hn",
		"hr", "ht", "hu", "id", "ie", "il", "in", "iq", "ir", "is",
		"it", "jm", "jo", "jp", "ke", "kg", "kh", "kr", "kw", "kz",
		"la", "lb", "lk", "lr", "ls", "lt", "lu", "lv", "ly", "ma",
		"md", "me", "mg", "mk", "ml", "mm", "mn", "mo", "mr", "mt",
		"mu", "mv", "mw", "mx", "my", "mz", "na", "ne", "ng", "ni",
		"nl", "no", "np", "nz", "om", "pa", "pe", "pg", "ph", "pk",
		"pl", "pr", "ps", "pt", "py", "qa", "ro", "rs", "ru", "rw",
		"sa", "sc", "sd", "se", "sg", "si", "sk", "sl", "sn", "so",
		"sr", "ss", "sv", "sy", "sz", "td", "tg", "th", "tj", "tl",
		"tm", "tn", "to", "tr", "tt", "tw", "tz", "ua", "ug", "us",
		"uy", "uz", "ve", "vn", "ws", "ye", "za", "zm", "zw",
	}
	m := make(map[string]struct{}, len(codes))
	for _, c := range codes {
		m[c] = struct{}{}
	}
	return m
}()

// NormalizeSerpGeoLang normalizes country and lang parameters for the external
// SERP API. The API expects ISO 3166-1 country codes in both fields (despite
// `lang` suggesting a language code). This function:
//   - lowercases both values
//   - maps language codes (e.g. "da") to country codes (e.g. "dk") in `lang`
//   - validates `country` is a known country code
func NormalizeSerpGeoLang(country, lang string) (string, string) {
	country = strings.ToLower(strings.TrimSpace(country))
	lang = strings.ToLower(strings.TrimSpace(lang))

	// Normalize country: if it looks like a language code, map to country
	if _, isCountry := validCountryCodes[country]; !isCountry && country != "" {
		if mapped, ok := langToCountry[country]; ok {
			country = mapped
		}
	}

	// Normalize lang: the SERP API expects a country code here too.
	// Always check langToCountry first — in the `lang` field, users almost
	// always mean a language code (e.g. "sv" = Swedish, not El Salvador).
	if lang != "" {
		if mapped, ok := langToCountry[lang]; ok {
			lang = mapped
		}
	}

	return country, lang
}

var stopwords = map[string][]string{
	"default": {"and", "the", "for", "are", "but", "this", "that", "from", "with", "have", "has", "not", "you", "your"},
	"en":      {"and", "the", "for", "are", "but", "this", "that", "from", "with", "have", "has", "not", "you", "your"},
	"sv":      {"och", "att", "det", "som", "med", "den", "på", "inte", "har", "för", "ett", "om", "också", "till", "deras", "vara"},
	"ru":      {"и", "в", "во", "не", "что", "он", "на", "я", "с", "со", "как", "а", "то", "все"},
}

func toInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}

func average(nums []int) float64 {
	if len(nums) == 0 {
		return 0
	}
	var sum int
	for _, n := range nums {
		sum += n
	}
	return float64(sum) / float64(len(nums))
}

func median(nums []int) float64 {
	if len(nums) == 0 {
		return 0
	}
	cp := append([]int(nil), nums...)
	sort.Ints(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return float64(cp[mid-1]+cp[mid]) / 2
	}
	return float64(cp[mid])
}

func maxValue(nums []int) float64 {
	if len(nums) == 0 {
		return 0
	}
	max := nums[0]
	for _, n := range nums {
		if n > max {
			max = n
		}
	}
	return float64(max)
}
