package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

type stubLLMUsageStore struct {
	mu              sync.Mutex
	events          []sqlstore.LLMUsageEvent
	stats           sqlstore.LLMUsageStats
	lastListFilters sqlstore.LLMUsageFilters
	lastCountFilter sqlstore.LLMUsageFilters
	lastStatsFilter sqlstore.LLMUsageFilters
}

func (s *stubLLMUsageStore) CreateEvent(context.Context, sqlstore.LLMUsageEvent) error { return nil }

func (s *stubLLMUsageStore) ListEvents(_ context.Context, filters sqlstore.LLMUsageFilters) ([]sqlstore.LLMUsageEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastListFilters = filters
	filtered := s.filteredEvents(filters, false)
	return append([]sqlstore.LLMUsageEvent(nil), filtered...), nil
}

func (s *stubLLMUsageStore) CountEvents(_ context.Context, filters sqlstore.LLMUsageFilters) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastCountFilter = filters
	return len(s.filteredEvents(filters, true)), nil
}

func (s *stubLLMUsageStore) AggregateStats(_ context.Context, filters sqlstore.LLMUsageFilters) (sqlstore.LLMUsageStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastStatsFilter = filters
	return s.stats, nil
}

func (s *stubLLMUsageStore) filteredEvents(filters sqlstore.LLMUsageFilters, noPaging bool) []sqlstore.LLMUsageEvent {
	out := make([]sqlstore.LLMUsageEvent, 0, len(s.events))
	for _, item := range s.events {
		if filters.From != nil && item.CreatedAt.Before(*filters.From) {
			continue
		}
		if filters.To != nil && item.CreatedAt.After(*filters.To) {
			continue
		}
		if filters.UserEmail != nil && item.RequesterEmail != *filters.UserEmail {
			continue
		}
		if filters.ProjectID != nil && (!item.ProjectID.Valid || item.ProjectID.String != *filters.ProjectID) {
			continue
		}
		if filters.DomainID != nil && (!item.DomainID.Valid || item.DomainID.String != *filters.DomainID) {
			continue
		}
		if filters.Model != nil && item.Model != *filters.Model {
			continue
		}
		if filters.Operation != nil && item.Operation != *filters.Operation {
			continue
		}
		if filters.Status != nil && item.Status != *filters.Status {
			continue
		}
		out = append(out, item)
	}
	if noPaging {
		return out
	}
	limit := filters.Limit
	if limit <= 0 {
		limit = len(out)
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(out) {
		return []sqlstore.LLMUsageEvent{}
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end]
}

type stubModelPricingStore struct {
	mu         sync.Mutex
	items      []sqlstore.LLMModelPricing
	lastUpsert struct {
		provider string
		model    string
		input    float64
		output   float64
		updated  string
		at       time.Time
	}
}

func (s *stubModelPricingStore) GetActiveByModel(_ context.Context, provider, model string, at time.Time) (*sqlstore.LLMModelPricing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.items {
		if item.Provider != provider || item.Model != model || !item.IsActive {
			continue
		}
		if !item.ActiveFrom.IsZero() && item.ActiveFrom.After(at) {
			continue
		}
		if item.ActiveTo.Valid && !item.ActiveTo.Time.After(at) {
			continue
		}
		out := item
		return &out, nil
	}
	return nil, sql.ErrNoRows
}

func (s *stubModelPricingStore) ListActive(context.Context) ([]sqlstore.LLMModelPricing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	active := make([]sqlstore.LLMModelPricing, 0, len(s.items))
	for _, item := range s.items {
		if item.IsActive {
			active = append(active, item)
		}
	}
	return active, nil
}

func (s *stubModelPricingStore) UpsertActive(_ context.Context, provider, model string, inputUSDPerMillion, outputUSDPerMillion float64, updatedBy string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastUpsert.provider = provider
	s.lastUpsert.model = model
	s.lastUpsert.input = inputUSDPerMillion
	s.lastUpsert.output = outputUSDPerMillion
	s.lastUpsert.updated = updatedBy
	s.lastUpsert.at = at.UTC()
	for i := range s.items {
		if s.items[i].Provider == provider && s.items[i].Model == model {
			s.items[i].IsActive = false
			s.items[i].ActiveTo = sql.NullTime{Time: at.UTC(), Valid: true}
		}
	}
	s.items = append(s.items, sqlstore.LLMModelPricing{
		ID:                  "pricing-new",
		Provider:            provider,
		Model:               model,
		InputUSDPerMillion:  inputUSDPerMillion,
		OutputUSDPerMillion: outputUSDPerMillion,
		ActiveFrom:          at.UTC(),
		IsActive:            true,
		UpdatedBy:           updatedBy,
		UpdatedAt:           at.UTC(),
	})
	return nil
}

func TestAdminLLMUsageEventsAuthFiltersAndPagination(t *testing.T) {
	s := setupServer(t)
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	usage := &stubLLMUsageStore{
		events: []sqlstore.LLMUsageEvent{
			{
				ID:             "event-1",
				Provider:       "gemini",
				Operation:      "editor_ai_suggest",
				Model:          "gemini-2.5-flash",
				Status:         "success",
				RequesterEmail: "manager@example.com",
				ProjectID:      sqlstore.NullableString("project-1"),
				CreatedAt:      now,
			},
			{
				ID:             "event-2",
				Provider:       "gemini",
				Operation:      "editor_ai_suggest",
				Model:          "gemini-2.5-flash",
				Status:         "success",
				RequesterEmail: "manager@example.com",
				ProjectID:      sqlstore.NullableString("project-1"),
				CreatedAt:      now.Add(-time.Hour),
			},
			{
				ID:             "event-3",
				Provider:       "gemini",
				Operation:      "generation_step",
				Model:          "gemini-2.5-pro",
				Status:         "error",
				RequesterEmail: "other@example.com",
				ProjectID:      sqlstore.NullableString("project-2"),
				CreatedAt:      now.Add(-2 * time.Hour),
			},
		},
	}
	s.llmUsage = usage
	handler := s.requireAdmin(http.HandlerFunc(s.handleAdminLLMUsageEvents))

	t.Run("forbidden for non-admin", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
			Email: "manager@example.com",
			Role:  "user",
		})
		req := httptest.NewRequest(http.MethodGet, "/api/admin/llm-usage/events", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("admin receives filtered and paginated list", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
			Email: "admin@example.com",
			Role:  "admin",
		})
		req := httptest.NewRequest(
			http.MethodGet,
			"/api/admin/llm-usage/events?page=2&limit=1&user_email=manager@example.com&model=gemini-2.5-flash&operation=editor_ai_suggest&status=success&from=2026-02-01&to=2026-02-28",
			nil,
		).WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp llmUsageListDTO
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Total != 2 {
			t.Fatalf("expected total=2, got %d", resp.Total)
		}
		if len(resp.Items) != 1 || resp.Items[0].ID != "event-2" {
			t.Fatalf("expected second page with event-2, got %+v", resp.Items)
		}
		if usage.lastListFilters.Limit != 1 || usage.lastListFilters.Offset != 1 {
			t.Fatalf("unexpected pagination filters: %+v", usage.lastListFilters)
		}
		if usage.lastListFilters.ProjectID != nil {
			t.Fatalf("admin list should not force project filter")
		}
	})
}

