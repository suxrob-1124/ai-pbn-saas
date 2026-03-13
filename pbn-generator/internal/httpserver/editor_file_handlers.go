package httpserver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

func (s *Server) handleDomainFiles(w http.ResponseWriter, r *http.Request, domainID string, parts []string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.siteFiles == nil || s.fileEdits == nil {
		writeError(w, http.StatusInternalServerError, "file storage not configured")
		return
	}
	domain, project, err := s.authorizeDomain(r.Context(), domainID)
	if err != nil {
		respondAuthzError(w, err, "domain not found")
		return
	}

	memberRole := ""
	if !strings.EqualFold(user.Role, "admin") {
		role, err := s.getProjectMemberRole(r.Context(), project.ID, user.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		memberRole = role
	}

	// /api/domains/:id/files
	if len(parts) == 0 || parts[0] == "" {
		switch r.Method {
		case http.MethodGet:
			if !hasProjectPermission(user.Role, memberRole, "viewer") {
				writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
				return
			}
			includeDeleted := false
			if raw := strings.TrimSpace(r.URL.Query().Get("include_deleted")); raw != "" {
				includeDeleted = raw == "1" || strings.EqualFold(raw, "true")
			}
			resp, err := s.loadDomainFilesListCached(r.Context(), domain, includeDeleted)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not list files")
				return
			}
			writeJSON(w, http.StatusOK, resp)
		case http.MethodPost:
			if !hasProjectPermission(user.Role, memberRole, "editor") {
				writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
				return
			}
			s.handleCreateDomainFile(w, r, domain, user.Email)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// File lock endpoints: GET|POST|DELETE /files/locks
	if len(parts) == 1 && strings.EqualFold(parts[0], "locks") {
		switch r.Method {
		case http.MethodGet:
			s.handleFileLockList(w, r, domainID)
		case http.MethodPost:
			s.handleFileLockAcquire(w, r, domainID)
		case http.MethodDelete:
			s.handleFileLockRelease(w, r, domainID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// /api/domains/:id/files/upload
	if len(parts) == 1 && strings.EqualFold(parts[0], "upload") {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		s.handleUploadDomainFile(w, r, domain, user.Email)
		return
	}

	// suffix маршруты /files/:path/meta|history|revert|ai-suggest
	if len(parts) >= 2 {
		last := strings.ToLower(parts[len(parts)-1])
		switch last {
		case "meta":
			if !hasProjectPermission(user.Role, memberRole, "viewer") {
				writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
				return
			}
			rawPath, err := joinURLPath(parts[:len(parts)-1])
			if err != nil {
				writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
				return
			}
			cleanPath, err := normalizeEditorPath(rawPath)
			if err != nil {
				writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
				return
			}
			s.handleFileMeta(w, r, domain, cleanPath)
			return
		case "revert":
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if !hasProjectPermission(user.Role, memberRole, "editor") {
				writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
				return
			}
			rawPath, err := joinURLPath(parts[:len(parts)-1])
			if err != nil {
				writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
				return
			}
			cleanPath, err := normalizeEditorPath(rawPath)
			if err != nil {
				writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
				return
			}
			s.handleRevertFile(w, r, domain, cleanPath, user.Email)
			return
		case "restore":
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if !hasProjectPermission(user.Role, memberRole, "editor") {
				writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
				return
			}
			rawPath, err := joinURLPath(parts[:len(parts)-1])
			if err != nil {
				writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
				return
			}
			cleanPath, err := normalizeEditorPath(rawPath)
			if err != nil {
				writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
				return
			}
			s.handleRestoreFile(w, r, domain, cleanPath, user.Email)
			return
		case "history":
			if !hasProjectPermission(user.Role, memberRole, "viewer") {
				writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
				return
			}
			// Legacy compatibility: /files/:fileId/history
			if len(parts) == 2 {
				rawFirst, err := url.PathUnescape(parts[0])
				if err == nil {
					if cleanFirst, serr := normalizeEditorPath(rawFirst); serr == nil {
						if fileByPath, ferr := s.siteFiles.GetByPath(r.Context(), domain.ID, cleanFirst); ferr == nil && fileByPath != nil {
							s.handleFileHistoryByPath(w, r, domain.ID, cleanFirst)
							return
						}
					}
				}
				fileID := strings.TrimSpace(rawFirst)
				if fileID == "" {
					writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid file id", nil)
					return
				}
				s.handleFileHistory(w, r, domain.ID, fileID)
				return
			}
			rawPath, err := joinURLPath(parts[:len(parts)-1])
			if err != nil {
				writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
				return
			}
			cleanPath, err := normalizeEditorPath(rawPath)
			if err != nil {
				writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
				return
			}
			s.handleFileHistoryByPath(w, r, domain.ID, cleanPath)
			return
		case "ai-suggest":
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if !hasProjectPermission(user.Role, memberRole, "editor") {
				writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
				return
			}
			rawPath, err := joinURLPath(parts[:len(parts)-1])
			if err != nil {
				writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
				return
			}
			cleanPath, err := normalizeEditorPath(rawPath)
			if err != nil {
				writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
				return
			}
			s.handleDomainEditorAISuggest(w, r, domain, project, cleanPath, user.Email)
			return
		}
	}

	rawPath, err := joinURLPath(parts)
	if err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
		return
	}
	cleanPath, err := normalizeEditorPath(rawPath)
	if err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid path", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !hasProjectPermission(user.Role, memberRole, "viewer") {
			writeError(w, http.StatusForbidden, "insufficient permissions: viewer role required")
			return
		}
		s.handleGetFile(w, r, domain, cleanPath)
	case http.MethodPut:
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		s.handleUpdateFile(w, r, domain, cleanPath, user.Email)
	case http.MethodPatch:
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		s.handleMoveFile(w, r, domain, cleanPath)
	case http.MethodDelete:
		if !hasProjectPermission(user.Role, memberRole, "editor") {
			writeError(w, http.StatusForbidden, "insufficient permissions: editor role required")
			return
		}
		hardDelete := false
		if raw := strings.TrimSpace(r.URL.Query().Get("hard")); raw != "" {
			hardDelete = raw == "1" || strings.EqualFold(raw, "true")
		}
		recursiveDelete := false
		if raw := strings.TrimSpace(r.URL.Query().Get("recursive")); raw != "" {
			recursiveDelete = raw == "1" || strings.EqualFold(raw, "true")
		}
		if hardDelete && !strings.EqualFold(user.Role, "admin") && !strings.EqualFold(memberRole, "owner") && !strings.EqualFold(memberRole, "manager") {
			writeError(w, http.StatusForbidden, "hard delete requires owner or admin role")
			return
		}
		s.handleDeleteFile(w, r, domain, cleanPath, user.Email, hardDelete, recursiveDelete)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) buildDomainFilesListResponse(ctx context.Context, domain sqlstore.Domain, includeDeleted bool) ([]fileDTO, error) {
	files, err := s.siteFiles.List(ctx, domain.ID)
	if err != nil {
		return nil, err
	}
	resp := make([]fileDTO, 0, len(files)+16)
	indexByPath := make(map[string]struct{}, len(files)+16)
	for _, f := range files {
		item := toFileDTO(domain, f)
		indexByPath[item.Path] = struct{}{}
		resp = append(resp, item)
	}
	if includeDeleted {
		deletedFiles, err := s.siteFiles.ListDeleted(ctx, domain.ID)
		if err != nil {
			return nil, err
		}
		for _, f := range deletedFiles {
			item := toFileDTO(domain, f)
			indexByPath[item.Path] = struct{}{}
			resp = append(resp, item)
		}
	}
	if dirs, err := s.listDomainDirs(ctx, domain); err == nil {
		now := time.Now().UTC()
		for _, dirPath := range dirs {
			if _, exists := indexByPath[dirPath]; exists {
				continue
			}
			indexByPath[dirPath] = struct{}{}
			resp = append(resp, fileDTO{
				ID:         "dir:" + dirPath,
				Path:       dirPath,
				Size:       0,
				MimeType:   "inode/directory",
				Version:    1,
				IsEditable: false,
				IsBinary:   false,
				UpdatedAt:  now,
			})
		}
	} else if s.logger != nil {
		s.logger.Warnf("could not list directories for domain %s: %v", domain.URL, err)
	}
	return resp, nil
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, relPath string) {
	file, err := s.siteFiles.GetByPath(r.Context(), domain.ID, relPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	content, mimeType, err := s.readDomainFileContent(r.Context(), domain, relPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		if errors.Is(err, fs.ErrPermission) {
			writeError(w, http.StatusForbidden, "permission denied to read file")
			return
		}
		if isRequestCanceled(err) {
			// Client aborted the request — not a server error, no response needed.
			return
		}
		if s.logger != nil {
			dctx := s.makeDomainFSContext(domain)
			s.logger.Errorf(
				"read file failed: domain_id=%s domain_url=%s mode=%s server_id=%s published_path=%s path=%s err=%v",
				domain.ID,
				domain.URL,
				dctx.DeploymentMode,
				dctx.ServerID,
				dctx.PublishedPath,
				relPath,
				err,
			)
		}
		writeError(w, http.StatusInternalServerError, "could not read file")
		return
	}
	if r.URL.Query().Get("raw") == "1" {
		if strings.HasPrefix(mimeType, "text/") && !strings.Contains(strings.ToLower(mimeType), "charset=") {
			mimeType = mimeType + "; charset=utf-8"
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(content))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"content":  content,
		"mimeType": mimeType,
		"version":  file.Version,
	})
}

func (s *Server) handleCreateDomainFile(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, editedBy string) {
	if !ensureJSON(w, r) {
		return
	}
	defer r.Body.Close()
	var body struct {
		Path     string `json:"path"`
		Kind     string `json:"kind"`
		Content  string `json:"content"`
		MimeType string `json:"mime_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid body", nil)
		return
	}
	kind := strings.ToLower(strings.TrimSpace(body.Kind))
	if kind == "" {
		kind = "file"
	}
	if kind != "file" && kind != "dir" {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "kind must be file or dir", nil)
		return
	}
	cleanPath, err := normalizeEditorWritablePath(body.Path)
	if err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrForbiddenPath, err.Error(), nil)
		return
	}
	if _, err := s.statDomainPathInBackend(r.Context(), domain, cleanPath); err == nil {
		writeError(w, http.StatusConflict, "path already exists")
		return
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, "could not inspect path")
		return
	}
	if kind == "dir" {
		if err := s.ensureDomainDirInBackend(r.Context(), domain, cleanPath); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create directory")
			return
		}
		s.invalidateDomainFilesCache(r.Context(), domain.ID)
		writeJSON(w, http.StatusCreated, map[string]any{"status": "created", "kind": "dir", "path": cleanPath})
		return
	}
	newBytes := []byte(body.Content)
	detected, err := validateEditorMimeAndPayload(cleanPath, strings.TrimSpace(body.MimeType), newBytes)
	if err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrAssetValidationFail, err.Error(), nil)
		return
	}
	if err := s.writeDomainFileBytesToBackend(r.Context(), domain, cleanPath, newBytes); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			writeError(w, http.StatusForbidden, "permission denied to write file")
			return
		}
		if s.logger != nil {
			dctx := s.makeDomainFSContext(domain)
			s.logger.Errorf(
				"write file failed (create): domain_id=%s domain_url=%s mode=%s server_id=%s published_path=%s path=%s err=%v",
				domain.ID,
				domain.URL,
				dctx.DeploymentMode,
				dctx.ServerID,
				dctx.PublishedPath,
				cleanPath,
				err,
			)
		}
		writeError(w, http.StatusInternalServerError, "could not write file")
		return
	}
	hash := sha256.Sum256(newBytes)
	file := sqlstore.SiteFile{
		ID:           uuid.NewString(),
		DomainID:     domain.ID,
		Path:         cleanPath,
		ContentHash:  sql.NullString{String: hex.EncodeToString(hash[:]), Valid: true},
		SizeBytes:    int64(len(newBytes)),
		MimeType:     detected,
		Version:      1,
		LastEditedBy: sqlstore.NullableString(editedBy),
	}
	if err := s.siteFiles.Create(r.Context(), file); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create file metadata")
		return
	}
	_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(&file, newBytes, "manual", editedBy, "file created"))
	_ = s.fileEdits.Create(r.Context(), sqlstore.FileEdit{
		ID:               uuid.NewString(),
		FileID:           file.ID,
		EditedBy:         editedBy,
		EditType:         "create",
		EditDescription:  sql.NullString{String: "file created", Valid: true},
		ContentAfterHash: sql.NullString{String: hex.EncodeToString(hash[:]), Valid: true},
	})
	s.invalidateDomainFilesCache(r.Context(), domain.ID)
	writeJSON(w, http.StatusCreated, toFileDTO(domain, file))
}

func (s *Server) handleUploadDomainFile(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, editedBy string) {
	const maxUploadBytes = 20 << 20 // 20MB
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid multipart form", nil)
		return
	}
	uploaded, header, err := r.FormFile("file")
	if err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "file is required", nil)
		return
	}
	defer uploaded.Close()
	rawPath := strings.TrimSpace(r.FormValue("path"))
	if strings.HasSuffix(rawPath, "/") || strings.HasSuffix(rawPath, "\\") {
		rawPath = path.Join(rawPath, header.Filename)
	}
	if rawPath == "" {
		rawPath = header.Filename
	}
	cleanPath, err := normalizeEditorWritablePath(rawPath)
	if err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrForbiddenPath, err.Error(), nil)
		return
	}
	content, err := io.ReadAll(io.LimitReader(uploaded, maxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read upload")
		return
	}
	if int64(len(content)) > maxUploadBytes {
		writeEditorError(w, http.StatusBadRequest, editorErrAssetValidationFail, "file too large", nil)
		return
	}
	existing, getErr := s.siteFiles.GetByPath(r.Context(), domain.ID, cleanPath)
	if getErr != nil && !errors.Is(getErr, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	var oldContent []byte
	if existing != nil {
		oldContent, _ = s.readDomainFileBytesFromBackend(r.Context(), domain, cleanPath)
		_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(existing, oldContent, "manual", editedBy, "baseline before upload"))
	}
	if err := s.writeDomainFileBytesToBackend(r.Context(), domain, cleanPath, content); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			writeError(w, http.StatusForbidden, "permission denied to write file")
			return
		}
		if s.logger != nil {
			dctx := s.makeDomainFSContext(domain)
			s.logger.Errorf(
				"write file failed (upload): domain_id=%s domain_url=%s mode=%s server_id=%s published_path=%s path=%s err=%v",
				domain.ID,
				domain.URL,
				dctx.DeploymentMode,
				dctx.ServerID,
				dctx.PublishedPath,
				cleanPath,
				err,
			)
		}
		writeError(w, http.StatusInternalServerError, "could not save upload")
		return
	}
	detected, err := validateEditorMimeAndPayload(cleanPath, "", content)
	if err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrAssetValidationFail, err.Error(), nil)
		return
	}
	if existing == nil {
		hash := sha256.Sum256(content)
		file := sqlstore.SiteFile{
			ID:           uuid.NewString(),
			DomainID:     domain.ID,
			Path:         cleanPath,
			ContentHash:  sql.NullString{String: hex.EncodeToString(hash[:]), Valid: true},
			SizeBytes:    int64(len(content)),
			MimeType:     detected,
			Version:      1,
			LastEditedBy: sqlstore.NullableString(editedBy),
		}
		if err := s.siteFiles.Create(r.Context(), file); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create file metadata")
			return
		}
		_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(&file, content, "manual", editedBy, "uploaded file"))
		s.invalidateDomainFilesCache(r.Context(), domain.ID)
		writeJSON(w, http.StatusCreated, toFileDTO(domain, file))
		return
	}
	if err := s.siteFiles.Update(r.Context(), existing.ID, content); err != nil {
		writeError(w, http.StatusInternalServerError, "could not update file metadata")
		return
	}
	_ = s.siteFiles.SetLastEditedBy(r.Context(), existing.ID, sqlstore.NullableString(editedBy))
	updated, _ := s.siteFiles.Get(r.Context(), existing.ID)
	if updated != nil {
		_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(updated, content, "manual", editedBy, "uploaded update"))
	}
	beforeHash := sha256.Sum256(oldContent)
	afterHash := sha256.Sum256(content)
	_ = s.fileEdits.Create(r.Context(), sqlstore.FileEdit{
		ID:                uuid.NewString(),
		FileID:            existing.ID,
		EditedBy:          editedBy,
		EditType:          "upload_update",
		ContentBeforeHash: sql.NullString{String: hex.EncodeToString(beforeHash[:]), Valid: len(oldContent) > 0},
		ContentAfterHash:  sql.NullString{String: hex.EncodeToString(afterHash[:]), Valid: true},
	})
	s.invalidateDomainFilesCache(r.Context(), domain.ID)
	if updated != nil {
		writeJSON(w, http.StatusOK, toFileDTO(domain, *updated))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "uploaded"})
}

func (s *Server) handleUpdateFile(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, relPath, editedBy string) {
	if !ensureJSON(w, r) {
		return
	}
	defer r.Body.Close()
	var body struct {
		Content         string `json:"content"`
		Description     string `json:"description"`
		ExpectedVersion *int   `json:"expected_version"`
		ExpectedPath    string `json:"expected_path"`
		Source          string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid body", nil)
		return
	}
	if strings.TrimSpace(body.ExpectedPath) != "" {
		expectedPath, err := normalizeEditorPath(body.ExpectedPath)
		if err != nil {
			writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid expected_path", nil)
			return
		}
		if expectedPath != relPath {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":         "path conflict",
				"expected_path": expectedPath,
				"actual_path":   relPath,
			})
			return
		}
	}
	file, err := s.siteFiles.GetByPath(r.Context(), domain.ID, relPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	if body.ExpectedVersion != nil && *body.ExpectedVersion != file.Version {
		currentHash := ""
		if file.ContentHash.Valid {
			currentHash = file.ContentHash.String
		}
		resp := map[string]any{
			"error":           "version conflict",
			"current_version": file.Version,
			"current_hash":    currentHash,
			"updated_at":      file.UpdatedAt,
		}
		if file.LastEditedBy.Valid {
			resp["updated_by"] = file.LastEditedBy.String
		}
		writeJSON(w, http.StatusConflict, resp)
		return
	}
	oldContent, err := s.readDomainFileBytesFromBackend(r.Context(), domain, relPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		if errors.Is(err, fs.ErrPermission) {
			writeError(w, http.StatusForbidden, "permission denied to read file")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not read file")
		return
	}
	_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(file, oldContent, "manual", editedBy, "baseline before update"))
	newBytes := []byte(body.Content)
	detected := detectMimeType(relPath, newBytes)
	if err := validateMimeType(relPath, detected, file.MimeType); err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, err.Error(), nil)
		return
	}
	if err := s.writeDomainFileBytesToBackend(r.Context(), domain, relPath, newBytes); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			writeError(w, http.StatusForbidden, "permission denied to write file")
			return
		}
		if s.logger != nil {
			dctx := s.makeDomainFSContext(domain)
			s.logger.Errorf(
				"write file failed (update): domain_id=%s domain_url=%s mode=%s server_id=%s published_path=%s path=%s err=%v",
				domain.ID,
				domain.URL,
				dctx.DeploymentMode,
				dctx.ServerID,
				dctx.PublishedPath,
				relPath,
				err,
			)
		}
		writeError(w, http.StatusInternalServerError, "could not write file")
		return
	}
	if err := s.siteFiles.Update(r.Context(), file.ID, newBytes); err != nil {
		if s.logger != nil {
			s.logger.Errorf(
				"update file metadata failed: domain_id=%s file_id=%s path=%s err=%v",
				domain.ID,
				file.ID,
				relPath,
				err,
			)
		}
		writeError(w, http.StatusInternalServerError, "could not update file metadata")
		return
	}
	_ = s.siteFiles.SetLastEditedBy(r.Context(), file.ID, sqlstore.NullableString(editedBy))
	updatedFile, _ := s.siteFiles.Get(r.Context(), file.ID)
	if updatedFile != nil {
		_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(updatedFile, newBytes, normalizeRevisionSource(body.Source), editedBy, strings.TrimSpace(body.Description)))
	}
	beforeHash := sha256.Sum256(oldContent)
	afterHash := sha256.Sum256(newBytes)
	edit := sqlstore.FileEdit{
		ID:                uuid.NewString(),
		FileID:            file.ID,
		EditedBy:          editedBy,
		ContentBeforeHash: sql.NullString{String: hex.EncodeToString(beforeHash[:]), Valid: true},
		ContentAfterHash:  sql.NullString{String: hex.EncodeToString(afterHash[:]), Valid: true},
		EditType:          normalizeRevisionSource(body.Source),
	}
	if strings.TrimSpace(body.Description) != "" {
		edit.EditDescription = sql.NullString{String: strings.TrimSpace(body.Description), Valid: true}
	}
	if err := s.fileEdits.Create(r.Context(), edit); err != nil {
		writeError(w, http.StatusInternalServerError, "could not log file edit")
		return
	}
	resp := map[string]any{"status": "updated"}
	if updatedFile != nil {
		resp["version"] = updatedFile.Version
	}
	s.invalidateDomainFilesCache(r.Context(), domain.ID)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMoveFile(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, oldPath string) {
	if !ensureJSON(w, r) {
		return
	}
	defer r.Body.Close()
	var body struct {
		Op      string `json:"op"`
		NewPath string `json:"new_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "invalid body", nil)
		return
	}
	if strings.ToLower(strings.TrimSpace(body.Op)) != "move" {
		writeEditorError(w, http.StatusBadRequest, editorErrInvalidFormat, "op must be move", nil)
		return
	}
	newPath, err := normalizeEditorWritablePath(body.NewPath)
	if err != nil {
		writeEditorError(w, http.StatusBadRequest, editorErrForbiddenPath, err.Error(), nil)
		return
	}
	if oldPath == newPath {
		writeJSON(w, http.StatusOK, map[string]string{"status": "moved"})
		return
	}
	if _, err := s.statDomainPathInBackend(r.Context(), domain, newPath); err == nil {
		writeError(w, http.StatusConflict, "destination already exists")
		return
	}
	if err := s.moveDomainPathInBackend(r.Context(), domain, oldPath, newPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not move file")
		return
	}
	if file, err := s.siteFiles.GetByPath(r.Context(), domain.ID, oldPath); err == nil && file != nil {
		if err := s.siteFiles.Move(r.Context(), file.ID, newPath); err != nil {
			writeError(w, http.StatusInternalServerError, "could not update file metadata")
			return
		}
	} else {
		files, err := s.siteFiles.List(r.Context(), domain.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list files")
			return
		}
		prefix := oldPath + "/"
		for _, f := range files {
			if !strings.HasPrefix(f.Path, prefix) {
				continue
			}
			targetPath := newPath + strings.TrimPrefix(f.Path, oldPath)
			if err := s.siteFiles.Move(r.Context(), f.ID, targetPath); err != nil {
				writeError(w, http.StatusInternalServerError, "could not move nested file metadata")
				return
			}
		}
	}
	s.invalidateDomainFilesCache(r.Context(), domain.ID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "moved", "path": newPath})
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, relPath, deletedBy string, hardDelete bool, recursiveDelete bool) {
	softDeleteSingle := func(file sqlstore.SiteFile) error {
		if content, err := s.readDomainFileBytesFromBackend(r.Context(), domain, file.Path); err == nil {
			_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(&file, content, "manual", deletedBy, "baseline before delete"))
			beforeHash := sha256.Sum256(content)
			_ = s.fileEdits.Create(r.Context(), sqlstore.FileEdit{
				ID:                uuid.NewString(),
				FileID:            file.ID,
				EditedBy:          deletedBy,
				EditType:          "delete",
				EditDescription:   sql.NullString{String: "soft-delete", Valid: true},
				ContentBeforeHash: sql.NullString{String: hex.EncodeToString(beforeHash[:]), Valid: true},
			})
		}
		if err := s.deleteDomainPathInBackend(r.Context(), domain, file.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return s.siteFiles.SoftDelete(r.Context(), file.ID, sqlstore.NullableString(deletedBy), sqlstore.NullableString("deleted from editor"))
	}
	hardDeleteSingle := func(file sqlstore.SiteFile) error {
		if err := s.deleteDomainPathInBackend(r.Context(), domain, file.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return s.siteFiles.Delete(r.Context(), file.ID)
	}

	if file, err := s.siteFiles.GetByPath(r.Context(), domain.ID, relPath); err == nil && file != nil && !strings.EqualFold(strings.TrimSpace(file.MimeType), "inode/directory") {
		if hardDelete {
			if err := hardDeleteSingle(*file); err != nil {
				writeError(w, http.StatusInternalServerError, "could not hard-delete file")
				return
			}
		} else {
			if err := softDeleteSingle(*file); err != nil {
				writeError(w, http.StatusInternalServerError, "could not delete file")
				return
			}
		}
		// Clean up empty parent directories left after file deletion
		s.cleanupEmptyParentDirs(r.Context(), domain, relPath)
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "deleted",
			"mode":   map[bool]string{true: "hard", false: "soft"}[hardDelete],
		})
		s.invalidateDomainFilesCache(r.Context(), domain.ID)
		return
	}

	files, err := s.siteFiles.List(r.Context(), domain.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list files")
		return
	}
	info, statErr := s.statDomainPathInBackend(r.Context(), domain, relPath)
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, "could not inspect path")
		return
	}
	if statErr == nil && !info.IsDir {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	dirMetadata := make([]sqlstore.SiteFile, 0, 1)
	prefix := relPath + "/"
	nested := make([]sqlstore.SiteFile, 0, 8)
	for _, f := range files {
		if f.Path == relPath && strings.EqualFold(strings.TrimSpace(f.MimeType), "inode/directory") {
			dirMetadata = append(dirMetadata, f)
			continue
		}
		if !strings.HasPrefix(f.Path, prefix) {
			continue
		}
		nested = append(nested, f)
	}

	isDirectoryTarget := (statErr == nil && info.IsDir) || len(nested) > 0 || len(dirMetadata) > 0
	if !isDirectoryTarget {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	if len(nested) > 0 && !recursiveDelete {
		writeError(w, http.StatusConflict, "directory is not empty; use recursive=1")
		return
	}

	if len(nested) == 0 && statErr == nil && info.IsDir && !recursiveDelete {
		entries, readErr := s.readDomainDirInBackend(r.Context(), domain, relPath)
		if readErr != nil {
			writeError(w, http.StatusInternalServerError, "could not inspect directory")
			return
		}
		if len(entries) > 0 {
			writeError(w, http.StatusConflict, "directory is not empty; use recursive=1")
			return
		}
	}

	for _, f := range nested {
		if hardDelete {
			if err := hardDeleteSingle(f); err != nil {
				writeError(w, http.StatusInternalServerError, "could not hard-delete directory files")
				return
			}
		} else {
			if err := softDeleteSingle(f); err != nil {
				writeError(w, http.StatusInternalServerError, "could not delete directory files")
				return
			}
		}
	}

	for _, f := range dirMetadata {
		if err := s.siteFiles.Delete(r.Context(), f.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete directory metadata")
			return
		}
	}

	if statErr == nil && info.IsDir {
		if recursiveDelete {
			if err := s.deleteDomainPathRecursiveInBackend(r.Context(), domain, relPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
				if s.logger != nil {
					s.logger.Errorf("could not remove directory %s recursively for domain %s: %v", relPath, domain.URL, err)
				}
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not remove directory: %v", err))
				return
			}
		} else {
			if err := s.deleteDomainPathInBackend(r.Context(), domain, relPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
				if s.logger != nil {
					s.logger.Errorf("could not remove directory %s for domain %s: %v", relPath, domain.URL, err)
				}
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not remove directory: %v", err))
				return
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "deleted",
		"kind":   "dir",
		"mode":   map[bool]string{true: "hard", false: "soft"}[hardDelete],
		"count":  len(nested),
	})
	s.invalidateDomainFilesCache(r.Context(), domain.ID)
}

func (s *Server) handleRestoreFile(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, relPath, editedBy string) {
	file, err := s.siteFiles.GetByPathAny(r.Context(), domain.ID, relPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found in trash")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	if !file.DeletedAt.Valid {
		writeError(w, http.StatusConflict, "file is already active")
		return
	}
	if active, err := s.siteFiles.GetByPath(r.Context(), domain.ID, relPath); err == nil && active != nil && active.ID != file.ID {
		writeError(w, http.StatusConflict, "active file with same path already exists")
		return
	}
	revisions, err := s.fileEdits.ListRevisionsByFile(r.Context(), file.ID, 1)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load file revisions")
		return
	}
	if len(revisions) == 0 {
		writeError(w, http.StatusConflict, "cannot restore file without snapshots")
		return
	}
	latest := revisions[0]
	if err := s.writeDomainFileBytesToBackend(r.Context(), domain, relPath, latest.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "could not restore file")
		return
	}
	if err := s.siteFiles.Restore(r.Context(), file.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not restore file metadata")
		return
	}
	if err := s.siteFiles.Update(r.Context(), file.ID, latest.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "could not update restored file metadata")
		return
	}
	_ = s.siteFiles.SetLastEditedBy(r.Context(), file.ID, sqlstore.NullableString(editedBy))
	updated, _ := s.siteFiles.Get(r.Context(), file.ID)
	if updated != nil {
		_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(updated, latest.Content, "revert", editedBy, "restored from trash"))
	}
	_ = s.fileEdits.Create(r.Context(), sqlstore.FileEdit{
		ID:              uuid.NewString(),
		FileID:          file.ID,
		EditedBy:        editedBy,
		EditType:        "restore",
		EditDescription: sql.NullString{String: "restored from trash", Valid: true},
	})
	resp := map[string]any{"status": "restored"}
	if updated != nil {
		resp["version"] = updated.Version
	}
	s.invalidateDomainFilesCache(r.Context(), domain.ID)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleFileMeta(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, relPath string) {
	file, err := s.siteFiles.GetByPath(r.Context(), domain.ID, relPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	writeJSON(w, http.StatusOK, toFileDTO(domain, *file))
}

func (s *Server) handleRevertFile(w http.ResponseWriter, r *http.Request, domain sqlstore.Domain, relPath, editedBy string) {
	if !ensureJSON(w, r) {
		return
	}
	defer r.Body.Close()
	var body struct {
		RevisionID  string `json:"revision_id"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	revisionID := strings.TrimSpace(body.RevisionID)
	if revisionID == "" {
		writeError(w, http.StatusBadRequest, "revision_id is required")
		return
	}
	file, err := s.siteFiles.GetByPath(r.Context(), domain.ID, relPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	rev, err := s.fileEdits.GetRevision(r.Context(), revisionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "revision not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load revision")
		return
	}
	if rev.FileID != file.ID {
		writeError(w, http.StatusBadRequest, "revision does not belong to file")
		return
	}
	oldContent, _ := s.readDomainFileBytesFromBackend(r.Context(), domain, relPath)
	_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(file, oldContent, "manual", editedBy, "baseline before revert"))
	if err := s.writeDomainFileBytesToBackend(r.Context(), domain, relPath, rev.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "could not write file")
		return
	}
	if err := s.siteFiles.Update(r.Context(), file.ID, rev.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "could not update file metadata")
		return
	}
	_ = s.siteFiles.SetLastEditedBy(r.Context(), file.ID, sqlstore.NullableString(editedBy))
	updated, _ := s.siteFiles.Get(r.Context(), file.ID)
	if updated != nil {
		desc := strings.TrimSpace(body.Description)
		if desc == "" {
			desc = fmt.Sprintf("revert to revision %s", revisionID)
		}
		_ = s.fileEdits.CreateRevision(r.Context(), buildRevision(updated, rev.Content, "revert", editedBy, desc))
	}
	beforeHash := sha256.Sum256(oldContent)
	afterHash := sha256.Sum256(rev.Content)
	_ = s.fileEdits.Create(r.Context(), sqlstore.FileEdit{
		ID:                uuid.NewString(),
		FileID:            file.ID,
		EditedBy:          editedBy,
		EditType:          "revert",
		EditDescription:   sql.NullString{String: strings.TrimSpace(body.Description), Valid: strings.TrimSpace(body.Description) != ""},
		ContentBeforeHash: sql.NullString{String: hex.EncodeToString(beforeHash[:]), Valid: len(oldContent) > 0},
		ContentAfterHash:  sql.NullString{String: hex.EncodeToString(afterHash[:]), Valid: true},
	})
	resp := map[string]any{"status": "updated"}
	if updated != nil {
		resp["version"] = updated.Version
	}
	s.invalidateDomainFilesCache(r.Context(), domain.ID)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleFileHistory(w http.ResponseWriter, r *http.Request, domainID, fileID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	file, err := s.siteFiles.Get(r.Context(), fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	if file.DomainID != domainID {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	list, err := s.fileEdits.ListByFile(r.Context(), fileID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list file history")
		return
	}
	resp := make([]fileEditDTO, 0, len(list))
	for _, e := range list {
		item := fileEditDTO{
			ID:        e.ID,
			EditedBy:  e.EditedBy,
			EditType:  e.EditType,
			CreatedAt: e.CreatedAt,
		}
		if e.EditDescription.Valid {
			item.Description = e.EditDescription.String
		}
		resp = append(resp, item)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleFileHistoryByPath(w http.ResponseWriter, r *http.Request, domainID, relPath string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	file, err := s.siteFiles.GetByPath(r.Context(), domainID, relPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	// Fetch more than requested to account for agent_snapshot:* revisions that
	// will be filtered below — snapshot count is bounded by agent session count
	// per file (typically ≤5), so 2× is a safe over-fetch.
	fetchLimit := limit * 2
	if fetchLimit > 500 {
		fetchLimit = 500
	}
	allRevisions, err := s.fileEdits.ListRevisionsByFile(r.Context(), file.ID, fetchLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list file revisions")
		return
	}
	// Filter out internal rollback baseline revisions — they are implementation
	// details, not user-visible edit history — then cap at the requested limit.
	revisions := make([]sqlstore.FileRevision, 0, len(allRevisions))
	for _, rev := range allRevisions {
		if !strings.HasPrefix(rev.Source, "agent_snapshot:") {
			revisions = append(revisions, rev)
		}
	}
	if len(revisions) > limit {
		revisions = revisions[:limit]
	}
	resp := make([]fileRevisionDTO, 0, len(revisions))
	for _, rev := range revisions {
		item := fileRevisionDTO{
			ID:          rev.ID,
			FileID:      rev.FileID,
			Version:     rev.Version,
			EditedBy:    rev.EditedBy,
			Source:      rev.Source,
			ContentHash: rev.ContentHash,
			SizeBytes:   rev.SizeBytes,
			MimeType:    rev.MimeType,
			CreatedAt:   rev.CreatedAt,
		}
		if rev.Description.Valid {
			item.Description = rev.Description.String
		}
		if !isBinaryMimeType(rev.MimeType) {
			item.Content = string(rev.Content)
		}
		resp = append(resp, item)
	}
	writeJSON(w, http.StatusOK, resp)
}
