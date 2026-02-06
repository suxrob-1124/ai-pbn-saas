package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

func TestGenerationsListIncludesDomainURL(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	genStore := s.generations.(*stubGenerationStore)

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

	genStore.generations["gen-1"] = sqlstore.Generation{
		ID:        "gen-1",
		DomainID:  "domain-1",
		Status:    "pending",
		Progress:  10,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "admin@example.com",
		Role:       "admin",
		IsApproved: true,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/generations", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	s.handleGenerations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list []generationDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 generation, got %d", len(list))
	}
	if list[0].DomainURL == nil || *list[0].DomainURL != "example.com" {
		t.Fatalf("expected domain_url example.com, got %#v", list[0].DomainURL)
	}
}

func TestGenerationsListMissingDomainDoesNotFail(t *testing.T) {
	s := setupServer(t)

	genStore := s.generations.(*stubGenerationStore)
	genStore.generations["gen-1"] = sqlstore.Generation{
		ID:        "gen-1",
		DomainID:  "missing-domain",
		Status:    "pending",
		Progress:  0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "admin@example.com",
		Role:       "admin",
		IsApproved: true,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/generations", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	s.handleGenerations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list []generationDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 generation, got %d", len(list))
	}
	if list[0].DomainURL != nil {
		t.Fatalf("expected domain_url nil for missing domain, got %#v", list[0].DomainURL)
	}
}
