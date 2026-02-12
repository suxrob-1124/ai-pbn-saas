package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

func TestDomainIndexChecksList(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	checkStore := s.indexChecks.(*stubIndexCheckStore)

	project := sqlstore.Project{
		ID:        "project-1",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project
	domain := sqlstore.Domain{
		ID:        "domain-1",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "published",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	domStore.domains[domain.ID] = domain

	check := sqlstore.IndexCheck{
		ID:        "check-1",
		DomainID:  domain.ID,
		CheckDate: time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
		Status:    "success",
		IsIndexed: sql.NullBool{Bool: true, Valid: true},
		Attempts:  1,
		CreatedAt: time.Now().UTC(),
	}
	checkStore.checks[check.ID] = check

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "owner@example.com",
		Role:  "manager",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/domains/domain-1/index-checks", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list indexCheckListDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list.Items))
	}
	if list.Total != 1 {
		t.Fatalf("expected total=1, got %d", list.Total)
	}
	if list.Items[0].IsIndexed == nil || !*list.Items[0].IsIndexed {
		t.Fatalf("expected is_indexed=true, got %#v", list.Items[0].IsIndexed)
	}
}

func TestDomainIndexChecksListStoreError(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	checkStore := s.indexChecks.(*stubIndexCheckStore)

	project := sqlstore.Project{
		ID:        "project-1",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project
	domStore.domains["domain-1"] = sqlstore.Domain{
		ID:        "domain-1",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "published",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	checkStore.errListDomain = errors.New("boom")

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "owner@example.com",
		Role:  "manager",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/domains/domain-1/index-checks", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDomainIndexChecksManualRun(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	checkStore := s.indexChecks.(*stubIndexCheckStore)

	project := sqlstore.Project{
		ID:        "project-1",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project
	domain := sqlstore.Domain{
		ID:        "domain-1",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "published",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	domStore.domains[domain.ID] = domain

	today := dateOnlyUTC(time.Now().UTC())
	check := sqlstore.IndexCheck{
		ID:        "check-1",
		DomainID:  domain.ID,
		CheckDate: today,
		Status:    "success",
		IsIndexed: sql.NullBool{Bool: true, Valid: true},
		Attempts:  3,
		CreatedAt: time.Now().UTC(),
	}
	checkStore.checks[check.ID] = check

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/domains/domain-1/index-checks", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var dto indexCheckDTO
	if err := json.NewDecoder(rec.Body).Decode(&dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if dto.Status != "pending" {
		t.Fatalf("expected status pending, got %s", dto.Status)
	}
	if dto.IsIndexed != nil {
		t.Fatalf("expected is_indexed nil after reset, got %#v", dto.IsIndexed)
	}
}

func TestDomainIndexChecksManualRunUnpublished(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)

	project := sqlstore.Project{
		ID:        "project-1",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project
	domStore.domains["domain-1"] = sqlstore.Domain{
		ID:        "domain-1",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/domains/domain-1/index-checks", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectIndexChecksManualRunStoreError(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	checkStore := s.indexChecks.(*stubIndexCheckStore)

	project := sqlstore.Project{
		ID:        "project-1",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project
	domStore.domains["domain-1"] = sqlstore.Domain{
		ID:        "domain-1",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "published",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	checkStore.errGetByDomain = sql.ErrNoRows
	checkStore.errCreate = errors.New("boom")

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/projects/project-1/index-checks", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleProjectByID(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminIndexChecksFailed(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	checkStore := s.indexChecks.(*stubIndexCheckStore)

	project := sqlstore.Project{
		ID:        "project-1",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project
	domain := sqlstore.Domain{
		ID:        "domain-1",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	domStore.domains[domain.ID] = domain

	check := sqlstore.IndexCheck{
		ID:        "check-1",
		DomainID:  domain.ID,
		CheckDate: time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
		Status:    "failed_investigation",
		Attempts:  8,
		CreatedAt: time.Now().UTC(),
	}
	checkStore.checks[check.ID] = check

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/index-checks/failed", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleAdminIndexChecksFailed(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list indexCheckListDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list.Items))
	}
	if list.Total != 1 {
		t.Fatalf("expected total=1, got %d", list.Total)
	}
	if list.Items[0].Status != "failed_investigation" {
		t.Fatalf("unexpected status: %s", list.Items[0].Status)
	}
}

func TestAdminIndexChecksFailedStoreError(t *testing.T) {
	s := setupServer(t)
	checkStore := s.indexChecks.(*stubIndexCheckStore)
	checkStore.errListFailed = errors.New("boom")

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/index-checks/failed", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleAdminIndexChecksFailed(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminIndexChecksFilters(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	checkStore := s.indexChecks.(*stubIndexCheckStore)

	project := sqlstore.Project{
		ID:        "project-1",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project
	domStore.domains["domain-a"] = sqlstore.Domain{
		ID:        "domain-a",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	domStore.domains["domain-b"] = sqlstore.Domain{
		ID:        "domain-b",
		ProjectID: project.ID,
		URL:       "other.com",
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	checkStore.checks["check-1"] = sqlstore.IndexCheck{
		ID:        "check-1",
		DomainID:  "domain-a",
		CheckDate: time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
		Status:    "success",
		Attempts:  1,
		CreatedAt: time.Now().UTC(),
	}
	checkStore.checks["check-2"] = sqlstore.IndexCheck{
		ID:        "check-2",
		DomainID:  "domain-b",
		CheckDate: time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC),
		Status:    "failed_investigation",
		Attempts:  2,
		CreatedAt: time.Now().UTC(),
	}

	checkStore.domainURL["domain-a"] = "example.com"
	checkStore.domainURL["domain-b"] = "other.com"

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/index-checks?status=success&search=example.com&limit=1&page=1", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleAdminIndexChecks(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list indexCheckListDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list.Items))
	}
	if list.Total != 1 {
		t.Fatalf("expected total=1, got %d", list.Total)
	}
	if list.Items[0].DomainID != "domain-a" {
		t.Fatalf("unexpected domain id: %s", list.Items[0].DomainID)
	}
}

func TestAdminIndexChecksHistory(t *testing.T) {
	s := setupServer(t)

	checkStore := s.indexChecks.(*stubIndexCheckStore)
	historyStore := s.checkHistory.(*stubCheckHistoryStore)

	checkStore.checks["check-1"] = sqlstore.IndexCheck{
		ID:        "check-1",
		DomainID:  "domain-1",
		CheckDate: time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
		Status:    "success",
		Attempts:  1,
		CreatedAt: time.Now().UTC(),
	}

	historyStore.history["check-1"] = []sqlstore.CheckHistory{
		{
			ID:            "hist-1",
			CheckID:       "check-1",
			AttemptNumber: 1,
			Result:        sqlstore.NullableString("success"),
			ResponseData:  []byte(`{"indexed":true}`),
			DurationMS:    sql.NullInt64{Int64: 1200, Valid: true},
			CreatedAt:     time.Now().UTC(),
		},
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/index-checks/check-1/history", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleAdminIndexCheckRoute(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list []indexCheckHistoryDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list))
	}
	if list[0].AttemptNumber != 1 {
		t.Fatalf("unexpected attempt number: %d", list[0].AttemptNumber)
	}
}

func TestDomainIndexChecksHistory(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	checkStore := s.indexChecks.(*stubIndexCheckStore)
	historyStore := s.checkHistory.(*stubCheckHistoryStore)

	project := sqlstore.Project{
		ID:        "project-1",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project
	domStore.domains["domain-1"] = sqlstore.Domain{
		ID:        "domain-1",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "published",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	checkStore.checks["check-1"] = sqlstore.IndexCheck{
		ID:        "check-1",
		DomainID:  "domain-1",
		CheckDate: time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
		Status:    "success",
		Attempts:  1,
		CreatedAt: time.Now().UTC(),
	}

	historyStore.history["check-1"] = []sqlstore.CheckHistory{
		{
			ID:            "hist-1",
			CheckID:       "check-1",
			AttemptNumber: 1,
			Result:        sqlstore.NullableString("success"),
			ResponseData:  []byte(`{"indexed":false}`),
			DurationMS:    sql.NullInt64{Int64: 800, Valid: true},
			CreatedAt:     time.Now().UTC(),
		},
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "owner@example.com",
		Role:  "manager",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/domains/domain-1/index-checks/check-1/history", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list []indexCheckHistoryDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list))
	}
	if list[0].CheckID != "check-1" {
		t.Fatalf("unexpected check id: %s", list[0].CheckID)
	}
}

func TestAdminIndexChecksHistoryStoreError(t *testing.T) {
	s := setupServer(t)

	historyStore := s.checkHistory.(*stubCheckHistoryStore)
	historyStore.errList = errors.New("boom")

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/index-checks/check-1/history", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleAdminIndexCheckRoute(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminIndexChecksRun(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)

	project := sqlstore.Project{
		ID:        "project-1",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project
	domStore.domains["domain-1"] = sqlstore.Domain{
		ID:        "domain-1",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	body := `{"domain_id":"domain-1"}`
	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/index-checks/run", strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleAdminIndexChecksRun(rec, req)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("expected 200/201, got %d: %s", rec.Code, rec.Body.String())
	}
	var dto indexCheckDTO
	if err := json.NewDecoder(rec.Body).Decode(&dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if dto.DomainID != "domain-1" {
		t.Fatalf("expected domain_id domain-1, got %s", dto.DomainID)
	}
	if dto.Status != "pending" {
		t.Fatalf("expected status pending, got %s", dto.Status)
	}
}

func TestAdminIndexChecksStats(t *testing.T) {
	s := setupServer(t)

	checkStore := s.indexChecks.(*stubIndexCheckStore)
	checkStore.checks["check-1"] = sqlstore.IndexCheck{
		ID:        "check-1",
		DomainID:  "domain-1",
		CheckDate: time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC),
		Status:    "success",
		IsIndexed: sql.NullBool{Bool: true, Valid: true},
		Attempts:  2,
		CreatedAt: time.Now().UTC(),
	}
	checkStore.checks["check-2"] = sqlstore.IndexCheck{
		ID:        "check-2",
		DomainID:  "domain-2",
		CheckDate: time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
		Status:    "failed_investigation",
		Attempts:  1,
		CreatedAt: time.Now().UTC(),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/admin/index-checks/stats?from=2026-02-01&to=2026-02-12", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleAdminIndexChecksStats(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp indexCheckStatsDTO
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TotalChecks != 2 {
		t.Fatalf("expected total_checks=2, got %d", resp.TotalChecks)
	}
	if resp.IndexedTrue != 1 {
		t.Fatalf("expected indexed_true=1, got %d", resp.IndexedTrue)
	}
	if len(resp.Daily) == 0 {
		t.Fatalf("expected daily stats, got 0")
	}
}
