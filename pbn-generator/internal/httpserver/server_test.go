package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
	schedules := newStubScheduleStore()
	linkSchedules := newStubLinkScheduleStore()
	siteFiles := newStubSiteFileStore()
	fileEdits := newStubFileEditStore()
	linkTasks := newStubLinkTaskStore()
	genQueue := newStubGenQueueStore()
	indexChecks := newStubIndexCheckStore()
	checkHistory := newStubCheckHistoryStore()
	return New(cfg, svc, logger, proj, nil, dom, gen, prompts, schedules, linkSchedules, nil, siteFiles, fileEdits, linkTasks, genQueue, indexChecks, checkHistory, newStubEnqueuer())
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
	users := newStubUserStore()
	s.svc = auth.NewService(auth.ServiceDeps{
		Config:             s.cfg,
		Users:              users,
		Sessions:           newStubSessionStore(),
		VerificationTokens: newStubVerificationStore(),
		ResetTokens:        newStubResetStore(),
		Captchas:           newStubCaptchaStore(),
		Mailer:             stubMailer{},
		Logger:             zap.NewNop().Sugar(),
	})

	// Создаем двух админов
	admin1Ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin1@example.com",
		Role:  "admin",
	})

	// Создаем пользователей через stub
	users.users["admin1@example.com"] = "pass"
	users.roles["admin1@example.com"] = "admin"
	users.users["admin2@example.com"] = "pass"
	users.roles["admin2@example.com"] = "admin"
	users.users["manager@example.com"] = "pass"
	users.roles["manager@example.com"] = "manager"
	users.users["manager2@example.com"] = "pass"
	users.roles["manager2@example.com"] = "manager"

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
	req4 := httptest.NewRequest(http.MethodPatch, "/api/admin/users/manager2@example.com",
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
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", delRec.Code, delRec.Body.String())
	}
}

