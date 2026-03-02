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

	"github.com/google/uuid"

	"obzornik-pbn-generator/internal/store/sqlstore"
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
	deployMode := strings.TrimSpace(state.DeploymentMode)
	if deployMode == "" {
		deployMode = "local_mock"
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
	publishedPath := state.Publisher.GetPublishedPath(state.DomainID)
	ownerValue := "mock:www-data"
	if strings.EqualFold(strings.TrimSpace(deployMode), "ssh_remote") && state.Domain != nil {
		if owner := strings.TrimSpace(state.Domain.SiteOwner.String); state.Domain.SiteOwner.Valid && owner != "" {
			ownerValue = owner
		}
	}

	deploymentAttemptID := ""
	if state.Deployments != nil {
		deploymentAttemptID = uuid.NewString()
		item := sqlstore.DeploymentAttempt{
			ID:           deploymentAttemptID,
			DomainID:     state.DomainID,
			GenerationID: state.GenerationID,
			Mode:         deployMode,
			TargetPath:   publishedPath,
			OwnerBefore:  sqlstore.NullableString(ownerValue),
			Status:       "processing",
		}
		if err := state.Deployments.Create(ctx, item); err != nil && state.AppendLog != nil {
			state.AppendLog(fmt.Sprintf("publish: failed to create deployment attempt: %v", err))
		}
	}

	if err := state.Publisher.Publish(ctx, state.DomainID, files); err != nil {
		if state.Deployments != nil && deploymentAttemptID != "" {
			errMsg := err.Error()
			ownerAfter := ownerValue
			if finishErr := state.Deployments.Finish(ctx, deploymentAttemptID, "error", &errMsg, &ownerAfter, 0, 0, time.Now().UTC()); finishErr != nil && state.AppendLog != nil {
				state.AppendLog(fmt.Sprintf("publish: failed to finish deployment attempt with error: %v", finishErr))
			}
		}
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

	fileCount := 0
	totalSizeBytes := int64(0)
	if state.DomainStore != nil {
		if refreshed, err := state.DomainStore.Get(ctx, state.DomainID); err == nil {
			fileCount = refreshed.FileCount
			totalSizeBytes = refreshed.TotalSizeBytes
			if state.Domain != nil {
				state.Domain.Status = "published"
				state.Domain.PublishedPath = refreshed.PublishedPath
				state.Domain.PublishedAt = refreshed.PublishedAt
				state.Domain.FileCount = refreshed.FileCount
				state.Domain.TotalSizeBytes = refreshed.TotalSizeBytes
			}
		}
	}
	if state.Deployments != nil && deploymentAttemptID != "" {
		ownerAfter := ownerValue
		if err := state.Deployments.Finish(ctx, deploymentAttemptID, "success", nil, &ownerAfter, fileCount, totalSizeBytes, time.Now().UTC()); err != nil && state.AppendLog != nil {
			state.AppendLog(fmt.Sprintf("publish: failed to finish deployment attempt: %v", err))
		}
	}
	artifacts := map[string]any{
		"published_path":    publishedPath,
		"file_count":        fileCount,
		"total_size_bytes":  totalSizeBytes,
		"deployment_mode":   deployMode,
		"deployment_status": "success",
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
