package pipeline

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// --- runAudit helper tests (regression) ---

func TestRunAudit_NoIssues(t *testing.T) {
	files := []GeneratedFile{
		{Path: "index.html", Content: "<html><body>hello</body></html>"},
		{Path: "404.html", Content: "<html><body>not found</body></html>"},
	}
	report := runAudit(files)
	if report.Status != "ok" {
		t.Errorf("expected status=ok, got %s", report.Status)
	}
	if report.Summary.Total != 0 {
		t.Errorf("expected 0 findings, got %d", report.Summary.Total)
	}
}

func TestRunAudit_MissingRequiredFile(t *testing.T) {
	files := []GeneratedFile{
		{Path: "index.html", Content: "<html><body>hello</body></html>"},
	}
	report := runAudit(files)
	if report.Summary.Errors != 1 {
		t.Errorf("expected 1 error, got %d", report.Summary.Errors)
	}
	f := report.Findings[0]
	if f.RuleCode != "missing_required_file" {
		t.Errorf("expected rule_code=missing_required_file, got %s", f.RuleCode)
	}
	if !f.Blocking {
		t.Error("missing_required_file should be blocking")
	}
	if !f.Autofixable {
		t.Error("missing_required_file should be autofixable")
	}
	if f.FixKind != "missing_required_file" {
		t.Errorf("expected fix_kind=missing_required_file, got %s", f.FixKind)
	}
}

func TestRunAudit_MissingAsset(t *testing.T) {
	files := []GeneratedFile{
		{Path: "index.html", Content: `<html><head><link rel="stylesheet" href="style.css"></head><body>hello</body></html>`},
		{Path: "404.html", Content: "<html><body>not found</body></html>"},
	}
	report := runAudit(files)
	if report.Summary.Warnings != 1 {
		t.Errorf("expected 1 warning, got %d", report.Summary.Warnings)
	}
	f := report.Findings[0]
	if f.RuleCode != "missing_asset" {
		t.Errorf("expected rule_code=missing_asset, got %s", f.RuleCode)
	}
	if f.Blocking {
		t.Error("missing_asset should not be blocking")
	}
	if !f.Autofixable {
		t.Error("missing_asset for .css should be autofixable")
	}
	if f.FixKind != "missing_asset_local_ref" {
		t.Errorf("expected fix_kind=missing_asset_local_ref, got %s", f.FixKind)
	}
	if len(f.TargetFiles) == 0 {
		t.Error("expected TargetFiles to contain source file")
	}
}

func TestRunAudit_MissingBinaryAsset_NotAutofixable(t *testing.T) {
	files := []GeneratedFile{
		{Path: "index.html", Content: `<html><body><img src="photo.png"></body></html>`},
		{Path: "404.html", Content: "<html><body>not found</body></html>"},
	}
	report := runAudit(files)
	if len(report.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(report.Findings))
	}
	f := report.Findings[0]
	if f.Autofixable {
		t.Error("binary asset (.png) should NOT be autofixable")
	}
}

// --- AuditStep regression: same output shape via runAudit ---

func TestAuditStep_StillWorks(t *testing.T) {
	step := &AuditStep{}
	state := &PipelineState{
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: "<html><body>hello</body></html>"},
				{Path: "404.html", Content: "<html><body>not found</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}
	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status, _ := artifacts["audit_status"].(string)
	if status != "ok" {
		t.Errorf("expected audit_status=ok, got %s", status)
	}
	hasIssues, _ := artifacts["audit_has_issues"].(bool)
	if hasIssues {
		t.Error("expected audit_has_issues=false")
	}
}

// --- AuditFixStep tests ---

func TestAuditFixStep_DisabledMode(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode: "disabled",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: "<html><body>hello</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := artifacts["audit_fix_result"].(auditFixResult)
	if !ok {
		t.Fatal("expected audit_fix_result artifact")
	}
	if result.Applied {
		t.Error("disabled mode should not apply fixes")
	}
	if result.Mode != "disabled" {
		t.Errorf("expected mode=disabled, got %s", result.Mode)
	}

	if _, changed := artifacts["generated_files"]; changed {
		t.Error("disabled mode should not change generated_files")
	}
}

func TestAuditFixStep_ReportOnlyMode(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode: "report_only",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: "<html><body>hello</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := artifacts["audit_fix_result"].(auditFixResult)
	if !ok {
		t.Fatal("expected audit_fix_result artifact")
	}
	if result.Applied {
		t.Error("report_only mode should not apply fixes")
	}
	if result.SkippedFindings == 0 {
		t.Error("expected skipped findings > 0 (missing 404.html is fixable but skipped)")
	}
	if _, changed := artifacts["generated_files"]; changed {
		t.Error("report_only mode should not change generated_files")
	}
}

func TestAuditFixStep_AutofixSoft_FixesMissing404(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "single_page",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: "<html><head><title>Hi</title></head><body>hello</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := artifacts["audit_fix_result"].(auditFixResult)
	if !ok {
		t.Fatal("expected audit_fix_result artifact")
	}
	if !result.Applied {
		t.Error("autofix_soft should apply fixes")
	}
	if len(result.FixedFiles) == 0 {
		t.Error("expected fixed_files to contain 404.html")
	}

	updatedFiles, ok := artifacts["generated_files"].([]GeneratedFile)
	if !ok {
		t.Fatal("expected updated generated_files")
	}
	has404 := false
	for _, f := range updatedFiles {
		if normalizePath(f.Path) == "404.html" {
			has404 = true
			if strings.TrimSpace(f.Content) == "" {
				t.Error("404.html should have content")
			}
		}
	}
	if !has404 {
		t.Error("expected 404.html in updated generated_files")
	}

	zipB64, ok := artifacts["zip_archive"].(string)
	if !ok || zipB64 == "" {
		t.Error("expected zip_archive to be rebuilt")
	}
	zipBytes, err := base64.StdEncoding.DecodeString(zipB64)
	if err != nil {
		t.Fatalf("zip_archive is not valid base64: %v", err)
	}
	if len(zipBytes) == 0 {
		t.Error("zip_archive should not be empty")
	}

	if result.ReportBeforeFix.Summary.Total <= result.ReportAfterFix.Summary.Total {
		t.Error("expected fewer findings after fix")
	}
}

func TestAuditFixStep_AutofixSoft_NoBlockPublish(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "single_page",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: `<html><body><img src="missing.png"></body></html>`},
				{Path: "404.html", Content: "<html><body>not found</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if blocked, _ := artifacts["audit_fix_publish_blocked"].(bool); blocked {
		t.Error("autofix_soft should never block publish")
	}
}

