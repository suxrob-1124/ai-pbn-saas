package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

type scopedPromptManager struct {
	promptStore    *sqlstore.PromptStore
	promptOverride *sqlstore.PromptOverrideStore
	domainID       string
	projectID      string
	mu             sync.Mutex
	traceByStage   map[string]map[string]any
}

func newScopedPromptManager(
	promptStore *sqlstore.PromptStore,
	promptOverrideStore *sqlstore.PromptOverrideStore,
	domainID, projectID string,
) *scopedPromptManager {
	return &scopedPromptManager{
		promptStore:    promptStore,
		promptOverride: promptOverrideStore,
		domainID:       strings.TrimSpace(domainID),
		projectID:      strings.TrimSpace(projectID),
		traceByStage:   make(map[string]map[string]any),
	}
}

func (m *scopedPromptManager) GetPromptByStage(ctx context.Context, stage string) (string, string, string, error) {
	stage = strings.TrimSpace(stage)
	if stage == "" {
		return "", "", "", fmt.Errorf("stage is required")
	}

	if m.promptOverride != nil {
		resolved, err := m.promptOverride.ResolveForDomainStage(ctx, m.domainID, m.projectID, stage)
		if err == nil {
			model := ""
			if resolved.Model.Valid {
				model = strings.TrimSpace(resolved.Model.String)
			}
			promptID := ""
			if resolved.PromptID.Valid {
				promptID = strings.TrimSpace(resolved.PromptID.String)
			}
			overrideID := ""
			if resolved.OverrideID.Valid {
				overrideID = strings.TrimSpace(resolved.OverrideID.String)
				if promptID == "" {
					promptID = overrideID
				}
			}
			if promptID == "" {
				promptID = stage
			}
			m.recordTrace(stage, resolved.Source, nullableString(resolved.PromptID), nullableString(resolved.OverrideID), model)
			return promptID, resolved.Body, model, nil
		}
	}

	if m.promptStore == nil {
		return "", "", "", fmt.Errorf("prompt store is not configured")
	}

	prompt, err := m.promptStore.GetByStage(ctx, stage)
	if err != nil {
		return "", "", "", err
	}
	model := ""
	if prompt.Model.Valid {
		model = strings.TrimSpace(prompt.Model.String)
	}
	promptID := strings.TrimSpace(prompt.ID)
	if promptID == "" {
		promptID = stage
	}
	m.recordTrace(stage, "global", promptID, "", model)
	return promptID, prompt.Body, model, nil
}

func (m *scopedPromptManager) Trace() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	trace := make(map[string]any, len(m.traceByStage))
	for stage, item := range m.traceByStage {
		copied := make(map[string]any, len(item))
		for k, v := range item {
			copied[k] = v
		}
		trace[stage] = copied
	}
	return trace
}

func (m *scopedPromptManager) recordTrace(stage, source, promptID, overrideID, model string) {
	traceItem := map[string]any{
		"resolved_source": source,
	}
	if strings.TrimSpace(promptID) != "" {
		traceItem["resolved_prompt_id"] = strings.TrimSpace(promptID)
	}
	if strings.TrimSpace(overrideID) != "" {
		traceItem["override_id"] = strings.TrimSpace(overrideID)
	}
	if strings.TrimSpace(model) != "" {
		traceItem["model"] = strings.TrimSpace(model)
	}

	m.mu.Lock()
	m.traceByStage[stage] = traceItem
	m.mu.Unlock()
}

func buildGenerationArtifactsSummary(artifacts map[string]any, promptTrace map[string]any, updatedAt time.Time) map[string]any {
	if artifacts == nil {
		artifacts = map[string]any{}
	}

	finalHTML := strings.TrimSpace(extractArtifactText(artifacts, "final_html"))
	if finalHTML == "" {
		finalHTML = strings.TrimSpace(extractArtifactText(artifacts, "html_raw"))
	}
	zipArchive := strings.TrimSpace(extractArtifactText(artifacts, "zip_archive"))

	fileCount, totalSizeBytes := extractGeneratedFilesStats(artifacts["generated_files"])
	if fc, ok := asInt(artifacts["file_count"]); ok {
		fileCount = fc
	}
	if ts, ok := asInt64(artifacts["total_size_bytes"]); ok {
		totalSizeBytes = ts
	}

	warningsCount, errorsCount := extractAuditCounters(artifacts["audit_report"])

	summary := map[string]any{
		"has_final_html":      finalHTML != "",
		"has_zip_archive":     zipArchive != "",
		"has_generated_files": fileCount > 0,
		"file_count":          fileCount,
		"total_size_bytes":    totalSizeBytes,
		"warnings_count":      warningsCount,
		"errors_count":        errorsCount,
		"updated_at":          updatedAt.UTC().Format(time.RFC3339),
	}
	if publishedPath := strings.TrimSpace(extractArtifactText(artifacts, "published_path")); publishedPath != "" {
		summary["published_path"] = publishedPath
	}
	if legacyMeta, ok := artifacts["legacy_decode_meta"]; ok && legacyMeta != nil {
		summary["legacy_decode_meta"] = legacyMeta
	}
	if brandMode, primaryBrand, brandSource, ok := extractBrandResolutionMeta(artifacts["brand_resolution"]); ok {
		summary["brand_mode"] = brandMode
		if primaryBrand != "" {
			summary["primary_brand"] = primaryBrand
		}
		if brandSource != "" {
			summary["brand_source"] = brandSource
		}
	}
	if validationOK, violationsCount, ok := extractBrandValidationMeta(artifacts["brand_validation"]); ok {
		summary["brand_validation_ok"] = validationOK
		summary["brand_violations_count"] = violationsCount
	}
	if len(promptTrace) > 0 {
		summary["prompt_trace"] = promptTrace
	}

	return summary
}

