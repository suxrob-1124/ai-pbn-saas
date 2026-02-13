package legacy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/publisher"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

type userStore interface {
	Get(ctx context.Context, email string) (auth.User, error)
}

type projectStore interface {
	Create(ctx context.Context, p sqlstore.Project) error
	Update(ctx context.Context, p sqlstore.Project) error
	ListByNameExact(ctx context.Context, name string) ([]sqlstore.Project, error)
}

type domainStore interface {
	GetByURL(ctx context.Context, url string) (sqlstore.Domain, error)
	Create(ctx context.Context, d sqlstore.Domain) error
	UpdateProject(ctx context.Context, id, projectID string) error
	UpdateKeyword(ctx context.Context, id, keyword string) error
	UpdateExtras(ctx context.Context, id, country, language string, exclude, server sql.NullString) (bool, error)
	UpdateStatus(ctx context.Context, id, status string) error
	UpdatePublishState(ctx context.Context, id, publishedPath string, fileCount int, totalSizeBytes int64) error
	UpdateLinkSettings(ctx context.Context, id string, anchorText, acceptorURL sql.NullString) (bool, error)
	UpdateLinkState(ctx context.Context, id string, status string, lastTaskID string, filePath string, anchorSnapshot string) error
	EnsureDefaultServer(ctx context.Context, email string) error
}

type linkTaskStore interface {
	Create(ctx context.Context, task sqlstore.LinkTask) error
}

// Importer выполняет импорт legacy-сайтов из server/* в БД.
type Importer struct {
	users    userStore
	projects projectStore
	domains  domainStore
	files    publisher.SiteFileStore
	links    linkTaskStore
}

// NewImporter создает сервис импорта.
func NewImporter(users userStore, projects projectStore, domains domainStore, files publisher.SiteFileStore, links linkTaskStore) *Importer {
	return &Importer{
		users:    users,
		projects: projects,
		domains:  domains,
		files:    files,
		links:    links,
	}
}

// Run запускает импорт по манифесту.
func (i *Importer) Run(ctx context.Context, opts RunOptions) (Report, error) {
	if err := validateRunOptions(opts); err != nil {
		return Report{}, err
	}

	allRows, err := ParseManifestCSV(opts.ManifestPath)
	if err != nil {
		return Report{}, err
	}
	rows, err := selectBatch(allRows, opts.Batch)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		Mode:         opts.Mode,
		ManifestPath: opts.ManifestPath,
		ServerDir:    opts.ServerDir,
		BatchSize:    opts.Batch.BatchSize,
		BatchNumber:  opts.Batch.BatchNumber,
		StartedAt:    time.Now().UTC(),
		Rows:         make([]RowReport, 0, len(rows)),
	}

	for _, row := range rows {
		res := i.processRow(ctx, row, opts)
		report.Rows = append(report.Rows, res)
		report.Summary.Processed++
		switch res.Status {
		case "success":
			report.Summary.Success++
		case "warned":
			report.Summary.Warned++
		case "failed":
			report.Summary.Failed++
		}
	}
	report.FinishedAt = time.Now().UTC()

	return report, nil
}

