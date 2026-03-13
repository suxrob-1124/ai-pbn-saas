package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/db"
	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/httpserver"
	"obzornik-pbn-generator/internal/notify"
	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
)

func main() {
	zl, _ := zap.NewProduction()
	defer zl.Sync()
	logger := zl.Sugar()
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		logger.Fatalf("invalid config: %v", err)
	}
	database := db.Open(cfg, logger)
	defer database.Close()

	userStore := sqlstore.NewUserStore(database)
	sessionStore := sqlstore.NewSessionStore(database)
	verificationStore := sqlstore.NewVerificationTokenStore(database)
	resetStore := sqlstore.NewResetTokenStore(database)
	captchaStore := sqlstore.NewCaptchaStore(database)
	emailChangeStore := sqlstore.NewEmailChangeStore(database)
	projectStore := sqlstore.NewProjectStore(database)
	projectMemberStore := sqlstore.NewProjectMemberStore(database)
	domainStore := sqlstore.NewDomainStore(database)
	generationStore := sqlstore.NewGenerationStore(database)
	promptStore := sqlstore.NewPromptStore(database)
	promptOverrideStore := sqlstore.NewPromptOverrideStore(database)
	deploymentStore := sqlstore.NewDeploymentAttemptStore(database)
	scheduleStore := sqlstore.NewScheduleStore(database)
	linkScheduleStore := sqlstore.NewLinkScheduleStore(database)
	auditStore := sqlstore.NewAuditStore(database)
	siteFileStore := sqlstore.NewSiteFileStore(database)
	fileEditStore := sqlstore.NewFileEditStore(database, cfg.FileRevisionMaxPerFile)
	linkTaskStore := sqlstore.NewLinkTaskStore(database)
	genQueueStore := sqlstore.NewGenQueueStore(database)
	indexCheckStore := sqlstore.NewIndexCheckStore(database)
	checkHistoryStore := sqlstore.NewCheckHistoryStore(database)
	llmUsageStore := sqlstore.NewLLMUsageStore(database)
	modelPricingStore := sqlstore.NewModelPricingStore(database)
	legacyImportStore := sqlstore.NewLegacyImportStore(database)
	appSettingsStore := sqlstore.NewAppSettingsStore(database)
	agentSessionStore := sqlstore.NewAgentSessionStore(database)
	runLogStore := sqlstore.NewScheduleRunLogStore(database)
	mailer := buildMailer(cfg, logger)
	taskClient, err := tasks.NewClient(cfg)
	if err != nil {
		logger.Fatalf("failed to init task client: %v", err)
	}
	defer taskClient.Close()

	svc := auth.NewService(auth.ServiceDeps{
		Config:             cfg,
		Users:              userStore,
		Sessions:           sessionStore,
		VerificationTokens: verificationStore,
		ResetTokens:        resetStore,
		Captchas:           captchaStore,
		EmailChanges:       emailChangeStore,
		Mailer:             mailer,
		Logger:             logger,
	})

	srv := httpserver.New(
		cfg,
		svc,
		logger,
		projectStore,
		projectMemberStore,
		domainStore,
		generationStore,
		promptStore,
		promptOverrideStore,
		deploymentStore,
		scheduleStore,
		linkScheduleStore,
		auditStore,
		siteFileStore,
		fileEditStore,
		linkTaskStore,
		genQueueStore,
		indexCheckStore,
		checkHistoryStore,
		llmUsageStore,
		modelPricingStore,
		legacyImportStore,
		appSettingsStore,
		taskClient,
	)
	contentBackend, sshPool, err := buildContentBackend(cfg)
	if err != nil {
		logger.Fatalf("failed to init content backend: %v", err)
	}
	if sshPool != nil {
		defer sshPool.Close()
	}
	srv.SetAgentSessions(agentSessionStore)
	srv.SetScheduleRunLogs(runLogStore)
	fileLockStore := sqlstore.NewFileLockStore(database)
	srv.SetFileLocks(fileLockStore)
	srv.SetContentBackend(contentBackend)
	domainFilesCacheClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer domainFilesCacheClient.Close()
	srv.SetDomainFilesRedisCache(domainFilesCacheClient)
	handler := srv.Handler()

	go startSessionCleanup(svc, cfg.SessionCleanInterval)
	go startSoftDeleteCleanup(projectStore, domainStore, cfg.SoftDeleteRetentionDays, logger)
	go startDBCleanup(dbCleanupDeps{
		siteFiles:                    siteFileStore,
		llmUsage:                     llmUsageStore,
		linkTasks:                    linkTaskStore,
		genQueue:                     genQueueStore,
		agentSessions:                agentSessionStore,
		checkHistory:                 checkHistoryStore,
		llmUsageRetentionDays:        cfg.LLMUsageRetentionDays,
		linkTaskRetentionDays:        cfg.LinkTaskRetentionDays,
		genQueueRetentionDays:        cfg.GenQueueRetentionDays,
		agentSessionRetentionDays:    cfg.AgentSessionRetentionDays,
		siteFileRetentionDays:        cfg.SoftDeleteRetentionDays,
		indexCheckHistoryKeepPerCheck: cfg.IndexCheckHistoryKeepPerCheck,
	}, logger)
	go startAgentStaleCleanup(agentSessionStore, time.Duration(cfg.AgentTimeoutSec)*time.Second, logger)
	go startFileLockCleanup(fileLockStore, logger)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	logger.Infof("starting auth server on :%s", cfg.Port)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func buildContentBackend(cfg config.Config) (domainfs.SiteContentBackend, *domainfs.SSHPool, error) {
	localBackend := domainfs.NewLocalFSBackend(cfg.DeployBaseDir)
	targetsRaw := strings.TrimSpace(cfg.DeployTargetsJSON)
	if !strings.EqualFold(strings.TrimSpace(cfg.DeployMode), "ssh_remote") && targetsRaw == "" {
		return localBackend, nil, nil
	}

	targets, err := domainfs.ParseDeployTargetsJSON(cfg.DeployTargetsJSON)
	if err != nil {
		return nil, nil, err
	}
	for alias, target := range targets {
		if strings.TrimSpace(target.KeyPath) == "" {
			target.KeyPath = strings.TrimSpace(cfg.DeploySSHKeyPath)
			targets[alias] = target
		}
	}
	if len(targets) == 0 {
		if strings.EqualFold(strings.TrimSpace(cfg.DeployMode), "ssh_remote") {
			return nil, nil, fmt.Errorf("DEPLOY_TARGETS_JSON must contain at least one target for ssh_remote")
		}
		return localBackend, nil, nil
	}

	pool, err := domainfs.NewSSHPool(domainfs.SSHPoolConfig{
		MaxOpen:        cfg.DeploySSHPoolMaxOpen,
		MaxIdle:        cfg.DeploySSHPoolMaxIdle,
		IdleTTL:        cfg.DeploySSHPoolIdleTTL,
		DialTimeout:    cfg.DeployTimeout,
		KnownHostsPath: cfg.DeployKnownHostsPath,
	})
	if err != nil {
		return nil, nil, err
	}

	sshBackend, err := domainfs.NewSSHBackend(pool, targets)
	if err != nil {
		_ = pool.Close()
		return nil, nil, err
	}
	return domainfs.NewRouterBackend(localBackend, sshBackend), pool, nil
}

