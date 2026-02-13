package legacy

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

type stubUsers struct {
	users map[string]auth.User
}

func (s *stubUsers) Get(ctx context.Context, email string) (auth.User, error) {
	u, ok := s.users[email]
	if !ok {
		return auth.User{}, errors.New("user not found")
	}
	return u, nil
}

type stubProjects struct {
	listByName map[string][]sqlstore.Project
	created    []sqlstore.Project
	updated    []sqlstore.Project
}

func (s *stubProjects) Create(ctx context.Context, p sqlstore.Project) error {
	s.created = append(s.created, p)
	s.listByName[p.Name] = append([]sqlstore.Project{p}, s.listByName[p.Name]...)
	return nil
}

func (s *stubProjects) Update(ctx context.Context, p sqlstore.Project) error {
	s.updated = append(s.updated, p)
	list := s.listByName[p.Name]
	for i := range list {
		if list[i].ID == p.ID {
			list[i] = p
		}
	}
	s.listByName[p.Name] = list
	return nil
}

func (s *stubProjects) ListByNameExact(ctx context.Context, name string) ([]sqlstore.Project, error) {
	res := s.listByName[name]
	out := make([]sqlstore.Project, len(res))
	copy(out, res)
	return out, nil
}

type stubDomains struct {
	byURL map[string]sqlstore.Domain
	byID  map[string]sqlstore.Domain
}

func (s *stubDomains) GetByURL(ctx context.Context, url string) (sqlstore.Domain, error) {
	d, ok := s.byURL[url]
	if !ok {
		return sqlstore.Domain{}, sql.ErrNoRows
	}
	return d, nil
}

func (s *stubDomains) Create(ctx context.Context, d sqlstore.Domain) error {
	s.byURL[d.URL] = d
	s.byID[d.ID] = d
	return nil
}

func (s *stubDomains) UpdateProject(ctx context.Context, id, projectID string) error {
	d := s.byID[id]
	d.ProjectID = projectID
	s.byID[id] = d
	s.byURL[d.URL] = d
	return nil
}

func (s *stubDomains) UpdateKeyword(ctx context.Context, id, keyword string) error {
	d := s.byID[id]
	d.MainKeyword = keyword
	s.byID[id] = d
	s.byURL[d.URL] = d
	return nil
}

func (s *stubDomains) UpdateExtras(ctx context.Context, id, country, language string, exclude, server sql.NullString) (bool, error) {
	d := s.byID[id]
	d.TargetCountry = country
	d.TargetLanguage = language
	d.ExcludeDomains = exclude
	d.ServerID = server
	s.byID[id] = d
	s.byURL[d.URL] = d
	return true, nil
}

func (s *stubDomains) UpdateStatus(ctx context.Context, id, status string) error {
	d := s.byID[id]
	d.Status = status
	s.byID[id] = d
	s.byURL[d.URL] = d
	return nil
}

func (s *stubDomains) UpdatePublishState(ctx context.Context, id, publishedPath string, fileCount int, totalSizeBytes int64) error {
	d := s.byID[id]
	d.PublishedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	s.byID[id] = d
	s.byURL[d.URL] = d
	return nil
}

func (s *stubDomains) UpdateLinkSettings(ctx context.Context, id string, anchorText, acceptorURL sql.NullString) (bool, error) {
	d := s.byID[id]
	d.LinkAnchorText = anchorText
	d.LinkAcceptorURL = acceptorURL
	s.byID[id] = d
	s.byURL[d.URL] = d
	return true, nil
}

func (s *stubDomains) UpdateLinkState(ctx context.Context, id string, status string, lastTaskID string, filePath string, anchorSnapshot string) error {
	d := s.byID[id]
	d.LinkStatus = sql.NullString{String: status, Valid: status != ""}
	d.LinkLastTaskID = sql.NullString{String: lastTaskID, Valid: lastTaskID != ""}
	d.LinkFilePath = sql.NullString{String: filePath, Valid: filePath != ""}
	d.LinkAnchorSnapshot = sql.NullString{String: anchorSnapshot, Valid: anchorSnapshot != ""}
	s.byID[id] = d
	s.byURL[d.URL] = d
	return nil
}

func (s *stubDomains) EnsureDefaultServer(ctx context.Context, email string) error {
	return nil
}

type stubSiteFiles struct {
	byDomainPath map[string]map[string]sqlstore.SiteFile
}

func (s *stubSiteFiles) Create(ctx context.Context, file sqlstore.SiteFile) error {
	if s.byDomainPath[file.DomainID] == nil {
		s.byDomainPath[file.DomainID] = map[string]sqlstore.SiteFile{}
	}
	s.byDomainPath[file.DomainID][file.Path] = file
	return nil
}