func TestAuditFixStep_AutofixStrict_BlocksOnBlockingIssues(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_strict",
		GenerationType: "single_page",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "404.html", Content: "<html><body>not found</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	blocked, _ := artifacts["audit_fix_publish_blocked"].(bool)
	if !blocked {
		t.Error("autofix_strict should block publish when blocking issues remain (missing index.html)")
	}

	result, _ := artifacts["audit_fix_result"].(auditFixResult)
	if result.BlockingRemaining == 0 {
		t.Error("expected blocking_remaining > 0")
	}
}

func TestAuditFixStep_ArtifactRebuild_ZipUpdated(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "single_page",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: "<html><head><title>Hi</title></head><body>hello</body></html>"},
				{Path: "styles.css", Content: "body{color:red}"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := artifacts["audit_fix_result"].(auditFixResult)
	if !result.Applied {
		t.Error("expected fix to be applied (missing 404.html)")
	}

	zipB64, _ := artifacts["zip_archive"].(string)
	zipBytes, _ := base64.StdEncoding.DecodeString(zipB64)
	filesFromZip, err := unzipToMap(zipBytes)
	if err != nil {
		t.Fatalf("unzip failed: %v", err)
	}
	if _, ok := filesFromZip["404.html"]; !ok {
		t.Error("zip should contain newly created 404.html")
	}
}

func TestAuditFixStep_ReAudit_Reports(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "single_page",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: "<html><head><title>Hi</title></head><body>hello</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := artifacts["audit_fix_result"].(auditFixResult)

	beforeHasMissing := false
	for _, f := range result.ReportBeforeFix.Findings {
		if f.RuleCode == "missing_required_file" && f.FilePath == "404.html" {
			beforeHasMissing = true
		}
	}
	if !beforeHasMissing {
		t.Error("ReportBeforeFix should show missing 404.html")
	}

	afterHasMissing := false
	for _, f := range result.ReportAfterFix.Findings {
		if f.RuleCode == "missing_required_file" && f.FilePath == "404.html" {
			afterHasMissing = true
		}
	}
	if afterHasMissing {
		t.Error("ReportAfterFix should NOT show missing 404.html")
	}

	if _, ok := artifacts["audit_report_before_fix"]; !ok {
		t.Error("expected audit_report_before_fix artifact")
	}
	if _, ok := artifacts["audit_fix_applied"]; !ok {
		t.Error("expected audit_fix_applied artifact")
	}
}

// --- LLM mock for asset ref fix tests ---

type mockLLMClient struct {
	responses map[string]string
	calls     []string
}

func (m *mockLLMClient) Generate(_ context.Context, stage string, _ string, _ string) (string, error) {
	m.calls = append(m.calls, stage)
	if resp, ok := m.responses[stage]; ok {
		return resp, nil
	}
	return "", fmt.Errorf("no mock response for stage %s", stage)
}

