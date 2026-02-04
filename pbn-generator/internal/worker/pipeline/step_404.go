package pipeline

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"obzornik-pbn-generator/internal/llm"
)

// Page404GenerationStep генерирует полноценный 404.html в стиле сайта
// Требует: design_system, html_raw, css_content, js_content
// Артефакты: 404_html, generated_files (добавляет 404.html)
type Page404GenerationStep struct{}

func (s *Page404GenerationStep) Name() string { return Step404Page }

func (s *Page404GenerationStep) ArtifactKey() string { return "404_html" }

func (s *Page404GenerationStep) Progress() int { return 97 }

func (s *Page404GenerationStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало генерации 404 страницы")

	designSystem, _ := state.Artifacts["design_system"].(string)
	htmlRaw, _ := state.Artifacts["html_raw"].(string)
	cssContent, _ := state.Artifacts["css_content"].(string)
	jsContent, _ := state.Artifacts["js_content"].(string)

	if strings.TrimSpace(designSystem) == "" || strings.TrimSpace(htmlRaw) == "" || strings.TrimSpace(cssContent) == "" || strings.TrimSpace(jsContent) == "" {
		return nil, fmt.Errorf("design_system, html_raw, css_content и js_content обязательны для 404 страницы")
	}
	lang := ""
	if state.Domain != nil {
		lang = state.Domain.TargetLanguage
	}
	if strings.TrimSpace(lang) == "" {
		lang = "sv"
	}

	promptID, promptBody, promptModel, err := state.PromptManager.GetPromptByStage(ctx, Step404Page)
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt for 404_page: %w", err)
	}
	if promptID != "" {
		state.AppendLog(fmt.Sprintf("Используется промпт: %s для этапа 404_page", promptID))
	}
	if promptModel != "" {
		state.AppendLog(fmt.Sprintf("Используется модель LLM: %s", promptModel))
	}

	variables := map[string]string{
		"design_system": designSystem,
		"html_raw":      htmlRaw,
		"css_content":   cssContent,
		"js_content":    jsContent,
		"language":      lang,
	}
	prompt := llm.BuildPrompt(promptBody, "", variables)
	preview := prompt
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	state.AppendLog(fmt.Sprintf("Промпт для 404 (первые 500 символов): %s", preview))

	model := promptModel
	if model == "" {
		model = state.DefaultModel
	}

	raw, err := state.LLMClient.Generate(ctx, Step404Page, prompt, model)
	if err != nil {
		return nil, fmt.Errorf("404 generation failed: %w", err)
	}

	clean := normalizeLLMOutput(raw)
	if doc := extractHTMLDocument(clean); doc != "" {
		clean = doc
	}
	if !looksLikeHTMLDocument(clean) {
		if looksLikeHTMLFragment(clean) {
			state.AppendLog("Ответ 404 содержит HTML-фрагменты, применяется обертка документа")
			clean = wrapHTMLFragment(clean)
		} else {
			state.AppendLog("Ответ 404 не похож на HTML, применяется markdown-постобработка")
			clean = wrapMarkdown404(clean)
		}
	}

	if strings.TrimSpace(clean) == "" {
		state.AppendLog("Ответ 404 не похож на HTML, применяется markdown-постобработка")
		clean = wrapMarkdown404("Страница не найдена")
	}

	files := mergeGeneratedFiles(state.Artifacts["generated_files"], []GeneratedFile{
		{Path: "404.html", Content: clean},
	})

	return map[string]any{
		"404_page_prompt": formatPromptForArtifact(prompt),
		"404_html":        clean,
		"generated_files": files,
	}, nil
}

func normalizeLLMOutput(raw string) string {
	clean := strings.TrimSpace(raw)
	if strings.HasPrefix(clean, "```") {
		lines := strings.Split(clean, "\n")
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
			lines = lines[1:]
		}
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
			lines = lines[:len(lines)-1]
		}
		clean = strings.Join(lines, "\n")
	}
	clean = strings.TrimSpace(clean)
	return clean
}

func extractHTMLDocument(content string) string {
	re := regexp.MustCompile(`(?is)<html[^>]*>.*?</html>`)
	match := re.FindString(content)
	return strings.TrimSpace(match)
}

func looksLikeHTMLDocument(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "<html") && strings.Contains(lower, "</html>") && strings.Contains(lower, "<body")
}

func looksLikeHTMLFragment(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "<div") ||
		strings.Contains(lower, "<section") ||
		strings.Contains(lower, "<main") ||
		strings.Contains(lower, "<p") ||
		strings.Contains(lower, "<h1") ||
		strings.Contains(lower, "<h2") ||
		strings.Contains(lower, "<ul") ||
		strings.Contains(lower, "<ol") ||
		strings.Contains(lower, "<body")
}

