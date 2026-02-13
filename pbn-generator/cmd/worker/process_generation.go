package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/crypto/secretbox"
	"obzornik-pbn-generator/internal/llm"
	"obzornik-pbn-generator/internal/publisher"
	"obzornik-pbn-generator/internal/retry"
	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
	"obzornik-pbn-generator/internal/worker/pipeline"
)

// processGeneration обрабатывает задачу генерации используя pipeline
func processGeneration(
	ctx context.Context,
	payload tasks.GeneratePayload,
	cfg config.Config,
	sugar *zap.SugaredLogger,
	domainStore *sqlstore.DomainStore,
	genStore *sqlstore.GenerationStore,
	promptStore *sqlstore.PromptStore,
	promptOverrideStore *sqlstore.PromptOverrideStore,
	projectStore *sqlstore.ProjectStore,
	linkScheduleStore *sqlstore.LinkScheduleStore,
	deploymentStore *sqlstore.DeploymentAttemptStore,
	userStore *sqlstore.UserStore,
	apiKeyUsageStore *sqlstore.APIKeyUsageStore,
	siteFileStore *sqlstore.SiteFileSQLStore,
	auditStore *sqlstore.AuditStore,
) error {
	// Проверяем статус задачи в начале
	gen, err := genStore.Get(ctx, payload.GenerationID)
	if err != nil {
		return fmt.Errorf("failed to get generation: %w", err)
	}

	// Если задача уже не в состоянии processing, не обрабатываем
	if gen.Status != "processing" && gen.Status != "pending" {
		sugar.Warnf("generation %s is in status %s, skipping", payload.GenerationID, gen.Status)
		return nil
	}

	now := time.Now()
	if gen.Status == "pending" {
		_ = genStore.UpdateFull(ctx, payload.GenerationID, "processing", 10, nil, nil, nil, &now, nil, nil)
	}

	// Debug: сырой artifacts из БД
	if len(gen.Artifacts) > 0 {
		sugar.Infow("loaded generation artifacts (raw)", "generation", payload.GenerationID, "raw", string(gen.Artifacts))
	}

	// Инициализация логирования с мгновенным сохранением
	logs := []string{}
	var (
		logMu         sync.Mutex
		lastFlush     time.Time
		flushInterval = 2 * time.Second
	)
	flushLogs := func(force bool) {
		logMu.Lock()
		defer logMu.Unlock()
		if !force && time.Since(lastFlush) < flushInterval {
			return
		}
		if len(logs) == 0 {
			return
		}
		b, err := json.Marshal(logs)
		if err != nil {
			return
		}
		if err := genStore.UpdateLogs(ctx, payload.GenerationID, b); err != nil {
			sugar.Warnf("failed to persist logs: %v", err)
			return
		}
		lastFlush = time.Now()
	}
	appendLog := func(msg string) {
		logMu.Lock()
		line := time.Now().Format(time.RFC3339) + " " + msg
		logs = append(logs, line)
		needFlush := time.Since(lastFlush) >= flushInterval
		logMu.Unlock()
		if needFlush {
			go flushLogs(false)
		}
	}
	defer flushLogs(true)

	// Загружаем domain и project
	domain, err := domainStore.Get(ctx, payload.DomainID)
	if err != nil {
		errMsg := "domain not found"
		_ = genStore.UpdateStatus(ctx, payload.GenerationID, "error", 0, &errMsg)
		_ = domainStore.UpdateStatus(ctx, payload.DomainID, "waiting")
		return fmt.Errorf("domain not found: %w", err)
	}
	appendLog("start for domain " + domain.URL)

	// Если домен уже публиковался и есть link-настройки, помечаем, что нужна повторная вставка ссылок.
	if domain.LinkAnchorText.Valid && domain.LinkAcceptorURL.Valid {
		anchor := strings.TrimSpace(domain.LinkAnchorText.String)
		target := strings.TrimSpace(domain.LinkAcceptorURL.String)
		if anchor != "" && target != "" {
			if err := domainStore.UpdateLinkStatus(ctx, domain.ID, "needs_relink"); err != nil && sugar != nil {
				sugar.Warnf("failed to mark needs_relink for domain %s: %v", domain.ID, err)
			}
		}
	}

	project, err := projectStore.GetByID(ctx, domain.ProjectID)
	if err != nil {
		errMsg := "project not found"
		_ = genStore.UpdateStatus(ctx, payload.GenerationID, "error", 0, &errMsg)
		_ = domainStore.UpdateStatus(ctx, payload.DomainID, stableDomainStatus(domain))
		return fmt.Errorf("project not found: %w", err)
	}

	// Определяем API ключ
	apiKey := cfg.GeminiAPIKey
	keyType := "global"
	keyOwnerEmail := project.UserEmail

	if gen.RequestedBy.Valid {
		requested := strings.ToLower(strings.TrimSpace(gen.RequestedBy.String))
		if requested != "" {
			if u, err := userStore.Get(ctx, requested); err == nil && strings.EqualFold(u.Role, "admin") {
				keyOwnerEmail = requested
			}
		}
	}

	userAPIKeyEnc, _, err := userStore.GetAPIKey(ctx, keyOwnerEmail)
	if err == nil && len(userAPIKeyEnc) > 0 {
		keySecret := secretbox.DeriveKey(cfg.APIKeySecret)
		decrypted, err := secretbox.Decrypt(keySecret, userAPIKeyEnc)
		if err == nil {
			apiKey = string(decrypted)
			keyType = "user"
			appendLog(fmt.Sprintf("используется API ключ пользователя %s", keyOwnerEmail))
		} else {
			appendLog("предупреждение: не удалось расшифровать API ключ пользователя, используется глобальный")
		}
	} else if keyOwnerEmail != project.UserEmail {
		// fallback на владельца проекта, если админский ключ отсутствует
		userAPIKeyEnc, _, err := userStore.GetAPIKey(ctx, project.UserEmail)
		if err == nil && len(userAPIKeyEnc) > 0 {
			keySecret := secretbox.DeriveKey(cfg.APIKeySecret)
			decrypted, err := secretbox.Decrypt(keySecret, userAPIKeyEnc)
			if err == nil {
				apiKey = string(decrypted)
				keyType = "user"
				keyOwnerEmail = project.UserEmail
				appendLog(fmt.Sprintf("используется API ключ пользователя %s", keyOwnerEmail))
			} else {
				appendLog("предупреждение: не удалось расшифровать API ключ пользователя, используется глобальный")
			}
		} else {
			appendLog("используется глобальный API ключ")
		}
	} else {
		appendLog("используется глобальный API ключ")
	}

	if apiKey == "" {
		errMsg := "Gemini API key не настроен (ни глобальный, ни пользовательский)"
		appendLog(errMsg)
		logBytes, _ := json.Marshal(logs)
		_ = genStore.UpdateFull(ctx, payload.GenerationID, "error", 0, &errMsg, logBytes, nil, &now, pipeline.PtrTime(time.Now()), nil)
		_ = domainStore.UpdateStatus(ctx, payload.DomainID, stableDomainStatus(domain))
		return fmt.Errorf("API key not configured")
	}

	// Инициализация сервисов
	llmCfg := llm.Config{
		APIKey:          apiKey,
		DefaultModel:    cfg.GeminiDefaultModel,
		MaxRetries:      cfg.GeminiMaxRetries,
		RetryDelay:      cfg.GeminiRetryDelay,
		RequestTimeout:  cfg.GeminiRequestTimeout,
		RateLimitPerMin: cfg.GeminiRateLimitPerMin,
	}
	llmClient := llm.NewClient(llmCfg)

	// Defer для сохранения LLM запросов
	defer func() {
		llmRequests := llmClient.GetRequests()
		if len(llmRequests) > 0 {
			gen, err := genStore.Get(ctx, payload.GenerationID)
			if err == nil {
				var currentArtifacts map[string]any
				if len(gen.Artifacts) > 0 {
					json.Unmarshal(gen.Artifacts, &currentArtifacts)
				}
				if currentArtifacts == nil {
					currentArtifacts = make(map[string]any)
				}
				currentArtifacts["llm_requests"] = llmRequests
				artBytes, _ := json.Marshal(currentArtifacts)
				logBytes, _ := json.Marshal(logs)
				genStore.UpdateFull(ctx, payload.GenerationID, gen.Status, gen.Progress, nil, logBytes, artBytes, nil, nil, nil)

				// Логируем использование API ключа
				var totalTokens int64
				for _, req := range llmRequests {
					totalTokens += req.TokensUsed
				}
				if err := apiKeyUsageStore.LogUsage(ctx, keyOwnerEmail, payload.GenerationID, keyType, len(llmRequests), totalTokens); err != nil {
					sugar.Warnf("failed to log API key usage: %v", err)
				}
			}
		}
		llmClient.Reset()
	}()

	scopedPrompts := newScopedPromptManager(promptStore, promptOverrideStore, domain.ID, project.ID)
	deployMode := "local_mock"
	if mode := strings.TrimSpace(domain.DeploymentMode.String); mode != "" {
		deployMode = mode
	}

	// Создаем pipeline state
	// Подготовим артефакты, загруженные из БД (для skip/restore)
	existingArtifacts := make(map[string]any)
	if len(gen.Artifacts) > 0 {
		_ = json.Unmarshal(gen.Artifacts, &existingArtifacts)
	}
	if len(existingArtifacts) > 0 {
		keys := make([]string, 0, len(existingArtifacts))
		for k := range existingArtifacts {
			keys = append(keys, k)
		}
		sugar.Infow("artifacts keys before pipeline", "generation", payload.GenerationID, "keys", keys)
	}

	state := &pipeline.PipelineState{
		GenerationID:      payload.GenerationID,
		DomainID:          payload.DomainID,
		ForceStep:         payload.ForceStep,
		DomainStore:       domainStore,
		GenerationStore:   genStore,
		PromptStore:       promptStore,
		ProjectStore:      projectStore,
		LinkScheduleStore: linkScheduleStore,
		AuditStore:        auditStore,
		PromptOverrides:   promptOverrideStore,
		Deployments:       deploymentStore,
		LLMClient:         pipeline.NewLLMClient(llmClient),
		PromptManager:     scopedPrompts,
		Analyzer:          pipeline.NewAnalyzer(),
		Publisher:         publisher.NewLocalPublisher(cfg.DeployBaseDir, domainStore, siteFileStore),
		DeploymentMode:    deployMode,
		DefaultModel:      cfg.GeminiDefaultModel,
		Domain:            &domain,
		Project:           &project,
		Artifacts:         existingArtifacts,
		AppendLog:         appendLog,
		Logs:              &logs,
		Context:           make(map[string]any),
	}

	// Создаем шаги pipeline
	steps := []pipeline.Step{
		&pipeline.SERPAnalysisStep{},
		&pipeline.CompetitorAnalysisStep{},
		&pipeline.TechnicalSpecStep{},
		&pipeline.ContentGenerationStep{},
		&pipeline.DesignArchitectureStep{},
		&pipeline.LogoGenerationStep{},
		&pipeline.HTMLGenerationStep{},
		&pipeline.CSSGenerationStep{},
		&pipeline.JSGenerationStep{},
		&pipeline.ImageGenerationStep{},
		&pipeline.Page404GenerationStep{},
		&pipeline.AssemblyStep{},
		&pipeline.AuditStep{},
		&pipeline.PublishStep{},
	}

	// Создаем и запускаем pipeline
	generationPipeline := pipeline.NewPipeline(steps, state)
	generationPipeline.CheckStatus = func() (string, error) {
		g, err := genStore.Get(ctx, payload.GenerationID)
		if err != nil {
			return "", err
		}
		return g.Status, nil
	}
	if err := generationPipeline.Run(ctx); err != nil {
		if err == pipeline.ErrPipelinePaused {
			appendLog("Pipeline paused by request")
			logBytes, _ := json.Marshal(logs)
			gen, _ := genStore.Get(ctx, payload.GenerationID)
			_ = genStore.UpdateFull(ctx, payload.GenerationID, "paused", gen.Progress, nil, logBytes, gen.Artifacts, &now, nil, nil)
			_ = domainStore.UpdateStatus(ctx, payload.DomainID, stableDomainStatus(domain))
			return nil
		}
		if err == pipeline.ErrPipelineCancelled {
			appendLog("Pipeline cancelled by request")
			logBytes, _ := json.Marshal(logs)
			gen, _ := genStore.Get(ctx, payload.GenerationID)
			_ = genStore.UpdateFull(ctx, payload.GenerationID, "cancelled", gen.Progress, nil, logBytes, gen.Artifacts, &now, pipeline.PtrTime(time.Now()), nil)
			_ = domainStore.UpdateStatus(ctx, payload.DomainID, stableDomainStatus(domain))
			return nil
		}
		// Обрабатываем ошибки паузы/отмены отдельно
		if err.Error() == "generation paused" || err.Error() == "generation cancelled" {
			return nil // Это не ошибка, а нормальное завершение
		}

		errMsg := err.Error()
		appendLog(fmt.Sprintf("Pipeline failed: %s", errMsg))
		gen, _ := genStore.Get(ctx, payload.GenerationID)
		progress := gen.Progress
		if progress <= 0 {
			progress = 1
		}
		artifacts := gen.Artifacts
		if len(artifacts) == 0 && len(state.Artifacts) > 0 {
			if artBytes, _ := json.Marshal(state.Artifacts); len(artBytes) > 0 {
				artifacts = artBytes
			}
		}
		attempts := gen.Attempts + 1
		retryable := isRetryableGenerationError(errMsg)
		nextRetryAt := sql.NullTime{}
		lastErrorAt := sql.NullTime{Time: time.Now(), Valid: true}
		if retryable {
			if next, rerr := retry.NextRetryAt(attempts, gen.CreatedAt, time.Now()); rerr == nil {
				nextRetryAt = sql.NullTime{Time: next, Valid: true}
				appendLog(fmt.Sprintf("retry scheduled at %s (attempt %d/%d)", next.UTC().Format(time.RFC3339), attempts, retry.MaxRetries))
			} else {
				retryable = false
				appendLog(fmt.Sprintf("retry exhausted: %v", rerr))
			}
		}
		logBytes, _ := json.Marshal(logs)
		_ = genStore.UpdateFull(ctx, payload.GenerationID, "error", progress, &errMsg, logBytes, artifacts, &now, pipeline.PtrTime(time.Now()), nil)
		_ = genStore.UpdateRetry(ctx, payload.GenerationID, attempts, retryable, nextRetryAt, lastErrorAt)
		_ = domainStore.UpdateStatus(ctx, payload.DomainID, stableDomainStatus(domain))
		return err
	}

	// Финальное сохранение
	fin := time.Now()
	appendLog("Генерация завершена успешно")
	logBytes, _ := json.Marshal(logs)

	// Получаем финальные artifacts
	gen, _ = genStore.Get(ctx, payload.GenerationID)
	var finalArtifacts []byte
	if len(gen.Artifacts) > 0 {
		finalArtifacts = gen.Artifacts
	} else {
		artBytes, _ := json.Marshal(state.Artifacts)
		finalArtifacts = artBytes
	}

	// Финальное обновление статуса
	_ = genStore.UpdateFull(ctx, payload.GenerationID, "success", 100, nil, logBytes, finalArtifacts, &now, &fin, nil)
	_ = genStore.UpdateRetry(ctx, payload.GenerationID, gen.Attempts, false, sql.NullTime{}, sql.NullTime{})
	summaryMap := buildGenerationArtifactsSummary(state.Artifacts, scopedPrompts.Trace(), fin)
	if summaryBytes, err := json.Marshal(summaryMap); err == nil {
		_ = genStore.UpdateArtifactsSummary(ctx, payload.GenerationID, summaryBytes)
	}
	_ = domainStore.SetLastSuccessGeneration(ctx, payload.DomainID, payload.GenerationID)
	_ = domainStore.UpdateStatus(ctx, payload.DomainID, "published")

	return nil
}

