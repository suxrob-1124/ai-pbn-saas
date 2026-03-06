package legacy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"obzornik-pbn-generator/internal/domainfs"
	"obzornik-pbn-generator/internal/publisher"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// webDomainStore extends the base domainStore with inventory/deployment methods.
type webDomainStore interface {
	domainStore
	Get(ctx context.Context, id string) (sqlstore.Domain, error)
	UpdateInventoryState(ctx context.Context, id string, publishedPath, siteOwner, inventoryStatus, inventoryError sql.NullString, checkedAt time.Time) error
	UpdateDeploymentMode(ctx context.Context, id, mode string) error
}

// sshBackend defines the SSH operations needed for web import.
type sshBackend interface {
	DiscoverDomain(ctx context.Context, serverID, domainHost string) (string, string, error)
	ListTree(ctx context.Context, dctx domainfs.DomainFSContext, dirPath string) ([]domainfs.FileInfo, error)
	ReadFile(ctx context.Context, dctx domainfs.DomainFSContext, filePath string) ([]byte, error)
}

// legacyImportStore persists job/item progress.
type legacyImportStore interface {
	UpdateItemStatus(ctx context.Context, id, status, step string, progress int, errMsg *string) error
	UpdateItemResult(ctx context.Context, id, status string, actions, warnings []byte, fileCount int, totalSize int64, errMsg *string) error
}

// DomainPreview holds preflight check results for a domain.
type DomainPreview struct {
	DomainID           string `json:"domain_id"`
	DomainURL          string `json:"domain_url"`
	HasFiles           bool   `json:"has_files"`
	HasLink            bool   `json:"has_link"`
	LinkAnchor         string `json:"link_anchor,omitempty"`
	LinkTarget         string `json:"link_target,omitempty"`
	HasLegacyArtifacts bool   `json:"has_legacy_artifacts"`
	HasNonLegacyGen    bool   `json:"has_non_legacy_gen"`
}

// WebImportService processes legacy imports triggered from the web UI.
type WebImportService struct {
	domains  webDomainStore
	files    publisher.SiteFileStore
	links    linkTaskStore
	gens     generationStore
	prompts  promptStore
	ssh      sshBackend
	imports  legacyImportStore
}

// NewWebImportService creates a new WebImportService.
func NewWebImportService(
	domains webDomainStore,
	files publisher.SiteFileStore,
	links linkTaskStore,
	gens generationStore,
	prompts promptStore,
	ssh sshBackend,
	imports legacyImportStore,
) *WebImportService {
	return &WebImportService{
		domains: domains,
		files:   files,
		links:   links,
		gens:    gens,
		prompts: prompts,
		ssh:     ssh,
		imports: imports,
	}
}

// PreviewDomain checks the current state of a domain without making changes.
func (s *WebImportService) PreviewDomain(ctx context.Context, domain sqlstore.Domain) (DomainPreview, error) {
	p := DomainPreview{
		DomainID:  domain.ID,
		DomainURL: domain.URL,
	}

	// Check for existing files.
	if domain.FileCount > 0 {
		p.HasFiles = true
	}

	// Check for existing link.
	if domain.LinkAnchorText.Valid && domain.LinkAcceptorURL.Valid &&
		domain.LinkStatus.Valid && strings.ToLower(strings.TrimSpace(domain.LinkStatus.String)) == "inserted" {
		p.HasLink = true
		p.LinkAnchor = domain.LinkAnchorText.String
		p.LinkTarget = domain.LinkAcceptorURL.String
	}

	// Check for legacy/non-legacy generations.
	if s.gens != nil {
		gens, err := s.gens.ListByDomain(ctx, domain.ID)
		if err != nil {
			return p, fmt.Errorf("list generations: %w", err)
		}
		legacyGen, hasNonLegacy := findLegacyGeneration(gens)
		p.HasLegacyArtifacts = legacyGen != nil
		p.HasNonLegacyGen = hasNonLegacy
	}

	return p, nil
}

