package httpserver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"obzornik-pbn-generator/internal/crypto/secretbox"
	"obzornik-pbn-generator/internal/llm"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

type editorAPIKeyMeta struct {
	Source        string
	KeyOwnerEmail string
	KeyType       string
}

func (s *Server) resolveEditorAPIKey(ctx context.Context, requesterEmail, ownerEmail string) (string, editorAPIKeyMeta, error) {
	tryUser := func(email string) (string, bool) {
		email = strings.TrimSpace(email)
		if email == "" {
			return "", false
		}
		encKey, err := s.svc.GetUserAPIKeyEncrypted(ctx, email)
		if err != nil || len(encKey) == 0 {
			return "", false
		}
		keySecret := secretbox.DeriveKey(s.cfg.APIKeySecret)
		decrypted, err := secretbox.Decrypt(keySecret, encKey)
		if err != nil {
			return "", false
		}
		val := strings.TrimSpace(string(decrypted))
		if val == "" {
			return "", false
		}
		return val, true
	}
	if key, ok := tryUser(requesterEmail); ok {
		return key, editorAPIKeyMeta{Source: "user", KeyOwnerEmail: requesterEmail, KeyType: "user"}, nil
	}
	if key, ok := tryUser(ownerEmail); ok {
		return key, editorAPIKeyMeta{Source: "owner", KeyOwnerEmail: ownerEmail, KeyType: "user"}, nil
	}
	if key := strings.TrimSpace(s.cfg.GeminiAPIKey); key != "" {
		return key, editorAPIKeyMeta{Source: "global", KeyType: "global"}, nil
	}
	return "", editorAPIKeyMeta{}, errors.New("gemini api key not configured")
}

func (s *Server) resolveEditorPrompt(ctx context.Context, domainID, projectID, stage, fallback string) (string, string, string) {
	promptBody := fallback
	promptSource := "fallback"
	promptModel := ""
	if s.promptOverrides != nil {
		if resolved, err := s.promptOverrides.ResolveForDomainStage(ctx, domainID, projectID, stage); err == nil {
			if strings.TrimSpace(resolved.Body) != "" {
				promptBody = resolved.Body
				promptSource = resolved.Source
			}
			if resolved.Model.Valid {
				promptModel = strings.TrimSpace(resolved.Model.String)
			}
		}
	}
	return promptBody, promptSource, promptModel
}

func extractCSSVariables(styleContent string, limit int) []string {
	if limit <= 0 {
		limit = 20
	}
	out := make([]string, 0, limit)
	for _, line := range strings.Split(styleContent, "\n") {
		l := strings.TrimSpace(line)
		if !strings.Contains(l, "--") || !strings.Contains(l, ":") {
			continue
		}
		if strings.HasPrefix(l, "/*") || strings.HasPrefix(l, "*") {
			continue
		}
		out = append(out, l)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func extractLocalHrefValues(html string, limit int) []string {
	if limit <= 0 {
		limit = 20
	}
	out := make([]string, 0, limit)
	seen := map[string]struct{}{}
	rest := html
	for len(out) < limit {
		idx := strings.Index(strings.ToLower(rest), "href=")
		if idx < 0 {
			break
		}
		rest = rest[idx+5:]
		rest = strings.TrimLeft(rest, " \t\r\n")
		if len(rest) == 0 {
			break
		}
		quote := rest[0]
		if quote != '"' && quote != '\'' {
			continue
		}
		rest = rest[1:]
		end := strings.IndexByte(rest, quote)
		if end < 0 {
			break
		}
		val := strings.TrimSpace(rest[:end])
		rest = rest[end+1:]
		if val == "" || strings.HasPrefix(val, "#") {
			continue
		}
		lower := strings.ToLower(val)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "tel:") || strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
			continue
		}
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		out = append(out, val)
	}
	return out
}

func (s *Server) pruneEditorContextCacheLocked(now time.Time) {
	for key, item := range s.editorCtxCache {
		if now.After(item.ExpiresAt) {
			delete(s.editorCtxCache, key)
		}
	}
	if len(s.editorCtxCache) <= editorContextPackCacheMaxEntries {
		return
	}
	type pair struct {
		Key string
		Exp time.Time
	}
	all := make([]pair, 0, len(s.editorCtxCache))
	for key, item := range s.editorCtxCache {
		all = append(all, pair{Key: key, Exp: item.ExpiresAt})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Exp.Before(all[j].Exp) })
	needDrop := len(s.editorCtxCache) - editorContextPackCacheMaxEntries
	for i := 0; i < needDrop && i < len(all); i++ {
		delete(s.editorCtxCache, all[i].Key)
	}
}

