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
	byID     map[string]sqlstore.Project
	err      error
}

func (s *stubProjectStore) ListAll(ctx context.Context) ([]sqlstore.Project, error) {
	return s.projects, s.err
}

func (s *stubProjectStore) GetByID(ctx context.Context, id string) (sqlstore.Project, error) {
	if s.err != nil {
		return sqlstore.Project{}, s.err
	}
	project, ok := s.byID[id]
	if !ok {
		return sqlstore.Project{}, sql.ErrNoRows
	}
	return project, nil
}

type stubDomainStore struct {
	byProject map[string][]sqlstore.Domain
	byID      map[string]sqlstore.Domain
	err       error
}

func (s *stubDomainStore) ListByProject(ctx context.Context, projectID string) ([]sqlstore.Domain, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.byProject[projectID], nil
}

func (s *stubDomainStore) Get(ctx context.Context, id string) (sqlstore.Domain, error) {
	if s.err != nil {
		return sqlstore.Domain{}, s.err
	}
	domain, ok := s.byID[id]
	if !ok {
		return sqlstore.Domain{}, sql.ErrNoRows
	}
	return domain, nil
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
	checks         map[string]sqlstore.IndexCheck
	updateCalls    []updateCall
	incremented    []string
	nextRetries    []nextRetryCall
	markErr        map[string]error
	pendingErr     error
	staleErr       error
	getByDomainErr error
	createErr      error
}

func newStubIndexCheckStore() *stubIndexCheckStore {
	return &stubIndexCheckStore{
		checks:      make(map[string]sqlstore.IndexCheck),
		markErr:     make(map[string]error),
		updateCalls: make([]updateCall, 0),
	}
}

func (s *stubIndexCheckStore) Create(ctx context.Context, check sqlstore.IndexCheck) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.checks[check.ID] = check
	return nil
}

