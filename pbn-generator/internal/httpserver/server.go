package httpserver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/crypto/secretbox"
	"obzornik-pbn-generator/internal/llm"
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
	Delete(ctx context.Context, id, email string) error
}

type DomainStore interface {
	Create(ctx context.Context, d sqlstore.Domain) error
	ListByProject(ctx context.Context, projectID string) ([]sqlstore.Domain, error)
	ListByIDs(ctx context.Context, ids []string) ([]sqlstore.Domain, error)
	Get(ctx context.Context, id string) (sqlstore.Domain, error)
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateKeyword(ctx context.Context, id, keyword string) error
	SetLastGeneration(ctx context.Context, id, genID string) error
	UpdateExtras(ctx context.Context, id, country, language string, exclude, server sql.NullString) (bool, error)
	UpdateLinkSettings(ctx context.Context, id string, anchorText, acceptorURL sql.NullString) (bool, error)
	Delete(ctx context.Context, id string) error
	EnsureDefaultServer(ctx context.Context, email string) error
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
	Delete(ctx context.Context, itemID string) error
}

type SiteFileStore interface {
	Get(ctx context.Context, fileID string) (*sqlstore.SiteFile, error)
	List(ctx context.Context, domainID string) ([]sqlstore.SiteFile, error)
	GetByPath(ctx context.Context, domainID, path string) (*sqlstore.SiteFile, error)
	Update(ctx context.Context, fileID string, content []byte) error
	Delete(ctx context.Context, fileID string) error
}

type FileEditStore interface {
	Create(ctx context.Context, edit sqlstore.FileEdit) error
	ListByFile(ctx context.Context, fileID string, limit int) ([]sqlstore.FileEdit, error)
}

type LinkTaskStore interface {
	Create(ctx context.Context, task sqlstore.LinkTask) error
	Get(ctx context.Context, taskID string) (*sqlstore.LinkTask, error)
	ListByDomain(ctx context.Context, domainID string, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error)
	ListByProject(ctx context.Context, projectID string, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error)
	ListAll(ctx context.Context, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error)
	ListPending(ctx context.Context, limit int) ([]sqlstore.LinkTask, error)
	Update(ctx context.Context, taskID string, updates sqlstore.LinkTaskUpdates) error
	Delete(ctx context.Context, taskID string) error
}

type IndexCheckStore interface {
	Create(ctx context.Context, check sqlstore.IndexCheck) error
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
	ResetForManual(ctx context.Context, checkID string, nextRetry time.Time) error
}

type CheckHistoryStore interface {
	ListByCheck(ctx context.Context, checkID string, limit int) ([]sqlstore.CheckHistory, error)
}

type Server struct {
	cfg            config.Config
	svc            *auth.Service
	projects       ProjectStore
	projectMembers *sqlstore.ProjectMemberStore
	domains        DomainStore
	generations    GenerationStore
	prompts        PromptStore
	schedules      ScheduleStore
	linkSchedules  LinkScheduleStore
	auditRules     *sqlstore.AuditStore
	siteFiles      SiteFileStore
	fileEdits      FileEditStore
	linkTasks      LinkTaskStore
	genQueue       GenQueueStore
	indexChecks    IndexCheckStore
	checkHistory   CheckHistoryStore
	tasks          tasks.Enqueuer
	reqDuration    *prometheus.HistogramVec
	reqCounter     *prometheus.CounterVec
	genStatus      *prometheus.GaugeVec
	registry       *prometheus.Registry
	logger         *zap.SugaredLogger
}

func New(cfg config.Config, svc *auth.Service, logger *zap.SugaredLogger, projects ProjectStore, projectMembers *sqlstore.ProjectMemberStore, domains DomainStore, generations GenerationStore, prompts PromptStore, schedules ScheduleStore, linkSchedules LinkScheduleStore, auditRules *sqlstore.AuditStore, siteFiles SiteFileStore, fileEdits FileEditStore, linkTasks LinkTaskStore, genQueue GenQueueStore, indexChecks IndexCheckStore, checkHistory CheckHistoryStore, tq tasks.Enqueuer) *Server {
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
		cfg:            cfg,
		svc:            svc,
		projects:       projects,
		projectMembers: projectMembers,
		domains:        domains,
		generations:    generations,
		prompts:        prompts,
		schedules:      schedules,
		linkSchedules:  linkSchedules,
		auditRules:     auditRules,
		siteFiles:      siteFiles,
		fileEdits:      fileEdits,
		linkTasks:      linkTasks,
		genQueue:       genQueue,
		indexChecks:    indexChecks,
		checkHistory:   checkHistory,
		tasks:          tq,
		reqDuration:    reqDuration,
		reqCounter:     reqCounter,
		genStatus:      genStatus,
		registry:       registry,
		logger:         logger,
	}
	s.startGenerationMetricsLoop()
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/register", http.HandlerFunc(s.handleRegister))
	mux.Handle("/api/login", http.HandlerFunc(s.handleLogin))
	mux.Handle("/api/logout", http.HandlerFunc(s.handleLogout))
	mux.Handle("/api/logout-all", s.withAuth(http.HandlerFunc(s.handleLogoutAll)))
	mux.Handle("/api/captcha", http.HandlerFunc(s.handleCaptcha))
	mux.Handle("/api/me", s.withAuth(http.HandlerFunc(s.handleMe)))
	mux.Handle("/api/profile", s.withAuth(http.HandlerFunc(s.handleUpdateProfile)))
	mux.Handle("/api/profile/api-key", s.withAuth(http.HandlerFunc(s.handleProfileAPIKey)))
	mux.Handle("/api/refresh", http.HandlerFunc(s.handleRefresh))
	mux.Handle("/api/password", s.withAuth(http.HandlerFunc(s.handleChangePassword)))
	mux.Handle("/api/email/change/request", s.withAuth(http.HandlerFunc(s.handleEmailChangeRequest)))
	mux.Handle("/api/email/change/confirm", http.HandlerFunc(s.handleEmailChangeConfirm))
	mux.Handle("/api/verify/request", http.HandlerFunc(s.handleVerifyRequest))
	mux.Handle("/api/verify/confirm", http.HandlerFunc(s.handleVerifyConfirm))
	mux.Handle("/api/password/reset/request", http.HandlerFunc(s.handlePasswordResetRequest))
	mux.Handle("/api/password/reset/confirm", http.HandlerFunc(s.handlePasswordResetConfirm))
	mux.Handle("/api/dashboard", s.withAuth(http.HandlerFunc(s.handleDashboard)))
	mux.Handle("/api/projects", s.withAuth(http.HandlerFunc(s.handleProjects)))
	mux.Handle("/api/projects/", s.withAuth(http.HandlerFunc(s.handleProjectByID)))
	mux.Handle("/api/domains", s.withAuth(http.HandlerFunc(s.handleDomainsBatch)))
	mux.Handle("/api/domains/", s.withAuth(http.HandlerFunc(s.handleDomainActions)))
	mux.Handle("/api/generations", s.withAuth(http.HandlerFunc(s.handleGenerations)))
	mux.Handle("/api/generations/", s.withAuth(http.HandlerFunc(s.handleGenerationByID)))
	mux.Handle("/api/links", s.withAuth(http.HandlerFunc(s.handleLinks)))
	mux.Handle("/api/links/", s.withAuth(http.HandlerFunc(s.handleLinkByID)))
	mux.Handle("/api/queue/", s.withAuth(http.HandlerFunc(s.handleQueueItem)))
	mux.Handle("/api/admin/index-checks", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexChecks))))
	mux.Handle("/api/admin/index-checks/run", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexChecksRun))))
	mux.Handle("/api/admin/index-checks/stats", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexChecksStats))))
	mux.Handle("/api/admin/index-checks/calendar", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexChecksCalendar))))
	mux.Handle("/api/admin/index-checks/", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexCheckRoute))))
	mux.Handle("/api/admin/index-checks/failed", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexChecksFailed))))
	mux.Handle("/api/admin/users", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminUsers))))
	mux.Handle("/api/admin/users/", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminUserRoute))))
	mux.Handle("/api/admin/prompts", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminPrompts))))
	mux.Handle("/api/admin/prompts/", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminPromptByID))))
	mux.Handle("/api/admin/audit-rules", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminAuditRules))))
	mux.Handle("/api/admin/audit-rules/", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminAuditRuleByCode))))
	mux.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))
	mux.Handle("/metrics", promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}))
	return s.withRequestID(s.withLogging(s.withMetrics(s.withCORS(mux))))
}

func (s *Server) handleCaptcha(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	challenge, err := s.svc.GenerateCaptcha(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, "captcha not available")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       challenge.ID,
		"question": challenge.Question,
		"ttl":      int(challenge.TTL.Seconds()),
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	var (
		projects []sqlstore.Project
		gens     []sqlstore.Generation
		err      error
	)
	if strings.EqualFold(user.Role, "admin") {
		projects, err = s.projects.ListAll(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list projects")
			return
		}
		gens, err = s.generations.ListRecentAllLite(r.Context(), limit, 0, "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list generations")
			return
		}
	} else {
		projects, err = s.projects.ListByUser(r.Context(), user.Email)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list projects")
			return
		}
		gens, err = s.generations.ListRecentByUserLite(r.Context(), user.Email, limit, 0, "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list generations")
			return
		}
	}

	projectDTOs := make([]projectDTO, 0, len(projects))
	for _, p := range projects {
		projectDTOs = append(projectDTOs, s.toProjectDTO(r.Context(), p))
	}

	pending := 0
	processing := 0
	errorCount := 0
	durations := make([]time.Duration, 0, len(gens))
	for _, g := range gens {
		switch g.Status {
		case "pending":
			pending++
		case "processing":
			processing++
		case "error":
			errorCount++
		}
		if g.Status == "success" && g.StartedAt.Valid && g.FinishedAt.Valid {
			d := g.FinishedAt.Time.Sub(g.StartedAt.Time)
			if d > 0 {
				durations = append(durations, d)
			}
		}
	}
	var avgMinutes *int
	if len(durations) > 0 {
		var sum time.Duration
		for _, d := range durations {
			sum += d
		}
		avg := sum / time.Duration(len(durations))
		minutes := int(math.Round(avg.Minutes()))
		if minutes < 1 {
			minutes = 1
		}
		avgMinutes = &minutes
	}

	domainMap := map[string]string{}
	if s.domains != nil {
		for _, g := range gens {
			if _, ok := domainMap[g.DomainID]; ok {
				continue
			}
			if d, err := s.domains.Get(r.Context(), g.DomainID); err == nil && strings.TrimSpace(d.URL) != "" {
				domainMap[g.DomainID] = d.URL
			}
		}
	}

	recentErrors := make([]sqlstore.Generation, 0)
	for _, g := range gens {
		if g.Status == "error" {
			recentErrors = append(recentErrors, g)
		}
	}
	sort.Slice(recentErrors, func(i, j int) bool {
		return recentErrors[i].UpdatedAt.After(recentErrors[j].UpdatedAt)
	})
	if len(recentErrors) > 5 {
		recentErrors = recentErrors[:5]
	}
	recentErrorDTOs := make([]generationDTO, 0, len(recentErrors))
	for _, g := range recentErrors {
		var url *string
		if v, ok := domainMap[g.DomainID]; ok {
			val := v
			url = &val
		}
		recentErrorDTOs = append(recentErrorDTOs, toGenerationDTOWithDomain(g, url))
	}

	writeJSON(w, http.StatusOK, dashboardDTO{
		Projects: projectDTOs,
		Stats: dashboardStatsDTO{
			Pending:    pending,
			Processing: processing,
			Error:      errorCount,
			AvgMinutes: avgMinutes,
			AvgSample:  len(durations),
		},
		RecentErrors: recentErrorDTOs,
	})
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Если Origin пуст (curl, Prometheus и т.п.), пропускаем без запрета.
		if origin == "" {
			if len(s.cfg.AllowedOrigins) > 0 {
				origin = s.cfg.AllowedOrigins[0]
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		} else {
			if len(s.cfg.AllowedOrigins) > 0 && !isOriginAllowed(origin, s.cfg.AllowedOrigins) {
				http.Error(w, "CORS origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	creds, captchaID, captchaToken, err := readCredentialsWithCaptcha(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	u, err := s.svc.Register(r.Context(), clientIP(r), creds, captchaID, captchaToken)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, auth.ErrRateLimited) {
			status = http.StatusTooManyRequests
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"email":     u.Email,
		"createdAt": u.CreatedAt,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if !ensureJSON(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	creds, captchaID, captchaToken, err := readCredentialsWithCaptcha(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	res, tokens, err := s.svc.Login(r.Context(), clientIP(r), creds, captchaID, captchaToken)
	if err != nil {
		switch e := err.(type) {
		case *auth.LockoutError:
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":   e.Error(),
				"retryIn": e.RetryInSeconds,
			})
			return
		default:
			if errors.Is(err, auth.ErrRateLimited) {
				writeError(w, http.StatusTooManyRequests, "too many login attempts, please slow down")
				return
			}
			if strings.Contains(strings.ToLower(err.Error()), "captcha") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if errors.Is(err, auth.ErrInvalidCredentials) {
				writeError(w, http.StatusUnauthorized, "invalid email or password")
				return
			}
			if errors.Is(err, auth.ErrEmailNotVerified) {
				writeError(w, http.StatusForbidden, "email not verified")
				return
			}
			writeError(w, http.StatusInternalServerError, "could not login")
			return
		}
	}

	setAuthCookies(w, s.cfg, tokens)

	writeJSON(w, http.StatusOK, map[string]any{
		"email": res.Email,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	refresh := getRefreshFromCookie(r)
	if refresh != "" {
		_ = s.svc.Logout(r.Context(), refresh)
	}
	clearAuthCookies(w, s.cfg)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	email := userEmailFromContext(r.Context())
	sess := sessionFromContext(r.Context())

	var body struct {
		KeepCurrent bool `json:"keepCurrent"`
	}
	if r.Body != nil {
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	if err := s.svc.LogoutAllSessions(r.Context(), email, sess.JTI, body.KeepCurrent); err != nil {
		writeError(w, http.StatusInternalServerError, "could not revoke sessions")
		return
	}

	if !body.KeepCurrent {
		clearAuthCookies(w, s.cfg)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "sessions revoked",
		"keepCurrent": body.KeepCurrent,
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "could not parse JSON body")
		return
	}

	email := userEmailFromContext(r.Context())
	if email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := s.svc.ChangePassword(r.Context(), email, body.CurrentPassword, body.NewPassword); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		Name      string `json:"name"`
		AvatarURL string `json:"avatarUrl"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "could not parse JSON body")
		return
	}
	email := userEmailFromContext(r.Context())
	if email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.svc.UpdateProfile(r.Context(), email, body.Name, body.AvatarURL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "profile updated"})
}

func (s *Server) handleProfileAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodPost:
		if !ensureJSON(w, r) {
			return
		}
		var body struct {
			APIKey string `json:"apiKey"`
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}

		// Валидация API ключа перед сохранением
		// Используем увеличенный таймаут для учета возможных задержек сети
		validateCtx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
		defer cancel()
		if err := llm.ValidateAPIKey(validateCtx, body.APIKey, s.cfg.GeminiDefaultModel); err != nil {
			// Различаем ошибки валидации и таймауты
			// Ошибка уже санитизирована в ValidateAPIKey, но перестраховываемся
			sanitizedErr := llm.SanitizeError(err)
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
				writeError(w, http.StatusRequestTimeout, fmt.Sprintf("validation timeout: %v. Please try again.", sanitizedErr))
			} else {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid API key: %v", sanitizedErr))
			}
			return
		}

		if err := s.svc.SaveUserAPIKey(r.Context(), user.Email, body.APIKey); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "api key saved"})
	case http.MethodDelete:
		if err := s.svc.DeleteUserAPIKey(r.Context(), user.Email); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "api key removed"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleEmailChangeRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		NewEmail string `json:"newEmail"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.NewEmail) == "" {
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}
	email := userEmailFromContext(r.Context())
	if email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, err := s.svc.RequestEmailChange(r.Context(), email, body.NewEmail); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "confirmation sent"})
}

func (s *Server) handleEmailChangeConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
		writeError(w, http.StatusBadRequest, "invalid token")
		return
	}
	newEmail, err := s.svc.ConfirmEmailChange(r.Context(), body.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	if tokens, err := s.svc.IssueSession(r.Context(), newEmail); err == nil {
		setAuthCookies(w, s.cfg, tokens)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "email changed", "email": newEmail})
}
func (s *Server) handleVerifyRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		Email         string `json:"email"`
		CaptchaID     string `json:"captchaId"`
		CaptchaAnswer string `json:"captchaAnswer"`
		CaptchaToken  string `json:"captchaToken"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Email) == "" {
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}
	captchaToken := body.CaptchaAnswer
	if captchaToken == "" {
		captchaToken = body.CaptchaToken
	}
	token, err := s.svc.RequestEmailVerification(r.Context(), body.Email, body.CaptchaID, captchaToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = token // токен только на email
	writeJSON(w, http.StatusOK, map[string]any{"status": "verification sent"})
}

func (s *Server) handleVerifyConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
		writeError(w, http.StatusBadRequest, "invalid token")
		return
	}
	email, err := s.svc.VerifyEmail(r.Context(), body.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	tokens, err := s.svc.IssueSession(r.Context(), email)
	if err == nil {
		setAuthCookies(w, s.cfg, tokens)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "email verified"})
}

func (s *Server) handlePasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		Email         string `json:"email"`
		CaptchaID     string `json:"captchaId"`
		CaptchaAnswer string `json:"captchaAnswer"`
		CaptchaToken  string `json:"captchaToken"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Email) == "" {
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}
	captchaToken := body.CaptchaAnswer
	if captchaToken == "" {
		captchaToken = body.CaptchaToken
	}
	token, err := s.svc.RequestPasswordReset(r.Context(), body.Email, body.CaptchaID, captchaToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create reset token")
		return
	}
	_ = token
	writeJSON(w, http.StatusOK, map[string]any{"status": "reset email sent"})
}

