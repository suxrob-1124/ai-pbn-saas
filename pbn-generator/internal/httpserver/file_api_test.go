package httpserver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

type memorySiteFileStore struct {
	mu           sync.Mutex
	files        map[string]sqlstore.SiteFile
	byDomainPath map[string]string
}

func newMemorySiteFileStore() *memorySiteFileStore {
	return &memorySiteFileStore{
		files:        make(map[string]sqlstore.SiteFile),
		byDomainPath: make(map[string]string),
	}
}

func (s *memorySiteFileStore) add(file sqlstore.SiteFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if file.CreatedAt.IsZero() {
		file.CreatedAt = time.Now().UTC()
	}
	if file.UpdatedAt.IsZero() {
		file.UpdatedAt = file.CreatedAt
	}
	s.files[file.ID] = file
	key := file.DomainID + "::" + file.Path
	s.byDomainPath[key] = file.ID
}

func (s *memorySiteFileStore) Get(ctx context.Context, fileID string) (*sqlstore.SiteFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, ok := s.files[fileID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	copy := file
	return &copy, nil
}

func (s *memorySiteFileStore) List(ctx context.Context, domainID string) ([]sqlstore.SiteFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]sqlstore.SiteFile, 0)
	for _, f := range s.files {
		if f.DomainID == domainID {
			res = append(res, f)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Path < res[j].Path
	})
	return res, nil
}

