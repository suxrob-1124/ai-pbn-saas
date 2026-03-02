package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/importer/legacy"
	"obzornik-pbn-generator/internal/inventory"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

func normalizeSource(raw string, deployMode string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	switch trimmed {
	case "local", "remote":
		return trimmed
	case "auto", "":
		if strings.EqualFold(strings.TrimSpace(deployMode), "ssh_remote") {
			return "remote"
		}
		return "local"
	default:
		return "local"
	}
}

func prepareRemoteMirror(
	ctx context.Context,
	cfg config.Config,
	manifestPath string,
	batchSize int,
	batchNumber int,
	fallbackTarget string,
) (map[string]inventory.ProbeResult, string, func() error, error) {
	rows, err := legacy.ParseManifestCSV(manifestPath)
	if err != nil {
		return nil, "", nil, err
	}
	selectedRows, err := selectBatch(rows, batchSize, batchNumber)
	if err != nil {
		return nil, "", nil, err
	}

	targets, err := domainfs.ParseDeployTargetsJSON(cfg.DeployTargetsJSON)
	if err != nil {
		return nil, "", nil, err
	}
	for alias, target := range targets {
		if strings.TrimSpace(target.KeyPath) == "" {
			target.KeyPath = strings.TrimSpace(cfg.DeploySSHKeyPath)
			targets[alias] = target
		}
	}
	if len(targets) == 0 {
		return nil, "", nil, fmt.Errorf("DEPLOY_TARGETS_JSON is empty for remote source")
	}

	pool, err := domainfs.NewSSHPool(domainfs.SSHPoolConfig{
		MaxOpen:        cfg.DeploySSHPoolMaxOpen,
		MaxIdle:        cfg.DeploySSHPoolMaxIdle,
		IdleTTL:        cfg.DeploySSHPoolIdleTTL,
		DialTimeout:    cfg.DeployTimeout,
		KnownHostsPath: cfg.DeployKnownHostsPath,
	})
	if err != nil {
		return nil, "", nil, err
	}
	sshBackend, err := domainfs.NewSSHBackend(pool, targets)
	if err != nil {
		_ = pool.Close()
		return nil, "", nil, err
	}

	tmpRoot, err := os.MkdirTemp("", "legacy-import-remote-*")
	if err != nil {
		_ = pool.Close()
		return nil, "", nil, err
	}
	cleanup := func() error {
		_ = pool.Close()
		return os.RemoveAll(tmpRoot)
	}

	prober := inventory.SSHProber{
		Timeout:        cfg.DeployTimeout,
		KnownHostsPath: cfg.DeployKnownHostsPath,
	}

	remoteMeta := make(map[string]inventory.ProbeResult, len(selectedRows))
	for _, row := range selectedRows {
		domainHost, err := normalizeDomainHost(row.DomainURL)
		if err != nil {
			return nil, "", cleanup, fmt.Errorf("invalid domain_url %q: %w", row.DomainURL, err)
		}

		alias := strings.TrimSpace(row.ServerID)
		if alias == "" {
			alias = strings.TrimSpace(fallbackTarget)
		}
		if alias == "" {
			return nil, "", cleanup, fmt.Errorf("server_id is empty for domain %s and --target not provided", domainHost)
		}

		tgt, ok := targets[alias]
		if !ok {
			return nil, "", cleanup, fmt.Errorf("target alias %q not found in DEPLOY_TARGETS_JSON", alias)
		}
		if strings.TrimSpace(tgt.KeyPath) == "" {
			return nil, "", cleanup, fmt.Errorf("target %q has empty ssh key path", alias)
		}

		probeResult, err := prober.Probe(ctx, inventory.Target{
			Alias:   alias,
			Host:    tgt.Host,
			Port:    tgt.Port,
			User:    tgt.User,
			KeyPath: tgt.KeyPath,
		}, domainHost)
		if err != nil {
			return nil, "", cleanup, fmt.Errorf("probe %s: %w", domainHost, err)
		}
		if probeResult.Status != inventory.ProbeFound {
			return nil, "", cleanup, fmt.Errorf("probe %s returned status=%s", domainHost, probeResult.Status)
		}

		if err := mirrorRemoteDomain(ctx, sshBackend, alias, domainHost, probeResult.PublishedPath, probeResult.SiteOwner, tmpRoot); err != nil {
			return nil, "", cleanup, fmt.Errorf("mirror %s: %w", domainHost, err)
		}
		remoteMeta[domainHost] = probeResult
	}

	return remoteMeta, tmpRoot, cleanup, nil
}

