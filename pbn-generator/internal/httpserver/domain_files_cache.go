package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

const domainFilesCacheTTL = 5 * time.Minute

var errDomainFilesCacheMiss = errors.New("domain files cache miss")

type domainFilesCache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, data []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

type noopDomainFilesCache struct{}

func (noopDomainFilesCache) Get(_ context.Context, _ string) ([]byte, error) {
	return nil, errDomainFilesCacheMiss
}

func (noopDomainFilesCache) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}

func (noopDomainFilesCache) Delete(_ context.Context, _ ...string) error {
	return nil
}

type redisDomainFilesCache struct {
	client *redis.Client
}

func newRedisDomainFilesCache(client *redis.Client) domainFilesCache {
	if client == nil {
		return noopDomainFilesCache{}
	}
	return &redisDomainFilesCache{client: client}
}

func (c *redisDomainFilesCache) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errDomainFilesCacheMiss
		}
		return nil, err
	}
	return value, nil
}

func (c *redisDomainFilesCache) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *redisDomainFilesCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (s *Server) resolveDomainFilesCache() domainFilesCache {
	if s.domainFilesCache != nil {
		return s.domainFilesCache
	}
	return noopDomainFilesCache{}
}

func domainFilesCacheKey(domainID string, includeDeleted bool) string {
	flag := "0"
	if includeDeleted {
		flag = "1"
	}
	return fmt.Sprintf("editor:tree:%s:deleted:%s", strings.TrimSpace(domainID), flag)
}

func domainFilesCacheKeys(domainID string) []string {
	return []string{
		domainFilesCacheKey(domainID, false),
		domainFilesCacheKey(domainID, true),
	}
}

func (s *Server) loadDomainFilesListCached(ctx context.Context, domain sqlstore.Domain, includeDeleted bool) ([]fileDTO, error) {
	key := domainFilesCacheKey(domain.ID, includeDeleted)
	cache := s.resolveDomainFilesCache()
	if raw, err := cache.Get(ctx, key); err == nil {
		var cached []fileDTO
		if unmarshalErr := json.Unmarshal(raw, &cached); unmarshalErr == nil {
			return cached, nil
		}
	} else if err != nil && !errors.Is(err, errDomainFilesCacheMiss) && s.logger != nil {
		s.logger.Warnf("domain files cache get failed for %s: %v", domain.ID, err)
	}

	resp, err := s.buildDomainFilesListResponse(ctx, domain, includeDeleted)
	if err != nil {
		return nil, err
	}
	if encoded, err := json.Marshal(resp); err == nil {
		if setErr := cache.Set(ctx, key, encoded, domainFilesCacheTTL); setErr != nil && s.logger != nil {
			s.logger.Warnf("domain files cache set failed for %s: %v", domain.ID, setErr)
		}
	}
	return resp, nil
}

func (s *Server) invalidateDomainFilesCache(ctx context.Context, domainID string) {
	if strings.TrimSpace(domainID) == "" {
		return
	}
	if err := s.resolveDomainFilesCache().Delete(ctx, domainFilesCacheKeys(domainID)...); err != nil && s.logger != nil {
		s.logger.Warnf("domain files cache invalidate failed for %s: %v", domainID, err)
	}
}
