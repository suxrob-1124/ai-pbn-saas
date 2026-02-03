package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ProjectMemberStore управляет участниками проектов
type ProjectMemberStore struct {
	db *sql.DB
}

// NewProjectMemberStore создает новый store для участников проектов
func NewProjectMemberStore(db *sql.DB) *ProjectMemberStore {
	return &ProjectMemberStore{db: db}
}

// Add добавляет участника в проект
func (s *ProjectMemberStore) Add(ctx context.Context, projectID, email, role string) error {
	if projectID == "" || email == "" {
		return errors.New("project_id and email are required")
	}
	validRoles := map[string]bool{"owner": true, "editor": true, "viewer": true}
	if !validRoles[role] {
		return fmt.Errorf("invalid role: %s (allowed: owner, editor, viewer)", role)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO project_members(project_id, user_email, role, created_at) 
		VALUES($1,$2,$3,NOW()) 
		ON CONFLICT (project_id, user_email) DO UPDATE SET role=$3`, projectID, email, role)
	if err != nil {
		// Проверяем на foreign key constraint - пользователь не существует
		if strings.Contains(err.Error(), "foreign key constraint") || strings.Contains(err.Error(), "user_email_fkey") {
			return errors.New("user not found")
		}
		return fmt.Errorf("failed to add member: %w", err)
	}
	return nil
}

// Remove удаляет участника из проекта
func (s *ProjectMemberStore) Remove(ctx context.Context, projectID, email string) error {
	if projectID == "" || email == "" {
		return errors.New("project_id and email are required")
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1 AND user_email=$2`, projectID, email)
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("member not found")
	}
	return nil
}

// UpdateRole обновляет роль участника
func (s *ProjectMemberStore) UpdateRole(ctx context.Context, projectID, email, role string) error {
	if projectID == "" || email == "" {
		return errors.New("project_id and email are required")
	}
	validRoles := map[string]bool{"owner": true, "editor": true, "viewer": true}
	if !validRoles[role] {
		return fmt.Errorf("invalid role: %s (allowed: owner, editor, viewer)", role)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE project_members SET role=$3 WHERE project_id=$1 AND user_email=$2`, projectID, email, role)
	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("member not found")
	}
	return nil
}

// Get получает информацию об участнике
func (s *ProjectMemberStore) Get(ctx context.Context, projectID, email string) (ProjectMember, error) {
	var pm ProjectMember
	err := s.db.QueryRowContext(ctx, `SELECT project_id, user_email, role, created_at FROM project_members WHERE project_id=$1 AND user_email=$2`, projectID, email).
		Scan(&pm.ProjectID, &pm.UserEmail, &pm.Role, &pm.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectMember{}, errors.New("member not found")
		}
		return ProjectMember{}, fmt.Errorf("failed to get member: %w", err)
	}
	return pm, nil
}

// ListByProject возвращает список всех участников проекта
func (s *ProjectMemberStore) ListByProject(ctx context.Context, projectID string) ([]ProjectMember, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, user_email, role, created_at FROM project_members WHERE project_id=$1 ORDER BY created_at`, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %w", err)
	}
	defer rows.Close()
	var res []ProjectMember
	for rows.Next() {
		var pm ProjectMember
		if err := rows.Scan(&pm.ProjectID, &pm.UserEmail, &pm.Role, &pm.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, pm)
	}
	return res, rows.Err()
}

// ListByUser возвращает список проектов, в которых участвует пользователь
func (s *ProjectMemberStore) ListByUser(ctx context.Context, email string) ([]ProjectMember, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, user_email, role, created_at FROM project_members WHERE user_email=$1 ORDER BY created_at`, email)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()
	var res []ProjectMember
	for rows.Next() {
		var pm ProjectMember
		if err := rows.Scan(&pm.ProjectID, &pm.UserEmail, &pm.Role, &pm.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, pm)
	}
	return res, rows.Err()
}

// HasAccess проверяет, имеет ли пользователь доступ к проекту (как владелец или участник)
func (s *ProjectMemberStore) HasAccess(ctx context.Context, projectID, email string) (bool, string, error) {
	// Сначала проверяем, является ли пользователь владельцем
	var ownerEmail string
	err := s.db.QueryRowContext(ctx, `SELECT user_email FROM projects WHERE id=$1`, projectID).Scan(&ownerEmail)
	if err != nil {
		return false, "", fmt.Errorf("project not found: %w", err)
	}
	if ownerEmail == email {
		return true, "owner", nil
	}
	// Проверяем, является ли пользователь участником
	var role string
	err = s.db.QueryRowContext(ctx, `SELECT role FROM project_members WHERE project_id=$1 AND user_email=$2`, projectID, email).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("failed to check access: %w", err)
	}
	return true, role, nil
}
