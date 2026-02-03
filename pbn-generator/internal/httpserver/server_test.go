package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"database/sql"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
)

func setupServer(t *testing.T) *Server {
	t.Helper()
	logger := zap.NewNop().Sugar()
	cfg := config.Config{
		Port:                 "0",
		AllowedOrigin:        "*",
		SessionTTL:           24 * time.Hour,
		AccessTTL:            15 * time.Minute,
		RefreshTTL:           7 * 24 * time.Hour,
		SessionCleanInterval: time.Minute,
		LoginRateLimit:       100,
		LoginRateWindow:      time.Minute,
		RegisterRateLimit:    100,
		RegisterRateWindow:   time.Minute,
		LoginEmailIpLimit:    100,
		LoginEmailIpWindow:   time.Minute,
		LoginLockoutFails:    2,
		LoginLockoutDuration: time.Minute,
		EmailVerificationTTL: 24 * time.Hour,
		PasswordResetTTL:     time.Hour,
	}
	svc := auth.NewService(auth.ServiceDeps{
		Config:             cfg,
		Users:              newStubUserStore(),
		Sessions:           newStubSessionStore(),
		VerificationTokens: newStubVerificationStore(),
		ResetTokens:        newStubResetStore(),
		Captchas:           newStubCaptchaStore(),
		Mailer:             stubMailer{},
		Logger:             logger,
	})
	proj := newStubProjectStore()
	dom := newStubDomainStore()
	gen := newStubGenerationStore()
	prompts := newStubPromptStore()
	return New(cfg, svc, logger, proj, dom, gen, prompts, newStubEnqueuer())
}