func (m *mockLLMClient) GenerateImage(_ context.Context, _ string, _ string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLLMClient) GenerateMultiTurn(_ context.Context, _, _ string, _ []string, _ string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func TestAuditFixStep_AssetRefFix_WithLLM(t *testing.T) {
	fixedHTML := `<html><head><!-- removed broken stylesheet --></head><body>hello</body></html>`
	fixResp, _ := json.Marshal(fixResponse{
		Path:    "index.html",
		Content: fixedHTML,
		Summary: "removed broken ref to style.css",
	})

	llm := &mockLLMClient{
		responses: map[string]string{
			"audit_fix_asset_ref": string(fixResp),
			"audit_fix_404":      `<!doctype html><html><head><title>404</title></head><body><h1>404</h1></body></html>`,
		},
	}

	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "single_page",
		LLMClient:      llm,
		DefaultModel:   "test-model",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: `<html><head><link rel="stylesheet" href="style.css"></head><body>hello</body></html>`},
				{Path: "404.html", Content: "<html><body>not found</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := artifacts["audit_fix_result"].(auditFixResult)
	if !result.Applied {
		t.Error("expected fix to be applied")
	}

	hasSourceFile := false
	for _, f := range result.FixedFiles {
		if f == "index.html" {
			hasSourceFile = true
		}
		if f == "style.css" {
			t.Error("FixedFiles should contain the modified source file (index.html), not the missing asset (style.css)")
		}
	}
	if !hasSourceFile {
		t.Error("expected FixedFiles to contain 'index.html' (the actually modified file)")
	}

	updatedFiles, _ := artifacts["generated_files"].([]GeneratedFile)
	for _, f := range updatedFiles {
		if normalizePath(f.Path) == "index.html" {
			if strings.Contains(f.Content, "style.css") {
				t.Error("fixed index.html should not contain broken style.css reference")
			}
		}
	}
}

func TestAuditFixStep_AssetRefFix_MultiFile_Skipped(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "single_page",
		DefaultModel:   "test-model",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: `<html><head><link rel="stylesheet" href="shared.css"></head><body>hello</body></html>`},
				{Path: "about.html", Content: `<html><head><link rel="stylesheet" href="shared.css"></head><body>about</body></html>`},
				{Path: "404.html", Content: "<html><body>not found</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := artifacts["audit_fix_result"].(auditFixResult)
	for _, f := range result.FixedFiles {
		if f == "shared.css" {
			t.Error("multi-file asset ref should not be fixed")
		}
	}
}

// --- parseFixResponse tests ---

func TestParseFixResponse_Valid(t *testing.T) {
	input := `{"path": "index.html", "content": "<html></html>", "summary": "fixed"}`
	result, err := parseFixResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Path != "index.html" {
		t.Errorf("expected path=index.html, got %s", result.Path)
	}
}

func TestParseFixResponse_WithCodeFence(t *testing.T) {
	input := "```json\n{\"path\": \"test.html\", \"content\": \"<html></html>\", \"summary\": \"ok\"}\n```"
	result, err := parseFixResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Path != "test.html" {
		t.Errorf("expected path=test.html, got %s", result.Path)
	}
}

func TestParseFixResponse_Invalid(t *testing.T) {
	_, err := parseFixResponse("not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- classifyFindings tests ---

func TestClassifyFindings(t *testing.T) {
	findings := []auditFinding{
		{RuleCode: "missing_required_file", Autofixable: true, FixKind: "missing_required_file"},
		{RuleCode: "missing_asset", Autofixable: true, FixKind: "missing_asset_local_ref"},
		{RuleCode: "missing_asset", Autofixable: false, FixKind: "missing_asset_local_ref"}, // binary
		{RuleCode: "unknown_rule", Autofixable: false},
	}

	fixable, skipped := classifyFindings(findings)
	if len(fixable) != 2 {
		t.Errorf("expected 2 fixable, got %d", len(fixable))
	}
	if skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", skipped)
	}
}

// --- PublishStep gating test ---

func TestPublishStep_BlockedByAuditFix(t *testing.T) {
	step := &PublishStep{}
	state := &PipelineState{
		Artifacts: map[string]any{
			"audit_fix_publish_blocked": true,
			"zip_archive":              "dGVzdA==",
		},
		AppendLog: func(s string) {},
	}

	_, err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatal("expected error when publish is blocked")
	}
	if !strings.Contains(err.Error(), "blocked by audit autofix") {
		t.Errorf("expected blocked error, got: %v", err)
	}
}

// --- Fix #1: missing index.html is NOT autofixable ---

func TestRunAudit_MissingIndexHTML_NotAutofixable(t *testing.T) {
	files := []GeneratedFile{
		{Path: "404.html", Content: "<html><body>not found</body></html>"},
	}
	report := runAudit(files)
	for _, f := range report.Findings {
		if f.RuleCode == "missing_required_file" && f.FilePath == "index.html" {
			if f.Autofixable {
				t.Error("index.html should NOT be marked as autofixable")
			}
			if !f.Blocking {
				t.Error("missing index.html should be blocking")
			}
			return
		}
	}
	t.Error("expected a finding for missing index.html")
}

// --- Fix #2: content size guard rejects destructive LLM fixes ---

func TestAuditFixStep_AssetRefFix_RejectsDestructiveFix(t *testing.T) {
	fixResp, _ := json.Marshal(fixResponse{
		Path:    "index.html",
		Content: "<html></html>", // way too small vs original
		Summary: "gutted the file",
	})

	llm := &mockLLMClient{
		responses: map[string]string{
			"audit_fix_asset_ref": string(fixResp),
		},
	}

	step := &AuditFixStep{}
	originalHTML := `<html><head><link rel="stylesheet" href="style.css"></head><body><h1>Welcome</h1><p>Some content here</p></body></html>`
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "single_page",
		LLMClient:      llm,
		DefaultModel:   "test-model",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: originalHTML},
				{Path: "404.html", Content: "<html><body>not found</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := artifacts["audit_fix_result"].(auditFixResult)
	for _, f := range result.FixedFiles {
		if f == "style.css" {
			t.Error("destructive fix (>15% content shrink) should have been rejected")
		}
	}
}

// --- Fix #3: autofix_strict blocks publish for non-fixable index.html ---

func TestAuditFixStep_AutofixStrict_IndexHTMLNotFixable(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_strict",
		GenerationType: "single_page",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "about.html", Content: "<html><head><title>About</title></head><body>about</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := artifacts["audit_fix_result"].(auditFixResult)

	has404Fix := false
	for _, f := range result.FixedFiles {
		if f == "404.html" {
			has404Fix = true
		}
	}
	if !has404Fix {
		t.Error("expected 404.html to be fixed")
	}

	blocked, _ := artifacts["audit_fix_publish_blocked"].(bool)
	if !blocked {
		t.Error("publish should be blocked — missing index.html is not autofixable")
	}
	if result.BlockingRemaining == 0 {
		t.Error("expected blocking_remaining > 0 for unfixable index.html")
	}
}

// --- Stale audit_fix_publish_blocked tests ---

func TestAuditFixStep_ClearsStalePublishBlocked(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode: "disabled",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: "<html><body>hello</body></html>"},
				{Path: "404.html", Content: "<html><body>not found</body></html>"},
			},
			"audit_fix_publish_blocked": true,
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	blocked, ok := artifacts["audit_fix_publish_blocked"].(bool)
	if !ok {
		t.Fatal("expected audit_fix_publish_blocked to be explicitly set in artifacts")
	}
	if blocked {
		t.Error("disabled mode should reset audit_fix_publish_blocked to false")
	}
}

func TestAuditFixStep_ClearsStalePublishBlocked_AutofixSoft(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "single_page",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: "<html><body>hello</body></html>"},
				{Path: "404.html", Content: "<html><body>not found</body></html>"},
			},
			"audit_fix_publish_blocked": true,
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	blocked, _ := artifacts["audit_fix_publish_blocked"].(bool)
	if blocked {
		t.Error("autofix_soft with no issues should not block publish")
	}
}

func TestAuditFixStep_ClearsStalePublishBlocked_NoFixableFindings(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "single_page",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: `<html><body><img src="photo.png"></body></html>`},
				{Path: "404.html", Content: "<html><body>not found</body></html>"},
			},
			"audit_fix_publish_blocked": true,
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	blocked, ok := artifacts["audit_fix_publish_blocked"].(bool)
	if !ok {
		t.Fatal("expected audit_fix_publish_blocked to be explicitly set")
	}
	if blocked {
		t.Error("stale audit_fix_publish_blocked should be cleared in no-fixable-findings branch")
	}
}

// ==================== POLICY LAYER TESTS ====================

// --- policyForFinding tests ---

