package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

type stubLinkScheduleStore struct {
	schedules []sqlstore.LinkSchedule
}

func (s *stubLinkScheduleStore) ListActive(ctx context.Context) ([]sqlstore.LinkSchedule, error) {
	return s.schedules, nil
}

func (s *stubLinkScheduleStore) Update(ctx context.Context, scheduleID string, updates sqlstore.LinkScheduleUpdates) error {
	for i, sched := range s.schedules {
		if sched.ID != scheduleID {
			continue
		}
		if updates.Name != nil {
			sched.Name = *updates.Name
		}
		if updates.Config != nil {
			sched.Config = append([]byte(nil), (*updates.Config)...)
		}
		if updates.IsActive != nil {
			sched.IsActive = *updates.IsActive
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
		s.schedules[i] = sched
		return nil
	}
	return sql.ErrNoRows
}

func TestApplyLinkSchedulesUpsertByLimitAndEligibility(t *testing.T) {
	now := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	schedule := sqlstore.LinkSchedule{
		ID:        "link-s-1",
		ProjectID: "project-1",
		Name:      "Links",
		Config:    json.RawMessage(`{"limit":2}`),
		IsActive:  true,
		CreatedBy: "owner@example.com",
		CreatedAt: now,
	}
	scheduleStore := &stubLinkScheduleStore{[]sqlstore.LinkSchedule{schedule}}

	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{
			"project-1": {
				{
					ID:              "domain-a",
					ProjectID:       "project-1",
					URL:             "a.example.com",
					Status:          "published",
					LinkAnchorText:  sql.NullString{String: "Anchor A", Valid: true},
					LinkAcceptorURL: sql.NullString{String: "https://acceptor/a", Valid: true},
				},
				{
					ID:              "domain-b",
					ProjectID:       "project-1",
					URL:             "b.example.com",
					Status:          "published",
					LinkAnchorText:  sql.NullString{String: "Anchor B", Valid: true},
					LinkAcceptorURL: sql.NullString{String: "https://acceptor/b", Valid: true},
				},
				{
					ID:              "domain-c",
					ProjectID:       "project-1",
					URL:             "c.example.com",
					Status:          "published",
					LinkAnchorText:  sql.NullString{String: "Anchor C", Valid: true},
					LinkAcceptorURL: sql.NullString{String: "https://acceptor/c", Valid: true},
				},
				{
					ID:              "domain-d",
					ProjectID:       "project-1",
					URL:             "d.example.com",
					Status:          "draft",
					LinkAnchorText:  sql.NullString{String: "Anchor D", Valid: true},
					LinkAcceptorURL: sql.NullString{String: "https://acceptor/d", Valid: true},
				},
				{
					ID:              "domain-e",
					ProjectID:       "project-1",
					URL:             "e.example.com",
					Status:          "published",
					LinkAnchorText:  sql.NullString{},
					LinkAcceptorURL: sql.NullString{String: "https://acceptor/e", Valid: true},
				},
			},
		},
	}

	linkTaskStore := &stubLinkTaskStore{
		tasks:           make(map[string]sqlstore.LinkTask),
		domainToProject: make(map[string]string),
	}
	linkTaskStore.domainToProject["domain-a"] = "project-1"
	linkTaskStore.domainToProject["domain-b"] = "project-1"
	linkTaskStore.domainToProject["domain-c"] = "project-1"
	linkTaskStore.domainToProject["domain-d"] = "project-1"
	linkTaskStore.domainToProject["domain-e"] = "project-1"

	linkTaskStore.tasks["task-a"] = sqlstore.LinkTask{
		ID:           "task-a",
		DomainID:     "domain-a",
		AnchorText:   "Old Anchor",
		TargetURL:    "https://old/a",
		ScheduledFor: now.Add(-time.Hour),
		Status:       "pending",
		CreatedBy:    "owner@example.com",
		CreatedAt:    now.Add(-time.Hour),
	}

	if err := applyLinkSchedulesAt(context.Background(), scheduleStore, linkTaskStore, domainStore, nil, now); err != nil {
		t.Fatalf("apply link schedules: %v", err)
	}

	if len(linkTaskStore.tasks) != 2 {
		t.Fatalf("expected 2 tasks after upsert, got %d", len(linkTaskStore.tasks))
	}
	if !scheduleStore.schedules[0].LastRunAt.Valid {
		t.Fatalf("expected last_run_at to be updated")
	}
	if !scheduleStore.schedules[0].NextRunAt.Valid {
		t.Fatalf("expected next_run_at to be updated")
	}

	updated := linkTaskStore.tasks["task-a"]
	if updated.AnchorText != "Anchor A" || updated.TargetURL != "https://acceptor/a" {
		t.Fatalf("expected task-a updated with latest link data, got %q -> %q", updated.AnchorText, updated.TargetURL)
	}

	var createdForB *sqlstore.LinkTask
	for _, task := range linkTaskStore.tasks {
		if task.DomainID == "domain-b" {
			copy := task
			createdForB = &copy
			break
		}
	}
	if createdForB == nil {
		t.Fatalf("expected task for domain-b")
	}
	if createdForB.Status != "pending" {
		t.Fatalf("expected pending status for domain-b, got %s", createdForB.Status)
	}

	for _, task := range linkTaskStore.tasks {
		if task.DomainID == "domain-c" {
			t.Fatalf("did not expect task for domain-c due to limit")
		}
		if task.DomainID == "domain-d" {
			t.Fatalf("did not expect task for unpublished domain")
		}
		if task.DomainID == "domain-e" {
			t.Fatalf("did not expect task for missing anchor text")
		}
	}
}
