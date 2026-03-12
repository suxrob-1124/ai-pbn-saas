package httpserver

import (
	"context"
	"fmt"
	"mime"
	"path"

	"github.com/google/uuid"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// agentSnapshotTag returns the source tag used in file revisions for a given agent session.
func agentSnapshotTag(sessionID string) string {
	return "agent_snapshot:" + sessionID
}

// createAgentSnapshot saves a revision of every current domain file before the agent modifies anything.
// Returns the snapshot tag to use for subsequent rollback.
func (s *Server) createAgentSnapshot(ctx context.Context, domain sqlstore.Domain, sessionID string) (string, error) {
	tag := agentSnapshotTag(sessionID)

	files, err := s.siteFiles.List(ctx, domain.ID)
	if err != nil {
		return "", fmt.Errorf("list files for snapshot: %w", err)
	}

	for _, f := range files {
		content, err := s.readDomainFileBytesFromBackend(ctx, domain, f.Path)
		if err != nil {
			// Skip unreadable files (e.g. binary assets we don't manage)
			continue
		}

		mimeType := f.MimeType
		if mimeType == "" {
			mimeType = mime.TypeByExtension(path.Ext(f.Path))
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		rev := sqlstore.FileRevision{
			ID:       uuid.New().String(),
			FileID:   f.ID,
			Content:  content,
			MimeType: mimeType,
			Source:   tag,
			EditedBy: "agent",
		}
		// ContentHash and SizeBytes will be set by CreateRevision if needed.
		rev.SizeBytes = int64(len(content))

		if err := s.fileEdits.CreateRevision(ctx, rev); err != nil {
			// Non-fatal: log and continue; worst case rollback is incomplete
			continue
		}
	}

	return tag, nil
}

// rollbackAgentSnapshot restores all files to their pre-session content.
// Files that existed before the agent are restored from snapshot revisions.
// Files created by the agent (no snapshot revision) are deleted.
// Returns the number of files successfully restored.
func (s *Server) rollbackAgentSnapshot(ctx context.Context, domain sqlstore.Domain, snapshotTag string) (int, error) {
	revisions, err := s.fileEdits.ListRevisionsBySource(ctx, snapshotTag)
	if err != nil {
		return 0, fmt.Errorf("list snapshot revisions: %w", err)
	}

	// Build a map of fileID -> revision for files that existed before the agent.
	snapshotByFileID := make(map[string]sqlstore.FileRevision, len(revisions))
	for _, rev := range revisions {
		snapshotByFileID[rev.FileID] = rev
	}

	// Get all currently active files for the domain.
	currentFiles, err := s.siteFiles.List(ctx, domain.ID)
	if err != nil {
		return 0, fmt.Errorf("list current files for rollback: %w", err)
	}

	// Delete files that were created by the agent (they have no pre-agent snapshot revision).
	for _, f := range currentFiles {
		if _, ok := snapshotByFileID[f.ID]; ok {
			continue // will be restored below
		}
		_ = s.deleteDomainPathInBackend(ctx, domain, f.Path)
		s.cleanupEmptyParentDirs(ctx, domain, f.Path)
		_ = s.siteFiles.Delete(ctx, f.ID)
	}

	// Restore files that existed before the agent session.
	restored := 0
	for _, rev := range revisions {
		file, err := s.siteFiles.Get(ctx, rev.FileID)
		if err != nil || file == nil {
			continue
		}

		if err := s.writeDomainFileBytesToBackend(ctx, domain, file.Path, rev.Content); err != nil {
			continue
		}
		// Keep the DB record in sync with the restored content (hash, size, version).
		_ = s.siteFiles.Update(ctx, file.ID, rev.Content)
		restored++
	}

	return restored, nil
}
