package httpserver

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/idna"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

func parseLinkTaskFilters(r *http.Request) (sqlstore.LinkTaskFilters, error) {
	query := r.URL.Query()
	filters := sqlstore.LinkTaskFilters{SortDesc: true}

	if status := strings.TrimSpace(query.Get("status")); status != "" {
		filters.Status = &status
	}
	if from := strings.TrimSpace(query.Get("scheduled_from")); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return filters, fmt.Errorf("invalid scheduled_from")
		}
		tt := t.UTC()
		filters.ScheduledAfter = &tt
	} else if from := strings.TrimSpace(query.Get("scheduledFrom")); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return filters, fmt.Errorf("invalid scheduled_from")
		}
		tt := t.UTC()
		filters.ScheduledAfter = &tt
	}
	if to := strings.TrimSpace(query.Get("scheduled_to")); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return filters, fmt.Errorf("invalid scheduled_to")
		}
		tt := t.UTC()
		filters.ScheduledBefore = &tt
	} else if to := strings.TrimSpace(query.Get("scheduledTo")); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return filters, fmt.Errorf("invalid scheduled_to")
		}
		tt := t.UTC()
		filters.ScheduledBefore = &tt
	}

	if limitStr := strings.TrimSpace(query.Get("limit")); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			return filters, fmt.Errorf("invalid limit")
		}
		filters.Limit = limit
	}
	if search := strings.TrimSpace(query.Get("search")); search != "" {
		filters.Search = &search
	}
	if sortOrder := strings.ToLower(strings.TrimSpace(query.Get("sort"))); sortOrder != "" {
		switch sortOrder {
		case "asc":
			filters.SortDesc = false
		case "desc":
			filters.SortDesc = true
		default:
			return filters, fmt.Errorf("invalid sort")
		}
	}
	if pageStr := strings.TrimSpace(query.Get("page")); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			return filters, fmt.Errorf("invalid page")
		}
		if filters.Limit <= 0 {
			filters.Limit = 50
		}
		if page > 1 {
			filters.Offset = (page - 1) * filters.Limit
		}
	}

	return filters, nil
}

func parseScheduledFor(primary string, fallback string, defaultTime time.Time) (time.Time, error) {
	value := strings.TrimSpace(primary)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	if value == "" {
		return defaultTime, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

type scheduleConfig struct {
	Limit        int
	Cron         string
	Time         string
	Weekday      string
	Day          int
	Interval     string
	DelayMinutes int
}

func parseScheduleConfig(raw json.RawMessage) (scheduleConfig, error) {
	if len(raw) == 0 {
		return scheduleConfig{DelayMinutes: 5}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return scheduleConfig{}, err
	}
	cfg := scheduleConfig{}
	delaySet := false
	if v, ok := data["limit"].(float64); ok {
		cfg.Limit = int(v)
	}
	if v, ok := data["cron"].(string); ok {
		cfg.Cron = strings.TrimSpace(v)
	}
	if v, ok := data["time"].(string); ok {
		cfg.Time = strings.TrimSpace(v)
	}
	if v, ok := data["weekday"].(string); ok {
		cfg.Weekday = strings.TrimSpace(v)
	}
	if v, ok := data["day"].(float64); ok {
		cfg.Day = int(v)
	}
	if v, ok := data["day"].(string); ok {
		trimmed := strings.TrimSpace(v)
		if parsed, err := strconv.Atoi(trimmed); err == nil {
			cfg.Day = parsed
		} else if trimmed != "" {
			cfg.Weekday = strings.ToLower(trimmed)
		}
	}
	if v, ok := data["interval"].(string); ok {
		cfg.Interval = strings.TrimSpace(v)
	}
	if rawDelay, ok := data["delay_minutes"]; ok {
		delaySet = true
		switch value := rawDelay.(type) {
		case float64:
			cfg.DelayMinutes = int(value)
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				parsed, err := strconv.Atoi(trimmed)
				if err != nil {
					return scheduleConfig{}, fmt.Errorf("invalid delay_minutes")
				}
				cfg.DelayMinutes = parsed
			}
		default:
			return scheduleConfig{}, fmt.Errorf("invalid delay_minutes")
		}
		if cfg.DelayMinutes < 0 {
			return scheduleConfig{}, fmt.Errorf("invalid delay_minutes")
		}
	}
	if !delaySet {
		cfg.DelayMinutes = 5
	}
	return cfg, nil
}

func parseHourMinute(value string, now time.Time) (int, int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, 0, nil
	}
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minute")
	}
	return hour, minute, nil
}