func TestRegisterAndLogin(t *testing.T) {
	s := setupServer(t)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(`{"email":"user@example.com","password":"Password123"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	s.handleRegister(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d, body: %s", http.StatusCreated, registerRec.Code, registerRec.Body.String())
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"user@example.com","password":"Password123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	s.handleLogin(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body: %s", http.StatusOK, loginRec.Code, loginRec.Body.String())
	}
	if getCookie(loginRec, "access_token") == "" {
		t.Fatalf("expected access cookie, got: %s", loginRec.Body.String())
	}
}

func TestRegisterRateLimit(t *testing.T) {
	s := setupServer(t)
	s.cfg.RegisterRateLimit = 1
	s.cfg.RegisterRateWindow = time.Hour
	s.svc = auth.NewService(auth.ServiceDeps{
		Config:   s.cfg,
		Users:    newStubUserStore(),
		Sessions: newStubSessionStore(),
	})

	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(`{"email":"a@b.com","password":"Password123"}`))
	req1.Header.Set("Content-Type", "application/json")
	s.handleRegister(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first request expected %d, got %d", http.StatusCreated, rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(`{"email":"b@b.com","password":"Password123"}`))
	req2.Header.Set("Content-Type", "application/json")
	s.handleRegister(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request expected rate limit %d, got %d", http.StatusTooManyRequests, rec2.Code)
	}
}

func TestLoginLockout(t *testing.T) {
	s := setupServer(t)
	s.cfg.LoginLockoutFails = 2
	s.cfg.LoginLockoutDuration = time.Minute
	s.cfg.LoginRateLimit = 100
	s.svc = auth.NewService(auth.ServiceDeps{
		Config:   s.cfg,
		Users:    newStubUserStore(),
		Sessions: newStubSessionStore(),
	})

	regReq := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(`{"email":"u@u.com","password":"Password123"}`))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	s.handleRegister(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register expected %d, got %d", http.StatusCreated, regRec.Code)
	}

	badLogin := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"u@u.com","password":"Wrongpass1"}`))
		req.Header.Set("Content-Type", "application/json")
		s.handleLogin(rec, req)
		return rec
	}

	if res := badLogin(); res.Code != http.StatusUnauthorized {
		t.Fatalf("first bad login expected %d, got %d", http.StatusUnauthorized, res.Code)
	}
	if res := badLogin(); res.Code != http.StatusUnauthorized {
		t.Fatalf("second bad login expected %d, got %d", http.StatusUnauthorized, res.Code)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"u@u.com","password":"Wrongpass1"}`))
	req.Header.Set("Content-Type", "application/json")
	s.handleLogin(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked login expected %d, got %d", http.StatusTooManyRequests, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "retryIn") {
		t.Fatalf("expected retryIn in body, got %s", rec.Body.String())
	}
}

func TestLogout(t *testing.T) {
	s := setupServer(t)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(`{"email":"user@example.com","password":"Password123"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	s.handleRegister(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", registerRec.Code, registerRec.Body.String())
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"user@example.com","password":"Password123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	s.handleLogin(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", loginRec.Code, loginRec.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logoutRec := httptest.NewRecorder()
	s.handleLogout(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout failed: %d %s", logoutRec.Code, logoutRec.Body.String())
	}

	// Повторный вызов без куки просто очищает состояние, 200
	logoutRec2 := httptest.NewRecorder()
	s.handleLogout(logoutRec2, logoutReq)
	if logoutRec2.Code != http.StatusOK {
		t.Fatalf("expected ok on repeated logout, got %d", logoutRec2.Code)
	}
}

func TestLogoutAll(t *testing.T) {
	s := setupServer(t)

	// register + login
	registerReq := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(`{"email":"user@example.com","password":"Password123"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	s.handleRegister(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", registerRec.Code, registerRec.Body.String())
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"user@example.com","password":"Password123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	s.handleLogin(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", loginRec.Code, loginRec.Body.String())
	}

	access := getCookie(loginRec, "access_token")
	req := httptest.NewRequest(http.MethodPost, "/api/logout-all", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: access})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout-all failed: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRoleSecurity(t *testing.T) {
	s := setupServer(t)

	// Создаем двух админов
	admin1Ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin1@example.com",
		Role:  "admin",
	})
	admin2Ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin2@example.com",
		Role:  "admin",
	})

	// Создаем пользователей через stub
	store := s.svc.(*auth.Service).Users.(*stubUserStore)
	store.users["admin1@example.com"] = "pass"
	store.roles["admin1@example.com"] = "admin"
	store.users["admin2@example.com"] = "pass"
	store.roles["admin2@example.com"] = "admin"
	store.users["manager@example.com"] = "pass"
	store.roles["manager@example.com"] = "manager"

	handler := http.HandlerFunc(s.handleAdminUserRoute)

	// Тест 1: Админ не может изменить свою роль
	req1 := httptest.NewRequest(http.MethodPatch, "/api/admin/users/admin1@example.com",
		strings.NewReader(`{"role":"manager"}`)).WithContext(admin1Ctx)
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when admin tries to change own role, got %d: %s", rec1.Code, rec1.Body.String())
	}
	if !strings.Contains(rec1.Body.String(), "cannot change your own role") {
		t.Fatalf("expected error message about own role, got: %s", rec1.Body.String())
	}

	// Тест 2: Админ не может понизить другого админа
	req2 := httptest.NewRequest(http.MethodPatch, "/api/admin/users/admin2@example.com",
		strings.NewReader(`{"role":"manager"}`)).WithContext(admin1Ctx)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when admin tries to change another admin role, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), "cannot change admin role") {
		t.Fatalf("expected error message about admin role, got: %s", rec2.Body.String())
	}

	// Тест 3: Админ может изменить роль менеджера
	req3 := httptest.NewRequest(http.MethodPatch, "/api/admin/users/manager@example.com",
		strings.NewReader(`{"role":"admin"}`)).WithContext(admin1Ctx)
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 when admin changes manager role, got %d: %s", rec3.Code, rec3.Body.String())
	}

	// Тест 4: Валидация недопустимых ролей
	req4 := httptest.NewRequest(http.MethodPatch, "/api/admin/users/manager@example.com",
		strings.NewReader(`{"role":"superadmin"}`)).WithContext(admin1Ctx)
	req4.Header.Set("Content-Type", "application/json")
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid role, got %d: %s", rec4.Code, rec4.Body.String())
	}
	if !strings.Contains(rec4.Body.String(), "invalid role") {
		t.Fatalf("expected error message about invalid role, got: %s", rec4.Body.String())
	}
}

func TestAdminPromptCRUD(t *testing.T) {
	s := setupServer(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	list := http.HandlerFunc(s.handleAdminPrompts)
	detail := http.HandlerFunc(s.handleAdminPromptByID)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/prompts", nil).WithContext(adminCtx)
	rec := httptest.NewRecorder()
	list.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("expected empty list, got %d %s", rec.Code, rec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/prompts", strings.NewReader(`{"name":"Default","body":"Prompt body","isActive":true}`)).WithContext(adminCtx)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	list.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", createRec.Code, createRec.Body.String())
	}
	var created adminPromptDTO
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("missing id")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/prompts/"+created.ID, nil).WithContext(adminCtx)
	getRec := httptest.NewRecorder()
	detail.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get failed: %d %s", getRec.Code, getRec.Body.String())
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/admin/prompts/"+created.ID, strings.NewReader(`{"description":"desc","isActive":false}`)).WithContext(adminCtx)
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	detail.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch failed: %d %s", patchRec.Code, patchRec.Body.String())
	}
	var updated adminPromptDTO
	if err := json.NewDecoder(patchRec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated.Description == nil || *updated.Description != "desc" || updated.IsActive {
		t.Fatalf("unexpected updated prompt %+v", updated)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/admin/prompts/"+created.ID, nil).WithContext(adminCtx)
	delRec := httptest.NewRecorder()
	detail.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete failed: %d %s", delRec.Code, delRec.Body.String())
	}
}

func TestMe(t *testing.T) {
	s := setupServer(t)

	regReq := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(`{"email":"user@example.com","password":"Password123"}`))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	s.handleRegister(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", regRec.Code, regRec.Body.String())
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"user@example.com","password":"Password123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	s.handleLogin(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", loginRec.Code, loginRec.Body.String())
	}

	ctx := context.WithValue(context.Background(), userEmailContextKey, "user@example.com")
	ctx = context.WithValue(ctx, currentUserContextKey, auth.User{Email: "user@example.com"})
	meReq := httptest.NewRequest(http.MethodGet, "/api/me", nil).WithContext(ctx)
	meRec := httptest.NewRecorder()
	s.handleMe(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me failed: %d %s", meRec.Code, meRec.Body.String())
	}
	if !strings.Contains(meRec.Body.String(), `"email":"user@example.com"`) {
		t.Fatalf("me response missing email: %s", meRec.Body.String())
	}
}

func TestChangePassword(t *testing.T) {
	s := setupServer(t)

	// register
	regReq := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(`{"email":"user@example.com","password":"Password123"}`))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	s.handleRegister(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", regRec.Code, regRec.Body.String())
	}

	// login
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"user@example.com","password":"Password123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	s.handleLogin(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", loginRec.Code, loginRec.Body.String())
	}

	// change password
	changeBody := `{"currentPassword":"Password123","newPassword":"NewPassword123"}`
	changeReq := httptest.NewRequest(http.MethodPost, "/api/password", strings.NewReader(changeBody))
	changeReq.Header.Set("Content-Type", "application/json")
	changeReq = changeReq.WithContext(context.WithValue(context.Background(), userEmailContextKey, "user@example.com"))
	changeRec := httptest.NewRecorder()
	s.handleChangePassword(changeRec, changeReq)
	if changeRec.Code != http.StatusOK {
		t.Fatalf("change password failed: %d %s", changeRec.Code, changeRec.Body.String())
	}

	// old password should fail
	oldLoginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"user@example.com","password":"Password123"}`))
	oldLoginReq.Header.Set("Content-Type", "application/json")
	oldLoginRec := httptest.NewRecorder()
	s.handleLogin(oldLoginRec, oldLoginReq)
	if oldLoginRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password unauthorized, got %d", oldLoginRec.Code)
	}

	// new password works
	newLoginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"user@example.com","password":"NewPassword123"}`))
	newLoginReq.Header.Set("Content-Type", "application/json")
	newLoginRec := httptest.NewRecorder()
	s.handleLogin(newLoginRec, newLoginReq)
	if newLoginRec.Code != http.StatusOK {
		t.Fatalf("new password login failed: %d %s", newLoginRec.Code, newLoginRec.Body.String())
	}
}

func extractToken(t *testing.T, body string) string {
	t.Helper()
	idx := strings.Index(body, `"token":"`)
	if idx == -1 {
		t.Fatalf("token not found in response: %s", body)
	}
	rest := body[idx+9:]
	end := strings.Index(rest, `"`)
	if end == -1 {
		t.Fatalf("token closing quote not found")
	}
	return rest[:end]
}