func startSessionCleanup(svc *auth.Service, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for now := range ticker.C {
		svc.CleanupExpired(context.Background(), now)
	}
}

func startAgentStaleCleanup(store sqlstore.AgentSessionStore, timeout time.Duration, logger *zap.SugaredLogger) {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	// Check every minute; mark sessions running longer than their allowed timeout.
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		n, err := store.MarkStaleRunning(context.Background(), timeout+30*time.Second)
		if err != nil {
			logger.Warnf("agent stale cleanup: %v", err)
		} else if n > 0 {
			logger.Infof("agent stale cleanup: marked %d stale sessions as stopped", n)
		}
	}
}

func startFileLockCleanup(store sqlstore.FileLockStore, logger *zap.SugaredLogger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		n, err := store.PurgeExpired(context.Background())
		if err != nil {
			logger.Warnf("file lock cleanup: %v", err)
		} else if n > 0 {
			logger.Infof("file lock cleanup: purged %d expired locks", n)
		}
	}
}

type dbCleanupDeps struct {
	siteFiles                     *sqlstore.SiteFileSQLStore
	llmUsage                      *sqlstore.LLMUsageStore
	linkTasks                     sqlstore.LinkTaskStore
	genQueue                      *sqlstore.GenQueueSQLStore
	agentSessions                 sqlstore.AgentSessionStore
	checkHistory                  *sqlstore.CheckHistorySQLStore
	llmUsageRetentionDays         int
	linkTaskRetentionDays         int
	genQueueRetentionDays         int
	agentSessionRetentionDays     int
	siteFileRetentionDays         int
	indexCheckHistoryKeepPerCheck int
}