func parseInterval(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("interval is empty")
	}
	if strings.HasSuffix(value, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(value, "w") {
		n, err := strconv.Atoi(strings.TrimSuffix(value, "w"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return time.ParseDuration(value)
}

type domainImportItem struct {
	URL             string `json:"url"`
	Keyword         string `json:"keyword"`
	Country         string `json:"country"`
	Language        string `json:"language"`
	ServerID        string `json:"server_id"`
	Server          string `json:"server"`
	LinkAnchorText  string `json:"link_anchor_text"`
	Anchor          string `json:"anchor"`
	LinkAcceptorURL string `json:"link_acceptor_url"`
	Acceptor        string `json:"acceptor"`
	LinkPlaced      string `json:"link_placed"`
	GenerationType  string `json:"generation_type"`
}

func parseDomainImportText(text string) ([]domainImportItem, error) {
	lines := strings.Split(text, "\n")
	items := make([]domainImportItem, 0, len(lines))
	for idx, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		fields, err := parseDomainImportCSVLine(line)
		if err != nil {
			return nil, fmt.Errorf("invalid import format at line %d", idx+1)
		}
		if len(items) == 0 && domainImportLooksLikeHeader(fields) {
			continue
		}
		item, err := parseDomainImportFields(fields)
		if err != nil {
			return nil, fmt.Errorf("invalid import format at line %d: %s", idx+1, err.Error())
		}
		items = append(items, item)
	}
	return items, nil
}

func parseDomainImportCSVLine(line string) ([]string, error) {
	reader := csv.NewReader(strings.NewReader(line))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true
	fields, err := reader.Read()
	if err != nil {
		return nil, err
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	if len(fields) > 0 {
		fields[0] = strings.TrimPrefix(fields[0], "\ufeff")
	}
	return fields, nil
}

func parseDomainImportFields(fields []string) (domainImportItem, error) {
	if len(fields) == 0 {
		return domainImportItem{}, errors.New("empty line")
	}
	if len(fields) > 9 {
		return domainImportItem{}, errors.New("too many columns")
	}
	url := strings.TrimSpace(fields[0])
	if url == "" {
		return domainImportItem{}, errors.New("domain is required")
	}
	item := domainImportItem{URL: url}
	if len(fields) > 1 {
		item.Keyword = strings.TrimSpace(fields[1])
	}
	if len(fields) > 2 {
		item.Country = strings.TrimSpace(fields[2])
	}
	if len(fields) > 3 {
		item.Language = strings.TrimSpace(fields[3])
	}
	if len(fields) > 4 {
		item.ServerID = strings.TrimSpace(fields[4])
	}
	if len(fields) > 5 {
		item.LinkAnchorText = strings.TrimSpace(fields[5])
	}
	if len(fields) > 6 {
		item.LinkAcceptorURL = strings.TrimSpace(fields[6])
	}
	if len(fields) > 7 {
		item.LinkPlaced = strings.TrimSpace(fields[7])
	}
	if len(fields) > 8 {
		item.GenerationType = strings.TrimSpace(fields[8])
	}
	return item, nil
}

func isDateString(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func domainImportLooksLikeHeader(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(fields[0], "\ufeff")))
	if v == "url" || v == "domain" {
		return true
	}
	for _, f := range fields {
		lf := strings.ToLower(strings.TrimSpace(f))
		if lf == "generation_type" || lf == "type" {
			return true
		}
	}
	return false
}

func normalizeImportedDomain(input string) (string, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", errors.New("domain is required")
	}
	candidate := raw
	if strings.HasPrefix(candidate, "//") {
		candidate = "https:" + candidate
	} else if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", errors.New("invalid domain")
	}
	if parsed.User != nil {
		return "", errors.New("invalid domain")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("path is not allowed")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("query is not allowed")
	}
	if parsed.Port() != "" {
		return "", errors.New("port is not allowed")
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return "", errors.New("domain is required")
	}
	asciiHost, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", errors.New("invalid domain")
	}
	asciiHost = strings.ToLower(strings.TrimSpace(asciiHost))
	if net.ParseIP(asciiHost) != nil {
		return "", errors.New("ip address is not allowed")
	}
	if !isValidDomainHost(asciiHost) {
		return "", errors.New("invalid domain")
	}
	return asciiHost, nil
}

