package httpserver

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"obzornik-pbn-generator/internal/llm"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

type editorGeneratedFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	MimeType string `json:"mime_type"`
}

type editorGeneratedAsset struct {
	Path     string `json:"path"`
	Alt      string `json:"alt"`
	Prompt   string `json:"prompt"`
	MimeType string `json:"mime_type"`
}

type editorPageSuggestionPayload struct {
	Files    []editorGeneratedFile  `json:"files"`
	Assets   []editorGeneratedAsset `json:"assets"`
	Warnings []string               `json:"warnings"`
}

func defaultEditorFilePrompt() string {
	return `Ты — AI-редактор файлов сайта.
Верни ТОЛЬКО финальный контент текущего файла, без markdown fence, без JSON, без пояснений.

Контекст сайта:
{{site_context}}

Ограничения задачи:
{{task_constraints}}

Текущий файл:
Путь: {{current_file_path}}
Содержимое:
{{current_file_content}}

Инструкция пользователя:
{{instruction}}`
}

func defaultEditorPagePrompt() string {
	return `Ты — AI-ассистент по созданию страниц сайта.
Верни СТРОГО валидный JSON (без markdown fence, без комментариев):
{"files":[{"path":"...","content":"...","mime_type":"..."}],"assets":[{"path":"...","alt":"...","prompt":"...","mime_type":"image/webp"}],"warnings":[]}

Правила:
1) files должен содержать минимум 1 файл.
2) Путь target страницы: {{target_path}}.
3) Соблюдай язык сайта и стиль существующего проекта.
4) Для изображений не возвращай бинарные данные в files. Используй массив assets как манифест.
5) Не добавляй посторонних ключей вне schema.

Контекст сайта:
{{site_context}}

Ограничения задачи:
{{task_constraints}}

Инструкция пользователя:
{{instruction}}`
}

func defaultEditorAssetRegeneratePrompt() string {
	return `Ты — AI-ассистент по генерации изображений для уже существующего сайта.
Сгенерируй ОДНО изображение строго по задаче. Возвращай только изображение (binary response), без текста.

Контекст сайта:
{{site_context}}

Ограничения задачи:
{{task_constraints}}

Путь ассета:
{{asset_path}}

Желаемый mime:
{{asset_mime_type}}

Инструкция для изображения:
{{asset_prompt}}`
}

func buildEditorRepairPrompt(raw string) string {
	return fmt.Sprintf(`Исправь ответ в СТРОГО валидный JSON без markdown fence.
Schema:
{"files":[{"path":"...","content":"...","mime_type":"..."}],"assets":[{"path":"...","alt":"...","prompt":"...","mime_type":"image/webp"}],"warnings":[]}
Верни только JSON.

Исходный ответ:
%s`, raw)
}

func normalizeEditorPageSuggestionPayload(parsed editorPageSuggestionPayload) ([]map[string]any, []map[string]any, []string) {
	warnings := append([]string(nil), parsed.Warnings...)
	outFiles := make([]map[string]any, 0, len(parsed.Files))
	for _, file := range parsed.Files {
		cleanPath, reason, err := normalizeEditorWritablePathWithReason(file.Path)
		if err != nil {
			switch reason {
			case "protected":
				warnings = append(warnings, fmt.Sprintf("protected path skipped: %s", cleanPath))
			case "blocked":
				warnings = append(warnings, fmt.Sprintf("blocked file type skipped: %s", cleanPath))
			default:
				warnings = append(warnings, fmt.Sprintf("invalid path skipped: %s", file.Path))
			}
			continue
		}
		content := file.Content
		if strings.TrimSpace(content) == "" {
			warnings = append(warnings, fmt.Sprintf("empty content skipped: %s", cleanPath))
			continue
		}
		if isImagePath(cleanPath) || isImageMime(file.MimeType) {
			warnings = append(warnings, fmt.Sprintf("binary asset skipped from files, use assets manifest: %s", cleanPath))
			continue
		}
		detected := detectMimeType(cleanPath, []byte(content))
		mimeType := strings.TrimSpace(file.MimeType)
		if mimeType == "" {
			mimeType = detected
		} else if err := validateMimeType(cleanPath, detected, mimeType); err != nil {
			warnings = append(warnings, fmt.Sprintf("mime mismatch skipped: %s", cleanPath))
			continue
		}
		if err := validateUploadSecurity(cleanPath, mimeType, []byte(content)); err != nil {
			warnings = append(warnings, fmt.Sprintf("security validation skipped: %s", cleanPath))
			continue
		}
		outFiles = append(outFiles, map[string]any{
			"path":      cleanPath,
			"content":   content,
			"mime_type": mimeType,
		})
	}
	outAssets := make([]map[string]any, 0, len(parsed.Assets))
	for _, asset := range parsed.Assets {
		cleanPath, reason, err := normalizeEditorWritablePathWithReason(asset.Path)
		if err != nil {
			switch reason {
			case "protected":
				warnings = append(warnings, fmt.Sprintf("protected asset path skipped: %s", cleanPath))
			case "blocked":
				warnings = append(warnings, fmt.Sprintf("blocked asset type skipped: %s", cleanPath))
			default:
				warnings = append(warnings, fmt.Sprintf("invalid asset path skipped: %s", asset.Path))
			}
			continue
		}
		mimeType := strings.TrimSpace(asset.MimeType)
		if mimeType == "" {
			mimeType = detectMimeType(cleanPath, nil)
		}
		if !isImageMime(mimeType) && !isImagePath(cleanPath) {
			warnings = append(warnings, fmt.Sprintf("asset skipped (not an image): %s", cleanPath))
			continue
		}
		outAssets = append(outAssets, map[string]any{
			"path":      cleanPath,
			"alt":       strings.TrimSpace(asset.Alt),
			"prompt":    strings.TrimSpace(asset.Prompt),
			"mime_type": mimeType,
		})
	}
	return outFiles, outAssets, warnings
}