func getCookie(rec *httptest.ResponseRecorder, name string) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// Тестовые стораджи на мапах для хендлеров

type stubUserStore struct {
	mu       sync.Mutex
	users    map[string]string // email -> password
	verified map[string]bool
	roles    map[string]string
	approved map[string]bool
}

func newStubUserStore() *stubUserStore {
	return &stubUserStore{
		users:    make(map[string]string),
		verified: make(map[string]bool),
		roles:    make(map[string]string),
		approved: make(map[string]bool),
	}
}

func (s *stubUserStore) Create(ctx context.Context, email, password string) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; ok {
		return auth.User{}, errors.New("user already exists")
	}
	s.users[email] = password
	s.verified[email] = true
	s.roles[email] = "manager"
	s.approved[email] = true
	return auth.User{Email: email, CreatedAt: time.Now().UTC(), Verified: s.verified[email], Role: s.roles[email], IsApproved: s.approved[email]}, nil
}

func (s *stubUserStore) Authenticate(ctx context.Context, email, password string) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.users[email]
	if !ok || stored != password {
		return auth.User{}, errors.New("invalid email or password")
	}
	return auth.User{Email: email, CreatedAt: time.Now().UTC(), Verified: s.verified[email], Role: s.roles[email], IsApproved: s.approved[email]}, nil
}

