package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
)

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
	if isAdmin(user.Role) {
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
		domainCache := map[string]*sqlstore.Domain{}
		for i := range list {
			g := list[i]
			domain, cached := domainCache[g.DomainID]
			if !cached {
				d, err := s.domains.Get(r.Context(), g.DomainID)
				if err == nil {
					dCopy := d
					domain = &dCopy
					domainCache[g.DomainID] = domain
					if strings.TrimSpace(dCopy.URL) != "" {
						domainMap[g.DomainID] = dCopy.URL
					}
				} else {
					domainCache[g.DomainID] = nil
				}
			}
			updated, _, recoverErr := s.recoverStaleGenerationTransition(r.Context(), g, domain)
			if recoverErr != nil && s.logger != nil {
				s.logger.Warnw("failed to recover stale generation transition",
					"generation_id", g.ID,
					"status", g.Status,
					"error", recoverErr,
				)
				continue
			}
			list[i] = updated
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

const defaultGenerationTransitionStaleAfter = 2 * time.Minute

func (s *Server) recoverStaleGenerationTransition(
	ctx context.Context,
	gen sqlstore.Generation,
	domain *sqlstore.Domain,
) (sqlstore.Generation, bool, error) {
	status := strings.TrimSpace(gen.Status)
	if status != "pause_requested" && status != "cancelling" {
		return gen, false, nil
	}
	staleAfter := s.cfg.GenerationTransitionStaleAfter
	if staleAfter <= 0 {
		staleAfter = defaultGenerationTransitionStaleAfter
	}
	if gen.UpdatedAt.IsZero() || time.Since(gen.UpdatedAt) < staleAfter {
		return gen, false, nil
	}
	if s.domains == nil {
		return gen, false, nil
	}
	if domain == nil {
		d, err := s.domains.Get(ctx, gen.DomainID)
		if err != nil {
			return gen, false, nil
		}
		domain = &d
	}

	nextStatus := "paused"
	finishedAt := nullableTimePtr(gen.FinishedAt)
	if status == "cancelling" {
		nextStatus = "cancelled"
		if finishedAt == nil {
			now := time.Now().UTC()
			finishedAt = &now
		}
	}

	if err := s.generations.UpdateFull(
		ctx,
		gen.ID,
		nextStatus,
		gen.Progress,
		nil,
		gen.Logs,
		gen.Artifacts,
		nullableTimePtr(gen.StartedAt),
		finishedAt,
		nil,
	); err != nil {
		return gen, false, err
	}
	if status == "cancelling" {
		if err := s.generations.ClearCheckpoint(ctx, gen.ID); err != nil && s.logger != nil {
			s.logger.Warnw("failed to clear generation checkpoint on stale cancel recovery",
				"generation_id", gen.ID,
				"error", err,
			)
		}
	}
	if domain != nil {
		_ = s.domains.UpdateStatus(ctx, gen.DomainID, stableDomainStatusFromDomain(*domain))
	}

	updated, err := s.generations.Get(ctx, gen.ID)
	if err == nil {
		return updated, true, nil
	}
	gen.Status = nextStatus
	gen.UpdatedAt = time.Now().UTC()
	if nextStatus == "cancelled" && !gen.FinishedAt.Valid {
		gen.FinishedAt = sql.NullTime{Time: gen.UpdatedAt, Valid: true}
	}
	return gen, true, nil
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
		updated, _, recoverErr := s.recoverStaleGenerationTransition(r.Context(), gen, &d)
		if recoverErr != nil && s.logger != nil {
			s.logger.Warnw("failed to recover stale generation transition",
				"generation_id", gen.ID,
				"status", gen.Status,
				"error", recoverErr,
			)
		} else {
			gen = updated
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
		_ = s.domains.RecalculateGenerationPointers(r.Context(), gen.DomainID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleGenerationAction(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Action string `json:"action"`
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
		if gen.Status == "pending" {
			if err := s.generations.UpdateStatus(r.Context(), id, "paused", gen.Progress, nil); err != nil {
				writeError(w, http.StatusInternalServerError, "could not pause generation")
				return
			}
			_ = s.domains.UpdateStatus(r.Context(), gen.DomainID, stableDomainStatusFromDomain(d))
			writeJSON(w, http.StatusOK, map[string]string{"status": "paused", "message": "generation paused"})
			return
		}
		if gen.Status != "processing" {
			writeError(w, http.StatusBadRequest, "can only pause processing generation")
			return
		}
		if err := s.generations.UpdateStatus(r.Context(), id, "pause_requested", gen.Progress, nil); err != nil {
			writeError(w, http.StatusInternalServerError, "could not pause generation")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "pause_requested", "message": "pause requested, will pause after current step completes"})

	case "resume":
		if gen.Status != "paused" {
			writeError(w, http.StatusBadRequest, "can only resume paused generation")
			return
		}
		if s.tasks == nil {
			writeError(w, http.StatusServiceUnavailable, "queue unavailable")
			return
		}
		if err := s.generations.UpdateStatus(r.Context(), id, "pending", gen.Progress, nil); err != nil {
			writeError(w, http.StatusInternalServerError, "could not resume generation")
			return
		}
		_ = s.domains.UpdateStatus(r.Context(), gen.DomainID, "processing")
		task := tasks.NewGenerateTask(id, gen.DomainID, "", gen.GenerationType)
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
			_ = s.generations.UpdateStatus(r.Context(), id, "paused", gen.Progress, nil)
			_ = s.domains.UpdateStatus(r.Context(), gen.DomainID, stableDomainStatusFromDomain(d))
			writeError(w, http.StatusServiceUnavailable, "could not enqueue resumed generation")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "pending", "message": "generation resumed"})

	case "cancel":
		if gen.Status == "pending" {
			if err := s.generations.UpdateStatus(r.Context(), id, "cancelled", 0, nil); err != nil {
				writeError(w, http.StatusInternalServerError, "could not cancel generation")
				return
			}
			_ = s.domains.UpdateStatus(r.Context(), gen.DomainID, stableDomainStatusFromDomain(d))
			writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled", "message": "generation cancelled"})
			return
		}
		if gen.Status != "processing" && gen.Status != "pause_requested" && gen.Status != "paused" {
			writeError(w, http.StatusBadRequest, "can only cancel processing, pause_requested or paused generation")
			return
		}

		if gen.Status == "paused" {
			if err := s.generations.ClearCheckpoint(r.Context(), id); err != nil {
				writeError(w, http.StatusInternalServerError, "could not clear checkpoint")
				return
			}
			if err := s.generations.UpdateStatus(r.Context(), id, "cancelled", 0, nil); err != nil {
				writeError(w, http.StatusInternalServerError, "could not cancel generation")
				return
			}
			_ = s.domains.UpdateStatus(r.Context(), gen.DomainID, stableDomainStatusFromDomain(d))
			writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled", "message": "generation cancelled"})
			return
		}
		if err := s.generations.UpdateStatus(r.Context(), id, "cancelling", gen.Progress, nil); err != nil {
			writeError(w, http.StatusInternalServerError, "could not cancel generation")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "cancelling", "message": "cancellation requested"})
		return

	default:
		writeError(w, http.StatusBadRequest, "invalid action: must be pause, resume, or cancel")
	}
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
		if isAdmin(user.Role) {
			list, err = s.linkTasks.ListAll(r.Context(), filters)
		} else {
			list, err = s.linkTasks.ListByUser(r.Context(), user.Email, filters)
		}
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
		attempts := 0
		nullStr := sql.NullString{}
		nullTime := sql.NullTime{}
		emptyLogs := []string{}
		updates := sqlstore.LinkTaskUpdates{
			Status:           &status,
			Attempts:         &attempts,
			CreatedAt:        &now,
			ScheduledFor:     &now,
			FoundLocation:    &nullStr,
			GeneratedContent: &nullStr,
			ErrorMessage:     &sql.NullString{},
			CompletedAt:      &nullTime,
			LogLines:         &emptyLogs,
		}
		if err := s.linkTasks.Update(r.Context(), taskID, updates); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to retry link task")
			return
		}
		if err := s.domains.UpdateLinkStatus(r.Context(), domain.ID, "pending"); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update domain link status")
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