// ProcessSingleDomain runs the full legacy import pipeline for one domain.
// It updates item progress in the database as it proceeds through each step.
func (s *WebImportService) ProcessSingleDomain(
	ctx context.Context,
	jobID, itemID string,
	domain sqlstore.Domain,
	ownerEmail string,
	force bool,
) error {
	var actions []string
	var warnings []string

	fail := func(step string, progress int, err error) error {
		errStr := err.Error()
		actionsJSON, _ := json.Marshal(actions)
		warningsJSON, _ := json.Marshal(warnings)
		_ = s.imports.UpdateItemResult(ctx, itemID, "failed", actionsJSON, warningsJSON, 0, 0, &errStr)
		return fmt.Errorf("step %s: %w", step, err)
	}

	warn := func(msg string) {
		warnings = append(warnings, msg)
	}

	serverID := strings.TrimSpace(domain.ServerID.String)
	if !domain.ServerID.Valid || serverID == "" {
		return fail("ssh_probe", 0, fmt.Errorf("domain has no server_id configured"))
	}

	domainHost, err := extractHost(domain.URL)
	if err != nil {
		return fail("ssh_probe", 0, fmt.Errorf("invalid domain URL: %w", err))
	}

	// ── Step 1: SSH Probe ──────────────────────────────────────────────
	_ = s.imports.UpdateItemStatus(ctx, itemID, "running", "ssh_probe", 10, nil)

	publishedPath, siteOwner, err := s.ssh.DiscoverDomain(ctx, serverID, domainHost)
	if err != nil {
		return fail("ssh_probe", 10, fmt.Errorf("SSH probe failed: %w", err))
	}
	actions = append(actions, "ssh_probed")

	// Update inventory state on the domain.
	now := time.Now().UTC()
	if err := s.domains.UpdateInventoryState(ctx, domain.ID,
		sqlstore.NullableString(publishedPath),
		sqlstore.NullableString(siteOwner),
		sqlstore.NullableString("found"),
		sql.NullString{},
		now,
	); err != nil {
		return fail("ssh_probe", 10, fmt.Errorf("update inventory state: %w", err))
	}

	dctx := domainfs.DomainFSContext{
		DomainID:       domain.ID,
		DomainURL:      domain.URL,
		DeploymentMode: "ssh_remote",
		PublishedPath:  publishedPath,
		SiteOwner:      siteOwner,
		ServerID:       serverID,
	}

	// ── Step 2: File Sync ──────────────────────────────────────────────
	_ = s.imports.UpdateItemStatus(ctx, itemID, "running", "file_sync", 25, nil)

	tree, err := s.ssh.ListTree(ctx, dctx, "")
	if err != nil {
		return fail("file_sync", 25, fmt.Errorf("list remote files: %w", err))
	}

	files := make(map[string][]byte, len(tree))
	for _, fi := range tree {
		if fi.IsDir {
			continue
		}
		content, err := s.ssh.ReadFile(ctx, dctx, fi.Path)
		if err != nil {
			warn(fmt.Sprintf("read file %s: %v", fi.Path, err))
			continue
		}
		files[fi.Path] = content
	}

	if len(files) == 0 {
		return fail("file_sync", 25, fmt.Errorf("no files found on remote server"))
	}

	fileCount, totalSize, err := publisher.SyncPublishedFilesFromMap(ctx, domain.ID, files, s.files)
	if err != nil {
		return fail("file_sync", 25, fmt.Errorf("sync files to DB: %w", err))
	}

	if err := s.domains.UpdatePublishState(ctx, domain.ID, publishedPath, fileCount, totalSize); err != nil {
		return fail("file_sync", 25, fmt.Errorf("update publish state: %w", err))
	}
	if err := s.domains.UpdateStatus(ctx, domain.ID, "published"); err != nil {
		return fail("file_sync", 25, fmt.Errorf("update domain status: %w", err))
	}
	actions = append(actions, "files_synced")

	// ── Step 3: Link Decode ────────────────────────────────────────────
	_ = s.imports.UpdateItemStatus(ctx, itemID, "running", "link_decode", 50, nil)

	indexContent, hasIndex := files["index.html"]
	if !hasIndex {
		warn("index.html not found in synced files; link decode skipped")
	}

	var decoded *DecodedLink
	if hasIndex {
		decoded, err = DecodePrimaryHTTPSLink(string(indexContent), domain.URL)
		if err != nil {
			warn(fmt.Sprintf("link decode error: %v", err))
		}
		if decoded == nil {
			warn("external https link not found in index.html")
		} else {
			actions = append(actions, "link_decoded")
		}
	}

	// ── Step 4: Link Baseline ──────────────────────────────────────────
	_ = s.imports.UpdateItemStatus(ctx, itemID, "running", "link_baseline", 65, nil)

	if decoded != nil {
		if !force && shouldSkipLinkBaseline(domain, decoded) {
			actions = append(actions, "link_baseline_skipped")
			warn("link baseline already exists, use force to overwrite")
		} else {
			if _, err := s.domains.UpdateLinkSettings(ctx, domain.ID,
				sqlstore.NullableString(decoded.AnchorText),
				sqlstore.NullableString(decoded.TargetURL)); err != nil {
				return fail("link_baseline", 65, fmt.Errorf("update link settings: %w", err))
			}

			taskNow := time.Now().UTC()
			taskID := uuid.NewString()
			linkTask := sqlstore.LinkTask{
				ID:            taskID,
				DomainID:      domain.ID,
				AnchorText:    decoded.AnchorText,
				TargetURL:     decoded.TargetURL,
				ScheduledFor:  taskNow,
				Action:        "insert",
				Status:        "inserted",
				FoundLocation: sql.NullString{String: decoded.FoundLocation, Valid: true},
				Attempts:      0,
				CreatedBy:     ownerEmail,
				CreatedAt:     taskNow,
				CompletedAt:   sql.NullTime{Time: taskNow, Valid: true},
			}
			if err := s.links.Create(ctx, linkTask); err != nil {
				return fail("link_baseline", 65, fmt.Errorf("create synthetic link task: %w", err))
			}
			if err := s.domains.UpdateLinkState(ctx, domain.ID, "inserted", taskID, decoded.FoundPath, decoded.AnchorText); err != nil {
				return fail("link_baseline", 65, fmt.Errorf("update link state: %w", err))
			}
			actions = append(actions, "link_baseline_created")
		}
	} else if domain.LinkAnchorText.Valid && domain.LinkAcceptorURL.Valid &&
		strings.TrimSpace(domain.LinkAnchorText.String) != "" &&
		strings.TrimSpace(domain.LinkAcceptorURL.String) != "" {
		// Fallback: domain has pre-set anchor/acceptor but link not found in HTML.
		anchor := strings.TrimSpace(domain.LinkAnchorText.String)
		acceptor := strings.TrimSpace(domain.LinkAcceptorURL.String)

		existingStatus := strings.ToLower(strings.TrimSpace(domain.LinkStatus.String))
		if !force && domain.LinkStatus.Valid && (existingStatus == "inserted" || existingStatus == "pending") {
			actions = append(actions, "link_baseline_skipped")
			warn(fmt.Sprintf("link settings exist (anchor: %s, target: %s), link task already present", anchor, acceptor))
		} else {
			taskNow := time.Now().UTC()
			taskID := uuid.NewString()
			linkTask := sqlstore.LinkTask{
				ID:           taskID,
				DomainID:     domain.ID,
				AnchorText:   anchor,
				TargetURL:    acceptor,
				ScheduledFor: taskNow,
				Action:       "insert",
				Status:       "pending",
				Attempts:     0,
				CreatedBy:    ownerEmail,
				CreatedAt:    taskNow,
			}
			if err := s.links.Create(ctx, linkTask); err != nil {
				return fail("link_baseline", 65, fmt.Errorf("create pending link task from settings: %w", err))
			}
			if err := s.domains.UpdateLinkState(ctx, domain.ID, "pending", taskID, "", anchor); err != nil {
				return fail("link_baseline", 65, fmt.Errorf("update link state: %w", err))
			}
			actions = append(actions, "link_baseline_from_settings")
			warn("link not found in HTML, created pending task from domain settings")
		}
	}

	// ── Step 5: Artifacts ──────────────────────────────────────────────
	_ = s.imports.UpdateItemStatus(ctx, itemID, "running", "artifacts", 80, nil)

	if s.gens != nil {
		// Write files to a temp directory for BuildLegacyArtifacts.
		tmpDir, err := os.MkdirTemp("", "legacy-import-*")
		if err != nil {
			return fail("artifacts", 80, fmt.Errorf("create temp dir: %w", err))
		}
		defer os.RemoveAll(tmpDir)

		for relPath, content := range files {
			fullPath := filepath.Join(tmpDir, relPath)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				return fail("artifacts", 80, fmt.Errorf("mkdir for %s: %w", relPath, err))
			}
			if err := os.WriteFile(fullPath, content, 0o644); err != nil {
				return fail("artifacts", 80, fmt.Errorf("write temp file %s: %w", relPath, err))
			}
		}

		artifactActions, artifactWarnings, artifactErr := s.syncLegacyArtifacts(ctx, domain, ownerEmail, tmpDir, domain.URL, "import_legacy", force)
		actions = append(actions, artifactActions...)
		for _, w := range artifactWarnings {
			warn(w)
		}
		if artifactErr != nil {
			warn(fmt.Sprintf("artifacts error: %v", artifactErr))
			actions = append(actions, "legacy_artifacts_decode_skipped")
		}
	}

	// ── Step 6: Inventory Update ───────────────────────────────────────
	_ = s.imports.UpdateItemStatus(ctx, itemID, "running", "inventory_update", 95, nil)

	if err := s.domains.UpdateDeploymentMode(ctx, domain.ID, "ssh_remote"); err != nil {
		return fail("inventory_update", 95, fmt.Errorf("update deployment mode: %w", err))
	}
	actions = append(actions, "inventory_updated")

	// ── Done ───────────────────────────────────────────────────────────
	actionsJSON, _ := json.Marshal(actions)
	warningsJSON, _ := json.Marshal(warnings)
	status := "success"
	if len(warnings) > 0 {
		status = "success"
	}
	if err := s.imports.UpdateItemResult(ctx, itemID, status, actionsJSON, warningsJSON, fileCount, totalSize, nil); err != nil {
		return fmt.Errorf("update item result: %w", err)
	}

	return nil
}