func (s *stubUserStore) UpdatePassword(ctx context.Context, email, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("not found")
	}
	s.users[email] = newPassword
	return nil
}

func (s *stubUserStore) SetVerified(ctx context.Context, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("not found")
	}
	s.verified[email] = true
	return nil
}

func (s *stubUserStore) Get(ctx context.Context, email string) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pw, ok := s.users[email]
	if !ok {
		return auth.User{}, errors.New("user not found")
	}
	return auth.User{Email: email, CreatedAt: time.Now().UTC(), PasswordHash: []byte(pw), Verified: s.verified[email], Role: s.roles[email], IsApproved: s.approved[email]}, nil
}

func (s *stubUserStore) UpdateProfile(ctx context.Context, email, name, avatarURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("not found")
	}
	// Profile data not tracked in stub for simplicity
	return nil
}

func (s *stubUserStore) ChangeEmail(ctx context.Context, oldEmail, newEmail string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pw, ok := s.users[oldEmail]
	if !ok {
		return errors.New("not found")
	}
	delete(s.users, oldEmail)
	delete(s.verified, oldEmail)
	delete(s.roles, oldEmail)
	delete(s.approved, oldEmail)
	s.users[newEmail] = pw
	s.verified[newEmail] = true
	s.roles[newEmail] = "manager"
	s.approved[newEmail] = true
	return nil
}

func (s *stubUserStore) List(ctx context.Context) ([]auth.User, error) {
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

func (s *stubUserStore) UpdateRole(ctx context.Context, email, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("not found")
	}
	s.roles[email] = role
	return nil
}

func (s *stubUserStore) SetApproved(ctx context.Context, email string, approved bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("not found")
	}
	s.approved[email] = approved
	return nil
}

type stubSessionStore struct {
	mu        sync.Mutex
	sessions  map[string]auth.Session
	lastJTI   string
	lastEmail string
}

func newStubSessionStore() *stubSessionStore {
	return &stubSessionStore{sessions: make(map[string]auth.Session)}
}

func (s *stubSessionStore) Create(ctx context.Context, session auth.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.JTI] = session
	s.lastJTI = session.JTI
	s.lastEmail = session.Email
	return nil
}

