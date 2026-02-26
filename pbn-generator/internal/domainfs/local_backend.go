package domainfs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type LocalFSBackend struct {
	baseDir string
}

func NewLocalFSBackend(baseDir string) *LocalFSBackend {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = "server"
	}
	return &LocalFSBackend{baseDir: baseDir}
}

func (b *LocalFSBackend) ReadFile(_ context.Context, dctx DomainFSContext, filePath string) ([]byte, error) {
	fullPath, err := b.resolveFullPath(dctx, filePath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fullPath)
}

func (b *LocalFSBackend) WriteFile(_ context.Context, dctx DomainFSContext, filePath string, content []byte) error {
	fullPath, err := b.resolveFullPath(dctx, filePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, content, 0o644)
}

func (b *LocalFSBackend) EnsureDir(_ context.Context, dctx DomainFSContext, dirPath string) error {
	fullPath, err := b.resolveFullPath(dctx, dirPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(fullPath, 0o755)
}

func (b *LocalFSBackend) Stat(_ context.Context, dctx DomainFSContext, relPath string) (FileInfo, error) {
	fullPath, err := b.resolveFullPath(dctx, relPath)
	if err != nil {
		return FileInfo{}, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return FileInfo{}, err
	}
	cleanRel, err := normalizeRelativePath(relPath)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Path:    cleanRel,
		Name:    info.Name(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime(),
	}, nil
}

func (b *LocalFSBackend) ReadDir(_ context.Context, dctx DomainFSContext, dirPath string) ([]EntryInfo, error) {
	fullPath, err := b.resolveFullPath(dctx, dirPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}
	out := make([]EntryInfo, 0, len(entries))
	for _, entry := range entries {
		out = append(out, EntryInfo{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
		})
	}
	return out, nil
}

func (b *LocalFSBackend) Move(_ context.Context, dctx DomainFSContext, oldPath, newPath string) error {
	oldFull, err := b.resolveFullPath(dctx, oldPath)
	if err != nil {
		return err
	}
	newFull, err := b.resolveFullPath(dctx, newPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newFull), 0o755); err != nil {
		return err
	}
	return os.Rename(oldFull, newFull)
}

func (b *LocalFSBackend) Delete(_ context.Context, dctx DomainFSContext, relPath string) error {
	fullPath, err := b.resolveFullPath(dctx, relPath)
	if err != nil {
		return err
	}
	return os.Remove(fullPath)
}

func (b *LocalFSBackend) DeleteAll(_ context.Context, dctx DomainFSContext, relPath string) error {
	fullPath, err := b.resolveFullPath(dctx, relPath)
	if err != nil {
		return err
	}
	return os.RemoveAll(fullPath)
}

func (b *LocalFSBackend) ListTree(_ context.Context, dctx DomainFSContext, dirPath string) ([]FileInfo, error) {
	rootPath, err := b.resolveDomainRoot(dctx)
	if err != nil {
		return nil, err
	}
	fullPath, err := b.resolveFullPath(dctx, dirPath)
	if err != nil {
		return nil, err
	}
	out := make([]FileInfo, 0, 32)
	err = filepath.WalkDir(fullPath, func(curr string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if curr == fullPath {
			return nil
		}
		rel, err := filepath.Rel(rootPath, curr)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(strings.TrimSpace(rel))
		rel = strings.TrimPrefix(rel, "./")
		if rel == "" || rel == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out = append(out, FileInfo{
			Path:    rel,
			Name:    d.Name(),
			Size:    info.Size(),
			IsDir:   d.IsDir(),
			ModTime: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Name < out[j].Name
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func (b *LocalFSBackend) resolveDomainRoot(dctx DomainFSContext) (string, error) {
	host, err := normalizeDomainHost(dctx.DomainURL)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(b.baseDir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", b.baseDir)
	}
	full := filepath.Join(b.baseDir, host)
	if err := ensurePathWithin(b.baseDir, full); err != nil {
		return "", err
	}
	return full, nil
}

func (b *LocalFSBackend) resolveFullPath(dctx DomainFSContext, relPath string) (string, error) {
	rootPath, err := b.resolveDomainRoot(dctx)
	if err != nil {
		return "", err
	}
	cleanPath, err := normalizeRelativePath(relPath)
	if err != nil {
		return "", err
	}
	if cleanPath == "" {
		return rootPath, nil
	}
	full := filepath.Join(rootPath, filepath.FromSlash(cleanPath))
	if err := ensurePathWithin(rootPath, full); err != nil {
		return "", err
	}
	return full, nil
}

func normalizeRelativePath(relPath string) (string, error) {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return "", nil
	}
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	if strings.HasPrefix(relPath, "/") {
		return "", errors.New("absolute paths are not allowed")
	}
	clean := path.Clean("/" + relPath)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." {
		return "", nil
	}
	if clean == "" || strings.HasPrefix(clean, "..") {
		return "", errors.New("path escapes base dir")
	}
	return clean, nil
}

func ensurePathWithin(baseDir, target string) error {
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

func normalizeDomainHost(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("domain is empty")
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", errors.New("domain is empty")
	}
	if strings.Contains(host, "..") || strings.Contains(host, "\\") || strings.ContainsAny(host, " \t\n\r") {
		return "", errors.New("invalid domain")
	}
	return host, nil
}
