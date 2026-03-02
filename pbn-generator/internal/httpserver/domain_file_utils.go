package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

func domainFilesDir(domainURL string) (string, error) {
	domain := strings.TrimSpace(domainURL)
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	if idx := strings.IndexAny(domain, "/?"); idx >= 0 {
		domain = domain[:idx]
	}
	if idx := strings.Index(domain, ":"); idx >= 0 {
		domain = domain[:idx]
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", errors.New("domain is empty")
	}
	if strings.Contains(domain, "..") || strings.Contains(domain, "\\") || strings.ContainsAny(domain, " \t\n\r") {
		return "", errors.New("invalid domain")
	}
	baseDir := "server"
	info, err := os.Stat(baseDir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", baseDir)
	}
	full := filepath.Join(baseDir, domain)
	if err := ensurePathWithin(baseDir, full); err != nil {
		return "", err
	}
	return full, nil
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

func detectMimeType(path string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}
	if len(content) > 0 {
		return http.DetectContentType(content)
	}
	return "application/octet-stream"
}

func baseMimeType(m string) string {
	return strings.TrimSpace(strings.SplitN(m, ";", 2)[0])
}

func (s *Server) listDomainDirs(ctx context.Context, domain sqlstore.Domain) ([]string, error) {
	items, err := s.resolveContentBackend().ListTree(ctx, makeDomainFSContext(domain), "")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, 16)
	for _, item := range items {
		if !item.IsDir {
			continue
		}
		rel := filepath.ToSlash(strings.TrimSpace(item.Path))
		if rel == "" || rel == "." {
			continue
		}
		clean, err := sanitizeFilePath(rel)
		if err != nil {
			continue
		}
		if err := validateEditorPath(clean); err != nil {
			continue
		}
		out = append(out, clean)
	}
	sort.Strings(out)
	return out, nil
}

func isEditableMimeType(mimeType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(baseMimeType(mimeType)))
	if strings.HasPrefix(normalized, "text/") {
		return true
	}
	switch normalized {
	case "application/json", "application/javascript", "application/xml", "image/svg+xml":
		return true
	default:
		return false
	}
}

func isBinaryMimeType(mimeType string) bool {
	return !isEditableMimeType(mimeType)
}

func buildRevision(file *sqlstore.SiteFile, content []byte, source, editedBy, description string) sqlstore.FileRevision {
	hash := sha256.Sum256(content)
	rev := sqlstore.FileRevision{
		ID:          uuid.NewString(),
		FileID:      "",
		Version:     1,
		Content:     content,
		ContentHash: hex.EncodeToString(hash[:]),
		SizeBytes:   int64(len(content)),
		MimeType:    detectMimeType("", content),
		Source:      normalizeRevisionSource(source),
		EditedBy:    editedBy,
	}
	if file != nil {
		rev.FileID = file.ID
		if file.Version > 0 {
			rev.Version = file.Version
		}
		if strings.TrimSpace(file.MimeType) != "" {
			rev.MimeType = file.MimeType
		}
	}
	if strings.TrimSpace(description) != "" {
		rev.Description = sqlstore.NullableString(strings.TrimSpace(description))
	}
	return rev
}

func (s *Server) readDomainFileContent(ctx context.Context, domain sqlstore.Domain, relPath string) (string, string, error) {
	file, err := s.siteFiles.GetByPath(ctx, domain.ID, relPath)
	if err != nil {
		return "", "", err
	}
	content, err := s.readDomainFileBytesFromBackend(ctx, domain, relPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", "", sql.ErrNoRows
		}
		return "", "", err
	}
	mimeType := file.MimeType
	if mimeType == "" {
		mimeType = detectMimeType(relPath, content)
	}
	return string(content), mimeType, nil
}

func detectImageDimensions(domain sqlstore.Domain, relPath string) (int, int, bool) {
	domainDir, err := domainFilesDir(domain.URL)
	if err != nil {
		return 0, 0, false
	}
	fullPath := filepath.Join(domainDir, filepath.FromSlash(relPath))
	if err := ensurePathWithin(domainDir, fullPath); err != nil {
		return 0, 0, false
	}
	content, err := os.ReadFile(fullPath)
	if err != nil || len(content) == 0 {
		return 0, 0, false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

func (s *Server) readDomainFileBytes(ctx context.Context, domain sqlstore.Domain, relPath string) ([]byte, error) {
	content, err := s.readDomainFileBytesFromBackend(ctx, domain, relPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return content, nil
}
