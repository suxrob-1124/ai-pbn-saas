package pipeline

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

func TestAssemblyStep(t *testing.T) {
	step := &AssemblyStep{}
	logs := make([]string, 0)

	state := &PipelineState{
		Artifacts: map[string]any{
			"html_raw":    "<html><head></head><body>Test</body></html>",
			"css_content": "body { color: red; }",
			"js_content":  "console.log('test');",
			"404_html":    "<html><body>404</body></html>",
			"generated_files": []GeneratedFile{
				{Path: "test.png", ContentBase64: "dGVzdA=="},
			},
		},
		Domain: &sqlstore.Domain{
			URL: "example.com",
		},
		AppendLog: func(s string) {
			logs = append(logs, s)
		},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	// Проверяем наличие финального HTML
	finalHTML, ok := artifacts["final_html"].(string)
	if !ok || finalHTML == "" {
		t.Fatalf("expected final_html, got %#v", artifacts["final_html"])
	}

	// Проверяем, что CSS и JS инлайн
	if !strings.Contains(finalHTML, "<style>") {
		t.Errorf("expected final_html to contain inline <style>")
	}
	if !strings.Contains(finalHTML, "body { color: red; }") {
		t.Errorf("expected final_html to contain CSS content inline")
	}
	if !strings.Contains(finalHTML, "<script>") {
		t.Errorf("expected final_html to contain inline <script>")
	}
	if !strings.Contains(finalHTML, "console.log('test');") {
		t.Errorf("expected final_html to contain JS content inline")
	}

	// Проверяем наличие zip архива
	zipArchive, ok := artifacts["zip_archive"].(string)
	if !ok || zipArchive == "" {
		t.Fatalf("expected zip_archive, got %#v", artifacts["zip_archive"])
	}

	// Проверяем, что архив валидный base64
	zipBytes, err := base64.StdEncoding.DecodeString(zipArchive)
	if err != nil {
		t.Fatalf("zip_archive is not valid base64: %v", err)
	}
	if len(zipBytes) == 0 {
		t.Fatalf("zip_archive is empty")
	}

	// Проверяем generated_files
	files, ok := artifacts["generated_files"].([]GeneratedFile)
	if !ok || len(files) == 0 {
		t.Fatalf("expected generated_files slice, got %#v", artifacts["generated_files"])
	}

	// Проверяем наличие обязательных файлов (style.css/script.js теперь инлайн — не должно быть отдельных файлов)
	requiredFiles := map[string]bool{
		"index.html":  false,
		"robots.txt":  false,
		"sitemap.xml": false,
		".htaccess":   false,
		"404.html":    false,
		"website.zip": false,
	}

	for _, f := range files {
		if _, exists := requiredFiles[f.Path]; exists {
			requiredFiles[f.Path] = true
		}
	}

	for file, found := range requiredFiles {
		if !found {
			t.Errorf("expected file %s in generated_files, but not found", file)
		}
	}
}

func TestAssemblyStep_EmptyHTML(t *testing.T) {
	step := &AssemblyStep{}
	state := &PipelineState{
		Artifacts: map[string]any{
			"html_raw":    "",
			"css_content": "body { color: red; }",
			"js_content":  "console.log('test');",
		},
		AppendLog: func(s string) {},
	}

	_, err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatalf("expected error for empty html_raw, got nil")
	}
	if !strings.Contains(err.Error(), "html_raw is empty") {
		t.Errorf("expected error about empty html_raw, got: %v", err)
	}
}

func TestAssemblyStep_No404InFiles(t *testing.T) {
	step := &AssemblyStep{}
	state := &PipelineState{
		Artifacts: map[string]any{
			"html_raw":        "<html><head></head><body>Test</body></html>",
			"css_content":     "body { color: red; }",
			"js_content":      "console.log('test');",
			"404_html":        "<html><body>404</body></html>",
			"generated_files": []GeneratedFile{},
		},
		AppendLog: func(s string) {},
	}

	artifacts, err := step.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	files, ok := artifacts["generated_files"].([]GeneratedFile)
	if !ok {
		t.Fatalf("expected generated_files slice")
	}

	has404 := false
	for _, f := range files {
		if f.Path == "404.html" {
			has404 = true
			break
		}
	}
	if !has404 {
		t.Errorf("expected 404.html to be added when not in generated_files")
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"https://example.com/", "example.com"},
		{"  example.com  ", "example.com"},
		{"www.example.com", "www.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeDomain(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeDomain(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestInjectAssets(t *testing.T) {
	css := "body { color: red; }"
	js := "console.log('ok');"

	tests := []struct {
		name   string
		input  string
		hasCSS bool
		hasJS  bool
	}{
		{
			name:   "html with head and body",
			input:  "<html><head></head><body></body></html>",
			hasCSS: true,
			hasJS:  true,
		},
		{
			name:   "html without head",
			input:  "<html><body></body></html>",
			hasCSS: false,
			hasJS:  true,
		},
		{
			name:   "html without body",
			input:  "<html><head></head></html>",
			hasCSS: true,
			hasJS:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injectAssets(tt.input, css, js)
			hasCSS := strings.Contains(result, "<style>")
			hasJS := strings.Contains(result, "<script>")

			if hasCSS != tt.hasCSS {
				t.Errorf("injectAssets: hasCSS = %v, want %v", hasCSS, tt.hasCSS)
			}
			if hasJS != tt.hasJS {
				t.Errorf("injectAssets: hasJS = %v, want %v", hasJS, tt.hasJS)
			}

			// Ensure no external file references
			if strings.Contains(result, "style.css") {
				t.Errorf("injectAssets should not contain external style.css reference")
			}
			if strings.Contains(result, `script src=`) {
				t.Errorf("injectAssets should not contain external script.js reference")
			}
		})
	}
}
