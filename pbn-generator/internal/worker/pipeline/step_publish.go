package pipeline

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"path"
	"strings"
)

// PublishStep публикует собранную статику в локальное хранилище.
type PublishStep struct{}

func (s *PublishStep) Name() string { return StepPublish }

func (s *PublishStep) ArtifactKey() string { return "published_path" }

func (s *PublishStep) Progress() int { return 100 }

func (s *PublishStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	if state.Publisher == nil {
		return nil, fmt.Errorf("publish: publisher is not configured")
	}
	zipB64, _ := state.Artifacts["zip_archive"].(string)
	if strings.TrimSpace(zipB64) == "" {
		return nil, fmt.Errorf("publish: zip_archive artifact is empty")
	}
	zipBytes, err := base64.StdEncoding.DecodeString(zipB64)
	if err != nil {
		return nil, fmt.Errorf("publish: decode zip: %w", err)
	}
	files, err := unzipToMap(zipBytes)
	if err != nil {
		return nil, fmt.Errorf("publish: unzip: %w", err)
	}
	if err := state.Publisher.Publish(ctx, state.DomainID, files); err != nil {
		return nil, fmt.Errorf("publish: %w", err)
	}
	if err := state.DomainStore.UpdateStatus(ctx, state.DomainID, "published"); err != nil {
		return nil, fmt.Errorf("publish: update domain status: %w", err)
	}

	publishedPath := state.Publisher.GetPublishedPath(state.DomainID)
	artifacts := map[string]any{
		"published_path": publishedPath,
	}
	return artifacts, nil
}

func unzipToMap(zipBytes []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip: %w", err)
	}
	files := make(map[string][]byte)
	for _, f := range reader.File {
		name := strings.TrimSpace(f.Name)
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		clean := path.Clean(name)
		if clean == "." || clean == "" || path.IsAbs(clean) || strings.HasPrefix(clean, "../") || clean == ".." {
			return nil, fmt.Errorf("unsafe path in zip: %s", name)
		}
		if strings.Contains(clean, "..") {
			parts := strings.Split(clean, "/")
			for _, part := range parts {
				if part == ".." {
					return nil, fmt.Errorf("unsafe path in zip: %s", name)
				}
			}
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip entry %s: %w", name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read zip entry %s: %w", name, err)
		}
		files[clean] = data
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("zip has no files")
	}
	return files, nil
}
