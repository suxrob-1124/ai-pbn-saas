package pipeline

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"
)

// PublishStep публикует собранную статику в локальное хранилище.
type PublishStep struct{}

func (s *PublishStep) Name() string { return StepPublish }

func (s *PublishStep) ArtifactKey() string { return "published_path" }

func (s *PublishStep) Progress() int { return 100 }

func (s *PublishStep) Execute(ctx context.Context, state *PipelineState) (map[string]any, error) {
	const defaultDelayMinutes = 60
	if state.Publisher == nil {
		return nil, fmt.Errorf("publish: publisher is not configured")
	}
	now := time.Now().UTC()
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
	if state.DomainStore != nil && state.Domain != nil {
		anchor := strings.TrimSpace(state.Domain.LinkAnchorText.String)
		target := strings.TrimSpace(state.Domain.LinkAcceptorURL.String)
		if state.Domain.LinkAnchorText.Valid && state.Domain.LinkAcceptorURL.Valid && anchor != "" && target != "" {
			if err := state.DomainStore.UpdateLinkStatus(ctx, state.DomainID, "needs_relink"); err != nil {
				return nil, fmt.Errorf("publish: update link status: %w", err)
			}
		}
	}
	if state.DomainStore != nil {
		delayMinutes := defaultDelayMinutes
		if state.LinkScheduleStore != nil && state.Domain != nil {
			sched, err := state.LinkScheduleStore.GetByProject(ctx, state.Domain.ProjectID)
			if err == nil {
				if parsed, err := parseLinkDelayMinutes(sched.Config, defaultDelayMinutes); err == nil {
					delayMinutes = parsed
				}
			} else if !errors.Is(err, sql.ErrNoRows) && state.AppendLog != nil {
				state.AppendLog(fmt.Sprintf("publish: link schedule load failed: %v", err))
			}
		}
		readyAt := now.Add(time.Duration(delayMinutes) * time.Minute)
		if err := state.DomainStore.UpdateLinkReadyAt(ctx, state.DomainID, readyAt); err != nil {
			return nil, fmt.Errorf("publish: update link ready at: %w", err)
		}
	}

	publishedPath := state.Publisher.GetPublishedPath(state.DomainID)
	artifacts := map[string]any{
		"published_path": publishedPath,
	}
	return artifacts, nil
}

func parseLinkDelayMinutes(raw json.RawMessage, fallback int) (int, error) {
	if fallback <= 0 {
		fallback = 60
	}
	if len(raw) == 0 {
		return fallback, nil
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return fallback, err
	}
	rawDelay, ok := data["delay_minutes"]
	if !ok {
		return fallback, nil
	}
	switch value := rawDelay.(type) {
	case float64:
		if value < 0 {
			return fallback, fmt.Errorf("delay_minutes must be non-negative")
		}
		return int(value), nil
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fallback, nil
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil || parsed < 0 {
			return fallback, fmt.Errorf("delay_minutes must be non-negative")
		}
		return parsed, nil
	default:
		return fallback, fmt.Errorf("delay_minutes must be number or string")
	}
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
