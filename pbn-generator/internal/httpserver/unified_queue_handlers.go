package httpserver

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

type unifiedQueueItemDTO struct {
	ID           string `json:"id"`
	Type         string `json:"type"` // "generation" | "link"
	DomainID     string `json:"domain_id"`
	DomainURL    string `json:"domain_url,omitempty"`
	ProjectID    string `json:"project_id,omitempty"`
	Status       string `json:"status"`        // normalized: pending|processing|completed|failed
	StatusDetail string `json:"status_detail"` // original status
	ErrorMessage string `json:"error_message,omitempty"`
	ScheduledFor string `json:"scheduled_for"`
	CreatedAt    string `json:"created_at"`
	AnchorText   string `json:"anchor_text,omitempty"`
	TargetURL    string `json:"target_url,omitempty"`
}

func normalizeGenQueueStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending", "queued":
		return "pending"
	case "processing", "pause_requested", "cancelling":
		return "processing"
	case "completed":
		return "completed"
	case "failed", "error":
		return "failed"
	default:
		return "pending"
	}
}

func normalizeLinkTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending":
		return "pending"
	case "searching", "removing":
		return "processing"
	case "inserted", "generated", "removed":
		return "completed"
	case "failed":
		return "failed"
	default:
		return "pending"
	}
}

func genQueueItemToUnified(item sqlstore.QueueItem, domainURL, projectID string) unifiedQueueItemDTO {
	dto := unifiedQueueItemDTO{
		ID:           item.ID,
		Type:         "generation",
		DomainID:     item.DomainID,
		DomainURL:    domainURL,
		ProjectID:    projectID,
		Status:       normalizeGenQueueStatus(item.Status),
		StatusDetail: item.Status,
		ScheduledFor: item.ScheduledFor.UTC().Format(time.RFC3339),
		CreatedAt:    item.CreatedAt.UTC().Format(time.RFC3339),
	}
	if item.ErrorMessage.Valid && item.ErrorMessage.String != "" {
		dto.ErrorMessage = item.ErrorMessage.String
	}
	return dto
}

func linkTaskToUnified(task sqlstore.LinkTask, domainURL, projectID string) unifiedQueueItemDTO {
	dto := unifiedQueueItemDTO{
		ID:           task.ID,
		Type:         "link",
		DomainID:     task.DomainID,
		DomainURL:    domainURL,
		ProjectID:    projectID,
		Status:       normalizeLinkTaskStatus(task.Status),
		StatusDetail: task.Status,
		AnchorText:   task.AnchorText,
		TargetURL:    task.TargetURL,
		ScheduledFor: task.ScheduledFor.UTC().Format(time.RFC3339),
		CreatedAt:    task.CreatedAt.UTC().Format(time.RFC3339),
	}
	if task.ErrorMessage.Valid && task.ErrorMessage.String != "" {
		dto.ErrorMessage = task.ErrorMessage.String
	}
	return dto
}

func filterUnifiedByStatus(items []unifiedQueueItemDTO, statusFilter string) []unifiedQueueItemDTO {
	if statusFilter == "" || statusFilter == "all" {
		return items
	}
	out := make([]unifiedQueueItemDTO, 0, len(items))
	for _, item := range items {
		if item.Status == statusFilter {
			out = append(out, item)
		}
	}
	return out
}

func filterUnifiedByType(items []unifiedQueueItemDTO, typeFilter string) []unifiedQueueItemDTO {
	if typeFilter == "" || typeFilter == "all" {
		return items
	}
	out := make([]unifiedQueueItemDTO, 0, len(items))
	for _, item := range items {
		if item.Type == typeFilter {
			out = append(out, item)
		}
	}
	return out
}

func sortUnifiedByScheduledForDesc(items []unifiedQueueItemDTO) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].ScheduledFor > items[j].ScheduledFor
	})
}

