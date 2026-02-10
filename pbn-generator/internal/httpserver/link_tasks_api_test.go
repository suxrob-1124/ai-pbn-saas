package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

func TestDomainLinksCreateAndList(t *testing.T) {
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

	badReq := httptest.NewRequest(http.MethodPost, "/api/domains/domain-1/links", strings.NewReader(`{"target_url":"https://example.com"}`)).WithContext(ctx)
	badReq.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	s.handleDomainActions(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", badRec.Code, badRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/domains/domain-1/links", strings.NewReader(`{"anchor_text":"Test","target_url":"https://example.com"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/domains/domain-1/links", nil).WithContext(ctx)
	listRec := httptest.NewRecorder()
	s.handleDomainActions(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var list []linkTaskDTO
	if err := json.NewDecoder(listRec.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 task, got %d", len(list))
	}
}

func TestLinkTasksUpdateAndRetry(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	linkStore := s.linkTasks.(*stubLinkTaskStore)

	project := sqlstore.Project{
		ID:        "project-2",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project
	domStore.domains["domain-2"] = sqlstore.Domain{
		ID:        "domain-2",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	linkStore.domainToProject["domain-2"] = project.ID

	task := sqlstore.LinkTask{
		ID:           "task-1",
		DomainID:     "domain-2",
		AnchorText:   "Anchor",
		TargetURL:    "https://example.com",
		ScheduledFor: time.Now().Add(-time.Hour).UTC(),
		Action:       "insert",
		Status:       "failed",
		Attempts:     1,
		CreatedBy:    "owner@example.com",
		CreatedAt:    time.Now().Add(-time.Hour).UTC(),
	}
	if err := linkStore.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})

	newTime := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/links/task-1", strings.NewReader(`{"scheduled_for":"`+newTime+`"}`)).WithContext(ctx)
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	s.handleLinkByID(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", patchRec.Code, patchRec.Body.String())
	}

	retryReq := httptest.NewRequest(http.MethodPost, "/api/links/task-1/retry", nil).WithContext(ctx)
	retryRec := httptest.NewRecorder()
	s.handleLinkByID(retryRec, retryReq)
	if retryRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", retryRec.Code, retryRec.Body.String())
	}

	updated, err := linkStore.Get(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updated.Status != "pending" {
		t.Fatalf("expected status pending, got %s", updated.Status)
	}
	if updated.ScheduledFor.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("expected scheduled_for updated, got %s", updated.ScheduledFor.Format(time.RFC3339))
	}
}