func (s *stubSessionStore) Get(ctx context.Context, jti string) (auth.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[jti]
	if !ok {
		if s.lastJTI == jti && s.lastEmail != "" {
			sess = auth.Session{JTI: jti, Email: s.lastEmail, ExpiresAt: time.Now().Add(time.Hour)}
			s.sessions[jti] = sess
		} else {
			return auth.Session{}, errors.New("not found")
		}
	}
	return sess, nil
}

func (s *stubSessionStore) Delete(ctx context.Context, jti string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, jti)
	return nil
}

func (s *stubSessionStore) CleanupExpired(ctx context.Context, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.sessions {
		if v.ExpiresAt.Before(now) {
			delete(s.sessions, k)
		}
	}
	return nil
}

func (s *stubSessionStore) RevokeByEmail(ctx context.Context, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.sessions {
		if v.Email == email {
			delete(s.sessions, k)
		}
	}
	return nil
}

func (s *stubSessionStore) RevokeAll(ctx context.Context, email string) error {
	return s.RevokeByEmail(ctx, email)
}

func (s *stubSessionStore) RevokeAllExcept(ctx context.Context, email, keepJTI string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.sessions {
		if v.Email == email && k != keepJTI {
			delete(s.sessions, k)
		}
	}
	return nil
}

func (s *stubSessionStore) UpdateEmail(ctx context.Context, oldEmail, newEmail string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.sessions {
		if v.Email == oldEmail {
			v.Email = newEmail
			s.sessions[k] = v
		}
	}
	return nil
}

type stubVerificationStore struct {
	mu     sync.Mutex
	tokens map[string]struct {
		Email   string
		Expires time.Time
	}
}

func newStubVerificationStore() *stubVerificationStore {
	return &stubVerificationStore{tokens: make(map[string]struct {
		Email   string
		Expires time.Time
	})}
}

func (s *stubVerificationStore) SaveVerification(ctx context.Context, email, tokenHash string, expires time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[tokenHash] = struct {
		Email   string
		Expires time.Time
	}{Email: email, Expires: expires}
	return nil
}

func (s *stubVerificationStore) ConsumeVerification(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	val, ok := s.tokens[tokenHash]
	if !ok || val.Expires.Before(now) {
		return "", errors.New("not found")
	}
	delete(s.tokens, tokenHash)
	return val.Email, nil
}

type stubResetStore struct {
	mu     sync.Mutex
	tokens map[string]struct {
		Email   string
		Expires time.Time
	}
}

func newStubResetStore() *stubResetStore {
	return &stubResetStore{tokens: make(map[string]struct {
		Email   string
		Expires time.Time
	})}
}

func (s *stubResetStore) SaveReset(ctx context.Context, email, tokenHash string, expires time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[tokenHash] = struct {
		Email   string
		Expires time.Time
	}{Email: email, Expires: expires}
	return nil
}

func (s *stubResetStore) ConsumeReset(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	val, ok := s.tokens[tokenHash]
	if !ok || val.Expires.Before(now) {
		return "", errors.New("not found")
	}
	delete(s.tokens, tokenHash)
	return val.Email, nil
}

type stubMailer struct{}

func (stubMailer) Send(ctx context.Context, to, subject, body string) error {
	return nil
}

type stubCaptchaStore struct {
	mu    sync.Mutex
	items map[string]struct {
		Hash    string
		Expires time.Time
	}
}

func newStubCaptchaStore() *stubCaptchaStore {
	return &stubCaptchaStore{items: make(map[string]struct {
		Hash    string
		Expires time.Time
	})}
}

func (s *stubCaptchaStore) Save(ctx context.Context, id, answerHash string, expires time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[id] = struct {
		Hash    string
		Expires time.Time
	}{Hash: answerHash, Expires: expires}
	return nil
}

func (s *stubCaptchaStore) Consume(ctx context.Context, id, answerHash string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	val, ok := s.items[id]
	if !ok || val.Expires.Before(now) || val.Hash != answerHash {
		return errors.New("not found")
	}
	delete(s.items, id)
	return nil
}

