package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"errors"
	"io"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/config"
)

// Интеграционный сценарий: регистрация -> верификация -> логин -> reset -> логин.
func TestE2E_RegisterVerifyReset(t *testing.T) {
	cfg := config.Config{
		Port:                     "0",
		AllowedOrigin:            "*",
		SessionTTL:               24 * time.Hour,
		AccessTTL:                15 * time.Minute,
		RefreshTTL:               7 * 24 * time.Hour,
		SessionCleanInterval:     time.Minute,
		LoginRateLimit:           100,
		LoginRateWindow:          time.Minute,
		RegisterRateLimit:        100,
		RegisterRateWindow:       time.Minute,
		LoginEmailIpLimit:        100,
		LoginEmailIpWindow:       time.Minute,
		LoginLockoutFails:        5,
		LoginLockoutDuration:     time.Minute,
		EmailVerificationTTL:     24 * time.Hour,
		PasswordResetTTL:         time.Hour,
		RequireEmailVerification: true,
	}

	mailer := &captureMailer{}
	svc := auth.NewService(auth.ServiceDeps{
		Config:             cfg,
		Users:              newE2EUserStore(),
		Sessions:           newStubSessionStore(),
		VerificationTokens: newStubVerificationStore(),
		ResetTokens:        newStubResetStore(),
		Captchas:           newStubCaptchaStore(),
		EmailChanges:       newStubEmailChangeStore(),
		Mailer:             mailer,
		Logger:             nil,
	})
	logger := zap.NewNop().Sugar()
	server := New(cfg, svc, logger, newStubProjectStore(), newStubDomainStore(), newStubGenerationStore(), newStubPromptStore(), newStubEnqueuer())
	handler := server.Handler()

	client := &http.Client{}
	jar, _ := cookiejar.New(nil)
	client.Jar = jar
	client.Transport = handlerTransport{h: handler}

	email := "user@example.com"
	password := "Supersecret1"
	newPassword := "NewPassword123"

	// Регистрация
	status, _ := doPost(client, "http://local/api/register", map[string]any{"email": email, "password": password})
	if status != http.StatusCreated {
		t.Fatalf("register status = %d", status)
	}

	// Запрос письма с подтверждением
	status, _ = doPost(client, "http://local/api/verify/request", map[string]any{"email": email})
	if status != http.StatusOK {
		t.Fatalf("verify request status = %d", status)
	}
	token := mailer.lastToken()
	if token == "" {
		t.Fatalf("verification token not captured")
	}

	// Подтвердить
	status, _ = doPost(client, "http://local/api/verify/confirm", map[string]any{"token": token})
	if status != http.StatusOK {
		t.Fatalf("verify confirm status = %d", status)
	}

	// Логин
	status, _ = doPost(client, "http://local/api/login", map[string]any{"email": email, "password": password})
	if status != http.StatusOK {
		t.Fatalf("login status = %d", status)
	}
	// Проверка доступа по куке
	status, _ = doGet(client, "http://local/api/me")
	if status != http.StatusOK {
		t.Fatalf("me after login status = %d", status)
	}

	// Сброс пароля
	status, _ = doPost(client, "http://local/api/password/reset/request", map[string]any{"email": email})
	if status != http.StatusOK {
		t.Fatalf("reset request status = %d", status)
	}
	resetToken := mailer.lastToken()
	if resetToken == "" {
		t.Fatalf("reset token not captured")
	}

	status, _ = doPost(client, "http://local/api/password/reset/confirm", map[string]any{"token": resetToken, "newPassword": newPassword})
	if status != http.StatusOK {
		t.Fatalf("reset confirm status = %d", status)
	}

	// Логин новым паролем
	status, _ = doPost(client, "http://local/api/login", map[string]any{"email": email, "password": newPassword})
	if status != http.StatusOK {
		t.Fatalf("login with new password status = %d", status)
	}
	status, _ = doGet(client, "http://local/api/me")
	if status != http.StatusOK {
		t.Fatalf("me after new password login status = %d", status)
	}
}

