package pipeline

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// AuditFixStep applies bounded, allowlisted autofix for audit findings.
// It runs between AuditStep and PublishStep.
//
// Modes:
//   - "disabled"        — no-op, pipeline proceeds as before
//   - "report_only"     — analyses what could be fixed, but changes nothing
//   - "autofix_soft"    — fixes allowlisted issues; publishes even if some remain
//   - "autofix_strict"  — fixes allowlisted issues; blocks publish if blocking issues remain
type AuditFixStep struct{}

func (s *AuditFixStep) Name() string       { return StepAuditFix }
func (s *AuditFixStep) ArtifactKey() string { return "audit_fix_result" }
func (s *AuditFixStep) Progress() int       { return 98 }

// auditFixResult is stored as the audit_fix_result artifact.
type auditFixResult struct {
	Mode              string           `json:"mode"`
	Applied           bool             `json:"applied"`
	FixedFiles        []string         `json:"fixed_files,omitempty"`
	SkippedFindings   int              `json:"skipped_findings"`
	ReportBeforeFix   AuditReport      `json:"report_before_fix"`
	ReportAfterFix    AuditReport      `json:"report_after_fix"`
	BlockingRemaining int              `json:"blocking_remaining"`
	FixOutcomes       []fixOutcomeItem `json:"fix_outcomes,omitempty"`
}

// fixOutcomeItem records how each finding was handled.
type fixOutcomeItem struct {
	RuleCode string `json:"rule_code"`
	FilePath string `json:"file_path"`
	FixMode  string `json:"fix_mode"`
	Outcome  string `json:"outcome"`
}