func wrapMarkdown404(markdown string) string {
	body := markdownToHTML(markdown)
	if strings.TrimSpace(body) == "" {
		body = "<h1>Страница не найдена</h1><p>Попробуйте вернуться на главную.</p>"
	}
	return `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Страница не найдена</title>
  <link rel="stylesheet" href="style.css">
</head>
<body>
  <main class="page-404">
    ` + body + `
  </main>
  <script src="script.js"></script>
</body>
</html>`
}

var (
	linkRe   = regexp.MustCompile(`\[(.+?)\]\((.+?)\)`)
	boldRe   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	italicRe = regexp.MustCompile(`\*(.+?)\*`)
	codeRe   = regexp.MustCompile("`([^`]+)`")
	tableSep = regexp.MustCompile(`^\s*\|?(\s*:?-{3,}:?\s*\|)+\s*:?-{3,}:?\s*\|?\s*$`)
)

func markdownToHTML(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var out strings.Builder
	inList := false

	closeList := func() {
		if inList {
			out.WriteString("</ul>")
			inList = false
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			closeList()
			continue
		}
		if isTableStart(lines, i) {
			closeList()
			tableHTML, consumed := renderTable(lines[i:])
			out.WriteString(tableHTML)
			i += consumed - 1
			continue
		}
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			closeList()
			out.WriteString("<hr>")
			continue
		}
		if strings.HasPrefix(trimmed, "<") {
			closeList()
			out.WriteString(trimmed)
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			closeList()
			level := countHeadingLevel(trimmed)
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, strings.Repeat("#", level)))
			out.WriteString(fmt.Sprintf("<h%d>%s</h%d>", level, inlineMarkdown(text), level))
			continue
		}
		if strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "- ") {
			if !inList {
				out.WriteString("<ul>")
				inList = true
			}
			text := strings.TrimSpace(trimmed[2:])
			out.WriteString(fmt.Sprintf("<li>%s</li>", inlineMarkdown(text)))
			continue
		}
		closeList()
		out.WriteString(fmt.Sprintf("<p>%s</p>", inlineMarkdown(trimmed)))
	}
	closeList()
	return out.String()
}

func countHeadingLevel(line string) int {
	level := 0
	for _, r := range line {
		if r != '#' {
			break
		}
		level++
	}
	if level == 0 {
		return 1
	}
	if level > 6 {
		return 6
	}
	return level
}

func inlineMarkdown(text string) string {
	if strings.Contains(text, "<") && strings.Contains(text, ">") {
		return text
	}
	text = linkRe.ReplaceAllString(text, `<a href="$2">$1</a>`)
	text = boldRe.ReplaceAllString(text, `<strong>$1</strong>`)
	text = italicRe.ReplaceAllString(text, `<em>$1</em>`)
	text = codeRe.ReplaceAllString(text, `<code>$1</code>`)
	return text
}

func wrapHTMLFragment(fragment string) string {
	return `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Страница не найдена</title>
  <link rel="stylesheet" href="style.css">
</head>
<body>
  ` + fragment + `
  <script src="script.js"></script>
</body>
</html>`
}

func isTableStart(lines []string, index int) bool {
	if index+1 >= len(lines) {
		return false
	}
	current := strings.TrimSpace(lines[index])
	next := strings.TrimSpace(lines[index+1])
	if !strings.Contains(current, "|") {
		return false
	}
	return tableSep.MatchString(next)
}

func renderTable(lines []string) (string, int) {
	if len(lines) < 2 {
		return "", 1
	}
	header := splitTableRow(lines[0])
	sep := lines[1]
	if !tableSep.MatchString(strings.TrimSpace(sep)) {
		return "", 1
	}
	var body [][]string
	consumed := 2
	for i := 2; i < len(lines); i++ {
		if !strings.Contains(lines[i], "|") || strings.TrimSpace(lines[i]) == "" {
			break
		}
		body = append(body, splitTableRow(lines[i]))
		consumed++
	}

	var out strings.Builder
	out.WriteString("<table><thead><tr>")
	for _, cell := range header {
		out.WriteString("<th>")
		out.WriteString(inlineMarkdown(cell))
		out.WriteString("</th>")
	}
	out.WriteString("</tr></thead><tbody>")
	for _, row := range body {
		out.WriteString("<tr>")
		for _, cell := range row {
			out.WriteString("<td>")
			out.WriteString(inlineMarkdown(cell))
			out.WriteString("</td>")
		}
		out.WriteString("</tr>")
	}
	out.WriteString("</tbody></table>")
	return out.String(), consumed
}

func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
