package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

type stubScheduleStore struct {
	schedules map[string]sqlstore.Schedule
}

func (s *stubScheduleStore) ListActive(ctx context.Context) ([]sqlstore.Schedule, error) {
	res := make([]sqlstore.Schedule, 0)
	for _, sched := range s.schedules {
		if sched.IsActive {
			res = append(res, sched)
		}
	}
	return res, nil
}

func (s *stubScheduleStore) Get(ctx context.Context, scheduleID string) (*sqlstore.Schedule, error) {
	sched, ok := s.schedules[scheduleID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	out := sched
	return &out, nil
}

func (s *stubScheduleStore) Update(ctx context.Context, scheduleID string, updates sqlstore.ScheduleUpdates) error {
	sched, ok := s.schedules[scheduleID]
	if !ok {
		return sql.ErrNoRows
	}
	if updates.LastRunAt != nil {
		sched.LastRunAt = *updates.LastRunAt
	}
	if updates.NextRunAt != nil {
		sched.NextRunAt = *updates.NextRunAt
	}
	if updates.Timezone != nil {
		sched.Timezone = *updates.Timezone
	}
	s.schedules[scheduleID] = sched
	return nil
}

type stubGenQueueStore struct {
	items []sqlstore.QueueItem
}

func (s *stubGenQueueStore) ListByProject(ctx context.Context, projectID string) ([]sqlstore.QueueItem, error) {
	var res []sqlstore.QueueItem
	for _, item := range s.items {
		if item.DomainID == "" {
			continue
		}
		if item.Status == "" {
			continue
		}
		if item.ScheduledFor.IsZero() {
			continue
		}
		res = append(res, item)
	}
	return res, nil
}

func (s *stubGenQueueStore) Enqueue(ctx context.Context, item sqlstore.QueueItem) error {
	s.items = append(s.items, item)
	return nil
}

func (s *stubGenQueueStore) GetPending(ctx context.Context, limit int) ([]sqlstore.QueueItem, error) {
	return nil, nil
}

func (s *stubGenQueueStore) MarkProcessed(ctx context.Context, itemID, status string, errorMsg *string) error {
	return nil
}

type stubDomainStore struct {
	byProject map[string][]sqlstore.Domain
}

func (s *stubDomainStore) ListByProject(ctx context.Context, projectID string) ([]sqlstore.Domain, error) {
	return s.byProject[projectID], nil
}

func (s *stubDomainStore) Get(ctx context.Context, id string) (sqlstore.Domain, error) {
	for _, domains := range s.byProject {
		for _, d := range domains {
			if d.ID == id {
				return d, nil
			}
		}
	}
	return sqlstore.Domain{}, sql.ErrNoRows
}

func (s *stubDomainStore) UpdateStatus(ctx context.Context, id, status string) error {
	return nil
}

func (s *stubDomainStore) SetLastGeneration(ctx context.Context, id, genID string) error {
	return nil
}

func TestApplySchedulesImmediateEnqueue(t *testing.T) {
	now := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	cfg := json.RawMessage(`{"limit":1}`)
	sched := sqlstore.Schedule{
		ID:        "sched-1",
		ProjectID: "project-1",
		Name:      "Immediate",
		Strategy:  "immediate",
		Config:    cfg,
		IsActive:  true,
		CreatedBy: "owner@example.com",
	}

	scheduleStore := &stubScheduleStore{schedules: map[string]sqlstore.Schedule{sched.ID: sched}}
	queueStore := &stubGenQueueStore{}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{
			"project-1": {
				{ID: "domain-1", ProjectID: "project-1", URL: "example.com"},
				{ID: "domain-2", ProjectID: "project-1", URL: "example.org"},
			},
		},
	}

	logger := zap.NewNop().Sugar()
	if err := applySchedulesAt(context.Background(), scheduleStore, queueStore, domainStore, logger, now); err != nil {
		t.Fatalf("apply schedules: %v", err)
	}
	if len(queueStore.items) != 1 {
		t.Fatalf("expected 1 queued item, got %d", len(queueStore.items))
	}
	if !queueStore.items[0].ScheduleID.Valid || queueStore.items[0].ScheduleID.String != sched.ID {
		t.Fatalf("expected schedule id on queue item")
	}
	updated := scheduleStore.schedules[sched.ID]
	if !updated.LastRunAt.Valid {
		t.Fatalf("expected last_run_at updated")
	}
}

func TestApplySchedulesDailySkipsSameDay(t *testing.T) {
	now := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	cfg := json.RawMessage(`{"time":"10:00"}`)
	sched := sqlstore.Schedule{
		ID:        "sched-2",
		ProjectID: "project-2",
		Name:      "Daily",
		Strategy:  "daily",
		Config:    cfg,
		IsActive:  true,
		CreatedBy: "owner@example.com",
		LastRunAt: sql.NullTime{Time: now.Add(-time.Hour), Valid: true},
	}

	scheduleStore := &stubScheduleStore{schedules: map[string]sqlstore.Schedule{sched.ID: sched}}
	queueStore := &stubGenQueueStore{
		items: []sqlstore.QueueItem{
			{
				ID:           "item-1",
				DomainID:     "domain-1",
				ScheduleID:   sql.NullString{String: sched.ID, Valid: true},
				ScheduledFor: now,
				Status:       "pending",
			},
		},
	}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{
			"project-2": {
				{ID: "domain-1", ProjectID: "project-2", URL: "example.com"},
			},
		},
	}

	logger := zap.NewNop().Sugar()
	if err := applySchedulesAt(context.Background(), scheduleStore, queueStore, domainStore, logger, now); err != nil {
		t.Fatalf("apply schedules: %v", err)
	}
	if len(queueStore.items) != 1 {
		t.Fatalf("expected no new items, got %d", len(queueStore.items))
	}
	updated := scheduleStore.schedules[sched.ID]
	if !updated.NextRunAt.Valid {
		t.Fatalf("expected next_run_at updated")
	}
}

func TestApplySchedulesCustomEnqueue(t *testing.T) {
	now := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	cfg := json.RawMessage(`{"cron":"* * * * *"}`)
	sched := sqlstore.Schedule{
		ID:        "sched-3",
		ProjectID: "project-3",
		Name:      "Custom",
		Strategy:  "custom",
		Config:    cfg,
		IsActive:  true,
		CreatedBy: "owner@example.com",
	}

	scheduleStore := &stubScheduleStore{schedules: map[string]sqlstore.Schedule{sched.ID: sched}}
	queueStore := &stubGenQueueStore{}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{
			"project-3": {
				{ID: "domain-1", ProjectID: "project-3", URL: "example.com"},
			},
		},
	}

	logger := zap.NewNop().Sugar()
	if err := applySchedulesAt(context.Background(), scheduleStore, queueStore, domainStore, logger, now); err != nil {
		t.Fatalf("apply schedules: %v", err)
	}
	if len(queueStore.items) != 1 {
		t.Fatalf("expected 1 queued item, got %d", len(queueStore.items))
	}
	updated := scheduleStore.schedules[sched.ID]
	if !updated.LastRunAt.Valid {
		t.Fatalf("expected last_run_at updated")
	}
}