func TestAdminLLMUsageStatsGroupBy(t *testing.T) {
	s := setupServer(t)
	s.llmUsage = &stubLLMUsageStore{
		stats: sqlstore.LLMUsageStats{
			Totals: sqlstore.LLMUsageTotalStats{
				TotalRequests: 10,
				TotalTokens:   2000,
				TotalCostUSD:  1.25,
			},
			ByDay:       []sqlstore.LLMUsageStatsBucket{{Key: "2026-02-25", Requests: 10, Tokens: 2000, CostUSD: 1.25}},
			ByModel:     []sqlstore.LLMUsageStatsBucket{{Key: "gemini-2.5-flash", Requests: 8, Tokens: 1500, CostUSD: 0.7}},
			ByOperation: []sqlstore.LLMUsageStatsBucket{{Key: "editor_ai_suggest", Requests: 5, Tokens: 900, CostUSD: 0.4}},
			ByUser:      []sqlstore.LLMUsageStatsBucket{{Key: "admin@example.com", Requests: 6, Tokens: 1000, CostUSD: 0.6}},
		},
	}
	handler := s.requireAdmin(http.HandlerFunc(s.handleAdminLLMUsageStats))
	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/llm-usage/stats?group_by=model,operation", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp llmUsageStatsDTO
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.ByModel) == 0 || len(resp.ByOperation) == 0 {
		t.Fatalf("expected grouped model+operation data")
	}
	if len(resp.ByDay) != 0 || len(resp.ByUser) != 0 {
		t.Fatalf("unexpected non-requested groups: by_day=%d by_user=%d", len(resp.ByDay), len(resp.ByUser))
	}
}