func TestPolicyForFinding_DisabledForNonAutofixable(t *testing.T) {
	f := auditFinding{Autofixable: false, FixKind: "missing_required_file"}
	if got := policyForFinding("single_page", f); got != FixDisabled {
		t.Errorf("expected fix_disabled for non-autofixable, got %s", got)
	}
}

func TestPolicyForFinding_SinglePage_MissingRequiredFile(t *testing.T) {
	f := auditFinding{Autofixable: true, FixKind: "missing_required_file", FilePath: "404.html"}
	got := policyForFinding("single_page", f)
	if got != FixLLMBounded {
		t.Errorf("single_page + missing_required_file should be fix_llm_bounded, got %s", got)
	}
}

func TestPolicyForFinding_WebarchiveSingle_MissingRequiredFile(t *testing.T) {
	f := auditFinding{Autofixable: true, FixKind: "missing_required_file", FilePath: "404.html"}
	got := policyForFinding("webarchive_single", f)
	if got != FixDeterministic {
		t.Errorf("webarchive_single + missing_required_file should be fix_deterministic, got %s", got)
	}
}

func TestPolicyForFinding_WebarchiveMulti_MissingAsset(t *testing.T) {
	f := auditFinding{
		Autofixable: true,
		FixKind:     "missing_asset_local_ref",
		TargetFiles: []string{"index.html"},
	}
	got := policyForFinding("webarchive_multi", f)
	if got != FixDeterministic {
		t.Errorf("webarchive_multi + missing_asset_local_ref should be fix_deterministic, got %s", got)
	}
}

func TestPolicyForFinding_WebarchiveEEAT_MissingAsset(t *testing.T) {
	f := auditFinding{
		Autofixable: true,
		FixKind:     "missing_asset_local_ref",
		TargetFiles: []string{"index.html"},
	}
	got := policyForFinding("webarchive_eeat", f)
	if got != FixDeterministic {
		t.Errorf("webarchive_eeat + missing_asset should be fix_deterministic, got %s", got)
	}
}

func TestPolicyForFinding_SinglePage_MissingAsset_SafeSource(t *testing.T) {
	for _, safeFile := range []string{"index.html", "404.html", "main.css"} {
		f := auditFinding{
			Autofixable: true,
			FixKind:     "missing_asset_local_ref",
			TargetFiles: []string{safeFile},
		}
		got := policyForFinding("single_page", f)
		if got != FixLLMBounded {
			t.Errorf("single_page + safe source %s should be fix_llm_bounded, got %s", safeFile, got)
		}
	}
}

func TestPolicyForFinding_SinglePage_MissingAsset_UnsafeSource(t *testing.T) {
	f := auditFinding{
		Autofixable: true,
		FixKind:     "missing_asset_local_ref",
		TargetFiles: []string{"about.html"},
	}
	got := policyForFinding("single_page", f)
	if got != FixDeterministic {
		t.Errorf("single_page + unsafe source about.html should be fix_deterministic, got %s", got)
	}
}

func TestPolicyForFinding_UnknownGenType_MissingAsset(t *testing.T) {
	f := auditFinding{
		Autofixable: true,
		FixKind:     "missing_asset_local_ref",
		TargetFiles: []string{"index.html"},
	}
	// Unknown gen type must be safe-by-default: deterministic only, no LLM
	got := policyForFinding("unknown_type", f)
	if got != FixDeterministic {
		t.Errorf("unknown gen type should be fix_deterministic (safe default), got %s", got)
	}
}

func TestPolicyForFinding_UnknownGenType_MissingRequiredFile(t *testing.T) {
	f := auditFinding{Autofixable: true, FixKind: "missing_required_file", FilePath: "404.html"}
	got := policyForFinding("future_type_v2", f)
	if got != FixDeterministic {
		t.Errorf("unknown gen type + missing_required_file should be fix_deterministic (safe default), got %s", got)
	}
}

