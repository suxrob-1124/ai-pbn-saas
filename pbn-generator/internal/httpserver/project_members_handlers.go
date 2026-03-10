package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) handleProjectMembers(w http.ResponseWriter, r *http.Request, projectID string) {
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

	memberRole, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
	if err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	if !strings.EqualFold(user.Role, "admin") && memberRole != "owner" {
		writeError(w, http.StatusForbidden, "only owner or admin can manage members")
		return
	}

	switch r.Method {
	case http.MethodGet:
		members, err := s.projectMembers.ListByProject(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load members")
			return
		}

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

	project, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}

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
	if email == strings.ToLower(user.Email) {
		writeError(w, http.StatusForbidden, "cannot change your own role")
		return
	}

	switch r.Method {
	case http.MethodPatch:
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
		if err := s.projectMembers.Remove(r.Context(), projectID, email); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "member removed"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
