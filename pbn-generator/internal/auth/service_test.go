package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
)

// mockUserStore для тестирования SetUserRole
type mockUserStore struct {
	users map[string]User
}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{
		users: make(map[string]User),
	}
}

func (m *mockUserStore) Create(ctx context.Context, email, password string) (User, error) {
	return User{}, errors.New("not implemented")
}

func (m *mockUserStore) Authenticate(ctx context.Context, email, password string) (User, error) {
	return User{}, errors.New("not implemented")
}

func (m *mockUserStore) UpdatePassword(ctx context.Context, email, newPassword string) error {
	return errors.New("not implemented")
}

func (m *mockUserStore) SetVerified(ctx context.Context, email string) error {
	return errors.New("not implemented")
}

func (m *mockUserStore) Get(ctx context.Context, email string) (User, error) {
	user, ok := m.users[email]
	if !ok {
		return User{}, errors.New("user not found")
	}
	return user, nil
}

func (m *mockUserStore) UpdateProfile(ctx context.Context, email, name, avatarURL string) error {
	return errors.New("not implemented")
}

func (m *mockUserStore) ChangeEmail(ctx context.Context, oldEmail, newEmail string) error {
	return errors.New("not implemented")
}

func (m *mockUserStore) List(ctx context.Context) ([]User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserStore) UpdateRole(ctx context.Context, email, role string) error {
	user, ok := m.users[email]
	if !ok {
		return errors.New("user not found")
	}
	user.Role = role
	m.users[email] = user
	return nil
}

func (m *mockUserStore) SetApproved(ctx context.Context, email string, approved bool) error {
	return errors.New("not implemented")
}

func (m *mockUserStore) SetAPIKey(ctx context.Context, email string, ciphertext []byte, updatedAt time.Time) error {
	return errors.New("not implemented")
}

func (m *mockUserStore) ClearAPIKey(ctx context.Context, email string) error {
	return errors.New("not implemented")
}

func (m *mockUserStore) GetAPIKey(ctx context.Context, email string) ([]byte, *time.Time, error) {
	return nil, nil, errors.New("not implemented")
}

func (m *mockUserStore) Delete(ctx context.Context, email string) error {
	if _, ok := m.users[email]; !ok {
		return errors.New("not found")
	}
	delete(m.users, email)
	return nil
}

func setupServiceForRoleTests(t *testing.T) (*Service, *mockUserStore) {
	t.Helper()
	logger := zap.NewNop().Sugar()
	cfg := config.Config{}
	store := newMockUserStore()
	svc := NewService(ServiceDeps{
		Config: cfg,
		Users:  store,
		Logger: logger,
	})
	return svc, store
}

func TestSetUserRole_ValidRoles(t *testing.T) {
	svc, store := setupServiceForRoleTests(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		email    string
		role     string
		wantErr  bool
		errorMsg string
	}{
		{
			name:    "valid admin role",
			email:   "user@example.com",
			role:    "admin",
			wantErr: false,
		},
		{
			name:    "valid manager role",
			email:   "user@example.com",
			role:    "manager",
			wantErr: false,
		},
		{
			name:    "admin role with uppercase",
			email:   "user@example.com",
			role:    "ADMIN",
			wantErr: false,
		},
		{
			name:    "manager role with mixed case",
			email:   "user@example.com",
			role:    "Manager",
			wantErr: false,
		},
		{
			name:    "admin role with whitespace",
			email:   "user@example.com",
			role:    " admin ",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем пользователя
			store.users[tt.email] = User{
				Email: tt.email,
				Role:  "manager",
			}

			err := svc.SetUserRole(ctx, tt.email, tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetUserRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Проверяем, что роль обновилась (нормализованная версия)
				user, _ := store.Get(ctx, tt.email)
				expectedRole := strings.ToLower(strings.TrimSpace(tt.role))
				if user.Role != expectedRole {
					t.Errorf("SetUserRole() role = %v, want %v", user.Role, expectedRole)
				}
			}

			// Очищаем после теста
			delete(store.users, tt.email)
		})
	}
}

func TestSetUserRole_InvalidRoles(t *testing.T) {
	svc, store := setupServiceForRoleTests(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		email    string
		role     string
		wantErr  bool
		errorMsg string
	}{
		{
			name:     "invalid role: superadmin",
			email:    "user@example.com",
			role:     "superadmin",
			wantErr:  true,
			errorMsg: "invalid role",
		},
		{
			name:     "invalid role: hacker",
			email:    "user@example.com",
			role:     "hacker",
			wantErr:  true,
			errorMsg: "invalid role",
		},
		{
			name:     "invalid role: empty string",
			email:    "user@example.com",
			role:     "",
			wantErr:  true,
			errorMsg: "role is required",
		},
		{
			name:     "invalid role: whitespace only",
			email:    "user@example.com",
			role:     "   ",
			wantErr:  true,
			errorMsg: "role is required",
		},
		{
			name:     "invalid role: owner",
			email:    "user@example.com",
			role:     "owner",
			wantErr:  true,
			errorMsg: "invalid role",
		},
		{
			name:     "invalid role: editor",
			email:    "user@example.com",
			role:     "editor",
			wantErr:  true,
			errorMsg: "invalid role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем пользователя
			store.users[tt.email] = User{
				Email: tt.email,
				Role:  "manager",
			}

			err := svc.SetUserRole(ctx, tt.email, tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetUserRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if err == nil {
					t.Errorf("SetUserRole() expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("SetUserRole() error = %v, want error containing %v", err, tt.errorMsg)
				}
				// Проверяем, что роль НЕ изменилась
				user, _ := store.Get(ctx, tt.email)
				if user.Role != "manager" {
					t.Errorf("SetUserRole() role should not change, got %v", user.Role)
				}
			}

			// Очищаем после теста
			delete(store.users, tt.email)
		})
	}
}

func TestSetUserRole_ValidSystemRoles(t *testing.T) {
	// Тест проверяет, что ValidSystemRoles содержит только допустимые роли
	if len(ValidSystemRoles) != 3 {
		t.Errorf("ValidSystemRoles should contain exactly 3 roles, got %d", len(ValidSystemRoles))
	}

	if !ValidSystemRoles["admin"] {
		t.Error("ValidSystemRoles should contain 'admin'")
	}

	if !ValidSystemRoles["manager"] {
		t.Error("ValidSystemRoles should contain 'manager'")
	}

	if !ValidSystemRoles["user"] {
		t.Error("ValidSystemRoles should contain 'user'")
	}

	// Проверяем, что нет других ролей
	for role := range ValidSystemRoles {
		if role != "admin" && role != "manager" && role != "user" {
			t.Errorf("ValidSystemRoles contains unexpected role: %s", role)
		}
	}
}

func TestSetUserRole_UserNotFound(t *testing.T) {
	svc, _ := setupServiceForRoleTests(t)
	ctx := context.Background()

	err := svc.SetUserRole(ctx, "nonexistent@example.com", "admin")
	if err == nil {
		t.Error("SetUserRole() expected error for nonexistent user, got nil")
	}
}