func isValidDomainHost(host string) bool {
	if len(host) == 0 || len(host) > 253 || !strings.Contains(host, ".") {
		return false
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			ch := label[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sanitizeDomain(input string) string {
	s := strings.TrimSpace(input)
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "//")
	s = strings.TrimRight(s, "/")
	return s
}

func parseIndexCheckFilters(r *http.Request, fallbackLimit int, maxLimit int) sqlstore.IndexCheckFilters {
	limit := parseLimitParam(r, fallbackLimit, maxLimit)
	page := parsePageParam(r, 1)
	offset := (page - 1) * limit
	filters := sqlstore.IndexCheckFilters{
		Limit:  limit,
		Offset: offset,
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		filters.Statuses = parseStatusList(status)
	}
	if search := strings.TrimSpace(r.URL.Query().Get("search")); search != "" {
		filters.Search = &search
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("sort")); raw != "" {
		if key, dir := parseIndexCheckSort(raw); key != "" {
			filters.SortBy = key
			filters.SortDir = dir
		}
	}
	if domainID := strings.TrimSpace(r.URL.Query().Get("domain_id")); domainID != "" {
		filters.DomainID = &domainID
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("is_indexed")); raw != "" {
		if val, err := strconv.ParseBool(raw); err == nil {
			filters.IsIndexed = &val
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		if dt, ok := parseIndexCheckDate(raw); ok {
			filters.From = &dt
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		if dt, ok := parseIndexCheckDate(raw); ok {
			filters.To = &dt
		}
	}
	return filters
}

func parseLLMUsageFilters(r *http.Request) sqlstore.LLMUsageFilters {
	limit := parseLimitParam(r, 50, 500)
	page := parsePageParam(r, 1)
	offset := (page - 1) * limit
	filters := sqlstore.LLMUsageFilters{
		Limit:  limit,
		Offset: offset,
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		if t, ok := parseLLMUsageTime(raw, true); ok {
			filters.From = &t
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		if t, ok := parseLLMUsageTime(raw, false); ok {
			filters.To = &t
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("user_email")); v != "" {
		filters.UserEmail = &v
	}
	if v := strings.TrimSpace(r.URL.Query().Get("project_id")); v != "" {
		filters.ProjectID = &v
	}
	if v := strings.TrimSpace(r.URL.Query().Get("domain_id")); v != "" {
		filters.DomainID = &v
	}
	if v := strings.TrimSpace(r.URL.Query().Get("model")); v != "" {
		filters.Model = &v
	}
	if v := strings.TrimSpace(r.URL.Query().Get("operation")); v != "" {
		filters.Operation = &v
	}
	if v := strings.TrimSpace(r.URL.Query().Get("status")); v != "" {
		filters.Status = &v
	}
	return filters
}

type llmUsageGroupBySelection struct {
	ByDay       bool
	ByModel     bool
	ByOperation bool
	ByUser      bool
}

func parseLLMUsageGroupBy(raw string) llmUsageGroupBySelection {
	selection := llmUsageGroupBySelection{
		ByDay:       true,
		ByModel:     true,
		ByOperation: true,
		ByUser:      true,
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "all") {
		return selection
	}
	selection = llmUsageGroupBySelection{}
	for _, part := range strings.Split(raw, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "all":
			return llmUsageGroupBySelection{ByDay: true, ByModel: true, ByOperation: true, ByUser: true}
		case "day":
			selection.ByDay = true
		case "model":
			selection.ByModel = true
		case "operation":
			selection.ByOperation = true
		case "user":
			selection.ByUser = true
		}
	}
	if !selection.ByDay && !selection.ByModel && !selection.ByOperation && !selection.ByUser {
		return llmUsageGroupBySelection{ByDay: true, ByModel: true, ByOperation: true, ByUser: true}
	}
	return selection
}

func filterLLMUsageStatsDTO(dto llmUsageStatsDTO, groupBy llmUsageGroupBySelection) llmUsageStatsDTO {
	if !groupBy.ByDay {
		dto.ByDay = []llmUsageBucketDTO{}
	}
	if !groupBy.ByModel {
		dto.ByModel = []llmUsageBucketDTO{}
	}
	if !groupBy.ByOperation {
		dto.ByOperation = []llmUsageBucketDTO{}
	}
	if !groupBy.ByUser {
		dto.ByUser = []llmUsageBucketDTO{}
	}
	return dto
}

func parseLLMUsageTime(raw string, startOfDay bool) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), true
	}
	if d, err := time.Parse("2006-01-02", raw); err == nil {
		d = d.UTC()
		if !startOfDay {
			d = d.Add(24*time.Hour - time.Nanosecond)
		}
		return d, true
	}
	return time.Time{}, false
}

func resolveIndexCheckStatsRange(filters *sqlstore.IndexCheckFilters, now time.Time) (time.Time, time.Time) {
	const defaultDays = 30
	var from time.Time
	var to time.Time
	if filters.From != nil {
		from = dateOnlyUTC(*filters.From)
	}
	if filters.To != nil {
		to = dateOnlyUTC(*filters.To)
	}
	switch {
	case !from.IsZero() && !to.IsZero():
		// keep
	case !from.IsZero() && to.IsZero():
		to = dateOnlyUTC(now)
	case from.IsZero() && !to.IsZero():
		from = dateOnlyUTC(to.AddDate(0, 0, -defaultDays+1))
	default:
		to = dateOnlyUTC(now)
		from = dateOnlyUTC(now.AddDate(0, 0, -defaultDays+1))
	}
	if from.After(to) {
		from, to = to, from
	}
	filters.From = &from
	filters.To = &to
	return from, to
}

func resolveIndexCheckCalendarRange(filters *sqlstore.IndexCheckFilters, r *http.Request, now time.Time) (time.Time, time.Time) {
	if month := strings.TrimSpace(r.URL.Query().Get("month")); month != "" {
		if from, to, ok := parseIndexCheckMonth(month); ok {
			filters.From = &from
			filters.To = &to
			return from, to
		}
	}
	if filters.From != nil || filters.To != nil {
		from, to := resolveIndexCheckStatsRange(filters, now)
		return from, to
	}
	current := dateOnlyUTC(now)
	from := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(current.Year(), current.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	filters.From = &from
	filters.To = &to
	return from, to
}

func parseIndexCheckMonth(raw string) (time.Time, time.Time, bool) {
	t, err := time.Parse("2006-01", raw)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	from := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	return from, to, true
}

func parseStatusList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		status := strings.TrimSpace(part)
		if status == "" {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		out = append(out, status)
	}
	return out
}

func parseIndexCheckSort(raw string) (string, string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", ""
	}
	parts := strings.SplitN(value, ":", 2)
	key := strings.TrimSpace(parts[0])
	if !isAllowedIndexCheckSort(key) {
		return "", ""
	}
	dir := "desc"
	if len(parts) > 1 {
		switch strings.ToLower(strings.TrimSpace(parts[1])) {
		case "asc":
			dir = "asc"
		case "desc":
			dir = "desc"
		}
	}
	return key, dir
}

func isAllowedIndexCheckSort(key string) bool {
	switch key {
	case "domain", "check_date", "status", "attempts", "is_indexed", "last_attempt_at", "next_retry_at", "created_at":
		return true
	default:
		return false
	}
}

func parseIndexCheckDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if dt, err := time.Parse(time.RFC3339, raw); err == nil {
		return dateOnlyUTC(dt), true
	}
	if dt, err := time.Parse("2006-01-02", raw); err == nil {
		return dateOnlyUTC(dt), true
	}
	return time.Time{}, false
}