// syncLegacyArtifacts builds and persists legacy decode artifacts.
// Reuses the same logic as Importer.syncLegacyArtifacts but with local interfaces.
func (s *WebImportService) syncLegacyArtifacts(ctx context.Context, domain sqlstore.Domain, ownerEmail, domainDir, domainURL, source string, force bool) ([]string, []string, error) {
	if s.gens == nil {
		return nil, []string{"generation store is not configured; legacy artifacts sync skipped"}, nil
	}
	if s.prompts != nil {
		if err := s.ensureLegacyPrompt(ctx); err != nil {
			return nil, nil, fmt.Errorf("ensure legacy synthetic prompt: %w", err)
		}
	}

	artifacts, meta, err := BuildLegacyArtifacts(domainDir, domainURL, source)
	if err != nil {
		return nil, nil, fmt.Errorf("build legacy artifacts: %w", err)
	}

	artifactBytes, err := json.Marshal(artifacts)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal legacy artifacts: %w", err)
	}

	gens, err := s.gens.ListByDomain(ctx, domain.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("list domain generations: %w", err)
	}

	legacyGen, hasNonLegacy := findLegacyGeneration(gens)
	if hasNonLegacy && !force {
		return []string{"legacy_artifacts_skipped_non_legacy_exists"}, nil, nil
	}
	if legacyGen != nil {
		existingHash := extractLegacyArtifactHash(legacyGen.Artifacts)
		if existingHash != "" && existingHash == meta.ArtifactHash {
			return []string{"legacy_artifacts_unchanged"}, nil, nil
		}
	}

	now := time.Now().UTC()
	promptID := LegacyDecodePromptID
	logs := []byte(`["legacy decode artifacts synthetic generation"]`)

	var generationID string
	if legacyGen != nil {
		generationID = legacyGen.ID
		if err := s.gens.UpdateFull(ctx, legacyGen.ID, "success", 100, nil, logs, artifactBytes, &now, &now, &promptID); err != nil {
			return nil, nil, fmt.Errorf("update legacy synthetic generation: %w", err)
		}
	} else {
		genID := uuid.NewString()
		generationID = genID
		gen := sqlstore.Generation{
			ID:          genID,
			DomainID:    domain.ID,
			RequestedBy: sqlstore.NullableString(ownerEmail),
			Status:      "success",
			Progress:    100,
			Logs:        logs,
			Artifacts:   artifactBytes,
			StartedAt:   sql.NullTime{Time: now, Valid: true},
			FinishedAt:  sql.NullTime{Time: now, Valid: true},
			PromptID:    sqlstore.NullableString(promptID),
		}
		if err := s.gens.Create(ctx, gen); err != nil {
			return nil, nil, fmt.Errorf("create legacy synthetic generation: %w", err)
		}
	}

	if shouldUpdateLastGeneration(domain, gens) {
		if err := s.domains.SetLastGeneration(ctx, domain.ID, generationID); err != nil {
			return nil, nil, fmt.Errorf("update domain last_generation_id: %w", err)
		}
	}

	if legacyGen != nil {
		return []string{"legacy_artifacts_updated"}, nil, nil
	}
	return []string{"legacy_artifacts_created"}, nil, nil
}

