package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// ─── isRequestCanceled unit tests ─────────────────────────────────────────────

func TestIsRequestCanceled(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"context.Canceled", context.Canceled, true},
		{"context.DeadlineExceeded", context.DeadlineExceeded, true},
		{"wrapped Canceled", fmt.Errorf("ssh read: %w", context.Canceled), true},
		{"wrapped DeadlineExceeded", fmt.Errorf("timeout: %w", context.DeadlineExceeded), true},
		{"sql.ErrNoRows", sql.ErrNoRows, false},
		{"fs.ErrPermission", fs.ErrPermission, false},
		{"fs.ErrNotExist", fs.ErrNotExist, false},
		{"generic error", fmt.Errorf("connection refused"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isRequestCanceled(tc.err)
			if got != tc.want {
				t.Errorf("isRequestCanceled(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// ─── handleGetFile cancellation tests ─────────────────────────────────────────

// cancelingBackend is a SiteContentBackend stub that always returns the given
// error from ReadFile. All other methods are no-ops.
type cancelingBackend struct{ readErr error }

func (b *cancelingBackend) ReadFile(_ context.Context, _ domainfs.DomainFSContext, _ string) ([]byte, error) {
	return nil, b.readErr
}
func (b *cancelingBackend) WriteFile(_ context.Context, _ domainfs.DomainFSContext, _ string, _ []byte) error {
	return nil
}
func (b *cancelingBackend) EnsureDir(_ context.Context, _ domainfs.DomainFSContext, _ string) error {
	return nil
}
func (b *cancelingBackend) Stat(_ context.Context, _ domainfs.DomainFSContext, _ string) (domainfs.FileInfo, error) {
	return domainfs.FileInfo{}, nil
}
func (b *cancelingBackend) ReadDir(_ context.Context, _ domainfs.DomainFSContext, _ string) ([]domainfs.EntryInfo, error) {
	return nil, nil
}
func (b *cancelingBackend) Move(_ context.Context, _ domainfs.DomainFSContext, _, _ string) error {
	return nil
}
func (b *cancelingBackend) Delete(_ context.Context, _ domainfs.DomainFSContext, _ string) error {
	return nil
}
func (b *cancelingBackend) DeleteAll(_ context.Context, _ domainfs.DomainFSContext, _ string) error {
	return nil
}
func (b *cancelingBackend) ListTree(_ context.Context, _ domainfs.DomainFSContext, _ string) ([]domainfs.FileInfo, error) {
	return nil, nil
}
func (b *cancelingBackend) DiscoverDomain(_ context.Context, _, _ string) (string, string, error) {
	return "", "", nil
}

// fileReturningStore is a SiteFileStore stub that returns a specific file for
// any GetByPath call. It embeds stubSiteFileStore for all other methods.
type fileReturningStore struct {
	*stubSiteFileStore
	file *sqlstore.SiteFile
}

func (s *fileReturningStore) GetByPath(_ context.Context, _, _ string) (*sqlstore.SiteFile, error) {
	if s.file != nil {
		return s.file, nil
	}
	return nil, sql.ErrNoRows
}

func newFileReturningStore(file *sqlstore.SiteFile) *fileReturningStore {
	return &fileReturningStore{stubSiteFileStore: newStubSiteFileStore(), file: file}
}

// setupGetFileServer builds a minimal Server for handleGetFile tests.
// backendErr is what ReadFile will return; if file is non-nil it is returned by GetByPath.
func setupGetFileServer(t *testing.T, file *sqlstore.SiteFile, backendErr error) *Server {
	t.Helper()
	srv := setupServer(t)
	srv.contentBackend = &cancelingBackend{readErr: backendErr}
	if file != nil {
		srv.siteFiles = newFileReturningStore(file)
	}
	return srv
}

func fakeFile(path string) *sqlstore.SiteFile {
	return &sqlstore.SiteFile{
		ID:       "file-test-1",
		DomainID: "dom-test",
		Path:     path,
		MimeType: "text/html",
	}
}

func fakeDomain() sqlstore.Domain {
	return sqlstore.Domain{ID: "dom-test", URL: "example.com"}
}

// TestHandleGetFile_ContextCanceled verifies that a context.Canceled error from
// the content backend does NOT produce a 500 response — handler returns silently.
func TestHandleGetFile_ContextCanceled(t *testing.T) {
	srv := setupGetFileServer(t, fakeFile("index.html"), context.Canceled)

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	w := httptest.NewRecorder()

	srv.handleGetFile(w, req, fakeDomain(), "index.html")

	if w.Code == http.StatusInternalServerError {
		t.Errorf("context.Canceled should not produce 500, got %d body=%s", w.Code, w.Body.String())
	}
	// Response body must be empty — no false error message written.
	if w.Body.Len() > 0 {
		t.Errorf("expected empty response body on client cancel, got: %s", w.Body.String())
	}
}

// TestHandleGetFile_DeadlineExceeded verifies that context.DeadlineExceeded is
// also handled as a cancellation (not a 500).
func TestHandleGetFile_DeadlineExceeded(t *testing.T) {
	srv := setupGetFileServer(t, fakeFile("page.html"), context.DeadlineExceeded)

	req := httptest.NewRequest(http.MethodGet, "/page.html", nil)
	w := httptest.NewRecorder()

	srv.handleGetFile(w, req, fakeDomain(), "page.html")

	if w.Code == http.StatusInternalServerError {
		t.Errorf("context.DeadlineExceeded should not produce 500, got %d", w.Code)
	}
}

// TestHandleGetFile_RealErrorStill500 verifies that a genuine read error (not a
// cancellation) still produces a 500.
func TestHandleGetFile_RealErrorStill500(t *testing.T) {
	srv := setupGetFileServer(t, fakeFile("index.html"), fmt.Errorf("ssh: connection reset by peer"))

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	w := httptest.NewRecorder()

	srv.handleGetFile(w, req, fakeDomain(), "index.html")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("real backend error should produce 500, got %d", w.Code)
	}
}

// TestHandleGetFile_FileNotFound verifies sql.ErrNoRows → 404 is unchanged.
func TestHandleGetFile_FileNotFound(t *testing.T) {
	// siteFiles.GetByPath returns ErrNoRows (default stub behaviour)
	srv := setupGetFileServer(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/missing.html", nil)
	w := httptest.NewRecorder()

	srv.handleGetFile(w, req, fakeDomain(), "missing.html")

	if w.Code != http.StatusNotFound {
		t.Errorf("missing file should produce 404, got %d", w.Code)
	}
}

// TestHandleGetFile_PermissionDenied verifies fs.ErrPermission → 403 is unchanged.
func TestHandleGetFile_PermissionDenied(t *testing.T) {
	srv := setupGetFileServer(t, fakeFile("secret.html"), fs.ErrPermission)

	req := httptest.NewRequest(http.MethodGet, "/secret.html", nil)
	w := httptest.NewRecorder()

	srv.handleGetFile(w, req, fakeDomain(), "secret.html")

	if w.Code != http.StatusForbidden {
		t.Errorf("permission error should produce 403, got %d", w.Code)
	}
}

// ─── handleFileHistoryByPath snapshot filter tests ────────────────────────────

// TestHandleFileHistory_HidesSnapshotRevisions verifies that revisions with
// source "agent_snapshot:<id>" are NOT returned by the history endpoint — they
// are internal rollback markers, not user-visible edit history.
func TestHandleFileHistory_HidesSnapshotRevisions(t *testing.T) {
	srv := setupServer(t)
	fileStore := newMemorySiteFileStore()
	editStore := newMemoryFileEditStore()
	srv.siteFiles = fileStore
	srv.fileEdits = editStore

	domainID := "dom-hist"
	fileID := "hist-file-1"
	sessionID := "sess-hist-1"

	fileStore.add(sqlstore.SiteFile{
		ID:       fileID,
		DomainID: domainID,
		Path:     "index.html",
		MimeType: "text/html",
		Version:  3,
	})

	ctx := context.Background()
	// Revision 1: user manual edit — should be visible
	_ = editStore.CreateRevision(ctx, sqlstore.FileRevision{
		ID: "rev-manual", FileID: fileID, Version: 1,
		Content: []byte("<html>v1</html>"), SizeBytes: 15, MimeType: "text/html",
		Source: "manual", EditedBy: "user@example.com",
	})
	// Revision 2: agent snapshot (rollback baseline) — must be hidden
	_ = editStore.CreateRevision(ctx, sqlstore.FileRevision{
		ID: "rev-snap", FileID: fileID, Version: 2,
		Content: []byte("<html>v1</html>"), SizeBytes: 15, MimeType: "text/html",
		Source: "agent_snapshot:" + sessionID, EditedBy: "user@example.com",
	})
	// Revision 3: final agent edit — should be visible
	_ = editStore.CreateRevision(ctx, sqlstore.FileRevision{
		ID: "rev-agent", FileID: fileID, Version: 3,
		Content: []byte("<html>v2</html>"), SizeBytes: 15, MimeType: "text/html",
		Source: "agent", EditedBy: "user@example.com",
	})

	req := httptest.NewRequest(http.MethodGet, "/history", nil)
	w := httptest.NewRecorder()
	srv.handleFileHistoryByPath(w, req, domainID, "index.html")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var revisions []fileRevisionDTO
	if err := json.NewDecoder(w.Body).Decode(&revisions); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Expect exactly 2 visible revisions (manual + agent), not the snapshot.
	if len(revisions) != 2 {
		t.Errorf("expected 2 visible revisions, got %d", len(revisions))
	}
	for _, rev := range revisions {
		if strings.HasPrefix(rev.Source, "agent_snapshot:") {
			t.Errorf("agent_snapshot revision must not appear in history, got source=%q", rev.Source)
		}
	}
}
