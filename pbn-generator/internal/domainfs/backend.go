package domainfs

import "context"

type SiteContentBackend interface {
	ReadFile(ctx context.Context, dctx DomainFSContext, filePath string) ([]byte, error)
	WriteFile(ctx context.Context, dctx DomainFSContext, filePath string, content []byte) error
	EnsureDir(ctx context.Context, dctx DomainFSContext, dirPath string) error
	Stat(ctx context.Context, dctx DomainFSContext, path string) (FileInfo, error)
	ReadDir(ctx context.Context, dctx DomainFSContext, dirPath string) ([]EntryInfo, error)
	Move(ctx context.Context, dctx DomainFSContext, oldPath, newPath string) error
	Delete(ctx context.Context, dctx DomainFSContext, path string) error
	DeleteAll(ctx context.Context, dctx DomainFSContext, path string) error
	ListTree(ctx context.Context, dctx DomainFSContext, dirPath string) ([]FileInfo, error)
	DiscoverDomain(ctx context.Context, serverID, domainHost string) (publishedPath string, siteOwner string, err error)
}