func (s *Server) buildEditorContextPack(ctx context.Context, domain sqlstore.Domain, targetPath, currentPath string, userSelected []string, mode string) (string, editorContextPackMeta, error) {
	meta := editorContextPackMeta{
		SourceFiles: make([]string, 0, editorContextPackMaxFiles),
	}
	mode = normalizeEditorContextMode(mode)
	files, err := s.siteFiles.List(ctx, domain.ID)
	if err != nil {
		return "", meta, err
	}
	byPath := make(map[string]sqlstore.SiteFile, len(files))
	for _, f := range files {
		byPath[f.Path] = f
	}

	baseCandidates := []string{
		"index.html",
		"style.css",
		"script.js",
		"404.html",
		"robots.txt",
		"sitemap.xml",
	}
	if strings.TrimSpace(currentPath) != "" {
		baseCandidates = append([]string{currentPath}, baseCandidates...)
	}
	if strings.TrimSpace(targetPath) != "" {
		baseCandidates = append([]string{targetPath}, baseCandidates...)
	}
	logoCandidates := make([]string, 0, 4)
	for _, f := range files {
		name := strings.ToLower(path.Base(f.Path))
		if strings.Contains(name, "logo.") || strings.HasPrefix(name, "logo-") {
			logoCandidates = append(logoCandidates, f.Path)
		}
	}
	sort.Strings(logoCandidates)
	if len(logoCandidates) > 4 {
		logoCandidates = logoCandidates[:4]
	}

	sanitizeSelected := make([]string, 0, len(userSelected))
	for _, raw := range userSelected {
		clean, err := sanitizeFilePath(raw)
		if err != nil {
			continue
		}
		if err := validateEditorPath(clean); err != nil {
			continue
		}
		if _, ok := byPath[clean]; ok {
			sanitizeSelected = append(sanitizeSelected, clean)
		}
	}

	candidates := make([]string, 0, editorContextPackMaxFiles+8)
	switch mode {
	case "manual":
		candidates = append(candidates, sanitizeSelected...)
	case "hybrid":
		candidates = append(candidates, baseCandidates...)
		candidates = append(candidates, logoCandidates...)
		candidates = append(candidates, sanitizeSelected...)
	default:
		candidates = append(candidates, baseCandidates...)
		candidates = append(candidates, logoCandidates...)
	}
	seen := map[string]struct{}{}
	finalPaths := make([]string, 0, editorContextPackMaxFiles)
	for _, p := range candidates {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := byPath[p]; !ok {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		finalPaths = append(finalPaths, p)
		if len(finalPaths) >= editorContextPackMaxFiles {
			break
		}
	}

	signatureParts := make([]string, 0, len(finalPaths)+8)
	signatureParts = append(signatureParts, domain.ID, mode, targetPath, currentPath)
	for _, p := range finalPaths {
		f := byPath[p]
		hash := ""
		if f.ContentHash.Valid {
			hash = strings.TrimSpace(f.ContentHash.String)
		}
		signatureParts = append(signatureParts, fmt.Sprintf("%s|%d|%s", p, f.Version, hash))
	}
	cacheKeyBytes := sha256.Sum256([]byte(strings.Join(signatureParts, "\n")))
	cacheKey := hex.EncodeToString(cacheKeyBytes[:])
	now := time.Now().UTC()
	s.editorCtxMu.Lock()
	if cached, ok := s.editorCtxCache[cacheKey]; ok && now.Before(cached.ExpiresAt) {
		s.editorCtxMu.Unlock()
		return cached.Prompt, cached.Meta, nil
	}
	s.editorCtxMu.Unlock()

	var builder strings.Builder
	builder.WriteString("SITE_CONTEXT\n")
	builder.WriteString(fmt.Sprintf("identity.domain_url=%s\n", domain.URL))
	builder.WriteString(fmt.Sprintf("identity.language=%s\n", strings.TrimSpace(domain.TargetLanguage)))
	builder.WriteString(fmt.Sprintf("identity.country=%s\n", strings.TrimSpace(domain.TargetCountry)))
	builder.WriteString(fmt.Sprintf("identity.keyword=%s\n", strings.TrimSpace(domain.MainKeyword)))
	builder.WriteString(fmt.Sprintf("identity.context_mode=%s\n", mode))

	totalBytes := 0
	truncated := false
	styleContent := ""
	indexContent := ""
	for _, p := range finalPaths {
		file := byPath[p]
		meta.SourceFiles = append(meta.SourceFiles, p)
		raw, err := s.readDomainFileBytes(domain, p)
		if err != nil {
			continue
		}
		builder.WriteString("\n\n[FILE] ")
		builder.WriteString(p)
		builder.WriteString("\n")
		builder.WriteString("mime=")
		builder.WriteString(file.MimeType)
		builder.WriteString("\n")
		if isBinaryMimeType(file.MimeType) {
			builder.WriteString(fmt.Sprintf("binary_meta.size_bytes=%d\n", len(raw)))
			continue
		}
		content := string(raw)
		if p == "style.css" {
			styleContent = content
		}
		if p == "index.html" {
			indexContent = content
		}
		if len(content) > editorContextPackMaxFileBytes {
			content = content[:editorContextPackMaxFileBytes]
			truncated = true
		}
		if totalBytes+len(content) > editorContextPackMaxTotalBytes {
			left := editorContextPackMaxTotalBytes - totalBytes
			if left <= 0 {
				truncated = true
				break
			}
			content = content[:left]
			truncated = true
		}
		totalBytes += len(content)
		builder.WriteString(content)
		if totalBytes >= editorContextPackMaxTotalBytes {
			break
		}
	}

	builder.WriteString("\n\n[DERIVED]\n")
	if strings.TrimSpace(styleContent) != "" {
		vars := extractCSSVariables(styleContent, 20)
		if len(vars) > 0 {
			builder.WriteString("design_tokens:\n")
			for _, item := range vars {
				builder.WriteString("- ")
				builder.WriteString(item)
				builder.WriteByte('\n')
			}
		}
	}
	if strings.TrimSpace(indexContent) != "" {
		links := extractLocalHrefValues(indexContent, 20)
		if len(links) > 0 {
			builder.WriteString("page_structure_links:\n")
			for _, href := range links {
				builder.WriteString("- ")
				builder.WriteString(href)
				builder.WriteByte('\n')
			}
		}
	}

	contextText := builder.String()
	packHashBytes := sha256.Sum256([]byte(contextText))
	meta.PackHash = hex.EncodeToString(packHashBytes[:])
	meta.FilesUsed = len(meta.SourceFiles)
	meta.BytesUsed = totalBytes
	meta.Truncated = truncated

	s.editorCtxMu.Lock()
	s.editorCtxCache[cacheKey] = editorContextPackCacheEntry{
		Prompt:    contextText,
		Meta:      meta,
		ExpiresAt: now.Add(editorContextPackCacheTTL),
	}
	s.pruneEditorContextCacheLocked(now)
	s.editorCtxMu.Unlock()
	return contextText, meta, nil
}

func (s *Server) generateEditorSuggestion(ctx context.Context, requesterEmail, ownerEmail, projectID, domainID, operation, stage, filePath, model, prompt string) (string, map[string]any, error) {
	apiKey, keyMeta, err := s.resolveEditorAPIKey(ctx, requesterEmail, ownerEmail)
	if err != nil {
		return "", nil, err
	}
	client := llm.NewClient(llm.Config{
		APIKey:          apiKey,
		DefaultModel:    s.cfg.GeminiDefaultModel,
		MaxRetries:      s.cfg.GeminiMaxRetries,
		RetryDelay:      s.cfg.GeminiRetryDelay,
		RequestTimeout:  s.cfg.GeminiRequestTimeout,
		RateLimitPerMin: s.cfg.GeminiRateLimitPerMin,
	})
	selectedModel := strings.TrimSpace(model)
	result, err := client.Generate(ctx, stage, prompt, selectedModel)
	reqs := client.GetRequests()
	var req llm.LLMRequest
	if len(reqs) > 0 {
		req = reqs[len(reqs)-1]
	}
	if req.Model == "" {
		req.Model = selectedModel
	}
	if req.TokenSource == "" {
		req.TokenSource = "estimated"
	}
	inputPrice, outputPrice, estCost := s.resolveLLMPriceSnapshot(ctx, req)

	if err != nil {
		s.logEditorLLMUsageEventWithPricing(ctx, req, requesterEmail, keyMeta, projectID, domainID, operation, stage, filePath, inputPrice, outputPrice, estCost)
		return "", nil, llm.SanitizeError(err)
	}
	s.logEditorLLMUsageEventWithPricing(ctx, req, requesterEmail, keyMeta, projectID, domainID, operation, stage, filePath, inputPrice, outputPrice, estCost)

	tokenUsage := map[string]any{
		"source":            keyMeta.Source,
		"model":             req.Model,
		"stage":             stage,
		"prompt_tokens":     req.PromptTokens,
		"completion_tokens": req.CompletionTokens,
		"total_tokens":      req.TotalTokens,
		"token_source":      llmTokenSource(req.TokenSource),
	}
	if estCost.Valid {
		tokenUsage["estimated_cost_usd"] = estCost.Float64
	}
	if inputPrice.Valid {
		tokenUsage["input_price_usd_per_million"] = inputPrice.Float64
	}
	if outputPrice.Valid {
		tokenUsage["output_price_usd_per_million"] = outputPrice.Float64
	}
	return strings.TrimSpace(result), tokenUsage, nil
}

func (s *Server) generateEditorImage(ctx context.Context, requesterEmail, ownerEmail, projectID, domainID, operation, stage, filePath, model, prompt string) ([]byte, map[string]any, error) {
	apiKey, keyMeta, err := s.resolveEditorAPIKey(ctx, requesterEmail, ownerEmail)
	if err != nil {
		return nil, nil, err
	}
	client := llm.NewClient(llm.Config{
		APIKey:          apiKey,
		DefaultModel:    s.cfg.GeminiDefaultModel,
		MaxRetries:      s.cfg.GeminiMaxRetries,
		RetryDelay:      s.cfg.GeminiRetryDelay,
		RequestTimeout:  s.cfg.GeminiRequestTimeout,
		RateLimitPerMin: s.cfg.GeminiRateLimitPerMin,
	})
	selectedModel := normalizeImageGenerationModel(strings.TrimSpace(model))
	imageBytes, err := client.GenerateImage(ctx, prompt, selectedModel)
	reqs := client.GetRequests()
	var req llm.LLMRequest
	if len(reqs) > 0 {
		req = reqs[len(reqs)-1]
	}
	if req.Model == "" {
		req.Model = selectedModel
	}
	if req.TokenSource == "" {
		req.TokenSource = "estimated"
	}
	inputPrice, outputPrice, estCost := s.resolveLLMPriceSnapshot(ctx, req)

	if err != nil {
		s.logEditorLLMUsageEventWithPricing(ctx, req, requesterEmail, keyMeta, projectID, domainID, operation, stage, filePath, inputPrice, outputPrice, estCost)
		return nil, nil, llm.SanitizeError(err)
	}
	s.logEditorLLMUsageEventWithPricing(ctx, req, requesterEmail, keyMeta, projectID, domainID, operation, stage, filePath, inputPrice, outputPrice, estCost)
	tokenUsage := map[string]any{
		"source":            keyMeta.Source,
		"model":             req.Model,
		"stage":             stage,
		"prompt_tokens":     req.PromptTokens,
		"completion_tokens": req.CompletionTokens,
		"total_tokens":      req.TotalTokens,
		"token_source":      llmTokenSource(req.TokenSource),
	}
	if estCost.Valid {
		tokenUsage["estimated_cost_usd"] = estCost.Float64
	}
	if inputPrice.Valid {
		tokenUsage["input_price_usd_per_million"] = inputPrice.Float64
	}
	if outputPrice.Valid {
		tokenUsage["output_price_usd_per_million"] = outputPrice.Float64
	}
	return imageBytes, tokenUsage, nil
}

func isImageGenerationModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	switch model {
	case "gemini-2.5-flash-image":
		return true
	default:
		return strings.Contains(model, "image")
	}
}

