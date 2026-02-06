package linkbuilder

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type stubTaskStore struct {
	tasks  map[string]Task
	last   TaskUpdates
	errGet error
	errUpd error
}

func newStubTaskStore(task Task) *stubTaskStore {
	return &stubTaskStore{tasks: map[string]Task{task.ID: task}}
}

func (s *stubTaskStore) Get(ctx context.Context, taskID string) (Task, error) {
	if s.errGet != nil {
		return Task{}, s.errGet
	}
	t, ok := s.tasks[taskID]
	if !ok {
		return Task{}, errors.New("not found")
	}
	return t, nil
}

func (s *stubTaskStore) Update(ctx context.Context, taskID string, updates TaskUpdates) error {
	if s.errUpd != nil {
		return s.errUpd
	}
	t, ok := s.tasks[taskID]
	if !ok {
		return errors.New("not found")
	}
	if updates.Status != nil {
		t.Status = *updates.Status
	}
	if updates.FoundLocation != nil {
		t.FoundLocation = *updates.FoundLocation
	}
	if updates.GeneratedContent != nil {
		t.GeneratedContent = *updates.GeneratedContent
	}
	if updates.ErrorMessage != nil {
		t.ErrorMessage = *updates.ErrorMessage
	}
	if updates.Attempts != nil {
		t.Attempts = *updates.Attempts
	}
	s.tasks[taskID] = t
	s.last = updates
	return nil
}

type stubHTMLStore struct {
	html    map[string]string
	saved   string
	errLoad error
	errSave error
}

func newStubHTMLStore(domainID, html string) *stubHTMLStore {
	return &stubHTMLStore{html: map[string]string{domainID: html}}
}

func (s *stubHTMLStore) Load(ctx context.Context, domainID string) (string, error) {
	if s.errLoad != nil {
		return "", s.errLoad
	}
	return s.html[domainID], nil
}

func (s *stubHTMLStore) Save(ctx context.Context, domainID string, htmlContent string) error {
	if s.errSave != nil {
		return s.errSave
	}
	s.saved = htmlContent
	s.html[domainID] = htmlContent
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

func TestFindAnchor(t *testing.T) {
	b := NewBuilder(nil, nil, nil)
	html := "<html><head><title>Anchor title</title></head><body>Hello Anchor text</body></html>"
	pos, found := b.FindAnchor(html, "anchor")
	if !found || pos == -1 {
		t.Fatalf("expected to find anchor")
	}

	_, found = b.FindAnchor(html, "missing")
	if found {
		t.Fatalf("expected missing anchor")
	}
}

func TestFindAnchorSkipsTitleAndHeadings(t *testing.T) {
	b := NewBuilder(nil, nil, nil)
	html := "<html><head><title>Anchor title</title></head><body><h1>Anchor heading</h1></body></html>"
	if _, found := b.FindAnchor(html, "anchor"); found {
		t.Fatalf("expected to skip anchor in title or heading")
	}
}

func TestInsertLink(t *testing.T) {
	b := NewBuilder(nil, nil, nil)
	html := "<body>Hello anchor text</body>"
	pos, _ := b.FindAnchor(html, "anchor")
	out := b.InsertLink(html, pos, "anchor", "https://example.com")
	if !strings.Contains(out, "<a href=\"https://example.com\">anchor</a>") {
		t.Fatalf("expected inserted link, got: %s", out)
	}
}

func TestProcessTaskFound(t *testing.T) {
	task := Task{
		ID:         "task-1",
		DomainID:   "domain-1",
		AnchorText: "anchor",
		TargetURL:  "https://example.com",
	}
	store := newStubTaskStore(task)
	htmlStore := newStubHTMLStore(task.DomainID, "<body>Hello anchor here</body>")
	builder := NewBuilder(store, htmlStore, &stubGenerator{})

	if err := builder.ProcessTask(context.Background(), task.ID); err != nil {
		t.Fatalf("process task failed: %v", err)
	}

	updated := store.tasks[task.ID]
	if updated.Status != "inserted" {
		t.Fatalf("expected inserted status, got %s", updated.Status)
	}
	if updated.FoundLocation == "" {
		t.Fatalf("expected found location")
	}
	if updated.Attempts != 1 {
		t.Fatalf("expected attempts=1, got %d", updated.Attempts)
	}
	if !strings.Contains(htmlStore.saved, "<a href=\"https://example.com\">anchor</a>") {
		t.Fatalf("expected link in saved html")
	}
}

func TestProcessTaskGenerate(t *testing.T) {
	task := Task{
		ID:         "task-2",
		DomainID:   "domain-2",
		AnchorText: "anchor",
		TargetURL:  "https://example.com",
	}
	store := newStubTaskStore(task)
	htmlStore := newStubHTMLStore(task.DomainID, "<body>No links here</body>")
	gen := &stubGenerator{content: "Generated paragraph"}
	builder := NewBuilder(store, htmlStore, gen)

	if err := builder.ProcessTask(context.Background(), task.ID); err != nil {
		t.Fatalf("process task failed: %v", err)
	}

	updated := store.tasks[task.ID]
	if updated.Status != "generated" {
		t.Fatalf("expected generated status, got %s", updated.Status)
	}
	if updated.GeneratedContent == "" {
		t.Fatalf("expected generated content")
	}
	if !strings.Contains(htmlStore.saved, "Generated paragraph") {
		t.Fatalf("expected generated content in html")
	}
}

func TestProcessTaskGenerateError(t *testing.T) {
	task := Task{
		ID:         "task-3",
		DomainID:   "domain-3",
		AnchorText: "anchor",
		TargetURL:  "https://example.com",
	}
	store := newStubTaskStore(task)
	htmlStore := newStubHTMLStore(task.DomainID, "<body>No links here</body>")
	gen := &stubGenerator{err: errors.New("boom")}
	builder := NewBuilder(store, htmlStore, gen)

	if err := builder.ProcessTask(context.Background(), task.ID); err == nil {
		t.Fatalf("expected error")
	}

	updated := store.tasks[task.ID]
	if updated.Status != "failed" {
		t.Fatalf("expected failed status, got %s", updated.Status)
	}
	if updated.ErrorMessage == "" {
		t.Fatalf("expected error message")
	}
}

func TestProcessTaskLoadError(t *testing.T) {
	task := Task{
		ID:         "task-4",
		DomainID:   "domain-4",
		AnchorText: "anchor",
		TargetURL:  "https://example.com",
	}
	store := newStubTaskStore(task)
	htmlStore := newStubHTMLStore(task.DomainID, "")
	htmlStore.errLoad = errors.New("load failed")
	builder := NewBuilder(store, htmlStore, &stubGenerator{})

	if err := builder.ProcessTask(context.Background(), task.ID); err == nil {
		t.Fatalf("expected error")
	}

	updated := store.tasks[task.ID]
	if updated.Status != "failed" {
		t.Fatalf("expected failed status, got %s", updated.Status)
	}
}