func (s *Server) handleProjectUnifiedQueue(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
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

	typeFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	statusFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	limit := parseLimitParam(r, 50, 200)
	page := parsePageParam(r, 1)
	offset := (page - 1) * limit

	// Build domain URL map.
	domainMap := map[string]string{}
	if domains, err := s.domains.ListByProject(r.Context(), projectID); err == nil {
		for _, d := range domains {
			domainMap[d.ID] = d.URL
		}
	}

	var items []unifiedQueueItemDTO

	// Fetch generation queue items.
	if typeFilter == "" || typeFilter == "all" || typeFilter == "generation" {
		if s.genQueue != nil {
			genItems, err := s.genQueue.ListByProjectPage(r.Context(), projectID, 200, 0, "")
			if err == nil {
				for _, item := range genItems {
					items = append(items, genQueueItemToUnified(item, domainMap[item.DomainID], projectID))
				}
			}
		}
	}

	// Fetch link tasks.
	if typeFilter == "" || typeFilter == "all" || typeFilter == "link" {
		if s.linkTasks != nil {
			linkItems, err := s.linkTasks.ListByProject(r.Context(), projectID, sqlstore.LinkTaskFilters{})
			if err == nil {
				for _, task := range linkItems {
					items = append(items, linkTaskToUnified(task, domainMap[task.DomainID], projectID))
				}
			}
		}
	}

	items = filterUnifiedByType(items, typeFilter)
	items = filterUnifiedByStatus(items, statusFilter)
	sortUnifiedByScheduledForDesc(items)

	// Apply pagination after merge+sort.
	if offset >= len(items) {
		items = []unifiedQueueItemDTO{}
	} else {
		end := offset + limit
		if end > len(items) {
			end = len(items)
		}
		items = items[offset:end]
	}

	if items == nil {
		items = []unifiedQueueItemDTO{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleUnifiedQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	typeFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	statusFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	limit := parseLimitParam(r, 50, 200)

	var items []unifiedQueueItemDTO

	if isAdmin(user.Role) {
		// Admin: fetch all projects and collect items across all.
		if s.projects != nil {
			projects, err := s.projects.ListAll(r.Context())
			if err == nil {
				for _, p := range projects {
					domainMap := map[string]string{}
					if s.domains != nil {
						if domains, err := s.domains.ListByProject(r.Context(), p.ID); err == nil {
							for _, d := range domains {
								domainMap[d.ID] = d.URL
							}
						}
					}
					if (typeFilter == "" || typeFilter == "all" || typeFilter == "generation") && s.genQueue != nil {
						genItems, err := s.genQueue.ListByProjectPage(r.Context(), p.ID, 200, 0, "")
						if err == nil {
							for _, item := range genItems {
								items = append(items, genQueueItemToUnified(item, domainMap[item.DomainID], p.ID))
							}
						}
					}
					if (typeFilter == "" || typeFilter == "all" || typeFilter == "link") && s.linkTasks != nil {
						linkItems, err := s.linkTasks.ListByProject(r.Context(), p.ID, sqlstore.LinkTaskFilters{})
						if err == nil {
							for _, task := range linkItems {
								items = append(items, linkTaskToUnified(task, domainMap[task.DomainID], p.ID))
							}
						}
					}
				}
			}
		}
	} else {
		// Non-admin: iterate user's projects.
		if s.projects != nil {
			projects, err := s.projects.ListByUser(r.Context(), user.Email)
			if err == nil {
				for _, p := range projects {
					domainMap := map[string]string{}
					if s.domains != nil {
						if domains, err := s.domains.ListByProject(r.Context(), p.ID); err == nil {
							for _, d := range domains {
								domainMap[d.ID] = d.URL
							}
						}
					}
					if (typeFilter == "" || typeFilter == "all" || typeFilter == "generation") && s.genQueue != nil {
						genItems, err := s.genQueue.ListByProjectPage(r.Context(), p.ID, 200, 0, "")
						if err == nil {
							for _, item := range genItems {
								items = append(items, genQueueItemToUnified(item, domainMap[item.DomainID], p.ID))
							}
						}
					}
					if (typeFilter == "" || typeFilter == "all" || typeFilter == "link") && s.linkTasks != nil {
						linkItems, err := s.linkTasks.ListByProject(r.Context(), p.ID, sqlstore.LinkTaskFilters{})
						if err == nil {
							for _, task := range linkItems {
								items = append(items, linkTaskToUnified(task, domainMap[task.DomainID], p.ID))
							}
						}
					}
				}
			}
		}
	}

	items = filterUnifiedByType(items, typeFilter)
	items = filterUnifiedByStatus(items, statusFilter)
	sortUnifiedByScheduledForDesc(items)

	if len(items) > 200 {
		items = items[:200]
	}
	if len(items) > limit {
		items = items[:limit]
	}

	if items == nil {
		items = []unifiedQueueItemDTO{}
	}
	writeJSON(w, http.StatusOK, items)
}
