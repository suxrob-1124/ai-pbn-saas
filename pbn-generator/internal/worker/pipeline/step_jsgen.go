package pipeline

import (
	"context"
	"fmt"
	"strings"

	"obzornik-pbn-generator/internal/llm"
)

// JSGenerationStep реализует генерацию итогового JS
type JSGenerationStep struct{}

func (s *JSGenerationStep) Name() string {
	return StepJSGeneration
}

func (s *JSGenerationStep) ArtifactKey() string {
	return "js_content"
}

func (s *JSGenerationStep) Progress() int {
	return 99
}

func (s *JSGenerationStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало генерации JavaScript")

	designSystem, ok := state.Context["design_system"].(string)
	if !ok || strings.TrimSpace(designSystem) == "" {
		if v, ok := state.Artifacts["design_system"].(string); ok {
			designSystem = v
		}
	}
	htmlRaw, ok := state.Context["html_raw"].(string)
	if !ok || strings.TrimSpace(htmlRaw) == "" {
		if v, ok := state.Artifacts["html_raw"].(string); ok {
			htmlRaw = v
		}
	}

	if strings.TrimSpace(designSystem) == "" {
		return nil, fmt.Errorf("design_system is required for js generation")
	}
	if strings.TrimSpace(htmlRaw) == "" {
		return nil, fmt.Errorf("html_raw is required for js generation")
	}

	promptID, promptBody, promptModel, err := state.PromptManager.GetPromptByStage(ctx, "js_generation")
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt for js_generation: %w", err)
	}
	state.AppendLog(fmt.Sprintf("Используется промпт: %s для этапа js_generation", promptID))
	if promptModel != "" {
		state.AppendLog(fmt.Sprintf("Используется модель LLM: %s", promptModel))
	}

	variables := map[string]string{
		"design_system": designSystem,
		"html_raw":      htmlRaw,
	}
	jsPrompt := llm.BuildPrompt(promptBody, "", variables)

	preview := jsPrompt
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	state.AppendLog(fmt.Sprintf("Промпт для JS (первые 500 символов): %s", preview))

	modelToUse := promptModel
	if modelToUse == "" {
		modelToUse = state.DefaultModel
	}

	rawJS, err := state.LLMClient.Generate(ctx, "js_generation", jsPrompt, modelToUse)
	if err != nil {
		return nil, fmt.Errorf("js generation failed: %w", err)
	}

	clean := strings.TrimSpace(rawJS)
	clean = strings.TrimPrefix(clean, "```javascript")
	clean = strings.TrimPrefix(clean, "```js")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	files := mergeGeneratedFiles(state.Artifacts["generated_files"], []GeneratedFile{
		{Path: "script.js", Content: clean},
	})

	artifacts := map[string]any{
		"js_content":           clean,
		"generated_files":      files,
		"js_generation_prompt": formatPromptForArtifact(jsPrompt),
	}

	state.Context["js_content"] = clean
	state.AppendLog("Генерация JavaScript завершена")

	return artifacts, nil
}
