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

	if err := processPendingLinkTasks(context.Background(), store, enq, nil); err != nil {
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
}

func TestProcessPendingLinkTasksEnqueueError(t *testing.T) {
	now := time.Now().Add(-time.Minute)
	store := &stubLinkTaskStore{
		tasks: map[string]sqlstore.LinkTask{
			"task-1": {ID: "task-1", Status: "pending", ScheduledFor: now},
		},
	}
	enq := &stubEnqueuer{errFor: map[string]error{"task-1": errors.New("boom")}}

	if err := processPendingLinkTasks(context.Background(), store, enq, nil); err != nil {
		t.Fatalf("process pending: %v", err)
	}

	task := store.tasks["task-1"]
	if task.Status != "pending" {
		t.Fatalf("expected pending, got %s", task.Status)
	}
	if !task.ErrorMessage.Valid {
		t.Fatalf("expected error message set")
	}
}

func TestProcessPendingLinkTasksListError(t *testing.T) {
	store := &stubLinkTaskStore{listErr: errors.New("list failed")}
	enq := &stubEnqueuer{}

	if err := processPendingLinkTasks(context.Background(), store, enq, nil); err == nil {
		t.Fatalf("expected error")
	}
}