func (s *Server) handlePasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
		writeError(w, http.StatusBadRequest, "invalid token")
		return
	}
	if err := s.svc.ResetPassword(r.Context(), body.Token, body.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "invalid token or password")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "password reset"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := currentUserFromContext(r.Context())
	if !ok || u.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Получаем информацию об API ключе
	hasApiKey := false
	apiKeyPrefix := ""
	if u.APIKeySetAt != nil {
		// Получаем зашифрованный ключ для показа префикса
		encKey, err := s.svc.GetUserAPIKeyEncrypted(r.Context(), u.Email)
		if err == nil && len(encKey) > 0 {
			hasApiKey = true
			// Расшифровываем для показа префикса (первые 4 символа)
			keySecret := secretbox.DeriveKey(s.cfg.APIKeySecret)
			decKey, err := secretbox.Decrypt(keySecret, encKey)
			if err == nil && len(decKey) >= 4 {
				apiKeyPrefix = string(decKey[:4]) + "..."
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"email":           u.Email,
		"name":            u.Name,
		"avatarUrl":       u.AvatarURL,
		"role":            u.Role,
		"isApproved":      u.IsApproved,
		"verified":        u.Verified,
		"apiKeyUpdatedAt": u.APIKeySetAt,
		"hasApiKey":       hasApiKey,
		"apiKeyPrefix":    apiKeyPrefix,
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	refresh := getRefreshFromCookie(r)
	if refresh == "" {
		if s.logger != nil {
			s.logger.Warnw("refresh failed: missing refresh cookie", "ip", clientIP(r))
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	tokens, err := s.svc.Refresh(r.Context(), refresh)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnw("refresh failed", "ip", clientIP(r), "err", err)
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	setAuthCookies(w, s.cfg, tokens)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readCredentialsWithCaptcha(r *http.Request) (auth.Credentials, string, string, error) {
	defer r.Body.Close()
	var payload struct {
		Email         string `json:"email"`
		Password      string `json:"password"`
		CaptchaID     string `json:"captchaId"`
		CaptchaAnswer string `json:"captchaAnswer"`
		CaptchaToken  string `json:"captchaToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return auth.Credentials{}, "", "", errors.New("could not parse JSON body")
	}
	captchaToken := payload.CaptchaAnswer
	if captchaToken == "" {
		captchaToken = payload.CaptchaToken
	}
	return auth.Credentials{Email: payload.Email, Password: payload.Password}, payload.CaptchaID, captchaToken, nil
}

func ensureJSON(w http.ResponseWriter, r *http.Request) bool {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if !strings.HasPrefix(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}
	return true
}

// --- Projects / Domains / Generations ---

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodGet:
		var (
			projects []sqlstore.Project
			err      error
		)
		if strings.EqualFold(user.Role, "admin") {
			projects, err = s.projects.ListAll(r.Context())
		} else {
			projects, err = s.projects.ListByUser(r.Context(), user.Email)
		}
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("failed to list projects: %v", err)
			}
			writeError(w, http.StatusInternalServerError, "could not list projects")
			return
		}
		// Используем новый метод для получения ownerHasApiKey
		projectDTOs := make([]projectDTO, 0, len(projects))
		for _, p := range projects {
			projectDTOs = append(projectDTOs, s.toProjectDTO(r.Context(), p))
		}
		writeJSON(w, http.StatusOK, projectDTOs)
	case http.MethodPost:
		if !ensureJSON(w, r) {
			return
		}
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		var body struct {
			Name     string `json:"name"`
			Country  string `json:"country"`
			Lang     string `json:"language"`
			Status   string `json:"status"`
			Timezone string `json:"timezone"`
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if body.Status == "" {
			body.Status = "draft"
		}
		tz := strings.TrimSpace(body.Timezone)
		if tz == "" {
			tz = "Europe/Moscow"
		}
		if _, err := time.LoadLocation(tz); err != nil {
			writeError(w, http.StatusBadRequest, "invalid timezone")
			return
		}

		id := uuid.NewString()
		p := sqlstore.Project{
			ID:             id,
			UserEmail:      user.Email,
			Name:           body.Name,
			TargetCountry:  body.Country,
			TargetLanguage: body.Lang,
			Status:         body.Status,
			Timezone:       sqlstore.NullableString(tz),
		}
		if err := s.projects.Create(r.Context(), p); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create project")
			return
		}
		writeJSON(w, http.StatusCreated, toProjectDTO(p))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "missing project id")
		return
	}
	projectID := parts[0]
	if len(parts) > 1 && parts[1] == "domains" {
		action := ""
		if len(parts) > 2 {
			action = parts[2]
		}
		s.handleProjectDomains(w, r, projectID, action)
		return
	}
	if len(parts) > 1 && parts[1] == "summary" {
		s.handleProjectSummary(w, r, projectID)
		return
	}
	if len(parts) > 1 && parts[1] == "members" {
		if len(parts) > 2 {
			// Обработка конкретного участника: /api/projects/:id/members/:email
			memberEmail, err := url.PathUnescape(parts[2])
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid email")
				return
			}
			s.handleProjectMemberByEmail(w, r, projectID, memberEmail)
		} else {
			// Список участников: /api/projects/:id/members
			s.handleProjectMembers(w, r, projectID)
		}
		return
	}
	if len(parts) > 1 && parts[1] == "schedules" {
		if len(parts) > 2 {
			scheduleID, err := url.PathUnescape(parts[2])
			if err != nil || strings.TrimSpace(scheduleID) == "" {
				writeError(w, http.StatusBadRequest, "invalid schedule id")
				return
			}
			action := ""
			if len(parts) > 3 {
				action = parts[3]
			}
			if action == "trigger" {
				s.handleProjectScheduleTrigger(w, r, projectID, scheduleID)
				return
			}
			s.handleProjectScheduleByID(w, r, projectID, scheduleID)
			return
		}
		s.handleProjectSchedules(w, r, projectID)
		return
	}
	if len(parts) > 1 && parts[1] == "link-schedule" {
		action := ""
		if len(parts) > 2 {
			action = parts[2]
		}
		if action == "trigger" {
			s.handleProjectLinkScheduleTrigger(w, r, projectID)
			return
		}
		s.handleProjectLinkSchedule(w, r, projectID)
		return
	}
	if len(parts) > 1 && parts[1] == "queue" {
		action := ""
		if len(parts) > 2 {
			action = parts[2]
		}
		if action == "cleanup" {
			s.handleProjectQueueCleanup(w, r, projectID)
			return
		}
		s.handleProjectQueue(w, r, projectID)
		return
	}
	if len(parts) > 1 && parts[1] == "index-checks" {
		s.handleProjectIndexChecks(w, r, projectID, parts[2:])
		return
	}

	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	project, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.toProjectDTO(r.Context(), project))
	case http.MethodPut:
		if !ensureJSON(w, r) {
			return
		}
		var body struct {
			Name     string `json:"name"`
			Country  string `json:"country"`
			Lang     string `json:"language"`
			Status   string `json:"status"`
			Timezone string `json:"timezone"`
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		tz := strings.TrimSpace(body.Timezone)
		if tz == "" {
			if project.Timezone.Valid && project.Timezone.String != "" {
				tz = project.Timezone.String
			} else {
				tz = "UTC"
			}
		}
		if _, err := time.LoadLocation(tz); err != nil {
			writeError(w, http.StatusBadRequest, "invalid timezone")
			return
		}

		p := sqlstore.Project{
			ID:             projectID,
			UserEmail:      project.UserEmail,
			Name:           body.Name,
			TargetCountry:  body.Country,
			TargetLanguage: body.Lang,
			Status:         body.Status,
			Timezone:       sqlstore.NullableString(tz),
		}
		if p.Status == "" {
			p.Status = "draft"
		}
		if err := s.projects.Update(r.Context(), p); err != nil {
			writeError(w, http.StatusInternalServerError, "could not update project")
			return
		}
		// Получаем созданный проект для возврата с ownerHasApiKey
		createdProject, err := s.projects.GetByID(r.Context(), p.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not get created project")
			return
		}
		writeJSON(w, http.StatusCreated, s.toProjectDTO(r.Context(), createdProject))
	case http.MethodDelete:
		if !strings.EqualFold(user.Role, "admin") && !strings.EqualFold(user.Email, project.UserEmail) {
			writeError(w, http.StatusForbidden, "only owner can delete project")
			return
		}
		if err := s.projects.Delete(r.Context(), projectID, project.UserEmail); err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete project")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectSummary(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	project, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}
	domains, err := s.domains.ListByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list domains")
		return
	}

	memberDTOs := []projectMemberDTO{}
	allowMembers := strings.EqualFold(user.Role, "admin")
	if !allowMembers {
		if role, err := s.getProjectMemberRole(r.Context(), projectID, user.Email); err == nil && role == "owner" {
			allowMembers = true
		}
	}
	if allowMembers && s.projectMembers != nil {
		members, err := s.projectMembers.ListByProject(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load members")
			return
		}
		memberDTOs = append(memberDTOs, projectMemberDTO{
			Email:     project.UserEmail,
			Role:      "owner",
			CreatedAt: project.CreatedAt,
		})
		for _, m := range members {
			memberDTOs = append(memberDTOs, projectMemberDTO{
				Email:     m.UserEmail,
				Role:      m.Role,
				CreatedAt: m.CreatedAt,
			})
		}
	}

	resp := projectSummaryDTO{
		Project: s.toProjectDTO(r.Context(), project),
		Domains: toDomainDTOs(domains),
		Members: memberDTOs,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDomainSummary(w http.ResponseWriter, r *http.Request, domainID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	domain, project, err := s.authorizeDomain(r.Context(), domainID)
	if err != nil {
		respondAuthzError(w, err, "domain not found")
		return
	}

	genLimit := 10
	if v := r.URL.Query().Get("gen_limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			genLimit = n
		}
	}
	linkLimit := 20
	if v := r.URL.Query().Get("link_limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			linkLimit = n
		}
	}

	gens, err := s.generations.ListByDomain(r.Context(), domainID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list generations")
		return
	}
	if len(gens) > genLimit {
		gens = gens[:genLimit]
	}
	genDTOs := make([]generationDTO, 0, len(gens))
	for i, g := range gens {
		dto := toGenerationDTO(g)
		if i > 0 {
			dto.Logs = nil
			dto.Artifacts = nil
		}
		genDTOs = append(genDTOs, dto)
	}

	linkDTOs := make([]linkTaskDTO, 0)
	if s.linkTasks != nil {
		tasks, err := s.linkTasks.ListByDomain(r.Context(), domainID, sqlstore.LinkTaskFilters{Limit: linkLimit})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list link tasks")
			return
		}
		for _, t := range tasks {
			linkDTOs = append(linkDTOs, toLinkTaskDTO(t))
		}
	}

	resp := domainSummaryDTO{
		Domain:      toDomainDTO(domain),
		ProjectName: project.Name,
		Generations: genDTOs,
		LinkTasks:   linkDTOs,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleProjectDomains(w http.ResponseWriter, r *http.Request, projectID string, action string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	p, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	// Проверяем права на редактирование (для POST и import нужны права editor или owner)
	if r.Method == http.MethodPost || action == "import" {
		memberRole, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
	}
	// Импорт доменов пачкой
	if action == "import" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !ensureJSON(w, r) {
			return
		}
		defer r.Body.Close()
		var body struct {
			Items []struct {
				URL     string `json:"url"`
				Keyword string `json:"keyword"`
			} `json:"items"`
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if len(body.Items) == 0 && strings.TrimSpace(body.Text) == "" {
			writeError(w, http.StatusBadRequest, "no domains provided")
			return
		}
		records := body.Items
		if len(records) == 0 {
			lines := strings.Split(body.Text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				url := line
				kw := ""
				if strings.Contains(line, ",") {
					parts := strings.SplitN(line, ",", 2)
					url = strings.TrimSpace(parts[0])
					kw = strings.TrimSpace(parts[1])
				}
				records = append(records, struct {
					URL     string `json:"url"`
					Keyword string `json:"keyword"`
				}{URL: url, Keyword: kw})
			}
		}
		created := 0
		for _, item := range records {
			cleanURL := sanitizeDomain(item.URL)
			if cleanURL == "" {
				continue
			}
			_ = s.domains.EnsureDefaultServer(r.Context(), p.UserEmail)
			d := sqlstore.Domain{
				ID:             uuid.NewString(),
				ProjectID:      projectID,
				URL:            cleanURL,
				MainKeyword:    strings.TrimSpace(item.Keyword),
				TargetCountry:  p.TargetCountry,
				TargetLanguage: p.TargetLanguage,
				ServerID:       sqlstore.NullableString(sqlstore.DefaultServerID),
				Status:         "waiting",
			}
			if err := s.domains.Create(r.Context(), d); err != nil {
				if s.logger != nil {
					s.logger.Warnf("import domain failed: %v", err)
				}
				continue
			}
			created++
		}
		writeJSON(w, http.StatusCreated, map[string]any{"created": created})
		return
	}

	switch r.Method {
	case http.MethodGet:
		list, err := s.domains.ListByProject(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list domains")
			return
		}
		writeJSON(w, http.StatusOK, toDomainDTOs(list))
	case http.MethodPost:
		if !ensureJSON(w, r) {
			return
		}
		var body struct {
			URL      string `json:"url"`
			Keyword  string `json:"keyword"`
			ServerID string `json:"serverId"`
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.URL) == "" {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		_ = s.domains.EnsureDefaultServer(r.Context(), p.UserEmail)
		d := sqlstore.Domain{
			ID:             uuid.NewString(),
			ProjectID:      projectID,
			URL:            sanitizeDomain(body.URL),
			MainKeyword:    body.Keyword,
			TargetCountry:  p.TargetCountry,
			TargetLanguage: p.TargetLanguage,
			ServerID:       sqlstore.NullableString(sqlstore.DefaultServerID),
			Status:         "waiting",
		}
		if body.ServerID != "" {
			d.ServerID = sqlstore.NullableString(body.ServerID)
		}
		if err := s.domains.Create(r.Context(), d); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create domain")
			return
		}
		writeJSON(w, http.StatusCreated, toDomainDTO(d))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleDomainsBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.domains == nil {
		writeError(w, http.StatusInternalServerError, "domains not configured")
		return
	}
	rawIDs := strings.TrimSpace(r.URL.Query().Get("ids"))
	if rawIDs == "" {
		writeJSON(w, http.StatusOK, []domainDTO{})
		return
	}
	parts := strings.Split(rawIDs, ",")
	unique := make(map[string]struct{}, len(parts))
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if _, ok := unique[id]; ok {
			continue
		}
		unique[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		writeJSON(w, http.StatusOK, []domainDTO{})
		return
	}
	if len(ids) > 200 {
		ids = ids[:200]
	}
	list, err := s.domains.ListByIDs(r.Context(), ids)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list domains")
		return
	}
	if !strings.EqualFold(user.Role, "admin") {
		filtered := make([]sqlstore.Domain, 0, len(list))
		for _, d := range list {
			if _, err := s.authorizeProject(r.Context(), d.ProjectID); err == nil {
				filtered = append(filtered, d)
			}
		}
		list = filtered
	}
	resp := make([]domainDTO, 0, len(list))
	for _, d := range list {
		resp = append(resp, toDomainDTO(d))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDomainActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/domains/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "missing domain id")
		return
	}
	domainID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "generate":
		s.handleDomainGenerate(w, r, domainID)
	case "generations":
		s.handleDomainGenerations(w, r, domainID)
	case "summary":
		s.handleDomainSummary(w, r, domainID)
	case "links":
		s.handleDomainLinks(w, r, domainID, parts[2:])
	case "link":
		s.handleDomainLinkRun(w, r, domainID, parts[2:])
	case "files":
		s.handleDomainFiles(w, r, domainID, parts[2:])
	case "index-checks":
		s.handleDomainIndexChecks(w, r, domainID, parts[2:])
	case "":
		s.handleDomainBase(w, r, domainID)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleDomainBase(w http.ResponseWriter, r *http.Request, domainID string) {
	d, _, err := s.authorizeDomain(r.Context(), domainID)
	if err != nil {
		respondAuthzError(w, err, "domain not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, toDomainDTO(d))
	case http.MethodPatch:
		if !ensureJSON(w, r) {
			return
		}
		defer r.Body.Close()
		var body struct {
			Keyword         *string `json:"keyword"`
			Status          *string `json:"status"`
			Country         *string `json:"country"`
			Language        *string `json:"language"`
			ExcludeDomains  *string `json:"exclude_domains"`
			ServerID        *string `json:"server_id"`
			LinkAnchorText  *string `json:"link_anchor_text"`
			LinkAcceptorURL *string `json:"link_acceptor_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if body.Keyword != nil {
			kw := strings.TrimSpace(*body.Keyword)
			if err := s.domains.UpdateKeyword(r.Context(), domainID, kw); err != nil {
				writeError(w, http.StatusInternalServerError, "could not update domain")
				return
			}
		}
		if body.Status != nil && *body.Status != "" {
			if err := s.domains.UpdateStatus(r.Context(), domainID, *body.Status); err != nil {
				writeError(w, http.StatusInternalServerError, "could not update domain")
				return
			}
		}
		if body.Country != nil {
			d.TargetCountry = strings.TrimSpace(*body.Country)
		}
		if body.Language != nil {
			d.TargetLanguage = strings.TrimSpace(*body.Language)
		}
		if body.ExcludeDomains != nil {
			ex := strings.TrimSpace(*body.ExcludeDomains)
			if ex == "" {
				d.ExcludeDomains = sql.NullString{}
			} else {
				d.ExcludeDomains = sqlstore.NullableString(ex)
			}
		}
		var server sql.NullString
		if body.ServerID != nil {
			sv := strings.TrimSpace(*body.ServerID)
			if sv != "" {
				server = sqlstore.NullableString(sv)
			} else {
				server = sql.NullString{}
			}
		} else {
			server = d.ServerID
		}
		linkAnchor := d.LinkAnchorText
		if body.LinkAnchorText != nil {
			val := strings.TrimSpace(*body.LinkAnchorText)
			if val == "" {
				linkAnchor = sql.NullString{}
			} else {
				linkAnchor = sqlstore.NullableString(val)
			}
		}
		linkAcceptor := d.LinkAcceptorURL
		if body.LinkAcceptorURL != nil {
			val := strings.TrimSpace(*body.LinkAcceptorURL)
			if val == "" {
				linkAcceptor = sql.NullString{}
			} else {
				linkAcceptor = sqlstore.NullableString(val)
			}
		}
		_, _ = s.domains.UpdateExtras(r.Context(), domainID, d.TargetCountry, d.TargetLanguage, d.ExcludeDomains, server)
		if body.LinkAnchorText != nil || body.LinkAcceptorURL != nil {
			if _, err := s.domains.UpdateLinkSettings(r.Context(), domainID, linkAnchor, linkAcceptor); err != nil {
				writeError(w, http.StatusInternalServerError, "could not update domain")
				return
			}
		}
		d, _ = s.domains.Get(r.Context(), domainID)
		writeJSON(w, http.StatusOK, toDomainDTO(d))
	case http.MethodDelete:
		if err := s.domains.Delete(r.Context(), domainID); err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete domain")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleDomainGenerate(w http.ResponseWriter, r *http.Request, domainID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	d, p, err := s.authorizeDomain(r.Context(), domainID)
	if err != nil {
		respondAuthzError(w, err, "domain not found")
		return
	}
	// Проверяем права на запуск генерации (нужны права editor или owner)
	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), p.ID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}
	if !hasProjectPermission(user.Role, memberRole, "editor") {
		writeError(w, http.StatusForbidden, "insufficient permissions: editor role required to run generation")
		return
	}

	var req struct {
		ForceStep string `json:"force_step,omitempty"`
	}
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	forceStep := strings.TrimSpace(req.ForceStep)

	// Проверяем наличие API ключа у владельца проекта (или у админа, если он запускает)
	keyOwnerEmail := p.UserEmail
	if strings.EqualFold(user.Role, "admin") {
		keyOwnerEmail = user.Email
	}
	encKey, err := s.svc.GetUserAPIKeyEncrypted(r.Context(), keyOwnerEmail)
	if err != nil || len(encKey) == 0 {
		if strings.EqualFold(user.Role, "admin") {
			writeError(w, http.StatusBadRequest, "API key not configured for admin user. Please configure API key in profile settings.")
		} else {
			writeError(w, http.StatusBadRequest, "API key not configured for project owner. Please configure API key in profile settings.")
		}
		return
	}

	// Не допускаем параллельных запусков для домена.
	// Проверяем наличие активной генерации (не только статус домена, но и статус последней генерации)
	activeStatuses := map[string]bool{
		"pending":         true,
		"processing":      true,
		"pause_requested": true,
		"cancelling":      true,
	}
	forceAllowed := map[string]bool{
		"error":     true,
		"paused":    true,
		"success":   true,
		"cancelled": true,
	}
	gens, err := s.generations.ListByDomain(r.Context(), domainID)
	if err == nil && len(gens) > 0 {
		lastGen := gens[0]
		if activeStatuses[lastGen.Status] {
			writeError(w, http.StatusConflict, "generation already running for this domain")
			return
		}
		if forceStep != "" && !forceAllowed[lastGen.Status] {
			writeError(w, http.StatusConflict, "generation already running for this domain")
			return
		}
	}
	if strings.TrimSpace(d.MainKeyword) == "" {
		writeError(w, http.StatusBadRequest, "keyword is required")
		return
	}
	genID := uuid.NewString()

	// Попытка скопировать артефакты из последней успешной генерации, если делаем force rerun
	var baseArtifacts []byte
	if forceStep != "" {
		if last, err := s.generations.GetLastSuccessfulByDomain(r.Context(), domainID); err == nil && len(last.Artifacts) > 0 {
			baseArtifacts = last.Artifacts
		} else if last, err := s.generations.GetLastByDomain(r.Context(), domainID); err == nil && len(last.Artifacts) > 0 {
			baseArtifacts = last.Artifacts
		}
	}

	gen := sqlstore.Generation{
		ID:          genID,
		DomainID:    domainID,
		RequestedBy: sqlstore.NullableString(user.Email),
		Status:      "pending",
		Progress:    0,
		Logs:        json.RawMessage(`[]`),
		Artifacts:   baseArtifacts,
	}
	if err := s.generations.Create(r.Context(), gen); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create generation")
		return
	}
	_ = s.domains.SetLastGeneration(r.Context(), domainID, genID)
	_ = s.domains.UpdateStatus(r.Context(), domainID, "processing")

	if s.tasks == nil {
		writeError(w, http.StatusServiceUnavailable, "queue unavailable")
		return
	}
	task := tasks.NewGenerateTask(genID, domainID, forceStep)
	queue := s.genQueueForProject(d.ProjectID)
	if _, err := s.tasks.Enqueue(r.Context(), task, asynq.Queue(queue)); err != nil {
		if s.logger != nil {
			s.logger.Warnf("failed to enqueue generation: %v", err)
		}
		writeError(w, http.StatusInternalServerError, "could not enqueue generation")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"id": genID, "status": "queued"})
}

func (s *Server) handleDomainGenerations(w http.ResponseWriter, r *http.Request, domainID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, _, err := s.authorizeDomain(r.Context(), domainID); err != nil {
		respondAuthzError(w, err, "domain not found")
		return
	}
	list, err := s.generations.ListByDomain(r.Context(), domainID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list generations")
		return
	}
	domainMap := map[string]string{}
	for _, g := range list {
		if _, ok := domainMap[g.DomainID]; ok {
			continue
		}
		if d, err := s.domains.Get(r.Context(), g.DomainID); err == nil {
			domainMap[g.DomainID] = d.URL
		}
	}
	resp := make([]generationDTO, 0, len(list))
	for _, g := range list {
		var url *string
		if v, ok := domainMap[g.DomainID]; ok {
			val := v
			url = &val
		}
		resp = append(resp, toGenerationDTOWithDomain(g, url))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDomainFiles(w http.ResponseWriter, r *http.Request, domainID string, parts []string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.siteFiles == nil || s.fileEdits == nil {
		writeError(w, http.StatusInternalServerError, "file storage not configured")
		return
	}
	domain, project, err := s.authorizeDomain(r.Context(), domainID)
	if err != nil {
		respondAuthzError(w, err, "domain not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), project.ID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}

	// История файла: /api/domains/:id/files/:fileId/history
	if len(parts) == 2 && parts[1] == "history" {
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		fileID, err := url.PathUnescape(parts[0])
		if err != nil || strings.TrimSpace(fileID) == "" {
			writeError(w, http.StatusBadRequest, "invalid file id")
			return
		}
		s.handleFileHistory(w, r, domain.ID, fileID)
		return
	}

	// Список файлов
	if len(parts) == 0 || parts[0] == "" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		files, err := s.siteFiles.List(r.Context(), domainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list files")
			return
		}
		resp := make([]fileDTO, 0, len(files))
		for _, f := range files {
			resp = append(resp, fileDTO{
				ID:        f.ID,
				Path:      f.Path,
				Size:      f.SizeBytes,
				MimeType:  f.MimeType,
				UpdatedAt: f.UpdatedAt,
			})
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	rawPath, err := joinURLPath(parts)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	cleanPath, err := sanitizeFilePath(rawPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		s.handleGetFile(w, r, domain, cleanPath)
	case http.MethodPut:
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		s.handleUpdateFile(w, r, domain, cleanPath, user.Email)
	case http.MethodDelete:
		if !strings.EqualFold(user.Role, "admin") {
			writeError(w, http.StatusForbidden, "admin only")
			return
		}
		s.handleDeleteFile(w, r, domain, cleanPath)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleDomainLinks(w http.ResponseWriter, r *http.Request, domainID string, parts []string) {
	type linkTaskImportItem struct {
		AnchorText        string `json:"anchor_text"`
		TargetURL         string `json:"target_url"`
		ScheduledFor      string `json:"scheduled_for"`
		ScheduledForCamel string `json:"scheduledFor"`
	}

	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.linkTasks == nil {
		writeError(w, http.StatusInternalServerError, "link tasks not configured")
		return
	}
	domain, project, err := s.authorizeDomain(r.Context(), domainID)
	if err != nil {
		respondAuthzError(w, err, "domain not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), project.ID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}

	action := ""
	if len(parts) > 0 {
		action = parts[0]
	}
	if action == "import" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !ensureJSON(w, r) {
			return
		}
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		defer r.Body.Close()
		var body struct {
			Items []linkTaskImportItem `json:"items"`
			Text  string               `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if len(body.Items) == 0 && strings.TrimSpace(body.Text) == "" {
			writeError(w, http.StatusBadRequest, "no tasks provided")
			return
		}

		records := body.Items
		if len(records) == 0 {
			lines := strings.Split(body.Text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, ",", 3)
				item := linkTaskImportItem{
					AnchorText: strings.TrimSpace(parts[0]),
				}
				if len(parts) > 1 {
					item.TargetURL = strings.TrimSpace(parts[1])
				}
				if len(parts) > 2 {
					item.ScheduledFor = strings.TrimSpace(parts[2])
				}
				records = append(records, item)
			}
		}

		created := 0
		now := time.Now().UTC()
		for _, item := range records {
			anchor := strings.TrimSpace(item.AnchorText)
			target := strings.TrimSpace(item.TargetURL)
			if anchor == "" || target == "" {
				continue
			}
			scheduledFor, err := parseScheduledFor(item.ScheduledFor, item.ScheduledForCamel, now)
			if err != nil {
				if s.logger != nil {
					s.logger.Warnf("import link task scheduled_for invalid: %v", err)
				}
				continue
			}
			task := sqlstore.LinkTask{
				ID:           uuid.NewString(),
				DomainID:     domain.ID,
				AnchorText:   anchor,
				TargetURL:    target,
				ScheduledFor: scheduledFor,
				Action:       "insert",
				Status:       "pending",
				Attempts:     0,
				CreatedBy:    user.Email,
				CreatedAt:    now,
			}
			if err := s.linkTasks.Create(r.Context(), task); err != nil {
				if s.logger != nil {
					s.logger.Warnf("import link task failed: %v", err)
				}
				continue
			}
			created++
		}
		writeJSON(w, http.StatusCreated, map[string]any{"created": created})
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		filters, err := parseLinkTaskFilters(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		list, err := s.linkTasks.ListByDomain(r.Context(), domainID, filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list link tasks")
			return
		}
		resp := make([]linkTaskDTO, 0, len(list))
		for _, task := range list {
			resp = append(resp, toLinkTaskDTO(task))
		}
		writeJSON(w, http.StatusOK, resp)
	case http.MethodPost:
		if !ensureJSON(w, r) {
			return
		}
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		defer r.Body.Close()
		var body struct {
			AnchorText        string `json:"anchor_text"`
			TargetURL         string `json:"target_url"`
			ScheduledFor      string `json:"scheduled_for"`
			ScheduledForCamel string `json:"scheduledFor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		anchor := strings.TrimSpace(body.AnchorText)
		target := strings.TrimSpace(body.TargetURL)
		if anchor == "" || target == "" {
			writeError(w, http.StatusBadRequest, "anchor_text and target_url are required")
			return
		}
		now := time.Now().UTC()
		scheduledFor, err := parseScheduledFor(body.ScheduledFor, body.ScheduledForCamel, now)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid scheduled_for")
			return
		}
		task := sqlstore.LinkTask{
			ID:           uuid.NewString(),
			DomainID:     domain.ID,
			AnchorText:   anchor,
			TargetURL:    target,
			ScheduledFor: scheduledFor,
			Action:       "insert",
			Status:       "pending",
			Attempts:     0,
			CreatedBy:    user.Email,
			CreatedAt:    now,
		}
		if err := s.linkTasks.Create(r.Context(), task); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create link task")
			return
		}
		writeJSON(w, http.StatusCreated, toLinkTaskDTO(task))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleDomainIndexChecks(w http.ResponseWriter, r *http.Request, domainID string, parts []string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index checks not configured")
		return
	}

	domain, project, err := s.authorizeDomain(r.Context(), domainID)
	if err != nil {
		respondAuthzError(w, err, "domain not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), project.ID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}

	if len(parts) > 0 {
		if len(parts) > 1 && parts[1] == "history" {
			s.handleDomainIndexCheckHistory(w, r, domainID, parts[0], memberRole)
			return
		}
		if len(parts) == 1 && parts[0] == "stats" {
			s.handleDomainIndexChecksStats(w, r, domainID, memberRole)
			return
		}
		if len(parts) == 1 && parts[0] == "calendar" {
			s.handleDomainIndexChecksCalendar(w, r, domainID, memberRole)
			return
		}
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		filters := parseIndexCheckFilters(r, 50, 200)
		list, err := s.indexChecks.ListByDomain(r.Context(), domainID, filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list index checks")
			return
		}
		total, err := s.indexChecks.CountByDomain(r.Context(), domainID, filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not count index checks")
			return
		}
		resp := make([]indexCheckDTO, 0, len(list))
		for _, check := range list {
			dto := toIndexCheckDTO(check)
			url := strings.TrimSpace(domain.URL)
			if url != "" {
				dto.DomainURL = &url
			}
			pid := project.ID
			dto.ProjectID = &pid
			resp = append(resp, dto)
		}
		writeJSON(w, http.StatusOK, indexCheckListDTO{Items: resp, Total: total})
	case http.MethodPost:
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		if !isDomainPublished(domain) {
			writeError(w, http.StatusConflict, "domain is not published")
			return
		}
		now := time.Now().UTC()
		check, created, err := s.upsertManualIndexCheck(r.Context(), domainID, now)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not start index check")
			return
		}
		dto := toIndexCheckDTO(check)
		if url := strings.TrimSpace(domain.URL); url != "" {
			dto.DomainURL = &url
		}
		pid := project.ID
		dto.ProjectID = &pid
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeJSON(w, status, dto)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleDomainIndexCheckHistory(w http.ResponseWriter, r *http.Request, domainID, checkID, memberRole string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.checkHistory == nil || s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index check history not configured")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !hasProjectPermission(user.Role, memberRole, "viewer") {
		writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
		return
	}
	checkID = strings.TrimSpace(checkID)
	if checkID == "" {
		writeError(w, http.StatusBadRequest, "missing check id")
		return
	}
	check, err := s.indexChecks.Get(r.Context(), checkID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "check not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load check")
		return
	}
	if check.DomainID != domainID {
		writeError(w, http.StatusNotFound, "check not found")
		return
	}

	limit := parseLimitParam(r, 50, 200)
	list, err := s.checkHistory.ListByCheck(r.Context(), checkID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load check history")
		return
	}
	resp := make([]indexCheckHistoryDTO, 0, len(list))
	for _, item := range list {
		resp = append(resp, toIndexCheckHistoryDTO(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDomainIndexChecksStats(w http.ResponseWriter, r *http.Request, domainID, memberRole string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index checks not configured")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !hasProjectPermission(user.Role, memberRole, "viewer") {
		writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
		return
	}
	now := time.Now().UTC()
	filters := parseIndexCheckFilters(r, 0, 0)
	from, to := resolveIndexCheckStatsRange(&filters, now)
	filters.Limit = 0
	filters.Offset = 0
	stats, err := s.indexChecks.AggregateStatsByDomain(r.Context(), domainID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not aggregate index checks")
		return
	}
	daily, err := s.indexChecks.AggregateDailyByDomain(r.Context(), domainID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not aggregate index checks")
		return
	}
	writeJSON(w, http.StatusOK, buildIndexCheckStatsDTO(stats, daily, from, to))
}

func (s *Server) handleDomainIndexChecksCalendar(w http.ResponseWriter, r *http.Request, domainID, memberRole string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index checks not configured")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !hasProjectPermission(user.Role, memberRole, "viewer") {
		writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
		return
	}
	now := time.Now().UTC()
	filters := parseIndexCheckFilters(r, 0, 0)
	_, _ = resolveIndexCheckCalendarRange(&filters, r, now)
	filters.Limit = 0
	filters.Offset = 0
	daily, err := s.indexChecks.AggregateDailyByDomain(r.Context(), domainID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not aggregate index checks")
		return
	}
	resp := make([]indexCheckDailyDTO, 0, len(daily))
	for _, item := range daily {
		resp = append(resp, toIndexCheckDailyDTO(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) upsertManualIndexCheck(ctx context.Context, domainID string, now time.Time) (sqlstore.IndexCheck, bool, error) {
	if s.indexChecks == nil {
		return sqlstore.IndexCheck{}, false, errors.New("index checks not configured")
	}
	today := dateOnlyUTC(now)
	check, err := s.indexChecks.GetByDomainAndDate(ctx, domainID, today)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			newCheck := sqlstore.IndexCheck{
				ID:        uuid.NewString(),
				DomainID:  domainID,
				CheckDate: today,
				Status:    "pending",
				Attempts:  0,
				NextRetryAt: sql.NullTime{
					Time:  now,
					Valid: true,
				},
				CreatedAt: now,
			}
			if err := s.indexChecks.Create(ctx, newCheck); err != nil {
				return sqlstore.IndexCheck{}, false, err
			}
			return newCheck, true, nil
		}
		return sqlstore.IndexCheck{}, false, err
	}
	if err := s.indexChecks.ResetForManual(ctx, check.ID, now); err != nil {
		return sqlstore.IndexCheck{}, false, err
	}
	check.Status = "pending"
	check.Attempts = 0
	check.IsIndexed = sql.NullBool{}
	check.ErrorMessage = sql.NullString{}
	check.LastAttemptAt = sql.NullTime{}
	check.CompletedAt = sql.NullTime{}
	check.NextRetryAt = sql.NullTime{Time: now, Valid: true}
	return *check, false, nil
}

func (s *Server) handleDomainLinkRun(w http.ResponseWriter, r *http.Request, domainID string, parts []string) {
	if len(parts) == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	action := "insert"
	switch parts[0] {
	case "run":
		action = "insert"
	case "remove":
		action = "remove"
	default:
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.linkTasks == nil {
		writeError(w, http.StatusInternalServerError, "link tasks not configured")
		return
	}
	if err := requireApprovedUser(user); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	domain, project, err := s.authorizeDomain(r.Context(), domainID)
	if err != nil {
		respondAuthzError(w, err, "domain not found")
		return
	}
	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), project.ID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}
	if !hasProjectPermission(user.Role, memberRole, "editor") {
		writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
		return
	}

	anchor := strings.TrimSpace(domain.LinkAnchorText.String)
	target := strings.TrimSpace(domain.LinkAcceptorURL.String)
	if !domain.LinkAnchorText.Valid || !domain.LinkAcceptorURL.Valid || anchor == "" || target == "" {
		if action == "remove" && domain.LinkLastTaskID.Valid && strings.TrimSpace(domain.LinkLastTaskID.String) != "" {
			prevTask, err := s.linkTasks.Get(r.Context(), strings.TrimSpace(domain.LinkLastTaskID.String))
			if err == nil && prevTask != nil {
				anchor = strings.TrimSpace(prevTask.AnchorText)
				target = strings.TrimSpace(prevTask.TargetURL)
			}
		}
	}
	if anchor == "" || target == "" {
		writeError(w, http.StatusBadRequest, "link settings not configured")
		return
	}

	now := time.Now().UTC()
	activeTask, err := s.findActiveLinkTask(r.Context(), domainID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load link tasks")
		return
	}

	if activeTask != nil {
		status := "pending"
		attempts := 0
		nullStr := sql.NullString{}
		nullTime := sql.NullTime{}
		updates := sqlstore.LinkTaskUpdates{
			AnchorText:       &anchor,
			TargetURL:        &target,
			Action:           &action,
			Status:           &status,
			Attempts:         &attempts,
			ScheduledFor:     &now,
			FoundLocation:    &nullStr,
			GeneratedContent: &nullStr,
			ErrorMessage:     &nullStr,
			CompletedAt:      &nullTime,
		}
		if err := s.linkTasks.Update(r.Context(), activeTask.ID, updates); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update link task")
			return
		}
		updated, _ := s.linkTasks.Get(r.Context(), activeTask.ID)
		if updated == nil {
			writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
			return
		}
		writeJSON(w, http.StatusOK, toLinkTaskDTO(*updated))
		return
	}

	task := sqlstore.LinkTask{
		ID:           uuid.NewString(),
		DomainID:     domain.ID,
		AnchorText:   anchor,
		TargetURL:    target,
		ScheduledFor: now,
		Action:       action,
		Status:       "pending",
		Attempts:     0,
		CreatedBy:    user.Email,
		CreatedAt:    now,
	}
	if err := s.linkTasks.Create(r.Context(), task); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create link task")
		return
	}
	writeJSON(w, http.StatusCreated, toLinkTaskDTO(task))
}

func (s *Server) findActiveLinkTask(ctx context.Context, domainID string) (*sqlstore.LinkTask, error) {
	activeStatuses := []string{"pending", "searching", "found", "removing"}
	for _, status := range activeStatuses {
		filters := sqlstore.LinkTaskFilters{Status: &status, Limit: 1}
		list, err := s.linkTasks.ListByDomain(ctx, domainID, filters)
		if err != nil {
			return nil, err
		}
		if len(list) > 0 {
			task := list[0]
			return &task, nil
		}
	}
	return nil, nil
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, relPath string) {
	file, err := s.siteFiles.GetByPath(r.Context(), domain.ID, relPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	domainDir, err := domainFilesDir(domain.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain")
		return
	}
	fullPath := filepath.Join(domainDir, filepath.FromSlash(relPath))
	if err := ensurePathWithin(domainDir, fullPath); err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not read file")
		return
	}
	mimeType := file.MimeType
	if mimeType == "" {
		mimeType = detectMimeType(relPath, content)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"content":  string(content),
		"mimeType": mimeType,
	})
}

func (s *Server) handleUpdateFile(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, relPath, editedBy string) {
	if !ensureJSON(w, r) {
		return
	}
	defer r.Body.Close()
	var body struct {
		Content     string `json:"content"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	file, err := s.siteFiles.GetByPath(r.Context(), domain.ID, relPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}

	domainDir, err := domainFilesDir(domain.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain")
		return
	}
	fullPath := filepath.Join(domainDir, filepath.FromSlash(relPath))
	if err := ensurePathWithin(domainDir, fullPath); err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	oldContent, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not read file")
		return
	}

	newBytes := []byte(body.Content)
	detected := detectMimeType(relPath, newBytes)
	if err := validateMimeType(relPath, detected, file.MimeType); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "could not write file")
		return
	}
	if err := os.WriteFile(fullPath, newBytes, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "could not write file")
		return
	}

	if err := s.siteFiles.Update(r.Context(), file.ID, newBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "could not update file metadata")
		return
	}

	beforeHash := sha256.Sum256(oldContent)
	afterHash := sha256.Sum256(newBytes)
	edit := sqlstore.FileEdit{
		ID:                uuid.NewString(),
		FileID:            file.ID,
		EditedBy:          editedBy,
		ContentBeforeHash: sql.NullString{String: hex.EncodeToString(beforeHash[:]), Valid: true},
		ContentAfterHash:  sql.NullString{String: hex.EncodeToString(afterHash[:]), Valid: true},
		EditType:          "manual",
	}
	if strings.TrimSpace(body.Description) != "" {
		edit.EditDescription = sql.NullString{String: strings.TrimSpace(body.Description), Valid: true}
	}
	if err := s.fileEdits.Create(r.Context(), edit); err != nil {
		writeError(w, http.StatusInternalServerError, "could not log file edit")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, relPath string) {
	file, err := s.siteFiles.GetByPath(r.Context(), domain.ID, relPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	domainDir, err := domainFilesDir(domain.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain")
		return
	}
	fullPath := filepath.Join(domainDir, filepath.FromSlash(relPath))
	if err := ensurePathWithin(domainDir, fullPath); err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		writeError(w, http.StatusInternalServerError, "could not delete file")
		return
	}
	if err := s.siteFiles.Delete(r.Context(), file.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete file metadata")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleFileHistory(w http.ResponseWriter, r *http.Request, domainID, fileID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	file, err := s.siteFiles.Get(r.Context(), fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	if file.DomainID != domainID {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	list, err := s.fileEdits.ListByFile(r.Context(), fileID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list file history")
		return
	}
	resp := make([]fileEditDTO, 0, len(list))
	for _, e := range list {
		item := fileEditDTO{
			ID:        e.ID,
			EditedBy:  e.EditedBy,
			EditType:  e.EditType,
			CreatedAt: e.CreatedAt,
		}
		if e.EditDescription.Valid {
			item.Description = e.EditDescription.String
		}
		resp = append(resp, item)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGenerations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	page := 1
	if v := strings.TrimSpace(r.URL.Query().Get("page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	offset := (page - 1) * limit
	lite := false
	if v := r.URL.Query().Get("lite"); v != "" {
		if v == "1" || strings.EqualFold(v, "true") {
			lite = true
		}
	}
	var (
		list []sqlstore.Generation
		err  error
	)
	if strings.EqualFold(user.Role, "admin") {
		if lite {
			list, err = s.generations.ListRecentAllLite(r.Context(), limit, offset, search)
		} else {
			list, err = s.generations.ListRecentAll(r.Context(), limit, offset, search)
		}
	} else {
		if lite {
			list, err = s.generations.ListRecentByUserLite(r.Context(), user.Email, limit, offset, search)
		} else {
			list, err = s.generations.ListRecentByUser(r.Context(), user.Email, limit, offset, search)
		}
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list generations")
		return
	}
	domainMap := map[string]string{}
	if s.domains != nil {
		for _, g := range list {
			if _, ok := domainMap[g.DomainID]; ok {
				continue
			}
			if d, err := s.domains.Get(r.Context(), g.DomainID); err == nil && strings.TrimSpace(d.URL) != "" {
				domainMap[g.DomainID] = d.URL
			}
		}
	}
	resp := make([]generationDTO, 0, len(list))
	for _, g := range list {
		var url *string
		if v, ok := domainMap[g.DomainID]; ok {
			val := v
			url = &val
		}
		resp = append(resp, toGenerationDTOWithDomain(g, url))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) genQueueForProject(projectID string) string {
	return tasks.QueueForProject(projectID, s.cfg.GenQueueShards)
}

func (s *Server) handleGenerationByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/generations/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		gen, err := s.generations.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		d, err := s.domains.Get(r.Context(), gen.DomainID)
		if err != nil {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		if _, err := s.authorizeProject(r.Context(), d.ProjectID); err != nil {
			respondAuthzError(w, err, "forbidden")
			return
		}
		writeJSON(w, http.StatusOK, toGenerationDTO(gen))
	case http.MethodPatch:
		s.handleGenerationAction(w, r, id)
	case http.MethodDelete:
		gen, err := s.generations.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		d, err := s.domains.Get(r.Context(), gen.DomainID)
		if err != nil {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		p, err := s.authorizeProject(r.Context(), d.ProjectID)
		if err != nil {
			respondAuthzError(w, err, "forbidden")
			return
		}
		// Проверяем права на удаление (нужны права editor или owner)
		user, ok := currentUserFromContext(r.Context())
		if !ok || user.Email == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		memberRole, err := s.getProjectMemberRole(r.Context(), p.ID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required to delete generation")
			return
		}
		if err := s.generations.Delete(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete generation")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleGenerationAction обрабатывает действия pause/resume/cancel
func (s *Server) handleGenerationAction(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Action string `json:"action"` // "pause", "resume", "cancel"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	gen, err := s.generations.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	d, err := s.domains.Get(r.Context(), gen.DomainID)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	p, err := s.authorizeProject(r.Context(), d.ProjectID)
	if err != nil {
		respondAuthzError(w, err, "forbidden")
		return
	}

	// Проверяем права (нужны права editor или owner)
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	memberRole, err := s.getProjectMemberRole(r.Context(), p.ID, user.Email)
	if err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	if !hasProjectPermission(user.Role, memberRole, "editor") {
		writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
		return
	}

	switch body.Action {
	case "pause":
		// Если задача еще не стартовала, можно сразу поставить на паузу
		if gen.Status == "pending" {
			if err := s.generations.UpdateStatus(r.Context(), id, "paused", gen.Progress, nil); err != nil {
				writeError(w, http.StatusInternalServerError, "could not pause generation")
				return
			}
			_ = s.domains.UpdateStatus(r.Context(), gen.DomainID, "waiting")
			writeJSON(w, http.StatusOK, map[string]string{"status": "paused", "message": "generation paused"})
			return
		}
		// Пауза возможна только из состояния PROCESSING
		if gen.Status != "processing" {
			writeError(w, http.StatusBadRequest, "can only pause processing generation")
			return
		}
		// Устанавливаем статус PAUSE_REQUESTED - worker сам переведет в PAUSED после завершения текущего шага
		if err := s.generations.UpdateStatus(r.Context(), id, "pause_requested", gen.Progress, nil); err != nil {
			writeError(w, http.StatusInternalServerError, "could not pause generation")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "pause_requested", "message": "pause requested, will pause after current step completes"})

	case "resume":
		// Возобновление возможно только из состояния PAUSED
		if gen.Status != "paused" {
			writeError(w, http.StatusBadRequest, "can only resume paused generation")
			return
		}
		// Устанавливаем статус PENDING (чтобы worker мог подхватить задачу) и перезапускаем задачу
		// Если чекпоинта нет, задача начнется сначала (это нормально)
		if err := s.generations.UpdateStatus(r.Context(), id, "pending", gen.Progress, nil); err != nil {
			writeError(w, http.StatusInternalServerError, "could not resume generation")
			return
		}
		// Обновляем статус домена обратно на processing
		_ = s.domains.UpdateStatus(r.Context(), gen.DomainID, "processing")
		// Перезапускаем задачу через очередь (если есть enqueuer)
		if s.tasks != nil {
			task := tasks.NewGenerateTask(id, gen.DomainID, "")
			queue := "default"
			if s.domains != nil {
				if d, err := s.domains.Get(r.Context(), gen.DomainID); err == nil {
					queue = s.genQueueForProject(d.ProjectID)
				}
			}
			if _, err := s.tasks.Enqueue(r.Context(), task, asynq.Queue(queue)); err != nil {
				if s.logger != nil {
					s.logger.Warnf("failed to enqueue resumed generation: %v", err)
				}
				// Не возвращаем ошибку, так как статус уже обновлен
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "pending", "message": "generation resumed"})

	case "cancel":
		// Отмена возможна из PROCESSING, PAUSE_REQUESTED или PAUSED
		if gen.Status == "pending" {
			if err := s.generations.UpdateStatus(r.Context(), id, "cancelled", 0, nil); err != nil {
				writeError(w, http.StatusInternalServerError, "could not cancel generation")
				return
			}
			_ = s.domains.UpdateStatus(r.Context(), gen.DomainID, "waiting")
			writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled", "message": "generation cancelled"})
			return
		}
		if gen.Status != "processing" && gen.Status != "pause_requested" && gen.Status != "paused" {
			writeError(w, http.StatusBadRequest, "can only cancel processing, pause_requested or paused generation")
			return
		}

		if gen.Status == "paused" {
			// Если задача на паузе, сразу очищаем чекпоинт и устанавливаем CANCELLED
			if err := s.generations.ClearCheckpoint(r.Context(), id); err != nil {
				writeError(w, http.StatusInternalServerError, "could not clear checkpoint")
				return
			}
			if err := s.generations.UpdateStatus(r.Context(), id, "cancelled", 0, nil); err != nil {
				writeError(w, http.StatusInternalServerError, "could not cancel generation")
				return
			}
			// Очищаем artifacts и logs для отмененной задачи
			emptyArtifacts, _ := json.Marshal(map[string]interface{}{})
			emptyLogs, _ := json.Marshal([]string{})
			_ = s.generations.UpdateFull(r.Context(), id, "cancelled", 0, nil, emptyLogs, emptyArtifacts, nil, nil, nil)
		} else {
			// Если задача выполняется, устанавливаем CANCELLING - worker сам переведет в CANCELLED
			if err := s.generations.UpdateStatus(r.Context(), id, "cancelling", gen.Progress, nil); err != nil {
				writeError(w, http.StatusInternalServerError, "could not cancel generation")
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": gen.Status, "message": "cancellation requested"})

	default:
		writeError(w, http.StatusBadRequest, "invalid action: must be pause, resume, or cancel")
	}
}

// --- Admin handlers ---

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	users, err := s.svc.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list users")
		return
	}
	resp := make([]adminUserDTO, 0, len(users))
	for _, u := range users {
		resp = append(resp, toAdminUserDTO(u))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAdminUserRoute(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	if rest == "" {
		writeError(w, http.StatusBadRequest, "missing user email")
		return
	}
	parts := strings.Split(rest, "/")
	emailPart, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(emailPart) == "" {
		writeError(w, http.StatusBadRequest, "invalid user email")
		return
	}
	targetEmail := strings.ToLower(strings.TrimSpace(emailPart))
	if len(parts) > 1 && parts[1] == "projects" {
		s.handleAdminUserProjects(w, r, targetEmail)
		return
	}
	if len(parts) > 1 && parts[1] == "password" {
		s.handleAdminUserPassword(w, r, targetEmail)
		return
	}

	switch r.Method {
	case http.MethodGet:
		u, err := s.svc.GetUser(r.Context(), targetEmail)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeJSON(w, http.StatusOK, toAdminUserDTO(u))
	case http.MethodPatch:
		if !ensureJSON(w, r) {
			return
		}
		defer r.Body.Close()
		var body struct {
			Role       *string `json:"role"`
			IsApproved *bool   `json:"isApproved"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		// Получаем текущего пользователя для проверок безопасности
		user, ok := currentUserFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		updated := false
		if body.Role != nil {
			// Защита: админ не может изменить свою собственную роль
			if strings.EqualFold(user.Email, targetEmail) {
				writeError(w, http.StatusForbidden, "cannot change your own role")
				return
			}

			// Защита: админ не может понизить другого админа
			targetUser, err := s.svc.GetUser(r.Context(), targetEmail)
			if err == nil && strings.EqualFold(targetUser.Role, "admin") {
				writeError(w, http.StatusForbidden, "cannot change admin role")
				return
			}

			if err := s.svc.SetUserRole(r.Context(), targetEmail, *body.Role); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			updated = true
		}
		if body.IsApproved != nil {
			if err := s.svc.SetUserApproval(r.Context(), targetEmail, *body.IsApproved); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			updated = true
		}
		if !updated {
			writeError(w, http.StatusBadRequest, "no changes supplied")
			return
		}
		u, err := s.svc.GetUser(r.Context(), targetEmail)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeJSON(w, http.StatusOK, toAdminUserDTO(u))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAdminUserPassword(w http.ResponseWriter, r *http.Request, email string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	defer r.Body.Close()
	var body struct {
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.NewPassword) == "" {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.svc.SetPasswordAdmin(r.Context(), email, body.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

func (s *Server) handleAdminUserProjects(w http.ResponseWriter, r *http.Request, email string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, err := s.svc.GetUser(r.Context(), email); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	projects, err := s.projects.ListByUser(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load projects")
		return
	}
	// Используем новый метод для получения ownerHasApiKey
	projectDTOs := make([]projectDTO, 0, len(projects))
	for _, p := range projects {
		projectDTOs = append(projectDTOs, s.toProjectDTO(r.Context(), p))
	}
	writeJSON(w, http.StatusOK, projectDTOs)
}

func (s *Server) handleProjectMembers(w http.ResponseWriter, r *http.Request, projectID string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Проверяем доступ к проекту
	project, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	// Получаем роль пользователя в проекте
	memberRole, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
	if err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Проверяем, что пользователь имеет права на управление участниками (owner или admin)
	if !strings.EqualFold(user.Role, "admin") && memberRole != "owner" {
		writeError(w, http.StatusForbidden, "only owner or admin can manage members")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Получаем список участников (включая владельца)
		members, err := s.projectMembers.ListByProject(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load members")
			return
		}

		// Добавляем владельца в список
		response := []projectMemberDTO{
			{
				Email:     project.UserEmail,
				Role:      "owner",
				CreatedAt: project.CreatedAt,
			},
		}
		for _, m := range members {
			response = append(response, projectMemberDTO{
				Email:     m.UserEmail,
				Role:      m.Role,
				CreatedAt: m.CreatedAt,
			})
		}
		writeJSON(w, http.StatusOK, response)

	case http.MethodPost:
		// Добавить участника
		if !ensureJSON(w, r) {
			return
		}
		defer r.Body.Close()
		var body struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		email := strings.ToLower(strings.TrimSpace(body.Email))
		if email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}
		if email == project.UserEmail {
			writeError(w, http.StatusBadRequest, "cannot add owner as member")
			return
		}
		role := strings.ToLower(strings.TrimSpace(body.Role))
		if role == "" {
			role = "editor"
		}
		if err := s.projectMembers.Add(r.Context(), projectID, email, role); err != nil {
			// Безопасный вывод ошибки - не показываем детали БД
			errMsg := err.Error()
			if strings.Contains(errMsg, "foreign key") || strings.Contains(errMsg, "user_email_fkey") {
				errMsg = "user not found"
			} else if strings.Contains(errMsg, "SQLSTATE") || strings.Contains(errMsg, "constraint") {
				errMsg = "failed to add member"
			}
			writeError(w, http.StatusBadRequest, errMsg)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "member added"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectMemberByEmail(w http.ResponseWriter, r *http.Request, projectID, email string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Проверяем доступ к проекту
	project, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	// Проверяем, что пользователь имеет права на управление участниками
	memberRole, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
	if err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	if !strings.EqualFold(user.Role, "admin") && memberRole != "owner" {
		writeError(w, http.StatusForbidden, "only owner or admin can manage members")
		return
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if email == project.UserEmail {
		writeError(w, http.StatusBadRequest, "cannot modify owner")
		return
	}

	switch r.Method {
	case http.MethodPatch:
		// Изменить роль участника
		if !ensureJSON(w, r) {
			return
		}
		defer r.Body.Close()
		var body struct {
			Role string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		role := strings.ToLower(strings.TrimSpace(body.Role))
		if role == "" {
			writeError(w, http.StatusBadRequest, "role is required")
			return
		}
		if err := s.projectMembers.UpdateRole(r.Context(), projectID, email, role); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "role updated"})

	case http.MethodDelete:
		// Удалить участника
		if err := s.projectMembers.Remove(r.Context(), projectID, email); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "member removed"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectSchedules(w http.ResponseWriter, r *http.Request, projectID string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.schedules == nil {
		writeError(w, http.StatusInternalServerError, "schedules not configured")
		return
	}

	project, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}

	switch r.Method {
	case http.MethodGet:
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		list, err := s.schedules.List(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list schedules")
			return
		}
		resp := make([]scheduleDTO, 0, len(list))
		for _, sched := range list {
			dto := toScheduleDTO(sched)
			if dto.Timezone == nil || strings.TrimSpace(*dto.Timezone) == "" {
				if project.Timezone.Valid {
					dto.Timezone = nullableStringPtr(project.Timezone)
				} else {
					tz := "UTC"
					dto.Timezone = &tz
				}
			}
			resp = append(resp, dto)
		}
		writeJSON(w, http.StatusOK, resp)
	case http.MethodPost:
		if !ensureJSON(w, r) {
			return
		}
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		if existing, err := s.schedules.List(r.Context(), projectID); err == nil && len(existing) > 0 {
			writeError(w, http.StatusConflict, "schedule already exists for project")
			return
		}
		defer r.Body.Close()
		var body struct {
			Name        string          `json:"name"`
			Description *string         `json:"description"`
			Strategy    string          `json:"strategy"`
			Config      json.RawMessage `json:"config"`
			IsActive    *bool           `json:"isActive"`
			Timezone    *string         `json:"timezone"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		name := strings.TrimSpace(body.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		strategy := strings.TrimSpace(body.Strategy)
		if strategy == "" {
			writeError(w, http.StatusBadRequest, "strategy is required")
			return
		}
		if len(body.Config) == 0 || strings.TrimSpace(string(body.Config)) == "null" {
			writeError(w, http.StatusBadRequest, "config is required")
			return
		}
		isActive := true
		if body.IsActive != nil {
			isActive = *body.IsActive
		}
		var timezone sql.NullString
		if body.Timezone != nil {
			tz := strings.TrimSpace(*body.Timezone)
			if tz == "" {
				tz = "UTC"
			}
			if _, err := time.LoadLocation(tz); err != nil {
				writeError(w, http.StatusBadRequest, "invalid timezone")
				return
			}
			timezone = sqlstore.NullableString(tz)
		} else if project.Timezone.Valid {
			timezone = project.Timezone
		} else {
			timezone = sqlstore.NullableString("UTC")
		}

		now := time.Now().UTC()
		sched := sqlstore.Schedule{
			ID:          uuid.NewString(),
			ProjectID:   projectID,
			Name:        name,
			Description: nullStringFromOptional(body.Description),
			Strategy:    strategy,
			Config:      body.Config,
			IsActive:    isActive,
			CreatedBy:   user.Email,
			Timezone:    timezone,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.schedules.Create(r.Context(), sched); err != nil {
			if strings.Contains(err.Error(), "uniq_gen_schedules_project") {
				writeError(w, http.StatusConflict, "schedule already exists for project")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to create schedule")
			return
		}
		created := sched
		if stored, err := s.schedules.Get(r.Context(), sched.ID); err == nil {
			created = *stored
		}
		writeJSON(w, http.StatusCreated, toScheduleDTO(created))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectScheduleByID(w http.ResponseWriter, r *http.Request, projectID, scheduleID string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.schedules == nil {
		writeError(w, http.StatusInternalServerError, "schedules not configured")
		return
	}

	if _, err := s.authorizeProject(r.Context(), projectID); err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}

	sched, err := s.schedules.Get(r.Context(), scheduleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "schedule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load schedule")
		return
	}
	if sched.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		writeJSON(w, http.StatusOK, toScheduleDTO(*sched))
	case http.MethodPatch:
		if !ensureJSON(w, r) {
			return
		}
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		defer r.Body.Close()
		var body struct {
			Name        *string          `json:"name"`
			Description *string          `json:"description"`
			Strategy    *string          `json:"strategy"`
			Config      *json.RawMessage `json:"config"`
			IsActive    *bool            `json:"isActive"`
			Timezone    *string          `json:"timezone"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		var updates sqlstore.ScheduleUpdates
		if body.Name != nil {
			name := strings.TrimSpace(*body.Name)
			if name == "" {
				writeError(w, http.StatusBadRequest, "name cannot be empty")
				return
			}
			updates.Name = &name
		}
		if body.Description != nil {
			desc := strings.TrimSpace(*body.Description)
			updates.Description = &sql.NullString{String: desc, Valid: desc != ""}
		}
		if body.Strategy != nil {
			strategy := strings.TrimSpace(*body.Strategy)
			if strategy == "" {
				writeError(w, http.StatusBadRequest, "strategy cannot be empty")
				return
			}
			updates.Strategy = &strategy
		}
		if body.Config != nil {
			if len(*body.Config) == 0 || strings.TrimSpace(string(*body.Config)) == "null" {
				writeError(w, http.StatusBadRequest, "config cannot be empty")
				return
			}
			updates.Config = body.Config
		}
		if body.IsActive != nil {
			updates.IsActive = body.IsActive
		}
		if body.Timezone != nil {
			tz := strings.TrimSpace(*body.Timezone)
			updates.Timezone = &sql.NullString{String: tz, Valid: tz != ""}
		}
		if updates.Name == nil && updates.Description == nil && updates.Strategy == nil && updates.Config == nil && updates.IsActive == nil && updates.Timezone == nil {
			writeError(w, http.StatusBadRequest, "no updates provided")
			return
		}
		if err := s.schedules.Update(r.Context(), scheduleID, updates); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update schedule")
			return
		}
		updated, err := s.schedules.Get(r.Context(), scheduleID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load schedule")
			return
		}
		writeJSON(w, http.StatusOK, toScheduleDTO(*updated))
	case http.MethodDelete:
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		if err := s.schedules.Delete(r.Context(), scheduleID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete schedule")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectScheduleTrigger(w http.ResponseWriter, r *http.Request, projectID, scheduleID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.schedules == nil {
		writeError(w, http.StatusInternalServerError, "schedules not configured")
		return
	}
	if s.genQueue == nil {
		writeError(w, http.StatusInternalServerError, "generation queue not configured")
		return
	}
	if err := requireApprovedUser(user); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	if _, err := s.authorizeProject(r.Context(), projectID); err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}
	if !hasProjectPermission(user.Role, memberRole, "editor") {
		writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
		return
	}

	sched, err := s.schedules.Get(r.Context(), scheduleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "schedule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load schedule")
		return
	}
	if sched.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	domains, err := s.domains.ListByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list domains")
		return
	}
	cfg, err := parseScheduleConfig(sched.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule config")
		return
	}
	queueItems, err := s.genQueue.ListByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load queue")
		return
	}

	now := time.Now().UTC().Truncate(time.Minute)
	enqueued, err := enqueueScheduleDomains(r.Context(), s.genQueue, *sched, cfg, domains, queueItems, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue generation")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":   "queued",
		"enqueued": enqueued,
	})
}

func (s *Server) handleProjectLinkSchedule(w http.ResponseWriter, r *http.Request, projectID string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.linkSchedules == nil {
		writeError(w, http.StatusInternalServerError, "link schedules not configured")
		return
	}

	project, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}

	switch r.Method {
	case http.MethodGet:
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		sched, err := s.linkSchedules.GetByProject(r.Context(), projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeJSON(w, http.StatusOK, nil)
				return
			}
			writeError(w, http.StatusInternalServerError, "could not load link schedule")
			return
		}
		dto := toLinkScheduleDTO(*sched)
		if dto.Timezone == nil || strings.TrimSpace(*dto.Timezone) == "" {
			if project.Timezone.Valid {
				dto.Timezone = nullableStringPtr(project.Timezone)
			} else {
				tz := "UTC"
				dto.Timezone = &tz
			}
		}
		writeJSON(w, http.StatusOK, dto)
	case http.MethodPut:
		if !ensureJSON(w, r) {
			return
		}
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		defer r.Body.Close()
		var body struct {
			Name     string          `json:"name"`
			Config   json.RawMessage `json:"config"`
			IsActive *bool           `json:"isActive"`
			Timezone *string         `json:"timezone"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		name := strings.TrimSpace(body.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		if len(body.Config) == 0 || strings.TrimSpace(string(body.Config)) == "null" {
			writeError(w, http.StatusBadRequest, "config is required")
			return
		}
		isActive := true
		if body.IsActive != nil {
			isActive = *body.IsActive
		}

		var timezone sql.NullString
		if body.Timezone != nil {
			tz := strings.TrimSpace(*body.Timezone)
			if tz == "" {
				tz = "UTC"
			}
			if _, err := time.LoadLocation(tz); err != nil {
				writeError(w, http.StatusBadRequest, "invalid timezone")
				return
			}
			timezone = sqlstore.NullableString(tz)
		}

		existing, _ := s.linkSchedules.GetByProject(r.Context(), projectID)
		if !timezone.Valid {
			if existing != nil && existing.Timezone.Valid {
				timezone = existing.Timezone
			} else if project.Timezone.Valid {
				timezone = project.Timezone
			} else {
				timezone = sqlstore.NullableString("UTC")
			}
		}

		cfg, err := parseScheduleConfig(body.Config)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid schedule config")
			return
		}

		loc := time.UTC
		if timezone.Valid {
			if tz, err := time.LoadLocation(strings.TrimSpace(timezone.String)); err == nil {
				loc = tz
			}
		}
		now := time.Now().UTC()
		localNow := now.In(loc)
		lastRunLocal := time.Time{}
		if existing != nil && existing.LastRunAt.Valid {
			lastRunLocal = existing.LastRunAt.Time.In(loc)
		}
		nextRunLocal, _, err := computeLinkScheduleNextRun(cfg, localNow, lastRunLocal)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid schedule config")
			return
		}
		nextRunUTC := sql.NullTime{}
		if !nextRunLocal.IsZero() {
			nextRunUTC = sql.NullTime{Time: nextRunLocal.In(time.UTC), Valid: true}
		}

		sched := sqlstore.LinkSchedule{
			ID:        uuid.NewString(),
			ProjectID: projectID,
			Name:      name,
			Config:    body.Config,
			IsActive:  isActive,
			CreatedBy: user.Email,
			CreatedAt: now,
			UpdatedAt: now,
			NextRunAt: nextRunUTC,
			Timezone:  timezone,
		}
		stored, err := s.linkSchedules.Upsert(r.Context(), sched)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to upsert link schedule")
			return
		}
		writeJSON(w, http.StatusOK, toLinkScheduleDTO(*stored))
	case http.MethodDelete:
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		if err := s.linkSchedules.DeleteByProject(r.Context(), projectID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "link schedule not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to delete link schedule")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectLinkScheduleTrigger(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.linkSchedules == nil {
		writeError(w, http.StatusInternalServerError, "link schedules not configured")
		return
	}
	if s.linkTasks == nil {
		writeError(w, http.StatusInternalServerError, "link tasks not configured")
		return
	}
	if err := requireApprovedUser(user); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	if _, err := s.authorizeProject(r.Context(), projectID); err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}
	if !hasProjectPermission(user.Role, memberRole, "editor") {
		writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
		return
	}

	sched, err := s.linkSchedules.GetByProject(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "link schedule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load link schedule")
		return
	}
	cfg, err := parseScheduleConfig(sched.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule config")
		return
	}

	loc := time.UTC
	if sched.Timezone.Valid {
		if tz, err := time.LoadLocation(strings.TrimSpace(sched.Timezone.String)); err == nil {
			loc = tz
		}
	}

	domains, err := s.domains.ListByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list domains")
		return
	}

	now := time.Now().UTC()
	scheduleRunLocal, err := scheduleRunAtToday(now.In(loc), cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule config")
		return
	}
	scheduleRunUTC := scheduleRunLocal.In(time.UTC)
	eligible := filterLinkDomains(domains, now, scheduleRunUTC)
	activeTasks, err := listActiveLinkTasksByProject(r.Context(), s.linkTasks, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load link tasks")
		return
	}
	activeByDomain := map[string]sqlstore.LinkTask{}
	for _, task := range activeTasks {
		activeByDomain[task.DomainID] = task
	}

	limit := cfg.Limit
	if limit <= 0 {
		limit = len(eligible)
	}
	activeEligible := 0
	for _, d := range eligible {
		if _, ok := activeByDomain[d.ID]; ok {
			activeEligible++
		}
	}
	remaining := limit - activeEligible
	if remaining < 0 {
		remaining = 0
	}

	updated := 0
	created := 0
	for _, d := range eligible {
		anchor := strings.TrimSpace(d.LinkAnchorText.String)
		target := strings.TrimSpace(d.LinkAcceptorURL.String)
		if task, ok := activeByDomain[d.ID]; ok {
			status := "pending"
			attempts := 0
			nullStr := sql.NullString{}
			nullTime := sql.NullTime{}
			action := "insert"
			updates := sqlstore.LinkTaskUpdates{
				AnchorText:       &anchor,
				TargetURL:        &target,
				Action:           &action,
				Status:           &status,
				Attempts:         &attempts,
				ScheduledFor:     &now,
				FoundLocation:    &nullStr,
				GeneratedContent: &nullStr,
				ErrorMessage:     &nullStr,
				CompletedAt:      &nullTime,
			}
			if err := s.linkTasks.Update(r.Context(), task.ID, updates); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update link task")
				return
			}
			updated++
			continue
		}

		if remaining == 0 {
			continue
		}
		task := sqlstore.LinkTask{
			ID:           uuid.NewString(),
			DomainID:     d.ID,
			AnchorText:   anchor,
			TargetURL:    target,
			ScheduledFor: now,
			Action:       "insert",
			Status:       "pending",
			Attempts:     0,
			CreatedBy:    user.Email,
			CreatedAt:    now,
		}
		if err := s.linkTasks.Create(r.Context(), task); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create link task")
			return
		}
		created++
		remaining--
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":   "queued",
		"enqueued": created + updated,
		"created":  created,
		"updated":  updated,
		"eligible": len(eligible),
	})
}

func (s *Server) handleProjectQueue(w http.ResponseWriter, r *http.Request, projectID string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.genQueue == nil {
		writeError(w, http.StatusInternalServerError, "generation queue not configured")
		return
	}

	if _, err := s.authorizeProject(r.Context(), projectID); err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}
	if !hasProjectPermission(user.Role, memberRole, "viewer") {
		writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	page := 1
	if v := strings.TrimSpace(r.URL.Query().Get("page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	offset := (page - 1) * limit

	items, err := s.genQueue.ListByProjectPage(r.Context(), projectID, limit, offset, search)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load queue")
		return
	}
	domainMap := map[string]string{}
	domainByID := map[string]sqlstore.Domain{}
	if domains, err := s.domains.ListByProject(r.Context(), projectID); err == nil {
		for _, d := range domains {
			domainMap[d.ID] = d.URL
			domainByID[d.ID] = d
		}
	}
	resp := make([]queueItemDTO, 0, len(items))
	for _, item := range items {
		if item.Status != "pending" && item.Status != "queued" {
			continue
		}
		if domain, ok := domainByID[item.DomainID]; ok && !isDomainWaiting(domain) {
			continue
		}
		dto := toQueueItemDTO(item)
		if url, ok := domainMap[item.DomainID]; ok {
			dto.DomainURL = &url
		}
		resp = append(resp, dto)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleProjectQueueCleanup(w http.ResponseWriter, r *http.Request, projectID string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.genQueue == nil {
		writeError(w, http.StatusInternalServerError, "generation queue not configured")
		return
	}

	if _, err := s.authorizeProject(r.Context(), projectID); err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}
	if !hasProjectPermission(user.Role, memberRole, "editor") {
		writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
		return
	}

	items, err := s.genQueue.ListByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load queue")
		return
	}
	domainByID := map[string]sqlstore.Domain{}
	if domains, err := s.domains.ListByProject(r.Context(), projectID); err == nil {
		for _, d := range domains {
			domainByID[d.ID] = d
		}
	}

	removed := 0
	for _, item := range items {
		if item.Status != "pending" && item.Status != "queued" {
			if err := s.genQueue.Delete(r.Context(), item.ID); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to delete queue item")
				return
			}
			removed++
			continue
		}
		domain, ok := domainByID[item.DomainID]
		if !ok || !isDomainWaiting(domain) {
			if err := s.genQueue.Delete(r.Context(), item.ID); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to delete queue item")
				return
			}
			removed++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"removed": removed,
	})
}

func (s *Server) handleProjectIndexChecks(w http.ResponseWriter, r *http.Request, projectID string, parts []string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index checks not configured")
		return
	}

	project, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}

	if len(parts) > 0 {
		if len(parts) > 1 && parts[1] == "history" {
			s.handleProjectIndexCheckHistory(w, r, projectID, parts[0], memberRole)
			return
		}
		if len(parts) == 1 && parts[0] == "stats" {
			s.handleProjectIndexChecksStats(w, r, projectID, memberRole)
			return
		}
		if len(parts) == 1 && parts[0] == "calendar" {
			s.handleProjectIndexChecksCalendar(w, r, projectID, memberRole)
			return
		}
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		filters := parseIndexCheckFilters(r, 100, 500)
		list, err := s.indexChecks.ListByProject(r.Context(), projectID, filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list index checks")
			return
		}
		total, err := s.indexChecks.CountByProject(r.Context(), projectID, filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not count index checks")
			return
		}
		domainMap := map[string]string{}
		if domains, err := s.domains.ListByProject(r.Context(), projectID); err == nil {
			for _, d := range domains {
				domainMap[d.ID] = d.URL
			}
		}
		resp := make([]indexCheckDTO, 0, len(list))
		for _, check := range list {
			dto := toIndexCheckDTO(check)
			if url, ok := domainMap[check.DomainID]; ok && strings.TrimSpace(url) != "" {
				dto.DomainURL = &url
			}
			pid := project.ID
			dto.ProjectID = &pid
			resp = append(resp, dto)
		}
		writeJSON(w, http.StatusOK, indexCheckListDTO{Items: resp, Total: total})
	case http.MethodPost:
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		domains, err := s.domains.ListByProject(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list domains")
			return
		}
		now := time.Now().UTC()
		created := 0
		updated := 0
		skipped := 0
		for _, d := range domains {
			if strings.TrimSpace(d.URL) == "" {
				skipped++
				continue
			}
			if !isDomainPublished(d) {
				skipped++
				continue
			}
			_, wasCreated, err := s.upsertManualIndexCheck(r.Context(), d.ID, now)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not start index checks")
				return
			}
			if wasCreated {
				created++
			} else {
				updated++
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"created": created,
			"updated": updated,
			"skipped": skipped,
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectIndexCheckHistory(w http.ResponseWriter, r *http.Request, projectID, checkID, memberRole string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.checkHistory == nil || s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index check history not configured")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !hasProjectPermission(user.Role, memberRole, "viewer") {
		writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
		return
	}
	checkID = strings.TrimSpace(checkID)
	if checkID == "" {
		writeError(w, http.StatusBadRequest, "missing check id")
		return
	}
	check, err := s.indexChecks.Get(r.Context(), checkID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "check not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load check")
		return
	}
	domain, err := s.domains.Get(r.Context(), check.DomainID)
	if err != nil {
		writeError(w, http.StatusNotFound, "check not found")
		return
	}
	if domain.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "check not found")
		return
	}

	limit := parseLimitParam(r, 50, 200)
	list, err := s.checkHistory.ListByCheck(r.Context(), checkID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load check history")
		return
	}
	resp := make([]indexCheckHistoryDTO, 0, len(list))
	for _, item := range list {
		resp = append(resp, toIndexCheckHistoryDTO(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleProjectIndexChecksStats(w http.ResponseWriter, r *http.Request, projectID, memberRole string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index checks not configured")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !hasProjectPermission(user.Role, memberRole, "viewer") {
		writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
		return
	}
	now := time.Now().UTC()
	filters := parseIndexCheckFilters(r, 0, 0)
	from, to := resolveIndexCheckStatsRange(&filters, now)
	filters.Limit = 0
	filters.Offset = 0
	stats, err := s.indexChecks.AggregateStatsByProject(r.Context(), projectID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not aggregate index checks")
		return
	}
	daily, err := s.indexChecks.AggregateDailyByProject(r.Context(), projectID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not aggregate index checks")
		return
	}
	writeJSON(w, http.StatusOK, buildIndexCheckStatsDTO(stats, daily, from, to))
}

func (s *Server) handleProjectIndexChecksCalendar(w http.ResponseWriter, r *http.Request, projectID, memberRole string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index checks not configured")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !hasProjectPermission(user.Role, memberRole, "viewer") {
		writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
		return
	}
	now := time.Now().UTC()
	filters := parseIndexCheckFilters(r, 0, 0)
	_, _ = resolveIndexCheckCalendarRange(&filters, r, now)
	filters.Limit = 0
	filters.Offset = 0
	daily, err := s.indexChecks.AggregateDailyByProject(r.Context(), projectID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not aggregate index checks")
		return
	}
	resp := make([]indexCheckDailyDTO, 0, len(daily))
	for _, item := range daily {
		resp = append(resp, toIndexCheckDailyDTO(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleQueueItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.genQueue == nil {
		writeError(w, http.StatusInternalServerError, "generation queue not configured")
		return
	}

	itemID := strings.TrimPrefix(r.URL.Path, "/api/queue/")
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		writeError(w, http.StatusBadRequest, "missing queue item id")
		return
	}

	item, err := s.genQueue.Get(r.Context(), itemID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "queue item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load queue item")
		return
	}

	domain, err := s.domains.Get(r.Context(), item.DomainID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load domain")
		return
	}

	if _, err := s.authorizeProject(r.Context(), domain.ProjectID); err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), domain.ProjectID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}
	if !hasProjectPermission(user.Role, memberRole, "editor") {
		writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
		return
	}

	if err := s.genQueue.Delete(r.Context(), itemID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete queue item")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleLinks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.linkTasks == nil {
		writeError(w, http.StatusInternalServerError, "link tasks not configured")
		return
	}

	filters, err := parseLinkTaskFilters(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	query := r.URL.Query()
	domainID := strings.TrimSpace(query.Get("domain_id"))
	projectID := strings.TrimSpace(query.Get("project_id"))

	var list []sqlstore.LinkTask
	switch {
	case domainID != "":
		filters.Search = nil
		domain, project, err := s.authorizeDomain(r.Context(), domainID)
		if err != nil {
			respondAuthzError(w, err, "domain not found")
			return
		}
		memberRole := ""
		if !strings.EqualFold(user.Role, "admin") {
			role, err := s.getProjectMemberRole(r.Context(), project.ID, user.Email)
			if err != nil {
				writeError(w, http.StatusForbidden, "access denied")
				return
			}
			memberRole = role
		}
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		list, err = s.linkTasks.ListByDomain(r.Context(), domain.ID, filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list link tasks")
			return
		}
	case projectID != "":
		if _, err := s.authorizeProject(r.Context(), projectID); err != nil {
			respondAuthzError(w, err, "project not found")
			return
		}
		memberRole := ""
		if !strings.EqualFold(user.Role, "admin") {
			role, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
			if err != nil {
				writeError(w, http.StatusForbidden, "access denied")
				return
			}
			memberRole = role
		}
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		list, err = s.linkTasks.ListByProject(r.Context(), projectID, filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list link tasks")
			return
		}
	default:
		if !strings.EqualFold(user.Role, "admin") {
			writeError(w, http.StatusForbidden, "admin only")
			return
		}
		list, err = s.linkTasks.ListAll(r.Context(), filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list link tasks")
			return
		}
	}

	resp := make([]linkTaskDTO, 0, len(list))
	for _, task := range list {
		resp = append(resp, toLinkTaskDTO(task))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLinkByID(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.linkTasks == nil {
		writeError(w, http.StatusInternalServerError, "link tasks not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/links/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusBadRequest, "missing link task id")
		return
	}
	taskID, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(taskID) == "" {
		writeError(w, http.StatusBadRequest, "invalid link task id")
		return
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	task, err := s.linkTasks.Get(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "link task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load link task")
		return
	}

	domain, err := s.domains.Get(r.Context(), task.DomainID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load domain")
		return
	}
	if _, err := s.authorizeProject(r.Context(), domain.ProjectID); err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), domain.ProjectID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}

	if action == "retry" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}

		now := time.Now().UTC()
		status := "pending"
		attempts := task.Attempts + 1
		updates := sqlstore.LinkTaskUpdates{
			Status:       &status,
			Attempts:     &attempts,
			ScheduledFor: &now,
			ErrorMessage: &sql.NullString{},
			CompletedAt:  &sql.NullTime{},
		}
		if err := s.linkTasks.Update(r.Context(), taskID, updates); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to retry link task")
			return
		}
		updated, _ := s.linkTasks.Get(r.Context(), taskID)
		if updated == nil {
			writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
			return
		}
		writeJSON(w, http.StatusOK, toLinkTaskDTO(*updated))
		return
	}

	switch r.Method {
	case http.MethodGet:
		if action != "" {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		writeJSON(w, http.StatusOK, toLinkTaskDTO(*task))
	case http.MethodPatch:
		if !ensureJSON(w, r) {
			return
		}
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		defer r.Body.Close()
		var body struct {
			ScheduledFor      string `json:"scheduled_for"`
			ScheduledForCamel string `json:"scheduledFor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		now := time.Now().UTC()
		scheduledFor, err := parseScheduledFor(body.ScheduledFor, body.ScheduledForCamel, now)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid scheduled_for")
			return
		}
		updates := sqlstore.LinkTaskUpdates{ScheduledFor: &scheduledFor}
		if err := s.linkTasks.Update(r.Context(), taskID, updates); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update link task")
			return
		}
		updated, _ := s.linkTasks.Get(r.Context(), taskID)
		if updated == nil {
			writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
			return
		}
		writeJSON(w, http.StatusOK, toLinkTaskDTO(*updated))
	case http.MethodDelete:
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		if err := s.linkTasks.Delete(r.Context(), taskID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete link task")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func parseLinkTaskFilters(r *http.Request) (sqlstore.LinkTaskFilters, error) {
	query := r.URL.Query()
	filters := sqlstore.LinkTaskFilters{}

	if status := strings.TrimSpace(query.Get("status")); status != "" {
		filters.Status = &status
	}
	if from := strings.TrimSpace(query.Get("scheduled_from")); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return filters, fmt.Errorf("invalid scheduled_from")
		}
		tt := t.UTC()
		filters.ScheduledAfter = &tt
	} else if from := strings.TrimSpace(query.Get("scheduledFrom")); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return filters, fmt.Errorf("invalid scheduled_from")
		}
		tt := t.UTC()
		filters.ScheduledAfter = &tt
	}
	if to := strings.TrimSpace(query.Get("scheduled_to")); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return filters, fmt.Errorf("invalid scheduled_to")
		}
		tt := t.UTC()
		filters.ScheduledBefore = &tt
	} else if to := strings.TrimSpace(query.Get("scheduledTo")); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return filters, fmt.Errorf("invalid scheduled_to")
		}
		tt := t.UTC()
		filters.ScheduledBefore = &tt
	}

	if limitStr := strings.TrimSpace(query.Get("limit")); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			return filters, fmt.Errorf("invalid limit")
		}
		filters.Limit = limit
	}
	if search := strings.TrimSpace(query.Get("search")); search != "" {
		filters.Search = &search
	}
	if pageStr := strings.TrimSpace(query.Get("page")); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			return filters, fmt.Errorf("invalid page")
		}
		if filters.Limit <= 0 {
			filters.Limit = 50
		}
		if page > 1 {
			filters.Offset = (page - 1) * filters.Limit
		}
	}

	return filters, nil
}

