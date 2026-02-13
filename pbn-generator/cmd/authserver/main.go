package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/db"
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
	fileEditStore := sqlstore.NewFileEditStore(database)
	linkTaskStore := sqlstore.NewLinkTaskStore(database)
	genQueueStore := sqlstore.NewGenQueueStore(database)
	indexCheckStore := sqlstore.NewIndexCheckStore(database)
	checkHistoryStore := sqlstore.NewCheckHistoryStore(database)
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
		taskClient,
	)
	handler := srv.Handler()

	go startSessionCleanup(svc, cfg.SessionCleanInterval)

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

func startSessionCleanup(svc *auth.Service, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for now := range ticker.C {
		svc.CleanupExpired(context.Background(), now)
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