func TestAdminCanGenerateWithoutMembership(t *testing.T) {
	s := setupServer(t)
	cfg := s.cfg
	cfg.APIKeySecret = "test-secret-key"
	users := newStubUserStore()
	s.svc = auth.NewService(auth.ServiceDeps{
		Config:             cfg,
		Users:              users,
		Sessions:           newStubSessionStore(),
		VerificationTokens: newStubVerificationStore(),
		ResetTokens:        newStubResetStore(),
		Captchas:           newStubCaptchaStore(),
		Mailer:             stubMailer{},
		Logger:             zap.NewNop().Sugar(),
	})

	const (
		projectID = "project-1"
		domainID  = "domain-1"
		owner     = "owner@example.com"
	)

	users.users[owner] = "pass"
	users.verified[owner] = true
	users.roles[owner] = "manager"
	users.approved[owner] = true
	users.users["admin@example.com"] = "pass"
	users.verified["admin@example.com"] = true
	users.roles["admin@example.com"] = "admin"
	users.approved["admin@example.com"] = true
	if err := users.SetAPIKey(context.Background(), "admin@example.com", []byte("enc-key"), time.Now().UTC()); err != nil {
		t.Fatalf("set admin api key: %v", err)
	}
	if err := users.SetAPIKey(context.Background(), owner, []byte("enc-key"), time.Now().UTC()); err != nil {
		t.Fatalf("set api key: %v", err)
	}

	projStore, ok := s.projects.(*stubProjectStore)
	if !ok {
		t.Fatalf("unexpected project store type")
	}
	projStore.projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: owner,
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	domainStore, ok := s.domains.(*stubDomainStore)
	if !ok {
		t.Fatalf("unexpected domain store type")
	}
	domainStore.domains[domainID] = sqlstore.Domain{
		ID:          domainID,
		ProjectID:   projectID,
		URL:         "kundservice.net",
		MainKeyword: "keyword",
		Status:      "waiting",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/generate", strings.NewReader(`{}`)).WithContext(adminCtx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleDomainGenerate(rec, req, domainID)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for admin generate, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetProjectMemberRole_AdminBypass(t *testing.T) {
	s := setupServer(t)
	projStore, ok := s.projects.(*stubProjectStore)
	if !ok {
		t.Fatalf("unexpected project store type")
	}
	projectID := "project-admin-1"
	projStore.projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	role, err := s.getProjectMemberRole(ctx, projectID, "admin@example.com")
	if err != nil {
		t.Fatalf("expected no error for admin, got: %v", err)
	}
	if role != "admin" {
		t.Fatalf("expected admin role, got %q", role)
	}
}

func TestProjectSummaryIncludesMyRoleOwner(t *testing.T) {
	s := setupServer(t)
	projectID := "project-summary-owner"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Owner Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "owner@example.com",
		Role:  "manager",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/summary", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleProjectSummary(rec, req, projectID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp projectSummaryDTO
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.MyRole != "owner" {
		t.Fatalf("expected my_role=owner, got %q", resp.MyRole)
	}
}

func TestProjectSummaryIncludesMyRoleAdmin(t *testing.T) {
	s := setupServer(t)
	projectID := "project-summary-admin"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Admin Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/summary", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleProjectSummary(rec, req, projectID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp projectSummaryDTO
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.MyRole != "admin" {
		t.Fatalf("expected my_role=admin, got %q", resp.MyRole)
	}
}

func TestDomainSummaryIncludesMyRoleEditor(t *testing.T) {
	s := setupServer(t)
	projectID := "project-summary-editor"
	domainID := "domain-summary-editor"
	memberEmail := "editor@example.com"
	now := time.Now().UTC()

	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Editor Project",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.domains.(*stubDomainStore).domains[domainID] = sqlstore.Domain{
		ID:             domainID,
		ProjectID:      projectID,
		URL:            "example.com",
		Status:         "published",
		PublishedAt:    sql.NullTime{Time: now, Valid: true},
		PublishedPath:  sql.NullString{String: "/server/example.com/", Valid: true},
		FileCount:      15,
		TotalSizeBytes: 1024,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	s.projectMembers = sqlstore.NewProjectMemberStore(db)

	rows := sqlmock.NewRows([]string{"project_id", "user_email", "role", "created_at"}).
		AddRow(projectID, memberEmail, "editor", now)
	query := regexp.QuoteMeta("SELECT project_id, user_email, role, created_at FROM project_members WHERE project_id=$1 AND user_email=$2")
	mock.ExpectQuery(query).WithArgs(projectID, memberEmail).WillReturnRows(rows)
	rows2 := sqlmock.NewRows([]string{"project_id", "user_email", "role", "created_at"}).
		AddRow(projectID, memberEmail, "editor", now)
	mock.ExpectQuery(query).WithArgs(projectID, memberEmail).WillReturnRows(rows2)

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: memberEmail,
		Role:  "manager",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/summary", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleDomainSummary(rec, req, domainID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domainSummaryDTO
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.MyRole != "editor" {
		t.Fatalf("expected my_role=editor, got %q", resp.MyRole)
	}
	if resp.Domain.PublishedPath == nil || *resp.Domain.PublishedPath != "/server/example.com/" {
		t.Fatalf("expected published_path in domain summary")
	}
	if resp.Domain.FileCount != 15 {
		t.Fatalf("expected file_count=15, got %d", resp.Domain.FileCount)
	}
	if resp.Domain.TotalSizeBytes != 1024 {
		t.Fatalf("expected total_size_bytes=1024, got %d", resp.Domain.TotalSizeBytes)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestAdminGenerateUsesAdminAPIKey(t *testing.T) {
	s := setupServer(t)
	cfg := s.cfg
	cfg.APIKeySecret = "test-secret-key"
	users := newStubUserStore()
	s.svc = auth.NewService(auth.ServiceDeps{
		Config:             cfg,
		Users:              users,
		Sessions:           newStubSessionStore(),
		VerificationTokens: newStubVerificationStore(),
		ResetTokens:        newStubResetStore(),
		Captchas:           newStubCaptchaStore(),
		Mailer:             stubMailer{},
		Logger:             zap.NewNop().Sugar(),
	})

	const (
		projectID = "project-admin-key"
		domainID  = "domain-admin-key"
		owner     = "owner@example.com"
		admin     = "admin@example.com"
	)

	users.users[owner] = "pass"
	users.verified[owner] = true
	users.roles[owner] = "manager"
	users.approved[owner] = true

	users.users[admin] = "pass"
	users.verified[admin] = true
	users.roles[admin] = "admin"
	users.approved[admin] = true
	if err := users.SetAPIKey(context.Background(), admin, []byte("enc-key"), time.Now().UTC()); err != nil {
		t.Fatalf("set admin api key: %v", err)
	}

	projStore, ok := s.projects.(*stubProjectStore)
	if !ok {
		t.Fatalf("unexpected project store type")
	}
	projStore.projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: owner,
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	domainStore, ok := s.domains.(*stubDomainStore)
	if !ok {
		t.Fatalf("unexpected domain store type")
	}
	domainStore.domains[domainID] = sqlstore.Domain{
		ID:          domainID,
		ProjectID:   projectID,
		URL:         "kundservice.net",
		MainKeyword: "keyword",
		Status:      "waiting",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: admin,
		Role:  "admin",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/generate", strings.NewReader(`{}`)).WithContext(adminCtx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleDomainGenerate(rec, req, domainID)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for admin generate with admin api key, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDomainPatchLinkSettings(t *testing.T) {
	s := setupServer(t)

	projectID := "project-link"
	domainID := "domain-link"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.domains.(*stubDomainStore).domains[domainID] = sqlstore.Domain{
		ID:        domainID,
		ProjectID: projectID,
		URL:       "example.com",
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	body := `{"link_anchor_text":"My Anchor","link_acceptor_url":"https://target.example"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/domains/"+domainID, strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleDomainBase(rec, req, domainID)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch domain status: %d %s", rec.Code, rec.Body.String())
	}

	var resp domainDTO
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.LinkAnchorText == nil || *resp.LinkAnchorText != "My Anchor" {
		t.Fatalf("unexpected link_anchor_text: %#v", resp.LinkAnchorText)
	}
	if resp.LinkAcceptorURL == nil || *resp.LinkAcceptorURL != "https://target.example" {
		t.Fatalf("unexpected link_acceptor_url: %#v", resp.LinkAcceptorURL)
	}

	stored := s.domains.(*stubDomainStore).domains[domainID]
	if !stored.LinkAnchorText.Valid || stored.LinkAnchorText.String != "My Anchor" {
		t.Fatalf("stored link_anchor_text mismatch: %#v", stored.LinkAnchorText)
	}
	if !stored.LinkAcceptorURL.Valid || stored.LinkAcceptorURL.String != "https://target.example" {
		t.Fatalf("stored link_acceptor_url mismatch: %#v", stored.LinkAcceptorURL)
	}
}

func TestDomainPatchLinkSettingsInvalidBody(t *testing.T) {
	s := setupServer(t)

	projectID := "project-link-invalid"
	domainID := "domain-link-invalid"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.domains.(*stubDomainStore).domains[domainID] = sqlstore.Domain{
		ID:        domainID,
		ProjectID: projectID,
		URL:       "example.com",
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/domains/"+domainID, strings.NewReader(`{"link_anchor_text":`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleDomainBase(rec, req, domainID)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestDomainLinkRunCreatesTask(t *testing.T) {
	s := setupServer(t)

	projectID := "project-link-run"
	domainID := "domain-link-run"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.domains.(*stubDomainStore).domains[domainID] = sqlstore.Domain{
		ID:              domainID,
		ProjectID:       projectID,
		URL:             "example.com",
		Status:          "waiting",
		LinkAnchorText:  sqlstore.NullableString("Anchor"),
		LinkAcceptorURL: sqlstore.NullableString("https://target.example"),
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/link/run", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp linkTaskDTO
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AnchorText != "Anchor" || resp.TargetURL != "https://target.example" {
		t.Fatalf("unexpected task data: %+v", resp)
	}
	if resp.Status != "pending" {
		t.Fatalf("expected pending status, got %s", resp.Status)
	}
}

func TestDomainLinkRunUpsertsActiveTask(t *testing.T) {
	s := setupServer(t)

	projectID := "project-link-upsert"
	domainID := "domain-link-upsert"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.domains.(*stubDomainStore).domains[domainID] = sqlstore.Domain{
		ID:              domainID,
		ProjectID:       projectID,
		URL:             "example.com",
		Status:          "waiting",
		LinkAnchorText:  sqlstore.NullableString("New Anchor"),
		LinkAcceptorURL: sqlstore.NullableString("https://new.example"),
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	s.linkTasks.(*stubLinkTaskStore).tasks["task-1"] = sqlstore.LinkTask{
		ID:           "task-1",
		DomainID:     domainID,
		AnchorText:   "Old Anchor",
		TargetURL:    "https://old.example",
		ScheduledFor: time.Now().Add(-time.Hour),
		Action:       "insert",
		Status:       "searching",
		Attempts:     2,
		CreatedBy:    "user@example.com",
		CreatedAt:    time.Now().Add(-2 * time.Hour),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/link/run", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	task := s.linkTasks.(*stubLinkTaskStore).tasks["task-1"]
	if task.AnchorText != "New Anchor" || task.TargetURL != "https://new.example" {
		t.Fatalf("task not updated: %+v", task)
	}
	if task.Status != "pending" || task.Attempts != 0 {
		t.Fatalf("task status/attempts not reset: %+v", task)
	}
}

func TestDomainLinkRunMissingSettings(t *testing.T) {
	s := setupServer(t)

	projectID := "project-link-missing"
	domainID := "domain-link-missing"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.domains.(*stubDomainStore).domains[domainID] = sqlstore.Domain{
		ID:        domainID,
		ProjectID: projectID,
		URL:       "example.com",
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/link/run", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGenerationAction_PausePending(t *testing.T) {
	s := setupServer(t)

	projectID := "project-pause"
	domainID := "domain-pause"
	genID := "gen-pause"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.domains.(*stubDomainStore).domains[domainID] = sqlstore.Domain{
		ID:          domainID,
		ProjectID:   projectID,
		URL:         "example.com",
		MainKeyword: "keyword",
		Status:      "processing",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	s.generations.(*stubGenerationStore).generations[genID] = sqlstore.Generation{
		ID:        genID,
		DomainID:  domainID,
		Status:    "pending",
		Progress:  0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/generations/"+genID, strings.NewReader(`{"action":"pause"}`)).WithContext(adminCtx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleGenerationAction(rec, req, genID)
	if rec.Code != http.StatusOK {
		t.Fatalf("pause pending status: %d %s", rec.Code, rec.Body.String())
	}
	gen := s.generations.(*stubGenerationStore).generations[genID]
	if gen.Status != "paused" {
		t.Fatalf("expected paused status, got %s", gen.Status)
	}
	domain := s.domains.(*stubDomainStore).domains[domainID]
	if domain.Status != "waiting" {
		t.Fatalf("expected domain waiting, got %s", domain.Status)
	}
}

func TestGenerationAction_CancelPending(t *testing.T) {
	s := setupServer(t)

	projectID := "project-cancel"
	domainID := "domain-cancel"
	genID := "gen-cancel"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.domains.(*stubDomainStore).domains[domainID] = sqlstore.Domain{
		ID:          domainID,
		ProjectID:   projectID,
		URL:         "example.com",
		MainKeyword: "keyword",
		Status:      "processing",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	s.generations.(*stubGenerationStore).generations[genID] = sqlstore.Generation{
		ID:        genID,
		DomainID:  domainID,
		Status:    "pending",
		Progress:  0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/generations/"+genID, strings.NewReader(`{"action":"cancel"}`)).WithContext(adminCtx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleGenerationAction(rec, req, genID)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel pending status: %d %s", rec.Code, rec.Body.String())
	}
	gen := s.generations.(*stubGenerationStore).generations[genID]
	if gen.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %s", gen.Status)
	}
	domain := s.domains.(*stubDomainStore).domains[domainID]
	if domain.Status != "waiting" {
		t.Fatalf("expected domain waiting, got %s", domain.Status)
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
	apiKeys  map[string][]byte
	apiKeyAt map[string]time.Time
}

func newStubUserStore() *stubUserStore {
	return &stubUserStore{
		users:    make(map[string]string),
		verified: make(map[string]bool),
		roles:    make(map[string]string),
		approved: make(map[string]bool),
		apiKeys:  make(map[string][]byte),
		apiKeyAt: make(map[string]time.Time),
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

func (s *stubUserStore) SetAPIKey(ctx context.Context, email string, ciphertext []byte, updatedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("not found")
	}
	s.apiKeys[email] = append([]byte(nil), ciphertext...)
	s.apiKeyAt[email] = updatedAt
	return nil
}

func (s *stubUserStore) ClearAPIKey(ctx context.Context, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return errors.New("not found")
	}
	delete(s.apiKeys, email)
	delete(s.apiKeyAt, email)
	return nil
}

func (s *stubUserStore) GetAPIKey(ctx context.Context, email string) ([]byte, *time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[email]; !ok {
		return nil, nil, errors.New("not found")
	}
	key, ok := s.apiKeys[email]
	if !ok {
		return nil, nil, errors.New("not found")
	}
	ts := s.apiKeyAt[email]
	return append([]byte(nil), key...), &ts, nil
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
		return sqlstore.Project{}, sql.ErrNoRows
	}
	return p, nil
}

func (s *stubProjectStore) GetByID(ctx context.Context, id string) (sqlstore.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.projects[id]
	if !ok {
		return sqlstore.Project{}, sql.ErrNoRows
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

func (s *stubDomainStore) ListByIDs(ctx context.Context, ids []string) ([]sqlstore.Domain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]sqlstore.Domain, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if d, ok := s.domains[id]; ok {
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

func (s *stubDomainStore) UpdateLinkSettings(ctx context.Context, id string, anchorText, acceptorURL sql.NullString) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.domains[id]; ok {
		d.LinkAnchorText = anchorText
		d.LinkAcceptorURL = acceptorURL
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
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}
	if g.UpdatedAt.IsZero() {
		g.UpdatedAt = g.CreatedAt
	}
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

func (s *stubGenerationStore) ListRecentByUser(ctx context.Context, email string, limit, offset int, search string) ([]sqlstore.Generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]sqlstore.Generation, 0, len(s.generations))
	for _, g := range s.generations {
		if search != "" && !strings.Contains(strings.ToLower(g.DomainID), strings.ToLower(search)) {
			continue
		}
		res = append(res, g)
	}
	if offset > 0 {
		if offset >= len(res) {
			return []sqlstore.Generation{}, nil
		}
		res = res[offset:]
	}
	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

func (s *stubGenerationStore) ListRecentByUserLite(ctx context.Context, email string, limit, offset int, search string) ([]sqlstore.Generation, error) {
	return s.ListRecentByUser(ctx, email, limit, offset, search)
}

func (s *stubGenerationStore) ListRecentAll(ctx context.Context, limit, offset int, search string) ([]sqlstore.Generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]sqlstore.Generation, 0, len(s.generations))
	for _, g := range s.generations {
		if search != "" && !strings.Contains(strings.ToLower(g.DomainID), strings.ToLower(search)) {
			continue
		}
		res = append(res, g)
	}
	if offset > 0 {
		if offset >= len(res) {
			return []sqlstore.Generation{}, nil
		}
		res = res[offset:]
	}
	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

func (s *stubGenerationStore) ListRecentAllLite(ctx context.Context, limit, offset int, search string) ([]sqlstore.Generation, error) {
	return s.ListRecentAll(ctx, limit, offset, search)
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
		g.UpdatedAt = time.Now().UTC()
		s.generations[id] = g
		return nil
	}
	return errors.New("not found")
}

func (s *stubGenerationStore) SaveCheckpoint(ctx context.Context, id string, checkpointData []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.generations[id]; ok {
		g.CheckpointData = append([]byte(nil), checkpointData...)
		g.UpdatedAt = time.Now().UTC()
		s.generations[id] = g
		return nil
	}
	return errors.New("not found")
}

func (s *stubGenerationStore) ClearCheckpoint(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.generations[id]; ok {
		g.CheckpointData = nil
		g.UpdatedAt = time.Now().UTC()
		s.generations[id] = g
		return nil
	}
	return errors.New("not found")
}

func (s *stubGenerationStore) UpdateStatusWithCheckpoint(ctx context.Context, id, status string, progress int, checkpointData []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.generations[id]; ok {
		g.Status = status
		g.Progress = progress
		g.CheckpointData = append([]byte(nil), checkpointData...)
		g.UpdatedAt = time.Now().UTC()
		s.generations[id] = g
		return nil
	}
	return errors.New("not found")
}

func (s *stubGenerationStore) GetLastSuccessfulByDomain(ctx context.Context, domainID string) (sqlstore.Generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		found bool
		last  sqlstore.Generation
	)
	for _, g := range s.generations {
		if g.DomainID != domainID || g.Status != "success" {
			continue
		}
		if !found {
			last = g
			found = true
			continue
		}
		lastTime := last.UpdatedAt
		if lastTime.IsZero() {
			lastTime = last.CreatedAt
		}
		curTime := g.UpdatedAt
		if curTime.IsZero() {
			curTime = g.CreatedAt
		}
		if curTime.After(lastTime) {
			last = g
		}
	}
	if !found {
		return sqlstore.Generation{}, errors.New("not found")
	}
	return last, nil
}

func (s *stubGenerationStore) GetLastByDomain(ctx context.Context, domainID string) (sqlstore.Generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		found bool
		last  sqlstore.Generation
	)
	for _, g := range s.generations {
		if g.DomainID != domainID {
			continue
		}
		if !found {
			last = g
			found = true
			continue
		}
		lastTime := last.UpdatedAt
		if lastTime.IsZero() {
			lastTime = last.CreatedAt
		}
		curTime := g.UpdatedAt
		if curTime.IsZero() {
			curTime = g.CreatedAt
		}
		if curTime.After(lastTime) {
			last = g
		}
	}
	if !found {
		return sqlstore.Generation{}, errors.New("not found")
	}
	return last, nil
}

type stubScheduleStore struct {
	mu        sync.Mutex
	schedules map[string]sqlstore.Schedule
}

func newStubScheduleStore() *stubScheduleStore {
	return &stubScheduleStore{schedules: make(map[string]sqlstore.Schedule)}
}

func (s *stubScheduleStore) Create(ctx context.Context, schedule sqlstore.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if schedule.CreatedAt.IsZero() {
		schedule.CreatedAt = now
	}
	if schedule.UpdatedAt.IsZero() {
		schedule.UpdatedAt = schedule.CreatedAt
	}
	s.schedules[schedule.ID] = schedule
	return nil
}

func (s *stubScheduleStore) Get(ctx context.Context, scheduleID string) (*sqlstore.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sched, ok := s.schedules[scheduleID]; ok {
		out := sched
		return &out, nil
	}
	return nil, sql.ErrNoRows
}

func (s *stubScheduleStore) List(ctx context.Context, projectID string) ([]sqlstore.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []sqlstore.Schedule
	for _, sched := range s.schedules {
		if sched.ProjectID == projectID {
			res = append(res, sched)
		}
	}
	return res, nil
}

func (s *stubScheduleStore) Update(ctx context.Context, scheduleID string, updates sqlstore.ScheduleUpdates) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sched, ok := s.schedules[scheduleID]
	if !ok {
		return sql.ErrNoRows
	}
	if updates.Name != nil {
		sched.Name = *updates.Name
	}
	if updates.Description != nil {
		sched.Description = *updates.Description
	}
	if updates.Strategy != nil {
		sched.Strategy = *updates.Strategy
	}
	if updates.Config != nil {
		sched.Config = append([]byte(nil), (*updates.Config)...)
	}
	if updates.IsActive != nil {
		sched.IsActive = *updates.IsActive
	}
	sched.UpdatedAt = time.Now().UTC()
	s.schedules[scheduleID] = sched
	return nil
}

func (s *stubScheduleStore) Delete(ctx context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.schedules[scheduleID]; !ok {
		return sql.ErrNoRows
	}
	delete(s.schedules, scheduleID)
	return nil
}

func (s *stubScheduleStore) ListActive(ctx context.Context) ([]sqlstore.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []sqlstore.Schedule
	for _, sched := range s.schedules {
		if sched.IsActive {
			res = append(res, sched)
		}
	}
	return res, nil
}

type stubLinkScheduleStore struct {
	mu        sync.Mutex
	schedules map[string]sqlstore.LinkSchedule
}

func newStubLinkScheduleStore() *stubLinkScheduleStore {
	return &stubLinkScheduleStore{schedules: make(map[string]sqlstore.LinkSchedule)}
}

func (s *stubLinkScheduleStore) GetByProject(ctx context.Context, projectID string) (*sqlstore.LinkSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sched := range s.schedules {
		if sched.ProjectID == projectID {
			out := sched
			return &out, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *stubLinkScheduleStore) Upsert(ctx context.Context, schedule sqlstore.LinkSchedule) (*sqlstore.LinkSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, existing := range s.schedules {
		if existing.ProjectID != schedule.ProjectID {
			continue
		}
		if schedule.Name != "" {
			existing.Name = schedule.Name
		}
		if len(schedule.Config) > 0 {
			existing.Config = append([]byte(nil), schedule.Config...)
		}
		existing.IsActive = schedule.IsActive
		if schedule.NextRunAt.Valid {
			existing.NextRunAt = schedule.NextRunAt
		}
		if schedule.Timezone.Valid {
			existing.Timezone = schedule.Timezone
		}
		if !schedule.UpdatedAt.IsZero() {
			existing.UpdatedAt = schedule.UpdatedAt
		}
		s.schedules[id] = existing
		out := existing
		return &out, nil
	}
	if schedule.ID == "" {
		schedule.ID = "link-schedule-1"
	}
	if schedule.CreatedAt.IsZero() {
		schedule.CreatedAt = time.Now().UTC()
	}
	if schedule.UpdatedAt.IsZero() {
		schedule.UpdatedAt = schedule.CreatedAt
	}
	s.schedules[schedule.ID] = schedule
	out := schedule
	return &out, nil
}

func (s *stubLinkScheduleStore) DisableByProject(ctx context.Context, projectID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sched := range s.schedules {
		if sched.ProjectID != projectID {
			continue
		}
		sched.IsActive = false
		sched.UpdatedAt = time.Now().UTC()
		s.schedules[id] = sched
		return nil
	}
	return sql.ErrNoRows
}

func (s *stubLinkScheduleStore) DeleteByProject(ctx context.Context, projectID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sched := range s.schedules {
		if sched.ProjectID != projectID {
			continue
		}
		delete(s.schedules, id)
		return nil
	}
	return sql.ErrNoRows
}

type stubGenQueueStore struct {
	mu              sync.Mutex
	items           map[string]sqlstore.QueueItem
	domainToProject map[string]string
	err             error
}

func newStubGenQueueStore() *stubGenQueueStore {
	return &stubGenQueueStore{
		items:           make(map[string]sqlstore.QueueItem),
		domainToProject: make(map[string]string),
	}
}

func (s *stubGenQueueStore) Enqueue(ctx context.Context, item sqlstore.QueueItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.items[item.ID] = item
	return nil
}

func (s *stubGenQueueStore) Get(ctx context.Context, itemID string) (*sqlstore.QueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[itemID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	out := item
	return &out, nil
}

func (s *stubGenQueueStore) ListByProject(ctx context.Context, projectID string) ([]sqlstore.QueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []sqlstore.QueueItem
	for _, item := range s.items {
		if s.domainToProject[item.DomainID] == projectID {
			res = append(res, item)
		}
	}
	return res, nil
}

func (s *stubGenQueueStore) ListByProjectPage(ctx context.Context, projectID string, limit, offset int, search string) ([]sqlstore.QueueItem, error) {
	res, err := s.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if search != "" {
		filtered := res[:0]
		for _, item := range res {
			if strings.Contains(strings.ToLower(item.DomainID), strings.ToLower(search)) {
				filtered = append(filtered, item)
			}
		}
		res = filtered
	}
	if offset > 0 {
		if offset >= len(res) {
			return []sqlstore.QueueItem{}, nil
		}
		res = res[offset:]
	}
	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

func (s *stubGenQueueStore) Delete(ctx context.Context, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[itemID]; !ok {
		return sql.ErrNoRows
	}
	delete(s.items, itemID)
	return nil
}

type stubIndexCheckStore struct {
	mu              sync.Mutex
	checks          map[string]sqlstore.IndexCheck
	domainToProject map[string]string
	domainURL       map[string]string
	errGetByDomain  error
	errCreate       error
	errReset        error
	errListDomain   error
	errListProject  error
	errListAll      error
	errListFailed   error
	errCount        error
	errAggStats     error
	errAggDaily     error
}

func newStubIndexCheckStore() *stubIndexCheckStore {
	return &stubIndexCheckStore{
		checks:          make(map[string]sqlstore.IndexCheck),
		domainToProject: make(map[string]string),
		domainURL:       make(map[string]string),
	}
}

func (s *stubIndexCheckStore) Create(ctx context.Context, check sqlstore.IndexCheck) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errCreate != nil {
		return s.errCreate
	}
	if check.CreatedAt.IsZero() {
		check.CreatedAt = time.Now().UTC()
	}
	if check.Status == "" {
		check.Status = "pending"
	}
	s.checks[check.ID] = check
	return nil
}

func (s *stubIndexCheckStore) Get(ctx context.Context, checkID string) (*sqlstore.IndexCheck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	check, ok := s.checks[checkID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	out := check
	return &out, nil
}

func (s *stubIndexCheckStore) GetByDomainAndDate(ctx context.Context, domainID string, date time.Time) (*sqlstore.IndexCheck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errGetByDomain != nil {
		return nil, s.errGetByDomain
	}
	for _, check := range s.checks {
		if check.DomainID != domainID {
			continue
		}
		if check.CheckDate.Equal(date) {
			out := check
			return &out, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *stubIndexCheckStore) ListByDomain(ctx context.Context, domainID string, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errListDomain != nil {
		return nil, s.errListDomain
	}
	res := make([]sqlstore.IndexCheck, 0)
	for _, check := range s.checks {
		if check.DomainID != domainID {
			continue
		}
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		res = append(res, check)
	}
	sortIndexChecks(res)
	res = applyIndexCheckPagination(res, filters)
	return res, nil
}

func (s *stubIndexCheckStore) ListByProject(ctx context.Context, projectID string, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errListProject != nil {
		return nil, s.errListProject
	}
	res := make([]sqlstore.IndexCheck, 0)
	for _, check := range s.checks {
		if s.domainToProject[check.DomainID] != projectID {
			continue
		}
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		res = append(res, check)
	}
	sortIndexChecks(res)
	res = applyIndexCheckPagination(res, filters)
	return res, nil
}

func (s *stubIndexCheckStore) ListAll(ctx context.Context, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errListAll != nil {
		return nil, s.errListAll
	}
	res := make([]sqlstore.IndexCheck, 0, len(s.checks))
	for _, check := range s.checks {
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		res = append(res, check)
	}
	sortIndexChecks(res)
	res = applyIndexCheckPagination(res, filters)
	return res, nil
}

func (s *stubIndexCheckStore) ListFailed(ctx context.Context, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errListFailed != nil {
		return nil, s.errListFailed
	}
	filters.Statuses = []string{"failed_investigation"}
	res := make([]sqlstore.IndexCheck, 0)
	for _, check := range s.checks {
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		res = append(res, check)
	}
	sortIndexChecks(res)
	res = applyIndexCheckPagination(res, filters)
	return res, nil
}

func (s *stubIndexCheckStore) CountByDomain(ctx context.Context, domainID string, filters sqlstore.IndexCheckFilters) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errCount != nil {
		return 0, s.errCount
	}
	count := 0
	for _, check := range s.checks {
		if check.DomainID != domainID {
			continue
		}
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		count++
	}
	return count, nil
}

func (s *stubIndexCheckStore) CountByProject(ctx context.Context, projectID string, filters sqlstore.IndexCheckFilters) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errCount != nil {
		return 0, s.errCount
	}
	count := 0
	for _, check := range s.checks {
		if s.domainToProject[check.DomainID] != projectID {
			continue
		}
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		count++
	}
	return count, nil
}

func (s *stubIndexCheckStore) CountAll(ctx context.Context, filters sqlstore.IndexCheckFilters) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errCount != nil {
		return 0, s.errCount
	}
	count := 0
	for _, check := range s.checks {
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		count++
	}
	return count, nil
}

func (s *stubIndexCheckStore) CountFailed(ctx context.Context, filters sqlstore.IndexCheckFilters) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errCount != nil {
		return 0, s.errCount
	}
	count := 0
	filters.Statuses = []string{"failed_investigation"}
	for _, check := range s.checks {
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		count++
	}
	return count, nil
}

func (s *stubIndexCheckStore) ResetForManual(ctx context.Context, checkID string, nextRetry time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errReset != nil {
		return s.errReset
	}
	check, ok := s.checks[checkID]
	if !ok {
		return sql.ErrNoRows
	}
	check.Status = "pending"
	check.Attempts = 0
	check.IsIndexed = sql.NullBool{}
	check.ErrorMessage = sql.NullString{}
	check.LastAttemptAt = sql.NullTime{}
	check.CompletedAt = sql.NullTime{}
	check.NextRetryAt = sql.NullTime{Time: nextRetry, Valid: true}
	s.checks[checkID] = check
	return nil
}

func (s *stubIndexCheckStore) AggregateStats(ctx context.Context, filters sqlstore.IndexCheckFilters) (sqlstore.IndexCheckStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errAggStats != nil {
		return sqlstore.IndexCheckStats{}, s.errAggStats
	}
	res := make([]sqlstore.IndexCheck, 0, len(s.checks))
	for _, check := range s.checks {
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		res = append(res, check)
	}
	return aggregateIndexCheckStats(res), nil
}

func (s *stubIndexCheckStore) AggregateStatsByDomain(ctx context.Context, domainID string, filters sqlstore.IndexCheckFilters) (sqlstore.IndexCheckStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errAggStats != nil {
		return sqlstore.IndexCheckStats{}, s.errAggStats
	}
	res := make([]sqlstore.IndexCheck, 0)
	for _, check := range s.checks {
		if check.DomainID != domainID {
			continue
		}
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		res = append(res, check)
	}
	return aggregateIndexCheckStats(res), nil
}

func (s *stubIndexCheckStore) AggregateStatsByProject(ctx context.Context, projectID string, filters sqlstore.IndexCheckFilters) (sqlstore.IndexCheckStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errAggStats != nil {
		return sqlstore.IndexCheckStats{}, s.errAggStats
	}
	res := make([]sqlstore.IndexCheck, 0)
	for _, check := range s.checks {
		if s.domainToProject[check.DomainID] != projectID {
			continue
		}
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		res = append(res, check)
	}
	return aggregateIndexCheckStats(res), nil
}

func (s *stubIndexCheckStore) AggregateDaily(ctx context.Context, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheckDailySummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errAggDaily != nil {
		return nil, s.errAggDaily
	}
	res := make([]sqlstore.IndexCheck, 0, len(s.checks))
	for _, check := range s.checks {
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		res = append(res, check)
	}
	return aggregateIndexCheckDaily(res), nil
}

func (s *stubIndexCheckStore) AggregateDailyByDomain(ctx context.Context, domainID string, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheckDailySummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errAggDaily != nil {
		return nil, s.errAggDaily
	}
	res := make([]sqlstore.IndexCheck, 0)
	for _, check := range s.checks {
		if check.DomainID != domainID {
			continue
		}
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		res = append(res, check)
	}
	return aggregateIndexCheckDaily(res), nil
}

func (s *stubIndexCheckStore) AggregateDailyByProject(ctx context.Context, projectID string, filters sqlstore.IndexCheckFilters) ([]sqlstore.IndexCheckDailySummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errAggDaily != nil {
		return nil, s.errAggDaily
	}
	res := make([]sqlstore.IndexCheck, 0)
	for _, check := range s.checks {
		if s.domainToProject[check.DomainID] != projectID {
			continue
		}
		if !filterIndexCheck(check, filters, s.domainURL) {
			continue
		}
		res = append(res, check)
	}
	return aggregateIndexCheckDaily(res), nil
}

type stubCheckHistoryStore struct {
	mu      sync.Mutex
	history map[string][]sqlstore.CheckHistory
	errList error
}

func newStubCheckHistoryStore() *stubCheckHistoryStore {
	return &stubCheckHistoryStore{
		history: make(map[string][]sqlstore.CheckHistory),
	}
}

func (s *stubCheckHistoryStore) ListByCheck(ctx context.Context, checkID string, limit int) ([]sqlstore.CheckHistory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errList != nil {
		return nil, s.errList
	}
	list := append([]sqlstore.CheckHistory(nil), s.history[checkID]...)
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

func filterIndexCheck(check sqlstore.IndexCheck, filters sqlstore.IndexCheckFilters, domainURL map[string]string) bool {
	if len(filters.Statuses) > 0 {
		match := false
		for _, status := range filters.Statuses {
			if strings.TrimSpace(status) == check.Status {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	if filters.Search != nil && strings.TrimSpace(*filters.Search) != "" {
		term := strings.ToLower(strings.TrimSpace(*filters.Search))
		match := strings.Contains(strings.ToLower(check.DomainID), term)
		if url := strings.TrimSpace(domainURL[check.DomainID]); url != "" {
			if strings.Contains(strings.ToLower(url), term) {
				match = true
			}
		}
		if !match {
			return false
		}
	}
	if filters.DomainID != nil && strings.TrimSpace(*filters.DomainID) != "" {
		if check.DomainID != strings.TrimSpace(*filters.DomainID) {
			return false
		}
	}
	if filters.IsIndexed != nil {
		if !check.IsIndexed.Valid || check.IsIndexed.Bool != *filters.IsIndexed {
			return false
		}
	}
	if filters.From != nil && check.CheckDate.Before(dateOnlyUTC(*filters.From)) {
		return false
	}
	if filters.To != nil && check.CheckDate.After(dateOnlyUTC(*filters.To)) {
		return false
	}
	return true
}

func aggregateIndexCheckStats(list []sqlstore.IndexCheck) sqlstore.IndexCheckStats {
	var stats sqlstore.IndexCheckStats
	var successAttempts []int
	for _, check := range list {
		stats.TotalChecks++
		if check.Status == "success" {
			successAttempts = append(successAttempts, check.Attempts)
			if check.IsIndexed.Valid {
				stats.TotalResolved++
				if check.IsIndexed.Bool {
					stats.IndexedTrue++
				}
			}
		}
		if check.Status == "failed_investigation" {
			stats.FailedInvestigation++
		}
	}
	if len(successAttempts) > 0 {
		sum := 0
		for _, val := range successAttempts {
			sum += val
		}
		stats.AvgAttemptsToSuccess = float64(sum) / float64(len(successAttempts))
	}
	return stats
}

func aggregateIndexCheckDaily(list []sqlstore.IndexCheck) []sqlstore.IndexCheckDailySummary {
	summary := make(map[time.Time]*sqlstore.IndexCheckDailySummary)
	for _, check := range list {
		day := dateOnlyUTC(check.CheckDate)
		item := summary[day]
		if item == nil {
			item = &sqlstore.IndexCheckDailySummary{Date: day}
			summary[day] = item
		}
		item.Total++
		switch check.Status {
		case "pending":
			item.Pending++
		case "checking":
			item.Checking++
		case "failed_investigation":
			item.FailedInvestigation++
		case "success":
			item.Success++
			if check.IsIndexed.Valid {
				if check.IsIndexed.Bool {
					item.IndexedTrue++
				} else {
					item.IndexedFalse++
				}
			}
		}
	}
	out := make([]sqlstore.IndexCheckDailySummary, 0, len(summary))
	for _, item := range summary {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

func applyIndexCheckPagination(list []sqlstore.IndexCheck, filters sqlstore.IndexCheckFilters) []sqlstore.IndexCheck {
	if filters.Offset > 0 {
		if filters.Offset >= len(list) {
			return []sqlstore.IndexCheck{}
		}
		list = list[filters.Offset:]
	}
	if filters.Limit > 0 && len(list) > filters.Limit {
		list = list[:filters.Limit]
	}
	return list
}

func sortIndexChecks(list []sqlstore.IndexCheck) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].CheckDate.Equal(list[j].CheckDate) {
			return list[i].CreatedAt.After(list[j].CreatedAt)
		}
		return list[i].CheckDate.After(list[j].CheckDate)
	})
}

type stubLinkTaskStore struct {
	mu              sync.Mutex
	tasks           map[string]sqlstore.LinkTask
	domainToProject map[string]string
	userProjects    map[string]map[string]bool
}

func newStubLinkTaskStore() *stubLinkTaskStore {
	return &stubLinkTaskStore{
		tasks:           make(map[string]sqlstore.LinkTask),
		domainToProject: make(map[string]string),
		userProjects:    make(map[string]map[string]bool),
	}
}

func (s *stubLinkTaskStore) Create(ctx context.Context, task sqlstore.LinkTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.ScheduledFor.IsZero() {
		task.ScheduledFor = now
	}
	if task.Status == "" {
		task.Status = "pending"
	}
	if strings.TrimSpace(task.Action) == "" {
		task.Action = "insert"
	}
	s.tasks[task.ID] = task
	return nil
}

func (s *stubLinkTaskStore) Get(ctx context.Context, taskID string) (*sqlstore.LinkTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	out := task
	return &out, nil
}

func (s *stubLinkTaskStore) ListByDomain(ctx context.Context, domainID string, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []sqlstore.LinkTask
	for _, task := range s.tasks {
		if task.DomainID != domainID {
			continue
		}
		res = append(res, task)
	}
	return filterLinkTasks(res, filters), nil
}

func (s *stubLinkTaskStore) ListByProject(ctx context.Context, projectID string, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []sqlstore.LinkTask
	for _, task := range s.tasks {
		if s.domainToProject[task.DomainID] != projectID {
			continue
		}
		res = append(res, task)
	}
	return filterLinkTasks(res, filters), nil
}

func (s *stubLinkTaskStore) ListByUser(ctx context.Context, email string, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []sqlstore.LinkTask
	projects := s.userProjects[email]
	for _, task := range s.tasks {
		projectID := s.domainToProject[task.DomainID]
		if projectID == "" || !projects[projectID] {
			continue
		}
		res = append(res, task)
	}
	return filterLinkTasks(res, filters), nil
}

func (s *stubLinkTaskStore) ListAll(ctx context.Context, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]sqlstore.LinkTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		res = append(res, task)
	}
	return filterLinkTasks(res, filters), nil
}

func (s *stubLinkTaskStore) ListPending(ctx context.Context, limit int) ([]sqlstore.LinkTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []sqlstore.LinkTask
	now := time.Now().UTC()
	for _, task := range s.tasks {
		if task.Status != "pending" {
			continue
		}
		if task.ScheduledFor.After(now) {
			continue
		}
		res = append(res, task)
	}
	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

func (s *stubLinkTaskStore) Update(ctx context.Context, taskID string, updates sqlstore.LinkTaskUpdates) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return sql.ErrNoRows
	}
	if updates.AnchorText != nil {
		task.AnchorText = *updates.AnchorText
	}
	if updates.TargetURL != nil {
		task.TargetURL = *updates.TargetURL
	}
	if updates.Action != nil {
		task.Action = *updates.Action
	}
	if updates.Status != nil {
		task.Status = *updates.Status
	}
	if updates.FoundLocation != nil {
		task.FoundLocation = *updates.FoundLocation
	}
	if updates.GeneratedContent != nil {
		task.GeneratedContent = *updates.GeneratedContent
	}
	if updates.ErrorMessage != nil {
		task.ErrorMessage = *updates.ErrorMessage
	}
	if updates.Attempts != nil {
		task.Attempts = *updates.Attempts
	}
	if updates.ScheduledFor != nil {
		task.ScheduledFor = *updates.ScheduledFor
	}
	if updates.CompletedAt != nil {
		task.CompletedAt = *updates.CompletedAt
	}
	s.tasks[taskID] = task
	return nil
}

func (s *stubLinkTaskStore) Delete(ctx context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[taskID]; !ok {
		return sql.ErrNoRows
	}
	delete(s.tasks, taskID)
	return nil
}

func filterLinkTasks(tasks []sqlstore.LinkTask, filters sqlstore.LinkTaskFilters) []sqlstore.LinkTask {
	res := make([]sqlstore.LinkTask, 0, len(tasks))
	for _, task := range tasks {
		if filters.Status != nil && task.Status != *filters.Status {
			continue
		}
		if filters.ScheduledAfter != nil && task.ScheduledFor.Before(*filters.ScheduledAfter) {
			continue
		}
		if filters.ScheduledBefore != nil && task.ScheduledFor.After(*filters.ScheduledBefore) {
			continue
		}
		res = append(res, task)
	}
	if filters.Limit > 0 && len(res) > filters.Limit {
		res = res[:filters.Limit]
	}
	return res
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

func TestParseIndexCheckFiltersSortAndSearch(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/admin/index-checks?status=success,checking&search=Example.COM&domain_id=domain-1&is_indexed=true&from=2026-02-01&to=2026-02-12&sort=check_date:asc&limit=20&page=2",
		nil,
	)

	filters := parseIndexCheckFilters(req, 50, 200)
	if filters.Limit != 20 {
		t.Fatalf("expected limit 20, got %d", filters.Limit)
	}
	if filters.Offset != 20 {
		t.Fatalf("expected offset 20, got %d", filters.Offset)
	}
	if len(filters.Statuses) != 2 || filters.Statuses[0] != "success" || filters.Statuses[1] != "checking" {
		t.Fatalf("unexpected statuses: %#v", filters.Statuses)
	}
	if filters.Search == nil || *filters.Search != "Example.COM" {
		t.Fatalf("unexpected search: %#v", filters.Search)
	}
	if filters.DomainID == nil || *filters.DomainID != "domain-1" {
		t.Fatalf("unexpected domain id: %#v", filters.DomainID)
	}
	if filters.IsIndexed == nil || *filters.IsIndexed != true {
		t.Fatalf("unexpected is_indexed: %#v", filters.IsIndexed)
	}
	expectedFrom := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	expectedTo := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)
	if filters.From == nil || !filters.From.Equal(expectedFrom) {
		t.Fatalf("unexpected from: %#v", filters.From)
	}
	if filters.To == nil || !filters.To.Equal(expectedTo) {
		t.Fatalf("unexpected to: %#v", filters.To)
	}
	if filters.SortBy != "check_date" || filters.SortDir != "asc" {
		t.Fatalf("unexpected sort: %s %s", filters.SortBy, filters.SortDir)
	}
}

type stubSiteFileStore struct{}

func newStubSiteFileStore() *stubSiteFileStore {
	return &stubSiteFileStore{}
}

func (s *stubSiteFileStore) Get(ctx context.Context, fileID string) (*sqlstore.SiteFile, error) {
	return nil, sql.ErrNoRows
}

func (s *stubSiteFileStore) List(ctx context.Context, domainID string) ([]sqlstore.SiteFile, error) {
	return []sqlstore.SiteFile{}, nil
}

func (s *stubSiteFileStore) GetByPath(ctx context.Context, domainID, path string) (*sqlstore.SiteFile, error) {
	return nil, sql.ErrNoRows
}

func (s *stubSiteFileStore) Update(ctx context.Context, fileID string, content []byte) error {
	return errors.New("not found")
}

func (s *stubSiteFileStore) Delete(ctx context.Context, fileID string) error {
	return errors.New("not found")
}

type stubFileEditStore struct{}

func newStubFileEditStore() *stubFileEditStore {
	return &stubFileEditStore{}
}

func (s *stubFileEditStore) Create(ctx context.Context, edit sqlstore.FileEdit) error {
	return nil
}

func (s *stubFileEditStore) ListByFile(ctx context.Context, fileID string, limit int) ([]sqlstore.FileEdit, error) {
	return []sqlstore.FileEdit{}, nil
}
