package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
)

type stubLinkTaskStore struct {
	tasks           map[string]sqlstore.LinkTask
	domainToProject map[string]string
	listErr         error
}

func (s *stubLinkTaskStore) ListPending(ctx context.Context, limit int) ([]sqlstore.LinkTask, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	var res []sqlstore.LinkTask
	for _, task := range s.tasks {
		if task.Status != "pending" {
			continue
		}
		if task.ScheduledFor.After(time.Now().UTC()) {
			continue
		}
		res = append(res, task)
	}
	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

func (s *stubLinkTaskStore) ListByProject(ctx context.Context, projectID string, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error) {
	var res []sqlstore.LinkTask
	for _, task := range s.tasks {
		if s.domainToProject != nil && s.domainToProject[task.DomainID] != projectID {
			continue
		}
		if filters.Status != nil && task.Status != *filters.Status {
			continue
		}
		res = append(res, task)
		if filters.Limit > 0 && len(res) >= filters.Limit {
			break
		}
	}
	return res, nil
}

func (s *stubLinkTaskStore) Create(ctx context.Context, task sqlstore.LinkTask) error {
	if s.tasks == nil {
		s.tasks = make(map[string]sqlstore.LinkTask)
	}
	s.tasks[task.ID] = task
	return nil
}

func (s *stubLinkTaskStore) Update(ctx context.Context, taskID string, updates sqlstore.LinkTaskUpdates) error {
	task, ok := s.tasks[taskID]
	if !ok {
		return sql.ErrNoRows
	}
	if updates.Status != nil {
		task.Status = *updates.Status
	}
	if updates.AnchorText != nil {
		task.AnchorText = *updates.AnchorText
	}
	if updates.TargetURL != nil {
		task.TargetURL = *updates.TargetURL
	}
	if updates.Attempts != nil {
		task.Attempts = *updates.Attempts
	}
	if updates.ScheduledFor != nil {
		task.ScheduledFor = *updates.ScheduledFor
	}
	if updates.CreatedAt != nil {
		task.CreatedAt = *updates.CreatedAt
	}
	if updates.FoundLocation != nil {
		task.FoundLocation = *updates.FoundLocation
	}
	if updates.GeneratedContent != nil {
		task.GeneratedContent = *updates.GeneratedContent
	}
	if updates.ErrorMessage != nil {
		task.ErrorMessage = *updates.ErrorMessage
	}
	if updates.CompletedAt != nil {
		task.CompletedAt = *updates.CompletedAt
	}
	if updates.LogLines != nil {
		task.LogLines = *updates.LogLines
	}
	s.tasks[taskID] = task
	return nil
}

type stubEnqueuer struct {
	calls  []string
	errFor map[string]error
}

func (s *stubEnqueuer) Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	payload, err := tasks.ParseLinkTaskPayload(task)
	if err != nil {
		return nil, err
	}
	s.calls = append(s.calls, payload.TaskID)
	if s.errFor != nil {
		if err := s.errFor[payload.TaskID]; err != nil {
			return nil, err
		}
	}
	return &asynq.TaskInfo{ID: payload.TaskID}, nil
}

