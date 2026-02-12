package indexchecker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

type stubProjectStore struct {
	projects []sqlstore.Project
	err      error
}

func (s *stubProjectStore) ListAll(ctx context.Context) ([]sqlstore.Project, error) {
	return s.projects, s.err
}

type stubDomainStore struct {
	byProject map[string][]sqlstore.Domain
	err       error
}

func (s *stubDomainStore) ListByProject(ctx context.Context, projectID string) ([]sqlstore.Domain, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.byProject[projectID], nil
}

type updateCall struct {
	checkID   string
	status    string
	isIndexed *bool
	errMsg    *string
}

type nextRetryCall struct {
	checkID   string
	nextRetry time.Time
}

type stubIndexCheckStore struct {
	getByDomainErr   error
	getByDomainCheck *sqlstore.IndexCheck
	created          *sqlstore.IndexCheck
	pendingRetries   []sqlstore.IndexCheck
	updateCalls      []updateCall
	incremented      []string
	nextRetries      []nextRetryCall
}

func (s *stubIndexCheckStore) Create(ctx context.Context, check sqlstore.IndexCheck) error {
	s.created = &check
	return nil
}

func (s *stubIndexCheckStore) Get(ctx context.Context, checkID string) (*sqlstore.IndexCheck, error) {
	return nil, errors.New("not implemented")
}

func (s *stubIndexCheckStore) GetByDomainAndDate(ctx context.Context, domainID string, date time.Time) (*sqlstore.IndexCheck, error) {
	if s.getByDomainErr != nil {
		return nil, s.getByDomainErr
	}
	return s.getByDomainCheck, nil
}

func (s *stubIndexCheckStore) ListByDomain(ctx context.Context, domainID string, limit int) ([]sqlstore.IndexCheck, error) {
	return nil, errors.New("not implemented")
}

func (s *stubIndexCheckStore) ListPendingRetries(ctx context.Context) ([]sqlstore.IndexCheck, error) {
	return s.pendingRetries, nil
}

func (s *stubIndexCheckStore) UpdateStatus(ctx context.Context, checkID string, status string, isIndexed *bool, errMsg *string) error {
	s.updateCalls = append(s.updateCalls, updateCall{
		checkID:   checkID,
		status:    status,
		isIndexed: isIndexed,
		errMsg:    errMsg,
	})
	return nil
}

func (s *stubIndexCheckStore) IncrementAttempts(ctx context.Context, checkID string) error {
	s.incremented = append(s.incremented, checkID)
	return nil
}

func (s *stubIndexCheckStore) SetNextRetry(ctx context.Context, checkID string, nextRetry time.Time) error {
	s.nextRetries = append(s.nextRetries, nextRetryCall{checkID: checkID, nextRetry: nextRetry})
	return nil
}

type stubHistoryStore struct {
	created []sqlstore.CheckHistory
	err     error
}

func (s *stubHistoryStore) Create(ctx context.Context, history sqlstore.CheckHistory) error {
	if s.err != nil {
		return s.err
	}
	s.created = append(s.created, history)
	return nil
}

type stubChecker struct {
	indexed bool
	err     error
}

func (s *stubChecker) Check(ctx context.Context, domain, geo string) (bool, error) {
	return s.indexed, s.err
}

func TestRunIndexCheckerTickSuccess(t *testing.T) {
	now := time.Now().UTC()
	projectStore := &stubProjectStore{
		projects: []sqlstore.Project{{ID: "proj-1", TargetCountry: "SE"}},
	}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{
			"proj-1": {{ID: "dom-1", ProjectID: "proj-1", URL: "example.com"}},
		},
	}
	checkStore := &stubIndexCheckStore{
		getByDomainErr: sql.ErrNoRows,
	}
	historyStore := &stubHistoryStore{}
	checker := &stubChecker{indexed: true}

	logger := zap.NewNop().Sugar()
	if err := RunIndexCheckerTick(context.Background(), now, projectStore, domainStore, checkStore, historyStore, checker, logger); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	if checkStore.created == nil {
		t.Fatal("expected check to be created")
	}
	if len(historyStore.created) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyStore.created))
	}
	h := historyStore.created[0]
	if h.CheckID != checkStore.created.ID {
		t.Fatalf("unexpected history check_id: %s", h.CheckID)
	}
	if !h.Result.Valid || h.Result.String != "success" {
		t.Fatalf("unexpected history result: %#v", h.Result)
	}
	var payload map[string]any
	if err := json.Unmarshal(h.ResponseData, &payload); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if indexed, ok := payload["indexed"].(bool); !ok || !indexed {
		t.Fatalf("unexpected response data: %#v", payload)
	}
	if len(checkStore.updateCalls) != 1 || checkStore.updateCalls[0].status != "success" {
		t.Fatalf("expected status update to success, got %#v", checkStore.updateCalls)
	}
	if checkStore.updateCalls[0].isIndexed == nil || !*checkStore.updateCalls[0].isIndexed {
		t.Fatalf("expected is_indexed=true, got %#v", checkStore.updateCalls[0].isIndexed)
	}
	if len(checkStore.incremented) != 1 || checkStore.incremented[0] != checkStore.created.ID {
		t.Fatalf("expected attempts increment for %s, got %#v", checkStore.created.ID, checkStore.incremented)
	}
}

