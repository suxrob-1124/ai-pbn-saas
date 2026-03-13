package httpserver

import (
	"context"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

// finalizeAgentRevisions creates one visible file revision per unique file path
// in filesChanged, reflecting the post-session content on disk.
//
// source controls the revision kind:
//   - "agent"         — successful session (status "done")
//   - "agent_partial" — interrupted session (status "error" / "stopped")
//
// Deleted files: when a file no longer exists (agent called delete_file), the
// function looks up the snapshot baseline revision to recover the original FileID
// and creates a zero-content deletion-marker revision so the file's history
// records that the agent deleted it.  Files created and then deleted within the
// same session (no baseline) are silently skipped — they never existed before
// the agent, so no history entry is needed.
//
// Duplicate paths in filesChanged are deduplicated: only the first occurrence
// triggers a revision (the file is read from the backend at that point, so its
// content is already the final result of all agent writes).
//
// Non-fatal: if reading or creating a revision fails for a file, it is skipped
// and other files are still processed.
func (s *Server) finalizeAgentRevisions(
	ctx context.Context,
	domain sqlstore.Domain,
	filesChanged []string,
	sessionID, editedBy, source string,
) {
	if len(filesChanged) == 0 {
		return
	}

	desc := "agent session " + sessionID
	if source == "agent_partial" {
		desc = "partial update by agent session " + sessionID
	}

	// Lazily loaded map of path → snapshot baseline for deleted files.
	var deletedBaselines map[string]sqlstore.FileRevision
	loadDeletedBaselines := func() {
		if deletedBaselines != nil {
			return
		}
		deletedBaselines = make(map[string]sqlstore.FileRevision)
		baselines, err := s.fileEdits.ListRevisionsBySource(ctx, agentSnapshotTag(sessionID))
		if err != nil {
			return
		}
		for _, rev := range baselines {
			if rev.Description.Valid && rev.Description.String != "" {
				deletedBaselines[rev.Description.String] = rev
			}
		}
	}

	seen := make(map[string]bool, len(filesChanged))
	for _, filePath := range filesChanged {
		if seen[filePath] {
			continue
		}
		seen[filePath] = true

		file, err := s.siteFiles.GetByPath(ctx, domain.ID, filePath)
		if err != nil || file == nil {
			// File was deleted by the agent — create a deletion-marker revision
			// using the FileID from the snapshot baseline.
			loadDeletedBaselines()
			baseline, ok := deletedBaselines[filePath]
			if !ok {
				continue // created and deleted in same session — no prior history
			}
			marker := &sqlstore.SiteFile{
				ID:       baseline.FileID,
				Version:  baseline.Version + 1,
				MimeType: baseline.MimeType,
			}
			_ = s.fileEdits.CreateRevision(ctx, buildRevision(marker, nil, source, editedBy, "deleted by "+desc))
			continue
		}

		content, err := s.readDomainFileBytesFromBackend(ctx, domain, filePath)
		if err != nil {
			continue
		}

		_ = s.fileEdits.CreateRevision(ctx, buildRevision(file, content, source, editedBy, desc))
	}
}