func dateOnlyUTC(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func sanitizeFilePath(p string) (string, error) {
	if strings.Contains(p, "\\") {
		return "", errors.New("path contains backslash")
	}
	clean := path.Clean(strings.TrimSpace(p))
	if clean == "." || clean == "" {
		return "", errors.New("path is empty")
	}
	if path.IsAbs(clean) {
		return "", errors.New("path is absolute")
	}
	if strings.HasPrefix(clean, "../") || clean == ".." {
		return "", errors.New("path traversal detected")
	}
	parts := strings.Split(clean, "/")
	for _, part := range parts {
		if part == ".." {
			return "", errors.New("path traversal detected")
		}
	}
	return clean, nil
}

func validateMimeType(path string, detected string, existing string) error {
	if strings.TrimSpace(detected) == "" {
		return errors.New("mime type is empty")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return errors.New("file extension is required")
	}
	expected := mime.TypeByExtension(ext)
	if expected == "" && baseMimeType(detected) == "application/octet-stream" {
		return errors.New("unsupported mime type")
	}
	if expected != "" && baseMimeType(detected) != baseMimeType(expected) {
		return fmt.Errorf("mime type mismatch: expected %s, got %s", baseMimeType(expected), baseMimeType(detected))
	}
	if existing != "" && baseMimeType(existing) != baseMimeType(detected) {
		return fmt.Errorf("mime type mismatch: expected %s, got %s", baseMimeType(existing), baseMimeType(detected))
	}
	return nil
}

func validateEditorPath(relPath string) error {
	parts := strings.Split(relPath, "/")
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" || p == "." || p == ".." {
			return errors.New("invalid path segment")
		}
		if strings.HasPrefix(p, ".") && p != ".well-known" {
			return errors.New("hidden files are not allowed")
		}
		switch strings.ToLower(p) {
		case ".git", ".gitignore", ".env", ".env.local", ".env.production", ".env.development":
			return errors.New("protected path is not allowed")
		}
	}
	return nil
}