func (s *Server) resolveLLMPriceSnapshot(ctx context.Context, req llm.LLMRequest) (sql.NullFloat64, sql.NullFloat64, sql.NullFloat64) {
	if s.modelPricing == nil {
		return sql.NullFloat64{}, sql.NullFloat64{}, sql.NullFloat64{}
	}
	at := req.Timestamp
	if at.IsZero() {
		at = time.Now().UTC()
	}
	pricing, err := s.modelPricing.GetActiveByModel(ctx, "gemini", req.Model, at)
	if err != nil || pricing == nil {
		return sql.NullFloat64{}, sql.NullFloat64{}, sql.NullFloat64{}
	}
	inputPrice := sql.NullFloat64{Float64: pricing.InputUSDPerMillion, Valid: true}
	outputPrice := sql.NullFloat64{Float64: pricing.OutputUSDPerMillion, Valid: true}
	estCost := sql.NullFloat64{
		Float64: estimateLLMCostUSD(req.PromptTokens, req.CompletionTokens, pricing.InputUSDPerMillion, pricing.OutputUSDPerMillion),
		Valid:   true,
	}
	return inputPrice, outputPrice, estCost
}

func (s *Server) logEditorLLMUsageEvent(
	ctx context.Context,
	req llm.LLMRequest,
	requesterEmail string,
	keyMeta editorAPIKeyMeta,
	projectID string,
	domainID string,
	operation string,
	stage string,
	filePath string,
) {
	s.logEditorLLMUsageEventWithPricing(ctx, req, requesterEmail, keyMeta, projectID, domainID, operation, stage, filePath, sql.NullFloat64{}, sql.NullFloat64{}, sql.NullFloat64{})
}