// projects/domains/generations stubs
type stubProjectStore struct {
	mu       sync.Mutex
	projects map[string]sqlstore.Project
}

func newStubProjectStore() *stubProjectStore {
	return &stubProjectStore{projects: make(map[string]sqlstore.Project)}
}

func (s *stubProjectStore) Create(ctx context.Context, p sqlstore.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects[p.ID] = p
	return nil
}

func (s *stubProjectStore) ListByUser(ctx context.Context, email string) ([]sqlstore.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []sqlstore.Project
	for _, p := range s.projects {
		if p.UserEmail == email {
			res = append(res, p)
		}
	}
	return res, nil
}

func (s *stubProjectStore) ListAll(ctx context.Context) ([]sqlstore.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []sqlstore.Project
	for _, p := range s.projects {
		res = append(res, p)
	}
	return res, nil
}

func (s *stubProjectStore) Get(ctx context.Context, id, email string) (sqlstore.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.projects[id]
	if !ok || p.UserEmail != email {
		return sqlstore.Project{}, errors.New("not found")
	}
	return p, nil
}

func (s *stubProjectStore) GetByID(ctx context.Context, id string) (sqlstore.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.projects[id]
	if !ok {
		return sqlstore.Project{}, errors.New("not found")
	}
	return p, nil
}

func (s *stubProjectStore) Update(ctx context.Context, p sqlstore.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects[p.ID] = p
	return nil
}

func (s *stubProjectStore) Delete(ctx context.Context, id, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.projects[id]; ok && p.UserEmail == email {
		delete(s.projects, id)
		return nil
	}
	return errors.New("not found")
}

type stubDomainStore struct {
	mu      sync.Mutex
	domains map[string]sqlstore.Domain
}

func newStubDomainStore() *stubDomainStore {
	return &stubDomainStore{domains: make(map[string]sqlstore.Domain)}
}

func (s *stubDomainStore) Create(ctx context.Context, d sqlstore.Domain) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.domains[d.ID] = d
	return nil
}

func (s *stubDomainStore) ListByProject(ctx context.Context, projectID string) ([]sqlstore.Domain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []sqlstore.Domain
	for _, d := range s.domains {
		if d.ProjectID == projectID {
			res = append(res, d)
		}
	}
	return res, nil
}

func (s *stubDomainStore) Get(ctx context.Context, id string) (sqlstore.Domain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.domains[id]
	if !ok {
		return sqlstore.Domain{}, errors.New("not found")
	}
	return d, nil
}

func (s *stubDomainStore) UpdateStatus(ctx context.Context, id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.domains[id]; ok {
		d.Status = status
		s.domains[id] = d
		return nil
	}
	return errors.New("not found")
}

func (s *stubDomainStore) UpdateKeyword(ctx context.Context, id, keyword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.domains[id]; ok {
		d.MainKeyword = keyword
		s.domains[id] = d
		return nil
	}
	return errors.New("not found")
}

func (s *stubDomainStore) UpdateExtras(ctx context.Context, id, country, language string, exclude, server sql.NullString) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.domains[id]; ok {
		d.TargetCountry = country
		d.TargetLanguage = language
		d.ExcludeDomains = exclude
		d.ServerID = server
		s.domains[id] = d
		return true, nil
	}
	return false, errors.New("not found")
}

func (s *stubDomainStore) EnsureDefaultServer(ctx context.Context, email string) error { return nil }

func (s *stubDomainStore) SetLastGeneration(ctx context.Context, id, genID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.domains[id]; ok {
		d.LastGenerationID = sqlstore.NullableString(genID)
		s.domains[id] = d
		return nil
	}
	return errors.New("not found")
}

func (s *stubDomainStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.domains, id)
	return nil
}

type stubGenerationStore struct {
	mu          sync.Mutex
	generations map[string]sqlstore.Generation
}

func newStubGenerationStore() *stubGenerationStore {
	return &stubGenerationStore{generations: make(map[string]sqlstore.Generation)}
}

func (s *stubGenerationStore) Get(ctx context.Context, id string) (sqlstore.Generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.generations[id]; ok {
		return g, nil
	}
	return sqlstore.Generation{}, errors.New("not found")
}

