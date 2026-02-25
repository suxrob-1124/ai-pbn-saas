package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
)

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
		enqueueErr := s.enqueueIndexCheckRunNow(r.Context(), domain, check.ID)
		enqueued := enqueueErr == nil
		dto.RunNowEnqueued = &enqueued
		if enqueueErr != nil {
			msg := enqueueErr.Error()
			dto.RunNowError = &msg
		}
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
	return s.indexChecks.UpsertManualByDomainAndDate(ctx, domainID, today, now)
}

func (s *Server) enqueueIndexCheckRunNow(ctx context.Context, domain sqlstore.Domain, checkID string) error {
	if s.tasks == nil {
		return errors.New("task queue is not configured")
	}
	task := tasks.NewIndexCheckTask(checkID)
	queueName := tasks.QueueForProject(domain.ProjectID, s.cfg.GenQueueShards)
	opts := make([]asynq.Option, 0, 1)
	if strings.TrimSpace(queueName) != "" {
		opts = append(opts, asynq.Queue(queueName))
	}
	_, err := s.tasks.Enqueue(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("enqueue index check run-now: %w", err)
	}
	return nil
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
		upsertFailed := 0
		enqueued := 0
		enqueueFailed := 0
		for _, d := range domains {
			if strings.TrimSpace(d.URL) == "" {
				skipped++
				continue
			}
			if !isDomainPublished(d) {
				skipped++
				continue
			}
			check, wasCreated, err := s.upsertManualIndexCheck(r.Context(), d.ID, now)
			if err != nil {
				upsertFailed++
				if s.logger != nil {
					s.logger.Warnw("project index run: upsert manual check failed", "project_id", projectID, "domain_id", d.ID, "err", err)
				}
				continue
			}
			if err := s.enqueueIndexCheckRunNow(r.Context(), d, check.ID); err != nil {
				enqueueFailed++
			} else {
				enqueued++
			}
			if wasCreated {
				created++
			} else {
				updated++
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"created":        created,
			"updated":        updated,
			"skipped":        skipped,
			"upsert_failed":  upsertFailed,
			"enqueued":       enqueued,
			"enqueue_failed": enqueueFailed,
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
	var body struct {
		DomainID string `json:"domain_id"`
	}
	if !decodeJSONBody(w, r, &body) {
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
	enqueueErr := s.enqueueIndexCheckRunNow(r.Context(), domain, check.ID)
	enqueued := enqueueErr == nil
	dto.RunNowEnqueued = &enqueued
	if enqueueErr != nil {
		msg := enqueueErr.Error()
		dto.RunNowError = &msg
	}
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