func (s *Server) handleDomainEditorAICreatePage(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, project sqlstore.Project, requesterEmail string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	defer r.Body.Close()
	var body struct {
		Instruction  string   `json:"instruction"`
		TargetPath   string   `json:"target_path"`
		WithAssets   bool     `json:"with_assets"`
		Model        string   `json:"model"`
		ContextMode  string   `json:"context_mode"`
		ContextFiles []string `json:"context_files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid body", nil)
		return
	}
	instruction := strings.TrimSpace(body.Instruction)
	targetPath := strings.TrimSpace(body.TargetPath)
	if instruction == "" || targetPath == "" {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "instruction and target_path are required", nil)
		return
	}
	if !strings.HasSuffix(strings.ToLower(targetPath), ".html") {
		targetPath += ".html"
	}
	cleanTarget, err := normalizeEditorProtectedPath(targetPath)
	if err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrForbiddenPath, err.Error(), nil)
		return
	}
	contextMode := normalizeEditorContextMode(body.ContextMode)
	siteContext, contextMeta, err := s.buildEditorContextPack(r.Context(), domain, cleanTarget, "", body.ContextFiles, contextMode)
	if err != nil {
		writeEditorContextPackError(w, err)
		return
	}
	systemPrompt, promptSource, modelFromPrompt := s.resolveEditorPrompt(r.Context(), domain.ID, project.ID, "editor_page_create", defaultEditorPagePrompt())
	selectedModel := strings.TrimSpace(body.Model)
	if selectedModel == "" {
		selectedModel = modelFromPrompt
	}
	withAssets := "false"
	if body.WithAssets {
		withAssets = "true"
	}
	taskConstraints := fmt.Sprintf("operation=create_page\ncontext_mode=%s\ntarget_path=%s\nlanguage=%s\nkeyword=%s\nwith_assets=%s",
		contextMode,
		cleanTarget,
		domain.TargetLanguage,
		domain.MainKeyword,
		withAssets,
	)
	prompt := llm.BuildPrompt(systemPrompt, "", map[string]string{
		"instruction":      instruction,
		"target_path":      cleanTarget,
		"domain_url":       domain.URL,
		"language":         domain.TargetLanguage,
		"keyword":          domain.MainKeyword,
		"with_assets":      withAssets,
		"site_context":     siteContext,
		"task_constraints": taskConstraints,
	})
	response, tokenUsage, err := s.generateEditorSuggestion(
		r.Context(),
		requesterEmail,
		project.UserEmail,
		project.ID,
		domain.ID,
		"editor_ai_create_page",
		"editor_page_create",
		cleanTarget,
		selectedModel,
		prompt,
	)
	if err != nil {
		writeEditorError(w, http.StatusBadGateway, editorErrInvalidFormat, err.Error(), nil)
		return
	}
	repairCount := 0
	var parsed editorPageSuggestionPayload
	var parseErr error
	currentResponse := response
	for attempt := 0; attempt < 3; attempt++ {
		parsed, parseErr = parseJSONGeneratedFiles(currentResponse)
		if parseErr == nil {
			break
		}
		if attempt == 2 {
			break
		}
		repairPrompt := buildEditorRepairPrompt(currentResponse)
		currentResponse, tokenUsage, err = s.generateEditorSuggestion(
			r.Context(),
			requesterEmail,
			project.UserEmail,
			project.ID,
			domain.ID,
			"editor_ai_create_page",
			"editor_page_create",
			cleanTarget,
			selectedModel,
			repairPrompt,
		)
		if err != nil {
			writeEditorError(w, http.StatusBadGateway, editorErrInvalidFormat, err.Error(), map[string]any{
				"repair_attempts": repairCount,
			})
			return
		}
		repairCount++
	}
	if parseErr != nil {
		writeEditorError(w, http.StatusUnprocessableEntity, editorErrInvalidFormat, "AI returned invalid JSON format after repair attempts", map[string]any{
			"repair_attempts":   repairCount,
			"context_pack_meta": contextMeta,
		})
		return
	}
	outFiles, outAssets, warnings := normalizeEditorPageSuggestionPayload(parsed)
	if len(outFiles) == 0 {
		writeEditorError(w, http.StatusUnprocessableEntity, editorErrInvalidFormat, "no valid files after validation", map[string]any{
			"warnings":          warnings,
			"context_pack_meta": contextMeta,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"files":    outFiles,
		"assets":   outAssets,
		"warnings": warnings,
		"prompt_trace": map[string]any{
			"resolved_source": promptSource,
			"model":           selectedModel,
			"stage":           "editor_page_create",
			"repair_attempts": repairCount,
		},
		"token_usage":       tokenUsage,
		"context_pack_meta": contextMeta,
	})
}

func (s *Server) handleDomainEditorAIRegenerateAsset(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, project sqlstore.Project, requesterEmail string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	defer r.Body.Close()
	var body struct {
		Path         string   `json:"path"`
		Prompt       string   `json:"prompt"`
		Instruction  string   `json:"instruction"`
		MimeType     string   `json:"mime_type"`
		Model        string   `json:"model"`
		ContextMode  string   `json:"context_mode"`
		ContextFiles []string `json:"context_files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid body", nil)
		return
	}
	cleanPath, err := normalizeEditorWritablePath(strings.TrimSpace(body.Path))
	if err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrForbiddenPath, err.Error(), nil)
		return
	}
	desiredMime := strings.TrimSpace(body.MimeType)
	if desiredMime == "" {
		desiredMime = detectMimeType(cleanPath, nil)
	}
	if !isImagePath(cleanPath) && !isImageMime(desiredMime) {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "asset path/mime must be image/*", nil)
		return
	}
	promptInstruction := strings.TrimSpace(body.Prompt)
	if promptInstruction == "" {
		promptInstruction = strings.TrimSpace(body.Instruction)
	}
	if promptInstruction == "" {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "prompt is required", nil)
		return
	}
	contextMode := normalizeEditorContextMode(body.ContextMode)
	siteContext, contextMeta, err := s.buildEditorContextPack(r.Context(), domain, "index.html", cleanPath, body.ContextFiles, contextMode)
	if err != nil {
		writeEditorContextPackError(w, err)
		return
	}
	systemPrompt, promptSource, modelFromPrompt := s.resolveEditorPrompt(r.Context(), domain.ID, project.ID, "editor_asset_regenerate", defaultEditorAssetRegeneratePrompt())
	selectedModel := normalizeImageGenerationModel(strings.TrimSpace(body.Model), modelFromPrompt)
	taskConstraints := fmt.Sprintf("operation=regenerate_asset\ncontext_mode=%s\nasset_path=%s\nasset_mime_type=%s\nlanguage=%s\nkeyword=%s",
		contextMode,
		cleanPath,
		desiredMime,
		domain.TargetLanguage,
		domain.MainKeyword,
	)
	prompt := llm.BuildPrompt(systemPrompt, "", map[string]string{
		"asset_path":       cleanPath,
		"asset_prompt":     promptInstruction,
		"asset_mime_type":  desiredMime,
		"site_context":     siteContext,
		"task_constraints": taskConstraints,
		"domain_url":       domain.URL,
		"language":         domain.TargetLanguage,
		"keyword":          domain.MainKeyword,
	})
	imageBytes, tokenUsage, err := s.generateEditorImage(
		r.Context(),
		requesterEmail,
		project.UserEmail,
		project.ID,
		domain.ID,
		"editor_ai_regenerate_asset",
		"editor_asset_regenerate",
		cleanPath,
		selectedModel,
		prompt,
	)
	if err != nil {
		writeEditorError(w, http.StatusBadGateway, editorErrImageGenerationFail, err.Error(), nil)
		return
	}
	detectedMime := strings.ToLower(strings.TrimSpace(http.DetectContentType(imageBytes)))
	if !isImageMime(detectedMime) {
		detectedMime = detectMimeType(cleanPath, imageBytes)
	}
	regenWarnings := []string{"asset regenerated via AI image model"}
	wantsWebP := strings.EqualFold(filepath.Ext(cleanPath), ".webp") || strings.EqualFold(baseMimeType(desiredMime), "image/webp")
	if wantsWebP && !strings.EqualFold(baseMimeType(detectedMime), "image/webp") {
		converted, convErr := convertToWebP(imageBytes)
		if convErr != nil {
			writeEditorError(w, http.StatusUnprocessableEntity, editorErrAssetValidationFail, fmt.Sprintf("webp conversion failed: %v", convErr), map[string]any{
				"context_pack_meta": contextMeta,
			})
			return
		}
		imageBytes = converted
		detectedMime = "image/webp"
		regenWarnings = append(regenWarnings, "image converted to webp")
	}
	if err := validateMimeType(cleanPath, detectedMime, desiredMime); err != nil {
		if isImageMime(desiredMime) && isImageMime(detectedMime) {
			regenWarnings = append(regenWarnings, fmt.Sprintf("image mime adjusted: requested %s, got %s", baseMimeType(desiredMime), baseMimeType(detectedMime)))
		} else {
			writeEditorError(w, http.StatusUnprocessableEntity, editorErrAssetValidationFail, err.Error(), map[string]any{
				"context_pack_meta": contextMeta,
			})
			return
		}
	}
	if err := validateUploadSecurity(cleanPath, detectedMime, imageBytes); err != nil {
		if isImageMime(detectedMime) {
			if ext := imageExtByMime(detectedMime); ext != "" {
				altPath := strings.TrimSuffix(cleanPath, filepath.Ext(cleanPath)) + ext
				if altErr := validateUploadSecurity(altPath, detectedMime, imageBytes); altErr == nil {
					regenWarnings = append(regenWarnings, fmt.Sprintf("image payload validated as %s (path extension differs)", baseMimeType(detectedMime)))
				} else {
					writeEditorError(w, http.StatusUnprocessableEntity, editorErrAssetValidationFail, err.Error(), map[string]any{
						"context_pack_meta": contextMeta,
					})
					return
				}
			} else {
				writeEditorError(w, http.StatusUnprocessableEntity, editorErrAssetValidationFail, err.Error(), map[string]any{
					"context_pack_meta": contextMeta,
				})
				return
			}
		} else {
			writeEditorError(w, http.StatusUnprocessableEntity, editorErrAssetValidationFail, err.Error(), map[string]any{
				"context_pack_meta": contextMeta,
			})
			return
		}
	}

	existing, getErr := s.siteFiles.GetByPath(r.Context(), domain.ID, cleanPath)
	if getErr != nil && !errors.Is(getErr, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	var oldContent []byte
	if existing != nil {
		oldContent, _ = s.readDomainFileBytesFromBackend(r.Context(), domain, cleanPath)
		_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(existing, oldContent, "ai", requesterEmail, "baseline before ai regenerate asset"))
	}
	if err := s.writeDomainFileBytesToBackend(r.Context(), domain, cleanPath, imageBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "could not write file")
		return
	}
	if existing == nil {
		hash := sha256.Sum256(imageBytes)
		file := sqlstore.SiteFile{
			ID:           uuid.NewString(),
			DomainID:     domain.ID,
			Path:         cleanPath,
			ContentHash:  sql.NullString{String: hex.EncodeToString(hash[:]), Valid: true},
			SizeBytes:    int64(len(imageBytes)),
			MimeType:     detectedMime,
			Version:      1,
			LastEditedBy: sqlstore.NullableString(requesterEmail),
		}
		if err := s.siteFiles.Create(r.Context(), file); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create file metadata")
			return
		}
		_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(&file, imageBytes, "ai", requesterEmail, "ai regenerated asset"))
		_ = s.fileEdits.Create(r.Context(), sqlstore.FileEdit{
			ID:               uuid.NewString(),
			FileID:           file.ID,
			EditedBy:         requesterEmail,
			EditType:         "ai",
			EditDescription:  sql.NullString{String: "ai regenerated asset", Valid: true},
			ContentAfterHash: sql.NullString{String: hex.EncodeToString(hash[:]), Valid: true},
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "regenerated",
			"file":     toFileDTO(domain, file),
			"warnings": regenWarnings,
			"prompt_trace": map[string]any{
				"resolved_source": promptSource,
				"model":           selectedModel,
				"stage":           "editor_asset_regenerate",
			},
			"token_usage":       tokenUsage,
			"context_pack_meta": contextMeta,
		})
		return
	}
	if err := s.siteFiles.Update(r.Context(), existing.ID, imageBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "could not update file metadata")
		return
	}
	_ = s.siteFiles.SetLastEditedBy(r.Context(), existing.ID, sqlstore.NullableString(requesterEmail))
	updated, _ := s.siteFiles.Get(r.Context(), existing.ID)
	if updated != nil {
		_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(updated, imageBytes, "ai", requesterEmail, "ai regenerated asset"))
	}
	beforeHash := sha256.Sum256(oldContent)
	afterHash := sha256.Sum256(imageBytes)
	_ = s.fileEdits.Create(r.Context(), sqlstore.FileEdit{
		ID:                uuid.NewString(),
		FileID:            existing.ID,
		EditedBy:          requesterEmail,
		EditType:          "ai",
		EditDescription:   sql.NullString{String: "ai regenerated asset", Valid: true},
		ContentBeforeHash: sql.NullString{String: hex.EncodeToString(beforeHash[:]), Valid: len(oldContent) > 0},
		ContentAfterHash:  sql.NullString{String: hex.EncodeToString(afterHash[:]), Valid: true},
	})
	if updated == nil {
		writeError(w, http.StatusInternalServerError, "could not load updated file")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "regenerated",
		"file":     toFileDTO(domain, *updated),
		"warnings": regenWarnings,
		"prompt_trace": map[string]any{
			"resolved_source": promptSource,
			"model":           selectedModel,
			"stage":           "editor_asset_regenerate",
		},
		"token_usage":       tokenUsage,
		"context_pack_meta": contextMeta,
	})
}