func (s *stubIndexCheckStore) Get(ctx context.Context, checkID string) (*sqlstore.IndexCheck, error) {
	check, ok := s.checks[checkID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	out := check
	return &out, nil
}

func (s *stubIndexCheckStore) GetByDomainAndDate(ctx context.Context, domainID string, date time.Time) (*sqlstore.IndexCheck, error) {
	if s.getByDomainErr != nil {
		return nil, s.getByDomainErr
	}
	for _, check := range s.checks {
		if check.DomainID == domainID && check.CheckDate.Equal(date) {
			out := check
			return &out, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *stubIndexCheckStore) ListPendingRetries(ctx context.Context) ([]sqlstore.IndexCheck, error) {
	if s.pendingErr != nil {
		return nil, s.pendingErr
	}
	out := make([]sqlstore.IndexCheck, 0)
	for _, check := range s.checks {
		if check.Status == "checking" && check.NextRetryAt.Valid {
			out = append(out, check)
		}
	}
	return out, nil
}

func (s *stubIndexCheckStore) ListStaleChecking(ctx context.Context, olderThan time.Time) ([]sqlstore.IndexCheck, error) {
	if s.staleErr != nil {
		return nil, s.staleErr
	}
	out := make([]sqlstore.IndexCheck, 0)
	for _, check := range s.checks {
		if check.Status != "checking" || check.NextRetryAt.Valid {
			continue
		}
		ref := check.CreatedAt
		if check.LastAttemptAt.Valid {
			ref = check.LastAttemptAt.Time
		}
		if ref.After(olderThan) {
			continue
		}
		out = append(out, check)
	}
	return out, nil
}

func (s *stubIndexCheckStore) TryMarkChecking(ctx context.Context, checkID string) (bool, sqlstore.IndexCheck, error) {
	if err, ok := s.markErr[checkID]; ok && err != nil {
		return false, sqlstore.IndexCheck{}, err
	}
	check, ok := s.checks[checkID]
	if !ok {
		return false, sqlstore.IndexCheck{}, sql.ErrNoRows
	}
	switch check.Status {
	case "pending":
		// allowed
	case "checking":
		if !check.NextRetryAt.Valid || check.NextRetryAt.Time.After(time.Now().UTC()) {
			return false, check, nil
		}
	default:
		return false, check, nil
	}
	check.Status = "checking"
	check.NextRetryAt = sql.NullTime{}
	check.LastAttemptAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	s.checks[checkID] = check
	return true, check, nil
}

func (s *stubIndexCheckStore) UpdateStatus(ctx context.Context, checkID string, status string, isIndexed *bool, errMsg *string) error {
	check, ok := s.checks[checkID]
	if !ok {
		return sql.ErrNoRows
	}
	check.Status = status
	if isIndexed != nil {
		check.IsIndexed = sql.NullBool{Bool: *isIndexed, Valid: true}
	} else {
		check.IsIndexed = sql.NullBool{}
	}
	if errMsg != nil {
		check.ErrorMessage = sql.NullString{String: *errMsg, Valid: true}
	} else {
		check.ErrorMessage = sql.NullString{}
	}
	if status == "success" || status == "failed_investigation" {
		check.CompletedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
		check.NextRetryAt = sql.NullTime{}
	} else {
		check.CompletedAt = sql.NullTime{}
	}
	s.checks[checkID] = check
	s.updateCalls = append(s.updateCalls, updateCall{
		checkID:   checkID,
		status:    status,
		isIndexed: isIndexed,
		errMsg:    errMsg,
	})
	return nil
}

func (s *stubIndexCheckStore) IncrementAttempts(ctx context.Context, checkID string) error {
	check, ok := s.checks[checkID]
	if !ok {
		return sql.ErrNoRows
	}
	check.Attempts++
	check.LastAttemptAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	s.checks[checkID] = check
	s.incremented = append(s.incremented, checkID)
	return nil
}

func (s *stubIndexCheckStore) SetNextRetry(ctx context.Context, checkID string, nextRetry time.Time) error {
	check, ok := s.checks[checkID]
	if !ok {
		return sql.ErrNoRows
	}
	check.NextRetryAt = sql.NullTime{Time: nextRetry, Valid: true}
	s.checks[checkID] = check
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
	indexedByDomain map[string]bool
	errByDomain     map[string]error
	defaultIndexed  bool
}

func (s *stubChecker) Check(ctx context.Context, domain, geo string) (bool, error) {
	if err, ok := s.errByDomain[domain]; ok {
		return false, err
	}
	if val, ok := s.indexedByDomain[domain]; ok {
		return val, nil
	}
	return s.defaultIndexed, nil
}

func TestRunIndexCheckerTickSuccess(t *testing.T) {
	now := time.Now().UTC()
	project := sqlstore.Project{ID: "proj-1", TargetCountry: "SE"}
	projectStore := &stubProjectStore{
		projects: []sqlstore.Project{project},
		byID:     map[string]sqlstore.Project{project.ID: project},
	}
	domain := sqlstore.Domain{ID: "dom-1", ProjectID: "proj-1", URL: "example.com", Status: "published"}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{"proj-1": {domain}},
		byID:      map[string]sqlstore.Domain{domain.ID: domain},
	}
	checkStore := newStubIndexCheckStore()
	checkStore.getByDomainErr = sql.ErrNoRows
	historyStore := &stubHistoryStore{}
	checker := &stubChecker{defaultIndexed: true}

	if err := RunIndexCheckerTick(
		context.Background(),
		now,
		20*time.Minute,
		projectStore,
		domainStore,
		checkStore,
		historyStore,
		checker,
		zap.NewNop().Sugar(),
	); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	if len(historyStore.created) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyStore.created))
	}
	entry := historyStore.created[0]
	if !entry.Result.Valid || entry.Result.String != "success" {
		t.Fatalf("unexpected history result: %#v", entry.Result)
	}
	var payload map[string]any
	if err := json.Unmarshal(entry.ResponseData, &payload); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if indexed, ok := payload["indexed"].(bool); !ok || !indexed {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if len(checkStore.updateCalls) == 0 || checkStore.updateCalls[len(checkStore.updateCalls)-1].status != "success" {
		t.Fatalf("expected success status update, got %#v", checkStore.updateCalls)
	}
}