// Интеграционный сценарий: refresh, logout, смена email.
func TestE2E_RefreshLogoutChangeEmail(t *testing.T) {
	cfg := config.Config{
		Port:                     "0",
		AllowedOrigin:            "*",
		SessionTTL:               24 * time.Hour,
		AccessTTL:                15 * time.Minute,
		RefreshTTL:               7 * 24 * time.Hour,
		SessionCleanInterval:     time.Minute,
		LoginRateLimit:           100,
		LoginRateWindow:          time.Minute,
		RegisterRateLimit:        100,
		RegisterRateWindow:       time.Minute,
		LoginEmailIpLimit:        100,
		LoginEmailIpWindow:       time.Minute,
		LoginLockoutFails:        5,
		LoginLockoutDuration:     time.Minute,
		EmailVerificationTTL:     24 * time.Hour,
		PasswordResetTTL:         time.Hour,
		RequireEmailVerification: true,
	}

	mailer := &captureMailer{}
	svc := auth.NewService(auth.ServiceDeps{
		Config:             cfg,
		Users:              newE2EUserStore(),
		Sessions:           newStubSessionStore(),
		VerificationTokens: newStubVerificationStore(),
		ResetTokens:        newStubResetStore(),
		Captchas:           newStubCaptchaStore(),
		EmailChanges:       newStubEmailChangeStore(),
		Mailer:             mailer,
		Logger:             nil,
	})
	logger := zap.NewNop().Sugar()
	server := New(cfg, svc, logger, newStubProjectStore(), newStubDomainStore(), newStubGenerationStore(), newStubPromptStore(), newStubEnqueuer())
	handler := server.Handler()
	client := &http.Client{}
	jar, _ := cookiejar.New(nil)
	client.Jar = jar
	client.Transport = handlerTransport{h: handler}

	email := "user2@example.com"
	newEmail := "user2+new@example.com"
	password := "Supersecret1"

	// Регистрация и верификация
	status, _ := doPost(client, "http://local/api/register", map[string]any{"email": email, "password": password})
	if status != http.StatusCreated {
		t.Fatalf("register status = %d", status)
	}
	status, _ = doPost(client, "http://local/api/verify/request", map[string]any{"email": email})
	if status != http.StatusOK {
		t.Fatalf("verify request status = %d", status)
	}
	token := mailer.lastToken()
	if token == "" {
		t.Fatalf("verification token not captured")
	}
	status, _ = doPost(client, "http://local/api/verify/confirm", map[string]any{"token": token})
	if status != http.StatusOK {
		t.Fatalf("verify confirm status = %d", status)
	}

	// Логин
	status, _ = doPost(client, "http://local/api/login", map[string]any{"email": email, "password": password})
	if status != http.StatusOK {
		t.Fatalf("login status = %d", status)
	}

	// Refresh
	status, _ = doPost(client, "http://local/api/refresh", map[string]any{})
	if status != http.StatusOK {
		t.Fatalf("refresh status = %d", status)
	}

	// Logout -> доступ должен пропасть
	status, _ = doPost(client, "http://local/api/logout", map[string]any{})
	if status != http.StatusOK {
		t.Fatalf("logout status = %d", status)
	}
	status, _ = doGet(client, "http://local/api/me")
	if status == http.StatusOK {
		t.Fatalf("expected unauthorized after logout")
	}

	// Логин снова
	status, _ = doPost(client, "http://local/api/login", map[string]any{"email": email, "password": password})
	if status != http.StatusOK {
		t.Fatalf("login2 status = %d", status)
	}

	// Смена email
	status, _ = doPost(client, "http://local/api/email/change/request", map[string]any{"newEmail": newEmail})
	if status != http.StatusOK {
		t.Fatalf("email change request status = %d", status)
	}
	changeToken := mailer.lastToken()
	if changeToken == "" {
		t.Fatalf("email change token not captured")
	}
	status, _ = doPost(client, "http://local/api/email/change/confirm", map[string]any{"token": changeToken})
	if status != http.StatusOK {
		t.Fatalf("email change confirm status = %d", status)
	}
	// После смены email и перевыдачи сессии /api/me должен вернуть новый email
	status, body := doGet(client, "http://local/api/me")
	if status != http.StatusOK {
		t.Fatalf("me after email change status = %d", status)
	}
	if !strings.Contains(string(body), newEmail) {
		t.Fatalf("me should contain new email, got: %s", string(body))
	}
}

func doPost(client *http.Client, url string, body map[string]any) (int, []byte) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return http.StatusServiceUnavailable, nil
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func doGet(client *http.Client, url string) (int, []byte) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return http.StatusServiceUnavailable, nil
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

type captureMailer struct {
	mu    sync.Mutex
	token string
	body  string
}

func (m *captureMailer) Send(ctx context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.body = body
	// попытаться вытащить токен из ссылки token=...
	if idx := strings.Index(body, "token="); idx != -1 {
		part := body[idx+len("token="):]
		for i, r := range part {
			if r == '\n' || r == '\r' || r == ' ' || r == '&' {
				part = part[:i]
				break
			}
		}
		m.token = strings.TrimSpace(part)
	} else {
		parts := strings.Fields(body)
		if len(parts) > 0 {
			m.token = parts[len(parts)-1]
		}
	}
	return nil
}

