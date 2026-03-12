package httpserver

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"obzornik-pbn-generator/internal/scheduler"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

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
		var body struct {
			Name        string          `json:"name"`
			Description *string         `json:"description"`
			Strategy    string          `json:"strategy"`
			Config      json.RawMessage `json:"config"`
			IsActive    *bool           `json:"isActive"`
			Timezone    *string         `json:"timezone"`
		}
		if !decodeJSONBody(w, r, &body) {
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
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		var body struct {
			Name        *string          `json:"name"`
			Description *string          `json:"description"`
			Strategy    *string          `json:"strategy"`
			Config      *json.RawMessage `json:"config"`
			IsActive    *bool            `json:"isActive"`
			Timezone    *string          `json:"timezone"`
		}
		if !decodeJSONBody(w, r, &body) {
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
		if err := requireApprovedUser(user); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		var body struct {
			Name     string          `json:"name"`
			Config   json.RawMessage `json:"config"`
			IsActive *bool           `json:"isActive"`
			Timezone *string         `json:"timezone"`
		}
		if !decodeJSONBody(w, r, &body) {
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
			emptyLogs := []string{}
			updates := sqlstore.LinkTaskUpdates{
				AnchorText:       &anchor,
				TargetURL:        &target,
				Status:           &status,
				Attempts:         &attempts,
				CreatedAt:        &now,
				ScheduledFor:     &now,
				FoundLocation:    &nullStr,
				GeneratedContent: &nullStr,
				ErrorMessage:     &nullStr,
				CompletedAt:      &nullTime,
				LogLines:         &emptyLogs,
			}
			if err := s.linkTasks.Update(r.Context(), task.ID, updates); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update link task")
				return
			}
			if err := s.domains.UpdateLinkStatus(r.Context(), d.ID, "pending"); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update domain link status")
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
		if err := s.domains.UpdateLinkStatus(r.Context(), d.ID, "pending"); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update domain link status")
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

// scheduleRunLogDTO is the API representation of a ScheduleRunLog record.
type scheduleRunLogDTO struct {
	ID            string              `json:"id"`
	ScheduleID    string              `json:"schedule_id"`
	ScheduleType  string              `json:"schedule_type"`
	ProjectID     string              `json:"project_id"`
	RunAt         string              `json:"run_at"`
	TotalDomains  int                 `json:"total_domains"`
	EligibleCount int                 `json:"eligible_count"`
	EnqueuedCount int                 `json:"enqueued_count"`
	SkippedCount  int                 `json:"skipped_count"`
	SkipDetails   []sqlstore.SkipDetail `json:"skip_details"`
	ErrorMessage  *string             `json:"error_message,omitempty"`
	NextRunAt     *string             `json:"next_run_at,omitempty"`
	CreatedAt     string              `json:"created_at"`
}

func toScheduleRunLogDTO(log sqlstore.ScheduleRunLog) scheduleRunLogDTO {
	dto := scheduleRunLogDTO{
		ID:            log.ID,
		ScheduleID:    log.ScheduleID,
		ScheduleType:  log.ScheduleType,
		ProjectID:     log.ProjectID,
		RunAt:         log.RunAt.UTC().Format(time.RFC3339),
		TotalDomains:  log.TotalDomains,
		EligibleCount: log.EligibleCount,
		EnqueuedCount: log.EnqueuedCount,
		SkippedCount:  log.SkippedCount,
		SkipDetails:   log.SkipDetails,
		CreatedAt:     log.CreatedAt.UTC().Format(time.RFC3339),
	}
	if log.ErrorMessage.Valid && log.ErrorMessage.String != "" {
		dto.ErrorMessage = &log.ErrorMessage.String
	}
	if log.NextRunAt.Valid {
		s := log.NextRunAt.Time.UTC().Format(time.RFC3339)
		dto.NextRunAt = &s
	}
	if dto.SkipDetails == nil {
		dto.SkipDetails = []sqlstore.SkipDetail{}
	}
	return dto
}

func (s *Server) handleProjectScheduleRuns(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.scheduleRunLogs == nil {
		writeError(w, http.StatusInternalServerError, "schedule run logs not configured")
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

	limit := parseLimitParam(r, 20, 100)
	page := parsePageParam(r, 1)
	offset := (page - 1) * limit

	scheduleID := strings.TrimSpace(r.URL.Query().Get("schedule_id"))

	var (
		logs []sqlstore.ScheduleRunLog
		err  error
	)
	if scheduleID != "" {
		logs, err = s.scheduleRunLogs.ListBySchedule(r.Context(), scheduleID, limit, offset)
	} else {
		logs, err = s.scheduleRunLogs.ListByProject(r.Context(), projectID, limit, offset)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load schedule run logs")
		return
	}

	resp := make([]scheduleRunLogDTO, 0, len(logs))
	for _, l := range logs {
		resp = append(resp, toScheduleRunLogDTO(l))
	}
	writeJSON(w, http.StatusOK, resp)
}

// schedulePreviewDomainDTO is a minimal domain reference used in preview responses.
type schedulePreviewDomainDTO struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// schedulePreviewSectionDTO describes one half (generation or link) of the preview.
type schedulePreviewSectionDTO struct {
	HasSchedule     bool                       `json:"has_schedule"`
	IsActive        bool                       `json:"is_active"`
	NextRunAt       *string                    `json:"next_run_at,omitempty"`
	EligibleDomains []schedulePreviewDomainDTO `json:"eligible_domains"`
	WouldSkip       []sqlstore.SkipDetail      `json:"would_skip"`
	WouldEnqueue    int                        `json:"would_enqueue"`
}

func (s *Server) handleProjectSchedulesPreview(w http.ResponseWriter, r *http.Request, projectID string) {
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

	domains, err := s.domains.ListByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list domains")
		return
	}

	now := time.Now().UTC()

	// --- Generation section ---
	genSection := schedulePreviewSectionDTO{
		EligibleDomains: []schedulePreviewDomainDTO{},
		WouldSkip:       []sqlstore.SkipDetail{},
	}
	if s.schedules != nil {
		schedList, err := s.schedules.List(r.Context(), projectID)
		if err == nil && len(schedList) > 0 {
			sched := schedList[0]
			genSection.HasSchedule = true
			genSection.IsActive = sched.IsActive
			if sched.NextRunAt.Valid {
				s := sched.NextRunAt.Time.UTC().Format(time.RFC3339)
				genSection.NextRunAt = &s
			}
			eligible, skipped := scheduler.FilterGenerationDomainsWithReasons(domains)
			for _, d := range eligible {
				genSection.EligibleDomains = append(genSection.EligibleDomains, schedulePreviewDomainDTO{ID: d.ID, URL: d.URL})
			}
			genSection.WouldSkip = skipped
			genSection.WouldEnqueue = len(eligible)
		}
	}

	// --- Link section ---
	linkSection := schedulePreviewSectionDTO{
		EligibleDomains: []schedulePreviewDomainDTO{},
		WouldSkip:       []sqlstore.SkipDetail{},
	}
	if s.linkSchedules != nil {
		linkSched, err := s.linkSchedules.GetByProject(r.Context(), projectID)
		if err == nil && linkSched != nil {
			linkSection.HasSchedule = true
			linkSection.IsActive = linkSched.IsActive
			scheduleRunAt := now
			if linkSched.NextRunAt.Valid && !linkSched.NextRunAt.Time.IsZero() {
				scheduleRunAt = linkSched.NextRunAt.Time.UTC()
			}
			if linkSched.NextRunAt.Valid {
				s := linkSched.NextRunAt.Time.UTC().Format(time.RFC3339)
				linkSection.NextRunAt = &s
			}
			eligible, skipped := scheduler.FilterLinkDomainsWithReasons(domains, now, scheduleRunAt)
			for _, d := range eligible {
				linkSection.EligibleDomains = append(linkSection.EligibleDomains, schedulePreviewDomainDTO{ID: d.ID, URL: d.URL})
			}
			linkSection.WouldSkip = skipped
			linkSection.WouldEnqueue = len(eligible)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"generation": genSection,
		"link":       linkSection,
	})
}