func startDBCleanup(deps dbCleanupDeps, logger *zap.SugaredLogger) {
	// Run immediately on startup, then every 6 hours.
	run := func() {
		ctx := context.Background()
		type job struct {
			name string
			fn   func() (int64, error)
		}
		jobs := []job{
			{"llm_usage_events", func() (int64, error) {
				return deps.llmUsage.PurgeOlderThan(ctx, deps.llmUsageRetentionDays)
			}},
			{"site_files (soft-deleted)", func() (int64, error) {
				return deps.siteFiles.PurgeDeletedOlderThan(ctx, deps.siteFileRetentionDays)
			}},
			{"link_tasks (completed)", func() (int64, error) {
				return deps.linkTasks.PurgeCompleted(ctx, deps.linkTaskRetentionDays)
			}},
			{"generation_queue (processed)", func() (int64, error) {
				return deps.genQueue.PurgeProcessed(ctx, deps.genQueueRetentionDays)
			}},
			{"agent_sessions (finished)", func() (int64, error) {
				return deps.agentSessions.PurgeOlderThan(ctx, deps.agentSessionRetentionDays)
			}},
			{"index_check_history (prune per check)", func() (int64, error) {
				return deps.checkHistory.PrunePerCheck(ctx, deps.indexCheckHistoryKeepPerCheck)
			}},
		}
		for _, j := range jobs {
			n, err := j.fn()
			if err != nil {
				logger.Warnf("db cleanup [%s]: %v", j.name, err)
			} else if n > 0 {
				logger.Infof("db cleanup [%s]: removed %d rows", j.name, n)
			}
		}
	}
	run()
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		run()
	}
}

func startSoftDeleteCleanup(projects *sqlstore.ProjectStore, domains *sqlstore.DomainStore, retentionDays int, logger *zap.SugaredLogger) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		ctx := context.Background()
		if n, err := domains.PurgeExpired(ctx, retentionDays); err != nil {
			logger.Warnf("trash cleanup: domains purge error: %v", err)
		} else if n > 0 {
			logger.Infof("trash cleanup: purged %d expired domains", n)
		}
		if n, err := projects.PurgeExpired(ctx, retentionDays); err != nil {
			logger.Warnf("trash cleanup: projects purge error: %v", err)
		} else if n > 0 {
			logger.Infof("trash cleanup: purged %d expired projects", n)
		}
	}
}

func buildMailer(cfg config.Config, logger *zap.SugaredLogger) notify.Mailer {
	if cfg.SMTPHost == "" || cfg.SMTPSender == "" {
		logger.Infow("smtp not configured, using noop mailer")
		return notify.NoopMailer{}
	}
	return notify.NewSMTPMailer(notify.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		User:     cfg.SMTPUser,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPSender,
		UseTLS:   cfg.SMTPUseTLS,
	})
}