func TestRunIndexCheckerTickNotIndexed(t *testing.T) {
	now := time.Now().UTC()
	projectStore := &stubProjectStore{
		projects: []sqlstore.Project{{ID: "proj-1", TargetCountry: "SE"}},
	}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{
			"proj-1": {{ID: "dom-1", ProjectID: "proj-1", URL: "example.com"}},
		},
	}
	checkStore := &stubIndexCheckStore{
		getByDomainErr: sql.ErrNoRows,
	}
	historyStore := &stubHistoryStore{}
	checker := &stubChecker{indexed: false}

	logger := zap.NewNop().Sugar()
	if err := RunIndexCheckerTick(context.Background(), now, projectStore, domainStore, checkStore, historyStore, checker, logger); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	if len(historyStore.created) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyStore.created))
	}
	h := historyStore.created[0]
	var payload map[string]any
	if err := json.Unmarshal(h.ResponseData, &payload); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if indexed, ok := payload["indexed"].(bool); !ok || indexed {
		t.Fatalf("expected indexed=false, got %#v", payload)
	}
	if len(checkStore.updateCalls) != 1 || checkStore.updateCalls[0].status != "success" {
		t.Fatalf("expected status update to success, got %#v", checkStore.updateCalls)
	}
	if checkStore.updateCalls[0].isIndexed == nil || *checkStore.updateCalls[0].isIndexed {
		t.Fatalf("expected is_indexed=false, got %#v", checkStore.updateCalls[0].isIndexed)
	}
	if len(checkStore.incremented) != 1 || checkStore.incremented[0] != checkStore.created.ID {
		t.Fatalf("expected attempts increment for %s, got %#v", checkStore.created.ID, checkStore.incremented)
	}
}

func TestRunIndexCheckerTickError(t *testing.T) {
	now := time.Now().UTC()
	projectStore := &stubProjectStore{
		projects: []sqlstore.Project{{ID: "proj-1", TargetCountry: "SE"}},
	}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{
			"proj-1": {{ID: "dom-1", ProjectID: "proj-1", URL: "example.com"}},
		},
	}
	check := &sqlstore.IndexCheck{
		ID:        "check-1",
		DomainID:  "dom-1",
		CheckDate: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Status:    "pending",
		Attempts:  0,
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	checkStore := &stubIndexCheckStore{
		getByDomainCheck: check,
	}
	historyStore := &stubHistoryStore{}
	checker := &stubChecker{err: errors.New("boom")}

	logger := zap.NewNop().Sugar()
	if err := RunIndexCheckerTick(context.Background(), now, projectStore, domainStore, checkStore, historyStore, checker, logger); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	if len(historyStore.created) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyStore.created))
	}
	h := historyStore.created[0]
	if !h.Result.Valid || h.Result.String != "error" {
		t.Fatalf("unexpected history result: %#v", h.Result)
	}
	if !h.ErrorMessage.Valid || h.ErrorMessage.String != "boom" {
		t.Fatalf("unexpected history error message: %#v", h.ErrorMessage)
	}
	if len(checkStore.incremented) != 1 || checkStore.incremented[0] != "check-1" {
		t.Fatalf("expected attempts incremented, got %#v", checkStore.incremented)
	}
	if len(checkStore.nextRetries) != 1 {
		t.Fatalf("expected next retry set, got %#v", checkStore.nextRetries)
	}
	expectedNext := now.Add(30 * time.Minute)
	diff := checkStore.nextRetries[0].nextRetry.Sub(expectedNext)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Fatalf("unexpected next retry time: %v", checkStore.nextRetries[0].nextRetry)
	}
	if len(checkStore.updateCalls) != 1 || checkStore.updateCalls[0].status != "checking" {
		t.Fatalf("expected status update to checking, got %#v", checkStore.updateCalls)
	}
	if checkStore.updateCalls[0].errMsg == nil || *checkStore.updateCalls[0].errMsg != "boom" {
		t.Fatalf("expected error message to be propagated, got %#v", checkStore.updateCalls[0].errMsg)
	}
}
