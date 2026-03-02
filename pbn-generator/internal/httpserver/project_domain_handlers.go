package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
)

// --- Projects / Domains / Generations ---

var errDomainGeneratePreflightServerID = errors.New("domain generate preflight: missing server_id")

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
		if action == "history" {
			s.handleProjectQueueHistory(w, r, projectID)
			return
		}
		s.handleProjectQueue(w, r, projectID)
		return
	}
	if len(parts) > 1 && parts[1] == "index-checks" {
		s.handleProjectIndexChecks(w, r, projectID, parts[2:])
		return
	}
	if len(parts) > 1 && parts[1] == "llm-usage" {
		action := ""
		if len(parts) > 2 {
			action = parts[2]
		}
		if action == "stats" {
			s.handleProjectLLMUsageStats(w, r, projectID)
			return
		}
		s.handleProjectLLMUsageEvents(w, r, projectID)
		return
	}
	if len(parts) > 1 && parts[1] == "prompts" {
		stage := ""
		if len(parts) > 2 {
			stage, _ = url.PathUnescape(parts[2])
		}
		s.handleProjectPrompts(w, r, projectID, stage)
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
	myRole := s.resolveProjectRole(r.Context(), project, user)
	domains, err := s.domains.ListByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list domains")
		return
	}
	activeLinkTasksByDomain := map[string]sqlstore.LinkTask{}
	if s.linkTasks != nil && len(domains) > 0 {
		domainIDs := make([]string, 0, len(domains))
		for _, d := range domains {
			domainIDs = append(domainIDs, d.ID)
		}
		activeLinkTasksByDomain, err = s.linkTasks.ListActiveByDomainIDs(r.Context(), domainIDs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not resolve effective link status")
			return
		}
	}
	domainDTOs := make([]domainDTO, 0, len(domains))
	for _, d := range domains {
		dto := toDomainDTO(d)
		if activeTask, ok := activeLinkTasksByDomain[d.ID]; ok {
			taskCopy := activeTask
			applyEffectiveLinkStatus(&dto, &taskCopy)
		} else {
			applyEffectiveLinkStatus(&dto, nil)
		}
		domainDTOs = append(domainDTOs, dto)
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
		Domains: domainDTOs,
		Members: memberDTOs,
		MyRole:  myRole,
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
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	myRole := s.resolveProjectRole(r.Context(), project, user)

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
	genDTOs := make([]generationDTO, 0, minInt(len(gens), genLimit))
	for idx, g := range gens {
		if idx >= genLimit {
			break
		}
		genDTOs = append(genDTOs, toGenerationLightDTO(g))
	}

	var latestAttempt *generationDTO
	if len(gens) > 0 {
		dto := toGenerationLightDTO(gens[0])
		latestAttempt = &dto
	}

	var latestSuccess *generationDTO
	if domain.LastSuccessGenID.Valid && strings.TrimSpace(domain.LastSuccessGenID.String) != "" {
		targetID := strings.TrimSpace(domain.LastSuccessGenID.String)
		for _, g := range gens {
			if g.ID == targetID {
				dto := toGenerationLightDTO(g)
				latestSuccess = &dto
				break
			}
		}
		if latestSuccess == nil {
			if g, err := s.generations.Get(r.Context(), targetID); err == nil {
				dto := toGenerationLightDTO(g)
				latestSuccess = &dto
			}
		}
	}
	if latestSuccess == nil {
		for _, g := range gens {
			if g.Status == "success" {
				dto := toGenerationLightDTO(g)
				latestSuccess = &dto
				break
			}
		}
	}

	linkDTOs := make([]linkTaskDTO, 0)
	if s.linkTasks != nil {
		tasks, err := s.linkTasks.ListByDomain(r.Context(), domainID, sqlstore.LinkTaskFilters{Limit: linkLimit, SortDesc: true})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list link tasks")
			return
		}
		for _, t := range tasks {
			linkDTOs = append(linkDTOs, toLinkTaskDTO(t))
		}
	}
	domainView := toDomainDTO(domain)
	if s.linkTasks != nil {
		activeMap, err := s.linkTasks.ListActiveByDomainIDs(r.Context(), []string{domainID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not resolve effective link status")
			return
		}
		if activeTask, ok := activeMap[domainID]; ok {
			taskCopy := activeTask
			applyEffectiveLinkStatus(&domainView, &taskCopy)
		} else {
			applyEffectiveLinkStatus(&domainView, nil)
		}
	} else {
		applyEffectiveLinkStatus(&domainView, nil)
	}

	resp := domainSummaryDTO{
		Domain:        domainView,
		ProjectName:   project.Name,
		Generations:   genDTOs,
		LatestAttempt: latestAttempt,
		LatestSuccess: latestSuccess,
		LinkTasks:     linkDTOs,
		MyRole:        myRole,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDomainDeployments(w http.ResponseWriter, r *http.Request, domainID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, _, err := s.authorizeDomain(r.Context(), domainID); err != nil {
		respondAuthzError(w, err, "domain not found")
		return
	}
	if s.deployments == nil {
		writeJSON(w, http.StatusOK, []deploymentAttemptDTO{})
		return
	}
	limit := 20
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	list, err := s.deployments.ListByDomain(r.Context(), domainID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list deployments")
		return
	}
	resp := make([]deploymentAttemptDTO, 0, len(list))
	for _, item := range list {
		resp = append(resp, toDeploymentAttemptDTO(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleProjectPrompts(w http.ResponseWriter, r *http.Request, projectID, stage string) {
	if s.promptOverrides == nil {
		writeError(w, http.StatusInternalServerError, "prompt overrides not configured")
		return
	}
	project, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	requireEditor := func() bool {
		memberRole := ""
		if !strings.EqualFold(user.Role, "admin") {
			role, err := s.getProjectMemberRole(r.Context(), project.ID, user.Email)
			if err != nil {
				writeError(w, http.StatusForbidden, "access denied")
				return false
			}
			memberRole = role
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return false
		}
		return true
	}

	switch r.Method {
	case http.MethodGet:
		overrides, err := s.promptOverrides.ListByScope(r.Context(), sqlstore.PromptScopeProject, projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list project prompt overrides")
			return
		}
		resolved := make([]resolvedPromptDTO, 0, len(sqlstore.GenerationPromptStages))
		for _, st := range sqlstore.GenerationPromptStages {
			item, err := s.promptOverrides.ResolveForDomainStage(r.Context(), "", projectID, st)
			if err == nil {
				resolved = append(resolved, toResolvedPromptDTO(item))
			}
		}
		resp := domainPromptsDTO{
			Overrides: toPromptOverrideDTOs(overrides),
			Resolved:  resolved,
		}
		writeJSON(w, http.StatusOK, resp)
	case http.MethodPut:
		if !requireEditor() {
			return
		}
		stage = strings.TrimSpace(stage)
		if !isGenerationPromptStage(stage) {
			writeError(w, http.StatusBadRequest, "invalid stage")
			return
		}
		if !ensureJSON(w, r) {
			return
		}
		defer r.Body.Close()
		var body struct {
			Body            string  `json:"body"`
			Model           *string `json:"model"`
			BasedOnPromptID *string `json:"based_on_prompt_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		pBody := strings.TrimSpace(body.Body)
		if pBody == "" {
			writeError(w, http.StatusBadRequest, "body is required")
			return
		}
		item := sqlstore.PromptOverride{
			ID:        uuid.NewString(),
			ScopeType: sqlstore.PromptScopeProject,
			ScopeID:   projectID,
			Stage:     stage,
			Body:      pBody,
			UpdatedBy: user.Email,
		}
		if body.Model != nil {
			model := strings.TrimSpace(*body.Model)
			if model != "" {
				item.Model = sqlstore.NullableString(model)
			}
		}
		if body.BasedOnPromptID != nil {
			ref := strings.TrimSpace(*body.BasedOnPromptID)
			if ref != "" {
				item.BasedOnPrompt = sqlstore.NullableString(ref)
			}
		}
		if err := s.promptOverrides.Upsert(r.Context(), item); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save override")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if !requireEditor() {
			return
		}
		stage = strings.TrimSpace(stage)
		if !isGenerationPromptStage(stage) {
			writeError(w, http.StatusBadRequest, "invalid stage")
			return
		}
		if err := s.promptOverrides.Delete(r.Context(), sqlstore.PromptScopeProject, projectID, stage); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete override")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleDomainPrompts(w http.ResponseWriter, r *http.Request, domainID, stage string) {
	if s.promptOverrides == nil {
		writeError(w, http.StatusInternalServerError, "prompt overrides not configured")
		return
	}
	_, project, err := s.authorizeDomain(r.Context(), domainID)
	if err != nil {
		respondAuthzError(w, err, "domain not found")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	requireEditor := func() bool {
		memberRole := ""
		if !strings.EqualFold(user.Role, "admin") {
			role, err := s.getProjectMemberRole(r.Context(), project.ID, user.Email)
			if err != nil {
				writeError(w, http.StatusForbidden, "access denied")
				return false
			}
			memberRole = role
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return false
		}
		return true
	}

	switch r.Method {
	case http.MethodGet:
		overrides, err := s.promptOverrides.ListByScope(r.Context(), sqlstore.PromptScopeDomain, domainID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list domain prompt overrides")
			return
		}
		resolved := make([]resolvedPromptDTO, 0, len(sqlstore.GenerationPromptStages))
		for _, st := range sqlstore.GenerationPromptStages {
			item, err := s.promptOverrides.ResolveForDomainStage(r.Context(), domainID, project.ID, st)
			if err == nil {
				resolved = append(resolved, toResolvedPromptDTO(item))
			}
		}
		resp := domainPromptsDTO{
			Overrides: toPromptOverrideDTOs(overrides),
			Resolved:  resolved,
		}
		writeJSON(w, http.StatusOK, resp)
	case http.MethodPut:
		if !requireEditor() {
			return
		}
		stage = strings.TrimSpace(stage)
		if !isGenerationPromptStage(stage) {
			writeError(w, http.StatusBadRequest, "invalid stage")
			return
		}
		if !ensureJSON(w, r) {
			return
		}
		defer r.Body.Close()
		var body struct {
			Body            string  `json:"body"`
			Model           *string `json:"model"`
			BasedOnPromptID *string `json:"based_on_prompt_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		pBody := strings.TrimSpace(body.Body)
		if pBody == "" {
			writeError(w, http.StatusBadRequest, "body is required")
			return
		}
		item := sqlstore.PromptOverride{
			ID:        uuid.NewString(),
			ScopeType: sqlstore.PromptScopeDomain,
			ScopeID:   domainID,
			Stage:     stage,
			Body:      pBody,
			UpdatedBy: user.Email,
		}
		if body.Model != nil {
			model := strings.TrimSpace(*body.Model)
			if model != "" {
				item.Model = sqlstore.NullableString(model)
			}
		}
		if body.BasedOnPromptID != nil {
			ref := strings.TrimSpace(*body.BasedOnPromptID)
			if ref != "" {
				item.BasedOnPrompt = sqlstore.NullableString(ref)
			}
		}
		if err := s.promptOverrides.Upsert(r.Context(), item); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save override")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if !requireEditor() {
			return
		}
		stage = strings.TrimSpace(stage)
		if !isGenerationPromptStage(stage) {
			writeError(w, http.StatusBadRequest, "invalid stage")
			return
		}
		if err := s.promptOverrides.Delete(r.Context(), sqlstore.PromptScopeDomain, domainID, stage); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete override")
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
			Items []domainImportItem `json:"items"`
			Text  string             `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if len(body.Items) == 0 && strings.TrimSpace(body.Text) == "" {
			writeError(w, http.StatusBadRequest, "no domains provided")
			return
		}
		records := make([]domainImportItem, 0, len(body.Items))
		records = append(records, body.Items...)
		if strings.TrimSpace(body.Text) != "" {
			parsed, err := parseDomainImportText(body.Text)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			records = append(records, parsed...)
		}
		if len(records) == 0 {
			writeError(w, http.StatusBadRequest, "no domains provided")
			return
		}

		_ = s.domains.EnsureDefaultServer(r.Context(), p.UserEmail)
		created := 0
		skipped := 0
		for idx, item := range records {
			cleanURL, err := normalizeImportedDomain(item.URL)
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid domain at row %d: %s", idx+1, err.Error()))
				return
			}
			serverID := strings.TrimSpace(firstNonEmpty(item.ServerID, item.Server))
			if serverID == "" {
				serverID = sqlstore.DefaultServerID
			}
			targetCountry := strings.TrimSpace(item.Country)
			if targetCountry == "" {
				targetCountry = p.TargetCountry
			}
			targetLanguage := strings.TrimSpace(item.Language)
			if targetLanguage == "" {
				targetLanguage = p.TargetLanguage
			}

			var pubPath, siteOwner string
			domainStatus := "waiting"

			if s.resolveDomainDeploymentMode(sqlstore.Domain{}) == "ssh_remote" {
				parsedUrl, _ := url.Parse(cleanURL)
				host := parsedUrl.Hostname()
				if host == "" {
					host = cleanURL
				}

				discoveredPath, discoveredOwner, discErr := s.contentBackend.DiscoverDomain(r.Context(), serverID, host)
				if discErr != nil {
					if s.logger != nil {
						s.logger.Warnf("Сайт %s пропущен: не настроен на сервере (%v)", cleanURL, discErr)
					}
					// Если домена нет на сервере, просто пропускаем его и не добавляем в БД
					skipped++
					continue
				}
				pubPath = discoveredPath
				siteOwner = discoveredOwner
				domainStatus = "published"
			}
			d := sqlstore.Domain{
				ID:             uuid.NewString(),
				ProjectID:      projectID,
				URL:            cleanURL,
				MainKeyword:    strings.TrimSpace(item.Keyword),
				TargetCountry:  targetCountry,
				TargetLanguage: targetLanguage,
				ServerID:       sqlstore.NullableString(serverID),
				Status:         domainStatus,
			}

			if pubPath != "" {
				d.PublishedPath = sqlstore.NullableString(pubPath)
			}
			if siteOwner != "" {
				d.SiteOwner = sqlstore.NullableString(siteOwner)
			}
			if err := s.domains.Create(r.Context(), d); err != nil {
				if s.logger != nil {
					s.logger.Warnf("import domain failed (url=%s): %v", cleanURL, err)
				}
				skipped++
				continue
			}
			anchor := strings.TrimSpace(firstNonEmpty(item.LinkAnchorText, item.Anchor))
			acceptor := strings.TrimSpace(firstNonEmpty(item.LinkAcceptorURL, item.Acceptor))
			if anchor != "" || acceptor != "" {
				_, err := s.domains.UpdateLinkSettings(
					r.Context(),
					d.ID,
					nullStringFromOptional(&anchor),
					nullStringFromOptional(&acceptor),
				)
				if err != nil && s.logger != nil {
					s.logger.Warnf("import link settings failed (domain=%s): %v", d.ID, err)
				}
			}
			created++
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"created": created,
			"skipped": skipped,
			"total":   len(records),
		})
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
		cleanURL, err := normalizeImportedDomain(body.URL)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid domain")
			return
		}
		_ = s.domains.EnsureDefaultServer(r.Context(), p.UserEmail)

		serverIDToUse := sqlstore.DefaultServerID
		if body.ServerID != "" {
			serverIDToUse = body.ServerID
		}

		var pubPath, siteOwner string
		domainStatus := "waiting"

		if s.resolveDomainDeploymentMode(sqlstore.Domain{}) == "ssh_remote" {
			parsedUrl, _ := url.Parse(cleanURL)
			host := parsedUrl.Hostname()
			if host == "" {
				host = cleanURL
			}

			discoveredPath, discoveredOwner, discErr := s.contentBackend.DiscoverDomain(r.Context(), serverIDToUse, host)
			if discErr != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("Сайт еще не настроен на сервере: %v", discErr))
				return
			}
			pubPath = discoveredPath
			siteOwner = discoveredOwner
			domainStatus = "published"
		}

		d := sqlstore.Domain{
			ID:             uuid.NewString(),
			ProjectID:      projectID,
			URL:            cleanURL,
			MainKeyword:    body.Keyword,
			TargetCountry:  p.TargetCountry,
			TargetLanguage: p.TargetLanguage,
			ServerID:       sqlstore.NullableString(serverIDToUse),
			Status:         domainStatus,
		}

		if pubPath != "" {
			d.PublishedPath = sqlstore.NullableString(pubPath)
		}
		if siteOwner != "" {
			d.SiteOwner = sqlstore.NullableString(siteOwner)
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
	case "prompts":
		stage := ""
		if len(parts) > 2 {
			stage, _ = url.PathUnescape(parts[2])
		}
		s.handleDomainPrompts(w, r, domainID, stage)
	case "deployments":
		s.handleDomainDeployments(w, r, domainID)
	case "editor":
		s.handleDomainEditorActions(w, r, domainID, parts[2:])
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

func (s *Server) resolveDomainDeploymentMode(domain sqlstore.Domain) string {
	if mode := strings.ToLower(strings.TrimSpace(s.cfg.DeployMode)); mode == "local_mock" {
		return "local_mock"
	}
	return "ssh_remote"
}

func (s *Server) ensureDomainGeneratePreflight(ctx context.Context, domain sqlstore.Domain) error {
	if s.resolveDomainDeploymentMode(domain) != "ssh_remote" {
		return nil
	}
	if strings.TrimSpace(nullableStringValue(domain.ServerID)) == "" {
		return errDomainGeneratePreflightServerID
	}
	if strings.TrimSpace(nullableStringValue(domain.PublishedPath)) == "" {
		return errors.New("published_path is empty")
	}
	if strings.TrimSpace(s.cfg.DeployMode) != "ssh_remote" {
		return fmt.Errorf("remote backend is disabled: DEPLOY_MODE=%s", strings.TrimSpace(s.cfg.DeployMode))
	}
	if _, err := s.statDomainPathInBackend(ctx, domain, ""); err != nil {
		return err
	}
	return nil
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
		updated, _, recoverErr := s.recoverStaleGenerationTransition(r.Context(), lastGen, &d)
		if recoverErr != nil && s.logger != nil {
			s.logger.Warnw("failed to recover stale generation transition before run",
				"generation_id", lastGen.ID,
				"status", lastGen.Status,
				"error", recoverErr,
			)
		} else {
			lastGen = updated
		}
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
	if err := s.ensureDomainGeneratePreflight(r.Context(), d); err != nil {
		if s.logger != nil {
			s.logger.Warnw("domain generate preflight failed",
				"domain_id", d.ID,
				"project_id", d.ProjectID,
				"deployment_mode", s.resolveDomainDeploymentMode(d),
				"server_id", nullableStringValue(d.ServerID),
				"published_path", nullableStringValue(d.PublishedPath),
				"error", err,
			)
		}
		if errors.Is(err, errDomainGeneratePreflightServerID) {
			writeError(w, http.StatusBadRequest, "Не указан сервер (server_id) для удаленной публикации.")
			return
		}
		writeError(w, http.StatusBadRequest, "Сайт еще не настроен на сервере, обратитесь к администратору.")
		return
	}
	genID := uuid.NewString()

	// Попытка скопировать артефакты из последней успешной генерации, если делаем force rerun
	var baseArtifacts []byte
	if forceStep != "" {
		if last, err := s.generations.GetLastByDomain(r.Context(), domainID); err == nil && len(last.Artifacts) > 0 {
			baseArtifacts = last.Artifacts
		} else if last, err := s.generations.GetLastSuccessfulByDomain(r.Context(), domainID); err == nil && len(last.Artifacts) > 0 {
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

	if s.tasks == nil {
		errMsg := "queue unavailable"
		now := time.Now().UTC()
		_ = s.generations.UpdateFull(r.Context(), genID, "error", 0, &errMsg, json.RawMessage(`[]`), nil, &now, &now, nil)
		_ = s.domains.UpdateStatus(r.Context(), domainID, stableDomainStatusFromDomain(d))
		writeError(w, http.StatusServiceUnavailable, "queue unavailable")
		return
	}
	task := tasks.NewGenerateTask(genID, domainID, forceStep)
	queue := s.genQueueForProject(d.ProjectID)
	if _, err := s.tasks.Enqueue(r.Context(), task, asynq.Queue(queue)); err != nil {
		if s.logger != nil {
			s.logger.Warnf("failed to enqueue generation: %v", err)
		}
		errMsg := "enqueue failed"
		now := time.Now().UTC()
		_ = s.generations.UpdateFull(r.Context(), genID, "error", 0, &errMsg, json.RawMessage(`[]`), nil, &now, &now, nil)
		_ = s.domains.UpdateStatus(r.Context(), domainID, stableDomainStatusFromDomain(d))
		writeError(w, http.StatusInternalServerError, "could not enqueue generation")
		return
	}
	_ = s.domains.SetLastGeneration(r.Context(), domainID, genID)
	_ = s.domains.UpdateStatus(r.Context(), domainID, "processing")

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

const (
	editorContextPackMaxTotalBytes   = 180 * 1024
	editorContextPackMaxFileBytes    = 40 * 1024
	editorContextPackMaxFiles        = 20
	editorContextPackCacheTTL        = 10 * time.Minute
	editorContextPackCacheMaxEntries = 200
)

type editorContextPackCacheEntry struct {
	Prompt    string
	Meta      editorContextPackMeta
	ExpiresAt time.Time
}

type editorContextPackMeta struct {
	PackHash    string   `json:"pack_hash"`
	FilesUsed   int      `json:"files_used"`
	BytesUsed   int      `json:"bytes_used"`
	Truncated   bool     `json:"truncated"`
	SourceFiles []string `json:"source_files"`
}

func (s *Server) handleDomainEditorActions(w http.ResponseWriter, r *http.Request, domainID string, parts []string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
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
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch parts[0] {
	case "context-pack":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		targetPath := strings.TrimSpace(r.URL.Query().Get("target_path"))
		if targetPath == "" {
			targetPath = "index.html"
		}
		if !strings.HasSuffix(strings.ToLower(targetPath), ".html") {
			targetPath += ".html"
		}
		cleanTarget, err := normalizeEditorProtectedPath(targetPath)
		if err != nil {
			writeEditorError(w, http.StatusBadRequest, editorErrForbiddenPath, err.Error(), nil)
			return
		}
		contextMode := normalizeEditorContextMode(r.URL.Query().Get("context_mode"))
		var contextFiles []string
		for _, raw := range strings.Split(strings.TrimSpace(r.URL.Query().Get("context_files")), ",") {
			raw = strings.TrimSpace(raw)
			if raw != "" {
				contextFiles = append(contextFiles, raw)
			}
		}
		packPrompt, meta, err := s.buildEditorContextPack(r.Context(), domain, cleanTarget, "", contextFiles, contextMode)
		if err != nil {
			writeEditorContextPackError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"target_path":       cleanTarget,
			"context_mode":      contextMode,
			"context_pack_meta": meta,
			"site_context":      packPrompt,
		})
		return
	case "ai-create-page":
		s.handleDomainEditorAICreatePage(w, r, domain, project, user.Email)
		return
	case "ai-regenerate-asset":
		s.handleDomainEditorAIRegenerateAsset(w, r, domain, project, user.Email)
		return
	default:
		writeError(w, http.StatusNotFound, "not found")
		return
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
			if err := s.domains.UpdateLinkStatus(r.Context(), domain.ID, "pending"); err != nil {
				if s.logger != nil {
					s.logger.Warnf("import link task status update failed: %v", err)
				}
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
		if err := s.domains.UpdateLinkStatus(r.Context(), domain.ID, "pending"); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update domain link status")
			return
		}
		writeJSON(w, http.StatusCreated, toLinkTaskDTO(task))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
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
		emptyLogs := []string{}
		updates := sqlstore.LinkTaskUpdates{
			AnchorText:       &anchor,
			TargetURL:        &target,
			Action:           &action,
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
		if err := s.linkTasks.Update(r.Context(), activeTask.ID, updates); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update link task")
			return
		}
		if err := s.domains.UpdateLinkStatus(r.Context(), domain.ID, "pending"); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update domain link status")
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
	if err := s.domains.UpdateLinkStatus(r.Context(), domain.ID, "pending"); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update domain link status")
		return
	}
	writeJSON(w, http.StatusCreated, toLinkTaskDTO(task))
}

func (s *Server) findActiveLinkTask(ctx context.Context, domainID string) (*sqlstore.LinkTask, error) {
	activeStatuses := []string{"removing", "searching", "pending"}
	for _, status := range activeStatuses {
		filters := sqlstore.LinkTaskFilters{Status: &status, Limit: 1, SortDesc: true}
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