func (s *memorySiteFileStore) GetByPath(ctx context.Context, domainID, path string) (*sqlstore.SiteFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byDomainPath[domainID+"::"+path]
	if !ok {
		return nil, sql.ErrNoRows
	}
	file, ok := s.files[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	copy := file
	return &copy, nil
}

func (s *memorySiteFileStore) Update(ctx context.Context, fileID string, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, ok := s.files[fileID]
	if !ok {
		return sql.ErrNoRows
	}
	hash := sha256.Sum256(content)
	file.ContentHash = sql.NullString{String: hex.EncodeToString(hash[:]), Valid: true}
	file.SizeBytes = int64(len(content))
	file.MimeType = detectMimeType(file.Path, content)
	file.UpdatedAt = time.Now().UTC()
	s.files[fileID] = file
	return nil
}

func (s *memorySiteFileStore) Delete(ctx context.Context, fileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, ok := s.files[fileID]
	if !ok {
		return sql.ErrNoRows
	}
	delete(s.files, fileID)
	delete(s.byDomainPath, file.DomainID+"::"+file.Path)
	return nil
}

type memoryFileEditStore struct {
	mu    sync.Mutex
	edits []sqlstore.FileEdit
}

func newMemoryFileEditStore() *memoryFileEditStore {
	return &memoryFileEditStore{edits: make([]sqlstore.FileEdit, 0)}
}

func (s *memoryFileEditStore) Create(ctx context.Context, edit sqlstore.FileEdit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if edit.CreatedAt.IsZero() {
		edit.CreatedAt = time.Now().UTC()
	}
	s.edits = append(s.edits, edit)
	return nil
}

func (s *memoryFileEditStore) ListByFile(ctx context.Context, fileID string, limit int) ([]sqlstore.FileEdit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	items := make([]sqlstore.FileEdit, 0, len(s.edits))
	for _, e := range s.edits {
		if e.FileID == fileID {
			items = append(items, e)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func setupFileAPIDomainFixture(t *testing.T) (*Server, string, string, string, *memorySiteFileStore) {
	t.Helper()
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir server: %v", err)
	}
	content := []byte("<h1>Hello</h1>")
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	origWD, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	s := setupServer(t)
	siteStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	s.siteFiles = siteStore
	s.fileEdits = editStore

	projectID := "project-1"
	domainID := "domain-1"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.domains.(*stubDomainStore).domains[domainID] = sqlstore.Domain{
		ID:          domainID,
		ProjectID:   projectID,
		URL:         "example.com",
		MainKeyword: "keyword",
		Status:      "published",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	siteStore.add(sqlstore.SiteFile{
		ID:        "file-1",
		DomainID:  domainID,
		Path:      "index.html",
		SizeBytes: int64(len(content)),
		MimeType:  "text/html",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	return s, projectID, domainID, domainDir, siteStore
}

func TestFileAPI_ListReadSaveHistory(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir server: %v", err)
	}
	content := []byte("<h1>Hello</h1>")
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	origWD, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	s := setupServer(t)
	siteStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	s.siteFiles = siteStore
	s.fileEdits = editStore

	projectID := "project-1"
	domainID := "domain-1"
	projStore := s.projects.(*stubProjectStore)
	projStore.projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	domainStore := s.domains.(*stubDomainStore)
	domainStore.domains[domainID] = sqlstore.Domain{
		ID:          domainID,
		ProjectID:   projectID,
		URL:         "example.com",
		MainKeyword: "keyword",
		Status:      "waiting",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	siteStore.add(sqlstore.SiteFile{
		ID:        "file-1",
		DomainID:  domainID,
		Path:      "index.html",
		SizeBytes: int64(len(content)),
		MimeType:  "text/html",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	listReq := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/files", nil).WithContext(adminCtx)
	listRec := httptest.NewRecorder()
	s.handleDomainActions(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status: %d %s", listRec.Code, listRec.Body.String())
	}
	var listResp []fileDTO
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp) != 1 || listResp[0].Path != "index.html" {
		t.Fatalf("unexpected list response: %+v", listResp)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/files/index.html", nil).WithContext(adminCtx)
	getRec := httptest.NewRecorder()
	s.handleDomainActions(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status: %d %s", getRec.Code, getRec.Body.String())
	}
	var getResp struct {
		Content  string `json:"content"`
		MimeType string `json:"mimeType"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&getResp); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if getResp.Content != string(content) || !strings.HasPrefix(getResp.MimeType, "text/html") {
		t.Fatalf("unexpected get response: %+v", getResp)
	}

	updateBody := `{"content":"<h1>Updated</h1>","description":"update heading"}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/domains/"+domainID+"/files/index.html", strings.NewReader(updateBody)).WithContext(adminCtx)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	s.handleDomainActions(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status: %d %s", updateRec.Code, updateRec.Body.String())
	}
	updatedBytes, err := os.ReadFile(filepath.Join(domainDir, "index.html"))
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(updatedBytes) != "<h1>Updated</h1>" {
		t.Fatalf("unexpected file content: %s", string(updatedBytes))
	}
	updatedFile, err := siteStore.Get(context.Background(), "file-1")
	if err != nil {
		t.Fatalf("get updated file: %v", err)
	}
	if updatedFile.SizeBytes != int64(len(updatedBytes)) {
		t.Fatalf("unexpected size: %d", updatedFile.SizeBytes)
	}
	if !updatedFile.ContentHash.Valid || updatedFile.ContentHash.String == "" {
		t.Fatalf("expected content hash to be set")
	}
	if len(editStore.edits) != 1 {
		t.Fatalf("expected 1 edit entry, got %d", len(editStore.edits))
	}

	historyReq := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/files/file-1/history", nil).WithContext(adminCtx)
	historyRec := httptest.NewRecorder()
	s.handleDomainActions(historyRec, historyReq)
	if historyRec.Code != http.StatusOK {
		t.Fatalf("history status: %d %s", historyRec.Code, historyRec.Body.String())
	}
	var historyResp []fileEditDTO
	if err := json.NewDecoder(historyRec.Body).Decode(&historyResp); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(historyResp) != 1 || historyResp[0].EditedBy == "" {
		t.Fatalf("unexpected history response: %+v", historyResp)
	}
}

func TestFileAPI_ReadMissingFile(t *testing.T) {
	tempDir := t.TempDir()
	serverDir := filepath.Join(tempDir, "server")
	domainDir := filepath.Join(serverDir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir server: %v", err)
	}
	origWD, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	s := setupServer(t)
	s.siteFiles = newMemorySiteFileStore()
	s.fileEdits = newMemoryFileEditStore()

	projectID := "project-1"
	domainID := "domain-1"
	s.projects.(*stubProjectStore).projects[projectID] = sqlstore.Project{
		ID:        projectID,
		UserEmail: "owner@example.com",
		Name:      "Project",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.domains.(*stubDomainStore).domains[domainID] = sqlstore.Domain{
		ID:          domainID,
		ProjectID:   projectID,
		URL:         "example.com",
		MainKeyword: "keyword",
		Status:      "waiting",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	getReq := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/files/missing.html", nil).WithContext(adminCtx)
	getRec := httptest.NewRecorder()
	s.handleDomainActions(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", getRec.Code, getRec.Body.String())
	}
}

func TestFileAPI_OwnerCanUpdate(t *testing.T) {
	s, _, domainID, domainDir, _ := setupFileAPIDomainFixture(t)
	ownerCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "owner@example.com",
		Role:  "manager",
	})

	updateReq := httptest.NewRequest(http.MethodPut, "/api/domains/"+domainID+"/files/index.html", strings.NewReader(`{"content":"<h1>Owner Edit</h1>"}`)).WithContext(ownerCtx)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	s.handleDomainActions(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	updatedBytes, err := os.ReadFile(filepath.Join(domainDir, "index.html"))
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(updatedBytes) != "<h1>Owner Edit</h1>" {
		t.Fatalf("unexpected file content: %s", string(updatedBytes))
	}
}

func TestFileAPI_ViewerCannotUpdate(t *testing.T) {
	s, projectID, domainID, _, _ := setupFileAPIDomainFixture(t)
	viewerEmail := "viewer@example.com"

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	s.projectMembers = sqlstore.NewProjectMemberStore(db)

	query := regexp.QuoteMeta("SELECT project_id, user_email, role, created_at FROM project_members WHERE project_id=$1 AND user_email=$2")
	rows := sqlmock.NewRows([]string{"project_id", "user_email", "role", "created_at"}).AddRow(projectID, viewerEmail, "viewer", time.Now().UTC())
	mock.ExpectQuery(query).WithArgs(projectID, viewerEmail).WillReturnRows(rows)
	rows2 := sqlmock.NewRows([]string{"project_id", "user_email", "role", "created_at"}).AddRow(projectID, viewerEmail, "viewer", time.Now().UTC())
	mock.ExpectQuery(query).WithArgs(projectID, viewerEmail).WillReturnRows(rows2)

	viewerCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: viewerEmail,
		Role:  "manager",
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/domains/"+domainID+"/files/index.html", strings.NewReader(`{"content":"<h1>Viewer Edit</h1>"}`)).WithContext(viewerCtx)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	s.handleDomainActions(updateRec, updateReq)
	if updateRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", updateRec.Code, updateRec.Body.String())
	}
	if !strings.Contains(updateRec.Body.String(), "editor role required") {
		t.Fatalf("expected editor permission error, got %s", updateRec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestFileAPI_EditorCanUpdate(t *testing.T) {
	s, projectID, domainID, domainDir, _ := setupFileAPIDomainFixture(t)
	editorEmail := "editor@example.com"

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	s.projectMembers = sqlstore.NewProjectMemberStore(db)

	query := regexp.QuoteMeta("SELECT project_id, user_email, role, created_at FROM project_members WHERE project_id=$1 AND user_email=$2")
	rows := sqlmock.NewRows([]string{"project_id", "user_email", "role", "created_at"}).AddRow(projectID, editorEmail, "editor", time.Now().UTC())
	mock.ExpectQuery(query).WithArgs(projectID, editorEmail).WillReturnRows(rows)
	rows2 := sqlmock.NewRows([]string{"project_id", "user_email", "role", "created_at"}).AddRow(projectID, editorEmail, "editor", time.Now().UTC())
	mock.ExpectQuery(query).WithArgs(projectID, editorEmail).WillReturnRows(rows2)

	editorCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: editorEmail,
		Role:  "manager",
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/domains/"+domainID+"/files/index.html", strings.NewReader(`{"content":"<h1>Editor Edit</h1>"}`)).WithContext(editorCtx)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	s.handleDomainActions(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	updatedBytes, err := os.ReadFile(filepath.Join(domainDir, "index.html"))
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(updatedBytes) != "<h1>Editor Edit</h1>" {
		t.Fatalf("unexpected file content: %s", string(updatedBytes))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
