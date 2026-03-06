package httpserver

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	return s.withRequestID(s.withLogging(s.withMetrics(s.withCORS(mux))))
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
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
	mux.Handle("/api/admin/index-checker/control", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexCheckerControl))))
	mux.Handle("/api/admin/index-checks", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexChecks))))
	mux.Handle("/api/admin/index-checks/run", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexChecksRun))))
	mux.Handle("/api/admin/index-checks/stats", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexChecksStats))))
	mux.Handle("/api/admin/index-checks/calendar", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexChecksCalendar))))
	mux.Handle("/api/admin/index-checks/", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexCheckRoute))))
	mux.Handle("/api/admin/index-checks/failed", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminIndexChecksFailed))))
	mux.Handle("/api/admin/llm-usage/events", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminLLMUsageEvents))))
	mux.Handle("/api/admin/llm-usage/stats", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminLLMUsageStats))))
	mux.Handle("/api/admin/llm-pricing", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminLLMPricing))))
	mux.Handle("/api/admin/llm-pricing/", s.withAuth(s.requireAdmin(http.HandlerFunc(s.handleAdminLLMPricingByModel))))
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
}