func (s *AuditFixStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	mode := strings.TrimSpace(strings.ToLower(state.AuditFixMode))
	if mode == "" {
		mode = "disabled"
	}

	genType := strings.TrimSpace(strings.ToLower(state.GenerationType))

	// Read current files and audit report from prior AuditStep
	files := parseGeneratedFiles(state.Artifacts["generated_files"])
	if len(files) == 0 {
		return nil, fmt.Errorf("audit_fix: generated_files is empty")
	}

	// Re-run audit on current state (source of truth)
	reportBefore := runAudit(files)

	if mode == "disabled" {
		if state.AppendLog != nil {
			state.AppendLog("AuditFixStep: mode=disabled, skipping")
		}
		result := auditFixResult{
			Mode:            mode,
			Applied:         false,
			ReportBeforeFix: reportBefore,
			ReportAfterFix:  reportBefore,
		}
		arts := s.buildArtifacts(result, reportBefore)
		arts["audit_fix_publish_blocked"] = false
		return arts, nil
	}

	// Identify autofixable findings from the allowlist
	fixable, skipped := classifyFindings(reportBefore.Findings)

	if len(fixable) == 0 {
		if state.AppendLog != nil {
			state.AppendLog(fmt.Sprintf("AuditFixStep: mode=%s, no autofixable findings", mode))
		}
		blockingRemaining := countBlocking(reportBefore.Findings)
		result := auditFixResult{
			Mode:              mode,
			Applied:           false,
			SkippedFindings:   skipped,
			ReportBeforeFix:   reportBefore,
			ReportAfterFix:    reportBefore,
			BlockingRemaining: blockingRemaining,
		}
		arts := s.buildArtifacts(result, reportBefore)
		if mode == "autofix_strict" && blockingRemaining > 0 {
			arts["audit_fix_publish_blocked"] = true
			if state.AppendLog != nil {
				state.AppendLog(fmt.Sprintf("AuditFixStep: autofix_strict — %d blocking issues remain (none fixable), publish will be blocked", blockingRemaining))
			}
		} else {
			arts["audit_fix_publish_blocked"] = false
		}
		return arts, nil
	}

	if mode == "report_only" {
		if state.AppendLog != nil {
			state.AppendLog(fmt.Sprintf("AuditFixStep: mode=report_only, %d fixable findings identified but not applied", len(fixable)))
		}
		result := auditFixResult{
			Mode:              mode,
			Applied:           false,
			SkippedFindings:   skipped + len(fixable),
			ReportBeforeFix:   reportBefore,
			ReportAfterFix:    reportBefore,
			BlockingRemaining: countBlocking(reportBefore.Findings),
		}
		arts := s.buildArtifacts(result, reportBefore)
		arts["audit_fix_publish_blocked"] = false
		return arts, nil
	}

	// autofix_soft or autofix_strict: attempt bounded fixes using policy
	fileMap := buildFileMap(files)
	var fixedFiles []string
	var outcomes []fixOutcomeItem

	for _, finding := range fixable {
		fixMode := policyForFinding(genType, finding)

		if fixMode == FixDisabled || fixMode == FixReportOnly {
			outcomes = append(outcomes, fixOutcomeItem{
				RuleCode: finding.RuleCode,
				FilePath: finding.FilePath,
				FixMode:  fixMode,
				Outcome:  fixOutcomeLabel(fixMode, false),
			})
			skipped++
			continue
		}

		modifiedFile, usedMode, err := s.applyFixWithPolicy(ctx, state, finding, fileMap, fixMode)
		if err != nil {
			if state.AppendLog != nil {
				state.AppendLog(fmt.Sprintf("AuditFixStep: fix failed for %s (%s, mode=%s): %v", finding.FilePath, finding.RuleCode, fixMode, err))
			}
			outcomes = append(outcomes, fixOutcomeItem{
				RuleCode: finding.RuleCode,
				FilePath: finding.FilePath,
				FixMode:  usedMode,
				Outcome:  fixOutcomeLabel(usedMode, false),
			})
			continue
		}
		if modifiedFile != "" {
			fixedFiles = append(fixedFiles, modifiedFile)
			outcomes = append(outcomes, fixOutcomeItem{
				RuleCode: finding.RuleCode,
				FilePath: finding.FilePath,
				FixMode:  usedMode,
				Outcome:  fixOutcomeLabel(usedMode, true),
			})
		}
	}

	// Rebuild files slice from the updated fileMap
	updatedFiles := fileMapToSlice(fileMap)

	// Re-run audit after fixes
	reportAfter := runAudit(updatedFiles)

	// Post-fix audit regression guard.
	// Two checks: (1) counts must not increase, (2) no NEW findings may appear
	// that were not present before (catches swapped findings at equal counts).
	if hasAuditRegression(reportBefore, reportAfter) {
		if state.AppendLog != nil {
			state.AppendLog(fmt.Sprintf("AuditFixStep: post-fix audit regression detected (errors %d→%d, warnings %d→%d, new findings=%v), reverting all fixes",
				reportBefore.Summary.Errors, reportAfter.Summary.Errors,
				reportBefore.Summary.Warnings, reportAfter.Summary.Warnings,
				hasNewFindings(reportBefore.Findings, reportAfter.Findings)))
		}
		// Revert: use original files
		updatedFiles = files
		reportAfter = reportBefore
		fixedFiles = nil
		outcomes = nil
	}

	if state.AppendLog != nil {
		state.AppendLog(fmt.Sprintf("AuditFixStep: mode=%s, fixed=%d files, findings before=%d after=%d",
			mode, len(fixedFiles), reportBefore.Summary.Total, reportAfter.Summary.Total))
	}

	applied := len(fixedFiles) > 0

	// Update AuditStore with post-fix findings so DB consumers see the final state
	if applied && state.AuditStore != nil {
		if err := state.AuditStore.ReplaceFindings(ctx, state.GenerationID, toStoreFindings(state, reportAfter.Findings)); err != nil {
			if state.AppendLog != nil {
				state.AppendLog(fmt.Sprintf("AuditFixStep: failed to update audit findings in store: %v", err))
			}
		}
	}

	// Rebuild artifacts if we actually fixed something
	artifacts := map[string]any{}
	if applied {
		// Update generated_files
		artifacts["generated_files"] = updatedFiles

		// Update final_html if index.html was changed
		if _, ok := fileMap["index.html"]; ok {
			for _, f := range updatedFiles {
				if normalizePath(f.Path) == "index.html" {
					artifacts["final_html"] = fileTextContent(f)
					break
				}
			}
		}

		// Update 404_html if 404.html was changed
		for _, f := range updatedFiles {
			if normalizePath(f.Path) == "404.html" {
				artifacts["404_html"] = fileTextContent(f)
				break
			}
		}

		// Rebuild zip_archive — critical, PublishStep uses this
		zipBytes, err := buildZip(updatedFiles)
		if err != nil {
			return nil, fmt.Errorf("audit_fix: rebuild zip: %w", err)
		}
		artifacts["zip_archive"] = base64.StdEncoding.EncodeToString(zipBytes)

		// Update audit artifacts to reflect post-fix state
		artifacts["audit_report"] = reportAfter
		artifacts["audit_status"] = reportAfter.Status
		artifacts["audit_has_issues"] = reportAfter.Summary.Total > 0
	}

	blockingRemaining := countBlocking(reportAfter.Findings)
	result := auditFixResult{
		Mode:              mode,
		Applied:           applied,
		FixedFiles:        fixedFiles,
		SkippedFindings:   skipped,
		ReportBeforeFix:   reportBefore,
		ReportAfterFix:    reportAfter,
		BlockingRemaining: blockingRemaining,
		FixOutcomes:       outcomes,
	}

	for k, v := range s.buildArtifacts(result, reportAfter) {
		artifacts[k] = v
	}

	// Always explicitly set publish blocked flag to prevent stale true from a previous run.
	// Must be set AFTER buildArtifacts merge.
	if mode == "autofix_strict" && blockingRemaining > 0 {
		artifacts["audit_fix_publish_blocked"] = true
		if state.AppendLog != nil {
			state.AppendLog(fmt.Sprintf("AuditFixStep: autofix_strict — %d blocking issues remain, publish will be blocked", blockingRemaining))
		}
	} else {
		artifacts["audit_fix_publish_blocked"] = false
	}

	return artifacts, nil
}