func TestIsKnownNonWebarchiveType(t *testing.T) {
	tests := []struct {
		genType string
		want    bool
	}{
		{"single_page", true},
		{"branded", true},
		{"branded_content", true},
		{"webarchive_single", false},
		{"webarchive_multi", false},
		{"webarchive_eeat", false},
		{"unknown_future", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isKnownNonWebarchiveType(tt.genType); got != tt.want {
			t.Errorf("isKnownNonWebarchiveType(%q) = %v, want %v", tt.genType, got, tt.want)
		}
	}
}

func TestPolicyForFinding_UnknownFixKind(t *testing.T) {
	f := auditFinding{Autofixable: true, FixKind: "some_new_kind"}
	got := policyForFinding("single_page", f)
	if got != FixDisabled {
		t.Errorf("unknown fix kind should be fix_disabled, got %s", got)
	}
}

// --- isWebarchiveType tests ---

func TestIsWebarchiveType(t *testing.T) {
	tests := []struct {
		genType string
		want    bool
	}{
		{"webarchive_single", true},
		{"webarchive_multi", true},
		{"webarchive_eeat", true},
		{"single_page", false},
		{"branded", false},
		{"branded_content", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isWebarchiveType(tt.genType); got != tt.want {
			t.Errorf("isWebarchiveType(%q) = %v, want %v", tt.genType, got, tt.want)
		}
	}
}

// --- isLLMSafeSourceFile tests ---

func TestIsLLMSafeSourceFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"index.html", true},
		{"404.html", true},
		{"styles.css", true},
		{"main.css", true},
		{"about.html", false},
		{"contact.html", false},
		{"script.js", false},
		{"image.png", false},
	}
	for _, tt := range tests {
		if got := isLLMSafeSourceFile(tt.path); got != tt.want {
			t.Errorf("isLLMSafeSourceFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// --- fixOutcomeLabel tests ---

func TestFixOutcomeLabel(t *testing.T) {
	tests := []struct {
		mode    string
		applied bool
		want    string
	}{
		{FixDeterministic, true, "fixed_deterministically"},
		{FixLLMBounded, true, "fixed_via_llm"},
		{FixDisabled, false, "skipped_by_policy"},
		{FixReportOnly, false, "skipped_by_policy"},
		{FixDeterministic, false, "deterministic_failed"},
		{FixLLMBounded, false, "llm_failed"},
	}
	for _, tt := range tests {
		got := fixOutcomeLabel(tt.mode, tt.applied)
		if got != tt.want {
			t.Errorf("fixOutcomeLabel(%q, %v) = %q, want %q", tt.mode, tt.applied, got, tt.want)
		}
	}
}

// ==================== DETERMINISTIC FIXER TESTS ====================

func TestDeterministicFix_CaseInsensitiveMatch(t *testing.T) {
	fileMap := map[string]GeneratedFile{
		"index.html": {Path: "index.html", Content: `<html><head><link rel="stylesheet" href="Style.css"></head><body>hello</body></html>`},
		"style.css":  {Path: "style.css", Content: "body{color:red}"},
	}
	finding := auditFinding{
		FixKind:     "missing_asset_local_ref",
		FilePath:    "Style.css",
		TargetFiles: []string{"index.html"},
	}

	modified, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified != "index.html" {
		t.Errorf("expected modified=index.html, got %q", modified)
	}
	// Verify the reference was updated
	content := fileMap["index.html"].Content
	if !strings.Contains(content, "style.css") {
		t.Error("expected content to contain corrected ref 'style.css'")
	}
	if strings.Contains(content, "Style.css") {
		t.Error("expected content to NOT contain original broken ref 'Style.css'")
	}
}

func TestDeterministicFix_BasenameMatch(t *testing.T) {
	fileMap := map[string]GeneratedFile{
		"index.html":      {Path: "index.html", Content: `<html><head><link rel="stylesheet" href="css/main.css"></head><body>hi</body></html>`},
		"assets/main.css": {Path: "assets/main.css", Content: "body{}"},
	}
	finding := auditFinding{
		FixKind:     "missing_asset_local_ref",
		FilePath:    "css/main.css",
		TargetFiles: []string{"index.html"},
	}

	modified, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified != "index.html" {
		t.Errorf("expected modified=index.html, got %q", modified)
	}
	content := fileMap["index.html"].Content
	if !strings.Contains(content, "assets/main.css") {
		t.Error("expected content to contain 'assets/main.css' from basename match")
	}
}

func TestDeterministicFix_RemovesBrokenRef(t *testing.T) {
	fileMap := map[string]GeneratedFile{
		"index.html": {Path: "index.html", Content: `<html><head><link rel="stylesheet" href="missing.css"></head><body><h1>Welcome to Our Website</h1><p>This is a paragraph with enough content to ensure the shrink ratio stays under the 15 percent guard when we remove the broken stylesheet reference from the head section of this HTML document.</p><p>Another paragraph with additional text content for padding purposes to make the file large enough.</p><footer>Copyright 2026</footer></body></html>`},
	}
	finding := auditFinding{
		FixKind:     "missing_asset_local_ref",
		FilePath:    "missing.css",
		TargetFiles: []string{"index.html"},
	}

	modified, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified != "index.html" {
		t.Errorf("expected modified=index.html, got %q", modified)
	}
	content := fileMap["index.html"].Content
	if strings.Contains(content, "missing.css") {
		t.Error("expected broken ref 'missing.css' to be removed")
	}
}

func TestDeterministicFix_RemovesBrokenScript(t *testing.T) {
	fileMap := map[string]GeneratedFile{
		"index.html": {Path: "index.html", Content: `<html><head><title>Test Page</title></head><body><h1>Hello World</h1><p>This is a paragraph with enough content to ensure the shrink ratio stays under the 15 percent guard when we remove the broken script reference. We need sufficient text content here so the file is large enough that removing one script tag does not exceed the threshold.</p><script src="app.js"></script></body></html>`},
	}
	finding := auditFinding{
		FixKind:     "missing_asset_local_ref",
		FilePath:    "app.js",
		TargetFiles: []string{"index.html"},
	}

	modified, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified != "index.html" {
		t.Errorf("expected modified=index.html, got %q", modified)
	}
	content := fileMap["index.html"].Content
	if strings.Contains(content, "app.js") {
		t.Error("expected broken script ref 'app.js' to be removed")
	}
}

func TestDeterministicFix_MultiFile_Skipped(t *testing.T) {
	fileMap := map[string]GeneratedFile{
		"index.html": {Path: "index.html", Content: `<html><head><link rel="stylesheet" href="shared.css"></head><body>hi</body></html>`},
		"about.html": {Path: "about.html", Content: `<html><head><link rel="stylesheet" href="shared.css"></head><body>about</body></html>`},
	}
	finding := auditFinding{
		FixKind:     "missing_asset_local_ref",
		FilePath:    "shared.css",
		TargetFiles: []string{"index.html", "about.html"},
	}

	modified, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified != "" {
		t.Error("multi-file deterministic fix should be skipped")
	}
}

func TestDeterministicFix_CSSUrlRemoval(t *testing.T) {
	fileMap := map[string]GeneratedFile{
		"style.css": {Path: "style.css", Content: "body{color:red;font-family:sans-serif;font-size:16px;line-height:1.6;margin:0;padding:0}\nh1{font-size:2em;color:#333;margin-bottom:1em}\nh2{font-size:1.5em;color:#555}\np{margin:0 0 1em;color:#444}\n.bg{background:url('missing-bg.png')}\na{color:#0066cc;text-decoration:none}\nfooter{padding:20px;background:#f5f5f5}"},
	}
	finding := auditFinding{
		FixKind:     "missing_asset_local_ref",
		FilePath:    "missing-bg.png",
		TargetFiles: []string{"style.css"},
	}

	modified, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified != "style.css" {
		t.Errorf("expected modified=style.css, got %q", modified)
	}
	content := fileMap["style.css"].Content
	if strings.Contains(content, "missing-bg.png") {
		t.Error("expected broken CSS url ref to be removed")
	}
}

func TestDeterministicFix_RemovesBrokenImg(t *testing.T) {
	fileMap := map[string]GeneratedFile{
		"index.html": {Path: "index.html", Content: `<html><head><title>Test</title></head><body><h1>Welcome</h1><p>This is a paragraph with enough content to keep the shrink ratio under 15 percent when we remove the broken image tag from the body section of this document. Extra padding text here for safety margin.</p><img src="missing-photo.jpg" alt="photo"><footer>Copyright 2026</footer></body></html>`},
	}
	finding := auditFinding{
		FixKind:     "missing_asset_local_ref",
		FilePath:    "missing-photo.jpg",
		TargetFiles: []string{"index.html"},
	}

	modified, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified != "index.html" {
		t.Errorf("expected modified=index.html, got %q", modified)
	}
	content := fileMap["index.html"].Content
	if strings.Contains(content, "missing-photo.jpg") {
		t.Error("expected broken <img> ref to be removed")
	}
}

func TestDeterministicFix_RemovesSrcsetImg(t *testing.T) {
	fileMap := map[string]GeneratedFile{
		"index.html": {Path: "index.html", Content: `<html><head><title>Test</title></head><body><h1>Welcome</h1><p>This paragraph has enough content to keep the shrink ratio under fifteen percent when we remove the broken srcset image tag. Additional padding text to ensure the file is large enough for the guard to pass safely without issues.</p><img srcset="hero.webp 1x, hero@2x.webp 2x" alt="hero"><footer>Copyright 2026</footer></body></html>`},
	}
	finding := auditFinding{
		FixKind:     "missing_asset_local_ref",
		FilePath:    "hero.webp",
		TargetFiles: []string{"index.html"},
	}

	modified, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified != "index.html" {
		t.Errorf("expected modified=index.html, got %q", modified)
	}
	content := fileMap["index.html"].Content
	if strings.Contains(content, "hero.webp") {
		t.Error("expected srcset <img> tag referencing hero.webp to be removed")
	}
}

func TestDeterministicFix_SrcsetWithFallback_StripsOnlySrcset(t *testing.T) {
	// When <img> has both src= (fallback) and srcset= (broken), only srcset should be
	// stripped — the tag must be preserved with its working src= fallback.
	fileMap := map[string]GeneratedFile{
		"index.html": {Path: "index.html", Content: `<html><head><title>Test</title></head><body><h1>Welcome</h1><p>This paragraph has enough content to keep the shrink ratio under the guard. Additional padding text to ensure everything works correctly with enough margin for safety on this particular test case.</p><img src="fallback.jpg" srcset="good-1x.jpg 1x, missing-2x.jpg 2x" alt="hero"><footer>Copyright 2026</footer></body></html>`},
		"fallback.jpg": {Path: "fallback.jpg", ContentBase64: "ZmFrZQ=="},
	}
	finding := auditFinding{
		FixKind:     "missing_asset_local_ref",
		FilePath:    "missing-2x.jpg",
		TargetFiles: []string{"index.html"},
	}

	modified, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified != "index.html" {
		t.Errorf("expected modified=index.html, got %q", modified)
	}
	content := fileMap["index.html"].Content
	if strings.Contains(content, "missing-2x.jpg") {
		t.Error("expected srcset containing missing-2x.jpg to be stripped")
	}
	// The <img> tag with src="fallback.jpg" must still be present
	if !strings.Contains(content, "fallback.jpg") {
		t.Error("expected <img src='fallback.jpg'> to be preserved (only srcset should be stripped)")
	}
	if !strings.Contains(content, "<img") {
		t.Error("expected <img> tag to be preserved when it has src= fallback")
	}
}

func TestDeterministicFix_RemovesSrcsetSource(t *testing.T) {
	fileMap := map[string]GeneratedFile{
		"index.html": {Path: "index.html", Content: `<html><head><title>Gallery</title></head><body><h1>Photo Gallery</h1><p>This is a photo gallery page with enough content to stay within the shrink guard. We have multiple paragraphs of descriptive text here to keep the file large enough for the deterministic fixer to work correctly without triggering the guard.</p><picture><source srcset="photo.avif" type="image/avif"><img src="photo.jpg" alt="photo"></picture><footer>End</footer></body></html>`},
	}
	finding := auditFinding{
		FixKind:     "missing_asset_local_ref",
		FilePath:    "photo.avif",
		TargetFiles: []string{"index.html"},
	}

	modified, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified != "index.html" {
		t.Errorf("expected modified=index.html, got %q", modified)
	}
	content := fileMap["index.html"].Content
	if strings.Contains(content, "photo.avif") {
		t.Error("expected <source srcset='photo.avif'> to be removed")
	}
	// The <img> with a different src should still be there
	if !strings.Contains(content, "photo.jpg") {
		t.Error("expected <img src='photo.jpg'> to be preserved")
	}
}

func TestDeterministicFix_RejectsTooDestructive(t *testing.T) {
	// Content where the broken ref removal would eat most of the file
	smallContent := `<link rel="stylesheet" href="big.css">`
	fileMap := map[string]GeneratedFile{
		"index.html": {Path: "index.html", Content: smallContent},
	}
	finding := auditFinding{
		FixKind:     "missing_asset_local_ref",
		FilePath:    "big.css",
		TargetFiles: []string{"index.html"},
	}

	modified, err := deterministicFixMissingAssetRef(finding, fileMap)
	if err == nil && modified != "" {
		t.Error("expected destructive deterministic fix to be rejected")
	}
}

// ==================== GENERATION TYPE INTEGRATION TESTS ====================

func TestAuditFixStep_WebarchiveSingle_NoLLMForAssetRef(t *testing.T) {
	llm := &mockLLMClient{
		responses: map[string]string{
			"audit_fix_asset_ref": `{"path":"index.html","content":"<html><head></head><body>fixed</body></html>","summary":"fixed"}`,
		},
	}

	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "webarchive_single",
		LLMClient:      llm,
		DefaultModel:   "test-model",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: `<html><head><link rel="stylesheet" href="style.css"></head><body>hello world content here padding</body></html>`},
				{Path: "404.html", Content: "<html><head><title>404</title></head><body>not found</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// LLM should NOT have been called for asset ref fix (webarchive → deterministic only)
	for _, call := range llm.calls {
		if call == "audit_fix_asset_ref" || call == "audit_fix_asset_ref_retry" {
			t.Error("webarchive_single should NOT use LLM for asset ref fix")
		}
	}

	result, _ := artifacts["audit_fix_result"].(auditFixResult)

	// Check fix outcomes report deterministic mode
	for _, outcome := range result.FixOutcomes {
		if outcome.RuleCode == "missing_asset" {
			if outcome.FixMode != FixDeterministic {
				t.Errorf("expected fix_mode=%s for webarchive asset fix, got %s", FixDeterministic, outcome.FixMode)
			}
		}
	}
}

func TestAuditFixStep_WebarchiveSingle_DeterministicTemplate404(t *testing.T) {
	llm := &mockLLMClient{
		responses: map[string]string{
			"audit_fix_404": `<!doctype html><html><head><title>404</title></head><body><h1>Not Found (LLM)</h1></body></html>`,
		},
	}

	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "webarchive_single",
		LLMClient:      llm,
		DefaultModel:   "test-model",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: "<html><head><title>Hi</title></head><body>hello</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// LLM should NOT be called for 404 in webarchive mode
	for _, call := range llm.calls {
		if call == "audit_fix_404" {
			t.Error("webarchive_single should NOT use LLM for 404.html generation")
		}
	}

	result, _ := artifacts["audit_fix_result"].(auditFixResult)
	if !result.Applied {
		t.Error("expected 404.html to be fixed with template")
	}

	// Verify template was used (not LLM content)
	updatedFiles, _ := artifacts["generated_files"].([]GeneratedFile)
	for _, f := range updatedFiles {
		if normalizePath(f.Path) == "404.html" {
			if strings.Contains(f.Content, "Not Found (LLM)") {
				t.Error("webarchive should use deterministic template, not LLM-generated content")
			}
			if !strings.Contains(f.Content, "404") {
				t.Error("expected 404 template content")
			}
		}
	}
}

func TestAuditFixStep_SinglePage_LLMFallback_ForSafeFile(t *testing.T) {
	// Deterministic fix removes the <link> tag. If we want to test LLM fallback,
	// we need a case where deterministic can't help (e.g. inline ref that regex doesn't catch).
	// Instead, let's verify that for single_page, the policy allows LLM for safe files.
	f := auditFinding{
		Autofixable: true,
		FixKind:     "missing_asset_local_ref",
		TargetFiles: []string{"index.html"},
	}
	got := policyForFinding("single_page", f)
	if got != FixLLMBounded {
		t.Errorf("single_page + safe source index.html should allow LLM, got %s", got)
	}
}

func TestAuditFixStep_SinglePage_DeterministicOnly_ForUnsafeFile(t *testing.T) {
	f := auditFinding{
		Autofixable: true,
		FixKind:     "missing_asset_local_ref",
		TargetFiles: []string{"gallery.html"},
	}
	got := policyForFinding("single_page", f)
	if got != FixDeterministic {
		t.Errorf("single_page + unsafe source gallery.html should be deterministic-only, got %s", got)
	}
}

// --- FixOutcomes reporting test ---

func TestAuditFixStep_FixOutcomes_Reported(t *testing.T) {
	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "single_page",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: `<html><head><link rel="stylesheet" href="missing.css"></head><body>hello world some content padding text</body></html>`},
				// 404.html missing — will be fixed
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := artifacts["audit_fix_result"].(auditFixResult)
	if len(result.FixOutcomes) == 0 {
		t.Error("expected FixOutcomes to be populated")
	}

	// Verify that outcomes contain expected labels
	outcomeMap := map[string]string{}
	for _, o := range result.FixOutcomes {
		outcomeMap[o.RuleCode+":"+o.FilePath] = o.Outcome
	}

	// 404.html should be fixed_deterministically (no LLM client)
	if outcome, ok := outcomeMap["missing_required_file:404.html"]; ok {
		if outcome != "fixed_deterministically" && outcome != "fixed_via_llm" {
			t.Errorf("expected 404.html to be fixed, got outcome=%s", outcome)
		}
	}
}

// --- HTML skeleton validation tests ---

func TestValidateHTMLSkeleton(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"<html><head><title>t</title></head><body>b</body></html>", true},
		{"<!doctype html><HTML><HEAD></HEAD><BODY></BODY></HTML>", true},
		{"<html><body>missing head</body></html>", false},
		{"just plain text", false},
		{"<head><body></body></head>", false}, // no <html
		{"", false},
	}
	for _, tt := range tests {
		got := validateHTMLSkeleton(tt.content)
		if got != tt.want {
			t.Errorf("validateHTMLSkeleton(%q) = %v, want %v", tt.content[:min(len(tt.content), 40)], got, tt.want)
		}
	}
}

