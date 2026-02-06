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

func TestProjectSchedulesCreateAndList(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	project := sqlstore.Project{
		ID:        "project-1",
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

	badReq := httptest.NewRequest(http.MethodPost, "/api/projects/project-1/schedules", strings.NewReader(`{"strategy":"daily","config":{"cron":"0 9 * * *"}}`)).WithContext(ctx)
	badReq.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	s.handleProjectByID(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", badRec.Code, badRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/projects/project-1/schedules", strings.NewReader(`{"name":"Daily","strategy":"daily","config":{"cron":"0 9 * * *"}}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleProjectByID(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var created scheduleDTO
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("missing schedule id")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/projects/project-1/schedules", nil).WithContext(ctx)
	listRec := httptest.NewRecorder()
	s.handleProjectByID(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var list []scheduleDTO
	if err := json.NewDecoder(listRec.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(list))
	}
}

func TestProjectScheduleTriggerEnqueues(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	schedStore := s.schedules.(*stubScheduleStore)
	queueStore := s.genQueue.(*stubGenQueueStore)

	project := sqlstore.Project{
		ID:        "project-2",
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

	schedule := sqlstore.Schedule{
		ID:        "schedule-1",
		ProjectID: project.ID,
		Name:      "Manual",
		Strategy:  "immediate",
		Config:    json.RawMessage(`{"cron":"0 9 * * *"}`),
		IsActive:  true,
		CreatedBy: "owner@example.com",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := schedStore.Create(context.Background(), schedule); err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/projects/project-2/schedules/schedule-1/trigger", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleProjectByID(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(queueStore.items) != 1 {
		t.Fatalf("expected 1 queued item, got %d", len(queueStore.items))
	}
	var queued sqlstore.QueueItem
	for _, item := range queueStore.items {
		queued = item
		break
	}
	if queued.ScheduleID.String != schedule.ID {
		t.Fatalf("unexpected schedule id: %s", queued.ScheduleID.String)
	}
}

func TestProjectScheduleTriggerRespectsLimitAndCooldown(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	schedStore := s.schedules.(*stubScheduleStore)
	queueStore := s.genQueue.(*stubGenQueueStore)

	project := sqlstore.Project{
		ID:        "project-3",
		UserEmail: "owner@example.com",
		Name:      "Demo",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project

	domains := []sqlstore.Domain{
		{ID: "domain-a", ProjectID: project.ID, URL: "a.example.com", Status: "waiting", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		{ID: "domain-b", ProjectID: project.ID, URL: "b.example.com", Status: "waiting", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		{ID: "domain-c", ProjectID: project.ID, URL: "c.example.com", Status: "waiting", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}
	for _, d := range domains {
		domStore.domains[d.ID] = d
		queueStore.domainToProject[d.ID] = project.ID
	}

	schedule := sqlstore.Schedule{
		ID:        "schedule-limit",
		ProjectID: project.ID,
		Name:      "Daily Limit",
		Strategy:  "daily",
		Config:    json.RawMessage(`{"limit":2}`),
		IsActive:  true,
		CreatedBy: "owner@example.com",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := schedStore.Create(context.Background(), schedule); err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/projects/project-3/schedules/schedule-limit/trigger", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleProjectByID(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(queueStore.items) != 2 {
		t.Fatalf("expected 2 queued items, got %d", len(queueStore.items))
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/projects/project-3/schedules/schedule-limit/trigger", nil).WithContext(ctx)
	rec2 := httptest.NewRecorder()
	s.handleProjectByID(rec2, req2)
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if len(queueStore.items) != 2 {
		t.Fatalf("expected queue size to remain 2, got %d", len(queueStore.items))
	}
}

func TestProjectScheduleTriggerDoesNotUpdateScheduleMeta(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	domStore := s.domains.(*stubDomainStore)
	schedStore := s.schedules.(*stubScheduleStore)

	project := sqlstore.Project{
		ID:        "project-meta",
		UserEmail: "owner@example.com",
		Name:      "Meta",
		Status:    "draft",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	projStore.projects[project.ID] = project
	domStore.domains["domain-meta"] = sqlstore.Domain{
		ID:        "domain-meta",
		ProjectID: project.ID,
		URL:       "example.com",
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	lastRun := time.Date(2026, 2, 4, 8, 0, 0, 0, time.UTC)
	nextRun := time.Date(2026, 2, 5, 8, 0, 0, 0, time.UTC)
	schedule := sqlstore.Schedule{
		ID:        "schedule-meta",
		ProjectID: project.ID,
		Name:      "Daily",
		Strategy:  "daily",
		Config:    json.RawMessage(`{"time":"08:00"}`),
		IsActive:  true,
		LastRunAt: sql.NullTime{Time: lastRun, Valid: true},
		NextRunAt: sql.NullTime{Time: nextRun, Valid: true},
		CreatedBy: "owner@example.com",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := schedStore.Create(context.Background(), schedule); err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email:      "owner@example.com",
		Role:       "manager",
		IsApproved: true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/projects/project-meta/schedules/schedule-meta/trigger", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleProjectByID(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	stored := schedStore.schedules[schedule.ID]
	if !stored.LastRunAt.Valid || !stored.LastRunAt.Time.Equal(lastRun) {
		t.Fatalf("last_run_at changed: %v", stored.LastRunAt)
	}
	if !stored.NextRunAt.Valid || !stored.NextRunAt.Time.Equal(nextRun) {
		t.Fatalf("next_run_at changed: %v", stored.NextRunAt)
	}
}

func TestProjectSchedulesSinglePerProject(t *testing.T) {
	s := setupServer(t)

	projStore := s.projects.(*stubProjectStore)
	project := sqlstore.Project{
		ID:        "project-single",
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

	req := httptest.NewRequest(http.MethodPost, "/api/projects/project-single/schedules", strings.NewReader(`{"name":"Daily","strategy":"daily","config":{"time":"10:00"}}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleProjectByID(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/projects/project-single/schedules", strings.NewReader(`{"name":"Weekly","strategy":"weekly","config":{"weekday":"mon","time":"10:00"}}`)).WithContext(ctx)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	s.handleProjectByID(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec2.Code, rec2.Body.String())
	}
}