func parseScheduledFor(primary string, fallback string, defaultTime time.Time) (time.Time, error) {
	value := strings.TrimSpace(primary)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	if value == "" {
		return defaultTime, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func (s *Server) handleAdminPrompts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.prompts.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list prompts")
			return
		}
		resp := make([]adminPromptDTO, 0, len(list))
		for _, p := range list {
			resp = append(resp, toAdminPromptDTO(p))
		}
		writeJSON(w, http.StatusOK, resp)
	case http.MethodPost:
		if !ensureJSON(w, r) {
			return
		}
		defer r.Body.Close()
		var body struct {
			Name        string  `json:"name"`
			Description *string `json:"description"`
			Body        string  `json:"body"`
			Stage       *string `json:"stage"`
			Model       *string `json:"model"`
			IsActive    bool    `json:"isActive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		name := strings.TrimSpace(body.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		promptBody := strings.TrimSpace(body.Body)
		if promptBody == "" {
			writeError(w, http.StatusBadRequest, "body is required")
			return
		}
		prompt := sqlstore.SystemPrompt{
			ID:          uuid.NewString(),
			Name:        name,
			Description: nullStringFromOptional(body.Description),
			Body:        promptBody,
			Stage:       nullStringFromOptional(body.Stage),
			Model:       nullStringFromOptional(body.Model),
			IsActive:    body.IsActive,
		}
		if err := s.prompts.Create(r.Context(), prompt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create prompt")
			return
		}
		writeJSON(w, http.StatusCreated, toAdminPromptDTO(prompt))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAdminPromptByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/prompts/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing prompt id")
		return
	}
	switch r.Method {
	case http.MethodGet:
		p, err := s.prompts.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "prompt not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to load prompt")
			return
		}
		writeJSON(w, http.StatusOK, toAdminPromptDTO(p))
	case http.MethodPatch:
		if !ensureJSON(w, r) {
			return
		}
		defer r.Body.Close()
		var body struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
			Body        *string `json:"body"`
			Stage       *string `json:"stage"`
			Model       *string `json:"model"`
			IsActive    *bool   `json:"isActive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		prompt, err := s.prompts.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "prompt not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to load prompt")
			return
		}
		updated := false
		if body.Name != nil {
			name := strings.TrimSpace(*body.Name)
			if name == "" {
				writeError(w, http.StatusBadRequest, "name cannot be empty")
				return
			}
			prompt.Name = name
			updated = true
		}
		if body.Description != nil {
			prompt.Description = nullStringFromOptional(body.Description)
			updated = true
		}
		if body.Body != nil {
			pBody := strings.TrimSpace(*body.Body)
			if pBody == "" {
				writeError(w, http.StatusBadRequest, "body cannot be empty")
				return
			}
			prompt.Body = pBody
			updated = true
		}
		if body.Stage != nil {
			stageStr := strings.TrimSpace(*body.Stage)
			if stageStr == "" {
				prompt.Stage = sql.NullString{}
			} else {
				prompt.Stage = sqlstore.NullableString(stageStr)
			}
			updated = true
		}
		if body.Model != nil {
			modelStr := strings.TrimSpace(*body.Model)
			if modelStr == "" {
				prompt.Model = sql.NullString{}
			} else {
				prompt.Model = sqlstore.NullableString(modelStr)
			}
			updated = true
		}
		if body.IsActive != nil {
			prompt.IsActive = *body.IsActive
			updated = true
		}
		if !updated {
			writeError(w, http.StatusBadRequest, "no changes supplied")
			return
		}
		if err := s.prompts.Update(r.Context(), prompt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update prompt")
			return
		}
		writeJSON(w, http.StatusOK, toAdminPromptDTO(prompt))
	case http.MethodDelete:
		if err := s.prompts.Delete(r.Context(), id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "prompt not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to delete prompt")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAdminAuditRules(w http.ResponseWriter, r *http.Request) {
	if s.auditRules == nil {
		writeError(w, http.StatusInternalServerError, "audit store not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := s.auditRules.ListRules(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list audit rules")
			return
		}
		resp := make([]adminAuditRuleDTO, 0, len(list))
		for _, rule := range list {
			resp = append(resp, toAdminAuditRuleDTO(rule))
		}
		writeJSON(w, http.StatusOK, resp)
	case http.MethodPost:
		if !ensureJSON(w, r) {
			return
		}
		defer r.Body.Close()
		var body struct {
			Code        string  `json:"code"`
			Title       string  `json:"title"`
			Description *string `json:"description"`
			Severity    string  `json:"severity"`
			IsActive    bool    `json:"isActive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		code := strings.TrimSpace(body.Code)
		title := strings.TrimSpace(body.Title)
		if code == "" || title == "" {
			writeError(w, http.StatusBadRequest, "code and title are required")
			return
		}
		severity := strings.ToLower(strings.TrimSpace(body.Severity))
		if severity == "" {
			severity = "warn"
		}
		rule := sqlstore.AuditRule{
			Code:        code,
			Title:       title,
			Description: strings.TrimSpace(optionalString(body.Description)),
			Severity:    severity,
			IsActive:    body.IsActive,
		}
		if err := s.auditRules.CreateRule(r.Context(), rule); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create audit rule")
			return
		}
		writeJSON(w, http.StatusCreated, toAdminAuditRuleDTO(rule))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAdminIndexChecks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index checks not configured")
		return
	}
	filters := parseIndexCheckFilters(r, 200, 500)
	list, err := s.indexChecks.ListAll(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list index checks")
		return
	}
	total, err := s.indexChecks.CountAll(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not count index checks")
		return
	}
	resp := s.buildIndexCheckResponse(r.Context(), list)
	writeJSON(w, http.StatusOK, indexCheckListDTO{Items: resp, Total: total})
}

func (s *Server) handleAdminIndexChecksFailed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index checks not configured")
		return
	}
	filters := parseIndexCheckFilters(r, 200, 500)
	list, err := s.indexChecks.ListFailed(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list index checks")
		return
	}
	total, err := s.indexChecks.CountFailed(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not count index checks")
		return
	}
	resp := s.buildIndexCheckResponse(r.Context(), list)
	writeJSON(w, http.StatusOK, indexCheckListDTO{Items: resp, Total: total})
}

func (s *Server) handleAdminIndexChecksRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index checks not configured")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		DomainID string `json:"domain_id"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	domainID := strings.TrimSpace(body.DomainID)
	if domainID == "" {
		writeError(w, http.StatusBadRequest, "domain_id is required")
		return
	}
	if s.domains == nil {
		writeError(w, http.StatusInternalServerError, "domains not configured")
		return
	}
	domain, err := s.domains.Get(r.Context(), domainID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load domain")
		return
	}
	if !isDomainPublished(domain) {
		writeError(w, http.StatusConflict, "domain is not published")
		return
	}
	now := time.Now().UTC()
	check, created, err := s.upsertManualIndexCheck(r.Context(), domainID, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start index check")
		return
	}
	dto := toIndexCheckDTO(check)
	if url := strings.TrimSpace(domain.URL); url != "" {
		dto.DomainURL = &url
	}
	pid := domain.ProjectID
	dto.ProjectID = &pid
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, dto)
}

func (s *Server) handleAdminIndexChecksStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index checks not configured")
		return
	}
	now := time.Now().UTC()
	filters := parseIndexCheckFilters(r, 0, 0)
	from, to := resolveIndexCheckStatsRange(&filters, now)
	filters.Limit = 0
	filters.Offset = 0
	stats, err := s.indexChecks.AggregateStats(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not aggregate index checks")
		return
	}
	daily, err := s.indexChecks.AggregateDaily(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not aggregate index checks")
		return
	}
	writeJSON(w, http.StatusOK, buildIndexCheckStatsDTO(stats, daily, from, to))
}

