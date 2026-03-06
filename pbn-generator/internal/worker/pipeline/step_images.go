package pipeline

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"obzornik-pbn-generator/internal/llm"
)

const (
	// ModelGeminiImage - модель по умолчанию для генерации изображений
	ModelGeminiImage = "gemini-2.5-flash-image"
)

func isImageModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	switch model {
	case ModelGeminiImage:
		return true
	default:
		return strings.Contains(model, "image")
	}
}

type ImageTask struct {
	Slug   string `json:"slug"`
	Prompt string `json:"prompt"`
}

// ImageGenerationStep генерирует промпты и сами изображения
// для плейсхолдеров из технического задания.
type ImageGenerationStep struct{}

func (s *ImageGenerationStep) Name() string { return StepImageGeneration }

func (s *ImageGenerationStep) ArtifactKey() string { return "image_prompts" }

func (s *ImageGenerationStep) Progress() int { return 88 }

// sanitizeSlug очищает slug от опасных символов и защищает от path traversal атак
func sanitizeSlug(slug string) string {
	// Удаляем все опасные паттерны для path traversal
	cleaned := strings.ReplaceAll(slug, "../", "")
	cleaned = strings.ReplaceAll(cleaned, "./", "")
	cleaned = strings.ReplaceAll(cleaned, "..\\", "") // Windows paths
	cleaned = strings.ReplaceAll(cleaned, ".\\", "")
	cleaned = strings.ReplaceAll(cleaned, "/", "")
	cleaned = strings.ReplaceAll(cleaned, "\\", "")

	// Удаляем все символы, кроме букв, цифр, дефисов и подчеркиваний
	reg := regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
	cleaned = reg.ReplaceAllString(cleaned, "")

	// Заменяем пробелы на дефисы (если они остались после предыдущей очистки)
	cleaned = strings.ReplaceAll(cleaned, " ", "-")

	// Удаляем множественные дефисы
	regMultiDash := regexp.MustCompile(`-+`)
	cleaned = regMultiDash.ReplaceAllString(cleaned, "-")

	// Удаляем дефисы в начале и конце
	cleaned = strings.Trim(cleaned, "-_")

	// Обрезаем до 100 символов
	if len(cleaned) > 100 {
		cleaned = cleaned[:100]
		// Убеждаемся, что не обрезали посередине слова
		cleaned = strings.TrimRightFunc(cleaned, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})
	}

	// Если после очистки slug пустой, возвращаем дефолтное значение
	if cleaned == "" {
		cleaned = "image"
	}

	return cleaned
}

func (s *ImageGenerationStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	state.AppendLog("Начало генерации изображений")

	designSystem, _ := state.Artifacts["design_system"].(string)
	if dsCtx, ok := state.Context["design_system"].(string); designSystem == "" && ok {
		designSystem = dsCtx
	}
	technicalSpec, _ := state.Artifacts["technical_spec"].(string)
	if technicalSpec == "" {
		if tsCtx, ok := state.Context["technical_spec"].(string); ok {
			technicalSpec = tsCtx
		}
	}
	if strings.TrimSpace(designSystem) == "" || strings.TrimSpace(technicalSpec) == "" {
		return nil, fmt.Errorf("design_system и technical_spec обязательны для image_generation")
	}

	// Получаем промпт для генерации промптов картинок
	promptID, promptBody, promptModel, err := state.PromptManager.GetPromptByStage(ctx, "image_prompt_generation")
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt for image_prompt_generation: %w", err)
	}
	state.AppendLog(fmt.Sprintf("Используется промпт: %s для этапа image_prompt_generation", promptID))
	if promptModel != "" {
		state.AppendLog(fmt.Sprintf("Используется модель LLM: %s", promptModel))
	}

	// Определяем модель для генерации изображений
	imageModelToUse := promptModel
	if imageModelToUse == "" || !isImageModel(imageModelToUse) {
		if imageModelToUse != "" && !isImageModel(imageModelToUse) {
			state.AppendLog(fmt.Sprintf("Предупреждение: модель %s не поддерживает генерацию изображений, используем %s", imageModelToUse, ModelGeminiImage))
		}
		imageModelToUse = ModelGeminiImage
	}
	state.AppendLog(fmt.Sprintf("Используется модель для генерации изображений: %s", imageModelToUse))

	variables := map[string]string{
		"design_system":  designSystem,
		"technical_spec": technicalSpec,
	}
	prompt := llm.BuildPrompt(promptBody, "", variables)
	preview := prompt
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	state.AppendLog(fmt.Sprintf("Промпт для изображений (первые 500 символов): %s", preview))

	modelToUse := promptModel
	if modelToUse == "" {
		modelToUse = state.DefaultModel
	}

	raw, err := state.LLMClient.Generate(ctx, "image_prompt_generation", prompt, modelToUse)
	if err != nil {
		return nil, fmt.Errorf("image prompt generation failed: %w", err)
	}

	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var tasks []ImageTask
	if err := json.Unmarshal([]byte(clean), &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse image tasks JSON: %w", err)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("image prompt generation returned empty tasks")
	}

	generated := make([]GeneratedFile, 0)
	existing := mergeGeneratedFiles(state.Artifacts["generated_files"], nil)
	generated = append(generated, existing...)

	imageFailures := 0

	for i, task := range tasks {
		if strings.TrimSpace(task.Slug) == "" || strings.TrimSpace(task.Prompt) == "" {
			state.AppendLog(fmt.Sprintf("Пропуск изображения: пустой slug или prompt: %+v", task))
			continue
		}

		// Санитизируем slug для безопасности (защита от path traversal)
		safeSlug := sanitizeSlug(task.Slug)
		if safeSlug == "image" && task.Slug != "" {
			// Если slug был непустой, но стал "image" после санитизации, добавляем индекс
			safeSlug = fmt.Sprintf("image-%d", i+1)
		}

		// Генерация картинки
		bytesImg, err := state.LLMClient.GenerateImage(ctx, task.Prompt, imageModelToUse)
		if err != nil {
			state.AppendLog(fmt.Sprintf("Предупреждение: не удалось сгенерировать %s (original slug: %s): %v", safeSlug, task.Slug, err))
			imageFailures++
			continue
		}

		// Всегда конвертируем в WebP и сохраняем только WebP-артефакт.
		webpBytes, err := convertToWebP(bytesImg)
		if err != nil {
			state.AppendLog(fmt.Sprintf("⚠️ WebP conversion failed for %s (original slug: %s): %v", safeSlug, task.Slug, err))
			imageFailures++
			continue
		}
		webpEncoded := base64.StdEncoding.EncodeToString(webpBytes)
		generated = append(generated, GeneratedFile{
			Path:          fmt.Sprintf("%s.webp", safeSlug),
			ContentBase64: webpEncoded,
		})
	}

	artifacts := map[string]any{
		"image_prompts":   tasks,
		"generated_files": generated,
	}

	if imageFailures > 0 {
		state.AppendLog(fmt.Sprintf("Генерация изображений завершена с предупреждениями: не удалось создать %d изображений", imageFailures))
	} else {
		state.AppendLog("Генерация изображений завершена")
	}

	return artifacts, nil
}
