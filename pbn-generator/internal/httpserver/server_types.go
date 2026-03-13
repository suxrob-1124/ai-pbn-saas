package httpserver

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
)

type ProjectStore interface {
	Create(ctx context.Context, p sqlstore.Project) error
	ListByUser(ctx context.Context, email string) ([]sqlstore.Project, error)
	ListAll(ctx context.Context) ([]sqlstore.Project, error)
	Get(ctx context.Context, id, email string) (sqlstore.Project, error)
	GetByID(ctx context.Context, id string) (sqlstore.Project, error)
	Update(ctx context.Context, p sqlstore.Project) error
	UpdateIndexCheckEnabled(ctx context.Context, id string, enabled bool) error
	Delete(ctx context.Context, id, email string) error
	SoftDelete(ctx context.Context, id, deletedBy string) error
	Restore(ctx context.Context, id string) error
	ListDeleted(ctx context.Context) ([]sqlstore.Project, error)
	GetByIDIncludingDeleted(ctx context.Context, id string) (sqlstore.Project, error)
	PurgeExpired(ctx context.Context, retentionDays int) (int64, error)
}

type DomainStore interface {
	Create(ctx context.Context, d sqlstore.Domain) error
	ListByProject(ctx context.Context, projectID string) ([]sqlstore.Domain, error)
	ListByIDs(ctx context.Context, ids []string) ([]sqlstore.Domain, error)
	Get(ctx context.Context, id string) (sqlstore.Domain, error)
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateLinkStatus(ctx context.Context, id, status string) error
	UpdateKeyword(ctx context.Context, id, keyword string) error
	SetLastGeneration(ctx context.Context, id, genID string) error
	SetLastSuccessGeneration(ctx context.Context, id, genID string) error
	RecalculateGenerationPointers(ctx context.Context, id string) error
	UpdateExtras(ctx context.Context, id, country, language string, exclude, server sql.NullString) (bool, error)
	UpdateLinkSettings(ctx context.Context, id string, anchorText, acceptorURL sql.NullString) (bool, error)
	Delete(ctx context.Context, id string) error
	UpdateIndexCheckEnabled(ctx context.Context, id string, enabled bool) error
	UpdateGenerationType(ctx context.Context, id, genType string) error
	EnsureDefaultServer(ctx context.Context, email string) error
	SoftDelete(ctx context.Context, id, deletedBy string) error
	Restore(ctx context.Context, id string) error
	ListDeleted(ctx context.Context) ([]sqlstore.Domain, error)
	GetIncludingDeleted(ctx context.Context, id string) (sqlstore.Domain, error)
	PurgeExpired(ctx context.Context, retentionDays int) (int64, error)
}

type GenerationStore interface {
	Get(ctx context.Context, id string) (sqlstore.Generation, error)
	Create(ctx context.Context, g sqlstore.Generation) error
	ListByDomain(ctx context.Context, domainID string) ([]sqlstore.Generation, error)
	ListRecentByUser(ctx context.Context, email string, limit, offset int, search string) ([]sqlstore.Generation, error)
	ListRecentByUserLite(ctx context.Context, email string, limit, offset int, search string) ([]sqlstore.Generation, error)
	ListRecentAll(ctx context.Context, limit, offset int, search string) ([]sqlstore.Generation, error)
	ListRecentAllLite(ctx context.Context, limit, offset int, search string) ([]sqlstore.Generation, error)
	CountsByStatus(ctx context.Context) (map[string]int, error)
	UpdateStatus(ctx context.Context, id, status string, progress int, errText *string) error
	UpdateFull(ctx context.Context, id, status string, progress int, errText *string, logs, artifacts []byte, started, finished *time.Time, promptID *string) error
	UpdateArtifactsSummary(ctx context.Context, id string, artifactsSummary []byte) error
	Delete(ctx context.Context, id string) error
	SaveCheckpoint(ctx context.Context, id string, checkpointData []byte) error
	ClearCheckpoint(ctx context.Context, id string) error
	UpdateStatusWithCheckpoint(ctx context.Context, id, status string, progress int, checkpointData []byte) error
	GetLastSuccessfulByDomain(ctx context.Context, domainID string) (sqlstore.Generation, error)
	GetLastByDomain(ctx context.Context, domainID string) (sqlstore.Generation, error)
}