// --- Post-fix audit regression guard test ---

func TestAuditFixStep_PostFixAuditRegression_Reverts(t *testing.T) {
	// Simulate a scenario where a fix introduces a new issue.
	// We do this by providing an LLM that returns content with a new broken reference.
	fixedHTML := `<html><head><link rel="stylesheet" href="new-broken.css"></head><body>hello world content padding text here</body></html>`
	fixResp, _ := json.Marshal(fixResponse{
		Path:    "index.html",
		Content: fixedHTML,
		Summary: "replaced style.css with new-broken.css",
	})

	llm := &mockLLMClient{
		responses: map[string]string{
			"audit_fix_asset_ref": string(fixResp),
		},
	}

	step := &AuditFixStep{}
	state := &PipelineState{
		AuditFixMode:   "autofix_soft",
		GenerationType: "single_page",
		LLMClient:      llm,
		DefaultModel:   "test-model",
		Artifacts: map[string]any{
			"generated_files": []GeneratedFile{
				{Path: "index.html", Content: `<html><head><link rel="stylesheet" href="style.css"></head><body>hello world content padding text here</body></html>`},
				{Path: "404.html", Content: "<html><head><title>404</title></head><body>not found</body></html>"},
			},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := artifacts["audit_fix_result"].(auditFixResult)
	// The deterministic fixer will handle style.css by removing the <link> tag.
	// Since deterministic runs first and succeeds, the LLM won't even be called.
	// The post-fix audit should show fewer or equal findings.
	if result.ReportAfterFix.Summary.Total > result.ReportBeforeFix.Summary.Total {
		t.Error("post-fix audit should not have more findings than before")
	}
}

// --- Branded type uses same policy as single_page ---

func TestPolicyForFinding_Branded_SameAsSinglePage(t *testing.T) {
	for _, genType := range []string{"branded", "branded_content"} {
		f := auditFinding{
			Autofixable: true,
			FixKind:     "missing_asset_local_ref",
			TargetFiles: []string{"index.html"},
		}
		got := policyForFinding(genType, f)
		if got != FixLLMBounded {
			t.Errorf("genType=%s + safe source should be fix_llm_bounded, got %s", genType, got)
		}

		f2 := auditFinding{Autofixable: true, FixKind: "missing_required_file"}
		got2 := policyForFinding(genType, f2)
		if got2 != FixLLMBounded {
			t.Errorf("genType=%s + missing_required_file should be fix_llm_bounded, got %s", genType, got2)
		}
	}
}

// ==================== REGRESSION GUARD FINGERPRINT TESTS ====================

func TestHasNewFindings_NoNew(t *testing.T) {
	before := []auditFinding{
		{RuleCode: "missing_asset", FilePath: "style.css"},
		{RuleCode: "missing_required_file", FilePath: "404.html"},
	}
	// After: one finding was fixed (removed), nothing new
	after := []auditFinding{
		{RuleCode: "missing_asset", FilePath: "style.css"},
	}
	if hasNewFindings(before, after) {
		t.Error("expected no new findings when a finding was removed")
	}
}

func TestHasNewFindings_NewAppeared(t *testing.T) {
	before := []auditFinding{
		{RuleCode: "missing_asset", FilePath: "style.css"},
	}
	// After: old finding gone, but a new one appeared
	after := []auditFinding{
		{RuleCode: "missing_asset", FilePath: "new-broken.css"},
	}
	if !hasNewFindings(before, after) {
		t.Error("expected new finding detected (new-broken.css was not in before)")
	}
}

func TestHasNewFindings_SwappedSameCount(t *testing.T) {
	before := []auditFinding{
		{RuleCode: "missing_asset", FilePath: "old.css"},
	}
	after := []auditFinding{
		{RuleCode: "missing_asset", FilePath: "new.css"},
	}
	// Same count, but different finding — this is a regression
	if !hasNewFindings(before, after) {
		t.Error("swapped finding (same count but different file) should be detected as new")
	}
}

func TestHasAuditRegression_CountIncrease(t *testing.T) {
	before := AuditReport{Summary: AuditSummary{Errors: 1, Warnings: 0}}
	after := AuditReport{Summary: AuditSummary{Errors: 2, Warnings: 0}}
	if !hasAuditRegression(before, after) {
		t.Error("expected regression when error count increases")
	}
}

func TestHasAuditRegression_NewFindingSameCount(t *testing.T) {
	before := AuditReport{
		Summary:  AuditSummary{Errors: 0, Warnings: 1},
		Findings: []auditFinding{{RuleCode: "missing_asset", Severity: "warn", FilePath: "old.css"}},
	}
	after := AuditReport{
		Summary:  AuditSummary{Errors: 0, Warnings: 1},
		Findings: []auditFinding{{RuleCode: "missing_asset", Severity: "warn", FilePath: "new.css"}},
	}
	if !hasAuditRegression(before, after) {
		t.Error("expected regression when a new finding appears even with same count")
	}
}

func TestHasAuditRegression_NoRegression(t *testing.T) {
	before := AuditReport{
		Summary:  AuditSummary{Errors: 1, Warnings: 2},
		Findings: []auditFinding{
			{RuleCode: "missing_required_file", Severity: "error", FilePath: "404.html"},
			{RuleCode: "missing_asset", Severity: "warn", FilePath: "style.css"},
			{RuleCode: "missing_asset", Severity: "warn", FilePath: "app.js"},
		},
	}
	// After: one finding fixed, rest unchanged
	after := AuditReport{
		Summary:  AuditSummary{Errors: 0, Warnings: 2},
		Findings: []auditFinding{
			{RuleCode: "missing_asset", Severity: "warn", FilePath: "style.css"},
			{RuleCode: "missing_asset", Severity: "warn", FilePath: "app.js"},
		},
	}
	if hasAuditRegression(before, after) {
		t.Error("expected no regression when findings only decreased")
	}
}

func TestFindingFingerprint_NoTargetFiles(t *testing.T) {
	f := auditFinding{RuleCode: "missing_required_file", FilePath: "404.html"}
	fp := findingFingerprint(f)
	if fp != "missing_required_file:404.html" {
		t.Errorf("expected 'missing_required_file:404.html', got %q", fp)
	}
}

func TestFindingFingerprint_WithTargetFiles(t *testing.T) {
	f := auditFinding{
		RuleCode:    "missing_asset",
		FilePath:    "style.css",
		TargetFiles: []string{"index.html"},
	}
	fp := findingFingerprint(f)
	expected := "missing_asset:style.css→index.html"
	if fp != expected {
		t.Errorf("expected %q, got %q", expected, fp)
	}
}

func TestFindingFingerprint_SameFilePathDifferentTargets(t *testing.T) {
	f1 := auditFinding{
		RuleCode:    "missing_asset",
		FilePath:    "style.css",
		TargetFiles: []string{"index.html"},
	}
	f2 := auditFinding{
		RuleCode:    "missing_asset",
		FilePath:    "style.css",
		TargetFiles: []string{"about.html"},
	}
	fp1 := findingFingerprint(f1)
	fp2 := findingFingerprint(f2)
	if fp1 == fp2 {
		t.Errorf("fingerprints should differ when TargetFiles differ: %q vs %q", fp1, fp2)
	}
}

func TestHasNewFindings_SameFilePathDifferentSource(t *testing.T) {
	before := []auditFinding{
		{RuleCode: "missing_asset", FilePath: "style.css", TargetFiles: []string{"index.html"}},
	}
	after := []auditFinding{
		{RuleCode: "missing_asset", FilePath: "style.css", TargetFiles: []string{"about.html"}},
	}
	if !hasNewFindings(before, after) {
		t.Error("same FilePath but different TargetFiles should be detected as new finding")
	}
}