func (s *stubSiteFiles) GetByPath(ctx context.Context, domainID, path string) (*sqlstore.SiteFile, error) {
	m := s.byDomainPath[domainID]
	f, ok := m[path]
	if !ok {
		return nil, sql.ErrNoRows
	}
	cp := f
	return &cp, nil
}

func (s *stubSiteFiles) List(ctx context.Context, domainID string) ([]sqlstore.SiteFile, error) {
	m := s.byDomainPath[domainID]
	res := make([]sqlstore.SiteFile, 0, len(m))
	for _, v := range m {
		res = append(res, v)
	}
	return res, nil
}

func (s *stubSiteFiles) Update(ctx context.Context, fileID string, content []byte) error {
	for domainID, files := range s.byDomainPath {
		for path, f := range files {
			if f.ID != fileID {
				continue
			}
			f.SizeBytes = int64(len(content))
			files[path] = f
			s.byDomainPath[domainID] = files
			return nil
		}
	}
	return nil
}

func (s *stubSiteFiles) Delete(ctx context.Context, fileID string) error {
	for domainID, files := range s.byDomainPath {
		for path, f := range files {
			if f.ID == fileID {
				delete(files, path)
				s.byDomainPath[domainID] = files
				return nil
			}
		}
	}
	return nil
}

type stubLinks struct {
	created []sqlstore.LinkTask
}

func (s *stubLinks) Create(ctx context.Context, task sqlstore.LinkTask) error {
	s.created = append(s.created, task)
	return nil
}

func TestImporterRunApplyCreatesDomainAndBaseline(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.csv")
	serverDir := filepath.Join(dir, "server")
	if err := os.MkdirAll(filepath.Join(serverDir, "example.com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "example.com", "index.html"), []byte(`<html><body><a href="https://target.example/">Target</a></body></html>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "example.com", "style.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatalf("write css: %v", err)
	}

	csvData := "project_name,owner_email,project_country,project_language,domain_url,main_keyword\n" +
		"proj,owner@example.com,se,sv,example.com,keyword\n"
	if err := os.WriteFile(manifest, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	users := &stubUsers{users: map[string]auth.User{"owner@example.com": {Email: "owner@example.com"}}}
	projects := &stubProjects{listByName: map[string][]sqlstore.Project{}}
	domains := &stubDomains{byURL: map[string]sqlstore.Domain{}, byID: map[string]sqlstore.Domain{}}
	files := &stubSiteFiles{byDomainPath: map[string]map[string]sqlstore.SiteFile{}}
	links := &stubLinks{}

	imp := NewImporter(users, projects, domains, files, links)
	report, err := imp.Run(context.Background(), RunOptions{
		ManifestPath: manifest,
		ServerDir:    serverDir,
		Mode:         ModeApply,
		Batch: BatchConfig{
			BatchSize:   50,
			BatchNumber: 1,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if report.Summary.Success != 1 {
		t.Fatalf("expected success=1, got %d", report.Summary.Success)
	}
	if len(links.created) != 1 {
		t.Fatalf("expected 1 synthetic link task, got %d", len(links.created))
	}
	if links.created[0].TargetURL != "https://target.example/" {
		t.Fatalf("unexpected target url: %s", links.created[0].TargetURL)
	}
	if len(projects.created) != 1 {
		t.Fatalf("expected 1 created project, got %d", len(projects.created))
	}
}

func TestImporterRunFailsOnDuplicateProjectName(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.csv")
	csvData := "project_name,owner_email,project_country,project_language,domain_url,main_keyword\n" +
		"proj,owner@example.com,se,sv,example.com,keyword\n"
	if err := os.WriteFile(manifest, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	users := &stubUsers{users: map[string]auth.User{"owner@example.com": {Email: "owner@example.com"}}}
	projects := &stubProjects{listByName: map[string][]sqlstore.Project{
		"proj": {
			{ID: "p1", Name: "proj", UserEmail: "owner@example.com"},
			{ID: "p2", Name: "proj", UserEmail: "owner@example.com"},
		},
	}}
	domains := &stubDomains{byURL: map[string]sqlstore.Domain{}, byID: map[string]sqlstore.Domain{}}
	files := &stubSiteFiles{byDomainPath: map[string]map[string]sqlstore.SiteFile{}}
	links := &stubLinks{}

	imp := NewImporter(users, projects, domains, files, links)
	report, err := imp.Run(context.Background(), RunOptions{
		ManifestPath: manifest,
		ServerDir:    dir,
		Mode:         ModeDryRun,
		Batch:        BatchConfig{BatchSize: 50, BatchNumber: 1},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if report.Summary.Failed != 1 {
		t.Fatalf("expected failed=1, got %d", report.Summary.Failed)
	}
	if report.Rows[0].Error == "" {
		t.Fatalf("expected error message in row report")
	}
}
