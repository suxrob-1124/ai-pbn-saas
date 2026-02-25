package httpserver

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

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
	if domains, err := s.domains.ListByProject(r.Context(), projectID); err == nil {
		for _, d := range domains {
			domainMap[d.ID] = d.URL
		}
	}
	resp := make([]queueItemDTO, 0, len(items))
	for _, item := range items {
		if item.Status != "pending" && item.Status != "queued" {
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

func (s *Server) handleProjectQueueHistory(w http.ResponseWriter, r *http.Request, projectID string) {
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
	page := 1
	if v := strings.TrimSpace(r.URL.Query().Get("page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	offset := (page - 1) * limit
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	var statusFilter *string
	if rawStatus := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))); rawStatus != "" && rawStatus != "all" {
		if rawStatus != "completed" && rawStatus != "failed" {
			writeError(w, http.StatusBadRequest, "invalid history status")
			return
		}
		statusFilter = &rawStatus
	}

	var dateFrom *time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("date_from")); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid date_from")
			return
		}
		utc := parsed.UTC()
		dateFrom = &utc
	}
	var dateTo *time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("date_to")); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid date_to")
			return
		}
		utcEnd := parsed.UTC().Add(24*time.Hour - time.Nanosecond)
		dateTo = &utcEnd
	}

	items, err := s.genQueue.ListHistoryByProjectPage(r.Context(), projectID, limit, offset, search, statusFilter, dateFrom, dateTo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load queue history")
		return
	}
	domainMap := map[string]string{}
	if domains, err := s.domains.ListByProject(r.Context(), projectID); err == nil {
		for _, d := range domains {
			domainMap[d.ID] = d.URL
		}
	}
	resp := make([]queueItemDTO, 0, len(items))
	for _, item := range items {
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