func extractBrandResolutionMeta(raw any) (brandMode string, primaryBrand string, brandSource string, ok bool) {
	resolution, ok := toMap(raw)
	if !ok {
		return "", "", "", false
	}
	brandMode = strings.TrimSpace(asString(resolution["mode"]))
	primaryBrand = strings.TrimSpace(asString(resolution["primary_brand"]))
	brandSource = strings.TrimSpace(asString(resolution["source"]))
	if brandMode == "" {
		return "", "", "", false
	}
	return brandMode, primaryBrand, brandSource, true
}

func extractBrandValidationMeta(raw any) (validationOK bool, violationsCount int, ok bool) {
	root, ok := toMap(raw)
	if !ok || len(root) == 0 {
		return false, 0, false
	}
	hasStage := false
	overallOK := true
	totalViolations := 0

	if _, hasOK := root["ok"]; hasOK {
		v, vok := asBool(root["ok"])
		if vok {
			hasStage = true
			overallOK = v
		}
		totalViolations += len(toAnySlice(root["violations"]))
	} else {
		for _, stageRaw := range root {
			stage, stageOK := toMap(stageRaw)
			if !stageOK {
				continue
			}
			v, vok := asBool(stage["ok"])
			if vok {
				hasStage = true
				if !v {
					overallOK = false
				}
			}
			totalViolations += len(toAnySlice(stage["violations"]))
		}
	}

	if !hasStage {
		return false, 0, false
	}
	return overallOK, totalViolations, true
}

func toMap(raw any) (map[string]any, bool) {
	switch v := raw.(type) {
	case map[string]any:
		return v, true
	default:
		if raw == nil {
			return nil, false
		}
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, false
		}
		out := make(map[string]any)
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, false
		}
		return out, true
	}
}

func toAnySlice(raw any) []any {
	switch v := raw.(type) {
	case []any:
		return v
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func stableDomainStatus(domain sqlstore.Domain) string {
	if (domain.PublishedAt.Valid || strings.TrimSpace(domain.PublishedPath.String) != "") && domain.FileCount > 0 {
		return "published"
	}
	return "waiting"
}

func nullableString(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return strings.TrimSpace(v.String)
}

func extractArtifactText(artifacts map[string]any, key string) string {
	if artifacts == nil {
		return ""
	}
	raw, ok := artifacts[key]
	if !ok || raw == nil {
		return ""
	}
	if value, ok := raw.(string); ok {
		return value
	}
	return fmt.Sprintf("%v", raw)
}

func extractGeneratedFilesStats(raw any) (int, int64) {
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return 0, 0
	}
	count := 0
	total := int64(0)
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		count++
		switch {
		case strings.TrimSpace(asString(entry["content"])) != "":
			total += int64(len(asString(entry["content"])))
		case strings.TrimSpace(asString(entry["content_base64"])) != "":
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(asString(entry["content_base64"])))
			if err == nil {
				total += int64(len(decoded))
			}
		case strings.TrimSpace(asString(entry["preview_content"])) != "":
			total += int64(len(asString(entry["preview_content"])))
		}
	}
	return count, total
}

func extractAuditCounters(raw any) (int, int) {
	report, ok := raw.(map[string]any)
	if !ok {
		return 0, 0
	}
	warningsCount := 0
	errorsCount := 0
	if warnings, ok := report["warnings"].([]any); ok {
		warningsCount = len(warnings)
	}
	if errorsList, ok := report["errors"].([]any); ok {
		errorsCount = len(errorsList)
	}
	return warningsCount, errorsCount
}

func asString(raw any) string {
	if raw == nil {
		return ""
	}
	if value, ok := raw.(string); ok {
		return value
	}
	return fmt.Sprintf("%v", raw)
}

func asInt(raw any) (int, bool) {
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0, false
		}
		var out int
		_, err := fmt.Sscanf(v, "%d", &out)
		return out, err == nil
	default:
		return 0, false
	}
}

func asInt64(raw any) (int64, bool) {
	switch v := raw.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0, false
		}
		var out int64
		_, err := fmt.Sscanf(v, "%d", &out)
		return out, err == nil
	default:
		return 0, false
	}
}

func asBool(raw any) (bool, bool) {
	switch v := raw.(type) {
	case bool:
		return v, true
	case string:
		v = strings.TrimSpace(strings.ToLower(v))
		switch v {
		case "true", "1", "yes":
			return true, true
		case "false", "0", "no":
			return false, true
		default:
			return false, false
		}
	default:
		return false, false
	}
}
