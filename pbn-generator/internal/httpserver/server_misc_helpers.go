package httpserver

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"

	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

func isGenerationPromptStage(stage string) bool {
	stage = strings.TrimSpace(stage)
	for _, candidate := range sqlstore.GenerationPromptStages {
		if stage == candidate {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Server) SetContentBackend(backend domainfs.SiteContentBackend) {
	if backend == nil {
		return
	}
	s.contentBackend = backend
}

func (s *Server) SetDomainFilesRedisCache(client *redis.Client) {
	s.domainFilesCache = newRedisDomainFilesCache(client)
}

func stableDomainStatusFromDomain(domain sqlstore.Domain) string {
	if (domain.PublishedAt.Valid || strings.TrimSpace(domain.PublishedPath.String) != "") && domain.FileCount > 0 {
		return "published"
	}
	return "waiting"
}

func (s *Server) buildIndexCheckResponse(ctx context.Context, list []sqlstore.IndexCheck) []indexCheckDTO {
	resp := make([]indexCheckDTO, 0, len(list))
	if len(list) == 0 {
		return resp
	}

	domainIDs := make([]string, 0, len(list))
	seen := make(map[string]struct{})
	for _, check := range list {
		if _, ok := seen[check.DomainID]; ok {
			continue
		}
		seen[check.DomainID] = struct{}{}
		domainIDs = append(domainIDs, check.DomainID)
	}

	domainMap := map[string]sqlstore.Domain{}
	if s.domains != nil && len(domainIDs) > 0 {
		if domains, err := s.domains.ListByIDs(ctx, domainIDs); err == nil {
			for _, d := range domains {
				domainMap[d.ID] = d
			}
		}
	}

	for _, check := range list {
		dto := toIndexCheckDTO(check)
		if domain, ok := domainMap[check.DomainID]; ok {
			if url := strings.TrimSpace(domain.URL); url != "" {
				dto.DomainURL = &url
			}
			pid := domain.ProjectID
			dto.ProjectID = &pid
		}
		resp = append(resp, dto)
	}
	return resp
}
