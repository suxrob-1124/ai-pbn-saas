package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

func TestLinkWorkerSameRelinkDoesNotProduceFalseUpdate(t *testing.T) {
	baseDir := t.TempDir()
	domainDir := filepath.Join(baseDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	htmlPath := filepath.Join(domainDir, "index.html")
	original := `<body><p><a href="https://same.example">same anchor</a></p></body>`
	if err := os.WriteFile(htmlPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}

	prevTask := sqlstore.LinkTask{
		ID:            "task-prev-same",
		DomainID:      "domain-same",
		AnchorText:    "same anchor",
		TargetURL:     "https://same.example",
		Action:        "insert",
		FoundLocation: sqlstore.NullableString("index.html:1"),
		CreatedBy:     "user@example.com",
	}
	newTask := sqlstore.LinkTask{
		ID:         "task-new-same",
		DomainID:   "domain-same",
		AnchorText: "same anchor",
		TargetURL:  "https://same.example",
		Action:     "insert",
		CreatedBy:  "user@example.com",
	}
	taskStore := &stubLinkTaskStore{tasks: map[string]*sqlstore.LinkTask{
		prevTask.ID: &prevTask,
		newTask.ID:  &newTask,
	}}
	domainStore := &stubDomainStore{domains: map[string]sqlstore.Domain{
		"domain-same": {
			ID:             "domain-same",
			ProjectID:      "project-1",
			URL:            "example.com",
			LinkLastTaskID: sqlstore.NullableString(prevTask.ID),
		},
	}}
	fileEdits := &stubFileEditStore{}

	w := &LinkWorker{
		BaseDir:   baseDir,
		Config:    config.Config{},
		Tasks:     taskStore,
		Domains:   domainStore,
		Projects:  &stubProjectStore{projects: map[string]sqlstore.Project{"project-1": {ID: "project-1", UserEmail: "owner@example.com"}}},
		Users:     &stubUserStore{},
		SiteFiles: newStubSiteFileStore(),
		FileEdits: fileEdits,
		Now:       func() time.Time { return time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC) },
	}

	if err := w.ProcessTask(context.Background(), "task-new-same"); err != nil {
		t.Fatalf("process task: %v", err)
	}

	updated := taskStore.tasks["task-new-same"]
	if updated.Status != "inserted" {
		t.Fatalf("expected inserted, got %s", updated.Status)
	}
	if !updated.FoundLocation.Valid {
		t.Fatalf("expected found location")
	}
	if len(fileEdits.edits) != 0 {
		t.Fatalf("expected no file edits for idempotent relink, got %d", len(fileEdits.edits))
	}

	out, _ := os.ReadFile(htmlPath)
	if string(out) != original {
		t.Fatalf("expected html unchanged, got: %s", string(out))
	}

	for _, line := range updated.LogLines {
		if strings.Contains(line, "обновлена ссылка") {
			t.Fatalf("unexpected false update log: %s", line)
		}
	}
	foundExisting := false
	for _, line := range updated.LogLines {
		if strings.Contains(line, "ссылка уже существует") {
			foundExisting = true
			break
		}
	}
	if !foundExisting {
		t.Fatalf("expected existing-link log, got %#v", updated.LogLines)
	}
}