func (m *captureMailer) lastToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.token
}

// e2e-стобы
type e2eUserStore struct {
	mu       sync.Mutex
	users    map[string]string
	verified map[string]bool
	roles    map[string]string
	approved map[string]bool
}

func newE2EUserStore() *e2eUserStore {
	return &e2eUserStore{
		users:    make(map[string]string),
		verified: make(map[string]bool),
		roles:    make(map[string]string),
		approved: make(map[string]bool),
	}
}

func (s *e2eUserStore) Create(ctx context.Context, email, password string) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; ok {
		return auth.User{}, errors.New("user exists")
	}
	s.users[email] = password
	s.verified[email] = false
	s.roles[email] = "manager"
	s.approved[email] = true
	return auth.User{Email: email, CreatedAt: time.Now().UTC(), Verified: false, Role: s.roles[email], IsApproved: s.approved[email]}, nil
}

func (s *e2eUserStore) Authenticate(ctx context.Context, email, password string) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pw, ok := s.users[email]
	if !ok || pw != password {
		return auth.User{}, auth.ErrInvalidCredentials
	}
	return auth.User{Email: email, CreatedAt: time.Now().UTC(), Verified: s.verified[email], Role: s.roles[email], IsApproved: s.approved[email]}, nil
}

func (s *e2eUserStore) UpdatePassword(ctx context.Context, email, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("not found")
	}
	s.users[email] = newPassword
	return nil
}

func (s *e2eUserStore) SetVerified(ctx context.Context, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("not found")
	}
	s.verified[email] = true
	return nil
}

func (s *e2eUserStore) Get(ctx context.Context, email string) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pw, ok := s.users[email]
	if !ok {
		return auth.User{}, errors.New("not found")
	}
	return auth.User{Email: email, PasswordHash: []byte(pw), CreatedAt: time.Now().UTC(), Verified: s.verified[email], Role: s.roles[email], IsApproved: s.approved[email]}, nil
}

func (s *e2eUserStore) UpdateProfile(ctx context.Context, email, name, avatarURL string) error {
	return nil
}

func (s *e2eUserStore) ChangeEmail(ctx context.Context, oldEmail, newEmail string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pw, ok := s.users[oldEmail]
	if !ok {
		return errors.New("not found")
	}
	delete(s.users, oldEmail)
	s.users[newEmail] = pw
	s.verified[newEmail] = s.verified[oldEmail]
	delete(s.verified, oldEmail)
	return nil
}

func (s *e2eUserStore) List(ctx context.Context) ([]auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []auth.User
	for email := range s.users {
		res = append(res, auth.User{
			Email:      email,
			CreatedAt:  time.Now().UTC(),
			Verified:   s.verified[email],
			Role:       s.roles[email],
			IsApproved: s.approved[email],
		})
	}
	return res, nil
}

func (s *e2eUserStore) UpdateRole(ctx context.Context, email, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("not found")
	}
	s.roles[email] = role
	return nil
}

func (s *e2eUserStore) SetApproved(ctx context.Context, email string, approved bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("not found")
	}
	s.approved[email] = approved
	return nil
}

// Используем http.Client с кастомным Transport, чтобы обходиться без реального порта.
type handlerTransport struct {
	h http.Handler
}

func (t handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rr := httptest.NewRecorder()
	t.h.ServeHTTP(rr, req)
	resp := rr.Result()
	resp.Request = req
	return resp, nil
}

// email change stub
type stubEmailChangeStore struct {
	mu     sync.Mutex
	tokens map[string]struct {
		OldEmail string
		NewEmail string
		Expires  time.Time
	}
}

func newStubEmailChangeStore() *stubEmailChangeStore {
	return &stubEmailChangeStore{tokens: make(map[string]struct {
		OldEmail string
		NewEmail string
		Expires  time.Time
	})}
}

func (s *stubEmailChangeStore) SaveChange(ctx context.Context, email, newEmail, tokenHash string, expires time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[tokenHash] = struct {
		OldEmail string
		NewEmail string
		Expires  time.Time
	}{OldEmail: email, NewEmail: newEmail, Expires: expires}
	return nil
}

func (s *stubEmailChangeStore) ConsumeChange(ctx context.Context, tokenHash string, now time.Time) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	val, ok := s.tokens[tokenHash]
	if !ok || val.Expires.Before(now) {
		return "", "", errors.New("not found")
	}
	delete(s.tokens, tokenHash)
	return val.OldEmail, val.NewEmail, nil
}
