package httpserver

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
)

func New(cfg config.Config, svc *auth.Service, logger *zap.SugaredLogger, projects ProjectStore, projectMembers *sqlstore.ProjectMemberStore, domains DomainStore, generations GenerationStore, prompts PromptStore, promptOverrides PromptOverrideStore, deployments DeploymentAttemptStore, schedules ScheduleStore, linkSchedules LinkScheduleStore, auditRules *sqlstore.AuditStore, siteFiles SiteFileStore, fileEdits FileEditStore, linkTasks LinkTaskStore, genQueue GenQueueStore, indexChecks IndexCheckStore, checkHistory CheckHistoryStore, llmUsage LLMUsageStore, modelPricing ModelPricingStore, legacyImports LegacyImportStore, appSettings AppSettingsStore, tq tasks.Enqueuer) *Server {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	registry := prometheus.NewRegistry()
	reqDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "auth",
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request duration in seconds",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path", "status"})
	reqCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "auth",
		Name:      "http_requests_total",
		Help:      "Total HTTP requests",
	}, []string{"method", "path", "status"})
	genStatus := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "auth",
		Name:      "generation_status_total",
		Help:      "Amount of generations grouped by status",
	}, []string{"status"})
	registry.MustRegister(reqDuration, reqCounter, genStatus)

	s := &Server{
		cfg:              cfg,
		svc:              svc,
		projects:         projects,
		projectMembers:   projectMembers,
		domains:          domains,
		generations:      generations,
		prompts:          prompts,
		promptOverrides:  promptOverrides,
		deployments:      deployments,
		schedules:        schedules,
		linkSchedules:    linkSchedules,
		auditRules:       auditRules,
		siteFiles:        siteFiles,
		fileEdits:        fileEdits,
		contentBackend:   domainfs.NewLocalFSBackend(cfg.DeployBaseDir),
		domainFilesCache: noopDomainFilesCache{},
		linkTasks:        linkTasks,
		genQueue:         genQueue,
		indexChecks:      indexChecks,
		checkHistory:     checkHistory,
		llmUsage:         llmUsage,
		modelPricing:     modelPricing,
		legacyImports:    legacyImports,
		appSettings:      appSettings,
		tasks:            tq,
		reqDuration:      reqDuration,
		reqCounter:       reqCounter,
		genStatus:        genStatus,
		registry:         registry,
		logger:           logger,
		editorCtxCache:   make(map[string]editorContextPackCacheEntry),
	}
	s.startGenerationMetricsLoop()
	return s
}

// SetAgentSessions wires in the AgentSessionStore after construction.
func (s *Server) SetAgentSessions(store AgentSessionStore) {
	s.agentSessions = store
}

// SetFileLocks wires in the FileLockStore after construction.
func (s *Server) SetFileLocks(store FileLockStore) { s.fileLocks = store }