func TestAdminLLMPricingGetAndPut(t *testing.T) {
	s := setupServer(t)
	now := time.Date(2026, 2, 25, 14, 0, 0, 0, time.UTC)
	pricing := &stubModelPricingStore{
		items: []sqlstore.LLMModelPricing{
			{
				ID:                  "pricing-1",
				Provider:            "gemini",
				Model:               "gemini-2.5-flash",
				InputUSDPerMillion:  0.3,
				OutputUSDPerMillion: 0.6,
				ActiveFrom:          now.Add(-24 * time.Hour),
				IsActive:            true,
				UpdatedBy:           "seed",
				UpdatedAt:           now.Add(-24 * time.Hour),
			},
		},
	}
	s.modelPricing = pricing

	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	getHandler := s.requireAdmin(http.HandlerFunc(s.handleAdminLLMPricing))
	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/llm-pricing", nil).WithContext(adminCtx)
	getRec := httptest.NewRecorder()
	getHandler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var list []llmPricingDTO
	if err := json.NewDecoder(getRec.Body).Decode(&list); err != nil {
		t.Fatalf("decode pricing list: %v", err)
	}
	if len(list) != 1 || list[0].Model != "gemini-2.5-flash" {
		t.Fatalf("unexpected pricing list: %+v", list)
	}

	putHandler := s.requireAdmin(http.HandlerFunc(s.handleAdminLLMPricingByModel))
	putReq := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/llm-pricing/gemini-2.5-flash-image",
		strings.NewReader(`{"provider":"gemini","input_usd_per_million":0.4,"output_usd_per_million":30}`),
	).WithContext(adminCtx)
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	putHandler.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", putRec.Code, putRec.Body.String())
	}
	if pricing.lastUpsert.model != "gemini-2.5-flash-image" || pricing.lastUpsert.updated != "admin@example.com" {
		t.Fatalf("unexpected upsert capture: %+v", pricing.lastUpsert)
	}
}

