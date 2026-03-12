package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/domainfs"
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
	llmUsageStore *sqlstore.LLMUsageStore,
	modelPricingStore *sqlstore.ModelPricingStore,
	siteFileStore *sqlstore.SiteFileSQLStore,
	auditStore *sqlstore.AuditStore,
	contentBackend domainfs.SiteContentBackend,
) error {
	// Проверяем статус задачи в начале
	gen, err := genStore.Get(ctx, payload.GenerationID)
	if err != nil {
		return fmt.Errorf("failed to get generation: %w", err)
	}

	domainForStatus, domainErr := domainStore.Get(ctx, payload.DomainID)
	stableStatus := "waiting"
	if domainErr == nil {
		stableStatus = stableDomainStatus(domainForStatus)
	}

	// Нормализуем "переходные" статусы в терминальные, если задача пришла в воркер после запроса pause/cancel.
	// Это предотвращает зависание в pause_requested/cancelling при ретраях/повторной доставке.
	switch gen.Status {
	case "pause_requested":
		now := time.Now()
		if err := genStore.UpdateFull(ctx, payload.GenerationID, "paused", gen.Progress, nil, gen.Logs, gen.Artifacts, &now, nil, nil); err != nil {
			return fmt.Errorf("failed to finalize pause_requested: %w", err)
		}
		_ = domainStore.UpdateStatus(ctx, payload.DomainID, stableStatus)
		return nil
	case "cancelling":
		now := time.Now()
		if err := genStore.UpdateFull(ctx, payload.GenerationID, "cancelled", gen.Progress, nil, gen.Logs, gen.Artifacts, &now, pipeline.PtrTime(now), nil); err != nil {
			return fmt.Errorf("failed to finalize cancelling: %w", err)
		}
		_ = genStore.ClearCheckpoint(ctx, payload.GenerationID)
		_ = domainStore.UpdateStatus(ctx, payload.DomainID, stableStatus)
		return nil
	}

	// Если задача уже не в состоянии processing/pending/error, не обрабатываем
	// error разрешён — asynq может повторить задачу после сбоя пайплайна
	if gen.Status != "processing" && gen.Status != "pending" && gen.Status != "error" {
		sugar.Warnf("generation %s is in status %s, skipping", payload.GenerationID, gen.Status)
		return nil
	}

	now := time.Now()
	if gen.Status == "pending" || gen.Status == "error" {
		startProgress := gen.Progress
		if startProgress < 10 {
			startProgress = 10
		}
		_ = genStore.UpdateFull(ctx, payload.GenerationID, "processing", startProgress, nil, nil, nil, &now, nil, nil)
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

	// runCtx позволяет быстро прерывать длинный шаг по запросу pause/cancel без ожидания конца шага.
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	stopWatcher := make(chan struct{})
	var (
		stopReasonMu sync.Mutex
		stopReason   string
	)
	setStopReason := func(v string) {
		stopReasonMu.Lock()
		stopReason = strings.TrimSpace(v)
		stopReasonMu.Unlock()
	}
	getStopReason := func() string {
		stopReasonMu.Lock()
		defer stopReasonMu.Unlock()
		return stopReason
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopWatcher:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, err := genStore.Get(ctx, payload.GenerationID)
				if err != nil {
					continue
				}
				switch current.Status {
				case "pause_requested", "cancelling":
					setStopReason(current.Status)
					runCancel()
					return
				}
			}
		}
	}()
	defer close(stopWatcher)

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
	requesterEmail := project.UserEmail
	if gen.RequestedBy.Valid {
		v := strings.TrimSpace(gen.RequestedBy.String)
		if v != "" {
			requesterEmail = v
		}
	}

	// Определяем API ключ (используем глобальный из .env)
	apiKey := cfg.GeminiAPIKey
	keyType := "global"
	keyOwnerEmail := project.UserEmail
	appendLog("используется глобальный API ключ")

	if apiKey == "" {
		errMsg := "Gemini API key не настроен (GEMINI_API_KEY в .env)"
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
				genStore.UpdateFull(ctx, payload.GenerationID, gen.Status, gen.Progress, nil, logBytes, artBytes, pipeline.NullTimePtr(gen.StartedAt), pipeline.NullTimePtr(gen.FinishedAt), nil)

				// Логируем использование API ключа
				var totalTokens int64
				for _, req := range llmRequests {
					totalTokens += req.TotalTokens
				}
				if err := apiKeyUsageStore.LogUsage(ctx, keyOwnerEmail, payload.GenerationID, keyType, len(llmRequests), totalTokens); err != nil {
					sugar.Warnf("failed to log API key usage: %v", err)
				}
				if llmUsageStore != nil {
					for _, req := range llmRequests {
						var (
							inputPrice  sql.NullFloat64
							outputPrice sql.NullFloat64
							estCost     sql.NullFloat64
						)
						if modelPricingStore != nil {
							at := req.Timestamp
							if at.IsZero() {
								at = time.Now().UTC()
							}
							if pricing, err := modelPricingStore.GetActiveByModel(ctx, "gemini", req.Model, at); err == nil && pricing != nil {
								inputPrice = sql.NullFloat64{Float64: pricing.InputUSDPerMillion, Valid: true}
								outputPrice = sql.NullFloat64{Float64: pricing.OutputUSDPerMillion, Valid: true}
								cost := estimateLLMCostUSD(req.PromptTokens, req.CompletionTokens, pricing.InputUSDPerMillion, pricing.OutputUSDPerMillion)
								estCost = sql.NullFloat64{Float64: cost, Valid: true}
							}
						}
						stage := sql.NullString{String: req.Stage, Valid: strings.TrimSpace(req.Stage) != ""}
						operation := "generation_step"
						if strings.TrimSpace(req.Stage) != "" {
							operation = "generation_step/" + req.Stage
						}
						event := sqlstore.LLMUsageEvent{
							Provider:                 "gemini",
							Operation:                operation,
							Stage:                    stage,
							Model:                    req.Model,
							Status:                   llmUsageStatus(req.Error),
							RequesterEmail:           requesterEmail,
							KeyOwnerEmail:            sql.NullString{String: keyOwnerEmail, Valid: strings.TrimSpace(keyOwnerEmail) != ""},
							KeyType:                  sql.NullString{String: keyType, Valid: strings.TrimSpace(keyType) != ""},
							ProjectID:                sql.NullString{String: project.ID, Valid: project.ID != ""},
							DomainID:                 sql.NullString{String: domain.ID, Valid: domain.ID != ""},
							GenerationID:             sql.NullString{String: payload.GenerationID, Valid: payload.GenerationID != ""},
							PromptTokens:             sql.NullInt64{Int64: req.PromptTokens, Valid: true},
							CompletionTokens:         sql.NullInt64{Int64: req.CompletionTokens, Valid: true},
							TotalTokens:              sql.NullInt64{Int64: req.TotalTokens, Valid: true},
							TokenSource:              llmTokenSource(req.TokenSource),
							InputPriceUSDPerMillion:  inputPrice,
							OutputPriceUSDPerMillion: outputPrice,
							EstimatedCostUSD:         estCost,
							ErrorMessage:             sql.NullString{String: req.Error, Valid: strings.TrimSpace(req.Error) != ""},
							CreatedAt:                req.Timestamp,
						}
						if event.CreatedAt.IsZero() {
							event.CreatedAt = time.Now().UTC()
						}
						if err := llmUsageStore.CreateEvent(ctx, event); err != nil {
							sugar.Warnf("failed to log llm usage event: %v", err)
						}
					}
				}
			}
		}
		llmClient.Reset()
	}()

	scopedPrompts := newScopedPromptManager(promptStore, promptOverrideStore, domain.ID, project.ID)
	deployMode := "ssh_remote" // Теперь по умолчанию боевой режим
	if mode := strings.ToLower(strings.TrimSpace(cfg.DeployMode)); mode == "local_mock" {
		// Переходим в мок, ТОЛЬКО если в .env явно сказано local_mock
		deployMode = "local_mock"
	}
	currentPublisher := publisher.Publisher(publisher.NewLocalPublisher(cfg.DeployBaseDir, domainStore, siteFileStore))
	if strings.EqualFold(strings.TrimSpace(deployMode), "ssh_remote") {
		if contentBackend == nil {
			return fmt.Errorf("ssh publisher backend is not configured")
		}
		currentPublisher = publisher.NewSSHPublisher(contentBackend, domainStore, siteFileStore)
	}

	// Создаем pipeline state
	// Подготовим артефакты, загруженные из БД (для skip/restore)
	existingArtifacts := make(map[string]any)
	if len(gen.Artifacts) > 0 {
		_ = json.Unmarshal(gen.Artifacts, &existingArtifacts)
	}

	// Determine effective generation type: payload > generation record > domain > default
	effectiveGenType := payload.GenerationType
	if effectiveGenType == "" {
		effectiveGenType = gen.GenerationType
	}
	if effectiveGenType == "" {
		effectiveGenType = domain.GenerationType
	}
	if effectiveGenType == "" {
		effectiveGenType = pipeline.GenTypeSinglePage
	}
	appendLog(fmt.Sprintf("generation type: %s", effectiveGenType))

	state := &pipeline.PipelineState{
		GenerationID:      payload.GenerationID,
		DomainID:          payload.DomainID,
		GenerationType:    effectiveGenType,
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
		Publisher:         currentPublisher,
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
	// Общие шаги для всех типов генерации (от SERP до Publish)
	commonSteps := []pipeline.Step{
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

	var steps []pipeline.Step
	switch effectiveGenType {
	case pipeline.GenTypeWebarchiveSingle:
		// Webarchive: fetch snapshots → generate keyword → then standard pipeline
		steps = append([]pipeline.Step{
			&pipeline.WaybackFetchStep{},
			&pipeline.KeywordGenerationStep{},
		}, commonSteps...)
	default:
		// single_page and fallback: standard pipeline
		steps = commonSteps
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
	if err := generationPipeline.Run(runCtx); err != nil {
		if errors.Is(err, context.Canceled) {
			reason := getStopReason()
			if reason == "" {
				if latest, gerr := genStore.Get(ctx, payload.GenerationID); gerr == nil {
					reason = strings.TrimSpace(latest.Status)
				}
			}
			switch reason {
			case "pause_requested":
				appendLog("Pipeline paused by request")
				logBytes, _ := json.Marshal(logs)
				gen, _ := genStore.Get(ctx, payload.GenerationID)
				_ = genStore.UpdateFull(ctx, payload.GenerationID, "paused", gen.Progress, nil, logBytes, gen.Artifacts, &now, nil, nil)
				_ = domainStore.UpdateStatus(ctx, payload.DomainID, stableDomainStatus(domain))
				return nil
			case "cancelling":
				appendLog("Pipeline cancelled by request")
				logBytes, _ := json.Marshal(logs)
				gen, _ := genStore.Get(ctx, payload.GenerationID)
				_ = genStore.UpdateFull(ctx, payload.GenerationID, "cancelled", gen.Progress, nil, logBytes, gen.Artifacts, &now, pipeline.PtrTime(time.Now()), nil)
				_ = genStore.ClearCheckpoint(ctx, payload.GenerationID)
				_ = domainStore.UpdateStatus(ctx, payload.DomainID, stableDomainStatus(domain))
				return nil
			}
		}
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
		// Если статус уже зафиксирован как paused/cancelled (например, через API recovery),
		// не переводим задачу в error даже если pipeline вернул служебную ошибку.
		if latest, gerr := genStore.Get(ctx, payload.GenerationID); gerr == nil {
			switch strings.TrimSpace(latest.Status) {
			case "paused", "cancelled":
				_ = domainStore.UpdateStatus(ctx, payload.DomainID, stableDomainStatus(domain))
				return nil
			}
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

func isRetryableGenerationError(errMsg string) bool {
	if errMsg == "" {
		return false
	}
	msg := strings.ToLower(errMsg)
	if strings.Contains(msg, "brand policy violation") {
		return false
	}
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
