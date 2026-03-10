package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
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
	if file.Version <= 0 {
		file.Version = 1
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

func (s *memorySiteFileStore) Create(ctx context.Context, file sqlstore.SiteFile) error {
	s.add(file)
	return nil
}

func (s *memorySiteFileStore) List(ctx context.Context, domainID string) ([]sqlstore.SiteFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]sqlstore.SiteFile, 0)
	for _, f := range s.files {
		if f.DomainID == domainID && !f.DeletedAt.Valid {
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
	if file.DeletedAt.Valid {
		return nil, sql.ErrNoRows
	}
	copy := file
	return &copy, nil
}

func (s *memorySiteFileStore) GetByPathAny(ctx context.Context, domainID, path string) (*sqlstore.SiteFile, error) {
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
	file.Version++
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

func (s *memorySiteFileStore) SoftDelete(ctx context.Context, fileID string, deletedBy sql.NullString, reason sql.NullString) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, ok := s.files[fileID]
	if !ok {
		return sql.ErrNoRows
	}
	file.DeletedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	file.DeletedBy = deletedBy
	file.DeleteReason = reason
	file.UpdatedAt = time.Now().UTC()
	s.files[fileID] = file
	return nil
}

func (s *memorySiteFileStore) Restore(ctx context.Context, fileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, ok := s.files[fileID]
	if !ok {
		return sql.ErrNoRows
	}
	file.DeletedAt = sql.NullTime{}
	file.DeletedBy = sql.NullString{}
	file.DeleteReason = sql.NullString{}
	file.UpdatedAt = time.Now().UTC()
	s.files[fileID] = file
	return nil
}

func (s *memorySiteFileStore) ListDeleted(ctx context.Context, domainID string) ([]sqlstore.SiteFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]sqlstore.SiteFile, 0)
	for _, f := range s.files {
		if f.DomainID == domainID && f.DeletedAt.Valid {
			res = append(res, f)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Path < res[j].Path
	})
	return res, nil
}

func (s *memorySiteFileStore) Move(ctx context.Context, fileID, newPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, ok := s.files[fileID]
	if !ok {
		return sql.ErrNoRows
	}
	delete(s.byDomainPath, file.DomainID+"::"+file.Path)
	file.Path = newPath
	file.UpdatedAt = time.Now().UTC()
	s.files[fileID] = file
	s.byDomainPath[file.DomainID+"::"+newPath] = fileID
	return nil
}

func (s *memorySiteFileStore) SetLastEditedBy(ctx context.Context, fileID string, editedBy sql.NullString) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, ok := s.files[fileID]
	if !ok {
		return sql.ErrNoRows
	}
	file.LastEditedBy = editedBy
	file.UpdatedAt = time.Now().UTC()
	s.files[fileID] = file
	return nil
}

type memoryFileEditStore struct {
	mu        sync.Mutex
	edits     []sqlstore.FileEdit
	revisions map[string]sqlstore.FileRevision
}

