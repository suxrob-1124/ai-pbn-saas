package publisher

import (
	"context"
	"database/sql"
	"fmt"
	"path"
	"strings"

	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// SSHPublisher публикует файлы домена в удаленный target через domainfs backend.
type SSHPublisher struct {
	backend domainfs.SiteContentBackend
	domains DomainStore
	files   SiteFileStore
}

func NewSSHPublisher(backend domainfs.SiteContentBackend, domains DomainStore, files SiteFileStore) *SSHPublisher {
	return &SSHPublisher{
		backend: backend,
		domains: domains,
		files:   files,
	}
}

func (p *SSHPublisher) Publish(ctx context.Context, domainID string, files map[string][]byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if p.backend == nil {
		return fmt.Errorf("ssh publisher: backend is nil")
	}
	if p.domains == nil {
		return fmt.Errorf("ssh publisher: domain store is nil")
	}
	if len(files) == 0 {
		return fmt.Errorf("ssh publisher: no files provided")
	}

	domain, err := p.domains.Get(ctx, domainID)
	if err != nil {
		return fmt.Errorf("ssh publisher: load domain: %w", err)
	}
	dctx, publishedPath, err := p.buildDomainContext(domain)
	if err != nil {
		return err
	}

	// Full replace target contents for this deployment.
	if err := p.backend.DeleteAll(ctx, dctx, ""); err != nil {
		return fmt.Errorf("ssh publisher: clear target dir: %w", err)
	}
	if err := p.backend.EnsureDir(ctx, dctx, ""); err != nil {
		return fmt.Errorf("ssh publisher: recreate target dir: %w", err)
	}

	var (
		fileCount int
		totalSize int64
	)
	for rawPath, content := range files {
		clean, err := sanitizeFilePath(rawPath)
		if err != nil {
			return fmt.Errorf("ssh publisher: invalid file path %q: %w", rawPath, err)
		}
		if err := p.backend.WriteFile(ctx, dctx, clean, content); err != nil {
			return fmt.Errorf("ssh publisher: write file %s: %w", clean, err)
		}
		fileCount++
		totalSize += int64(len(content))
	}

	if p.files != nil {
		syncedCount, syncedSize, err := SyncPublishedFilesFromMap(ctx, domainID, files, p.files)
		if err != nil {
			return fmt.Errorf("ssh publisher: sync files: %w", err)
		}
		fileCount = syncedCount
		totalSize = syncedSize
	}

	if err := p.domains.UpdatePublishState(ctx, domainID, publishedPath, fileCount, totalSize); err != nil {
		return fmt.Errorf("ssh publisher: update publish state: %w", err)
	}
	return nil
}

func (p *SSHPublisher) Unpublish(ctx context.Context, domainID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if p.backend == nil {
		return fmt.Errorf("ssh publisher: backend is nil")
	}
	if p.domains == nil {
		return fmt.Errorf("ssh publisher: domain store is nil")
	}
	domain, err := p.domains.Get(ctx, domainID)
	if err != nil {
		return fmt.Errorf("ssh publisher: load domain: %w", err)
	}
	dctx, _, err := p.buildDomainContext(domain)
	if err != nil {
		return err
	}
	if err := p.backend.DeleteAll(ctx, dctx, ""); err != nil {
		return fmt.Errorf("ssh publisher: clear target dir: %w", err)
	}
	if err := p.backend.EnsureDir(ctx, dctx, ""); err != nil {
		return fmt.Errorf("ssh publisher: recreate target dir: %w", err)
	}
	if err := p.domains.UpdatePublishState(ctx, domainID, "", 0, 0); err != nil {
		return fmt.Errorf("ssh publisher: update publish state: %w", err)
	}
	return nil
}

func (p *SSHPublisher) GetPublishedPath(domainID string) string {
	if p.domains == nil {
		return ""
	}
	domain, err := p.domains.Get(context.Background(), domainID)
	if err != nil {
		return ""
	}
	_, publishedPath, err := p.buildDomainContext(domain)
	if err != nil {
		return ""
	}
	return publishedPath
}

func (p *SSHPublisher) buildDomainContext(domain sqlstore.Domain) (domainfs.DomainFSContext, string, error) {
	deploymentMode := strings.ToLower(strings.TrimSpace(nullStringValue(domain.DeploymentMode)))
	if deploymentMode == "" {
		deploymentMode = "local_mock"
	}
	if deploymentMode != "ssh_remote" {
		return domainfs.DomainFSContext{}, "", fmt.Errorf("ssh publisher: domain %s is in deployment mode %s", domain.ID, deploymentMode)
	}
	publishedPath := strings.TrimSpace(nullStringValue(domain.PublishedPath))
	if publishedPath == "" {
		return domainfs.DomainFSContext{}, "", fmt.Errorf("ssh publisher: published_path is required for domain %s", domain.ID)
	}
	publishedPath = normalizePublishedPath(publishedPath)
	return domainfs.DomainFSContext{
		DomainID:       domain.ID,
		DomainURL:      domain.URL,
		DeploymentMode: deploymentMode,
		PublishedPath:  strings.TrimSuffix(publishedPath, "/"),
		SiteOwner:      strings.TrimSpace(nullStringValue(domain.SiteOwner)),
		ServerID:       strings.TrimSpace(nullStringValue(domain.ServerID)),
	}, publishedPath, nil
}

func normalizePublishedPath(raw string) string {
	clean := path.Clean("/" + strings.TrimSpace(raw))
	if clean == "/" {
		return "/"
	}
	return clean + "/"
}

func nullStringValue(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}