type PromptStore interface {
	Create(ctx context.Context, p sqlstore.SystemPrompt) error
	Update(ctx context.Context, p sqlstore.SystemPrompt) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]sqlstore.SystemPrompt, error)
	Get(ctx context.Context, id string) (sqlstore.SystemPrompt, error)
}

type PromptOverrideStore interface {
	Upsert(ctx context.Context, item sqlstore.PromptOverride) error
	Delete(ctx context.Context, scopeType, scopeID, stage string) error
	ListByScope(ctx context.Context, scopeType, scopeID string) ([]sqlstore.PromptOverride, error)
	ResolveForDomainStage(ctx context.Context, domainID, projectID, stage string) (sqlstore.ResolvedPrompt, error)
}

type DeploymentAttemptStore interface {
	ListByDomain(ctx context.Context, domainID string, limit int) ([]sqlstore.DeploymentAttempt, error)
}

type ScheduleStore interface {
	Create(ctx context.Context, schedule sqlstore.Schedule) error
	Get(ctx context.Context, scheduleID string) (*sqlstore.Schedule, error)
	List(ctx context.Context, projectID string) ([]sqlstore.Schedule, error)
	Update(ctx context.Context, scheduleID string, updates sqlstore.ScheduleUpdates) error
	Delete(ctx context.Context, scheduleID string) error
	ListActive(ctx context.Context) ([]sqlstore.Schedule, error)
}

type LinkScheduleStore interface {
	GetByProject(ctx context.Context, projectID string) (*sqlstore.LinkSchedule, error)
	Upsert(ctx context.Context, schedule sqlstore.LinkSchedule) (*sqlstore.LinkSchedule, error)
	DisableByProject(ctx context.Context, projectID string) error
	DeleteByProject(ctx context.Context, projectID string) error
}

type GenQueueStore interface {
	Enqueue(ctx context.Context, item sqlstore.QueueItem) error
	Get(ctx context.Context, itemID string) (*sqlstore.QueueItem, error)
	ListByProject(ctx context.Context, projectID string) ([]sqlstore.QueueItem, error)
	ListByProjectPage(ctx context.Context, projectID string, limit, offset int, search string) ([]sqlstore.QueueItem, error)
	ListHistoryByProjectPage(ctx context.Context, projectID string, limit, offset int, search string, status *string, dateFrom *time.Time, dateTo *time.Time) ([]sqlstore.QueueItem, error)
	Delete(ctx context.Context, itemID string) error
}

type SiteFileStore interface {
	Create(ctx context.Context, file sqlstore.SiteFile) error
	Get(ctx context.Context, fileID string) (*sqlstore.SiteFile, error)
	List(ctx context.Context, domainID string) ([]sqlstore.SiteFile, error)
	ListDeleted(ctx context.Context, domainID string) ([]sqlstore.SiteFile, error)
	GetByPath(ctx context.Context, domainID, path string) (*sqlstore.SiteFile, error)
	GetByPathAny(ctx context.Context, domainID, path string) (*sqlstore.SiteFile, error)
	Update(ctx context.Context, fileID string, content []byte) error
	Delete(ctx context.Context, fileID string) error
	SoftDelete(ctx context.Context, fileID string, deletedBy sql.NullString, reason sql.NullString) error
	Restore(ctx context.Context, fileID string) error
	Move(ctx context.Context, fileID, newPath string) error
	SetLastEditedBy(ctx context.Context, fileID string, editedBy sql.NullString) error
}

type FileEditStore interface {
	Create(ctx context.Context, edit sqlstore.FileEdit) error
	ListByFile(ctx context.Context, fileID string, limit int) ([]sqlstore.FileEdit, error)
	CreateRevision(ctx context.Context, rev sqlstore.FileRevision) error
	GetRevision(ctx context.Context, revisionID string) (*sqlstore.FileRevision, error)
	ListRevisionsByFile(ctx context.Context, fileID string, limit int) ([]sqlstore.FileRevision, error)
	ListRevisionsBySource(ctx context.Context, source string) ([]sqlstore.FileRevision, error)
}

