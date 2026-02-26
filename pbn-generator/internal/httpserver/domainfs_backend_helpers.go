package httpserver

import (
	"context"
	"database/sql"
	"strings"

	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

func (s *Server) resolveContentBackend() domainfs.SiteContentBackend {
	if s.contentBackend != nil {
		return s.contentBackend
	}
	return domainfs.NewLocalFSBackend("server")
}

func makeDomainFSContext(domain sqlstore.Domain) domainfs.DomainFSContext {
	deploymentMode := "local_mock"
	if mode := strings.TrimSpace(nullableStringValue(domain.DeploymentMode)); mode != "" {
		deploymentMode = mode
	}
	return domainfs.DomainFSContext{
		DomainID:       strings.TrimSpace(domain.ID),
		DomainURL:      strings.TrimSpace(domain.URL),
		DeploymentMode: deploymentMode,
		PublishedPath:  strings.TrimSpace(nullableStringValue(domain.PublishedPath)),
		SiteOwner:      strings.TrimSpace(nullableStringValue(domain.SiteOwner)),
		ServerID:       strings.TrimSpace(nullableStringValue(domain.ServerID)),
	}
}

func nullableStringValue(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func (s *Server) readDomainFileBytesFromBackend(ctx context.Context, domain sqlstore.Domain, relPath string) ([]byte, error) {
	return s.resolveContentBackend().ReadFile(ctx, makeDomainFSContext(domain), relPath)
}

func (s *Server) writeDomainFileBytesToBackend(ctx context.Context, domain sqlstore.Domain, relPath string, content []byte) error {
	return s.resolveContentBackend().WriteFile(ctx, makeDomainFSContext(domain), relPath, content)
}

func (s *Server) ensureDomainDirInBackend(ctx context.Context, domain sqlstore.Domain, relPath string) error {
	return s.resolveContentBackend().EnsureDir(ctx, makeDomainFSContext(domain), relPath)
}

func (s *Server) statDomainPathInBackend(ctx context.Context, domain sqlstore.Domain, relPath string) (domainfs.FileInfo, error) {
	return s.resolveContentBackend().Stat(ctx, makeDomainFSContext(domain), relPath)
}

func (s *Server) readDomainDirInBackend(ctx context.Context, domain sqlstore.Domain, relPath string) ([]domainfs.EntryInfo, error) {
	return s.resolveContentBackend().ReadDir(ctx, makeDomainFSContext(domain), relPath)
}

func (s *Server) moveDomainPathInBackend(ctx context.Context, domain sqlstore.Domain, oldPath, newPath string) error {
	return s.resolveContentBackend().Move(ctx, makeDomainFSContext(domain), oldPath, newPath)
}

func (s *Server) deleteDomainPathInBackend(ctx context.Context, domain sqlstore.Domain, relPath string) error {
	return s.resolveContentBackend().Delete(ctx, makeDomainFSContext(domain), relPath)
}

func (s *Server) deleteDomainPathRecursiveInBackend(ctx context.Context, domain sqlstore.Domain, relPath string) error {
	return s.resolveContentBackend().DeleteAll(ctx, makeDomainFSContext(domain), relPath)
}