func (s *AuditFixStep) buildArtifacts(result auditFixResult, _ AuditReport) map[string]any {
	return map[string]any{
		"audit_fix_result":        result,
		"audit_fix_applied":       result.Applied,
		"audit_report_before_fix": result.ReportBeforeFix,
	}
}

// applyFixWithPolicy applies a fix using the policy-determined mode.
// It tries deterministic first when mode is FixLLMBounded, falling back to LLM.
// Returns (modifiedFilePath, actualModeUsed, error).
func (s *AuditFixStep) applyFixWithPolicy(ctx context.Context, state *PipelineState, finding auditFinding, fileMap map[string]GeneratedFile, fixMode string) (string, string, error) {
	switch finding.FixKind {
	case "missing_required_file":
		return s.fixMissingRequiredFileWithPolicy(ctx, state, finding, fileMap, fixMode)
	case "missing_asset_local_ref":
		return s.fixMissingAssetLocalRefWithPolicy(ctx, state, finding, fileMap, fixMode)
	default:
		return "", fixMode, nil
	}
}

// fixMissingRequiredFileWithPolicy generates a missing required file (e.g. 404.html).
func (s *AuditFixStep) fixMissingRequiredFileWithPolicy(ctx context.Context, state *PipelineState, finding auditFinding, fileMap map[string]GeneratedFile, fixMode string) (string, string, error) {
	filePath := normalizePath(finding.FilePath)
	if filePath == "" {
		return "", fixMode, nil
	}

	// Only handle known required files
	if filePath != "404.html" {
		return "", fixMode, nil
	}

	content := ""

	// Try LLM only if policy allows it
	if fixMode == FixLLMBounded && state.LLMClient != nil {
		domain := ""
		if state.Domain != nil {
			domain = strings.TrimSpace(state.Domain.URL)
		}
		prompt := fmt.Sprintf(`Generate a simple, clean 404 error page in HTML.
Domain: %s
Requirements:
- Complete HTML5 document
- Simple, clean design with inline CSS
- "Page not found" message
- Link back to homepage
- No external dependencies
Return ONLY the HTML content, nothing else.`, domain)

		model := state.DefaultModel
		resp, err := state.LLMClient.Generate(ctx, "audit_fix_404", prompt, model)
		if err != nil {
			if state.AppendLog != nil {
				state.AppendLog(fmt.Sprintf("AuditFixStep: LLM 404 generation failed, using template: %v", err))
			}
		} else {
			cleaned := extractHTMLFromResponse(resp)
			if strings.TrimSpace(cleaned) != "" && validateHTMLSkeleton(cleaned) {
				content = cleaned
			}
		}
	}

	// Always fall back to deterministic template
	if content == "" {
		content = `<!doctype html><html><head><meta charset="utf-8"><title>404 - Page Not Found</title><style>body{font-family:sans-serif;text-align:center;padding:50px}h1{font-size:2em}a{color:#0066cc}</style></head><body><h1>404</h1><p>Page not found</p><p><a href="/">Go to homepage</a></p></body></html>`
		fixMode = FixDeterministic
	}

	fileMap[filePath] = GeneratedFile{Path: filePath, Content: content}
	return filePath, fixMode, nil
}