type LinkTaskStore interface {
	Create(ctx context.Context, task sqlstore.LinkTask) error
	Get(ctx context.Context, taskID string) (*sqlstore.LinkTask, error)
	ListByDomain(ctx context.Context, domainID string, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error)
	ListByProject(ctx context.Context, projectID string, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error)
	ListByUser(ctx context.Context, email string, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error)
	ListAll(ctx context.Context, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error)
	ListPending(ctx context.Context, limit int) ([]sqlstore.LinkTask, error)
	ListActiveByDomainIDs(ctx context.Context, domainIDs []string) (map[string]sqlstore.LinkTask, error)
	ListStuck(ctx context.Context, olderThan time.Duration, limit int) ([]sqlstore.LinkTask, error)
	Update(ctx context.Context, taskID string, updates sqlstore.LinkTaskUpdates) error
	Delete(ctx context.Context, taskID string) error
	PurgeCompleted(ctx context.Context, retentionDays int) (int64, error)
}

type IndexCheckStore interface {
	Create(ctx context.Context, check sqlstore.IndexCheck) error
	UpsertManualByDomainAndDate(ctx context.Context, domainID string, date, now time.Time) (sqlstore.IndexCheck, bool, error)
	Get(ctx context.Context, checkID string) (*sqlstore.IndexCheck, error)
	GetByDomainAndDate(ctx context.Context, domainID string, date time.Time) (*sqlstore.IndexCheck, error)
	ListByDomain(ctx context.Context, domainID string, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheck, error)
	ListByProject(ctx context.Context, projectID string, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheck, error)
	ListAll(ctx context.Context, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheck, error)
	ListFailed(ctx context.Context, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheck, error)
	CountByDomain(ctx context.Context, domainID string, filters sqlstore.IndexCheckFilters) (int, error)
	CountByProject(ctx context.Context, projectID string, filters sqlstore.IndexCheckFilters) (int, error)
	CountAll(ctx context.Context, filters sqlstore.IndexCheckFilters) (int, error)
	CountFailed(ctx context.Context, filters sqlstore.IndexCheckFilters) (int, error)
	AggregateStats(ctx context.Context, filters sqlstore.IndexCheckFilters) (sqlstore.IndexCheckStats, error)
	AggregateStatsByDomain(ctx context.Context, domainID string, filters sqlstore.IndexCheckFilters) (sqlstore.IndexCheckStats, error)
	AggregateStatsByProject(ctx context.Context, projectID string, filters sqlstore.IndexCheckFilters) (sqlstore.IndexCheckStats, error)
	AggregateDaily(ctx context.Context, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheckDailySummary, error)
	AggregateDailyByDomain(ctx context.Context, domainID string, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheckDailySummary, error)
	AggregateDailyByProject(ctx context.Context, projectID string, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheckDailySummary, error)
}

type CheckHistoryStore interface {
	ListByCheck(ctx context.Context, checkID string, limit int) ([]sqlstore.CheckHistory, error)
}

type LLMUsageStore interface {
	CreateEvent(ctx context.Context, item sqlstore.LLMUsageEvent) error
	ListEvents(ctx context.Context, filters sqlstore.LLMUsageFilters) ([]sqlstore.LLMUsageEvent, error)
	CountEvents(ctx context.Context, filters sqlstore.LLMUsageFilters) (int, error)
	AggregateStats(ctx context.Context, filters sqlstore.LLMUsageFilters) (sqlstore.LLMUsageStats, error)
}

type ModelPricingStore interface {
	GetActiveByModel(ctx context.Context, provider, model string, at time.Time) (*sqlstore.LLMModelPricing, error)
	ListActive(ctx context.Context) ([]sqlstore.LLMModelPricing, error)
	UpsertActive(ctx context.Context, provider, model string, inputUSDPerMillion, outputUSDPerMillion float64, updatedBy string, at time.Time) error
}

type AppSettingsStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetBool(ctx context.Context, key string) (bool, error)
	SetBool(ctx context.Context, key string, val bool) error
}

type ScheduleRunLogStore interface {
	ListByProject(ctx context.Context, projectID string, limit, offset int) ([]sqlstore.ScheduleRunLog, error)
	ListBySchedule(ctx context.Context, scheduleID string, limit, offset int) ([]sqlstore.ScheduleRunLog, error)
}

type LegacyImportStore interface {
	CreateJob(ctx context.Context, job sqlstore.LegacyImportJob) error
	GetJob(ctx context.Context, id string) (sqlstore.LegacyImportJob, error)
	ListJobsByProject(ctx context.Context, projectID string) ([]sqlstore.LegacyImportJob, error)
	CreateItem(ctx context.Context, item sqlstore.LegacyImportItem) error
	GetItem(ctx context.Context, id string) (sqlstore.LegacyImportItem, error)
	ListItemsByJob(ctx context.Context, jobID string) ([]sqlstore.LegacyImportItem, error)
}

type AgentSessionStore interface {
	Create(ctx context.Context, sess sqlstore.AgentSession) error
	Get(ctx context.Context, id string) (sqlstore.AgentSession, error)
	ListByDomain(ctx context.Context, domainID string, limit int) ([]sqlstore.AgentSession, error)
	Finish(ctx context.Context, id, status, summary string, filesChanged []string, messageCount int) error
	SetSnapshotTag(ctx context.Context, id, tag string) error
	MarkStaleRunning(ctx context.Context, olderThan time.Duration) (int64, error)
	SaveMessages(ctx context.Context, id string, messagesJSON, chatLogJSON []byte) error
	SavePreFileIDs(ctx context.Context, id string, fileIDs []string) error
	AppendEvent(ctx context.Context, sessionID string, seq int, eventType string, payload []byte) error
	ListEvents(ctx context.Context, sessionID string) ([]sqlstore.AgentSessionEvent, error)
	NextEventSeq(ctx context.Context, sessionID string) (int, error)
	SaveDiagnostics(ctx context.Context, id string, diagJSON []byte) error
}

type FileLockStore interface {
	Acquire(ctx context.Context, domainID, filePath, userEmail string, ttl time.Duration) (sqlstore.FileLock, bool, error)
	Release(ctx context.Context, domainID, filePath, userEmail string) error
	ListByDomain(ctx context.Context, domainID string) ([]sqlstore.FileLock, error)
	PurgeExpired(ctx context.Context) (int64, error)
}

type Server struct {
	cfg              config.Config
	svc              *auth.Service
	projects         ProjectStore
	projectMembers   *sqlstore.ProjectMemberStore
	domains          DomainStore
	generations      GenerationStore
	prompts          PromptStore
	promptOverrides  PromptOverrideStore
	deployments      DeploymentAttemptStore
	schedules        ScheduleStore
	linkSchedules    LinkScheduleStore
	auditRules       *sqlstore.AuditStore
	siteFiles        SiteFileStore
	fileEdits        FileEditStore
	contentBackend   domainfs.SiteContentBackend
	domainFilesCache domainFilesCache
	linkTasks        LinkTaskStore
	genQueue         GenQueueStore
	indexChecks      IndexCheckStore
	checkHistory     CheckHistoryStore
	llmUsage         LLMUsageStore
	modelPricing     ModelPricingStore
	legacyImports    LegacyImportStore
	appSettings      AppSettingsStore
	scheduleRunLogs  ScheduleRunLogStore
	tasks            tasks.Enqueuer
	reqDuration      *prometheus.HistogramVec
	reqCounter       *prometheus.CounterVec
	genStatus        *prometheus.GaugeVec
	registry         *prometheus.Registry
	logger           *zap.SugaredLogger
	editorCtxMu      sync.Mutex
	editorCtxCache   map[string]editorContextPackCacheEntry
	agentSessions    AgentSessionStore
	fileLocks        FileLockStore
}