func newMemoryFileEditStore() *memoryFileEditStore {
	return &memoryFileEditStore{
		edits:     make([]sqlstore.FileEdit, 0),
		revisions: make(map[string]sqlstore.FileRevision),
	}
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

func (s *memoryFileEditStore) CreateRevision(ctx context.Context, rev sqlstore.FileRevision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revisions[rev.ID] = rev
	return nil
}

func (s *memoryFileEditStore) GetRevision(ctx context.Context, revisionID string) (*sqlstore.FileRevision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rev, ok := s.revisions[revisionID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	copy := rev
	return &copy, nil
}

func (s *memoryFileEditStore) ListRevisionsByFile(ctx context.Context, fileID string, limit int) ([]sqlstore.FileRevision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	items := make([]sqlstore.FileRevision, 0)
	for _, rev := range s.revisions {
		if rev.FileID == fileID {
			items = append(items, rev)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Version == items[j].Version {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].Version > items[j].Version
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
		Role:  "user",
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
		Role:  "user",
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
		Role:  "user",
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

func TestFileAPI_CreateMoveHistoryRevertByPath(t *testing.T) {
	s, _, domainID, domainDir, _ := setupFileAPIDomainFixture(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	createBody := `{"path":"pages/about.html","kind":"file","content":"<h1>About</h1>"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/files", strings.NewReader(createBody)).WithContext(adminCtx)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	s.handleDomainActions(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status: %d %s", createRec.Code, createRec.Body.String())
	}

	moveBody := `{"op":"move","new_path":"pages/about-us.html"}`
	moveReq := httptest.NewRequest(http.MethodPatch, "/api/domains/"+domainID+"/files/pages/about.html", strings.NewReader(moveBody)).WithContext(adminCtx)
	moveReq.Header.Set("Content-Type", "application/json")
	moveRec := httptest.NewRecorder()
	s.handleDomainActions(moveRec, moveReq)
	if moveRec.Code != http.StatusOK {
		t.Fatalf("move status: %d %s", moveRec.Code, moveRec.Body.String())
	}

	updateBody := `{"content":"<h1>About v2</h1>","expected_version":1}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/domains/"+domainID+"/files/pages/about-us.html", strings.NewReader(updateBody)).WithContext(adminCtx)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	s.handleDomainActions(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status: %d %s", updateRec.Code, updateRec.Body.String())
	}

	historyReq := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/files/pages/about-us.html/history", nil).WithContext(adminCtx)
	historyRec := httptest.NewRecorder()
	s.handleDomainActions(historyRec, historyReq)
	if historyRec.Code != http.StatusOK {
		t.Fatalf("history status: %d %s", historyRec.Code, historyRec.Body.String())
	}
	var revisions []fileRevisionDTO
	if err := json.NewDecoder(historyRec.Body).Decode(&revisions); err != nil {
		t.Fatalf("decode revisions: %v", err)
	}
	if len(revisions) == 0 {
		t.Fatalf("expected revisions")
	}
	var revertID string
	for _, rev := range revisions {
		if rev.Version == 1 {
			revertID = rev.ID
			break
		}
	}
	if revertID == "" {
		t.Fatalf("could not find version 1 revision")
	}

	revertBody := fmt.Sprintf(`{"revision_id":"%s","description":"rollback to v1"}`, revertID)
	revertReq := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/files/pages/about-us.html/revert", strings.NewReader(revertBody)).WithContext(adminCtx)
	revertReq.Header.Set("Content-Type", "application/json")
	revertRec := httptest.NewRecorder()
	s.handleDomainActions(revertRec, revertReq)
	if revertRec.Code != http.StatusOK {
		t.Fatalf("revert status: %d %s", revertRec.Code, revertRec.Body.String())
	}

	updatedBytes, err := os.ReadFile(filepath.Join(domainDir, "pages", "about-us.html"))
	if err != nil {
		t.Fatalf("read reverted file: %v", err)
	}
	if string(updatedBytes) != "<h1>About</h1>" {
		t.Fatalf("unexpected reverted content: %s", string(updatedBytes))
	}
}

func TestFileAPI_OptimisticConflict(t *testing.T) {
	s, _, domainID, _, _ := setupFileAPIDomainFixture(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	updateBody := `{"content":"<h1>Conflict</h1>","expected_version":99}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/domains/"+domainID+"/files/index.html", strings.NewReader(updateBody)).WithContext(adminCtx)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	s.handleDomainActions(updateRec, updateReq)
	if updateRec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", updateRec.Code, updateRec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(updateRec.Body).Decode(&body); err != nil {
		t.Fatalf("decode conflict body: %v", err)
	}
	if body["current_version"] == nil {
		t.Fatalf("expected current_version in conflict body")
	}
}

func TestFileAPI_ExpectedPathConflict(t *testing.T) {
	s, _, domainID, _, _ := setupFileAPIDomainFixture(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	updateBody := `{"content":"<h1>Conflict</h1>","expected_path":"other.html"}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/domains/"+domainID+"/files/index.html", strings.NewReader(updateBody)).WithContext(adminCtx)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	s.handleDomainActions(updateRec, updateReq)
	if updateRec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", updateRec.Code, updateRec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(updateRec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "path conflict" {
		t.Fatalf("unexpected error payload: %+v", body)
	}
	if body["actual_path"] != "index.html" {
		t.Fatalf("expected actual_path=index.html, got %+v", body["actual_path"])
	}
}

func TestFileAPI_CreateFolderVisibleInTreeList(t *testing.T) {
	s, _, domainID, _, _ := setupFileAPIDomainFixture(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	createBody := `{"path":"pages/blog","kind":"dir"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/files", strings.NewReader(createBody)).WithContext(adminCtx)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	s.handleDomainActions(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create dir status: %d %s", createRec.Code, createRec.Body.String())
	}

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
	found := false
	for _, item := range listResp {
		if item.Path == "pages/blog" {
			found = true
			if item.MimeType != "inode/directory" {
				t.Fatalf("expected directory mime type, got %s", item.MimeType)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected folder pages/blog in file list")
	}
}

func TestFileAPI_DeleteEmptyFolder(t *testing.T) {
	s, _, domainID, _, _ := setupFileAPIDomainFixture(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	createBody := `{"path":"pages/archive","kind":"dir"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/files", strings.NewReader(createBody)).WithContext(adminCtx)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	s.handleDomainActions(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create dir status: %d %s", createRec.Code, createRec.Body.String())
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/domains/"+domainID+"/files/pages/archive", nil).WithContext(adminCtx)
	delRec := httptest.NewRecorder()
	s.handleDomainActions(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete empty dir status: %d %s", delRec.Code, delRec.Body.String())
	}

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
	for _, item := range listResp {
		if item.Path == "pages/archive" {
			t.Fatalf("folder pages/archive should be removed")
		}
	}
}

func TestFileAPI_DeleteFolderRequiresRecursiveForNonEmpty(t *testing.T) {
	s, _, domainID, _, _ := setupFileAPIDomainFixture(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	createDirReq := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/files", strings.NewReader(`{"path":"pages/blog","kind":"dir"}`)).WithContext(adminCtx)
	createDirReq.Header.Set("Content-Type", "application/json")
	createDirRec := httptest.NewRecorder()
	s.handleDomainActions(createDirRec, createDirReq)
	if createDirRec.Code != http.StatusCreated {
		t.Fatalf("create dir status: %d %s", createDirRec.Code, createDirRec.Body.String())
	}

	createFileReq := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/files", strings.NewReader(`{"path":"pages/blog/post.html","kind":"file","content":"<h1>Post</h1>"}`)).WithContext(adminCtx)
	createFileReq.Header.Set("Content-Type", "application/json")
	createFileRec := httptest.NewRecorder()
	s.handleDomainActions(createFileRec, createFileReq)
	if createFileRec.Code != http.StatusCreated {
		t.Fatalf("create file status: %d %s", createFileRec.Code, createFileRec.Body.String())
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/domains/"+domainID+"/files/pages/blog", nil).WithContext(adminCtx)
	delRec := httptest.NewRecorder()
	s.handleDomainActions(delRec, delReq)
	if delRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for non-empty dir without recursive, got %d %s", delRec.Code, delRec.Body.String())
	}

	delRecursiveReq := httptest.NewRequest(http.MethodDelete, "/api/domains/"+domainID+"/files/pages/blog?recursive=1", nil).WithContext(adminCtx)
	delRecursiveRec := httptest.NewRecorder()
	s.handleDomainActions(delRecursiveRec, delRecursiveReq)
	if delRecursiveRec.Code != http.StatusOK {
		t.Fatalf("delete recursive status: %d %s", delRecursiveRec.Code, delRecursiveRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/files/pages/blog/post.html", nil).WithContext(adminCtx)
	getRec := httptest.NewRecorder()
	s.handleDomainActions(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after recursive delete, got %d %s", getRec.Code, getRec.Body.String())
	}
}

func TestFileAPI_UploadRejectsExecutable(t *testing.T) {
	s, _, domainID, _, _ := setupFileAPIDomainFixture(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "malware.exe")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("MZ-this-is-not-allowed")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.WriteField("path", "malware.exe"); err != nil {
		t.Fatalf("write path field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/files/upload", &body).WithContext(adminCtx)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "not allowed") {
		t.Fatalf("expected not allowed error, got %s", rec.Body.String())
	}
}

func TestFileAPI_UploadRejectsBrokenImagePayload(t *testing.T) {
	s, _, domainID, _, _ := setupFileAPIDomainFixture(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "hero.webp")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	// RIFF/WEBP header-like payload with invalid body.
	broken := []byte{'R', 'I', 'F', 'F', 0x10, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' ', 0x00, 0x00, 0x00, 0x00}
	if _, err := part.Write(broken); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.WriteField("path", "assets/hero.webp"); err != nil {
		t.Fatalf("write path field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/files/upload", &body).WithContext(adminCtx)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	lowerBody := strings.ToLower(rec.Body.String())
	if !strings.Contains(lowerBody, "invalid image") && !strings.Contains(lowerBody, "invalid webp") {
		t.Fatalf("expected invalid image error, got %s", rec.Body.String())
	}
}

func TestEditorAssetRegenerateRequiresPrompt(t *testing.T) {
	s, _, domainID, _, _ := setupFileAPIDomainFixture(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	body := `{"path":"assets/hero.webp","mime_type":"image/webp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/editor/ai-regenerate-asset", strings.NewReader(body)).WithContext(adminCtx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "prompt is required") {
		t.Fatalf("expected prompt required error, got %s", rec.Body.String())
	}
}

func TestEditorAssetRegenerateRejectsNonImagePath(t *testing.T) {
	s, _, domainID, _, _ := setupFileAPIDomainFixture(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	body := `{"path":"about.html","prompt":"create hero image","mime_type":"text/html"}`
	req := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/editor/ai-regenerate-asset", strings.NewReader(body)).WithContext(adminCtx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleDomainActions(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "image") {
		t.Fatalf("expected image validation error, got %s", rec.Body.String())
	}
}

func TestFileAPI_SoftDeleteAndRestore(t *testing.T) {
	s, _, domainID, _, _ := setupFileAPIDomainFixture(t)
	adminCtx := context.WithValue(context.Background(), currentUserContextKey, auth.User{
		Email: "admin@example.com",
		Role:  "admin",
	})

	delReq := httptest.NewRequest(http.MethodDelete, "/api/domains/"+domainID+"/files/index.html", nil).WithContext(adminCtx)
	delRec := httptest.NewRecorder()
	s.handleDomainActions(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete status: %d %s", delRec.Code, delRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/files/index.html", nil).WithContext(adminCtx)
	getRec := httptest.NewRecorder()
	s.handleDomainActions(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after soft-delete, got %d %s", getRec.Code, getRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/files?include_deleted=1", nil).WithContext(adminCtx)
	listRec := httptest.NewRecorder()
	s.handleDomainActions(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status: %d %s", listRec.Code, listRec.Body.String())
	}
	var listResp []fileDTO
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	hasDeleted := false
	for _, item := range listResp {
		if item.Path == "index.html" && item.DeletedAt != nil {
			hasDeleted = true
			break
		}
	}
	if !hasDeleted {
		t.Fatalf("expected deleted file in include_deleted list")
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/files/index.html/restore", nil).WithContext(adminCtx)
	restoreRec := httptest.NewRecorder()
	s.handleDomainActions(restoreRec, restoreReq)
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("restore status: %d %s", restoreRec.Code, restoreRec.Body.String())
	}

	getReq2 := httptest.NewRequest(http.MethodGet, "/api/domains/"+domainID+"/files/index.html", nil).WithContext(adminCtx)
	getRec2 := httptest.NewRecorder()
	s.handleDomainActions(getRec2, getReq2)
	if getRec2.Code != http.StatusOK {
		t.Fatalf("expected 200 after restore, got %d %s", getRec2.Code, getRec2.Body.String())
	}
}