func TestRunIndexCheckerTickContinuesOnPerCheckError(t *testing.T) {
	now := time.Now().UTC()
	project := sqlstore.Project{ID: "proj-1", TargetCountry: "SE"}
	projectStore := &stubProjectStore{
		projects: []sqlstore.Project{project},
		byID:     map[string]sqlstore.Project{project.ID: project},
	}
	domainA := sqlstore.Domain{ID: "dom-a", ProjectID: "proj-1", URL: "a.example", Status: "published"}
	domainB := sqlstore.Domain{ID: "dom-b", ProjectID: "proj-1", URL: "b.example", Status: "published"}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{"proj-1": {domainA, domainB}},
		byID: map[string]sqlstore.Domain{
			domainA.ID: domainA,
			domainB.ID: domainB,
		},
	}
	checkStore := newStubIndexCheckStore()
	checkStore.checks["check-a"] = sqlstore.IndexCheck{
		ID:        "check-a",
		DomainID:  "dom-a",
		CheckDate: dateOnlyUTC(now),
		Status:    "pending",
		CreatedAt: now.Add(-time.Hour),
	}
	checkStore.checks["check-b"] = sqlstore.IndexCheck{
		ID:        "check-b",
		DomainID:  "dom-b",
		CheckDate: dateOnlyUTC(now),
		Status:    "pending",
		CreatedAt: now.Add(-time.Hour),
	}
	checkStore.markErr["check-a"] = errors.New("db mark failed")
	historyStore := &stubHistoryStore{}
	checker := &stubChecker{defaultIndexed: true}

	if err := RunIndexCheckerTick(
		context.Background(),
		now,
		20*time.Minute,
		projectStore,
		domainStore,
		checkStore,
		historyStore,
		checker,
		zap.NewNop().Sugar(),
	); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	if len(historyStore.created) != 1 {
		t.Fatalf("expected only one processed history entry, got %d", len(historyStore.created))
	}
	if checkStore.checks["check-b"].Status != "success" {
		t.Fatalf("expected check-b success, got %s", checkStore.checks["check-b"].Status)
	}
	if checkStore.checks["check-a"].Status != "pending" {
		t.Fatalf("expected check-a to remain pending, got %s", checkStore.checks["check-a"].Status)
	}
}

func TestRunIndexCheckerTickFailsStaleChecking(t *testing.T) {
	now := time.Now().UTC()
	project := sqlstore.Project{ID: "proj-1", TargetCountry: "SE"}
	projectStore := &stubProjectStore{
		projects: []sqlstore.Project{project},
		byID:     map[string]sqlstore.Project{project.ID: project},
	}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{"proj-1": {}},
		byID:      map[string]sqlstore.Domain{},
	}
	checkStore := newStubIndexCheckStore()
	checkStore.checks["stale-1"] = sqlstore.IndexCheck{
		ID:            "stale-1",
		DomainID:      "dom-stale",
		CheckDate:     dateOnlyUTC(now),
		Status:        "checking",
		LastAttemptAt: sql.NullTime{Time: now.Add(-45 * time.Minute), Valid: true},
		CreatedAt:     now.Add(-2 * time.Hour),
	}
	historyStore := &stubHistoryStore{}
	checker := &stubChecker{defaultIndexed: true}

	if err := RunIndexCheckerTick(
		context.Background(),
		now,
		20*time.Minute,
		projectStore,
		domainStore,
		checkStore,
		historyStore,
		checker,
		zap.NewNop().Sugar(),
	); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	if checkStore.checks["stale-1"].Status != "failed_investigation" {
		t.Fatalf("expected stale check to be failed_investigation, got %s", checkStore.checks["stale-1"].Status)
	}
	if len(historyStore.created) != 1 {
		t.Fatalf("expected stale history entry, got %d", len(historyStore.created))
	}
	if !historyStore.created[0].Result.Valid || historyStore.created[0].Result.String != "failed_investigation" {
		t.Fatalf("unexpected stale history result: %#v", historyStore.created[0].Result)
	}
}

func TestProcessCheckByIDNoDuplicateStart(t *testing.T) {
	now := time.Now().UTC()
	project := sqlstore.Project{ID: "proj-1", TargetCountry: "SE"}
	projectStore := &stubProjectStore{
		projects: []sqlstore.Project{project},
		byID:     map[string]sqlstore.Project{project.ID: project},
	}
	domain := sqlstore.Domain{ID: "dom-1", ProjectID: "proj-1", URL: "example.com", Status: "published"}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{"proj-1": {domain}},
		byID:      map[string]sqlstore.Domain{domain.ID: domain},
	}
	checkStore := newStubIndexCheckStore()
	checkStore.checks["check-1"] = sqlstore.IndexCheck{
		ID:        "check-1",
		DomainID:  domain.ID,
		CheckDate: dateOnlyUTC(now),
		Status:    "pending",
		CreatedAt: now.Add(-time.Hour),
	}
	historyStore := &stubHistoryStore{}
	checker := &stubChecker{defaultIndexed: true}

	first, err := ProcessCheckByID(
		context.Background(),
		now,
		"check-1",
		projectStore,
		domainStore,
		checkStore,
		historyStore,
		checker,
		zap.NewNop().Sugar(),
	)
	if err != nil {
		t.Fatalf("first process failed: %v", err)
	}
	if !first.Started || first.Status != "success" {
		t.Fatalf("expected first run success start, got %#v", first)
	}

	second, err := ProcessCheckByID(
		context.Background(),
		now,
		"check-1",
		projectStore,
		domainStore,
		checkStore,
		historyStore,
		checker,
		zap.NewNop().Sugar(),
	)
	if err != nil {
		t.Fatalf("second process failed: %v", err)
	}
	if second.Started {
		t.Fatalf("expected second run skipped, got %#v", second)
	}
}

