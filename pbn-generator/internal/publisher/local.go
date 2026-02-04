package publisher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

// DomainStore описывает минимальный набор операций для обновления публикации.
type DomainStore interface {
	Get(ctx context.Context, id string) (sqlstore.Domain, error)
	UpdatePublishState(ctx context.Context, id, publishedPath string, fileCount int, totalSizeBytes int64) error
}

// LocalPublisher публикует файлы домена в локальную папку.
type LocalPublisher struct {
	baseDir string
	domains DomainStore
	files   SiteFileStore
}

// NewLocalPublisher создает LocalPublisher, который публикует файлы в baseDir.
func NewLocalPublisher(baseDir string, domains DomainStore, files SiteFileStore) *LocalPublisher {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "server"
	}
	return &LocalPublisher{baseDir: baseDir, domains: domains, files: files}
}

// Publish публикует файлы домена в локальную папку.
func (p *LocalPublisher) Publish(ctx context.Context, domainID string, files map[string][]byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if p.domains == nil {
		return fmt.Errorf("local publisher: domain store is nil")
	}
	baseDir, err := p.ensureBaseDir()
	if err != nil {
		return err
	}

	domain, err := p.domains.Get(ctx, domainID)
	if err != nil {
		return fmt.Errorf("local publisher: load domain: %w", err)
	}
	domainName, err := sanitizeDomain(domain.URL)
	if err != nil {
		return fmt.Errorf("local publisher: sanitize domain: %w", err)
	}

	targetDir, err := p.domainDir(baseDir, domainName)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("local publisher: clear target dir: %w", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("local publisher: create target dir: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("local publisher: no files provided")
	}

	var totalSize int64
	fileCount := 0
	for relPath, content := range files {
		clean, err := sanitizeFilePath(relPath)
		if err != nil {
			return fmt.Errorf("local publisher: invalid file path %q: %w", relPath, err)
		}
		fullPath := filepath.Join(targetDir, filepath.FromSlash(clean))
		if err := ensureWithinDir(targetDir, fullPath); err != nil {
			return fmt.Errorf("local publisher: invalid file path %q: %w", relPath, err)
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("local publisher: create dir for %s: %w", clean, err)
		}
		if err := os.WriteFile(fullPath, content, 0o644); err != nil {
			return fmt.Errorf("local publisher: write file %s: %w", clean, err)
		}
		totalSize += int64(len(content))
		fileCount++
	}

	if p.files != nil {
		syncedCount, syncedSize, err := SyncPublishedFiles(ctx, baseDir, domainName, domainID, p.files)
		if err != nil {
			return fmt.Errorf("local publisher: sync files: %w", err)
		}
		fileCount = syncedCount
		totalSize = syncedSize
	}

	publishedPath := buildPublishedPath(p.baseDir, domainName)
	if err := p.domains.UpdatePublishState(ctx, domainID, publishedPath, fileCount, totalSize); err != nil {
		return fmt.Errorf("local publisher: update publish state: %w", err)
	}

	return nil
}

// Unpublish удаляет опубликованные файлы домена.
func (p *LocalPublisher) Unpublish(ctx context.Context, domainID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if p.domains == nil {
		return fmt.Errorf("local publisher: domain store is nil")
	}
	baseDir, err := p.ensureBaseDir()
	if err != nil {
		return err
	}

	domain, err := p.domains.Get(ctx, domainID)
	if err != nil {
		return fmt.Errorf("local publisher: load domain: %w", err)
	}
	domainName, err := sanitizeDomain(domain.URL)
	if err != nil {
		return fmt.Errorf("local publisher: sanitize domain: %w", err)
	}

	targetDir, err := p.domainDir(baseDir, domainName)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("local publisher: remove target dir: %w", err)
	}
	if err := p.domains.UpdatePublishState(ctx, domainID, "", 0, 0); err != nil {
		return fmt.Errorf("local publisher: update publish state: %w", err)
	}
	return nil
}

// GetPublishedPath возвращает путь публикации для домена.
func (p *LocalPublisher) GetPublishedPath(domainID string) string {
	if p.domains == nil {
		return ""
	}
	domain, err := p.domains.Get(context.Background(), domainID)
	if err != nil {
		return ""
	}
	domainName, err := sanitizeDomain(domain.URL)
	if err != nil {
		return ""
	}
	return buildPublishedPath(p.baseDir, domainName)
}

func (p *LocalPublisher) ensureBaseDir() (string, error) {
	baseDir := p.baseDir
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "server"
	}
	info, err := os.Stat(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("local publisher: base dir %s does not exist", baseDir)
		}
		return "", fmt.Errorf("local publisher: stat base dir: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("local publisher: base dir %s is not a directory", baseDir)
	}
	return baseDir, nil
}

func (p *LocalPublisher) domainDir(baseDir, domain string) (string, error) {
	full := filepath.Join(baseDir, domain)
	if err := ensureWithinDir(baseDir, full); err != nil {
		return "", fmt.Errorf("local publisher: invalid domain path: %w", err)
	}
	return full, nil
}

func ensureWithinDir(baseDir, target string) error {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	baseAbs = filepath.Clean(baseAbs)
	if targetAbs == baseAbs {
		return errors.New("path equals base dir")
	}
	if !strings.HasPrefix(targetAbs, baseAbs+string(os.PathSeparator)) {
		return errors.New("path escapes base dir")
	}
	return nil
}

func sanitizeDomain(raw string) (string, error) {
	d := strings.TrimSpace(raw)
	if d == "" {
		return "", errors.New("domain is empty")
	}
	d = strings.TrimPrefix(d, "http://")
	d = strings.TrimPrefix(d, "https://")
	if idx := strings.IndexAny(d, "/?"); idx >= 0 {
		d = d[:idx]
	}
	if idx := strings.Index(d, ":"); idx >= 0 {
		d = d[:idx]
	}
	d = strings.TrimSuffix(d, "/")
	d = strings.TrimSpace(d)
	if d == "" {
		return "", errors.New("domain is empty")
	}
	if strings.Contains(d, "..") {
		return "", errors.New("domain contains invalid sequence")
	}
	if strings.ContainsAny(d, "\\") {
		return "", errors.New("domain contains invalid separator")
	}
	if strings.ContainsAny(d, " \t\n\r") {
		return "", errors.New("domain contains whitespace")
	}
	if !utf8.ValidString(d) {
		return "", errors.New("domain contains invalid utf8")
	}
	return d, nil
}

func sanitizeFilePath(p string) (string, error) {
	if strings.Contains(p, "\\") {
		return "", errors.New("path contains backslash")
	}
	clean := path.Clean(strings.TrimSpace(p))
	if clean == "." || clean == "" {
		return "", errors.New("path is empty")
	}
	if path.IsAbs(clean) {
		return "", errors.New("path is absolute")
	}
	if strings.HasPrefix(clean, "../") || clean == ".." {
		return "", errors.New("path traversal detected")
	}
	if strings.Contains(clean, "..") {
		parts := strings.Split(clean, "/")
		for _, part := range parts {
			if part == ".." {
				return "", errors.New("path traversal detected")
			}
		}
	}
	return clean, nil
}

func buildPublishedPath(baseDir, domain string) string {
	base := strings.TrimSpace(baseDir)
	if base == "" {
		base = "server"
	}
	return "/" + path.Join(path.Base(base), domain) + "/"
}
