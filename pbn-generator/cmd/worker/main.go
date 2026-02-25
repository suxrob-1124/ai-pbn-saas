package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/db"
	"obzornik-pbn-generator/internal/indexchecker"
	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
	"obzornik-pbn-generator/internal/worker"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()

	dbConn := db.Open(cfg, sugar)
	defer dbConn.Close()

	domainStore := sqlstore.NewDomainStore(dbConn)
	genStore := sqlstore.NewGenerationStore(dbConn)
	promptStore := sqlstore.NewPromptStore(dbConn)
	promptOverrideStore := sqlstore.NewPromptOverrideStore(dbConn)
	projectStore := sqlstore.NewProjectStore(dbConn)
	linkScheduleStore := sqlstore.NewLinkScheduleStore(dbConn)
	deploymentStore := sqlstore.NewDeploymentAttemptStore(dbConn)
	userStore := sqlstore.NewUserStore(dbConn)
	apiKeyUsageStore := sqlstore.NewAPIKeyUsageStore(dbConn)
	llmUsageStore := sqlstore.NewLLMUsageStore(dbConn)
	modelPricingStore := sqlstore.NewModelPricingStore(dbConn)
	siteFileStore := sqlstore.NewSiteFileStore(dbConn)
	fileEditStore := sqlstore.NewFileEditStore(dbConn)
	linkTaskStore := sqlstore.NewLinkTaskStore(dbConn)
	indexCheckStore := sqlstore.NewIndexCheckStore(dbConn)
	checkHistoryStore := sqlstore.NewCheckHistoryStore(dbConn)
	auditStore := sqlstore.NewAuditStore(dbConn)

	server := tasks.NewServer(cfg, 4, true, true)
	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TaskGenerate, func(ctx context.Context, t *asynq.Task) error {
		payload, err := tasks.ParseGeneratePayload(t)
		if err != nil {
			return err
		}

		// Используем новый pipeline-based обработчик
		return processGeneration(
			ctx,
			payload,
			cfg,
			sugar,
			domainStore,
			genStore,
			promptStore,
			promptOverrideStore,
			projectStore,
			linkScheduleStore,
			deploymentStore,
			userStore,
			apiKeyUsageStore,
			llmUsageStore,
			modelPricingStore,
			siteFileStore,
			auditStore,
		)
	})
	linkWorker := &worker.LinkWorker{
		BaseDir:   cfg.DeployBaseDir,
		Config:    cfg,
		Logger:    sugar,
		Tasks:     linkTaskStore,
		Domains:   domainStore,
		Projects:  projectStore,
		Users:     userStore,
		SiteFiles: siteFileStore,
		FileEdits: fileEditStore,
		LLMUsage:  llmUsageStore,
		Pricing:   modelPricingStore,
	}
	mux.HandleFunc(tasks.TaskProcessLink, func(ctx context.Context, t *asynq.Task) error {
		payload, err := tasks.ParseLinkTaskPayload(t)
		if err != nil {
			return err
		}
		return linkWorker.ProcessTask(ctx, payload.TaskID)
	})
	indexChecker := &indexchecker.SerpChecker{}
	mux.HandleFunc(tasks.TaskProcessIndex, func(ctx context.Context, t *asynq.Task) error {
		payload, err := tasks.ParseIndexCheckPayload(t)
		if err != nil {
			return err
		}
		_, err = indexchecker.ProcessCheckByID(
			ctx,
			time.Now().UTC(),
			payload.CheckID,
			projectStore,
			domainStore,
			indexCheckStore,
			checkHistoryStore,
			indexChecker,
			sugar,
		)
		return err
	})

	// Запускаем cleanup worker в отдельной горутине
	go runCleanupWorker(context.Background(), cfg, genStore, sugar)

	sugar.Infof("starting worker on redis %s", cfg.RedisAddr)
	if err := server.Run(mux); err != nil {
		sugar.Fatalf("worker stopped: %v", err)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// runCleanupWorker периодически запускает очистку старых артефактов
// Запускается раз в день в 3:00 UTC
func runCleanupWorker(ctx context.Context, cfg config.Config, genStore *sqlstore.GenerationStore, logger *zap.SugaredLogger) {
	// Вычисляем время до следующего запуска (3:00 UTC)
	now := time.Now().UTC()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, time.UTC)
	if nextRun.Before(now) || nextRun.Equal(now) {
		// Если уже прошло 3:00, запускаем на следующий день
		nextRun = nextRun.Add(24 * time.Hour)
	}
	initialDelay := nextRun.Sub(now)
	logger.Infof("cleanup worker will start in %v (at %s UTC)", initialDelay, nextRun.Format("2006-01-02 15:04:05"))

	// Ждем до первого запуска
	time.Sleep(initialDelay)

	// Запускаем cleanup сразу при старте (если прошло время)
	runCleanup(ctx, cfg, genStore, logger)

	// Затем запускаем каждые 24 часа
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("cleanup worker stopped")
			return
		case <-ticker.C:
			runCleanup(ctx, cfg, genStore, logger)
		}
	}
}

// runCleanup выполняет очистку старых генераций
func runCleanup(ctx context.Context, cfg config.Config, genStore *sqlstore.GenerationStore, logger *zap.SugaredLogger) {
	logger.Info("starting cleanup of old generations")
	startTime := time.Now()

	// Удаляем генерации со статусами cancelled и error старше ArtifactRetentionDays дней
	statuses := []string{"cancelled", "error"}
	deletedCount, err := genStore.DeleteOldGenerations(ctx, cfg.ArtifactRetentionDays, statuses)
	if err != nil {
		logger.Errorf("cleanup failed: %v", err)
		return
	}

	duration := time.Since(startTime)
	logger.Infof("cleanup completed: deleted %d generations older than %d days (statuses: %v) in %v",
		deletedCount, cfg.ArtifactRetentionDays, statuses, duration)
}