func (s *WebImportService) ensureLegacyPrompt(ctx context.Context) error {
	if s.prompts == nil {
		return nil
	}
	if _, err := s.prompts.Get(ctx, LegacyDecodePromptID); err == nil {
		return nil
	}

	prompt := sqlstore.SystemPrompt{
		ID:       LegacyDecodePromptID,
		Name:     "Legacy Decode v2 (Synthetic)",
		Body:     "Synthetic prompt marker for imported legacy decode artifacts.",
		IsActive: false,
	}
	if err := s.prompts.Create(ctx, prompt); err != nil {
		// Parallel runs may have created it already.
		if _, getErr := s.prompts.Get(ctx, LegacyDecodePromptID); getErr == nil {
			return nil
		}
		return err
	}
	return nil
}

// extractHost returns the hostname from a domain URL (e.g. "https://example.com" -> "example.com").
func extractHost(domainURL string) (string, error) {
	u := strings.TrimSpace(domainURL)
	if !strings.Contains(u, "://") {
		u = "https://" + u
	}
	u = strings.TrimRight(u, "/")
	// Simple extraction — take the part after ://
	parts := strings.SplitN(u, "://", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", fmt.Errorf("cannot extract host from %q", domainURL)
	}
	host := strings.SplitN(parts[1], "/", 2)[0]
	if host == "" {
		return "", fmt.Errorf("empty host in %q", domainURL)
	}
	return host, nil
}
