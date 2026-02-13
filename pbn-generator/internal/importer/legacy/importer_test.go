package legacy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	d.PublishedPath = sql.NullString{String: publishedPath, Valid: strings.TrimSpace(publishedPath) != ""}
	d.FileCount = fileCount
	d.TotalSizeBytes = totalSizeBytes
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

func (s *stubDomains) SetLastGeneration(ctx context.Context, id, genID string) error {
	d := s.byID[id]
	d.LastGenerationID = sql.NullString{String: genID, Valid: strings.TrimSpace(genID) != ""}
	s.byID[id] = d
	s.byURL[d.URL] = d
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

type stubGenerations struct {
	byDomain map[string][]sqlstore.Generation
	created  []sqlstore.Generation
	updated  []sqlstore.Generation
}

func (s *stubGenerations) ListByDomain(ctx context.Context, domainID string) ([]sqlstore.Generation, error) {
	items := s.byDomain[domainID]
	out := make([]sqlstore.Generation, len(items))
	copy(out, items)
	return out, nil
}

func (s *stubGenerations) Create(ctx context.Context, g sqlstore.Generation) error {
	s.created = append(s.created, g)
	s.byDomain[g.DomainID] = append([]sqlstore.Generation{g}, s.byDomain[g.DomainID]...)
	return nil
}

func (s *stubGenerations) UpdateFull(ctx context.Context, id, status string, progress int, errText *string, logs, artifacts []byte, started, finished *time.Time, promptID *string) error {
	for domainID, items := range s.byDomain {
		for idx := range items {
			if items[idx].ID != id {
				continue
			}
			items[idx].Status = status
			items[idx].Progress = progress
			items[idx].Logs = logs
			items[idx].Artifacts = artifacts
			if started != nil {
				items[idx].StartedAt = sql.NullTime{Time: *started, Valid: true}
			}
			if finished != nil {
				items[idx].FinishedAt = sql.NullTime{Time: *finished, Valid: true}
			}
			if promptID != nil {
				items[idx].PromptID = sql.NullString{String: *promptID, Valid: true}
			}
			s.byDomain[domainID] = items
			s.updated = append(s.updated, items[idx])
			return nil
		}
	}
	return errors.New("generation not found")
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
	gens := &stubGenerations{byDomain: map[string][]sqlstore.Generation{}}

	imp := NewImporter(users, projects, domains, files, links, gens)
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
	if len(gens.created) != 1 {
		t.Fatalf("expected 1 synthetic generation, got %d", len(gens.created))
	}
	if strings.TrimSpace(gens.created[0].PromptID.String) != LegacyDecodePromptID {
		t.Fatalf("unexpected prompt_id: %s", gens.created[0].PromptID.String)
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
	gens := &stubGenerations{byDomain: map[string][]sqlstore.Generation{}}

	imp := NewImporter(users, projects, domains, files, links, gens)
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

func TestImporterRunApplySkipsLegacyArtifactsWhenNonLegacyExists(t *testing.T) {
	dir := t.TempDir()
	domainDir := filepath.Join(dir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), []byte(`<html><body>skip</body></html>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	domainID := "dom-1"
	gens := &stubGenerations{
		byDomain: map[string][]sqlstore.Generation{
			domainID: {
				{ID: "real-1", DomainID: domainID, PromptID: sql.NullString{String: "manual_prompt", Valid: true}},
			},
		},
	}

	imp := NewImporter(&stubUsers{}, &stubProjects{}, &stubDomains{}, &stubSiteFiles{byDomainPath: map[string]map[string]sqlstore.SiteFile{}}, &stubLinks{}, gens)
	actions, _, err := imp.syncLegacyArtifacts(
		context.Background(),
		sqlstore.Domain{ID: domainID, URL: "example.com"},
		"owner@example.com",
		domainDir,
		"example.com",
		"import_legacy",
		ModeApply,
		false,
	)
	if err != nil {
		t.Fatalf("syncLegacyArtifacts failed: %v", err)
	}
	if !containsAction(actions, "legacy_artifacts_skipped_non_legacy_exists") {
		t.Fatalf("expected skip action, got %#v", actions)
	}
	if len(gens.created) != 0 {
		t.Fatalf("expected no create")
	}
}

func TestImporterRunApplyForceCreatesLegacyArtifacts(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.csv")
	serverDir := filepath.Join(dir, "server")
	if err := os.MkdirAll(filepath.Join(serverDir, "example.com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "example.com", "index.html"), []byte(`<html><body>Hello</body></html>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	csvData := "project_name,owner_email,project_country,project_language,domain_url,main_keyword\n" +
		"proj,owner@example.com,se,sv,example.com,keyword\n"
	if err := os.WriteFile(manifest, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	projectID := "proj-1"
	domainID := "dom-1"
	users := &stubUsers{users: map[string]auth.User{"owner@example.com": {Email: "owner@example.com"}}}
	projects := &stubProjects{listByName: map[string][]sqlstore.Project{
		"proj": {{ID: projectID, Name: "proj", UserEmail: "owner@example.com"}},
	}}
	domains := &stubDomains{
		byURL: map[string]sqlstore.Domain{
			"example.com": {ID: domainID, ProjectID: projectID, URL: "example.com", Status: "published"},
		},
		byID: map[string]sqlstore.Domain{
			domainID: {ID: domainID, ProjectID: projectID, URL: "example.com", Status: "published"},
		},
	}
	files := &stubSiteFiles{byDomainPath: map[string]map[string]sqlstore.SiteFile{}}
	links := &stubLinks{}
	gens := &stubGenerations{
		byDomain: map[string][]sqlstore.Generation{
			domainID: {
				{ID: "real-1", DomainID: domainID, PromptID: sql.NullString{String: "manual_prompt", Valid: true}},
			},
		},
	}

	imp := NewImporter(users, projects, domains, files, links, gens)
	report, err := imp.Run(context.Background(), RunOptions{
		ManifestPath: manifest,
		ServerDir:    serverDir,
		Mode:         ModeApply,
		Force:        true,
		Batch:        BatchConfig{BatchSize: 50, BatchNumber: 1},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("expected 1 row")
	}
	if !containsAction(report.Rows[0].Actions, "legacy_artifacts_created") {
		t.Fatalf("expected create action, got %#v", report.Rows[0].Actions)
	}
	if len(gens.created) != 1 {
		t.Fatalf("expected one synthetic generation, got %d", len(gens.created))
	}
}

func TestImporterRunApplyKeepsLegacyArtifactsWhenHashUnchanged(t *testing.T) {
	dir := t.TempDir()
	domainDir := filepath.Join(dir, "example.com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "index.html"), []byte(`<html><body>Hello</body></html>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	artifacts, _, err := BuildLegacyArtifacts(domainDir, "example.com", "import_legacy")
	if err != nil {
		t.Fatalf("BuildLegacyArtifacts failed: %v", err)
	}
	artifactBytes, err := json.Marshal(artifacts)
	if err != nil {
		t.Fatalf("marshal artifacts: %v", err)
	}

	domainID := "dom-1"
	gens := &stubGenerations{
		byDomain: map[string][]sqlstore.Generation{
			domainID: {
				{
					ID:        "legacy-1",
					DomainID:  domainID,
					PromptID:  sql.NullString{String: LegacyDecodePromptID, Valid: true},
					Artifacts: artifactBytes,
				},
			},
		},
	}

	imp := NewImporter(&stubUsers{}, &stubProjects{}, &stubDomains{}, &stubSiteFiles{byDomainPath: map[string]map[string]sqlstore.SiteFile{}}, &stubLinks{}, gens)
	actions, _, err := imp.syncLegacyArtifacts(
		context.Background(),
		sqlstore.Domain{ID: domainID, URL: "example.com"},
		"owner@example.com",
		domainDir,
		"example.com",
		"import_legacy",
		ModeApply,
		false,
	)
	if err != nil {
		t.Fatalf("syncLegacyArtifacts failed: %v", err)
	}
	if !containsAction(actions, "legacy_artifacts_unchanged") {
		t.Fatalf("expected unchanged action, got %#v", actions)
	}
	if len(gens.created) != 0 {
		t.Fatalf("expected no create")
	}
	if len(gens.updated) != 0 {
		t.Fatalf("expected no update")
	}
}

func TestImporterRunDecodeOnlyApplyCreatesSyntheticGeneration(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.csv")
	serverDir := filepath.Join(dir, "server")
	if err := os.MkdirAll(filepath.Join(serverDir, "example.com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "example.com", "index.html"), []byte(`<html><body>decode</body></html>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	csvData := "project_name,owner_email,project_country,project_language,domain_url,main_keyword\n" +
		"proj,owner@example.com,se,sv,example.com,keyword\n"
	if err := os.WriteFile(manifest, []byte(csvData), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	projectID := "proj-1"
	domainID := "dom-1"
	users := &stubUsers{users: map[string]auth.User{"owner@example.com": {Email: "owner@example.com"}}}
	projects := &stubProjects{listByName: map[string][]sqlstore.Project{}}
	domains := &stubDomains{
		byURL: map[string]sqlstore.Domain{
			"example.com": {ID: domainID, ProjectID: projectID, URL: "example.com", Status: "published"},
		},
		byID: map[string]sqlstore.Domain{
			domainID: {ID: domainID, ProjectID: projectID, URL: "example.com", Status: "published"},
		},
	}
	files := &stubSiteFiles{byDomainPath: map[string]map[string]sqlstore.SiteFile{}}
	links := &stubLinks{}
	gens := &stubGenerations{byDomain: map[string][]sqlstore.Generation{}}

	imp := NewImporter(users, projects, domains, files, links, gens)
	report, err := imp.Run(context.Background(), RunOptions{
		ManifestPath: manifest,
		ServerDir:    serverDir,
		Mode:         ModeApply,
		DecodeOnly:   true,
		DecodeSource: "decode_backfill",
		Batch:        BatchConfig{BatchSize: 50, BatchNumber: 1},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("expected 1 row")
	}
	if !containsAction(report.Rows[0].Actions, "legacy_artifacts_created") {
		t.Fatalf("expected created action, got %#v", report.Rows[0].Actions)
	}
	if len(projects.created) != 0 {
		t.Fatalf("decode-only must not create projects")
	}
	if len(links.created) != 0 {
		t.Fatalf("decode-only must not create link tasks")
	}
	if len(gens.created) != 1 {
		t.Fatalf("expected one synthetic generation")
	}
}

func containsAction(actions []string, want string) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}
