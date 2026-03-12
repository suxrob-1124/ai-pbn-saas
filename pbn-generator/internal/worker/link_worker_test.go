package worker

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

type stubLinkTaskStore struct {
	tasks map[string]*sqlstore.LinkTask
}

func newStubLinkTaskStore(task sqlstore.LinkTask) *stubLinkTaskStore {
	return &stubLinkTaskStore{tasks: map[string]*sqlstore.LinkTask{task.ID: &task}}
}

func (s *stubLinkTaskStore) Get(ctx context.Context, taskID string) (*sqlstore.LinkTask, error) {
	task, ok := s.tasks[taskID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	out := *task
	return &out, nil
}

func (s *stubLinkTaskStore) Update(ctx context.Context, taskID string, updates sqlstore.LinkTaskUpdates) error {
	task, ok := s.tasks[taskID]
	if !ok {
		return sql.ErrNoRows
	}
	if updates.AnchorText != nil {
		task.AnchorText = *updates.AnchorText
	}
	if updates.TargetURL != nil {
		task.TargetURL = *updates.TargetURL
	}
	if updates.Action != nil {
		task.Action = *updates.Action
	}
	if updates.Status != nil {
		task.Status = *updates.Status
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
	if updates.Attempts != nil {
		task.Attempts = *updates.Attempts
	}
	if updates.ScheduledFor != nil {
		task.ScheduledFor = *updates.ScheduledFor
	}
	if updates.CreatedAt != nil {
		task.CreatedAt = *updates.CreatedAt
	}
	if updates.CompletedAt != nil {
		task.CompletedAt = *updates.CompletedAt
	}
	if updates.LogLines != nil {
		task.LogLines = *updates.LogLines
	}
	return nil
}

type stubDomainStore struct {
	domains map[string]sqlstore.Domain
}

func (s *stubDomainStore) Get(ctx context.Context, id string) (sqlstore.Domain, error) {
	d, ok := s.domains[id]
	if !ok {
		return sqlstore.Domain{}, sql.ErrNoRows
	}
	return d, nil
}

func (s *stubDomainStore) UpdateLinkStatus(ctx context.Context, id string, status string) error {
	d, ok := s.domains[id]
	if !ok {
		return sql.ErrNoRows
	}
	d.LinkStatus = sqlstore.NullableString(status)
	d.LinkUpdatedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	s.domains[id] = d
	return nil
}

func (s *stubDomainStore) UpdateLinkState(ctx context.Context, id string, status string, lastTaskID string, filePath string, anchorSnapshot string) error {
	d, ok := s.domains[id]
	if !ok {
		return sql.ErrNoRows
	}
	d.LinkStatus = sqlstore.NullableString(status)
	d.LinkLastTaskID = sqlstore.NullableString(lastTaskID)
	if strings.TrimSpace(filePath) == "" {
		d.LinkFilePath = sql.NullString{}
	} else {
		d.LinkFilePath = sqlstore.NullableString(filePath)
	}
	if strings.TrimSpace(anchorSnapshot) == "" {
		d.LinkAnchorSnapshot = sql.NullString{}
	} else {
		d.LinkAnchorSnapshot = sqlstore.NullableString(anchorSnapshot)
	}
	d.LinkUpdatedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	s.domains[id] = d
	return nil
}

type stubProjectStore struct {
	projects map[string]sqlstore.Project
}

func (s *stubProjectStore) GetByID(ctx context.Context, id string) (sqlstore.Project, error) {
	p, ok := s.projects[id]
	if !ok {
		return sqlstore.Project{}, sql.ErrNoRows
	}
	return p, nil
}

type stubUserStore struct{}

func (s *stubUserStore) GetAPIKey(ctx context.Context, email string) ([]byte, *time.Time, error) {
	return nil, nil, sql.ErrNoRows
}

type stubSiteFileStore struct {
	files map[string]sqlstore.SiteFile
}

func newStubSiteFileStore() *stubSiteFileStore {
	return &stubSiteFileStore{files: make(map[string]sqlstore.SiteFile)}
}

func (s *stubSiteFileStore) GetByPath(ctx context.Context, domainID, path string) (*sqlstore.SiteFile, error) {
	key := domainID + ":" + path
	file, ok := s.files[key]
	if !ok {
		return nil, sql.ErrNoRows
	}
	out := file
	return &out, nil
}

func (s *stubSiteFileStore) Create(ctx context.Context, file sqlstore.SiteFile) error {
	key := file.DomainID + ":" + file.Path
	s.files[key] = file
	return nil
}

func (s *stubSiteFileStore) Update(ctx context.Context, fileID string, content []byte) error {
	for key, file := range s.files {
		if file.ID == fileID {
			file.SizeBytes = int64(len(content))
			s.files[key] = file
			return nil
		}
	}
	return sql.ErrNoRows
}

type stubFileEditStore struct {
	edits []sqlstore.FileEdit
}

func (s *stubFileEditStore) Create(ctx context.Context, edit sqlstore.FileEdit) error {
	s.edits = append(s.edits, edit)
	return nil
}

type stubGenerator struct {
	content string
	err     error
}

func (g *stubGenerator) Generate(ctx context.Context, anchorText, targetURL, pageContext string) (string, error) {
	if g.err != nil {
		return "", g.err
	}
	return g.content, nil
}

func TestLinkWorkerInsert(t *testing.T) {
	baseDir := t.TempDir()
	domainDir := filepath.Join(baseDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	htmlPath := filepath.Join(domainDir, "index.html")
	if err := os.WriteFile(htmlPath, []byte("<body>Hello anchor here</body>"), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}

	task := sqlstore.LinkTask{
		ID:         "task-1",
		DomainID:   "domain-1",
		AnchorText: "anchor",
		TargetURL:  "https://example.com",
		Action:     "insert",
		CreatedBy:  "user@example.com",
	}
	taskStore := newStubLinkTaskStore(task)

	w := &LinkWorker{
		BaseDir:   baseDir,
		Config:    config.Config{},
		Tasks:     taskStore,
		Domains:   &stubDomainStore{domains: map[string]sqlstore.Domain{"domain-1": {ID: "domain-1", ProjectID: "project-1", URL: "example.com"}}},
		Projects:  &stubProjectStore{projects: map[string]sqlstore.Project{"project-1": {ID: "project-1", UserEmail: "owner@example.com"}}},
		Users:     &stubUserStore{},
		SiteFiles: newStubSiteFileStore(),
		FileEdits: &stubFileEditStore{},
		Now:       func() time.Time { return time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC) },
	}

	if err := w.ProcessTask(context.Background(), "task-1"); err != nil {
		t.Fatalf("process task: %v", err)
	}

	updated := taskStore.tasks["task-1"]
	if updated.Status != "inserted" {
		t.Fatalf("expected inserted, got %s", updated.Status)
	}
	if !updated.FoundLocation.Valid {
		t.Fatalf("expected found location")
	}
	out, _ := os.ReadFile(htmlPath)
	if !strings.Contains(string(out), "<a href=\"https://example.com\">anchor</a>") {
		t.Fatalf("expected link inserted")
	}
}

func TestLinkWorkerExistingLinkInBody(t *testing.T) {
	baseDir := t.TempDir()
	domainDir := filepath.Join(baseDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	htmlPath := filepath.Join(domainDir, "index.html")
	original := "<body><p>Already <a href=\"https://example.com\">anchor</a> here</p></body>"
	if err := os.WriteFile(htmlPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}

	task := sqlstore.LinkTask{
		ID:         "task-1b",
		DomainID:   "domain-1b",
		AnchorText: "anchor",
		TargetURL:  "https://example.com",
		Action:     "insert",
		CreatedBy:  "user@example.com",
	}
	taskStore := newStubLinkTaskStore(task)

	w := &LinkWorker{
		BaseDir:   baseDir,
		Config:    config.Config{},
		Tasks:     taskStore,
		Domains:   &stubDomainStore{domains: map[string]sqlstore.Domain{"domain-1b": {ID: "domain-1b", ProjectID: "project-1b", URL: "example.com"}}},
		Projects:  &stubProjectStore{projects: map[string]sqlstore.Project{"project-1b": {ID: "project-1b", UserEmail: "owner@example.com"}}},
		Users:     &stubUserStore{},
		SiteFiles: newStubSiteFileStore(),
		FileEdits: &stubFileEditStore{},
		Now:       func() time.Time { return time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC) },
	}

	if err := w.ProcessTask(context.Background(), "task-1b"); err != nil {
		t.Fatalf("process task: %v", err)
	}

	updated := taskStore.tasks["task-1b"]
	if updated.Status != "inserted" {
		t.Fatalf("expected inserted, got %s", updated.Status)
	}
	if !updated.FoundLocation.Valid {
		t.Fatalf("expected found location")
	}
	out, _ := os.ReadFile(htmlPath)
	if string(out) != original {
		t.Fatalf("expected html unchanged when link already exists")
	}
}

func TestLinkWorkerGenerate(t *testing.T) {
	baseDir := t.TempDir()
	domainDir := filepath.Join(baseDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	htmlPath := filepath.Join(domainDir, "index.html")
	if err := os.WriteFile(htmlPath, []byte("<body>No links here</body>"), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}

	task := sqlstore.LinkTask{
		ID:         "task-2",
		DomainID:   "domain-2",
		AnchorText: "anchor",
		TargetURL:  "https://example.com",
		Action:     "insert",
		CreatedBy:  "user@example.com",
	}
	taskStore := newStubLinkTaskStore(task)

	w := &LinkWorker{
		BaseDir:   baseDir,
		Config:    config.Config{},
		Tasks:     taskStore,
		Domains:   &stubDomainStore{domains: map[string]sqlstore.Domain{"domain-2": {ID: "domain-2", ProjectID: "project-1", URL: "example.com"}}},
		Projects:  &stubProjectStore{projects: map[string]sqlstore.Project{"project-1": {ID: "project-1", UserEmail: "owner@example.com"}}},
		Users:     &stubUserStore{},
		SiteFiles: newStubSiteFileStore(),
		FileEdits: &stubFileEditStore{},
		Generator: &stubGenerator{content: "<p>Generated <a href=\"https://example.com\">anchor</a></p>"},
		Now:       func() time.Time { return time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC) },
	}

	if err := w.ProcessTask(context.Background(), "task-2"); err != nil {
		t.Fatalf("process task: %v", err)
	}

	updated := taskStore.tasks["task-2"]
	if updated.Status != "generated" {
		t.Fatalf("expected generated, got %s", updated.Status)
	}
	if !updated.GeneratedContent.Valid {
		t.Fatalf("expected generated content")
	}
	out, _ := os.ReadFile(htmlPath)
	if !strings.Contains(string(out), "Generated") {
		t.Fatalf("expected generated content in html")
	}
}

func TestLinkWorkerFailsWhenNoHTML(t *testing.T) {
	baseDir := t.TempDir()
	domainDir := filepath.Join(baseDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	now := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	task := sqlstore.LinkTask{
		ID:         "task-3",
		DomainID:   "domain-3",
		AnchorText: "anchor",
		TargetURL:  "https://example.com",
		Action:     "insert",
		CreatedBy:  "user@example.com",
		CreatedAt:  now,
	}
	taskStore := newStubLinkTaskStore(task)

	w := &LinkWorker{
		BaseDir:   baseDir,
		Config:    config.Config{},
		Tasks:     taskStore,
		Domains:   &stubDomainStore{domains: map[string]sqlstore.Domain{"domain-3": {ID: "domain-3", ProjectID: "project-1", URL: "example.com"}}},
		Projects:  &stubProjectStore{projects: map[string]sqlstore.Project{"project-1": {ID: "project-1", UserEmail: "owner@example.com"}}},
		Users:     &stubUserStore{},
		SiteFiles: newStubSiteFileStore(),
		FileEdits: &stubFileEditStore{},
		Now:       func() time.Time { return now },
	}

	if err := w.ProcessTask(context.Background(), "task-3"); err != nil {
		t.Fatalf("expected nil error for terminal worker failure, got %v", err)
	}

	updated := taskStore.tasks["task-3"]
	if updated.Status != "failed" {
		t.Fatalf("expected failed, got %s", updated.Status)
	}
	if updated.ErrorMessage.Valid == false {
		t.Fatalf("expected error message")
	}
}

func TestLinkWorkerRetryWindowExceeded(t *testing.T) {
	baseDir := t.TempDir()
	domainDir := filepath.Join(baseDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	now := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	task := sqlstore.LinkTask{
		ID:         "task-old",
		DomainID:   "domain-old",
		AnchorText: "anchor",
		TargetURL:  "https://example.com",
		Action:     "insert",
		CreatedBy:  "user@example.com",
		CreatedAt:  now.Add(-48 * time.Hour),
	}
	taskStore := newStubLinkTaskStore(task)

	w := &LinkWorker{
		BaseDir:   baseDir,
		Config:    config.Config{},
		Tasks:     taskStore,
		Domains:   &stubDomainStore{domains: map[string]sqlstore.Domain{"domain-old": {ID: "domain-old", ProjectID: "project-1", URL: "example.com"}}},
		Projects:  &stubProjectStore{projects: map[string]sqlstore.Project{"project-1": {ID: "project-1", UserEmail: "owner@example.com"}}},
		Users:     &stubUserStore{},
		SiteFiles: newStubSiteFileStore(),
		FileEdits: &stubFileEditStore{},
		Now:       func() time.Time { return now },
	}

	if err := w.ProcessTask(context.Background(), "task-old"); err != nil {
		t.Fatalf("expected nil error for terminal worker failure, got %v", err)
	}

	updated := taskStore.tasks["task-old"]
	if updated.Status != "failed" {
		t.Fatalf("expected failed, got %s", updated.Status)
	}
}

func TestLinkWorkerReplaceFromFoundLocation(t *testing.T) {
	baseDir := t.TempDir()
	domainDir := filepath.Join(baseDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	htmlPath := filepath.Join(domainDir, "index.html")
	if err := os.WriteFile(htmlPath, []byte(`<body><p><a href="https://old.example">old anchor</a></p></body>`), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}

	prevTask := sqlstore.LinkTask{
		ID:            "task-old",
		DomainID:      "domain-4",
		AnchorText:    "old anchor",
		TargetURL:     "https://old.example",
		Action:        "insert",
		FoundLocation: sql.NullString{String: "index.html:1", Valid: true},
		CreatedBy:     "user@example.com",
	}
	newTask := sqlstore.LinkTask{
		ID:         "task-new",
		DomainID:   "domain-4",
		AnchorText: "new anchor",
		TargetURL:  "https://new.example",
		Action:     "insert",
		CreatedBy:  "user@example.com",
	}
	taskStore := &stubLinkTaskStore{tasks: map[string]*sqlstore.LinkTask{
		prevTask.ID: &prevTask,
		newTask.ID:  &newTask,
	}}

	domainStore := &stubDomainStore{domains: map[string]sqlstore.Domain{
		"domain-4": {
			ID:             "domain-4",
			ProjectID:      "project-1",
			URL:            "example.com",
			LinkLastTaskID: sqlstore.NullableString(prevTask.ID),
		},
	}}

	w := &LinkWorker{
		BaseDir:   baseDir,
		Config:    config.Config{},
		Tasks:     taskStore,
		Domains:   domainStore,
		Projects:  &stubProjectStore{projects: map[string]sqlstore.Project{"project-1": {ID: "project-1", UserEmail: "owner@example.com"}}},
		Users:     &stubUserStore{},
		SiteFiles: newStubSiteFileStore(),
		FileEdits: &stubFileEditStore{},
		Now:       func() time.Time { return time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC) },
	}

	if err := w.ProcessTask(context.Background(), "task-new"); err != nil {
		t.Fatalf("process task: %v", err)
	}

	out, _ := os.ReadFile(htmlPath)
	if strings.Contains(string(out), "old anchor") {
		t.Fatalf("old anchor should be replaced")
	}
	if !strings.Contains(string(out), `<a href="https://new.example">new anchor</a>`) {
		t.Fatalf("expected new link")
	}
	updated := taskStore.tasks["task-new"]
	if updated.Status != "inserted" {
		t.Fatalf("expected inserted, got %s", updated.Status)
	}
	domain := domainStore.domains["domain-4"]
	if !domain.LinkLastTaskID.Valid || domain.LinkLastTaskID.String != "task-new" {
		t.Fatalf("expected domain link_last_task_id updated")
	}
}

func TestLinkWorkerRelinkFallsBackToFreshInsert(t *testing.T) {
	baseDir := t.TempDir()
	domainDir := filepath.Join(baseDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	htmlPath := filepath.Join(domainDir, "index.html")
	if err := os.WriteFile(htmlPath, []byte("<body>text with new anchor here</body>"), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}

	prevTask := sqlstore.LinkTask{
		ID:         "task-old-2",
		DomainID:   "domain-5",
		AnchorText: "old anchor",
		TargetURL:  "https://old.example",
		Action:     "insert",
		CreatedBy:  "user@example.com",
	}
	newTask := sqlstore.LinkTask{
		ID:         "task-new-2",
		DomainID:   "domain-5",
		AnchorText: "new anchor",
		TargetURL:  "https://new.example",
		Action:     "insert",
		CreatedBy:  "user@example.com",
	}
	taskStore := &stubLinkTaskStore{tasks: map[string]*sqlstore.LinkTask{
		prevTask.ID: &prevTask,
		newTask.ID:  &newTask,
	}}

	domainStore := &stubDomainStore{domains: map[string]sqlstore.Domain{
		"domain-5": {
			ID:             "domain-5",
			ProjectID:      "project-1",
			URL:            "example.com",
			LinkLastTaskID: sqlstore.NullableString(prevTask.ID),
		},
	}}

	w := &LinkWorker{
		BaseDir:   baseDir,
		Config:    config.Config{},
		Tasks:     taskStore,
		Domains:   domainStore,
		Projects:  &stubProjectStore{projects: map[string]sqlstore.Project{"project-1": {ID: "project-1", UserEmail: "owner@example.com"}}},
		Users:     &stubUserStore{},
		SiteFiles: newStubSiteFileStore(),
		FileEdits: &stubFileEditStore{},
		Now:       func() time.Time { return time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC) },
	}

	if err := w.ProcessTask(context.Background(), "task-new-2"); err != nil {
		t.Fatalf("process task should not return error for business failure: %v", err)
	}

	out, _ := os.ReadFile(htmlPath)
	if !strings.Contains(string(out), `<a href="https://new.example">new anchor</a>`) {
		t.Fatalf("expected fallback fresh insertion when old link source not found, got: %s", string(out))
	}
	updated := taskStore.tasks["task-new-2"]
	if updated.Status != "inserted" && updated.Status != "completed" {
		t.Fatalf("expected inserted or completed, got %s", updated.Status)
	}
}

func TestLinkWorkerRemove(t *testing.T) {
	baseDir := t.TempDir()
	domainDir := filepath.Join(baseDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	htmlPath := filepath.Join(domainDir, "index.html")
	content := `<html><head><title>Title <a href="https://example.com">anchor</a></title></head><body><h1>Header</h1><p>Text <a href="https://example.com">anchor</a></p></body></html>`
	if err := os.WriteFile(htmlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}

	task := sqlstore.LinkTask{
		ID:         "task-remove",
		DomainID:   "domain-remove",
		AnchorText: "anchor",
		TargetURL:  "https://example.com",
		Action:     "remove",
		CreatedBy:  "user@example.com",
	}
	taskStore := newStubLinkTaskStore(task)

	w := &LinkWorker{
		BaseDir:   baseDir,
		Config:    config.Config{},
		Tasks:     taskStore,
		Domains:   &stubDomainStore{domains: map[string]sqlstore.Domain{"domain-remove": {ID: "domain-remove", ProjectID: "project-1", URL: "example.com"}}},
		Projects:  &stubProjectStore{projects: map[string]sqlstore.Project{"project-1": {ID: "project-1", UserEmail: "owner@example.com"}}},
		Users:     &stubUserStore{},
		SiteFiles: newStubSiteFileStore(),
		FileEdits: &stubFileEditStore{},
		Now:       func() time.Time { return time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC) },
	}

	if err := w.ProcessTask(context.Background(), "task-remove"); err != nil {
		t.Fatalf("process task: %v", err)
	}

	out, _ := os.ReadFile(htmlPath)
	if strings.Contains(string(out), "<a href=\"https://example.com\">anchor</a>") {
		t.Fatalf("expected link removed")
	}
	updated := taskStore.tasks["task-remove"]
	if updated.Status != "removed" {
		t.Fatalf("expected removed, got %s", updated.Status)
	}
	foundLog := false
	for _, line := range updated.LogLines {
		if strings.Contains(line, "удалена ссылка из title") {
			foundLog = true
			break
		}
	}
	if !foundLog {
		t.Fatalf("expected log about title removal")
	}
}

func TestLinkWorkerRemoveNotFoundIsIdempotent(t *testing.T) {
	baseDir := t.TempDir()
	domainDir := filepath.Join(baseDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	htmlPath := filepath.Join(domainDir, "index.html")
	content := `<html><body><h1>Header</h1><p>No matching link here</p></body></html>`
	if err := os.WriteFile(htmlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}

	task := sqlstore.LinkTask{
		ID:         "task-remove-not-found",
		DomainID:   "domain-remove-not-found",
		AnchorText: "anchor",
		TargetURL:  "https://example.com",
		Action:     "remove",
		CreatedBy:  "user@example.com",
	}
	taskStore := newStubLinkTaskStore(task)
	domainStore := &stubDomainStore{domains: map[string]sqlstore.Domain{
		"domain-remove-not-found": {ID: "domain-remove-not-found", ProjectID: "project-1", URL: "example.com"},
	}}

	w := &LinkWorker{
		BaseDir:   baseDir,
		Config:    config.Config{},
		Tasks:     taskStore,
		Domains:   domainStore,
		Projects:  &stubProjectStore{projects: map[string]sqlstore.Project{"project-1": {ID: "project-1", UserEmail: "owner@example.com"}}},
		Users:     &stubUserStore{},
		SiteFiles: newStubSiteFileStore(),
		FileEdits: &stubFileEditStore{},
		Now:       func() time.Time { return time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC) },
	}

	if err := w.ProcessTask(context.Background(), "task-remove-not-found"); err != nil {
		t.Fatalf("process task: %v", err)
	}

	updated := taskStore.tasks["task-remove-not-found"]
	if updated.Status != "removed" {
		t.Fatalf("expected removed, got %s", updated.Status)
	}
	if updated.ErrorMessage.Valid {
		t.Fatalf("expected empty error message, got %s", updated.ErrorMessage.String)
	}
	foundWarning := false
	for _, line := range updated.LogLines {
		if strings.Contains(strings.ToLower(line), "идемпотентно") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Fatalf("expected idempotent warning in logs")
	}
	domain := domainStore.domains["domain-remove-not-found"]
	if !domain.LinkStatus.Valid || domain.LinkStatus.String != "removed" {
		t.Fatalf("expected domain link status removed, got %#v", domain.LinkStatus)
	}
}