func TestProcessCheckByIDUnpublishedDomainTerminatesCheck(t *testing.T) {
	now := time.Now().UTC()
	projectStore := &stubProjectStore{
		projects: []sqlstore.Project{},
		byID:     map[string]sqlstore.Project{},
	}
	domain := sqlstore.Domain{
		ID:        "dom-1",
		ProjectID: "proj-1",
		URL:       "example.com",
		Status:    "waiting",
	}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{},
		byID:      map[string]sqlstore.Domain{domain.ID: domain},
	}
	checkStore := newStubIndexCheckStore()
	checkStore.checks["check-1"] = sqlstore.IndexCheck{
		ID:        "check-1",
		DomainID:  domain.ID,
		CheckDate: dateOnlyUTC(now),
		Status:    "pending",
		CreatedAt: now.Add(-time.Hour),
	}
	historyStore := &stubHistoryStore{}
	checker := &stubChecker{defaultIndexed: true}

	result, err := ProcessCheckByID(
		context.Background(),
		now,
		"check-1",
		projectStore,
		domainStore,
		checkStore,
		historyStore,
		checker,
		zap.NewNop().Sugar(),
	)
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if !result.Started || result.Status != "failed_investigation" {
		t.Fatalf("expected terminal failed_investigation, got %#v", result)
	}
	check := checkStore.checks["check-1"]
	if check.Status != "failed_investigation" {
		t.Fatalf("expected failed_investigation status, got %s", check.Status)
	}
	if check.Attempts != 1 {
		t.Fatalf("expected attempts=1, got %d", check.Attempts)
	}
	if len(historyStore.created) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(historyStore.created))
	}
	entry := historyStore.created[0]
	if !entry.Result.Valid || entry.Result.String != "failed_investigation" {
		t.Fatalf("unexpected history result: %#v", entry.Result)
	}
	if !entry.ErrorMessage.Valid || entry.ErrorMessage.String != "domain is not published" {
		t.Fatalf("unexpected history error message: %#v", entry.ErrorMessage)
	}
}

func TestProcessCheckByIDEmptyDomainURLTerminatesCheck(t *testing.T) {
	now := time.Now().UTC()
	projectStore := &stubProjectStore{
		projects: []sqlstore.Project{},
		byID:     map[string]sqlstore.Project{},
	}
	domain := sqlstore.Domain{
		ID:        "dom-1",
		ProjectID: "proj-1",
		URL:       "   ",
		Status:    "published",
	}
	domainStore := &stubDomainStore{
		byProject: map[string][]sqlstore.Domain{},
		byID:      map[string]sqlstore.Domain{domain.ID: domain},
	}
	checkStore := newStubIndexCheckStore()
	checkStore.checks["check-1"] = sqlstore.IndexCheck{
		ID:        "check-1",
		DomainID:  domain.ID,
		CheckDate: dateOnlyUTC(now),
		Status:    "pending",
		CreatedAt: now.Add(-time.Hour),
	}
	historyStore := &stubHistoryStore{}
	checker := &stubChecker{defaultIndexed: true}

	result, err := ProcessCheckByID(
		context.Background(),
		now,
		"check-1",
		projectStore,
		domainStore,
		checkStore,
		historyStore,
		checker,
		zap.NewNop().Sugar(),
	)
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if !result.Started || result.Status != "failed_investigation" {
		t.Fatalf("expected terminal failed_investigation, got %#v", result)
	}
	check := checkStore.checks["check-1"]
	if check.Status != "failed_investigation" {
		t.Fatalf("expected failed_investigation status, got %s", check.Status)
	}
	if check.Attempts != 1 {
		t.Fatalf("expected attempts=1, got %d", check.Attempts)
	}
	if len(historyStore.created) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(historyStore.created))
	}
	entry := historyStore.created[0]
	if !entry.Result.Valid || entry.Result.String != "failed_investigation" {
		t.Fatalf("unexpected history result: %#v", entry.Result)
	}
	if !entry.ErrorMessage.Valid || entry.ErrorMessage.String != "domain url is empty" {
		t.Fatalf("unexpected history error message: %#v", entry.ErrorMessage)
	}
}
