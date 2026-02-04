package pipeline

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

// AuditStep выполняет аудит сгенерированного сайта и сохраняет отчет.
type AuditStep struct{}

func (s *AuditStep) Name() string { return StepAudit }

func (s *AuditStep) ArtifactKey() string { return "audit_report" }

func (s *AuditStep) Progress() int { return 99 }

func (s *AuditStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	files := parseGeneratedFiles(state.Artifacts["generated_files"])
	if len(files) == 0 {
		return nil, fmt.Errorf("audit: generated_files is empty")
	}

	fileMap := make(map[string]GeneratedFile)
	for _, f := range files {
		clean := normalizePath(f.Path)
		if clean == "" {
			continue
		}
		fileMap[clean] = f
	}

	findings := make([]auditFinding, 0)

	required := []string{"index.html", "style.css", "script.js", "404.html"}
	for _, req := range required {
		if _, ok := fileMap[req]; !ok {
			findings = append(findings, auditFinding{
				RuleCode: "missing_required_file",
				Severity: "error",
				Message:  fmt.Sprintf("missing required file: %s", req),
				FilePath: req,
			})
		}
	}

	missing := map[string]map[string]bool{}
	for _, f := range files {
		ext := strings.ToLower(path.Ext(f.Path))
		if ext != ".html" && ext != ".css" {
			continue
		}
		content := fileTextContent(f)
		if strings.TrimSpace(content) == "" {
			continue
		}
		refs := extractRefs(f.Path, content, ext)
		for _, ref := range refs {
			if _, ok := fileMap[ref]; ok {
				continue
			}
			if missing[ref] == nil {
				missing[ref] = map[string]bool{}
			}
			missing[ref][f.Path] = true
		}
	}

	for ref, sources := range missing {
		srcList := make([]string, 0, len(sources))
		for src := range sources {
			srcList = append(srcList, src)
		}
		details, _ := json.Marshal(map[string]any{
			"ref":     ref,
			"sources": srcList,
		})
		findings = append(findings, auditFinding{
			RuleCode: "missing_asset",
			Severity: "warn",
			Message:  fmt.Sprintf("missing asset referenced: %s", ref),
			FilePath: ref,
			Details:  details,
		})
	}

	errorsCount, warningsCount := countBySeverity(findings)
	status := "ok"
	if errorsCount > 0 {
		status = "error"
	} else if warningsCount > 0 {
		status = "warn"
	}

	if state.AppendLog != nil {
		state.AppendLog(fmt.Sprintf("Audit completed: %d findings (errors=%d, warnings=%d)", len(findings), errorsCount, warningsCount))
	}

	if state.AuditStore != nil {
		if err := state.AuditStore.UpsertRules(ctx, defaultAuditRules()); err != nil && state.AppendLog != nil {
			state.AppendLog(fmt.Sprintf("Audit rules upsert failed: %v", err))
		}
		if err := state.AuditStore.ReplaceFindings(ctx, state.GenerationID, toStoreFindings(state, findings)); err != nil && state.AppendLog != nil {
			state.AppendLog(fmt.Sprintf("Audit findings save failed: %v", err))
		}
	}

	report := map[string]any{
		"status": status,
		"summary": map[string]int{
			"total":    len(findings),
			"errors":   errorsCount,
			"warnings": warningsCount,
		},
		"findings": findings,
	}

	return map[string]any{
		"audit_report":    report,
		"audit_status":    status,
		"audit_has_issues": len(findings) > 0,
	}, nil
}

type auditFinding struct {
	RuleCode string          `json:"rule_code"`
	Severity string          `json:"severity"`
	Message  string          `json:"message"`
	FilePath string          `json:"file_path,omitempty"`
	Details  json.RawMessage `json:"details,omitempty"`
}

func defaultAuditRules() []sqlstore.AuditRule {
	return []sqlstore.AuditRule{
		{
			Code:        "missing_required_file",
			Title:       "Missing required file",
			Description: "Обязательный файл отсутствует в сборке",
			Severity:    "error",
			IsActive:    true,
		},
		{
			Code:        "missing_asset",
			Title:       "Missing asset reference",
			Description: "В HTML/CSS есть ссылка на файл, которого нет в сборке",
			Severity:    "warn",
			IsActive:    true,
		},
	}
}

