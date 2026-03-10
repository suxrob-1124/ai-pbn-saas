package httpserver

import (
	"context"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

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
	if isAdmin(user.Role) {
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
			// only count full generation runs; short durations are checkpoint resumes (e.g. publish-only)
			if d >= 3*time.Minute {
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