func (s *Server) logEditorLLMUsageEventWithPricing(
	ctx context.Context,
	req llm.LLMRequest,
	requesterEmail string,
	keyMeta editorAPIKeyMeta,
	projectID string,
	domainID string,
	operation string,
	stage string,
	filePath string,
	inputPrice sql.NullFloat64,
	outputPrice sql.NullFloat64,
	estCost sql.NullFloat64,
) {
	if s.llmUsage == nil {
		return
	}
	requesterEmail = strings.TrimSpace(requesterEmail)
	if requesterEmail == "" {
		return
	}
	if req.Model == "" {
		req.Model = strings.TrimSpace(s.cfg.GeminiDefaultModel)
	}
	if req.TokenSource == "" {
		req.TokenSource = "estimated"
	}
	event := sqlstore.LLMUsageEvent{
		Provider:                 "gemini",
		Operation:                operation,
		Stage:                    sql.NullString{String: stage, Valid: strings.TrimSpace(stage) != ""},
		Model:                    req.Model,
		Status:                   llmUsageStatus(req.Error),
		RequesterEmail:           requesterEmail,
		KeyOwnerEmail:            sql.NullString{String: strings.TrimSpace(keyMeta.KeyOwnerEmail), Valid: strings.TrimSpace(keyMeta.KeyOwnerEmail) != ""},
		KeyType:                  sql.NullString{String: strings.TrimSpace(keyMeta.KeyType), Valid: strings.TrimSpace(keyMeta.KeyType) != ""},
		ProjectID:                sql.NullString{String: strings.TrimSpace(projectID), Valid: strings.TrimSpace(projectID) != ""},
		DomainID:                 sql.NullString{String: strings.TrimSpace(domainID), Valid: strings.TrimSpace(domainID) != ""},
		FilePath:                 sql.NullString{String: strings.TrimSpace(filePath), Valid: strings.TrimSpace(filePath) != ""},
		PromptTokens:             sql.NullInt64{Int64: req.PromptTokens, Valid: true},
		CompletionTokens:         sql.NullInt64{Int64: req.CompletionTokens, Valid: true},
		TotalTokens:              sql.NullInt64{Int64: req.TotalTokens, Valid: true},
		TokenSource:              llmTokenSource(req.TokenSource),
		InputPriceUSDPerMillion:  inputPrice,
		OutputPriceUSDPerMillion: outputPrice,
		EstimatedCostUSD:         estCost,
		ErrorMessage:             sql.NullString{String: strings.TrimSpace(req.Error), Valid: strings.TrimSpace(req.Error) != ""},
		CreatedAt:                req.Timestamp,
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	_ = s.llmUsage.CreateEvent(ctx, event)
}

func estimateLLMCostUSD(promptTokens, completionTokens int64, inputUSDPerMillion, outputUSDPerMillion float64) float64 {
	promptCost := (float64(promptTokens) / 1_000_000.0) * inputUSDPerMillion
	completionCost := (float64(completionTokens) / 1_000_000.0) * outputUSDPerMillion
	return promptCost + completionCost
}

func llmUsageStatus(errText string) string {
	if strings.TrimSpace(errText) == "" {
		return "success"
	}
	return "error"
}

func llmTokenSource(src string) string {
	src = strings.TrimSpace(strings.ToLower(src))
	switch src {
	case "provider", "estimated", "mixed":
		return src
	default:
		return "estimated"
	}
}