func (i *Importer) processRow(ctx context.Context, row ManifestRow, opts RunOptions) RowReport {
	res := RowReport{
		RowNumber:   row.RowNumber,
		ProjectName: strings.TrimSpace(row.ProjectName),
		OwnerEmail:  strings.TrimSpace(row.OwnerEmail),
		DomainURL:   strings.TrimSpace(row.DomainURL),
		Status:      "success",
	}
	fail := func(err error) RowReport {
		res.Status = "failed"
		res.Error = err.Error()
		return res
	}
	warn := func(msg string) {
		res.Warnings = append(res.Warnings, msg)
		if res.Status != "failed" {
			res.Status = "warned"
		}
	}

	if err := validateManifestRow(row); err != nil {
		return fail(err)
	}

	owner := strings.TrimSpace(row.OwnerEmail)
	if _, err := i.users.Get(ctx, owner); err != nil {
		return fail(fmt.Errorf("owner_email not found: %s", owner))
	}

	normalizedDomain, err := normalizeDomain(row.DomainURL)
	if err != nil {
		return fail(fmt.Errorf("domain_url is invalid: %w", err))
	}
	res.DomainURL = normalizedDomain

	project, projectActions, err := i.resolveProject(ctx, row, opts.Mode)
	if err != nil {
		return fail(err)
	}
	res.Actions = append(res.Actions, projectActions...)

	domain, domainExisted, domainActions, err := i.resolveDomain(ctx, row, project, normalizedDomain, opts.Mode)
	if err != nil {
		return fail(err)
	}
	res.Actions = append(res.Actions, domainActions...)

	domainDir, err := resolveDomainDir(opts.ServerDir, normalizedDomain)
	if err != nil {
		return fail(err)
	}

	fileCount, totalSize, err := i.syncDomainFiles(ctx, opts.Mode, domain.ID, opts.ServerDir, normalizedDomain, domainDir)
	if err != nil {
		return fail(err)
	}
	res.FileCount = fileCount
	res.TotalSizeBytes = totalSize
	if opts.Mode == ModeApply {
		res.Actions = append(res.Actions, "files_synced", "domain_published")
	} else {
		res.Actions = append(res.Actions, "files_sync_preview", "domain_publish_preview")
	}

	indexPath := filepath.Join(domainDir, "index.html")
	decoded, err := DecodePrimaryHTTPSLinkFromFile(indexPath, normalizedDomain)
	if err != nil {
		return fail(err)
	}
	if decoded == nil {
		if domainExisted {
			warn("external https link not found in body/index.html; existing link fields are preserved")
		} else {
			warn("external https link not found in body/index.html")
		}
		return res
	}

	if opts.Mode == ModeApply {
		if shouldSkipLinkBaseline(domain, decoded) {
			res.Actions = append(res.Actions, "link_baseline_skipped")
			return res
		}
		if _, err := i.domains.UpdateLinkSettings(ctx, domain.ID,
			sqlstore.NullableString(decoded.AnchorText),
			sqlstore.NullableString(decoded.TargetURL)); err != nil {
			return fail(fmt.Errorf("update link settings: %w", err))
		}
		now := time.Now().UTC()
		taskID := uuid.NewString()
		task := sqlstore.LinkTask{
			ID:            taskID,
			DomainID:      domain.ID,
			AnchorText:    decoded.AnchorText,
			TargetURL:     decoded.TargetURL,
			ScheduledFor:  now,
			Action:        "insert",
			Status:        "inserted",
			FoundLocation: sql.NullString{String: decoded.FoundLocation, Valid: true},
			Attempts:      0,
			CreatedBy:     owner,
			CreatedAt:     now,
			CompletedAt:   sql.NullTime{Time: now, Valid: true},
		}
		if err := i.links.Create(ctx, task); err != nil {
			return fail(fmt.Errorf("create synthetic link task: %w", err))
		}
		if err := i.domains.UpdateLinkState(ctx, domain.ID, "inserted", taskID, decoded.FoundPath, decoded.AnchorText); err != nil {
			return fail(fmt.Errorf("update link state: %w", err))
		}
		res.Actions = append(res.Actions, "link_decoded", "link_baseline_created")
		return res
	}

	res.Actions = append(res.Actions, "link_decode_preview")
	return res
}

func (i *Importer) resolveProject(ctx context.Context, row ManifestRow, mode Mode) (sqlstore.Project, []string, error) {
	name := strings.TrimSpace(row.ProjectName)
	owner := strings.TrimSpace(row.OwnerEmail)
	country := strings.TrimSpace(row.ProjectCountry)
	lang := strings.TrimSpace(row.ProjectLanguage)
	actions := []string{}

	projects, err := i.projects.ListByNameExact(ctx, name)
	if err != nil {
		return sqlstore.Project{}, nil, fmt.Errorf("find project by name: %w", err)
	}
	if len(projects) > 1 {
		return sqlstore.Project{}, nil, fmt.Errorf("multiple projects found with name %q", name)
	}
	if len(projects) == 0 {
		p := sqlstore.Project{
			ID:             uuid.NewString(),
			UserEmail:      owner,
			Name:           name,
			TargetCountry:  country,
			TargetLanguage: lang,
			Status:         "draft",
		}
		if mode == ModeApply {
			if err := i.projects.Create(ctx, p); err != nil {
				return sqlstore.Project{}, nil, fmt.Errorf("create project: %w", err)
			}
			actions = append(actions, "project_created")
		} else {
			actions = append(actions, "project_would_create")
		}
		return p, actions, nil
	}

	p := projects[0]
	if !strings.EqualFold(strings.TrimSpace(p.UserEmail), owner) {
		return sqlstore.Project{}, nil, fmt.Errorf("owner mismatch for project %q", name)
	}
	needsUpdate := strings.TrimSpace(p.TargetCountry) != country || strings.TrimSpace(p.TargetLanguage) != lang
	if needsUpdate {
		p.TargetCountry = country
		p.TargetLanguage = lang
		if mode == ModeApply {
			if err := i.projects.Update(ctx, p); err != nil {
				return sqlstore.Project{}, nil, fmt.Errorf("update project fields: %w", err)
			}
			actions = append(actions, "project_updated")
		} else {
			actions = append(actions, "project_would_update")
		}
	}
	if len(actions) == 0 {
		actions = append(actions, "project_reused")
	}
	return p, actions, nil
}

