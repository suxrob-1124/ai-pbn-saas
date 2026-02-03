package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	Get(ctx context.Context, id string) (sqlstore.Domain, error)
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateKeyword(ctx context.Context, id, keyword string) error
	SetLastGeneration(ctx context.Context, id, genID string) error
	UpdateExtras(ctx context.Context, id, country, language string, exclude, server sql.NullString) (bool, error)
	Delete(ctx context.Context, id string) error
	EnsureDefaultServer(ctx context.Context, email string) error
}

type GenerationStore interface {
	Get(ctx context.Context, id string) (sqlstore.Generation, error)
	Create(ctx context.Context, g sqlstore.Generation) error
	ListByDomain(ctx context.Context, domainID string) ([]sqlstore.Generation, error)
	ListRecentByUser(ctx context.Context, email string, limit int) ([]sqlstore.Generation, error)
	ListRecentAll(ctx context.Context, limit int) ([]sqlstore.Generation, error)
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

type Server struct {
	cfg            config.Config
	svc            *auth.Service
	projects       ProjectStore
	projectMembers *sqlstore.ProjectMemberStore
	domains        DomainStore
	generations    GenerationStore
	prompts        PromptStore
	tasks          tasks.Enqueuer
	reqDuration    *prometheus.HistogramVec
	reqCounter     *prometheus.CounterVec
	genStatus      *prometheus.GaugeVec
	registry       *prometheus.Registry
	logger         *zap.SugaredLogger
}

func New(cfg config.Config, svc *auth.Service, logger *zap.SugaredLogger, projects ProjectStore, projectMembers *sqlstore.ProjectMemberStore, domains DomainStore, generations GenerationStore, prompts PromptStore, tq tasks.Enqueuer) *Server {
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
	mux.Handle("/api/projects", s.withAuth(http.HandlerFunc(s.handleProjects)))
	mux.Handle("/api/projects/", s.withAuth(http.HandlerFunc(s.handleProjectByID)))
	mux.Handle("/api/domains/", s.withAuth(http.HandlerFunc(s.handleDomainActions)))
	mux.Handle("/api/generations", s.withAuth(http.HandlerFunc(s.handleGenerations)))
	mux.Handle("/api/generations/", s.withAuth(http.HandlerFunc(s.handleGenerationByID)))
	mux.Handle("/api/admin/users", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminUsers))))
	mux.Handle("/api/admin/users/", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminUserRoute))))
	mux.Handle("/api/admin/prompts", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminPrompts))))
	mux.Handle("/api/admin/prompts/", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminPromptByID))))
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
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
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	tokens, err := s.svc.Refresh(r.Context(), refresh)
	if err != nil {
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
			Name    string `json:"name"`
			Country string `json:"country"`
			Lang    string `json:"language"`
			Status  string `json:"status"`
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if body.Status == "" {
			body.Status = "draft"
		}
		id := uuid.NewString()
		p := sqlstore.Project{
			ID:             id,
			UserEmail:      user.Email,
			Name:           body.Name,
			TargetCountry:  body.Country,
			TargetLanguage: body.Lang,
			Status:         body.Status,
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
			Name    string `json:"name"`
			Country string `json:"country"`
			Lang    string `json:"language"`
			Status  string `json:"status"`
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		p := sqlstore.Project{
			ID:             projectID,
			UserEmail:      project.UserEmail,
			Name:           body.Name,
			TargetCountry:  body.Country,
			TargetLanguage: body.Lang,
			Status:         body.Status,
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
			Keyword        *string `json:"keyword"`
			Status         *string `json:"status"`
			Country        *string `json:"country"`
			Language       *string `json:"language"`
			ExcludeDomains *string `json:"exclude_domains"`
			ServerID       *string `json:"server_id"`
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
		_, _ = s.domains.UpdateExtras(r.Context(), domainID, d.TargetCountry, d.TargetLanguage, d.ExcludeDomains, server)
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
	memberRole, err := s.getProjectMemberRole(r.Context(), p.ID, user.Email)
	if err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
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

	// Проверяем наличие API ключа у владельца проекта
	encKey, err := s.svc.GetUserAPIKeyEncrypted(r.Context(), p.UserEmail)
	if err != nil || len(encKey) == 0 {
		writeError(w, http.StatusBadRequest, "API key not configured for project owner. Please configure API key in profile settings.")
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
		ID:        genID,
		DomainID:  domainID,
		Status:    "pending",
		Progress:  0,
		Logs:      json.RawMessage(`[]`),
		Artifacts: baseArtifacts,
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
	if _, err := s.tasks.Enqueue(r.Context(), task); err != nil {
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
	writeJSON(w, http.StatusOK, toGenerationDTOs(list))
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
	var (
		list []sqlstore.Generation
		err  error
	)
	if strings.EqualFold(user.Role, "admin") {
		list, err = s.generations.ListRecentAll(r.Context(), limit)
	} else {
		list, err = s.generations.ListRecentByUser(r.Context(), user.Email, limit)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list generations")
		return
	}
	writeJSON(w, http.StatusOK, toGenerationDTOs(list))
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
			if _, err := s.tasks.Enqueue(r.Context(), task); err != nil {
				if s.logger != nil {
					s.logger.Warnf("failed to enqueue resumed generation: %v", err)
				}
				// Не возвращаем ошибку, так как статус уже обновлен
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "pending", "message": "generation resumed"})

	case "cancel":
		// Отмена возможна из PROCESSING, PAUSE_REQUESTED или PAUSED
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
	DefaultServerID *string         `json:"default_server_id,omitempty"`
	GlobalBlacklist json.RawMessage `json:"global_blacklist,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	OwnerHasApiKey  bool            `json:"ownerHasApiKey,omitempty"` // Есть ли API ключ у владельца проекта
}

type domainDTO struct {
	ID               string     `json:"id"`
	ProjectID        string     `json:"project_id"`
	ServerID         *string    `json:"server_id,omitempty"`
	URL              string     `json:"url"`
	MainKeyword      string     `json:"main_keyword,omitempty"`
	TargetCountry    string     `json:"target_country,omitempty"`
	TargetLanguage   string     `json:"target_language,omitempty"`
	ExcludeDomains   *string    `json:"exclude_domains,omitempty"`
	Status           string     `json:"status"`
	LastGenerationID *string    `json:"last_generation_id,omitempty"`
	PublishedAt      *time.Time `json:"published_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type generationDTO struct {
	ID         string     `json:"id"`
	DomainID   string     `json:"domain_id"`
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
		ID:               d.ID,
		ProjectID:        d.ProjectID,
		ServerID:         nullableStringPtr(d.ServerID),
		URL:              d.URL,
		MainKeyword:      d.MainKeyword,
		TargetCountry:    d.TargetCountry,
		TargetLanguage:   d.TargetLanguage,
		ExcludeDomains:   nullableStringPtr(d.ExcludeDomains),
		Status:           d.Status,
		LastGenerationID: nullableStringPtr(d.LastGenerationID),
		PublishedAt:      nullableTimePtr(d.PublishedAt),
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}

func toDomainDTOs(list []sqlstore.Domain) []domainDTO {
	out := make([]domainDTO, 0, len(list))
	for _, d := range list {
		out = append(out, toDomainDTO(d))
	}
	return out
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
