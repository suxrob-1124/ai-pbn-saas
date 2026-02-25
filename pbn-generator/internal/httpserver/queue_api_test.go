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

func TestProjectQueueList(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	queueStore := s.genQueue.(*stubGenQueueStore)

	project := sqlstore.Project{
		ID:        "project-queue",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project

	domain := sqlstore.Domain{
		ID:        "domain-queue",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "published",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	domStore.domains[domain.ID] = domain

	item := sqlstore.QueueItem{
		ID:           "queue-1",
		DomainID:     domain.ID,
		Priority:     1,
		ScheduledFor: time.Now().UTC(),
		Status:       "pending",
		CreatedAt:    time.Now().UTC(),
	}
	queueStore.items[item.ID] = item
	queueStore.domainToProject[domain.ID] = project.ID

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/projects/project-queue/queue", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		s.handleProjectByID(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var list []queueItemDTO
		if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
			t.Fatalf("decode list: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 item, got %d", len(list))
		}
		if list[0].DomainURL == nil || *list[0].DomainURL != domain.URL {
			t.Fatalf("expected domain_url %s, got %v", domain.URL, list[0].DomainURL)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects/project-queue/queue", strings.NewReader(`{}`)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.handleProjectByID(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d: %s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode error payload: %v", err)
		}
		if payload["code"] != "method_not_allowed" {
			t.Fatalf("expected code method_not_allowed, got %v", payload["code"])
		}
		if payload["message"] != "method not allowed" {
			t.Fatalf("expected message method not allowed, got %v", payload["message"])
		}
	})
}

func TestQueueDelete(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	queueStore := s.genQueue.(*stubGenQueueStore)

	project := sqlstore.Project{
		ID:        "project-del",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project

	domain := sqlstore.Domain{
		ID:        "domain-del",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	domStore.domains[domain.ID] = domain

	item := sqlstore.QueueItem{
		ID:           "queue-del",
		DomainID:     domain.ID,
		Priority:     0,
		ScheduledFor: time.Now().UTC(),
		Status:       "pending",
		CreatedAt:    time.Now().UTC(),
	}
	queueStore.items[item.ID] = item
	queueStore.domainToProject[domain.ID] = project.ID

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/queue/queue-del", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		s.handleQueueItem(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if _, ok := queueStore.items[item.ID]; ok {
			t.Fatalf("expected item deleted")
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/queue/missing", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		s.handleQueueItem(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode error payload: %v", err)
		}
		if payload["code"] != "not_found" {
			t.Fatalf("expected code not_found, got %v", payload["code"])
		}
	})
}
