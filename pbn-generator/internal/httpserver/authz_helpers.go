package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// projectMemberRoleKey используется для передачи роли участника проекта в контексте
type projectMemberRoleKeyType string

const projectMemberRoleKey projectMemberRoleKeyType = "project_member_role"

// isAdmin returns true only for system-level admin role.
func isAdmin(role string) bool {
	return strings.EqualFold(role, "admin")
}

// canCreateProject returns true for admin and manager system roles.
func canCreateProject(role string) bool {
	r := strings.ToLower(role)
	return r == "admin" || r == "manager"
}

func (s *Server) authorizeProject(ctx context.Context, projectID string) (sqlstore.Project, error) {
	u, ok := currentUserFromContext(ctx)
	if !ok || u.Email == "" {
		return sqlstore.Project{}, errUnauthorized
	}

	if isAdmin(u.Role) {
		return s.projects.GetByID(ctx, projectID)
	}

	p, err := s.projects.Get(ctx, projectID, u.Email)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return sqlstore.Project{}, err
	}

	if s.projectMembers != nil {
		member, err := s.projectMembers.Get(ctx, projectID, u.Email)
		if err == nil {
			ctx = context.WithValue(ctx, projectMemberRoleKey, member.Role)
			p, err = s.projects.GetByID(ctx, projectID)
			if err != nil {
				return sqlstore.Project{}, err
			}
			return p, nil
		}
	}

	return sqlstore.Project{}, errForbidden
}

// getProjectMemberRole возвращает роль пользователя в проекте (owner, editor, viewer)
func (s *Server) getProjectMemberRole(ctx context.Context, projectID, email string) (string, error) {
	if user, ok := currentUserFromContext(ctx); ok && isAdmin(user.Role) {
		return "admin", nil
	}
	p, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return "", err
	}
	if p.UserEmail == email {
		return "owner", nil
	}

	if s.projectMembers != nil {
		member, err := s.projectMembers.Get(ctx, projectID, email)
		if err == nil {
			return member.Role, nil
		}
	}

	return "", errors.New("user is not a member of this project")
}

func (s *Server) resolveProjectRole(ctx context.Context, project sqlstore.Project, user auth.User) string {
	if isAdmin(user.Role) {
		return "admin"
	}
	if strings.EqualFold(project.UserEmail, user.Email) {
		return "owner"
	}
	if s.projectMembers != nil {
		member, err := s.projectMembers.Get(ctx, project.ID, user.Email)
		if err == nil {
			role := strings.ToLower(strings.TrimSpace(member.Role))
			switch role {
			case "viewer", "editor", "owner", "manager":
				return role
			}
		}
	}
	return "viewer"
}

// hasProjectPermission проверяет, имеет ли пользователь достаточные права для действия
func hasProjectPermission(userRole string, memberRole string, requiredRole string) bool {
	if strings.EqualFold(userRole, "admin") {
		return true
	}

	actualRole := memberRole
	if memberRole == "" {
		actualRole = "owner"
	}

	roleHierarchy := map[string]int{
		"viewer":  1,
		"editor":  2,
		"manager": 4,
		"owner":   4,
	}

	userLevel := roleHierarchy[actualRole]
	requiredLevel := roleHierarchy[requiredRole]

	return userLevel >= requiredLevel
}

// requireProjectRole проверяет права доступа к проекту
func (s *Server) requireProjectRole(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/api/projects/")
			parts := strings.Split(path, "/")
			projectID := parts[0]

			user, ok := currentUserFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			memberRole, err := s.getProjectMemberRole(r.Context(), projectID, user.Email)
			if err != nil {
				writeError(w, http.StatusForbidden, "access denied")
				return
			}

			if !hasProjectPermission(user.Role, memberRole, requiredRole) {
				writeError(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) authorizeDomain(ctx context.Context, domainID string) (sqlstore.Domain, sqlstore.Project, error) {
	d, err := s.domains.Get(ctx, domainID)
	if err != nil {
		return sqlstore.Domain{}, sqlstore.Project{}, err
	}
	p, err := s.authorizeProject(ctx, d.ProjectID)
	if err != nil {
		return sqlstore.Domain{}, sqlstore.Project{}, err
	}
	return d, p, nil
}

func respondAuthzError(w http.ResponseWriter, err error, notFoundMsg string) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if notFoundMsg == "" {
			notFoundMsg = "not found"
		}
		writeError(w, http.StatusNotFound, notFoundMsg)
	case errors.Is(err, errForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, errUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, errPendingApproval):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
	return true
}
