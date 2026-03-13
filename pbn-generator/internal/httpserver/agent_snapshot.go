package httpserver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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

// createAgentSnapshot records which files exist before the agent starts.
// It saves only the file ID manifest (pre_file_ids) — no content is read.
// Content baselines are created lazily via ensureAgentBaselineForPath when
// the agent first mutates each individual file, making session start O(1 list query)
// instead of O(all domain files).
//
// The authoritative list of pre-existing file IDs is stored in the agent_sessions
// table and drives rollback logic: files not in the list are deleted, files in the
// list are protected from deletion.
func (s *Server) createAgentSnapshot(ctx context.Context, domain sqlstore.Domain, sessionID, editedBy string) (string, error) {
	if editedBy == "" {
		return "", fmt.Errorf("agent snapshot requires editedBy user")
	}

	tag := agentSnapshotTag(sessionID)

	files, err := s.siteFiles.List(ctx, domain.ID)
	if err != nil {
		return "", fmt.Errorf("list files for snapshot: %w", err)
	}

	// Save only the manifest — no content reads.
	fileIDs := make([]string, 0, len(files))
	for _, f := range files {
		fileIDs = append(fileIDs, f.ID)
	}
	if err := s.agentSessions.SavePreFileIDs(ctx, sessionID, fileIDs); err != nil {
		return "", fmt.Errorf("save pre-existing file IDs: %w", err)
	}

	return tag, nil
}

