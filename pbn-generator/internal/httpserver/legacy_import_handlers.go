package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
)

func (s *Server) handleLegacyImport(w http.ResponseWriter, r *http.Request, projectID, action string) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	project, err := s.authorizeProject(r.Context(), projectID)
	if err != nil {
		respondAuthzError(w, err, "project not found")
		return
	}
	_ = project

	switch {
	case action == "preview" && r.Method == http.MethodPost:
		s.handleLegacyImportPreview(w, r, projectID)
	case action == "" && r.Method == http.MethodPost:
		s.handleLegacyImportStart(w, r, projectID, user.Email)
	case action == "" && r.Method == http.MethodGet:
		s.handleLegacyImportListJobs(w, r, projectID)
	case action != "" && r.Method == http.MethodGet:
		s.handleLegacyImportGetJob(w, r, projectID, action)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// POST /api/projects/{id}/legacy-import/preview
func (s *Server) handleLegacyImportPreview(w http.ResponseWriter, r *http.Request, projectID string) {
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		DomainIDs []string `json:"domain_ids"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.DomainIDs) == 0 {
		writeError(w, http.StatusBadRequest, "domain_ids required")
		return
	}

	domains, err := s.domains.ListByIDs(r.Context(), body.DomainIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load domains")
		return
	}

	type previewItem struct {
		DomainID           string `json:"domain_id"`
		DomainURL          string `json:"domain_url"`
		HasFiles           bool   `json:"has_files"`
		HasLink            bool   `json:"has_link"`
		LinkAnchor         string `json:"link_anchor,omitempty"`
		LinkTarget         string `json:"link_target,omitempty"`
		HasLegacyArtifacts bool   `json:"has_legacy_artifacts"`
		HasNonLegacyGen    bool   `json:"has_non_legacy_gen"`
	}

	result := make([]previewItem, 0, len(domains))
	for _, d := range domains {
		item := previewItem{
			DomainID:  d.ID,
			DomainURL: d.URL,
			HasFiles:  d.FileCount > 0,
		}
		if d.LinkAnchorText.Valid && d.LinkAcceptorURL.Valid &&
			d.LinkStatus.Valid && strings.ToLower(strings.TrimSpace(d.LinkStatus.String)) == "inserted" {
			item.HasLink = true
			item.LinkAnchor = d.LinkAnchorText.String
			item.LinkTarget = d.LinkAcceptorURL.String
		}

		gens, err := s.generations.ListByDomain(r.Context(), d.ID)
		if err == nil {
			for _, g := range gens {
				pid := strings.TrimSpace(g.PromptID.String)
				if pid == "legacy_decode_v2" {
					item.HasLegacyArtifacts = true
				} else {
					item.HasNonLegacyGen = true
				}
			}
		}
		result = append(result, item)
	}

	writeJSON(w, http.StatusOK, result)
}

// POST /api/projects/{id}/legacy-import
func (s *Server) handleLegacyImportStart(w http.ResponseWriter, r *http.Request, projectID, userEmail string) {
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		DomainIDs []string `json:"domain_ids"`
		Force     bool     `json:"force"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.DomainIDs) == 0 {
		writeError(w, http.StatusBadRequest, "domain_ids required")
		return
	}

	domains, err := s.domains.ListByIDs(r.Context(), body.DomainIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load domains")
		return
	}
	if len(domains) == 0 {
		writeError(w, http.StatusBadRequest, "no valid domains found")
		return
	}

	jobID := uuid.NewString()
	job := sqlstore.LegacyImportJob{
		ID:             jobID,
		ProjectID:      projectID,
		RequestedBy:    userEmail,
		ForceOverwrite: body.Force,
		Status:         "pending",
		TotalItems:     len(domains),
	}
	if err := s.legacyImports.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create import job")
		return
	}

	cfg := s.cfg
	for _, d := range domains {
		itemID := uuid.NewString()
		item := sqlstore.LegacyImportItem{
			ID:        itemID,
			JobID:     jobID,
			DomainID:  d.ID,
			DomainURL: d.URL,
		}
		if err := s.legacyImports.CreateItem(r.Context(), item); err != nil {
			s.logger.Errorf("create legacy import item for domain %s: %v", d.ID, err)
			continue
		}

		task := tasks.NewLegacyImportTask(jobID, itemID, d.ID)
		queueName := tasks.QueueForProject(projectID, cfg.GenQueueShards)
		if _, err := s.tasks.Enqueue(r.Context(), task, asynq.Queue(queueName)); err != nil {
			s.logger.Errorf("enqueue legacy import task for domain %s: %v", d.ID, err)
		}
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

// GET /api/projects/{id}/legacy-import
func (s *Server) handleLegacyImportListJobs(w http.ResponseWriter, r *http.Request, projectID string) {
	jobs, err := s.legacyImports.ListJobsByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list import jobs")
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

// GET /api/projects/{id}/legacy-import/{jobId}
func (s *Server) handleLegacyImportGetJob(w http.ResponseWriter, r *http.Request, projectID, jobID string) {
	job, err := s.legacyImports.GetJob(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusNotFound, "import job not found")
		return
	}
	if job.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "import job not found")
		return
	}

	items, err := s.legacyImports.ListItemsByJob(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list import items")
		return
	}

	type jobDetail struct {
		sqlstore.LegacyImportJob
		Items []sqlstore.LegacyImportItem `json:"items"`
	}

	writeJSON(w, http.StatusOK, jobDetail{
		LegacyImportJob: job,
		Items:           items,
	})
}