func (i *Importer) resolveDomain(ctx context.Context, row ManifestRow, project sqlstore.Project, normalizedDomain string, mode Mode) (sqlstore.Domain, bool, []string, error) {
	actions := []string{}
	existing, err := i.domains.GetByURL(ctx, normalizedDomain)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return sqlstore.Domain{}, false, nil, fmt.Errorf("get domain by url: %w", err)
	}

	country := strings.TrimSpace(row.ProjectCountry)
	lang := strings.TrimSpace(row.ProjectLanguage)
	exclude := sql.NullString{}
	if v := strings.TrimSpace(row.ExcludeDomains); v != "" {
		exclude = sqlstore.NullableString(v)
	}

	if errors.Is(err, sql.ErrNoRows) {
		serverID := strings.TrimSpace(row.ServerID)
		if serverID == "" {
			serverID = sqlstore.DefaultServerID
		}
		d := sqlstore.Domain{
			ID:             uuid.NewString(),
			ProjectID:      project.ID,
			URL:            normalizedDomain,
			MainKeyword:    strings.TrimSpace(row.MainKeyword),
			TargetCountry:  country,
			TargetLanguage: lang,
			ExcludeDomains: exclude,
			ServerID:       sqlstore.NullableString(serverID),
			Status:         "waiting",
		}
		if mode == ModeApply {
			if err := i.domains.EnsureDefaultServer(ctx, project.UserEmail); err != nil {
				return sqlstore.Domain{}, false, nil, fmt.Errorf("ensure default server: %w", err)
			}
			if err := i.domains.Create(ctx, d); err != nil {
				return sqlstore.Domain{}, false, nil, fmt.Errorf("create domain: %w", err)
			}
			actions = append(actions, "domain_created")
		} else {
			actions = append(actions, "domain_would_create")
		}
		return d, false, actions, nil
	}

	d := existing
	if d.ProjectID != project.ID {
		if mode == ModeApply {
			if err := i.domains.UpdateProject(ctx, d.ID, project.ID); err != nil {
				return sqlstore.Domain{}, true, nil, fmt.Errorf("move domain to target project: %w", err)
			}
			actions = append(actions, "domain_moved")
		} else {
			actions = append(actions, "domain_would_move")
		}
		d.ProjectID = project.ID
	}

	newKeyword := strings.TrimSpace(row.MainKeyword)
	if strings.TrimSpace(d.MainKeyword) != newKeyword {
		if mode == ModeApply {
			if err := i.domains.UpdateKeyword(ctx, d.ID, newKeyword); err != nil {
				return sqlstore.Domain{}, true, nil, fmt.Errorf("update domain keyword: %w", err)
			}
			actions = append(actions, "domain_keyword_updated")
		} else {
			actions = append(actions, "domain_keyword_preview")
		}
		d.MainKeyword = newKeyword
	}

	server := d.ServerID
	if v := strings.TrimSpace(row.ServerID); v != "" {
		server = sqlstore.NullableString(v)
	} else if !server.Valid {
		server = sqlstore.NullableString(sqlstore.DefaultServerID)
	}
	extrasChanged := strings.TrimSpace(d.TargetCountry) != country || strings.TrimSpace(d.TargetLanguage) != lang ||
		d.ExcludeDomains.Valid != exclude.Valid || strings.TrimSpace(d.ExcludeDomains.String) != strings.TrimSpace(exclude.String) ||
		d.ServerID.Valid != server.Valid || strings.TrimSpace(d.ServerID.String) != strings.TrimSpace(server.String)
	if extrasChanged {
		if mode == ModeApply {
			if err := i.domains.EnsureDefaultServer(ctx, project.UserEmail); err != nil {
				return sqlstore.Domain{}, true, nil, fmt.Errorf("ensure default server: %w", err)
			}
			if _, err := i.domains.UpdateExtras(ctx, d.ID, country, lang, exclude, server); err != nil {
				return sqlstore.Domain{}, true, nil, fmt.Errorf("update domain extras: %w", err)
			}
			actions = append(actions, "domain_extras_updated")
		} else {
			actions = append(actions, "domain_extras_preview")
		}
		d.TargetCountry = country
		d.TargetLanguage = lang
		d.ExcludeDomains = exclude
		d.ServerID = server
	}

	if len(actions) == 0 {
		actions = append(actions, "domain_reused")
	}
	return d, true, actions, nil
}