// fixMissingAssetLocalRefWithPolicy fixes a missing local asset reference.
// Deterministic first (path normalization, basename matching, reference removal),
// then LLM as fallback if policy allows.
func (s *AuditFixStep) fixMissingAssetLocalRefWithPolicy(ctx context.Context, state *PipelineState, finding auditFinding, fileMap map[string]GeneratedFile, fixMode string) (string, string, error) {
	if len(finding.TargetFiles) == 0 {
		return "", fixMode, nil
	}
	if len(finding.TargetFiles) > 1 {
		return "", fixMode, nil
	}

	// Always try deterministic first
	modifiedFile, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err == nil && modifiedFile != "" {
		// Deterministic fix succeeded — validate HTML skeleton if applicable
		normalizedSource := normalizePath(finding.TargetFiles[0])
		if isHTMLFile(normalizedSource) {
			fixed := fileMap[normalizedSource]
			if !validateHTMLSkeleton(fileTextContent(fixed)) {
				// Revert: this shouldn't happen with deterministic but guard anyway
				return "", FixDeterministic, fmt.Errorf("deterministic fix broke HTML skeleton")
			}
		}
		return modifiedFile, FixDeterministic, nil
	}

	// Deterministic failed or was a no-op — try LLM if policy allows
	if fixMode != FixLLMBounded {
		return "", FixDeterministic, err
	}

	return s.fixMissingAssetLocalRefLLM(ctx, state, finding, fileMap)
}

// fixMissingAssetLocalRefLLM uses bounded LLM to fix a missing asset reference.
func (s *AuditFixStep) fixMissingAssetLocalRefLLM(ctx context.Context, state *PipelineState, finding auditFinding, fileMap map[string]GeneratedFile) (string, string, error) {
	sourceFile := finding.TargetFiles[0]
	src, ok := fileMap[normalizePath(sourceFile)]
	if !ok {
		return "", FixLLMBounded, nil
	}

	content := fileTextContent(src)
	if strings.TrimSpace(content) == "" {
		return "", FixLLMBounded, nil
	}

	if state.LLMClient == nil {
		return "", FixLLMBounded, nil
	}

	missingRef := finding.FilePath
	prompt := fmt.Sprintf(`Fix this file by removing or replacing the broken reference to "%s".
The referenced file does not exist. Remove the HTML/CSS reference to it, or replace it with an inline fallback.
Do NOT change anything else in the file.

File path: %s
File content:
%s

Return ONLY a JSON object with these fields:
{"path": "%s", "content": "<full fixed file content>", "summary": "<one-line description of fix>"}
Return ONLY the JSON, nothing else.`, missingRef, sourceFile, content, sourceFile)

	model := state.DefaultModel
	resp, err := state.LLMClient.Generate(ctx, "audit_fix_asset_ref", prompt, model)
	if err != nil {
		return "", FixLLMBounded, fmt.Errorf("LLM call failed: %w", err)
	}

	fixResult, err := parseFixResponse(resp)
	if err != nil {
		// Corrective retry
		retryPrompt := prompt + "\n\nIMPORTANT: Your response must be ONLY valid JSON. No markdown, no code fences, no explanation."
		resp2, err2 := state.LLMClient.Generate(ctx, "audit_fix_asset_ref_retry", retryPrompt, model)
		if err2 != nil {
			return "", FixLLMBounded, fmt.Errorf("corrective retry LLM failed: %w", err2)
		}
		fixResult, err = parseFixResponse(resp2)
		if err != nil {
			return "", FixLLMBounded, fmt.Errorf("corrective retry parse failed: %w", err)
		}
	}

	// Validate fix result
	expectedPath := normalizePath(sourceFile)
	if normalizePath(fixResult.Path) != expectedPath {
		return "", FixLLMBounded, fmt.Errorf("fix path mismatch: got %q, want %q", fixResult.Path, expectedPath)
	}
	if strings.TrimSpace(fixResult.Content) == "" {
		return "", FixLLMBounded, fmt.Errorf("fix returned empty content")
	}

	// Post-fix validation: broken reference must be gone
	if strings.Contains(fixResult.Content, missingRef) {
		return "", FixLLMBounded, fmt.Errorf("fix still contains broken reference %q", missingRef)
	}

	// Content size guard: reject if content shrank by more than 15%
	originalLen := len(content)
	fixedLen := len(fixResult.Content)
	if originalLen > 0 {
		shrinkRatio := float64(originalLen-fixedLen) / float64(originalLen)
		if shrinkRatio > 0.15 {
			return "", FixLLMBounded, fmt.Errorf("fix shrank content by %.0f%% (from %d to %d bytes), rejecting as too destructive",
				shrinkRatio*100, originalLen, fixedLen)
		}
	}

	// HTML skeleton guard
	if isHTMLFile(expectedPath) && !validateHTMLSkeleton(fixResult.Content) {
		return "", FixLLMBounded, fmt.Errorf("fix broke HTML skeleton (missing <html>, <head>, or <body> tags)")
	}

	fileMap[expectedPath] = GeneratedFile{Path: sourceFile, Content: fixResult.Content}
	return sourceFile, FixLLMBounded, nil
}

