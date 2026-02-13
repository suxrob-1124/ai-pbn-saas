package httpserver

import (
	"context"
	"database/sql"
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
		Attempts:     3,
		FoundLocation: sql.NullString{
			String: "index.html:10",
			Valid:  true,
		},
		GeneratedContent: sql.NullString{
			String: "<p>generated</p>",
			Valid:  true,
		},
		ErrorMessage: sql.NullString{
			String: "boom",
			Valid:  true,
		},
		CreatedBy: "owner@example.com",
		CreatedAt: time.Now().Add(-time.Hour).UTC(),
		CompletedAt: sql.NullTime{
			Time:  time.Now().Add(-30 * time.Minute).UTC(),
			Valid: true,
		},
	}
	originalCreatedAt := task.CreatedAt
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
	if updated.Attempts != 0 {
		t.Fatalf("expected attempts reset to 0, got %d", updated.Attempts)
	}
	if updated.ScheduledFor.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("expected scheduled_for updated, got %s", updated.ScheduledFor.Format(time.RFC3339))
	}
	if !updated.CreatedAt.After(originalCreatedAt) {
		t.Fatalf("expected created_at to be refreshed, old=%s new=%s", originalCreatedAt, updated.CreatedAt)
	}
	if updated.FoundLocation.Valid {
		t.Fatalf("expected found_location cleared")
	}
	if updated.GeneratedContent.Valid {
		t.Fatalf("expected generated_content cleared")
	}
	if updated.ErrorMessage.Valid {
		t.Fatalf("expected error_message cleared")
	}
	if updated.CompletedAt.Valid {
		t.Fatalf("expected completed_at cleared")
	}
	domain := domStore.domains["domain-2"]
	if !domain.LinkStatus.Valid || domain.LinkStatus.String != "pending" {
		t.Fatalf("expected domain link_status pending, got %#v", domain.LinkStatus)
	}
}

func TestLinksListGlobalScope(t *testing.T) {
	s := setupServer(t)
	linkStore := s.linkTasks.(*stubLinkTaskStore)

	now := time.Now().UTC()
	linkStore.tasks["task-1"] = sqlstore.LinkTask{
		ID:           "task-1",
		DomainID:     "domain-1",
		AnchorText:   "A",
		TargetURL:    "https://example.com/a",
		ScheduledFor: now,
		Action:       "insert",
		Status:       "pending",
		CreatedBy:    "owner@example.com",
		CreatedAt:    now,
	}
	linkStore.tasks["task-2"] = sqlstore.LinkTask{
		ID:           "task-2",
		DomainID:     "domain-2",
		AnchorText:   "B",
		TargetURL:    "https://example.com/b",
		ScheduledFor: now,
		Action:       "insert",
		Status:       "pending",
		CreatedBy:    "owner@example.com",
		CreatedAt:    now,
	}
	linkStore.tasks["task-3"] = sqlstore.LinkTask{
		ID:           "task-3",
		DomainID:     "domain-3",
		AnchorText:   "C",
		TargetURL:    "https://example.com/c",
		ScheduledFor: now,
		Action:       "insert",
		Status:       "pending",
		CreatedBy:    "owner@example.com",
		CreatedAt:    now,
	}
	linkStore.domainToProject["domain-1"] = "project-1"
	linkStore.domainToProject["domain-2"] = "project-2"
	linkStore.domainToProject["domain-3"] = "project-3"
	linkStore.userProjects["manager@example.com"] = map[string]bool{
		"project-1": true,
		"project-2": true,
	}

	t.Run("non-admin gets only accessible projects", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
			Email:      "manager@example.com",
			Role:       "manager",
			IsApproved: true,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/links", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		s.handleLinks(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var list []linkTaskDTO
		if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(list))
		}
		got := map[string]bool{}
		for _, task := range list {
			got[task.DomainID] = true
		}
		if !got["domain-1"] || !got["domain-2"] || got["domain-3"] {
			t.Fatalf("unexpected domains in response: %#v", got)
		}
	})

	t.Run("non-admin without access gets empty list", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
			Email:      "viewer@example.com",
			Role:       "manager",
			IsApproved: true,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/links", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		s.handleLinks(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var list []linkTaskDTO
		if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(list) != 0 {
			t.Fatalf("expected 0 tasks, got %d", len(list))
		}
	})

	t.Run("admin gets all tasks", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
			Email:      "admin@example.com",
			Role:       "admin",
			IsApproved: true,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/links", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		s.handleLinks(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var list []linkTaskDTO
		if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(list) != 3 {
			t.Fatalf("expected 3 tasks, got %d", len(list))
		}
	})
}