func (s *Server) handleDomainEditorAISuggest(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, project sqlstore.Project, relPath, requesterEmail string) {
	if !ensureJSON(w, r) {
		return
	}
	defer r.Body.Close()
	var body struct {
		Instruction  string   `json:"instruction"`
		Model        string   `json:"model"`
		Selection    string   `json:"selection"`
		ContextFiles []string `json:"context_files"`
		ContextMode  string   `json:"context_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid body", nil)
		return
	}
	instruction := strings.TrimSpace(body.Instruction)
	if instruction == "" {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "instruction is required", nil)
		return
	}
	content, mimeType, err := s.readDomainFileContent(r.Context(), domain, relPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not read file")
		return
	}
	contextMode := normalizeEditorContextMode(body.ContextMode)
	siteContext, contextMeta, err := s.buildEditorContextPack(r.Context(), domain, relPath, relPath, body.ContextFiles, contextMode)
	if err != nil {
		writeEditorContextPackError(w, err)
		return
	}
	systemPrompt, promptSource, modelFromPrompt := s.resolveEditorPrompt(r.Context(), domain.ID, project.ID, "editor_file_edit", defaultEditorFilePrompt())
	selectedModel := strings.TrimSpace(body.Model)
	if selectedModel == "" {
		selectedModel = modelFromPrompt
	}
	taskConstraints := fmt.Sprintf("operation=edit_file\ncontext_mode=%s\ncurrent_file=%s\nlanguage=%s\nkeyword=%s",
		contextMode,
		relPath,
		domain.TargetLanguage,
		domain.MainKeyword,
	)
	prompt := llm.BuildPrompt(systemPrompt, "", map[string]string{
		"instruction":          instruction,
		"current_file_path":    relPath,
		"current_file_content": content,
		"domain_url":           domain.URL,
		"language":             domain.TargetLanguage,
		"keyword":              domain.MainKeyword,
		"selection":            strings.TrimSpace(body.Selection),
		"site_context":         siteContext,
		"task_constraints":     taskConstraints,
	})
	suggestedRaw, tokenUsage, err := s.generateEditorSuggestion(
		r.Context(),
		requesterEmail,
		project.UserEmail,
		project.ID,
		domain.ID,
		"editor_ai_suggest",
		"editor_file_edit",
		relPath,
		selectedModel,
		prompt,
	)
	if err != nil {
		writeEditorError(w, http.StatusBadGateway, editorErrInvalidFormat, err.Error(), nil)
		return
	}
	suggested, warnings := sanitizeAISuggestContent(suggestedRaw, content)
	writeJSON(w, http.StatusOK, map[string]any{
		"suggested_content": suggested,
		"diff_summary": map[string]any{
			"old_bytes": len(content),
			"new_bytes": len(suggested),
		},
		"warnings": warnings,
		"prompt_trace": map[string]any{
			"resolved_source": promptSource,
			"model":           selectedModel,
			"stage":           "editor_file_edit",
		},
		"token_usage":       tokenUsage,
		"mime_type":         mimeType,
		"context_pack_meta": contextMeta,
	})
}