func mirrorRemoteDomain(
	ctx context.Context,
	backend *domainfs.SSHBackend,
	serverAlias string,
	domainHost string,
	publishedPath string,
	siteOwner string,
	localRoot string,
) error {
	dctx := domainfs.DomainFSContext{
		DomainURL:      domainHost,
		DeploymentMode: "ssh_remote",
		PublishedPath:  strings.TrimSpace(publishedPath),
		SiteOwner:      strings.TrimSpace(siteOwner),
		ServerID:       strings.TrimSpace(serverAlias),
	}
	tree, err := backend.ListTree(ctx, dctx, "")
	if err != nil {
		return err
	}
	domainDir := filepath.Join(localRoot, domainHost)
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		return err
	}
	fileCount := 0
	for _, item := range tree {
		if item.IsDir {
			continue
		}
		cleanRel := filepath.Clean(filepath.FromSlash(item.Path))
		if cleanRel == "." || strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
			return fmt.Errorf("unsafe remote path %q", item.Path)
		}
		content, err := backend.ReadFile(ctx, dctx, item.Path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(domainDir, cleanRel)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			return err
		}
		fileCount++
	}
	if fileCount == 0 {
		return fmt.Errorf("no files mirrored for %s", domainHost)
	}
	return nil
}

func applyRemoteInventory(
	ctx context.Context,
	domains *sqlstore.DomainStore,
	report legacy.Report,
	remoteMeta map[string]inventory.ProbeResult,
) (int, []error) {
	updated := 0
	errs := make([]error, 0)
	for _, row := range report.Rows {
		host, err := normalizeDomainHost(row.DomainURL)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		meta, ok := remoteMeta[host]
		if !ok {
			continue
		}
		domain, err := domains.GetByURL(ctx, host)
		if err != nil {
			errs = append(errs, fmt.Errorf("find domain %s: %w", host, err))
			continue
		}
		if err := domains.UpdateInventoryState(
			ctx,
			domain.ID,
			sqlstore.NullableString(meta.PublishedPath),
			sqlstore.NullableString(meta.SiteOwner),
			sqlstore.NullableString(inventoryStatusFromProbe(meta.Status)),
			sql.NullString{},
			time.Now().UTC(),
		); err != nil {
			errs = append(errs, fmt.Errorf("update inventory %s: %w", host, err))
			continue
		}
		if err := domains.UpdateDeploymentMode(ctx, domain.ID, "ssh_remote"); err != nil {
			errs = append(errs, fmt.Errorf("update deployment mode %s: %w", host, err))
			continue
		}
		updated++
	}
	return updated, errs
}

func inventoryStatusFromProbe(status inventory.ProbeStatus) string {
	switch status {
	case inventory.ProbeFound:
		return "ok"
	case inventory.ProbeAmbiguous:
		return "partial"
	default:
		return "failed"
	}
}

func selectBatch(rows []legacy.ManifestRow, batchSize, batchNumber int) ([]legacy.ManifestRow, error) {
	start := (batchNumber - 1) * batchSize
	if start >= len(rows) {
		return nil, fmt.Errorf("batch out of range: start=%d total=%d", start, len(rows))
	}
	end := start + batchSize
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end], nil
}

func normalizeDomainHost(raw string) (string, error) {
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