func (i *Importer) syncDomainFiles(ctx context.Context, mode Mode, domainID, serverDir, domainURL, domainDir string) (int, int64, error) {
	domainName, err := normalizeDomain(domainURL)
	if err != nil {
		return 0, 0, err
	}
	if mode == ModeDryRun {
		count, size, err := scanDomainFiles(domainDir)
		if err != nil {
			return 0, 0, err
		}
		return count, size, nil
	}

	count, size, err := publisher.SyncPublishedFiles(ctx, serverDir, domainName, domainID, i.files)
	if err != nil {
		return 0, 0, fmt.Errorf("sync files: %w", err)
	}
	if err := i.domains.UpdatePublishState(ctx, domainID, buildPublishedPath(domainName), count, size); err != nil {
		return 0, 0, fmt.Errorf("update publish state: %w", err)
	}
	if err := i.domains.UpdateStatus(ctx, domainID, "published"); err != nil {
		return 0, 0, fmt.Errorf("update domain status: %w", err)
	}
	return count, size, nil
}

func shouldSkipLinkBaseline(domain sqlstore.Domain, decoded *DecodedLink) bool {
	if decoded == nil {
		return true
	}
	if !domain.LinkAnchorText.Valid || !domain.LinkAcceptorURL.Valid || !domain.LinkStatus.Valid || !domain.LinkLastTaskID.Valid {
		return false
	}
	anchor := strings.TrimSpace(domain.LinkAnchorText.String)
	target := strings.TrimSpace(domain.LinkAcceptorURL.String)
	status := strings.ToLower(strings.TrimSpace(domain.LinkStatus.String))
	lastTaskID := strings.TrimSpace(domain.LinkLastTaskID.String)
	return anchor == strings.TrimSpace(decoded.AnchorText) && target == strings.TrimSpace(decoded.TargetURL) && status == "inserted" && lastTaskID != ""
}

func validateRunOptions(opts RunOptions) error {
	if strings.TrimSpace(opts.ManifestPath) == "" {
		return fmt.Errorf("--manifest is required")
	}
	if strings.TrimSpace(opts.ServerDir) == "" {
		return fmt.Errorf("--server-dir must not be empty")
	}
	if opts.Mode != ModeDryRun && opts.Mode != ModeApply {
		return fmt.Errorf("--mode must be dry-run or apply")
	}
	if opts.Batch.BatchSize <= 0 {
		return fmt.Errorf("--batch-size must be > 0")
	}
	if opts.Batch.BatchNumber <= 0 {
		return fmt.Errorf("--batch-number must be > 0")
	}
	return nil
}

func selectBatch(rows []ManifestRow, batch BatchConfig) ([]ManifestRow, error) {
	start := (batch.BatchNumber - 1) * batch.BatchSize
	if start >= len(rows) {
		return nil, fmt.Errorf("batch out of range: start=%d total=%d", start, len(rows))
	}
	end := start + batch.BatchSize
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end], nil
}

func normalizeDomain(raw string) (string, error) {
	d := strings.TrimSpace(raw)
	if d == "" {
		return "", fmt.Errorf("domain is empty")
	}
	d = strings.TrimPrefix(d, "http://")
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "//")
	if idx := strings.IndexAny(d, "/?"); idx >= 0 {
		d = d[:idx]
	}
	if idx := strings.Index(d, ":"); idx >= 0 {
		d = d[:idx]
	}
	d = strings.TrimSpace(strings.TrimSuffix(d, "/"))
	if d == "" {
		return "", fmt.Errorf("domain is empty")
	}
	if strings.Contains(d, "..") || strings.Contains(d, "\\") || strings.ContainsAny(d, " \t\n\r") {
		return "", fmt.Errorf("domain is invalid")
	}
	return d, nil
}

func resolveDomainDir(serverDir, domainURL string) (string, error) {
	domain, err := normalizeDomain(domainURL)
	if err != nil {
		return "", err
	}
	baseInfo, err := os.Stat(serverDir)
	if err != nil {
		return "", fmt.Errorf("stat server-dir: %w", err)
	}
	if !baseInfo.IsDir() {
		return "", fmt.Errorf("server-dir is not a directory")
	}
	full := filepath.Join(serverDir, domain)
	if err := ensureWithin(serverDir, full); err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", fmt.Errorf("domain directory not found: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("domain path is not a directory")
	}
	return full, nil
}

func ensureWithin(baseDir, target string) error {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	baseAbs = filepath.Clean(baseAbs)
	targetAbs = filepath.Clean(targetAbs)
	if !strings.HasPrefix(targetAbs, baseAbs+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes server-dir")
	}
	return nil
}

func scanDomainFiles(root string) (int, int64, error) {
	count := 0
	var total int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		count++
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("scan domain files: %w", err)
	}
	if count == 0 {
		return 0, 0, fmt.Errorf("domain has no files")
	}
	return count, total, nil
}

func buildPublishedPath(domain string) string {
	return "/server/" + strings.Trim(domain, "/") + "/"
}
