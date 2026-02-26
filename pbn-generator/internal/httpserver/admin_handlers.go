package httpserver

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

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
		user, ok := currentUserFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		updated := false
		if body.Role != nil {
			if strings.EqualFold(user.Email, targetEmail) {
				writeError(w, http.StatusForbidden, "cannot change your own role")
				return
			}

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
	projectDTOs := make([]projectDTO, 0, len(projects))
	for _, p := range projects {
		projectDTOs = append(projectDTOs, s.toProjectDTO(r.Context(), p))
	}
	writeJSON(w, http.StatusOK, projectDTOs)
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

func (s *Server) handleAdminLLMUsageEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.llmUsage == nil {
		writeError(w, http.StatusInternalServerError, "llm usage store not configured")
		return
	}
	filters := parseLLMUsageFilters(r)
	items, err := s.llmUsage.ListEvents(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list llm usage")
		return
	}
	total, err := s.llmUsage.CountEvents(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not count llm usage")
		return
	}
	resp := make([]llmUsageEventDTO, 0, len(items))
	for _, item := range items {
		resp = append(resp, toLLMUsageEventDTO(item))
	}
	writeJSON(w, http.StatusOK, llmUsageListDTO{Items: resp, Total: total})
}

func (s *Server) handleAdminLLMUsageStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.llmUsage == nil {
		writeError(w, http.StatusInternalServerError, "llm usage store not configured")
		return
	}
	filters := parseLLMUsageFilters(r)
	filters.Limit = 0
	filters.Offset = 0
	stats, err := s.llmUsage.AggregateStats(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not aggregate llm usage")
		return
	}
	dto := toLLMUsageStatsDTO(stats)
	dto = filterLLMUsageStatsDTO(dto, parseLLMUsageGroupBy(r.URL.Query().Get("group_by")))
	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) handleAdminLLMPricing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.modelPricing == nil {
		writeError(w, http.StatusInternalServerError, "model pricing store not configured")
		return
	}
	items, err := s.modelPricing.ListActive(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list llm model pricing")
		return
	}
	resp := make([]llmPricingDTO, 0, len(items))
	for _, item := range items {
		resp = append(resp, toLLMPricingDTO(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAdminLLMPricingByModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.modelPricing == nil {
		writeError(w, http.StatusInternalServerError, "model pricing store not configured")
		return
	}
	model := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/admin/llm-pricing/"))
	model, _ = url.PathUnescape(model)
	model = strings.TrimSpace(model)
	if model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	defer r.Body.Close()
	var body struct {
		Provider            string  `json:"provider"`
		InputUSDPerMillion  float64 `json:"input_usd_per_million"`
		OutputUSDPerMillion float64 `json:"output_usd_per_million"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	provider := strings.TrimSpace(body.Provider)
	if provider == "" {
		provider = "gemini"
	}
	if body.InputUSDPerMillion <= 0 || body.OutputUSDPerMillion <= 0 {
		writeError(w, http.StatusBadRequest, "input_usd_per_million and output_usd_per_million must be > 0")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || strings.TrimSpace(user.Email) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.modelPricing.UpsertActive(
		r.Context(),
		provider,
		model,
		body.InputUSDPerMillion,
		body.OutputUSDPerMillion,
		user.Email,
		time.Now().UTC(),
	); err != nil {
		writeError(w, http.StatusInternalServerError, "could not update model pricing")
		return
	}
	item, err := s.modelPricing.GetActiveByModel(r.Context(), provider, model, time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "updated"})
		return
	}
	writeJSON(w, http.StatusOK, toLLMPricingDTO(*item))
}

func (s *Server) handleProjectLLMUsageEvents(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.llmUsage == nil {
		writeError(w, http.StatusInternalServerError, "llm usage store not configured")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || strings.TrimSpace(user.Email) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	_, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}
	if !strings.EqualFold(user.Role, "admin") {
		memberRole, roleErr := s.getProjectMemberRole(r.Context(), projectID, user.Email)
		if roleErr != nil || !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "only admin or project owner/manager can view llm usage")
			return
		}
	}
	filters := parseLLMUsageFilters(r)
	filters.ProjectID = &projectID
	items, err := s.llmUsage.ListEvents(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list llm usage")
		return
	}
	total, err := s.llmUsage.CountEvents(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not count llm usage")
		return
	}
	resp := make([]llmUsageEventDTO, 0, len(items))
	for _, item := range items {
		resp = append(resp, toLLMUsageEventDTO(item))
	}
	writeJSON(w, http.StatusOK, llmUsageListDTO{Items: resp, Total: total})
}

func (s *Server) handleProjectLLMUsageStats(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.llmUsage == nil {
		writeError(w, http.StatusInternalServerError, "llm usage store not configured")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || strings.TrimSpace(user.Email) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	_, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}
	if !strings.EqualFold(user.Role, "admin") {
		memberRole, roleErr := s.getProjectMemberRole(r.Context(), projectID, user.Email)
		if roleErr != nil || !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "only admin or project owner/manager can view llm usage")
			return
		}
	}
	filters := parseLLMUsageFilters(r)
	filters.ProjectID = &projectID
	filters.Limit = 0
	filters.Offset = 0
	stats, err := s.llmUsage.AggregateStats(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not aggregate llm usage")
		return
	}
	dto := toLLMUsageStatsDTO(stats)
	dto = filterLLMUsageStatsDTO(dto, parseLLMUsageGroupBy(r.URL.Query().Get("group_by")))
	writeJSON(w, http.StatusOK, dto)
}

func optionalString(val *string) string {
	if val == nil {
		return ""
	}
	return *val
}
