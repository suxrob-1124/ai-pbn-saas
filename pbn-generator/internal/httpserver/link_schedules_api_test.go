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

func TestProjectLinkScheduleUpsertGetDelete(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	project := sqlstore.Project{
		ID:        "project-link",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})

	body := `{"name":"Links","config":{"limit":2,"time":"10:00"},"timezone":"UTC"}`
	putReq := httptest.NewRequest(http.MethodPut, "/api/projects/project-link/link-schedule", strings.NewReader(body)).WithContext(ctx)
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	s.handleProjectByID(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", putRec.Code, putRec.Body.String())
	}
	var created linkScheduleDTO
	if err := json.NewDecoder(putRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode upsert: %v", err)
	}
	if created.ProjectID != project.ID {
		t.Fatalf("unexpected project id: %s", created.ProjectID)
	}
	if created.Timezone == nil || *created.Timezone != "UTC" {
		t.Fatalf("expected timezone UTC, got %#v", created.Timezone)
	}
	if created.NextRunAt == nil {
		t.Fatalf("expected next_run_at populated")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/projects/project-link/link-schedule", nil).WithContext(ctx)
	getRec := httptest.NewRecorder()
	s.handleProjectByID(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/projects/project-link/link-schedule", nil).WithContext(ctx)
	delRec := httptest.NewRecorder()
	s.handleProjectByID(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", delRec.Code, delRec.Body.String())
	}
	lsStore := s.linkSchedules.(*stubLinkScheduleStore)
	sched, err := lsStore.GetByProject(context.Background(), project.ID)
	if err == nil {
		t.Fatalf("expected schedule to be deleted, got %+v", sched)
	}
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got: %v", err)
	}
}

func TestProjectLinkScheduleTriggerCreatesTasks(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	linkStore := s.linkTasks.(*stubLinkTaskStore)
	linkScheduleStore := s.linkSchedules.(*stubLinkScheduleStore)

	project := sqlstore.Project{
		ID:        "project-link-trigger",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project

	domains := []sqlstore.Domain{
		{
			ID:              "domain-a",
			ProjectID:       project.ID,
			URL:             "a.example.com",
			Status:          "published",
			LinkStatus:      sql.NullString{String: "pending", Valid: true},
			LinkAnchorText:  sql.NullString{String: "Anchor A", Valid: true},
			LinkAcceptorURL: sql.NullString{String: "https://acceptor/a", Valid: true},
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		},
		{
			ID:              "domain-b",
			ProjectID:       project.ID,
			URL:             "b.example.com",
			Status:          "published",
			LinkStatus:      sql.NullString{String: "pending", Valid: true},
			LinkAnchorText:  sql.NullString{String: "Anchor B", Valid: true},
			LinkAcceptorURL: sql.NullString{String: "https://acceptor/b", Valid: true},
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		},
	}
	for _, d := range domains {
		domStore.domains[d.ID] = d
		linkStore.domainToProject[d.ID] = project.ID
	}

	_, err := linkScheduleStore.Upsert(context.Background(), sqlstore.LinkSchedule{
		ID:        "ls-1",
		ProjectID: project.ID,
		Name:      "Links",
		Config:    json.RawMessage(`{"limit":1}`),
		IsActive:  true,
		CreatedBy: "owner@example.com",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("seed link schedule: %v", err)
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})
	triggerReq := httptest.NewRequest(http.MethodPost, "/api/projects/project-link-trigger/link-schedule/trigger", nil).WithContext(ctx)
	triggerRec := httptest.NewRecorder()
	s.handleProjectByID(triggerRec, triggerReq)
	if triggerRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", triggerRec.Code, triggerRec.Body.String())
	}
	if len(linkStore.tasks) != 1 {
		t.Fatalf("expected 1 task due to limit, got %d", len(linkStore.tasks))
	}
	for _, task := range linkStore.tasks {
		if task.Status != "pending" {
			t.Fatalf("expected pending status, got %s", task.Status)
		}
	}
}

func TestProjectLinkScheduleInvalidTimezone(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	project := sqlstore.Project{
		ID:        "project-link-bad-tz",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})

	body := `{"name":"Links","config":{"limit":1},"timezone":"Bad/Zone"}`
	req := httptest.NewRequest(http.MethodPut, "/api/projects/project-link-bad-tz/link-schedule", strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleProjectByID(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
