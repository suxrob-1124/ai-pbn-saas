package pipeline

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// AssemblyStep финальная сборка статики в zip
// Не использует LLM, только работает с артефактами
// Требует: html_raw, css_content, js_content, generated_files
// Результат: final_html, zip_archive, обновлённый generated_files (добавляет служебные файлы и website.zip)
type AssemblyStep struct{}

func (s *AssemblyStep) Name() string { return StepAssembly }

func (s *AssemblyStep) ArtifactKey() string { return "zip_archive" }

func (s *AssemblyStep) Progress() int { return 94 }

func (s *AssemblyStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	// 1. Собираем исходники
	htmlRaw, _ := state.Artifacts["html_raw"].(string)
	cssContent, _ := state.Artifacts["css_content"].(string)
	jsContent, _ := state.Artifacts["js_content"].(string)
	files := mergeGeneratedFiles(state.Artifacts["generated_files"], nil)
	page404, _ := state.Artifacts["404_html"].(string)

	if strings.TrimSpace(htmlRaw) == "" {
		return nil, fmt.Errorf("assembly: html_raw is empty")
	}

	// 2. Инлайн CSS/JS прямо в HTML (избегаем проблем с кешем отдельных файлов)
	finalHTML := injectAssets(htmlRaw, cssContent, jsContent)

	// 3. Служебные файлы
	domain := ""
	if state.Domain != nil {
		domain = strings.TrimSpace(state.Domain.URL)
	}
	if dCtx, ok := state.Context["domain"].(string); domain == "" && ok {
		domain = dCtx
	}
	domain = normalizeDomain(domain)
	today := time.Now().Format("2006-01-02")

	robots := fmt.Sprintf("User-agent: *\nAllow: /\nSitemap: https://%s/sitemap.xml\n", domain)
	sitemap := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://%s/</loc>
    <lastmod>%s</lastmod>
    <changefreq>weekly</changefreq>
  </url>
</urlset>
`, domain, today)

	wwwRedirectRule := "# No domain provided for redirect\n"
	if domain != "" {
		escaped := strings.ReplaceAll(domain, ".", "\\.")
		if strings.HasPrefix(domain, "www.") {
			wwwRedirectRule = fmt.Sprintf("RewriteCond %%{HTTP_HOST} !^%s$ [NC]\nRewriteRule ^ https://%s%%{REQUEST_URI} [R=301,L]\n", escaped, domain)
		} else {
			wwwRedirectRule = fmt.Sprintf("RewriteCond %%{HTTP_HOST} ^www\\.%s$ [NC]\nRewriteRule ^ https://%s%%{REQUEST_URI} [R=301,L]\n", escaped, domain)
		}
	}

	htaccess := fmt.Sprintf(`ErrorDocument 404 /404.html
RewriteEngine on
RewriteBase /

# Force WWW/Non-WWW (Dynamic logic based on domain input)
%s

# Trailing Slash Redirect
RewriteCond %%{REQUEST_URI} !/$
RewriteCond %%{REQUEST_URI} !\.[a-zA-Z0-9]{2,5}$
RewriteRule .* https://%%{HTTP_HOST}%%{REQUEST_URI}/ [R=301,L]

# SPA Fallback (если нужно, но для статики это не обязательно, если файлы реальные)
# RewriteCond %%{REQUEST_FILENAME} !-f
# RewriteRule ^ index.html [QSA,L]
`, wwwRedirectRule)

	// 4. Собираем финальные файлы
	has404 := false
	for _, f := range files {
		if strings.EqualFold(strings.TrimSpace(f.Path), "404.html") {
			has404 = true
			break
		}
	}

	extra := []GeneratedFile{
		{Path: "index.html", Content: finalHTML},
		{Path: "robots.txt", Content: robots},
		{Path: "sitemap.xml", Content: sitemap},
		{Path: ".htaccess", Content: htaccess},
	}
	if !has404 {
		content404 := page404
		if strings.TrimSpace(content404) == "" {
			content404 = "<!doctype html><html><head><title>404</title></head><body><h1>Page not found</h1><a href=\"/\">Home</a></body></html>"
		}
		content404 = inlineAssetsIn404(content404, cssContent, jsContent)
		extra = append(extra, GeneratedFile{Path: "404.html", Content: content404})
	}

	// Инлайним CSS/JS в уже существующие 404.html из generated_files
	for i, f := range files {
		if strings.EqualFold(strings.TrimSpace(f.Path), "404.html") {
			files[i].Content = inlineAssetsIn404(f.Content, cssContent, jsContent)
		}
	}

	// Удаляем отдельные style.css/script.js из generated_files (теперь инлайн)
	filtered := make([]GeneratedFile, 0, len(files))
	for _, f := range files {
		p := strings.TrimSpace(f.Path)
		if p == "style.css" || p == "script.js" {
			continue
		}
		filtered = append(filtered, f)
	}
	files = filtered
	files = mergeGeneratedFiles(files, extra)

	// 5. Строим ZIP
	zipBytes, err := buildZip(files)
	if err != nil {
		return nil, fmt.Errorf("failed to build zip: %w", err)
	}
	zipB64 := base64.StdEncoding.EncodeToString(zipBytes)

	// Добавим архив как файл для удобства скачивания
	files = mergeGeneratedFiles(files, []GeneratedFile{{Path: "website.zip", ContentBase64: zipB64}})

	artifacts := map[string]any{
		"final_html":      finalHTML,
		"generated_files": files,
		"zip_archive":     zipB64,
	}

	return artifacts, nil
}

// injectAssets вставляет CSS/JS инлайн в html (без отдельных файлов — решает проблему с кешем)
func injectAssets(html, css, js string) string {
	h := html
	if css != "" && strings.Contains(strings.ToLower(h), "</head>") {
		tag := "  <style>\n" + css + "\n  </style>\n"
		h = strings.Replace(h, "</head>", tag+"</head>", 1)
	}
	if js != "" && strings.Contains(strings.ToLower(h), "</body>") {
		tag := "  <script>\n" + js + "\n  </script>\n"
		h = strings.Replace(h, "</body>", tag+"</body>", 1)
	}
	return h
}

// inlineAssetsIn404 заменяет внешние ссылки на style.css/script.js на инлайн в 404 странице
func inlineAssetsIn404(html404, css, js string) string {
	h := html404
	if css != "" {
		h = strings.Replace(h, `<link rel="stylesheet" href="style.css">`, "<style>\n"+css+"\n</style>", 1)
	}
	if js != "" {
		h = strings.Replace(h, `<script src="script.js"></script>`, "<script>\n"+js+"\n</script>", 1)
		h = strings.Replace(h, `<script src="script.js" defer></script>`, "<script>\n"+js+"\n</script>", 1)
	}
	return h
}

// normalizeDomain убирает схему и слеши
func normalizeDomain(d string) string {
	d = strings.TrimSpace(d)
	d = strings.TrimPrefix(d, "http://")
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimSuffix(d, "/")
	return d
}