var blockedUploadExtensions = map[string]struct{}{
	".exe":   {},
	".dll":   {},
	".so":    {},
	".dylib": {},
	".msi":   {},
	".bin":   {},
	".com":   {},
	".cmd":   {},
	".bat":   {},
	".ps1":   {},
	".sh":    {},
	".bash":  {},
	".zsh":   {},
	".py":    {},
	".rb":    {},
	".pl":    {},
	".jar":   {},
	".class": {},
	".apk":   {},
	".deb":   {},
	".rpm":   {},
	".php":   {},
	".phtml": {},
	".phar":  {},
	".cgi":   {},
}

func validateUploadPathPolicy(relPath string) error {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(relPath)))
	if ext == "" {
		return nil
	}
	if _, blocked := blockedUploadExtensions[ext]; blocked {
		return fmt.Errorf("file type %s is not allowed", ext)
	}
	return nil
}

func isImageExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return true
	default:
		return false
	}
}

func isImagePath(path string) bool {
	return isImageExt(filepath.Ext(path))
}

func isImageMime(mimeType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(baseMimeType(mimeType))), "image/")
}

func imageExtByMime(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(baseMimeType(mimeType))) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	default:
		return ""
	}
}

func validateUploadSecurity(relPath, mimeType string, content []byte) error {
	if err := validateUploadPathPolicy(relPath); err != nil {
		return err
	}
	normalized := strings.ToLower(strings.TrimSpace(baseMimeType(mimeType)))
	switch normalized {
	case "application/x-dosexec",
		"application/x-msdownload",
		"application/x-msdos-program",
		"application/x-executable",
		"application/x-elf",
		"application/x-mach-binary",
		"application/x-sh":
		return fmt.Errorf("mime type %s is not allowed", normalized)
	}
	if strings.HasPrefix(normalized, "application/x-ms") {
		return fmt.Errorf("mime type %s is not allowed", normalized)
	}
	if strings.HasSuffix(strings.ToLower(relPath), ".svg") {
		lower := strings.ToLower(string(content))
		if strings.Contains(lower, "<script") {
			return errors.New("inline scripts in svg are not allowed")
		}
		if !strings.Contains(lower, "<svg") {
			return errors.New("invalid svg content")
		}
	}
	if strings.HasPrefix(normalized, "image/") {
		if err := validateImagePayload(relPath, normalized, content); err != nil {
			return err
		}
	}
	return nil
}

func validateImagePayload(relPath, mimeType string, content []byte) error {
	if len(content) == 0 {
		return errors.New("empty image content")
	}
	ext := strings.ToLower(filepath.Ext(relPath))
	switch ext {
	case ".png":
		if len(content) < 8 || !bytes.Equal(content[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) {
			return errors.New("invalid png signature")
		}
	case ".jpg", ".jpeg":
		if len(content) < 3 || content[0] != 0xFF || content[1] != 0xD8 {
			return errors.New("invalid jpeg signature")
		}
	case ".gif":
		if len(content) < 6 {
			return errors.New("invalid gif signature")
		}
		head := string(content[:6])
		if head != "GIF87a" && head != "GIF89a" {
			return errors.New("invalid gif signature")
		}
	case ".webp":
		if len(content) < 12 || string(content[:4]) != "RIFF" || string(content[8:12]) != "WEBP" {
			return errors.New("invalid webp signature")
		}
	case ".svg":
		// already validated above in validateUploadSecurity.
		return nil
	}

	if strings.HasPrefix(mimeType, "image/") && ext != ".svg" {
		if _, _, err := image.DecodeConfig(bytes.NewReader(content)); err != nil {
			return fmt.Errorf("invalid image data: %w", err)
		}
	}
	return nil
}

func normalizeRevisionSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "ai":
		return "ai"
	case "agent":
		return "agent"
	case "agent_partial":
		return "agent_partial"
	case "revert":
		return "revert"
	default:
		return "manual"
	}
}

func normalizeImageGenerationModel(candidates ...string) string {
	for _, candidate := range candidates {
		model := strings.TrimSpace(candidate)
		if model == "" {
			continue
		}
		if isImageGenerationModel(model) {
			return model
		}
	}
	return "gemini-2.5-flash-image"
}
