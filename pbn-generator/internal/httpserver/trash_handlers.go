package httpserver

import (
	"net/http"
	"strings"
	"time"
)

// ── DTOs ────────────────────────────────────────────────────────────────────────

type trashProjectDTO struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	UserEmail string     `json:"user_email"`
	DeletedAt *time.Time `json:"deleted_at"`
	DeletedBy string     `json:"deleted_by"`
}

type trashDomainDTO struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	URL         string     `json:"url"`
	DeleteBatch string     `json:"delete_batch,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at"`
	DeletedBy   string     `json:"deleted_by"`
}

type trashListDTO struct {
	Projects          []trashProjectDTO `json:"projects"`
	Domains           []trashDomainDTO  `json:"domains"`
	RetentionDays     int               `json:"retention_days"`
}

// ── Admin trash endpoints ───────────────────────────────────────────────────────

// GET /api/admin/trash — list all soft-deleted projects and domains.
func (s *Server) handleAdminTrash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	projects, err := s.projects.ListDeleted(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list deleted projects")
		return
	}

	domains, err := s.domains.ListDeleted(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list deleted domains")
		return
	}

	resp := trashListDTO{
		Projects:      make([]trashProjectDTO, 0, len(projects)),
		Domains:       make([]trashDomainDTO, 0, len(domains)),
		RetentionDays: s.cfg.SoftDeleteRetentionDays,
	}
	for _, p := range projects {
		var deletedAt *time.Time
		if p.DeletedAt.Valid {
			deletedAt = &p.DeletedAt.Time
		}
		resp.Projects = append(resp.Projects, trashProjectDTO{
			ID:        p.ID,
			Name:      p.Name,
			UserEmail: p.UserEmail,
			DeletedAt: deletedAt,
			DeletedBy: p.DeletedBy.String,
		})
	}
	for _, d := range domains {
		var deletedAt *time.Time
		if d.DeletedAt.Valid {
			deletedAt = &d.DeletedAt.Time
		}
		resp.Domains = append(resp.Domains, trashDomainDTO{
			ID:          d.ID,
			ProjectID:   d.ProjectID,
			URL:         d.URL,
			DeleteBatch: d.DeleteBatch.String,
			DeletedAt:   deletedAt,
			DeletedBy:   d.DeletedBy.String,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// /api/admin/trash/{type}/{id}/{action} dispatcher.
func (s *Server) handleAdminTrashAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/trash/")
	parts := strings.Split(path, "/")
	// Expect: {type}/{id}/{action?}
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusBadRequest, "invalid trash path")
		return
	}
	entityType := parts[0] // "projects" or "domains"
	entityID := parts[1]
	action := ""
	if len(parts) > 2 {
		action = parts[2]
	}

	switch entityType {
	case "projects":
		switch {
		case action == "restore" && r.Method == http.MethodPost:
			if err := s.projects.Restore(r.Context(), entityID); err != nil {
				writeError(w, http.StatusInternalServerError, "could not restore project")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
		case action == "" && r.Method == http.MethodDelete:
			p, err := s.projects.GetByIDIncludingDeleted(r.Context(), entityID)
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			if err := s.projects.Delete(r.Context(), entityID, p.UserEmail); err != nil {
				writeError(w, http.StatusInternalServerError, "could not permanently delete project")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "purged"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case "domains":
		switch {
		case action == "restore" && r.Method == http.MethodPost:
			if err := s.domains.Restore(r.Context(), entityID); err != nil {
				writeError(w, http.StatusInternalServerError, "could not restore domain")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
		case action == "" && r.Method == http.MethodDelete:
			if err := s.domains.Delete(r.Context(), entityID); err != nil {
				writeError(w, http.StatusInternalServerError, "could not permanently delete domain")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "purged"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown trash entity type")
	}
}

// ── Owner-level restore endpoints ───────────────────────────────────────────────

// POST /api/projects/{id}/restore — owner or admin can restore a soft-deleted project.
func (s *Server) handleProjectRestore(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	project, err := s.projects.GetByIDIncludingDeleted(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if !project.DeletedAt.Valid {
		writeError(w, http.StatusBadRequest, "project is not deleted")
		return
	}
	if !isAdmin(user.Role) && !strings.EqualFold(user.Email, project.UserEmail) {
		writeError(w, http.StatusForbidden, "only owner or admin can restore project")
		return
	}

	if err := s.projects.Restore(r.Context(), projectID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not restore project")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

// POST /api/domains/{id}/restore — owner or admin can restore a soft-deleted domain.
func (s *Server) handleDomainRestore(w http.ResponseWriter, r *http.Request, domainID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	domain, err := s.domains.GetIncludingDeleted(r.Context(), domainID)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if !domain.DeletedAt.Valid {
		writeError(w, http.StatusBadRequest, "domain is not deleted")
		return
	}

	// Check that the parent project is not deleted and user has access.
	if !isAdmin(user.Role) {
		if _, err := s.authorizeProject(r.Context(), domain.ProjectID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
	}

	if err := s.domains.Restore(r.Context(), domainID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not restore domain")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}