func TestProcessPendingLinkTasksEnqueue(t *testing.T) {
	now := time.Now().Add(-time.Minute)
	store := &stubLinkTaskStore{
		tasks: map[string]sqlstore.LinkTask{
			"task-1": {ID: "task-1", Status: "pending", ScheduledFor: now},
			"task-2": {ID: "task-2", Status: "pending", ScheduledFor: now},
		},
	}
	enq := &stubEnqueuer{}

	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{
			"project-1": {
				{ID: "domain-a", ProjectID: "project-1"},
				{ID: "domain-b", ProjectID: "project-1"},
			},
		},
	}
	store.tasks["task-1"] = sqlstore.LinkTask{ID: "task-1", Status: "pending", ScheduledFor: now, DomainID: "domain-a"}
	store.tasks["task-2"] = sqlstore.LinkTask{ID: "task-2", Status: "pending", ScheduledFor: now, DomainID: "domain-b"}

	if err := processPendingLinkTasks(context.Background(), store, domainStore, enq, 4, nil); err != nil {
		t.Fatalf("process pending: %v", err)
	}

	if len(enq.calls) != 2 {
		t.Fatalf("expected 2 enqueues, got %d", len(enq.calls))
	}
	if store.tasks["task-1"].Status != "searching" {
		t.Fatalf("expected task-1 searching, got %s", store.tasks["task-1"].Status)
	}
	if store.tasks["task-2"].Status != "searching" {
		t.Fatalf("expected task-2 searching, got %s", store.tasks["task-2"].Status)
	}
	if domainStore.linkStatus["domain-a"] != "searching" {
		t.Fatalf("expected domain-a searching, got %q", domainStore.linkStatus["domain-a"])
	}
	if domainStore.linkStatus["domain-b"] != "searching" {
		t.Fatalf("expected domain-b searching, got %q", domainStore.linkStatus["domain-b"])
	}
}

func TestProcessPendingLinkTasksEnqueueError(t *testing.T) {
	now := time.Now().Add(-time.Minute)
	store := &stubLinkTaskStore{
		tasks: map[string]sqlstore.LinkTask{
			"task-1": {ID: "task-1", Status: "pending", ScheduledFor: now},
		},
	}
	enq := &stubEnqueuer{errFor: map[string]error{"task-1": errors.New("boom")}}

	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{
			"project-1": {
				{ID: "domain-a", ProjectID: "project-1"},
			},
		},
	}
	store.tasks["task-1"] = sqlstore.LinkTask{ID: "task-1", Status: "pending", ScheduledFor: now, DomainID: "domain-a"}

	if err := processPendingLinkTasks(context.Background(), store, domainStore, enq, 4, nil); err != nil {
		t.Fatalf("process pending: %v", err)
	}

	task := store.tasks["task-1"]
	if task.Status != "pending" {
		t.Fatalf("expected pending, got %s", task.Status)
	}
	if !task.ErrorMessage.Valid {
		t.Fatalf("expected error message set")
	}
	if domainStore.linkStatus["domain-a"] != "pending" {
		t.Fatalf("expected domain-a pending after enqueue error, got %q", domainStore.linkStatus["domain-a"])
	}
}

func TestProcessPendingLinkTasksListError(t *testing.T) {
	store := &stubLinkTaskStore{listErr: errors.New("list failed")}
	enq := &stubEnqueuer{}

	if err := processPendingLinkTasks(context.Background(), store, nil, enq, 4, nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListActiveLinkTasksByProjectSkipsLegacyFound(t *testing.T) {
	now := time.Now().UTC()
	store := &stubLinkTaskStore{
		tasks: map[string]sqlstore.LinkTask{
			"pending":   {ID: "pending", DomainID: "domain-a", Status: "pending", ScheduledFor: now},
			"searching": {ID: "searching", DomainID: "domain-a", Status: "searching", ScheduledFor: now},
			"removing":  {ID: "removing", DomainID: "domain-a", Status: "removing", ScheduledFor: now},
			"found":     {ID: "found", DomainID: "domain-a", Status: "found", ScheduledFor: now},
		},
		domainToProject: map[string]string{"domain-a": "project-1"},
	}

	items, err := listActiveLinkTasksByProject(context.Background(), store, "project-1")
	if err != nil {
		t.Fatalf("list active tasks: %v", err)
	}
	ids := map[string]bool{}
	for _, item := range items {
		ids[item.ID] = true
	}
	if ids["found"] {
		t.Fatalf("legacy found status must be ignored in active list")
	}
	if !ids["pending"] || !ids["searching"] || !ids["removing"] {
		t.Fatalf("expected pending/searching/removing in active list, got %#v", ids)
	}
}