func TestProjectLLMUsageEventsForOwnerAndManager(t *testing.T) {
	s := setupServer(t)
	const (
		projectID = "project-llm-1"
		owner     = "owner@example.com"
		manager   = "manager@example.com"
	)
	now := time.Date(2026, 2, 25, 16, 0, 0, 0, time.UTC)
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: owner,
		Name:      "LLM usage project",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	usage := &stubLLMUsageStore{
		events: []sqlstore.LLMUsageEvent{
			{ID: "event-p1", ProjectID: sqlstore.NullableString(projectID), RequesterEmail: owner, Model: "gemini-2.5-flash", Operation: "editor_ai_suggest", Status: "success", CreatedAt: now},
			{ID: "event-p2", ProjectID: sqlstore.NullableString(projectID), RequesterEmail: manager, Model: "gemini-2.5-flash", Operation: "editor_ai_suggest", Status: "success", CreatedAt: now.Add(-time.Minute)},
			{ID: "event-other", ProjectID: sqlstore.NullableString("other-project"), RequesterEmail: manager, Model: "gemini-2.5-flash", Operation: "editor_ai_suggest", Status: "success", CreatedAt: now.Add(-2 * time.Minute)},
		},
	}
	s.llmUsage = usage

	t.Run("owner can access own project and project_id query is ignored", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
			Email: owner,
			Role:  "user",
		})
		req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/llm-usage/events?project_id=other-project&limit=50&page=1", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		s.handleProjectByID(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if usage.lastListFilters.ProjectID == nil || *usage.lastListFilters.ProjectID != projectID {
			t.Fatalf("project filter must be forced from path, got %+v", usage.lastListFilters.ProjectID)
		}
		var resp llmUsageListDTO
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Total != 2 {
			t.Fatalf("expected total=2 for project scope, got %d", resp.Total)
		}
	})

	t.Run("manager member can access project scope", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock new: %v", err)
		}
		defer db.Close()
		s.projectMembers = sqlstore.NewProjectMemberStore(db)

		query := regexp.QuoteMeta("SELECT project_id, user_email, role, created_at FROM project_members WHERE project_id=$1 AND user_email=$2")
		memberRows := sqlmock.NewRows([]string{"project_id", "user_email", "role", "created_at"}).
			AddRow(projectID, manager, "editor", now)
		mock.ExpectQuery(query).WithArgs(projectID, manager).WillReturnRows(memberRows)
		memberRowsSecond := sqlmock.NewRows([]string{"project_id", "user_email", "role", "created_at"}).
			AddRow(projectID, manager, "editor", now)
		mock.ExpectQuery(query).WithArgs(projectID, manager).WillReturnRows(memberRowsSecond)

		ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
			Email: manager,
			Role:  "user",
		})
		req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/llm-usage/events", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		s.handleProjectByID(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})
}

func TestProjectLLMUsageEventsForbiddenForNonMember(t *testing.T) {
	s := setupServer(t)
	const projectID = "project-llm-2"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Forbidden project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.llmUsage = &stubLLMUsageStore{}

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "stranger@example.com",
		Role:  "user",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/llm-usage/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleProjectByID(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode forbidden payload: %v", err)
	}
	if payload["code"] != "forbidden" {
		t.Fatalf("expected code forbidden, got %v", payload["code"])
	}
}

func TestProjectLLMUsageStatsGroupBy(t *testing.T) {
	s := setupServer(t)
	const (
		projectID = "project-llm-stats"
		owner     = "owner-stats@example.com"
	)
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: owner,
		Name:      "Stats project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	usage := &stubLLMUsageStore{
		stats: sqlstore.LLMUsageStats{
			Totals:      sqlstore.LLMUsageTotalStats{TotalRequests: 2, TotalTokens: 100, TotalCostUSD: 0.2},
			ByDay:       []sqlstore.LLMUsageStatsBucket{{Key: "2026-02-25", Requests: 2, Tokens: 100, CostUSD: 0.2}},
			ByModel:     []sqlstore.LLMUsageStatsBucket{{Key: "gemini-2.5-flash", Requests: 2, Tokens: 100, CostUSD: 0.2}},
			ByOperation: []sqlstore.LLMUsageStatsBucket{{Key: "generation_step", Requests: 2, Tokens: 100, CostUSD: 0.2}},
			ByUser:      []sqlstore.LLMUsageStatsBucket{{Key: owner, Requests: 2, Tokens: 100, CostUSD: 0.2}},
		},
	}
	s.llmUsage = usage

	ctx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: owner,
		Role:  "user",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/llm-usage/stats?group_by=user", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.handleProjectByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if usage.lastStatsFilter.ProjectID == nil || *usage.lastStatsFilter.ProjectID != projectID {
		t.Fatalf("expected forced project_id filter, got %+v", usage.lastStatsFilter.ProjectID)
	}
	if usage.lastStatsFilter.Limit != 0 || usage.lastStatsFilter.Offset != 0 {
		t.Fatalf("expected stats to reset pagination, got %+v", usage.lastStatsFilter)
	}
	var resp llmUsageStatsDTO
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.ByUser) == 0 {
		t.Fatalf("expected by_user bucket in response")
	}
	if len(resp.ByDay) != 0 || len(resp.ByModel) != 0 || len(resp.ByOperation) != 0 {
		t.Fatalf("only by_user should be present, got by_day=%d by_model=%d by_operation=%d", len(resp.ByDay), len(resp.ByModel), len(resp.ByOperation))
	}
}