func isRetryableGenerationError(errMsg string) bool {
	if errMsg == "" {
		return false
	}
	msg := strings.ToLower(errMsg)
	if strings.Contains(msg, "context deadline exceeded") {
		return true
	}
	if strings.Contains(msg, "client.timeout") {
		return true
	}
	if strings.Contains(msg, "timeout") && strings.Contains(msg, "awaiting headers") {
		return true
	}
	if strings.Contains(msg, "status 429") || strings.Contains(msg, "resource_exhausted") {
		return true
	}
	if strings.Contains(msg, "unexpected eof") || strings.Contains(msg, "eof") {
		return true
	}
	if strings.Contains(msg, "connection reset by peer") {
		return true
	}
	if strings.Contains(msg, "connection refused") {
		return true
	}
	if strings.Contains(msg, "tls handshake timeout") {
		return true
	}
	if strings.Contains(msg, "i/o timeout") {
		return true
	}
	if strings.Contains(msg, "server closed idle connection") {
		return true
	}
	if code, ok := extractSerpStatusCode(msg); ok {
		if code == 429 {
			return true
		}
		if code >= 500 && code <= 599 {
			return true
		}
		return false
	}
	return false
}

func extractSerpStatusCode(msg string) (int, bool) {
	const prefix = "serp status "
	idx := strings.Index(msg, prefix)
	if idx == -1 {
		return 0, false
	}
	rest := msg[idx+len(prefix):]
	code := 0
	i := 0
	for i < len(rest) {
		ch := rest[i]
		if ch < '0' || ch > '9' {
			break
		}
		code = code*10 + int(ch-'0')
		i++
	}
	if i == 0 {
		return 0, false
	}
	return code, true
}