type stubEnqueuer struct{}

func newStubEnqueuer() tasks.Enqueuer { return &stubEnqueuer{} }

func (s *stubEnqueuer) Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return &asynq.TaskInfo{ID: "stub"}, nil
}

func (s *stubGenerationStore) Create(ctx context.Context, g sqlstore.Generation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generations[g.ID] = g
	return nil
}

func (s *stubGenerationStore) ListByDomain(ctx context.Context, domainID string) ([]sqlstore.Generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []sqlstore.Generation
	for _, g := range s.generations {
		if g.DomainID == domainID {
			res = append(res, g)
		}
	}
	return res, nil
}

func (s *stubGenerationStore) ListRecentByUser(ctx context.Context, email string, limit int) ([]sqlstore.Generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]sqlstore.Generation, 0, len(s.generations))
	for _, g := range s.generations {
		res = append(res, g)
	}
	if len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

func (s *stubGenerationStore) ListRecentAll(ctx context.Context, limit int) ([]sqlstore.Generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]sqlstore.Generation, 0, len(s.generations))
	for _, g := range s.generations {
		res = append(res, g)
	}
	if len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

func (s *stubGenerationStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.generations, id)
	return nil
}

func (s *stubGenerationStore) CountsByStatus(ctx context.Context) (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make(map[string]int)
	for _, g := range s.generations {
		res[g.Status]++
	}
	return res, nil
}

func (s *stubGenerationStore) UpdateStatus(ctx context.Context, id, status string, progress int, errText *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.generations[id]; ok {
		g.Status = status
		g.Progress = progress
		if errText != nil {
			g.Error = sql.NullString{String: *errText, Valid: true}
		}
		s.generations[id] = g
		return nil
	}
	return errors.New("not found")
}

func (s *stubGenerationStore) UpdateFull(ctx context.Context, id, status string, progress int, errText *string, logs, artifacts []byte, started, finished *time.Time, promptID *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.generations[id]; ok {
		g.Status = status
		g.Progress = progress
		if errText != nil {
			g.Error = sqlstore.NullableString(*errText)
		}
		if len(logs) > 0 {
			g.Logs = logs
		}
		if len(artifacts) > 0 {
			g.Artifacts = artifacts
		}
		if started != nil {
			g.StartedAt = sql.NullTime{Time: *started, Valid: true}
		}
		if finished != nil {
			g.FinishedAt = sql.NullTime{Time: *finished, Valid: true}
		}
		if promptID != nil {
			g.PromptID = sqlstore.NullableString(*promptID)
		}
		s.generations[id] = g
		return nil
	}
	return errors.New("not found")
}

type stubPromptStore struct {
	mu      sync.Mutex
	prompts map[string]sqlstore.SystemPrompt
}

func newStubPromptStore() *stubPromptStore {
	return &stubPromptStore{prompts: make(map[string]sqlstore.SystemPrompt)}
}

func (s *stubPromptStore) Create(ctx context.Context, p sqlstore.SystemPrompt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = p.CreatedAt
	}
	s.prompts[p.ID] = p
	return nil
}

func (s *stubPromptStore) Update(ctx context.Context, p sqlstore.SystemPrompt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.prompts[p.ID]; !ok {
		return sql.ErrNoRows
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = time.Now()
	}
	s.prompts[p.ID] = p
	return nil
}

func (s *stubPromptStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.prompts[id]; !ok {
		return sql.ErrNoRows
	}
	delete(s.prompts, id)
	return nil
}

func (s *stubPromptStore) List(ctx context.Context) ([]sqlstore.SystemPrompt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]sqlstore.SystemPrompt, 0, len(s.prompts))
	for _, p := range s.prompts {
		res = append(res, p)
	}
	return res, nil
}

func (s *stubPromptStore) Get(ctx context.Context, id string) (sqlstore.SystemPrompt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.prompts[id]
	if !ok {
		return sqlstore.SystemPrompt{}, sql.ErrNoRows
	}
	return p, nil
}