// ensureAgentBaselineForPath lazily creates a content revision for filePath if one
// does not yet exist for this session.  It is called before the first mutation
// (write, patch, or delete) of each file so rollback can restore the pre-session content.
//
// Returns true if the file is safe to mutate:
//   - file does not exist yet (new file — rollback will delete it)
//   - baseline was already saved in a prior call this session
//   - baseline was successfully saved in this call
//
// Returns false if the file exists but its baseline could NOT be saved (transient
// backend read failure or DB write error).  Callers MUST NOT mutate the file when
// false is returned — doing so would leave the file in an unrollbackable state.
//
// baselined is a per-session in-memory set (keyed by siteFile.ID) that prevents
// duplicate revision creation attempts.  Agent tools run sequentially within a
// session, so no synchronisation is needed.
//
// Mark-after-persist: baselined[file.ID] is set only after a successful read AND
// a successful CreateRevision, so both a transient backend read error and a real DB
// write error allow a retry on the next mutation of the same file.
// A UNIQUE conflict (prior session already has a revision for this file+version) is
// handled with ON CONFLICT DO NOTHING by the store — CreateRevision returns nil —
// so the file is correctly marked and rollback can use the existing revision.
//
// The file path is stored in rev.Description so rollback can restore content even
// if agentToolDeleteFile later removes the SiteFile record (delete→recreate scenario).
func (s *Server) ensureAgentBaselineForPath(
	ctx context.Context,
	domain sqlstore.Domain,
	sessionID, filePath, editedBy string,
	baselined map[string]bool,
) bool {
	file, err := s.siteFiles.GetByPath(ctx, domain.ID, filePath)
	if err != nil || file == nil {
		// File does not exist yet (new file) — rollback will delete it.
		return true
	}
	if baselined[file.ID] {
		return true // already baselined this session
	}

	content, err := s.readDomainFileBytesFromBackend(ctx, domain, filePath)
	if err != nil {
		// Transient backend failure — do NOT mark, allow retry on next mutation.
		return false
	}

	mimeType := file.MimeType
	if mimeType == "" {
		mimeType = mime.TypeByExtension(path.Ext(filePath))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	rev := sqlstore.FileRevision{
		ID:        uuid.New().String(),
		FileID:    file.ID,
		Version:   file.Version,
		Content:   content,
		SizeBytes: int64(len(content)),
		MimeType:  mimeType,
		Source:    agentSnapshotTag(sessionID),
		EditedBy:  editedBy,
		// Store the path so rollbackAgentSnapshot can restore content even if the
		// SiteFile record is later deleted (e.g. agent delete_file → recreate at same path).
		Description: sql.NullString{String: filePath, Valid: true},
	}
	if err := s.fileEdits.CreateRevision(ctx, rev); err != nil {
		// Real DB error (conflicts are silenced by ON CONFLICT DO NOTHING in the store).
		// Do NOT mark — allow retry on next mutation.
		return false
	}
	// Mark only after the revision is durably saved (or a silent conflict confirms
	// a prior-session revision already exists and is usable for rollback).
	baselined[file.ID] = true
	return true
}

// agentRollbackResult holds the outcome of rollbackAgentSnapshot:
//   - RestoredExisting: pre-existing files whose content was reverted from snapshot baseline.
//   - DeletedCreated: files created by the agent that were deleted during rollback.
//
// Rollback is considered successful when either counter is non-zero.
type agentRollbackResult struct {
	RestoredExisting int
	DeletedCreated   int
}

// rollbackAgentSnapshot restores all files to their pre-session content.
// Files that existed before the agent are protected from deletion using the
// PreExistingFileIDs list stored in the session (not derived from revisions).
// Files created by the agent (not in the list) are deleted.
//
// Delete→recreate recovery: if a pre-existing file's SiteFile record was deleted
// by agentToolDeleteFile, the path is recovered from rev.Description and the
// SiteFile record is recreated so the site directory is left in a consistent state.
func (s *Server) rollbackAgentSnapshot(ctx context.Context, domain sqlstore.Domain, sess sqlstore.AgentSession) (agentRollbackResult, error) {
	var result agentRollbackResult
	if !sess.SnapshotTag.Valid || sess.SnapshotTag.String == "" {
		return result, fmt.Errorf("session has no snapshot tag")
	}
	snapshotTag := sess.SnapshotTag.String

	// Build set of file IDs that existed before the agent.
	preExisting := make(map[string]bool, len(sess.PreExistingFileIDs))
	for _, id := range sess.PreExistingFileIDs {
		preExisting[id] = true
	}

	// Get all currently active files for the domain.
	currentFiles, err := s.siteFiles.List(ctx, domain.ID)
	if err != nil {
		return result, fmt.Errorf("list current files for rollback: %w", err)
	}

	// Delete only files that were CREATED by the agent (not in pre-existing set).
	// Invalidate the cache now — agent-created files are already being removed, and
	// we must invalidate even if the revision load below fails (partial rollback).
	for _, f := range currentFiles {
		if preExisting[f.ID] {
			continue // existed before agent — do not delete
		}
		if err := s.deleteDomainPathInBackend(ctx, domain, f.Path); err != nil {
			continue // file still on disk — skip DB delete and do not count
		}
		s.cleanupEmptyParentDirs(ctx, domain, f.Path)
		if err := s.siteFiles.Delete(ctx, f.ID); err == nil {
			result.DeletedCreated++
		}
	}
	s.invalidateDomainFilesCache(ctx, domain.ID)

	// Load content revisions for restoration.
	revisions, err := s.fileEdits.ListRevisionsBySource(ctx, snapshotTag)
	if err != nil {
		return result, fmt.Errorf("list snapshot revisions: %w", err)
	}

	// Build map from file_id → baseline revision.
	// Take the revision with the LOWEST version to get the pre-session original,
	// guarding against edge cases where multiple revisions exist for the same file.
	revByFileID := make(map[string]sqlstore.FileRevision, len(revisions))
	for _, rev := range revisions {
		if existing, ok := revByFileID[rev.FileID]; !ok || rev.Version < existing.Version {
			revByFileID[rev.FileID] = rev
		}
	}

	// Restore files that existed before the agent and have a saved revision.
	for _, fileID := range sess.PreExistingFileIDs {
		rev, ok := revByFileID[fileID]
		if !ok || len(rev.Content) == 0 {
			// No content revision — file is protected from deletion but
			// its content cannot be restored (binary asset, or baseline never reached).
			// This is acceptable: the file stays as-is.
			continue
		}

		file, err := s.siteFiles.Get(ctx, fileID)
		if err != nil || file == nil {
			// SiteFile record was deleted (agent called delete_file on a pre-existing file).
			// Recover the path from the revision description set by ensureAgentBaselineForPath.
			if !rev.Description.Valid || rev.Description.String == "" {
				continue // no path available — cannot restore
			}
			filePath := rev.Description.String
			if err := s.writeDomainFileBytesToBackend(ctx, domain, filePath, rev.Content); err != nil {
				continue
			}
			// Recreate the SiteFile record (using original ID) so the file is visible
			// to list_files and future operations after rollback.
			// Only count as restored if the DB record is also successfully recreated;
			// a disk-only restore without a DB record is a partial failure.
			hash := sha256.Sum256(rev.Content)
			recreated := sqlstore.SiteFile{
				ID:           fileID,
				DomainID:     domain.ID,
				Path:         filePath,
				MimeType:     rev.MimeType,
				SizeBytes:    rev.SizeBytes,
				Version:      rev.Version,
				ContentHash:  sql.NullString{String: hex.EncodeToString(hash[:]), Valid: true},
				LastEditedBy: sql.NullString{String: rev.EditedBy, Valid: rev.EditedBy != ""},
			}
			if err := s.siteFiles.Create(ctx, recreated); err != nil {
				continue
			}
			// The snapshot baseline occupies (file_id, version=rev.Version); the revert
			// revision must use version+1 to avoid ON CONFLICT DO NOTHING silencing it.
			revertFile := recreated
			revertFile.Version = recreated.Version + 1
			_ = s.fileEdits.CreateRevision(ctx, buildRevision(&revertFile, rev.Content, "revert", sess.CreatedBy,
				"rollback of agent session "+sess.ID))
			result.RestoredExisting++
			continue
		}

		if err := s.writeDomainFileBytesToBackend(ctx, domain, file.Path, rev.Content); err != nil {
			continue
		}
		// Only count as restored if DB metadata is also updated; a disk-only restore
		// without an updated DB record is a partial failure.
		if err := s.siteFiles.Update(ctx, file.ID, rev.Content); err != nil {
			continue
		}
		// Re-fetch after Update so buildRevision uses the bumped version, avoiding
		// ON CONFLICT DO NOTHING silencing the revert revision due to version collision
		// with the snapshot baseline (which was saved at the pre-agent version).
		if updated, err2 := s.siteFiles.Get(ctx, file.ID); err2 == nil && updated != nil {
			_ = s.fileEdits.CreateRevision(ctx, buildRevision(updated, rev.Content, "revert", sess.CreatedBy,
				"rollback of agent session "+sess.ID))
		}
		result.RestoredExisting++
	}

	// Re-invalidate after the restore loop so list_files reflects newly-restored files.
	// (The first invalidation above covered deleted agent-created files.)
	s.invalidateDomainFilesCache(ctx, domain.ID)

	return result, nil
}