func (s *Server) handleAdminIndexChecksCalendar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.indexChecks == nil {
		writeError(w, http.StatusInternalServerError, "index checks not configured")
		return
	}
	now := time.Now().UTC()
	filters := parseIndexCheckFilters(r, 0, 0)
	_, _ = resolveIndexCheckCalendarRange(&filters, r, now)
	filters.Limit = 0
	filters.Offset = 0
	daily, err := s.indexChecks.AggregateDaily(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not aggregate index checks")
		return
	}
	resp := make([]indexCheckDailyDTO, 0, len(daily))
	for _, item := range daily {
		resp = append(resp, toIndexCheckDailyDTO(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAdminIndexCheckRoute(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/admin/index-checks/")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	checkID := strings.TrimSpace(parts[0])
	if checkID == "" {
		writeError(w, http.StatusBadRequest, "missing check id")
		return
	}
	switch parts[1] {
	case "history":
		s.handleAdminIndexCheckHistory(w, r, checkID)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleAdminIndexCheckHistory(w http.ResponseWriter, r *http.Request, checkID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.checkHistory == nil {
		writeError(w, http.StatusInternalServerError, "index check history not configured")
		return
	}
	limit := parseLimitParam(r, 50, 200)
	list, err := s.checkHistory.ListByCheck(r.Context(), checkID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load check history")
		return
	}
	resp := make([]indexCheckHistoryDTO, 0, len(list))
	for _, item := range list {
		resp = append(resp, toIndexCheckHistoryDTO(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAdminAuditRuleByCode(w http.ResponseWriter, r *http.Request) {
	if s.auditRules == nil {
		writeError(w, http.StatusInternalServerError, "audit store not configured")
		return
	}
	code := strings.TrimPrefix(r.URL.Path, "/api/admin/audit-rules/")
	code = strings.TrimSpace(code)
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing rule code")
		return
	}
	switch r.Method {
	case http.MethodGet:
		rule, err := s.auditRules.GetRule(r.Context(), code)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "rule not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to load rule")
			return
		}
		writeJSON(w, http.StatusOK, toAdminAuditRuleDTO(rule))
	case http.MethodPatch:
		if !ensureJSON(w, r) {
			return
		}
		defer r.Body.Close()
		var body struct {
			Title       *string `json:"title"`
			Description *string `json:"description"`
			Severity    *string `json:"severity"`
			IsActive    *bool   `json:"isActive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		rule, err := s.auditRules.GetRule(r.Context(), code)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "rule not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to load rule")
			return
		}
		updated := false
		if body.Title != nil {
			title := strings.TrimSpace(*body.Title)
			if title == "" {
				writeError(w, http.StatusBadRequest, "title cannot be empty")
				return
			}
			rule.Title = title
			updated = true
		}
		if body.Description != nil {
			rule.Description = strings.TrimSpace(*body.Description)
			updated = true
		}
		if body.Severity != nil {
			sev := strings.ToLower(strings.TrimSpace(*body.Severity))
			if sev == "" {
				sev = "warn"
			}
			rule.Severity = sev
			updated = true
		}
		if body.IsActive != nil {
			rule.IsActive = *body.IsActive
			updated = true
		}
		if !updated {
			writeError(w, http.StatusBadRequest, "no changes supplied")
			return
		}
		if err := s.auditRules.UpdateRule(r.Context(), rule); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update rule")
			return
		}
		writeJSON(w, http.StatusOK, toAdminAuditRuleDTO(rule))
	case http.MethodDelete:
		if err := s.auditRules.DeleteRule(r.Context(), code); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "rule not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to delete rule")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func optionalString(val *string) string {
	if val == nil {
		return ""
	}
	return *val
}

func (s *Server) startGenerationMetricsLoop() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			s.refreshGenerationMetrics()
			<-ticker.C
		}
	}()
}

func (s *Server) refreshGenerationMetrics() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	counts, err := s.generations.CountsByStatus(ctx)
	if err != nil {
		s.logger.Warnf("failed to collect generation stats: %v", err)
		return
	}
	statuses := []string{"pending", "processing", "success", "error"}
	for _, status := range statuses {
		val := counts[status]
		s.genStatus.WithLabelValues(status).Set(float64(val))
	}
}

// --- DTO helpers ---

type projectDTO struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Status          string          `json:"status"`
	TargetCountry   string          `json:"target_country,omitempty"`
	TargetLanguage  string          `json:"target_language,omitempty"`
	Timezone        *string         `json:"timezone,omitempty"`
	DefaultServerID *string         `json:"default_server_id,omitempty"`
	GlobalBlacklist json.RawMessage `json:"global_blacklist,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	OwnerHasApiKey  bool            `json:"ownerHasApiKey,omitempty"` // Есть ли API ключ у владельца проекта
}

type domainDTO struct {
	ID                 string     `json:"id"`
	ProjectID          string     `json:"project_id"`
	ServerID           *string    `json:"server_id,omitempty"`
	URL                string     `json:"url"`
	MainKeyword        string     `json:"main_keyword,omitempty"`
	TargetCountry      string     `json:"target_country,omitempty"`
	TargetLanguage     string     `json:"target_language,omitempty"`
	ExcludeDomains     *string    `json:"exclude_domains,omitempty"`
	Status             string     `json:"status"`
	LastGenerationID   *string    `json:"last_generation_id,omitempty"`
	PublishedAt        *time.Time `json:"published_at,omitempty"`
	LinkAnchorText     *string    `json:"link_anchor_text,omitempty"`
	LinkAcceptorURL    *string    `json:"link_acceptor_url,omitempty"`
	LinkStatus         *string    `json:"link_status,omitempty"`
	LinkUpdatedAt      *time.Time `json:"link_updated_at,omitempty"`
	LinkLastTaskID     *string    `json:"link_last_task_id,omitempty"`
	LinkFilePath       *string    `json:"link_file_path,omitempty"`
	LinkAnchorSnapshot *string    `json:"link_anchor_snapshot,omitempty"`
	LinkReadyAt        *time.Time `json:"link_ready_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type fileDTO struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	MimeType  string    `json:"mimeType"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type fileEditDTO struct {
	ID          string    `json:"id"`
	EditedBy    string    `json:"editedBy"`
	EditType    string    `json:"editType"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type linkTaskDTO struct {
	ID               string     `json:"id"`
	DomainID         string     `json:"domain_id"`
	AnchorText       string     `json:"anchor_text"`
	TargetURL        string     `json:"target_url"`
	ScheduledFor     time.Time  `json:"scheduled_for"`
	Action           string     `json:"action"`
	Status           string     `json:"status"`
	FoundLocation    *string    `json:"found_location,omitempty"`
	GeneratedContent *string    `json:"generated_content,omitempty"`
	ErrorMessage     *string    `json:"error_message,omitempty"`
	LogLines         []string   `json:"log_lines,omitempty"`
	Attempts         int        `json:"attempts"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

type indexCheckDTO struct {
	ID            string     `json:"id"`
	DomainID      string     `json:"domain_id"`
	ProjectID     *string    `json:"project_id,omitempty"`
	DomainURL     *string    `json:"domain_url,omitempty"`
	CheckDate     time.Time  `json:"check_date"`
	Status        string     `json:"status"`
	IsIndexed     *bool      `json:"is_indexed,omitempty"`
	Attempts      int        `json:"attempts"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`
	NextRetryAt   *time.Time `json:"next_retry_at,omitempty"`
	ErrorMessage  *string    `json:"error_message,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type indexCheckListDTO struct {
	Items []indexCheckDTO `json:"items"`
	Total int             `json:"total"`
}

type indexCheckHistoryDTO struct {
	ID            string          `json:"id"`
	CheckID       string          `json:"check_id"`
	AttemptNumber int             `json:"attempt_number"`
	Result        *string         `json:"result,omitempty"`
	ResponseData  json.RawMessage `json:"response_data,omitempty"`
	ErrorMessage  *string         `json:"error_message,omitempty"`
	DurationMS    *int64          `json:"duration_ms,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type indexCheckDailyDTO struct {
	Date                string `json:"date"`
	Total               int    `json:"total"`
	IndexedTrue         int    `json:"indexed_true"`
	IndexedFalse        int    `json:"indexed_false"`
	Pending             int    `json:"pending"`
	Checking            int    `json:"checking"`
	FailedInvestigation int    `json:"failed_investigation"`
	Success             int    `json:"success"`
}

type indexCheckStatsDTO struct {
	From                 string               `json:"from"`
	To                   string               `json:"to"`
	TotalChecks          int                  `json:"total_checks"`
	TotalResolved        int                  `json:"total_resolved"`
	IndexedTrue          int                  `json:"indexed_true"`
	PercentIndexed       int                  `json:"percent_indexed"`
	AvgAttemptsToSuccess float64              `json:"avg_attempts_to_success"`
	FailedInvestigation  int                  `json:"failed_investigation"`
	Daily                []indexCheckDailyDTO `json:"daily"`
}

type generationDTO struct {
	ID         string     `json:"id"`
	DomainID   string     `json:"domain_id"`
	DomainURL  *string    `json:"domain_url,omitempty"`
	Status     string     `json:"status"`
	Progress   int        `json:"progress"`
	Error      *string    `json:"error,omitempty"`
	PromptID   *string    `json:"prompt_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Logs       any        `json:"logs,omitempty"`
	Artifacts  any        `json:"artifacts,omitempty"`
}

type dashboardStatsDTO struct {
	Pending    int  `json:"pending"`
	Processing int  `json:"processing"`
	Error      int  `json:"error"`
	AvgMinutes *int `json:"avg_minutes,omitempty"`
	AvgSample  int  `json:"avg_sample"`
}

type dashboardDTO struct {
	Projects     []projectDTO      `json:"projects"`
	Stats        dashboardStatsDTO `json:"stats"`
	RecentErrors []generationDTO   `json:"recent_errors"`
}

type adminUserDTO struct {
	Email           string     `json:"email"`
	Name            string     `json:"name,omitempty"`
	Role            string     `json:"role"`
	IsApproved      bool       `json:"isApproved"`
	Verified        bool       `json:"verified"`
	CreatedAt       time.Time  `json:"createdAt"`
	APIKeyUpdatedAt *time.Time `json:"apiKeyUpdatedAt,omitempty"`
}

type adminPromptDTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Body        string    `json:"body"`
	Stage       *string   `json:"stage,omitempty"`
	Model       *string   `json:"model,omitempty"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type projectMemberDTO struct {
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type scheduleDTO struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"project_id"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Strategy    string          `json:"strategy"`
	Config      json.RawMessage `json:"config"`
	IsActive    bool            `json:"isActive"`
	CreatedBy   string          `json:"createdBy"`
	LastRunAt   *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time      `json:"next_run_at,omitempty"`
	Timezone    *string         `json:"timezone,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type linkScheduleDTO struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	IsActive  bool            `json:"isActive"`
	CreatedBy string          `json:"createdBy"`
	LastRunAt *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt *time.Time      `json:"next_run_at,omitempty"`
	Timezone  *string         `json:"timezone,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type queueItemDTO struct {
	ID           string     `json:"id"`
	DomainID     string     `json:"domain_id"`
	DomainURL    *string    `json:"domain_url,omitempty"`
	ScheduleID   *string    `json:"schedule_id,omitempty"`
	Priority     int        `json:"priority"`
	ScheduledFor time.Time  `json:"scheduled_for"`
	Status       string     `json:"status"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
}

type projectSummaryDTO struct {
	Project projectDTO         `json:"project"`
	Domains []domainDTO        `json:"domains"`
	Members []projectMemberDTO `json:"members"`
}

type domainSummaryDTO struct {
	Domain      domainDTO       `json:"domain"`
	ProjectName string          `json:"project_name"`
	Generations []generationDTO `json:"generations"`
	LinkTasks   []linkTaskDTO   `json:"link_tasks"`
}

func (s *Server) toProjectDTO(ctx context.Context, p sqlstore.Project) projectDTO {
	var gb json.RawMessage
	if len(p.GlobalBlacklist) > 0 {
		gb = json.RawMessage(p.GlobalBlacklist)
	}

	// Проверяем наличие API ключа у владельца проекта
	ownerHasApiKey := false
	if encKey, err := s.svc.GetUserAPIKeyEncrypted(ctx, p.UserEmail); err == nil && len(encKey) > 0 {
		ownerHasApiKey = true
	}

	return projectDTO{
		ID:              p.ID,
		Name:            p.Name,
		Status:          p.Status,
		TargetCountry:   p.TargetCountry,
		TargetLanguage:  p.TargetLanguage,
		Timezone:        nullableStringPtr(p.Timezone),
		DefaultServerID: nullableStringPtr(p.DefaultServerID),
		GlobalBlacklist: gb,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
		OwnerHasApiKey:  ownerHasApiKey,
	}
}

func toProjectDTO(p sqlstore.Project) projectDTO {
	// Deprecated: используйте s.toProjectDTO для получения ownerHasApiKey
	var gb json.RawMessage
	if len(p.GlobalBlacklist) > 0 {
		gb = json.RawMessage(p.GlobalBlacklist)
	}
	return projectDTO{
		ID:              p.ID,
		Name:            p.Name,
		Status:          p.Status,
		TargetCountry:   p.TargetCountry,
		TargetLanguage:  p.TargetLanguage,
		Timezone:        nullableStringPtr(p.Timezone),
		DefaultServerID: nullableStringPtr(p.DefaultServerID),
		GlobalBlacklist: gb,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
		OwnerHasApiKey:  false, // По умолчанию false, если не проверено
	}
}

func toProjectDTOs(list []sqlstore.Project) []projectDTO {
	out := make([]projectDTO, 0, len(list))
	for _, p := range list {
		out = append(out, toProjectDTO(p))
	}
	return out
}

func toDomainDTO(d sqlstore.Domain) domainDTO {
	return domainDTO{
		ID:                 d.ID,
		ProjectID:          d.ProjectID,
		ServerID:           nullableStringPtr(d.ServerID),
		URL:                d.URL,
		MainKeyword:        d.MainKeyword,
		TargetCountry:      d.TargetCountry,
		TargetLanguage:     d.TargetLanguage,
		ExcludeDomains:     nullableStringPtr(d.ExcludeDomains),
		Status:             d.Status,
		LastGenerationID:   nullableStringPtr(d.LastGenerationID),
		PublishedAt:        nullableTimePtr(d.PublishedAt),
		LinkAnchorText:     nullableStringPtr(d.LinkAnchorText),
		LinkAcceptorURL:    nullableStringPtr(d.LinkAcceptorURL),
		LinkStatus:         nullableStringPtr(d.LinkStatus),
		LinkUpdatedAt:      nullableTimePtr(d.LinkUpdatedAt),
		LinkLastTaskID:     nullableStringPtr(d.LinkLastTaskID),
		LinkFilePath:       nullableStringPtr(d.LinkFilePath),
		LinkAnchorSnapshot: nullableStringPtr(d.LinkAnchorSnapshot),
		LinkReadyAt:        nullableTimePtr(d.LinkReadyAt),
		CreatedAt:          d.CreatedAt,
		UpdatedAt:          d.UpdatedAt,
	}
}

func toDomainDTOs(list []sqlstore.Domain) []domainDTO {
	out := make([]domainDTO, 0, len(list))
	for _, d := range list {
		out = append(out, toDomainDTO(d))
	}
	return out
}

func toIndexCheckDTO(check sqlstore.IndexCheck) indexCheckDTO {
	dto := indexCheckDTO{
		ID:        check.ID,
		DomainID:  check.DomainID,
		CheckDate: check.CheckDate,
		Status:    check.Status,
		Attempts:  check.Attempts,
		CreatedAt: check.CreatedAt,
	}
	if check.IsIndexed.Valid {
		val := check.IsIndexed.Bool
		dto.IsIndexed = &val
	}
	if check.LastAttemptAt.Valid {
		val := check.LastAttemptAt.Time
		dto.LastAttemptAt = &val
	}
	if check.NextRetryAt.Valid {
		val := check.NextRetryAt.Time
		dto.NextRetryAt = &val
	}
	if check.ErrorMessage.Valid {
		val := check.ErrorMessage.String
		dto.ErrorMessage = &val
	}
	if check.CompletedAt.Valid {
		val := check.CompletedAt.Time
		dto.CompletedAt = &val
	}
	return dto
}

func toIndexCheckHistoryDTO(item sqlstore.CheckHistory) indexCheckHistoryDTO {
	dto := indexCheckHistoryDTO{
		ID:            item.ID,
		CheckID:       item.CheckID,
		AttemptNumber: item.AttemptNumber,
		CreatedAt:     item.CreatedAt,
	}
	if item.Result.Valid {
		val := item.Result.String
		dto.Result = &val
	}
	if len(item.ResponseData) > 0 {
		dto.ResponseData = json.RawMessage(item.ResponseData)
	}
	if item.ErrorMessage.Valid {
		val := item.ErrorMessage.String
		dto.ErrorMessage = &val
	}
	if item.DurationMS.Valid {
		val := item.DurationMS.Int64
		dto.DurationMS = &val
	}
	return dto
}

func toIndexCheckDailyDTO(item sqlstore.IndexCheckDailySummary) indexCheckDailyDTO {
	return indexCheckDailyDTO{
		Date:                item.Date.Format("2006-01-02"),
		Total:               item.Total,
		IndexedTrue:         item.IndexedTrue,
		IndexedFalse:        item.IndexedFalse,
		Pending:             item.Pending,
		Checking:            item.Checking,
		FailedInvestigation: item.FailedInvestigation,
		Success:             item.Success,
	}
}

func buildIndexCheckStatsDTO(
	stats sqlstore.IndexCheckStats,
	daily []sqlstore.IndexCheckDailySummary,
	from time.Time,
	to time.Time,
) indexCheckStatsDTO {
	percent := 0
	if stats.TotalResolved > 0 {
		percent = int(math.Round(float64(stats.IndexedTrue) / float64(stats.TotalResolved) * 100))
	}
	resp := indexCheckStatsDTO{
		From:                 from.Format("2006-01-02"),
		To:                   to.Format("2006-01-02"),
		TotalChecks:          stats.TotalChecks,
		TotalResolved:        stats.TotalResolved,
		IndexedTrue:          stats.IndexedTrue,
		PercentIndexed:       percent,
		AvgAttemptsToSuccess: stats.AvgAttemptsToSuccess,
		FailedInvestigation:  stats.FailedInvestigation,
		Daily:                []indexCheckDailyDTO{},
	}
	if len(daily) > 0 {
		resp.Daily = make([]indexCheckDailyDTO, 0, len(daily))
		for _, item := range daily {
			resp.Daily = append(resp.Daily, toIndexCheckDailyDTO(item))
		}
	}
	return resp
}

func toGenerationDTO(g sqlstore.Generation) generationDTO {
	return generationDTO{
		ID:         g.ID,
		DomainID:   g.DomainID,
		Status:     g.Status,
		Progress:   g.Progress,
		Error:      nullableStringPtr(g.Error),
		PromptID:   nullableStringPtr(g.PromptID),
		CreatedAt:  g.CreatedAt,
		UpdatedAt:  g.UpdatedAt,
		StartedAt:  nullableTimePtr(g.StartedAt),
		FinishedAt: nullableTimePtr(g.FinishedAt),
		Logs:       rawJSONOrNil(g.Logs),
		Artifacts:  rawJSONOrNil(g.Artifacts),
	}
}

func toGenerationDTOs(list []sqlstore.Generation) []generationDTO {
	out := make([]generationDTO, 0, len(list))
	for _, g := range list {
		out = append(out, toGenerationDTO(g))
	}
	return out
}

func toGenerationDTOWithDomain(g sqlstore.Generation, domainURL *string) generationDTO {
	dto := toGenerationDTO(g)
	dto.DomainURL = domainURL
	return dto
}

func (s *Server) buildIndexCheckResponse(ctx context.Context, list []sqlstore.IndexCheck) []indexCheckDTO {
	resp := make([]indexCheckDTO, 0, len(list))
	if len(list) == 0 {
		return resp
	}

	domainIDs := make([]string, 0, len(list))
	seen := make(map[string]struct{})
	for _, check := range list {
		if _, ok := seen[check.DomainID]; ok {
			continue
		}
		seen[check.DomainID] = struct{}{}
		domainIDs = append(domainIDs, check.DomainID)
	}

	domainMap := map[string]sqlstore.Domain{}
	if s.domains != nil && len(domainIDs) > 0 {
		if domains, err := s.domains.ListByIDs(ctx, domainIDs); err == nil {
			for _, d := range domains {
				domainMap[d.ID] = d
			}
		}
	}

	for _, check := range list {
		dto := toIndexCheckDTO(check)
		if domain, ok := domainMap[check.DomainID]; ok {
			if url := strings.TrimSpace(domain.URL); url != "" {
				dto.DomainURL = &url
			}
			pid := domain.ProjectID
			dto.ProjectID = &pid
		}
		resp = append(resp, dto)
	}
	return resp
}

func toAdminUserDTO(u auth.User) adminUserDTO {
	return adminUserDTO{
		Email:           u.Email,
		Name:            u.Name,
		Role:            u.Role,
		IsApproved:      u.IsApproved,
		Verified:        u.Verified,
		CreatedAt:       u.CreatedAt,
		APIKeyUpdatedAt: u.APIKeySetAt,
	}
}

func toAdminPromptDTO(p sqlstore.SystemPrompt) adminPromptDTO {
	return adminPromptDTO{
		ID:          p.ID,
		Name:        p.Name,
		Description: nullableStringPtr(p.Description),
		Body:        p.Body,
		Stage:       nullableStringPtr(p.Stage),
		Model:       nullableStringPtr(p.Model),
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func toScheduleDTO(s sqlstore.Schedule) scheduleDTO {
	var cfg json.RawMessage
	if len(s.Config) > 0 {
		cfg = json.RawMessage(s.Config)
	}
	return scheduleDTO{
		ID:          s.ID,
		ProjectID:   s.ProjectID,
		Name:        s.Name,
		Description: nullableStringPtr(s.Description),
		Strategy:    s.Strategy,
		Config:      cfg,
		IsActive:    s.IsActive,
		CreatedBy:   s.CreatedBy,
		LastRunAt:   nullableTimePtr(s.LastRunAt),
		NextRunAt:   nullableTimePtr(s.NextRunAt),
		Timezone:    nullableStringPtr(s.Timezone),
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func toLinkScheduleDTO(s sqlstore.LinkSchedule) linkScheduleDTO {
	return linkScheduleDTO{
		ID:        s.ID,
		ProjectID: s.ProjectID,
		Name:      s.Name,
		Config:    s.Config,
		IsActive:  s.IsActive,
		CreatedBy: s.CreatedBy,
		LastRunAt: nullableTimePtr(s.LastRunAt),
		NextRunAt: nullableTimePtr(s.NextRunAt),
		Timezone:  nullableStringPtr(s.Timezone),
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

type scheduleConfig struct {
	Limit        int
	Cron         string
	Time         string
	Weekday      string
	Day          int
	Interval     string
	DelayMinutes int
}

func parseScheduleConfig(raw json.RawMessage) (scheduleConfig, error) {
	if len(raw) == 0 {
		return scheduleConfig{DelayMinutes: 60}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return scheduleConfig{}, err
	}
	cfg := scheduleConfig{}
	delaySet := false
	if v, ok := data["limit"].(float64); ok {
		cfg.Limit = int(v)
	}
	if v, ok := data["cron"].(string); ok {
		cfg.Cron = strings.TrimSpace(v)
	}
	if v, ok := data["time"].(string); ok {
		cfg.Time = strings.TrimSpace(v)
	}
	if v, ok := data["weekday"].(string); ok {
		cfg.Weekday = strings.TrimSpace(v)
	}
	if v, ok := data["day"].(float64); ok {
		cfg.Day = int(v)
	}
	if v, ok := data["day"].(string); ok {
		trimmed := strings.TrimSpace(v)
		if parsed, err := strconv.Atoi(trimmed); err == nil {
			cfg.Day = parsed
		} else if trimmed != "" {
			cfg.Weekday = strings.ToLower(trimmed)
		}
	}
	if v, ok := data["interval"].(string); ok {
		cfg.Interval = strings.TrimSpace(v)
	}
	if rawDelay, ok := data["delay_minutes"]; ok {
		delaySet = true
		switch value := rawDelay.(type) {
		case float64:
			cfg.DelayMinutes = int(value)
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				parsed, err := strconv.Atoi(trimmed)
				if err != nil {
					return scheduleConfig{}, fmt.Errorf("invalid delay_minutes")
				}
				cfg.DelayMinutes = parsed
			}
		default:
			return scheduleConfig{}, fmt.Errorf("invalid delay_minutes")
		}
		if cfg.DelayMinutes < 0 {
			return scheduleConfig{}, fmt.Errorf("invalid delay_minutes")
		}
	}
	if !delaySet {
		cfg.DelayMinutes = 60
	}
	return cfg, nil
}

func computeLinkScheduleNextRun(cfg scheduleConfig, now time.Time, lastRun time.Time) (time.Time, bool, error) {
	strategy := "daily"
	if strings.TrimSpace(cfg.Cron) != "" || strings.TrimSpace(cfg.Interval) != "" {
		strategy = "custom"
	} else if strings.TrimSpace(cfg.Weekday) != "" || cfg.Day > 0 {
		strategy = "weekly"
	}
	return computeScheduleNextRun(strategy, cfg, now, lastRun)
}

func computeScheduleNextRun(strategy string, cfg scheduleConfig, now time.Time, lastRun time.Time) (time.Time, bool, error) {
	strategy = strings.ToLower(strings.TrimSpace(strategy))
	if strategy == "" {
		return time.Time{}, false, fmt.Errorf("strategy is required")
	}

	if strategy == "immediate" {
		return time.Time{}, lastRun.IsZero(), nil
	}

	if cfg.Interval != "" {
		interval, err := parseInterval(cfg.Interval)
		if err != nil {
			return time.Time{}, false, err
		}
		if lastRun.IsZero() || now.Sub(lastRun) >= interval {
			return now.Add(interval), true, nil
		}
		return lastRun.Add(interval), false, nil
	}

	switch strategy {
	case "daily":
		scheduled, err := scheduleAtTime(now, cfg.Time)
		if err != nil {
			return time.Time{}, false, err
		}
		if now.Before(scheduled) {
			return scheduled, false, nil
		}
		if !lastRun.IsZero() && sameDay(lastRun, now) {
			return scheduled.Add(24 * time.Hour), false, nil
		}
		return scheduled.Add(24 * time.Hour), true, nil
	case "weekly":
		targetWeekday, ok := resolveWeekday(cfg.Weekday, cfg.Day)
		if !ok {
			return time.Time{}, false, fmt.Errorf("weekday is required")
		}
		scheduled, err := scheduleAtWeekday(now, targetWeekday, cfg.Time)
		if err != nil {
			return time.Time{}, false, err
		}
		if now.Before(scheduled) {
			return scheduled, false, nil
		}
		if !lastRun.IsZero() && sameWeek(lastRun, now) {
			return scheduled.Add(7 * 24 * time.Hour), false, nil
		}
		return scheduled.Add(7 * 24 * time.Hour), true, nil
	case "custom":
		if cfg.Cron == "" && cfg.Interval == "" {
			return time.Time{}, false, fmt.Errorf("cron or interval is required for custom strategy")
		}
		if cfg.Cron == "" {
			return time.Time{}, false, fmt.Errorf("cron is required for custom strategy")
		}
		next, due, err := cronNext(cfg.Cron, now)
		return next, due, err
	default:
		return time.Time{}, false, fmt.Errorf("unknown strategy: %s", strategy)
	}
}

func cronNext(expr string, now time.Time) (time.Time, bool, error) {
	sched, err := cron.ParseStandard(expr)
	if err != nil {
		return time.Time{}, false, err
	}
	windowStart := now.Add(-time.Minute)
	next := sched.Next(windowStart)
	if !next.After(now) {
		return sched.Next(now), true, nil
	}
	return next, false, nil
}

func scheduleAtTime(now time.Time, value string) (time.Time, error) {
	hour, minute, err := parseHourMinute(value, now)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location()), nil
}

func scheduleAtWeekday(now time.Time, weekday time.Weekday, value string) (time.Time, error) {
	hour, minute, err := parseHourMinute(value, now)
	if err != nil {
		return time.Time{}, err
	}
	diff := int(weekday - now.Weekday())
	if diff < 0 {
		diff += 7
	}
	target := now.AddDate(0, 0, diff)
	return time.Date(target.Year(), target.Month(), target.Day(), hour, minute, 0, 0, now.Location()), nil
}

func parseHourMinute(value string, now time.Time) (int, int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, 0, nil
	}
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minute")
	}
	return hour, minute, nil
}

func resolveWeekday(value string, day int) (time.Weekday, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "mon", "monday":
		return time.Monday, true
	case "tue", "tues", "tuesday":
		return time.Tuesday, true
	case "wed", "wednesday":
		return time.Wednesday, true
	case "thu", "thurs", "thursday":
		return time.Thursday, true
	case "fri", "friday":
		return time.Friday, true
	case "sat", "saturday":
		return time.Saturday, true
	case "sun", "sunday":
		return time.Sunday, true
	}
	if day >= 1 && day <= 7 {
		return time.Weekday(day % 7), true
	}
	return time.Weekday(0), false
}

func enqueueScheduleDomains(
	ctx context.Context,
	genQueue GenQueueStore,
	sched sqlstore.Schedule,
	cfg scheduleConfig,
	domains []sqlstore.Domain,
	queueItems []sqlstore.QueueItem,
	now time.Time,
) (int, error) {
	eligibleDomains := filterGenerationDomains(domains)
	eligibleIDs := map[string]bool{}
	for _, d := range eligibleDomains {
		eligibleIDs[d.ID] = true
	}

	queuedDomains := map[string]bool{}
	queuedForSchedule := 0
	for _, item := range queueItems {
		if item.Status != "pending" && item.Status != "queued" {
			continue
		}
		if !eligibleIDs[item.DomainID] {
			continue
		}
		queuedDomains[item.DomainID] = true
		if item.ScheduleID.Valid && item.ScheduleID.String == sched.ID {
			queuedForSchedule++
		}
	}

	limit := cfg.Limit
	if limit <= 0 {
		limit = len(eligibleDomains)
	}
	remaining := limit - queuedForSchedule
	if remaining <= 0 {
		return 0, nil
	}

	count := 0
	for _, d := range eligibleDomains {
		if count >= remaining {
			break
		}
		if queuedDomains[d.ID] {
			continue
		}
		item := sqlstore.QueueItem{
			ID:           uuid.NewString(),
			DomainID:     d.ID,
			ScheduleID:   sql.NullString{String: sched.ID, Valid: true},
			Priority:     0,
			ScheduledFor: now,
			Status:       "pending",
		}
		if err := genQueue.Enqueue(ctx, item); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func shouldAllowManualTrigger(strategy string, cfg scheduleConfig, now time.Time, lastRun time.Time) (bool, error) {
	strategy = strings.ToLower(strings.TrimSpace(strategy))
	if strategy == "" {
		return true, nil
	}
	if lastRun.IsZero() {
		return true, nil
	}
	switch strategy {
	case "immediate":
		return false, nil
	case "daily":
		return !sameDay(lastRun, now), nil
	case "weekly":
		return !sameWeek(lastRun, now), nil
	case "custom":
		if cfg.Interval != "" {
			interval, err := parseInterval(cfg.Interval)
			if err != nil {
				return false, err
			}
			return now.Sub(lastRun) >= interval, nil
		}
		if cfg.Cron != "" {
			return cronAllowsManual(cfg.Cron, now, lastRun)
		}
		return true, nil
	default:
		return true, nil
	}
}

func cronAllowsManual(expr string, now time.Time, lastRun time.Time) (bool, error) {
	if lastRun.IsZero() {
		return true, nil
	}
	sched, err := cron.ParseStandard(expr)
	if err != nil {
		return false, err
	}
	next := sched.Next(lastRun)
	return !now.Before(next), nil
}

func lastScheduledAt(items []sqlstore.QueueItem, scheduleID string) time.Time {
	var last time.Time
	for _, item := range items {
		if !item.ScheduleID.Valid || item.ScheduleID.String != scheduleID {
			continue
		}
		if item.ScheduledFor.After(last) {
			last = item.ScheduledFor
		}
	}
	return last
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func sameWeek(a, b time.Time) bool {
	ay, aw := a.ISOWeek()
	by, bw := b.ISOWeek()
	return ay == by && aw == bw
}

func parseInterval(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("interval is empty")
	}
	if strings.HasSuffix(value, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(value, "w") {
		n, err := strconv.Atoi(strings.TrimSuffix(value, "w"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return time.ParseDuration(value)
}

func scheduleRunAtToday(now time.Time, cfg scheduleConfig) (time.Time, error) {
	return scheduleAtTime(now, cfg.Time)
}

func effectiveLinkReadyAt(domain sqlstore.Domain, scheduleRunAt time.Time) time.Time {
	effective := scheduleRunAt
	if effective.IsZero() && domain.LinkReadyAt.Valid {
		return domain.LinkReadyAt.Time
	}
	if domain.LinkReadyAt.Valid && domain.LinkReadyAt.Time.After(effective) {
		return domain.LinkReadyAt.Time
	}
	return effective
}

func isLinkStatusEligible(domain sqlstore.Domain) bool {
	if !domain.LinkStatus.Valid {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(domain.LinkStatus.String))
	if status == "" || status == "ready" {
		return true
	}
	return status == "needs_relink" || status == "pending"
}

func filterLinkDomains(domains []sqlstore.Domain, now time.Time, scheduleRunAt time.Time) []sqlstore.Domain {
	res := make([]sqlstore.Domain, 0, len(domains))
	for _, d := range domains {
		if !isDomainPublished(d) {
			continue
		}
		if !isLinkStatusEligible(d) {
			continue
		}
		anchor := strings.TrimSpace(d.LinkAnchorText.String)
		target := strings.TrimSpace(d.LinkAcceptorURL.String)
		if !d.LinkAnchorText.Valid || !d.LinkAcceptorURL.Valid || anchor == "" || target == "" {
			continue
		}
		effective := effectiveLinkReadyAt(d, scheduleRunAt)
		if !effective.IsZero() && effective.After(now) {
			continue
		}
		res = append(res, d)
	}
	return res
}

func filterGenerationDomains(domains []sqlstore.Domain) []sqlstore.Domain {
	res := make([]sqlstore.Domain, 0, len(domains))
	for _, d := range domains {
		if !isDomainWaiting(d) {
			continue
		}
		res = append(res, d)
	}
	return res
}

func isDomainPublished(d sqlstore.Domain) bool {
	if d.PublishedAt.Valid {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(d.Status), "published")
}

func isDomainWaiting(d sqlstore.Domain) bool {
	return strings.EqualFold(strings.TrimSpace(d.Status), "waiting")
}

func listActiveLinkTasksByProject(ctx context.Context, linkTaskStore LinkTaskStore, projectID string) ([]sqlstore.LinkTask, error) {
	activeStatuses := []string{"pending", "searching", "found", "removing"}
	var res []sqlstore.LinkTask
	for _, status := range activeStatuses {
		status := status
		filters := sqlstore.LinkTaskFilters{Status: &status}
		list, err := linkTaskStore.ListByProject(ctx, projectID, filters)
		if err != nil {
			return nil, err
		}
		res = append(res, list...)
	}
	return res, nil
}

func toQueueItemDTO(item sqlstore.QueueItem) queueItemDTO {
	return queueItemDTO{
		ID:           item.ID,
		DomainID:     item.DomainID,
		ScheduleID:   nullableStringPtr(item.ScheduleID),
		Priority:     item.Priority,
		ScheduledFor: item.ScheduledFor,
		Status:       item.Status,
		ErrorMessage: nullableStringPtr(item.ErrorMessage),
		CreatedAt:    item.CreatedAt,
		ProcessedAt:  nullableTimePtr(item.ProcessedAt),
	}
}

func toLinkTaskDTO(task sqlstore.LinkTask) linkTaskDTO {
	return linkTaskDTO{
		ID:               task.ID,
		DomainID:         task.DomainID,
		AnchorText:       task.AnchorText,
		TargetURL:        task.TargetURL,
		ScheduledFor:     task.ScheduledFor,
		Action:           task.Action,
		Status:           task.Status,
		FoundLocation:    nullableStringPtr(task.FoundLocation),
		GeneratedContent: nullableStringPtr(task.GeneratedContent),
		ErrorMessage:     nullableStringPtr(task.ErrorMessage),
		LogLines:         task.LogLines,
		Attempts:         task.Attempts,
		CreatedBy:        task.CreatedBy,
		CreatedAt:        task.CreatedAt,
		CompletedAt:      nullableTimePtr(task.CompletedAt),
	}
}

type adminAuditRuleDTO struct {
	Code        string    `json:"code"`
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	Severity    string    `json:"severity"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func toAdminAuditRuleDTO(r sqlstore.AuditRule) adminAuditRuleDTO {
	var desc *string
	if strings.TrimSpace(r.Description) != "" {
		desc = &r.Description
	}
	return adminAuditRuleDTO{
		Code:        r.Code,
		Title:       r.Title,
		Description: desc,
		Severity:    r.Severity,
		IsActive:    r.IsActive,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func nullableStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		v := ns.String
		return &v
	}
	return nil
}

func nullStringFromOptional(val *string) sql.NullString {
	if val == nil {
		return sql.NullString{}
	}
	trimmed := strings.TrimSpace(*val)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func nullableTimePtr(t sql.NullTime) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}

func rawJSONOrNil(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	return v
}

func sanitizeDomain(input string) string {
	s := strings.TrimSpace(input)
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "//")
	s = strings.TrimRight(s, "/")
	return s
}

type contextKey string

const tokenContextKey contextKey = "token"
const userEmailContextKey contextKey = "userEmail"
const sessionContextKey contextKey = "session"
const currentUserContextKey contextKey = "currentUser"

var (
	errForbidden       = errors.New("forbidden")
	errUnauthorized    = errors.New("unauthorized")
	errPendingApproval = errors.New("account pending approval")
)

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := getAccessFromCookie(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		sess, err := s.svc.ValidateToken(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		u, err := s.svc.GetUser(r.Context(), sess.Email)
		if err != nil {
			s.logger.Warnw("failed to load user for session", "email", sess.Email, "err", err)
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), tokenContextKey, token)
		ctx = context.WithValue(ctx, userEmailContextKey, sess.Email)
		ctx = context.WithValue(ctx, sessionContextKey, sess)
		ctx = context.WithValue(ctx, currentUserContextKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func tokenFromContext(ctx context.Context) string {
	val, _ := ctx.Value(tokenContextKey).(string)
	return val
}

func userEmailFromContext(ctx context.Context) string {
	val, _ := ctx.Value(userEmailContextKey).(string)
	return val
}

func sessionFromContext(ctx context.Context) auth.Session {
	val, _ := ctx.Value(sessionContextKey).(auth.Session)
	return val
}

func currentUserFromContext(ctx context.Context) (auth.User, bool) {
	val, ok := ctx.Value(currentUserContextKey).(auth.User)
	return val, ok
}

func requireApprovedUser(u auth.User) error {
	if strings.EqualFold(u.Role, "admin") {
		return nil
	}
	if !u.IsApproved {
		return errPendingApproval
	}
	return nil
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := currentUserFromContext(r.Context())
		if !ok || !strings.EqualFold(user.Role, "admin") {
			writeError(w, http.StatusForbidden, "admin only")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// projectMemberRoleKey используется для передачи роли участника проекта в контексте
type projectMemberRoleKeyType string

const projectMemberRoleKey projectMemberRoleKeyType = "project_member_role"

func (s *Server) authorizeProject(ctx context.Context, projectID string) (sqlstore.Project, error) {
	u, ok := currentUserFromContext(ctx)
	if !ok || u.Email == "" {
		return sqlstore.Project{}, errUnauthorized
	}

	// Admin имеет доступ ко всем проектам
	if strings.EqualFold(u.Role, "admin") {
		return s.projects.GetByID(ctx, projectID)
	}

	// Проверяем, является ли пользователь владельцем
	p, err := s.projects.Get(ctx, projectID, u.Email)
	if err == nil {
		// Пользователь - владелец, роль "owner" уже в контексте (неявно)
		return p, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return sqlstore.Project{}, err
	}

	// Проверяем, является ли пользователь участником
	if s.projectMembers != nil {
		member, err := s.projectMembers.Get(ctx, projectID, u.Email)
		if err == nil {
			// Пользователь - участник, сохраняем роль в контексте
			ctx = context.WithValue(ctx, projectMemberRoleKey, member.Role)
			p, err = s.projects.GetByID(ctx, projectID)
			if err != nil {
				return sqlstore.Project{}, err
			}
			return p, nil
		}
	}

	return sqlstore.Project{}, errForbidden
}

// getProjectMemberRole возвращает роль пользователя в проекте (owner, editor, viewer)
func (s *Server) getProjectMemberRole(ctx context.Context, projectID, email string) (string, error) {
	// Админ имеет доступ ко всем проектам
	if user, ok := currentUserFromContext(ctx); ok && strings.EqualFold(user.Role, "admin") {
		return "admin", nil
	}
	// Проверяем, является ли пользователь владельцем
	p, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return "", err
	}
	if p.UserEmail == email {
		return "owner", nil
	}

	// Проверяем роль участника
	if s.projectMembers != nil {
		member, err := s.projectMembers.Get(ctx, projectID, email)
		if err == nil {
			return member.Role, nil
		}
	}

	return "", errors.New("user is not a member of this project")
}

// hasProjectPermission проверяет, имеет ли пользователь достаточные права для действия
func hasProjectPermission(userRole string, memberRole string, requiredRole string) bool {
	// Admin имеет все права
	if strings.EqualFold(userRole, "admin") {
		return true
	}

	// Определяем фактическую роль пользователя в проекте
	actualRole := memberRole
	if memberRole == "" {
		actualRole = "owner" // Если нет роли в project_members, значит владелец
	}

	// Иерархия ролей: viewer < editor < owner
	roleHierarchy := map[string]int{
		"viewer": 1,
		"editor": 2,
		"owner":  3,
	}

	userLevel := roleHierarchy[actualRole]
	requiredLevel := roleHierarchy[requiredRole]

	return userLevel >= requiredLevel
}

func parseLimitParam(r *http.Request, fallback int, max int) int {
	limit := fallback
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if max > 0 && n > max {
				n = max
			}
			limit = n
		}
	}
	return limit
}

func parsePageParam(r *http.Request, fallback int) int {
	page := fallback
	if v := strings.TrimSpace(r.URL.Query().Get("page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	return page
}

func parseIndexCheckFilters(r *http.Request, fallbackLimit int, maxLimit int) sqlstore.IndexCheckFilters {
	limit := parseLimitParam(r, fallbackLimit, maxLimit)
	page := parsePageParam(r, 1)
	offset := (page - 1) * limit
	filters := sqlstore.IndexCheckFilters{
		Limit:  limit,
		Offset: offset,
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		filters.Statuses = parseStatusList(status)
	}
	if search := strings.TrimSpace(r.URL.Query().Get("search")); search != "" {
		filters.Search = &search
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("sort")); raw != "" {
		if key, dir := parseIndexCheckSort(raw); key != "" {
			filters.SortBy = key
			filters.SortDir = dir
		}
	}
	if domainID := strings.TrimSpace(r.URL.Query().Get("domain_id")); domainID != "" {
		filters.DomainID = &domainID
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("is_indexed")); raw != "" {
		if val, err := strconv.ParseBool(raw); err == nil {
			filters.IsIndexed = &val
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		if dt, ok := parseIndexCheckDate(raw); ok {
			filters.From = &dt
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		if dt, ok := parseIndexCheckDate(raw); ok {
			filters.To = &dt
		}
	}
	return filters
}

func resolveIndexCheckStatsRange(filters *sqlstore.IndexCheckFilters, now time.Time) (time.Time, time.Time) {
	const defaultDays = 30
	var from time.Time
	var to time.Time
	if filters.From != nil {
		from = dateOnlyUTC(*filters.From)
	}
	if filters.To != nil {
		to = dateOnlyUTC(*filters.To)
	}
	switch {
	case !from.IsZero() && !to.IsZero():
		// keep
	case !from.IsZero() && to.IsZero():
		to = dateOnlyUTC(now)
	case from.IsZero() && !to.IsZero():
		from = dateOnlyUTC(to.AddDate(0, 0, -defaultDays+1))
	default:
		to = dateOnlyUTC(now)
		from = dateOnlyUTC(now.AddDate(0, 0, -defaultDays+1))
	}
	if from.After(to) {
		from, to = to, from
	}
	filters.From = &from
	filters.To = &to
	return from, to
}

func resolveIndexCheckCalendarRange(filters *sqlstore.IndexCheckFilters, r *http.Request, now time.Time) (time.Time, time.Time) {
	if month := strings.TrimSpace(r.URL.Query().Get("month")); month != "" {
		if from, to, ok := parseIndexCheckMonth(month); ok {
			filters.From = &from
			filters.To = &to
			return from, to
		}
	}
	if filters.From != nil || filters.To != nil {
		from, to := resolveIndexCheckStatsRange(filters, now)
		return from, to
	}
	current := dateOnlyUTC(now)
	from := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(current.Year(), current.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	filters.From = &from
	filters.To = &to
	return from, to
}

func parseIndexCheckMonth(raw string) (time.Time, time.Time, bool) {
	t, err := time.Parse("2006-01", raw)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	from := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	return from, to, true
}

func parseStatusList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		status := strings.TrimSpace(part)
		if status == "" {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		out = append(out, status)
	}
	return out
}

func parseIndexCheckSort(raw string) (string, string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", ""
	}
	parts := strings.SplitN(value, ":", 2)
	key := strings.TrimSpace(parts[0])
	if !isAllowedIndexCheckSort(key) {
		return "", ""
	}
	dir := "desc"
	if len(parts) > 1 {
		switch strings.ToLower(strings.TrimSpace(parts[1])) {
		case "asc":
			dir = "asc"
		case "desc":
			dir = "desc"
		}
	}
	return key, dir
}

func isAllowedIndexCheckSort(key string) bool {
	switch key {
	case "domain", "check_date", "status", "attempts", "is_indexed", "last_attempt_at", "next_retry_at", "created_at":
		return true
	default:
		return false
	}
}

func parseIndexCheckDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if dt, err := time.Parse(time.RFC3339, raw); err == nil {
		return dateOnlyUTC(dt), true
	}
	if dt, err := time.Parse("2006-01-02", raw); err == nil {
		return dateOnlyUTC(dt), true
	}
	return time.Time{}, false
}

func dateOnlyUTC(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func joinURLPath(parts []string) (string, error) {
	if len(parts) == 0 {
		return "", nil
	}
	segs := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		v, err := url.PathUnescape(p)
		if err != nil {
			return "", err
		}
		segs = append(segs, v)
	}
	return strings.Join(segs, "/"), nil
}

func sanitizeFilePath(p string) (string, error) {
	if strings.Contains(p, "\\") {
		return "", errors.New("path contains backslash")
	}
	clean := path.Clean(strings.TrimSpace(p))
	if clean == "." || clean == "" {
		return "", errors.New("path is empty")
	}
	if path.IsAbs(clean) {
		return "", errors.New("path is absolute")
	}
	if strings.HasPrefix(clean, "../") || clean == ".." {
		return "", errors.New("path traversal detected")
	}
	parts := strings.Split(clean, "/")
	for _, part := range parts {
		if part == ".." {
			return "", errors.New("path traversal detected")
		}
	}
	return clean, nil
}

func domainFilesDir(domainURL string) (string, error) {
	domain := strings.TrimSpace(domainURL)
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	if idx := strings.IndexAny(domain, "/?"); idx >= 0 {
		domain = domain[:idx]
	}
	if idx := strings.Index(domain, ":"); idx >= 0 {
		domain = domain[:idx]
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", errors.New("domain is empty")
	}
	if strings.Contains(domain, "..") || strings.Contains(domain, "\\") || strings.ContainsAny(domain, " \t\n\r") {
		return "", errors.New("invalid domain")
	}
	baseDir := "server"
	info, err := os.Stat(baseDir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", baseDir)
	}
	full := filepath.Join(baseDir, domain)
	if err := ensurePathWithin(baseDir, full); err != nil {
		return "", err
	}
	return full, nil
}

func ensurePathWithin(baseDir, target string) error {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	baseAbs = filepath.Clean(baseAbs)
	if targetAbs == baseAbs {
		return errors.New("path equals base dir")
	}
	if !strings.HasPrefix(targetAbs, baseAbs+string(os.PathSeparator)) {
		return errors.New("path escapes base dir")
	}
	return nil
}

func detectMimeType(path string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}
	if len(content) > 0 {
		return http.DetectContentType(content)
	}
	return "application/octet-stream"
}

func baseMimeType(m string) string {
	return strings.TrimSpace(strings.SplitN(m, ";", 2)[0])
}

func validateMimeType(path string, detected string, existing string) error {
	if strings.TrimSpace(detected) == "" {
		return errors.New("mime type is empty")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return errors.New("file extension is required")
	}
	expected := mime.TypeByExtension(ext)
	if expected == "" && baseMimeType(detected) == "application/octet-stream" {
		return errors.New("unsupported mime type")
	}
	if expected != "" && baseMimeType(detected) != baseMimeType(expected) {
		return fmt.Errorf("mime type mismatch: expected %s, got %s", baseMimeType(expected), baseMimeType(detected))
	}
	if existing != "" && baseMimeType(existing) != baseMimeType(detected) {
		return fmt.Errorf("mime type mismatch: expected %s, got %s", baseMimeType(existing), baseMimeType(detected))
	}
	return nil
}

// requireProjectRole проверяет права доступа к проекту
func (s *Server) requireProjectRole(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Извлекаем projectID из пути
			path := strings.TrimPrefix(r.URL.Path, "/api/projects/")
			parts := strings.Split(path, "/")
			projectID := parts[0]

			user, ok := currentUserFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			// Получаем роль пользователя в проекте
			memberRole, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
			if err != nil {
				writeError(w, http.StatusForbidden, "access denied")
				return
			}

			// Проверяем права
			if !hasProjectPermission(user.Role, memberRole, requiredRole) {
				writeError(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) authorizeDomain(ctx context.Context, domainID string) (sqlstore.Domain, sqlstore.Project, error) {
	d, err := s.domains.Get(ctx, domainID)
	if err != nil {
		return sqlstore.Domain{}, sqlstore.Project{}, err
	}
	p, err := s.authorizeProject(ctx, d.ProjectID)
	if err != nil {
		return sqlstore.Domain{}, sqlstore.Project{}, err
	}
	return d, p, nil
}

func respondAuthzError(w http.ResponseWriter, err error, notFoundMsg string) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if notFoundMsg == "" {
			notFoundMsg = "not found"
		}
		writeError(w, http.StatusNotFound, notFoundMsg)
	case errors.Is(err, errForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, errUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, errPendingApproval):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
	return true
}

func setAuthCookies(w http.ResponseWriter, cfg config.Config, tokens auth.TokenPair) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    tokens.Access,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		Domain:   cfg.CookieDomain,
		SameSite: http.SameSiteLaxMode,
		Expires:  tokens.AccessExp,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    tokens.Refresh,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		Domain:   cfg.CookieDomain,
		SameSite: http.SameSiteLaxMode,
		Expires:  tokens.RefreshExp,
	})
}

func clearAuthCookies(w http.ResponseWriter, cfg config.Config) {
	expired := time.Unix(0, 0)
	for _, name := range []string{"access_token", "refresh_token"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.CookieSecure,
			Domain:   cfg.CookieDomain,
			SameSite: http.SameSiteLaxMode,
			Expires:  expired,
		})
	}
}

func getAccessFromCookie(r *http.Request) string {
	c, err := r.Cookie("access_token")
	if err != nil {
		return ""
	}
	return c.Value
}

func getRefreshFromCookie(r *http.Request) string {
	c, err := r.Cookie("refresh_token")
	if err != nil {
		return ""
	}
	return c.Value
}

func isOriginAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return true
	}
	for _, a := range allowed {
		if a == "*" {
			return true
		}
		if strings.EqualFold(origin, a) {
			return true
		}
	}
	return false
}

type ctxKey string

const reqIDKey ctxKey = "reqID"

func (s *Server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", rid)
		ctx := context.WithValue(r.Context(), reqIDKey, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIDFromContext(ctx context.Context) string {
	val, _ := ctx.Value(reqIDKey).(string)
	return val
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (s *Server) withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		labels := prometheus.Labels{
			"method": r.Method,
			"path":   r.URL.Path,
			"status": strconv.Itoa(sw.status),
		}
		s.reqDuration.With(labels).Observe(time.Since(start).Seconds())
		s.reqCounter.With(labels).Inc()
	})
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		duration := time.Since(start)
		rid := requestIDFromContext(r.Context())
		s.logger.Infow("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", duration.Seconds()*1000,
			"ip", clientIP(r),
			"request_id", rid,
		)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