func toStoreFindings(state *PipelineState, findings []auditFinding) []sqlstore.AuditFinding {
	result := make([]sqlstore.AuditFinding, 0, len(findings))
	for _, f := range findings {
		filePath := sql.NullString{}
		if strings.TrimSpace(f.FilePath) != "" {
			filePath = sqlstore.NullableString(f.FilePath)
		}
		result = append(result, sqlstore.AuditFinding{
			ID:           uuid.NewString(),
			GenerationID: state.GenerationID,
			DomainID:     state.DomainID,
			RuleCode:     f.RuleCode,
			Severity:     f.Severity,
			FilePath:     filePath,
			Message:      f.Message,
			Details:      f.Details,
		})
	}
	return result
}

func countBySeverity(findings []auditFinding) (errors int, warnings int) {
	for _, f := range findings {
		switch strings.ToLower(f.Severity) {
		case "error":
			errors++
		case "warn", "warning":
			warnings++
		}
	}
	return
}

func parseGeneratedFiles(value any) []GeneratedFile {
	switch v := value.(type) {
	case []GeneratedFile:
		return v
	case []any:
		out := make([]GeneratedFile, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				out = append(out, mapToGeneratedFile(m))
			}
		}
		return out
	case []map[string]any:
		out := make([]GeneratedFile, 0, len(v))
		for _, m := range v {
			out = append(out, mapToGeneratedFile(m))
		}
		return out
	default:
		return nil
	}
}

func mapToGeneratedFile(m map[string]any) GeneratedFile {
	var f GeneratedFile
	if v, ok := m["path"].(string); ok {
		f.Path = v
	}
	if v, ok := m["content"].(string); ok {
		f.Content = v
	}
	if v, ok := m["content_base64"].(string); ok {
		f.ContentBase64 = v
	}
	return f
}

func normalizePath(p string) string {
	clean := strings.TrimSpace(p)
	clean = strings.TrimPrefix(clean, "/")
	clean = strings.TrimPrefix(clean, "./")
	if clean == "" {
		return ""
	}
	clean = path.Clean(clean)
	if clean == "." || clean == ".." {
		return ""
	}
	return clean
}

func fileTextContent(f GeneratedFile) string {
	if strings.TrimSpace(f.Content) != "" {
		return f.Content
	}
	if strings.TrimSpace(f.ContentBase64) == "" {
		return ""
	}
	data, err := base64.StdEncoding.DecodeString(f.ContentBase64)
	if err != nil {
		return ""
	}
	return string(data)
}

func extractRefs(filePath, content, ext string) []string {
	refs := make([]string, 0)
	if ext == ".css" {
		refs = append(refs, extractCSSURLs(content)...)
		return refs
	}
	refs = append(refs, extractHTMLRefs(content)...)
	refs = append(refs, extractCSSURLs(content)...)
	return refs
}

var (
	htmlAttrRe = regexp.MustCompile(`(?i)\b(?:src|href)=["']([^"']+)["']`)
	srcsetRe   = regexp.MustCompile(`(?i)\bsrcset=["']([^"']+)["']`)
	cssURLRe   = regexp.MustCompile(`url\(([^)]+)\)`)
)

func extractHTMLRefs(content string) []string {
	found := make([]string, 0)
	for _, match := range htmlAttrRe.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}
		if ref, ok := normalizeRef(match[1]); ok {
			found = append(found, ref)
		}
	}
	for _, match := range srcsetRe.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}
		parts := strings.Split(match[1], ",")
		for _, part := range parts {
			fields := strings.Fields(strings.TrimSpace(part))
			if len(fields) == 0 {
				continue
			}
			if ref, ok := normalizeRef(fields[0]); ok {
				found = append(found, ref)
			}
		}
	}
	return found
}

func extractCSSURLs(content string) []string {
	found := make([]string, 0)
	for _, match := range cssURLRe.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}
		raw := strings.TrimSpace(match[1])
		raw = strings.Trim(raw, `"'`)
		if ref, ok := normalizeRef(raw); ok {
			found = append(found, ref)
		}
	}
	return found
}

func normalizeRef(ref string) (string, bool) {
	clean := strings.TrimSpace(ref)
	if clean == "" {
		return "", false
	}
	lower := strings.ToLower(clean)
	if strings.HasPrefix(lower, "#") ||
		strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "//") ||
		strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "javascript:") {
		return "", false
	}

	clean = strings.SplitN(clean, "?", 2)[0]
	clean = strings.SplitN(clean, "#", 2)[0]
	clean = strings.TrimPrefix(clean, "./")
	if clean == "/" || clean == "" {
		return "", false
	}
	clean = strings.TrimPrefix(clean, "/")
	clean = path.Clean(clean)
	if clean == "." || clean == ".." {
		return "", false
	}
	return clean, true
}
