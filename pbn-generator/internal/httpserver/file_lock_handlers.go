package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

const fileLockTTL = 120 * time.Second

type fileLockAcquireRequest struct {
	Path string `json:"path"`
}

type fileLockDTO struct {
	FilePath  string    `json:"file_path"`
	LockedBy  string    `json:"locked_by"`
	LockedAt  time.Time `json:"locked_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IsOwner   bool      `json:"is_owner,omitempty"`
}

// GET /api/domains/{id}/files/locks — list active locks for domain.
func (s *Server) handleFileLockList(w http.ResponseWriter, r *http.Request, domainID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	locks, err := s.fileLocks.ListByDomain(r.Context(), domainID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list locks")
		return
	}
	dtos := make([]fileLockDTO, 0, len(locks))
	for _, l := range locks {
		dtos = append(dtos, fileLockDTO{
			FilePath:  l.FilePath,
			LockedBy:  l.LockedBy,
			LockedAt:  l.LockedAt,
			ExpiresAt: l.ExpiresAt,
		})
	}
	writeJSON(w, http.StatusOK, dtos)
}

// POST /api/domains/{id}/files/locks — acquire or renew a lock.
func (s *Server) handleFileLockAcquire(w http.ResponseWriter, r *http.Request, domainID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req fileLockAcquireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	lock, isOwner, err := s.fileLocks.Acquire(r.Context(), domainID, req.Path, user.Email, fileLockTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not acquire lock")
		return
	}
	writeJSON(w, http.StatusOK, fileLockDTO{
		FilePath:  lock.FilePath,
		LockedBy:  lock.LockedBy,
		LockedAt:  lock.LockedAt,
		ExpiresAt: lock.ExpiresAt,
		IsOwner:   isOwner,
	})
}

// DELETE /api/domains/{id}/files/locks — release a lock.
func (s *Server) handleFileLockRelease(w http.ResponseWriter, r *http.Request, domainID string) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req fileLockAcquireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	if err := s.fileLocks.Release(r.Context(), domainID, req.Path, user.Email); err != nil {
		writeError(w, http.StatusInternalServerError, "could not release lock")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "released"})
}
