package domainfs

import (
	"context"
	"errors"
	"strings"
)

var ErrRemoteBackendUnavailable = errors.New("ssh backend is not configured")

type RouterBackend struct {
	local SiteContentBackend
	ssh   SiteContentBackend
}

func NewRouterBackend(local, ssh SiteContentBackend) *RouterBackend {
	return &RouterBackend{
		local: local,
		ssh:   ssh,
	}
}

func (b *RouterBackend) ReadFile(ctx context.Context, dctx DomainFSContext, filePath string) ([]byte, error) {
	backend, err := b.pickBackend(dctx)
	if err != nil {
		return nil, err
	}
	return backend.ReadFile(ctx, dctx, filePath)
}

func (b *RouterBackend) WriteFile(ctx context.Context, dctx DomainFSContext, filePath string, content []byte) error {
	backend, err := b.pickBackend(dctx)
	if err != nil {
		return err
	}
	return backend.WriteFile(ctx, dctx, filePath, content)
}

func (b *RouterBackend) EnsureDir(ctx context.Context, dctx DomainFSContext, dirPath string) error {
	backend, err := b.pickBackend(dctx)
	if err != nil {
		return err
	}
	return backend.EnsureDir(ctx, dctx, dirPath)
}

func (b *RouterBackend) Stat(ctx context.Context, dctx DomainFSContext, path string) (FileInfo, error) {
	backend, err := b.pickBackend(dctx)
	if err != nil {
		return FileInfo{}, err
	}
	return backend.Stat(ctx, dctx, path)
}

func (b *RouterBackend) ReadDir(ctx context.Context, dctx DomainFSContext, dirPath string) ([]EntryInfo, error) {
	backend, err := b.pickBackend(dctx)
	if err != nil {
		return nil, err
	}
	return backend.ReadDir(ctx, dctx, dirPath)
}

func (b *RouterBackend) Move(ctx context.Context, dctx DomainFSContext, oldPath, newPath string) error {
	backend, err := b.pickBackend(dctx)
	if err != nil {
		return err
	}
	return backend.Move(ctx, dctx, oldPath, newPath)
}

func (b *RouterBackend) Delete(ctx context.Context, dctx DomainFSContext, path string) error {
	backend, err := b.pickBackend(dctx)
	if err != nil {
		return err
	}
	return backend.Delete(ctx, dctx, path)
}

func (b *RouterBackend) DeleteAll(ctx context.Context, dctx DomainFSContext, path string) error {
	backend, err := b.pickBackend(dctx)
	if err != nil {
		return err
	}
	return backend.DeleteAll(ctx, dctx, path)
}

func (b *RouterBackend) ListTree(ctx context.Context, dctx DomainFSContext, dirPath string) ([]FileInfo, error) {
	backend, err := b.pickBackend(dctx)
	if err != nil {
		return nil, err
	}
	return backend.ListTree(ctx, dctx, dirPath)
}

func (b *RouterBackend) pickBackend(dctx DomainFSContext) (SiteContentBackend, error) {
	if strings.EqualFold(strings.TrimSpace(dctx.DeploymentMode), "ssh_remote") {
		if b.ssh == nil {
			return nil, ErrRemoteBackendUnavailable
		}
		return b.ssh, nil
	}
	if b.local == nil {
		return nil, errors.New("local backend is not configured")
	}
	return b.local, nil
}
