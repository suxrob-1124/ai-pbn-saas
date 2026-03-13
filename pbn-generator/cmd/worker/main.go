package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/db"
	"obzornik-pbn-generator/internal/importer/legacy"
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
	fileEditStore := sqlstore.NewFileEditStore(dbConn, cfg.FileRevisionMaxPerFile)
	linkTaskStore := sqlstore.NewLinkTaskStore(dbConn)
	indexCheckStore := sqlstore.NewIndexCheckStore(dbConn)
	checkHistoryStore := sqlstore.NewCheckHistoryStore(dbConn)
	auditStore := sqlstore.NewAuditStore(dbConn)
	publishContentBackend, sshPool, sshBackend, err := buildPublishContentBackend(cfg)
	if err != nil {
		sugar.Fatalf("failed to init publish content backend: %v", err)
	}
	if sshPool != nil {
		defer sshPool.Close()
	}

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
			publishContentBackend,
		)
	})
	linkWorker := &worker.LinkWorker{
		BaseDir:        cfg.DeployBaseDir,
		Config:         cfg,
		Logger:         sugar,
		Tasks:          linkTaskStore,
		Domains:        domainStore,
		Projects:       projectStore,
		Users:          userStore,
		SiteFiles:      siteFileStore,
		FileEdits:      fileEditStore,
		LLMUsage:       llmUsageStore,
		Pricing:        modelPricingStore,
		ContentBackend: publishContentBackend,
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

	// Legacy import handler
	legacyImportStore := sqlstore.NewLegacyImportStore(dbConn)
	if sshBackend != nil {
		webImportSvc := legacy.NewWebImportService(
			domainStore,
			siteFileStore,
			linkTaskStore,
			genStore,
			promptStore,
			sshBackend,
			legacyImportStore,
		)
		mux.HandleFunc(tasks.TaskLegacyImport, func(ctx context.Context, t *asynq.Task) error {
			payload, err := tasks.ParseLegacyImportPayload(t)
			if err != nil {
				return err
			}
			item, err := legacyImportStore.GetItem(ctx, payload.ItemID)
			if err != nil {
				return fmt.Errorf("load import item: %w", err)
			}
			domain, err := domainStore.Get(ctx, payload.DomainID)
			if err != nil {
				return fmt.Errorf("load domain: %w", err)
			}
			job, err := legacyImportStore.GetJob(ctx, payload.JobID)
			if err != nil {
				return fmt.Errorf("load import job: %w", err)
			}
			if job.Status == "pending" {
				_ = legacyImportStore.UpdateJobStarted(ctx, job.ID)
			}

			processErr := webImportSvc.ProcessSingleDomain(ctx, job.ID, item.ID, domain, job.RequestedBy, job.ForceOverwrite)

			if processErr != nil {
				_ = legacyImportStore.IncrementJobFailed(ctx, job.ID)
			} else {
				_ = legacyImportStore.IncrementJobCompleted(ctx, job.ID)
			}

			// Check if all items are processed and finalize the job.
			updatedJob, err := legacyImportStore.GetJob(ctx, job.ID)
			if err == nil {
				processed := updatedJob.CompletedItems + updatedJob.FailedItems + updatedJob.SkippedItems
				if processed >= updatedJob.TotalItems {
					status := "completed"
					if updatedJob.FailedItems > 0 && updatedJob.CompletedItems == 0 {
						status = "failed"
					}
					_ = legacyImportStore.UpdateJobFinished(ctx, job.ID, status, nil)
				}
			}

			return processErr
		})
		sugar.Info("legacy import handler registered")
	} else {
		sugar.Warn("SSH backend not configured; legacy import handler not registered")
	}

	// Запускаем cleanup worker в отдельной горутине
	go runCleanupWorker(context.Background(), cfg, genStore, fileEditStore, sugar)

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
func runCleanupWorker(ctx context.Context, cfg config.Config, genStore *sqlstore.GenerationStore, fileEditStore *sqlstore.FileEditSQLStore, logger *zap.SugaredLogger) {
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
	runCleanup(ctx, cfg, genStore, fileEditStore, logger)

	// Затем запускаем каждые 24 часа
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("cleanup worker stopped")
			return
		case <-ticker.C:
			runCleanup(ctx, cfg, genStore, fileEditStore, logger)
		}
	}
}

// runCleanup выполняет очистку старых генераций
func runCleanup(ctx context.Context, cfg config.Config, genStore *sqlstore.GenerationStore, fileEditStore *sqlstore.FileEditSQLStore, logger *zap.SugaredLogger) {
	logger.Info("starting cleanup")
	startTime := time.Now()

	// Удаляем генерации со статусами cancelled и error старше ArtifactRetentionDays дней
	statuses := []string{"cancelled", "error"}
	deletedGens, err := genStore.DeleteOldGenerations(ctx, cfg.ArtifactRetentionDays, statuses)
	if err != nil {
		logger.Errorf("generation cleanup failed: %v", err)
	} else {
		logger.Infof("generation cleanup: deleted %d older than %d days (statuses: %v)",
			deletedGens, cfg.ArtifactRetentionDays, statuses)
	}

	// Удаляем старые ревизии файлов, оставляя keepPerFile новейших
	if cfg.FileRevisionMaxPerFile > 0 {
		prunedRevs, err := fileEditStore.PruneOldRevisions(ctx, cfg.FileRevisionMaxPerFile)
		if err != nil {
			logger.Errorf("revision pruning failed: %v", err)
		} else if prunedRevs > 0 {
			logger.Infof("revision pruning: deleted %d old revisions (keeping %d per file)",
				prunedRevs, cfg.FileRevisionMaxPerFile)
		}
	}

	logger.Infof("cleanup completed in %v", time.Since(startTime))
}