// validateHTMLSkeleton checks that an HTML file still contains essential structural tags.
func validateHTMLSkeleton(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "<html") &&
		strings.Contains(lower, "<head") &&
		strings.Contains(lower, "<body")
}

// isHTMLFile returns true if the file path ends with .html or .htm.
func isHTMLFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm")
}

// fixResponse represents the expected JSON output from the LLM fixer.
type fixResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Summary string `json:"summary"`
}

// parseFixResponse extracts a fixResponse from LLM output.
func parseFixResponse(raw string) (fixResponse, error) {
	trimmed := strings.TrimSpace(raw)

	// Strip markdown code fences if present
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.SplitN(trimmed, "\n", 2)
		if len(lines) == 2 {
			trimmed = lines[1]
		}
		if idx := strings.LastIndex(trimmed, "```"); idx > 0 {
			trimmed = trimmed[:idx]
		}
		trimmed = strings.TrimSpace(trimmed)
	}

	var result fixResponse
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		return fixResponse{}, fmt.Errorf("invalid JSON: %w", err)
	}
	return result, nil
}

// extractHTMLFromResponse extracts HTML from LLM response, stripping code fences if needed.
func extractHTMLFromResponse(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.SplitN(trimmed, "\n", 2)
		if len(lines) == 2 {
			trimmed = lines[1]
		}
		if idx := strings.LastIndex(trimmed, "```"); idx > 0 {
			trimmed = trimmed[:idx]
		}
		trimmed = strings.TrimSpace(trimmed)
	}
	return trimmed
}

// classifyFindings splits findings into autofixable (allowlisted) and skipped counts.
func classifyFindings(findings []auditFinding) (fixable []auditFinding, skipped int) {
	allowlist := map[string]bool{
		"missing_required_file":   true,
		"missing_asset_local_ref": true,
	}
	for _, f := range findings {
		if f.Autofixable && allowlist[f.FixKind] {
			fixable = append(fixable, f)
		} else {
			skipped++
		}
	}
	return
}

// countBlocking counts findings that have the Blocking flag set.
func countBlocking(findings []auditFinding) int {
	n := 0
	for _, f := range findings {
		if f.Blocking {
			n++
		}
	}
	return n
}

// buildFileMap creates a normalized path → GeneratedFile map.
func buildFileMap(files []GeneratedFile) map[string]GeneratedFile {
	m := make(map[string]GeneratedFile, len(files))
	for _, f := range files {
		key := normalizePath(f.Path)
		if key != "" {
			m[key] = f
		}
	}
	return m
}

// fileMapToSlice converts a file map back to a slice.
func fileMapToSlice(m map[string]GeneratedFile) []GeneratedFile {
	out := make([]GeneratedFile, 0, len(m))
	for _, f := range m {
		out = append(out, f)
	}
	return out
}

// hasAuditRegression returns true if the post-fix audit is worse than pre-fix.
// Checks both aggregate counts AND whether any new findings appeared.
func hasAuditRegression(before, after AuditReport) bool {
	// Count-based check
	if after.Summary.Errors > before.Summary.Errors ||
		after.Summary.Warnings > before.Summary.Warnings {
		return true
	}
	// Fingerprint-based check: no NEW findings should appear
	return hasNewFindings(before.Findings, after.Findings)
}

// hasNewFindings returns true if afterFindings contains any finding
// that was not present in beforeFindings. A finding is identified by
// its (RuleCode, FilePath, sorted TargetFiles) tuple — see findingFingerprint.
func hasNewFindings(before, after []auditFinding) bool {
	beforeSet := make(map[string]bool, len(before))
	for _, f := range before {
		beforeSet[findingFingerprint(f)] = true
	}
	for _, f := range after {
		if !beforeSet[findingFingerprint(f)] {
			return true
		}
	}
	return false
}

// findingFingerprint returns a stable key for deduplicating findings.
// Includes RuleCode, FilePath, and sorted TargetFiles for missing_asset
// findings where the same FilePath can appear in different source contexts.
func findingFingerprint(f auditFinding) string {
	key := f.RuleCode + ":" + f.FilePath
	if len(f.TargetFiles) > 0 {
		sorted := make([]string, len(f.TargetFiles))
		copy(sorted, f.TargetFiles)
		sortStrings(sorted)
		key += "→" + strings.Join(sorted, ",")
	}
	return key
}

// sortStrings sorts a slice of strings in place (simple insertion sort,
// adequate for the tiny slices we deal with here).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
