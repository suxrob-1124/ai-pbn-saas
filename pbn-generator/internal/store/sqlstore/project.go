package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Project struct {
	ID                string
	UserEmail         string
	Name              string
	TargetCountry     string
	TargetLanguage    string
	Timezone          sql.NullString
	GlobalBlacklist   []byte
	DefaultServerID   sql.NullString
	Status            string
	IndexCheckEnabled bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         sql.NullTime
	DeletedBy         sql.NullString
}

type Domain struct {
	ID                 string
	ProjectID          string
	ServerID           sql.NullString
	URL                string
	MainKeyword        string
	TargetCountry      string
	TargetLanguage     string
	ExcludeDomains     sql.NullString
	SpecificBlacklist  []byte
	Status             string
	LastGenerationID   sql.NullString
	LastSuccessGenID   sql.NullString
	PublishedAt        sql.NullTime
	PublishedPath      sql.NullString
	FileCount          int
	TotalSizeBytes     int64
	DeploymentMode     sql.NullString
	SiteOwner          sql.NullString
	InventoryStatus    sql.NullString
	InventoryCheckedAt sql.NullTime
	InventoryError     sql.NullString
	LinkAnchorText     sql.NullString
	LinkAcceptorURL    sql.NullString
	LinkStatus         sql.NullString
	LinkUpdatedAt      sql.NullTime
	LinkLastTaskID     sql.NullString
	LinkFilePath       sql.NullString
	LinkAnchorSnapshot sql.NullString
	LinkReadyAt        sql.NullTime
	IndexCheckEnabled  bool
	GenerationType     string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          sql.NullTime
	DeletedBy          sql.NullString
	DeleteBatch        sql.NullString
}

type Generation struct {
	ID               string
	DomainID         string
	RequestedBy      sql.NullString
	Status           string
	Progress         int
	Error            sql.NullString
	Logs             []byte
	Artifacts        []byte
	ArtifactsSummary []byte
	CheckpointData   []byte // JSONB checkpoint data
	Attempts         int
	Retryable        bool
	NextRetryAt      sql.NullTime
	LastErrorAt      sql.NullTime
	StartedAt        sql.NullTime
	FinishedAt       sql.NullTime
	PromptID         sql.NullString
	GenerationType   string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ProjectMember struct {
	ProjectID string
	UserEmail string
	Role      string
	CreatedAt time.Time
}

type ProjectStore struct {
	db *sql.DB
}

func NewProjectStore(db *sql.DB) *ProjectStore {
	return &ProjectStore{db: db}
}

func (s *ProjectStore) Create(ctx context.Context, p Project) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO projects(id, user_email, name, target_country, target_language, timezone, global_blacklist, default_server_id, status, created_at, updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())`, p.ID, p.UserEmail, p.Name, p.TargetCountry, p.TargetLanguage, nullableString(p.Timezone), nullableBytes(p.GlobalBlacklist), nullableString(p.DefaultServerID), p.Status)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	return nil
}

func (s *ProjectStore) ListByUser(ctx context.Context, email string) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_email, name, target_country, target_language, timezone, global_blacklist, default_server_id, status, index_check_enabled, created_at, updated_at, deleted_at, deleted_by
FROM projects
WHERE deleted_at IS NULL AND (user_email=$1
   OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = projects.id AND pm.user_email = $1))
ORDER BY updated_at DESC`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Project
	for rows.Next() {
		var p Project
		var gb sql.NullString
		if err := rows.Scan(&p.ID, &p.UserEmail, &p.Name, &p.TargetCountry, &p.TargetLanguage, &p.Timezone, &gb, &p.DefaultServerID, &p.Status, &p.IndexCheckEnabled, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &p.DeletedBy); err != nil {
			return nil, err
		}
		if gb.Valid {
			p.GlobalBlacklist = []byte(gb.String)
		}
		res = append(res, p)
	}
	return res, rows.Err()
}

func (s *ProjectStore) Get(ctx context.Context, id, email string) (Project, error) {
	var p Project
	var gb sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, user_email, name, target_country, target_language, timezone, global_blacklist, default_server_id, status, index_check_enabled, created_at, updated_at, deleted_at, deleted_by
FROM projects
WHERE id=$1 AND deleted_at IS NULL AND (user_email=$2 OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = projects.id AND pm.user_email = $2))`, id, email).
		Scan(&p.ID, &p.UserEmail, &p.Name, &p.TargetCountry, &p.TargetLanguage, &p.Timezone, &gb, &p.DefaultServerID, &p.Status, &p.IndexCheckEnabled, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &p.DeletedBy)
	if err != nil {
		return Project{}, err
	}
	if gb.Valid {
		p.GlobalBlacklist = []byte(gb.String)
	}
	return p, nil
}

func (s *ProjectStore) Update(ctx context.Context, p Project) error {
	_, err := s.db.ExecContext(ctx, `UPDATE projects SET name=$1, target_country=$2, target_language=$3, timezone=$4, global_blacklist=$5, default_server_id=$6, status=$7, index_check_enabled=$8, updated_at=NOW() WHERE id=$9 AND user_email=$10 AND deleted_at IS NULL`,
		p.Name, p.TargetCountry, p.TargetLanguage, nullableString(p.Timezone), nullableBytes(p.GlobalBlacklist), nullableString(p.DefaultServerID), p.Status, p.IndexCheckEnabled, p.ID, p.UserEmail)
	return err
}

// UpdateIndexCheckEnabled toggles automatic index checking for a project.
func (s *ProjectStore) UpdateIndexCheckEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE projects SET index_check_enabled=$1, updated_at=NOW() WHERE id=$2`, enabled, id)
	return err
}

func (s *ProjectStore) Delete(ctx context.Context, id, email string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1 AND user_email=$2`, id, email)
	return err
}

// SoftDelete marks a project and all its active domains as deleted within a transaction.
func (s *ProjectStore) SoftDelete(ctx context.Context, id, deletedBy string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE projects SET deleted_at=NOW(), deleted_by=$1, updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`,
		deletedBy, id); err != nil {
		return fmt.Errorf("soft-delete project: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE domains SET deleted_at=NOW(), deleted_by=$1, delete_batch='project:'||$2, updated_at=NOW() WHERE project_id=$2 AND deleted_at IS NULL`,
		deletedBy, id); err != nil {
		return fmt.Errorf("soft-delete project domains: %w", err)
	}
	return tx.Commit()
}

// Restore unmarks a soft-deleted project and restores domains that were batch-deleted with it.
func (s *ProjectStore) Restore(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE projects SET deleted_at=NULL, deleted_by=NULL, updated_at=NOW() WHERE id=$1`,
		id); err != nil {
		return fmt.Errorf("restore project: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE domains SET deleted_at=NULL, deleted_by=NULL, delete_batch=NULL, updated_at=NOW() WHERE project_id=$1 AND delete_batch='project:'||$1`,
		id); err != nil {
		return fmt.Errorf("restore project domains: %w", err)
	}
	return tx.Commit()
}

// GetByIDIncludingDeleted returns a project by ID regardless of soft-delete status.
func (s *ProjectStore) GetByIDIncludingDeleted(ctx context.Context, id string) (Project, error) {
	var p Project
	var gb sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, user_email, name, target_country, target_language, timezone, global_blacklist, default_server_id, status, index_check_enabled, created_at, updated_at, deleted_at, deleted_by FROM projects WHERE id=$1`, id).
		Scan(&p.ID, &p.UserEmail, &p.Name, &p.TargetCountry, &p.TargetLanguage, &p.Timezone, &gb, &p.DefaultServerID, &p.Status, &p.IndexCheckEnabled, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &p.DeletedBy)
	if err != nil {
		return Project{}, err
	}
	if gb.Valid {
		p.GlobalBlacklist = []byte(gb.String)
	}
	return p, nil
}

// ListDeleted returns all soft-deleted projects.
func (s *ProjectStore) ListDeleted(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_email, name, target_country, target_language, timezone, global_blacklist, default_server_id, status, index_check_enabled, created_at, updated_at, deleted_at, deleted_by FROM projects WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Project
	for rows.Next() {
		var p Project
		var gb sql.NullString
		if err := rows.Scan(&p.ID, &p.UserEmail, &p.Name, &p.TargetCountry, &p.TargetLanguage, &p.Timezone, &gb, &p.DefaultServerID, &p.Status, &p.IndexCheckEnabled, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &p.DeletedBy); err != nil {
			return nil, err
		}
		if gb.Valid {
			p.GlobalBlacklist = []byte(gb.String)
		}
		res = append(res, p)
	}
	return res, rows.Err()
}

// PurgeExpired permanently deletes projects that have been soft-deleted longer than retentionDays.
func (s *ProjectStore) PurgeExpired(ctx context.Context, retentionDays int) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM projects WHERE deleted_at IS NOT NULL AND deleted_at < NOW() - make_interval(days => $1)`,
		retentionDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *ProjectStore) ListAll(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_email, name, target_country, target_language, timezone, global_blacklist, default_server_id, status, index_check_enabled, created_at, updated_at, deleted_at, deleted_by FROM projects WHERE deleted_at IS NULL ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Project
	for rows.Next() {
		var p Project
		var gb sql.NullString
		if err := rows.Scan(&p.ID, &p.UserEmail, &p.Name, &p.TargetCountry, &p.TargetLanguage, &p.Timezone, &gb, &p.DefaultServerID, &p.Status, &p.IndexCheckEnabled, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &p.DeletedBy); err != nil {
			return nil, err
		}
		if gb.Valid {
			p.GlobalBlacklist = []byte(gb.String)
		}
		res = append(res, p)
	}
	return res, rows.Err()
}

// ListByNameExact возвращает проекты с точным совпадением имени.
func (s *ProjectStore) ListByNameExact(ctx context.Context, name string) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_email, name, target_country, target_language, timezone, global_blacklist, default_server_id, status, index_check_enabled, created_at, updated_at, deleted_at, deleted_by
FROM projects
WHERE name=$1 AND deleted_at IS NULL
ORDER BY updated_at DESC`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Project
	for rows.Next() {
		var p Project
		var gb sql.NullString
		if err := rows.Scan(&p.ID, &p.UserEmail, &p.Name, &p.TargetCountry, &p.TargetLanguage, &p.Timezone, &gb, &p.DefaultServerID, &p.Status, &p.IndexCheckEnabled, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &p.DeletedBy); err != nil {
			return nil, err
		}
		if gb.Valid {
			p.GlobalBlacklist = []byte(gb.String)
		}
		res = append(res, p)
	}
	return res, rows.Err()
}

func (s *ProjectStore) GetByID(ctx context.Context, id string) (Project, error) {
	var p Project
	var gb sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, user_email, name, target_country, target_language, timezone, global_blacklist, default_server_id, status, index_check_enabled, created_at, updated_at, deleted_at, deleted_by FROM projects WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&p.ID, &p.UserEmail, &p.Name, &p.TargetCountry, &p.TargetLanguage, &p.Timezone, &gb, &p.DefaultServerID, &p.Status, &p.IndexCheckEnabled, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &p.DeletedBy)
	if err != nil {
		return Project{}, err
	}
	if gb.Valid {
		p.GlobalBlacklist = []byte(gb.String)
	}
	return p, nil
}

type DomainStore struct {
	db *sql.DB
}

func NewDomainStore(db *sql.DB) *DomainStore {
	return &DomainStore{db: db}
}

const DefaultServerID = "seotech-web-media1"
const DefaultServerName = "seotech-web-media1"
const DefaultServerIP = "46.21.250.153"

func (s *DomainStore) Create(ctx context.Context, d Domain) error {
  genType := d.GenerationType
  if genType == "" {
    genType = "single_page"
  }
  _, err := s.db.ExecContext(ctx, `INSERT INTO domains(id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, published_path, site_owner, generation_type, created_at, updated_at)
    VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW(),NOW())`,
    d.ID,
    d.ProjectID,
    nullableString(d.ServerID),
    d.URL,
    d.MainKeyword,
    d.TargetCountry,
    d.TargetLanguage,
    nullableString(d.ExcludeDomains),
    nullableBytes(d.SpecificBlacklist),
    d.Status,
    nullableString(d.PublishedPath),
    nullableString(d.SiteOwner),
    genType)
  if err != nil {
    return fmt.Errorf("failed to create domain: %w", err)
  }
  return nil
}

func (s *DomainStore) ListByProject(ctx context.Context, projectID string) ([]Domain, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, last_success_generation_id, published_at, published_path, file_count, total_size_bytes, deployment_mode, site_owner, inventory_status, inventory_checked_at, inventory_error, link_anchor_text, link_acceptor_url, link_status, link_updated_at, link_last_task_id, link_file_path, link_anchor_snapshot, link_ready_at, index_check_enabled, generation_type, created_at, updated_at, deleted_at, deleted_by, delete_batch FROM domains WHERE project_id=$1 AND deleted_at IS NULL ORDER BY updated_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Domain
	for rows.Next() {
		var d Domain
		var sb sql.NullString
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.ServerID, &d.URL, &d.MainKeyword, &d.TargetCountry, &d.TargetLanguage, &d.ExcludeDomains, &sb, &d.Status, &d.LastGenerationID, &d.LastSuccessGenID, &d.PublishedAt, &d.PublishedPath, &d.FileCount, &d.TotalSizeBytes, &d.DeploymentMode, &d.SiteOwner, &d.InventoryStatus, &d.InventoryCheckedAt, &d.InventoryError, &d.LinkAnchorText, &d.LinkAcceptorURL, &d.LinkStatus, &d.LinkUpdatedAt, &d.LinkLastTaskID, &d.LinkFilePath, &d.LinkAnchorSnapshot, &d.LinkReadyAt, &d.IndexCheckEnabled, &d.GenerationType, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt, &d.DeletedBy, &d.DeleteBatch); err != nil {
			return nil, err
		}
		if sb.Valid {
			d.SpecificBlacklist = []byte(sb.String)
		}
		res = append(res, d)
	}
	return res, rows.Err()
}

func (s *DomainStore) ListByIDs(ctx context.Context, ids []string) ([]Domain, error) {
	unique := make(map[string]struct{}, len(ids))
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := unique[trimmed]; ok {
			continue
		}
		unique[trimmed] = struct{}{}
		filtered = append(filtered, trimmed)
	}
	if len(filtered) == 0 {
		return []Domain{}, nil
	}
	placeholders := make([]string, len(filtered))
	args := make([]any, len(filtered))
	for i, id := range filtered {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf(`SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, last_success_generation_id, published_at, published_path, file_count, total_size_bytes, deployment_mode, site_owner, inventory_status, inventory_checked_at, inventory_error, link_anchor_text, link_acceptor_url, link_status, link_updated_at, link_last_task_id, link_file_path, link_anchor_snapshot, link_ready_at, index_check_enabled, generation_type, created_at, updated_at, deleted_at, deleted_by, delete_batch FROM domains WHERE id IN (%s) AND deleted_at IS NULL`, strings.Join(placeholders, ","))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Domain
	for rows.Next() {
		var d Domain
		var sb sql.NullString
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.ServerID, &d.URL, &d.MainKeyword, &d.TargetCountry, &d.TargetLanguage, &d.ExcludeDomains, &sb, &d.Status, &d.LastGenerationID, &d.LastSuccessGenID, &d.PublishedAt, &d.PublishedPath, &d.FileCount, &d.TotalSizeBytes, &d.DeploymentMode, &d.SiteOwner, &d.InventoryStatus, &d.InventoryCheckedAt, &d.InventoryError, &d.LinkAnchorText, &d.LinkAcceptorURL, &d.LinkStatus, &d.LinkUpdatedAt, &d.LinkLastTaskID, &d.LinkFilePath, &d.LinkAnchorSnapshot, &d.LinkReadyAt, &d.IndexCheckEnabled, &d.GenerationType, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt, &d.DeletedBy, &d.DeleteBatch); err != nil {
			return nil, err
		}
		if sb.Valid {
			d.SpecificBlacklist = []byte(sb.String)
		}
		res = append(res, d)
	}
	return res, rows.Err()
}

func (s *DomainStore) Get(ctx context.Context, id string) (Domain, error) {
	var d Domain
	var sb sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, last_success_generation_id, published_at, published_path, file_count, total_size_bytes, deployment_mode, site_owner, inventory_status, inventory_checked_at, inventory_error, link_anchor_text, link_acceptor_url, link_status, link_updated_at, link_last_task_id, link_file_path, link_anchor_snapshot, link_ready_at, index_check_enabled, generation_type, created_at, updated_at, deleted_at, deleted_by, delete_batch FROM domains WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&d.ID, &d.ProjectID, &d.ServerID, &d.URL, &d.MainKeyword, &d.TargetCountry, &d.TargetLanguage, &d.ExcludeDomains, &sb, &d.Status, &d.LastGenerationID, &d.LastSuccessGenID, &d.PublishedAt, &d.PublishedPath, &d.FileCount, &d.TotalSizeBytes, &d.DeploymentMode, &d.SiteOwner, &d.InventoryStatus, &d.InventoryCheckedAt, &d.InventoryError, &d.LinkAnchorText, &d.LinkAcceptorURL, &d.LinkStatus, &d.LinkUpdatedAt, &d.LinkLastTaskID, &d.LinkFilePath, &d.LinkAnchorSnapshot, &d.LinkReadyAt, &d.IndexCheckEnabled, &d.GenerationType, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt, &d.DeletedBy, &d.DeleteBatch)
	if err != nil {
		return Domain{}, err
	}
	if sb.Valid {
		d.SpecificBlacklist = []byte(sb.String)
	}
	return d, nil
}

// GetByURL возвращает домен по точному URL.
func (s *DomainStore) GetByURL(ctx context.Context, url string) (Domain, error) {
	var d Domain
	var sb sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, last_success_generation_id, published_at, published_path, file_count, total_size_bytes, deployment_mode, site_owner, inventory_status, inventory_checked_at, inventory_error, link_anchor_text, link_acceptor_url, link_status, link_updated_at, link_last_task_id, link_file_path, link_anchor_snapshot, link_ready_at, index_check_enabled, generation_type, created_at, updated_at, deleted_at, deleted_by, delete_batch FROM domains WHERE url=$1 AND deleted_at IS NULL`, url).
		Scan(&d.ID, &d.ProjectID, &d.ServerID, &d.URL, &d.MainKeyword, &d.TargetCountry, &d.TargetLanguage, &d.ExcludeDomains, &sb, &d.Status, &d.LastGenerationID, &d.LastSuccessGenID, &d.PublishedAt, &d.PublishedPath, &d.FileCount, &d.TotalSizeBytes, &d.DeploymentMode, &d.SiteOwner, &d.InventoryStatus, &d.InventoryCheckedAt, &d.InventoryError, &d.LinkAnchorText, &d.LinkAcceptorURL, &d.LinkStatus, &d.LinkUpdatedAt, &d.LinkLastTaskID, &d.LinkFilePath, &d.LinkAnchorSnapshot, &d.LinkReadyAt, &d.IndexCheckEnabled, &d.GenerationType, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt, &d.DeletedBy, &d.DeleteBatch)
	if err != nil {
		return Domain{}, err
	}
	if sb.Valid {
		d.SpecificBlacklist = []byte(sb.String)
	}
	return d, nil
}

func (s *DomainStore) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}

func (s *DomainStore) UpdateDeploymentMode(ctx context.Context, id, mode string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains SET deployment_mode=$1, updated_at=NOW() WHERE id=$2`, mode, id)
	return err
}

// UpdatePublishState обновляет путь публикации и статистику файлов домена.
func (s *DomainStore) UpdatePublishState(ctx context.Context, id, publishedPath string, fileCount int, totalSizeBytes int64) error {
	var pathVal *string
	if strings.TrimSpace(publishedPath) != "" {
		pathVal = &publishedPath
	}
	_, err := s.db.ExecContext(ctx, `UPDATE domains
		SET published_path=$1::text,
		    file_count=$2,
		    total_size_bytes=$3,
		    published_at=CASE WHEN $1::text IS NULL THEN NULL ELSE NOW() END,
		    updated_at=NOW()
		WHERE id=$4`,
		nullableString(nullString(pathVal)), fileCount, totalSizeBytes, id)
	return err
}

// UpdateInventoryState обновляет данные инвентаризации домена на удаленном сервере.
func (s *DomainStore) UpdateInventoryState(
	ctx context.Context,
	id string,
	publishedPath sql.NullString,
	siteOwner sql.NullString,
	inventoryStatus sql.NullString,
	inventoryError sql.NullString,
	checkedAt time.Time,
) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains
		SET published_path=$1,
		    site_owner=$2,
		    inventory_status=$3,
		    inventory_checked_at=$4,
		    inventory_error=$5,
		    updated_at=NOW()
		WHERE id=$6`,
		nullableString(publishedPath),
		nullableString(siteOwner),
		nullableString(inventoryStatus),
		checkedAt,
		nullableString(inventoryError),
		id,
	)
	return err
}

func (s *DomainStore) UpdateKeyword(ctx context.Context, id, keyword string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains SET main_keyword=$1, updated_at=NOW() WHERE id=$2`, keyword, id)
	return err
}

func (s *DomainStore) SetLastGeneration(ctx context.Context, id, genID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains SET last_generation_id=$1, updated_at=NOW() WHERE id=$2`, genID, id)
	return err
}

func (s *DomainStore) SetLastSuccessGeneration(ctx context.Context, id, genID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains SET last_success_generation_id=$1, updated_at=NOW() WHERE id=$2`, genID, id)
	return err
}

func (s *DomainStore) RecalculateGenerationPointers(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains d
SET last_generation_id = (
		SELECT g.id
		FROM generations g
		WHERE g.domain_id=d.id
		ORDER BY g.created_at DESC
		LIMIT 1
	),
	last_success_generation_id = (
		SELECT g.id
		FROM generations g
		WHERE g.domain_id=d.id AND g.status='success'
		ORDER BY g.created_at DESC
		LIMIT 1
	),
	updated_at=NOW()
WHERE d.id=$1`, id)
	return err
}

func (s *DomainStore) UpdateExtras(ctx context.Context, id, country, language string, exclude, server sql.NullString) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE domains SET target_country=$1, target_language=$2, exclude_domains=$3, server_id=$4, updated_at=NOW() WHERE id=$5`,
		country, language, nullableString(exclude), nullableString(server), id)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

func (s *DomainStore) UpdateLinkState(ctx context.Context, id string, status string, lastTaskID string, filePath string, anchorSnapshot string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains
		SET link_status=$1,
		    link_last_task_id=$2,
		    link_file_path=$3,
		    link_anchor_snapshot=$4,
		    link_updated_at=NOW(),
		    updated_at=NOW()
		WHERE id=$5`,
		status,
		nullableString(nullString(&lastTaskID)),
		nullableString(nullString(&filePath)),
		nullableString(nullString(&anchorSnapshot)),
		id,
	)
	return err
}

func (s *DomainStore) UpdateLinkStatus(ctx context.Context, id string, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains
		SET link_status=$1,
		    link_updated_at=NOW(),
		    updated_at=NOW()
		WHERE id=$2`,
		nullableString(nullString(&status)),
		id,
	)
	return err
}

// UpdateLinkReadyAt обновляет время готовности домена для линкбилдинга.
func (s *DomainStore) UpdateLinkReadyAt(ctx context.Context, id string, readyAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains
		SET link_ready_at=$1,
		    updated_at=NOW()
		WHERE id=$2`, readyAt, id)
	return err
}

func (s *DomainStore) UpdateLinkSettings(ctx context.Context, id string, anchorText, acceptorURL sql.NullString) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE domains
		SET link_anchor_text=$1,
		    link_acceptor_url=$2,
		    link_updated_at=NOW(),
		    updated_at=NOW()
		WHERE id=$3`,
		nullableString(anchorText), nullableString(acceptorURL), id)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

// UpdateIndexCheckEnabled toggles automatic index checking for a domain.
func (s *DomainStore) UpdateIndexCheckEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains SET index_check_enabled=$1, updated_at=NOW() WHERE id=$2`, enabled, id)
	return err
}

// UpdateGenerationType changes the generation type for a domain.
func (s *DomainStore) UpdateGenerationType(ctx context.Context, id, genType string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains SET generation_type=$1, updated_at=NOW() WHERE id=$2`, genType, id)
	return err
}

// UpdateProject переносит домен в другой проект.
func (s *DomainStore) UpdateProject(ctx context.Context, id, projectID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains SET project_id=$1, updated_at=NOW() WHERE id=$2`, projectID, id)
	return err
}

// EnsureDefaultServer creates a default server record if it doesn't exist.
func (s *DomainStore) EnsureDefaultServer(ctx context.Context, userEmail string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO servers(id, user_email, name, ip, ssh_user, ssh_key, created_at, updated_at)
		VALUES($1,$2,$3,$4,'','',NOW(),NOW())
		ON CONFLICT (id) DO NOTHING`,
		DefaultServerID, userEmail, DefaultServerName, DefaultServerIP)
	return err
}

func (s *DomainStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM domains WHERE id=$1`, id)
	return err
}

// SoftDelete marks a domain as deleted.
func (s *DomainStore) SoftDelete(ctx context.Context, id, deletedBy string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE domains SET deleted_at=NOW(), deleted_by=$1, updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`,
		deletedBy, id)
	return err
}

// Restore removes soft-delete marks from a domain.
func (s *DomainStore) Restore(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE domains SET deleted_at=NULL, deleted_by=NULL, delete_batch=NULL, updated_at=NOW() WHERE id=$1`,
		id)
	return err
}

// GetIncludingDeleted returns a domain by ID regardless of soft-delete status.
func (s *DomainStore) GetIncludingDeleted(ctx context.Context, id string) (Domain, error) {
	var d Domain
	var sb sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, last_success_generation_id, published_at, published_path, file_count, total_size_bytes, deployment_mode, site_owner, inventory_status, inventory_checked_at, inventory_error, link_anchor_text, link_acceptor_url, link_status, link_updated_at, link_last_task_id, link_file_path, link_anchor_snapshot, link_ready_at, index_check_enabled, generation_type, created_at, updated_at, deleted_at, deleted_by, delete_batch FROM domains WHERE id=$1`, id).
		Scan(&d.ID, &d.ProjectID, &d.ServerID, &d.URL, &d.MainKeyword, &d.TargetCountry, &d.TargetLanguage, &d.ExcludeDomains, &sb, &d.Status, &d.LastGenerationID, &d.LastSuccessGenID, &d.PublishedAt, &d.PublishedPath, &d.FileCount, &d.TotalSizeBytes, &d.DeploymentMode, &d.SiteOwner, &d.InventoryStatus, &d.InventoryCheckedAt, &d.InventoryError, &d.LinkAnchorText, &d.LinkAcceptorURL, &d.LinkStatus, &d.LinkUpdatedAt, &d.LinkLastTaskID, &d.LinkFilePath, &d.LinkAnchorSnapshot, &d.LinkReadyAt, &d.IndexCheckEnabled, &d.GenerationType, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt, &d.DeletedBy, &d.DeleteBatch)
	if err != nil {
		return Domain{}, err
	}
	if sb.Valid {
		d.SpecificBlacklist = []byte(sb.String)
	}
	return d, nil
}

// ListDeleted returns all soft-deleted domains.
func (s *DomainStore) ListDeleted(ctx context.Context) ([]Domain, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, last_success_generation_id, published_at, published_path, file_count, total_size_bytes, deployment_mode, site_owner, inventory_status, inventory_checked_at, inventory_error, link_anchor_text, link_acceptor_url, link_status, link_updated_at, link_last_task_id, link_file_path, link_anchor_snapshot, link_ready_at, index_check_enabled, generation_type, created_at, updated_at, deleted_at, deleted_by, delete_batch FROM domains WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Domain
	for rows.Next() {
		var d Domain
		var sb sql.NullString
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.ServerID, &d.URL, &d.MainKeyword, &d.TargetCountry, &d.TargetLanguage, &d.ExcludeDomains, &sb, &d.Status, &d.LastGenerationID, &d.LastSuccessGenID, &d.PublishedAt, &d.PublishedPath, &d.FileCount, &d.TotalSizeBytes, &d.DeploymentMode, &d.SiteOwner, &d.InventoryStatus, &d.InventoryCheckedAt, &d.InventoryError, &d.LinkAnchorText, &d.LinkAcceptorURL, &d.LinkStatus, &d.LinkUpdatedAt, &d.LinkLastTaskID, &d.LinkFilePath, &d.LinkAnchorSnapshot, &d.LinkReadyAt, &d.IndexCheckEnabled, &d.GenerationType, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt, &d.DeletedBy, &d.DeleteBatch); err != nil {
			return nil, err
		}
		if sb.Valid {
			d.SpecificBlacklist = []byte(sb.String)
		}
		res = append(res, d)
	}
	return res, rows.Err()
}

// PurgeExpired permanently deletes domains that have been soft-deleted longer than retentionDays,
// excluding domains whose parent project is also soft-deleted (those will be cleaned up by project cascade).
func (s *DomainStore) PurgeExpired(ctx context.Context, retentionDays int) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM domains WHERE deleted_at IS NOT NULL AND deleted_at < NOW() - make_interval(days => $1)
		 AND project_id NOT IN (SELECT id FROM projects WHERE deleted_at IS NOT NULL)`,
		retentionDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

type GenerationStore struct {
	db *sql.DB
}

func NewGenerationStore(db *sql.DB) *GenerationStore {
	return &GenerationStore{db: db}
}

func (s *GenerationStore) Get(ctx context.Context, id string) (Generation, error) {
	var g Generation
	var logs, artifacts, checkpoint sql.NullString
	var summary sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, requested_by, status, progress, error, logs, artifacts, artifacts_summary, checkpoint_data, attempts, retryable, next_retry_at, last_error_at, started_at, finished_at, prompt_id, generation_type, created_at, updated_at FROM generations WHERE id=$1`, id).
		Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &summary, &checkpoint, &g.Attempts, &g.Retryable, &g.NextRetryAt, &g.LastErrorAt, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.GenerationType, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return Generation{}, err
	}
	if logs.Valid {
		g.Logs = []byte(logs.String)
	}
	if artifacts.Valid {
		g.Artifacts = []byte(artifacts.String)
	}
	if summary.Valid {
		g.ArtifactsSummary = []byte(summary.String)
	}
	if checkpoint.Valid {
		g.CheckpointData = []byte(checkpoint.String)
	}
	return g, nil
}

func (s *GenerationStore) Create(ctx context.Context, g Generation) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO generations(id, domain_id, requested_by, status, progress, error, logs, artifacts, artifacts_summary, checkpoint_data, attempts, retryable, next_retry_at, last_error_at, started_at, finished_at, prompt_id, generation_type, created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,NOW())`,
		g.ID,
		g.DomainID,
		nullableString(g.RequestedBy),
		g.Status,
		g.Progress,
		nullableString(g.Error),
		nullableBytes(g.Logs),
		nullableBytes(g.Artifacts),
		nullableBytes(g.ArtifactsSummary),
		nullableBytes(g.CheckpointData),
		g.Attempts,
		g.Retryable,
		nullableTime(g.NextRetryAt),
		nullableTime(g.LastErrorAt),
		nullableTime(g.StartedAt),
		nullableTime(g.FinishedAt),
		nullableString(g.PromptID),
		g.GenerationType,
	)
	if err != nil {
		return fmt.Errorf("failed to create generation: %w", err)
	}
	return nil
}

func (s *GenerationStore) ListByDomain(ctx context.Context, domainID string) ([]Generation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, requested_by, status, progress, error, logs, artifacts, artifacts_summary, checkpoint_data, attempts, retryable, next_retry_at, last_error_at, started_at, finished_at, prompt_id, generation_type, created_at, updated_at FROM generations WHERE domain_id=$1 ORDER BY created_at DESC`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Generation
	for rows.Next() {
		var g Generation
		var logs, artifacts, summary, checkpoint sql.NullString
		if err := rows.Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &summary, &checkpoint, &g.Attempts, &g.Retryable, &g.NextRetryAt, &g.LastErrorAt, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.GenerationType, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		if logs.Valid {
			g.Logs = []byte(logs.String)
		}
		if artifacts.Valid {
			g.Artifacts = []byte(artifacts.String)
		}
		if summary.Valid {
			g.ArtifactsSummary = []byte(summary.String)
		}
		if checkpoint.Valid {
			g.CheckpointData = []byte(checkpoint.String)
		}
		res = append(res, g)
	}
	return res, rows.Err()
}

func (s *GenerationStore) UpdateStatus(ctx context.Context, id, status string, progress int, errText *string) error {
	var errVal sql.NullString
	if errText != nil {
		errVal = sql.NullString{String: *errText, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `UPDATE generations SET status=$1, progress=$2, error=$3, updated_at=NOW() WHERE id=$4`, status, progress, errVal, id)
	return err
}

// UpdateRetry updates retry metadata for a generation after a failure.
func (s *GenerationStore) UpdateRetry(ctx context.Context, id string, attempts int, retryable bool, nextRetryAt, lastErrorAt sql.NullTime) error {
	_, err := s.db.ExecContext(ctx, `UPDATE generations SET attempts=$1, retryable=$2, next_retry_at=$3, last_error_at=$4, updated_at=NOW() WHERE id=$5`,
		attempts, retryable, nullableTime(nextRetryAt), nullableTime(lastErrorAt), id)
	return err
}

// PrepareRetry marks generation as pending for a retry run.
func (s *GenerationStore) PrepareRetry(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE generations SET status='pending', error=NULL, updated_at=NOW() WHERE id=$1`, id)
	return err
}

// ListRetryDue returns retryable generations ready to be retried.
func (s *GenerationStore) ListRetryDue(ctx context.Context, now time.Time, limit int) ([]Generation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, requested_by, status, progress, error, logs, artifacts, artifacts_summary, checkpoint_data, attempts, retryable, next_retry_at, last_error_at, started_at, finished_at, prompt_id, generation_type, created_at, updated_at
FROM generations
WHERE status='error' AND retryable=TRUE AND next_retry_at IS NOT NULL AND next_retry_at <= $1
ORDER BY next_retry_at ASC
LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Generation
	for rows.Next() {
		var g Generation
		var logs, artifacts, summary, checkpoint sql.NullString
		if err := rows.Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &summary, &checkpoint, &g.Attempts, &g.Retryable, &g.NextRetryAt, &g.LastErrorAt, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.GenerationType, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		if logs.Valid {
			g.Logs = []byte(logs.String)
		}
		if artifacts.Valid {
			g.Artifacts = []byte(artifacts.String)
		}
		if summary.Valid {
			g.ArtifactsSummary = []byte(summary.String)
		}
		if checkpoint.Valid {
			g.CheckpointData = []byte(checkpoint.String)
		}
		res = append(res, g)
	}
	return res, rows.Err()
}

func (s *GenerationStore) UpdateFull(ctx context.Context, id, status string, progress int, errText *string, logs, artifacts []byte, started, finished *time.Time, promptID *string) error {
	var errVal sql.NullString
	if errText != nil {
		errVal = sql.NullString{String: *errText, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `UPDATE generations
		SET status=$1,
		    progress=$2,
		    error=$3,
		    logs=$4,
		    artifacts=$5,
		    started_at=$6,
		    finished_at=$7,
		    prompt_id=CASE WHEN $10 THEN $8 ELSE prompt_id END,
		    updated_at=NOW()
		WHERE id=$9`,
		status, progress, errVal, nullableBytes(logs), nullableBytes(artifacts), nullableTime(nullTime(started)), nullableTime(nullTime(finished)), nullableString(nullString(promptID)), id, promptID != nil)
	return err
}

func (s *GenerationStore) UpdateArtifactsSummary(ctx context.Context, id string, artifactsSummary []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE generations SET artifacts_summary=$1, updated_at=NOW() WHERE id=$2`, nullableBytes(artifactsSummary), id)
	return err
}

// UpdateLogs обновляет только поле logs, не затрагивая остальные колонки.
func (s *GenerationStore) UpdateLogs(ctx context.Context, id string, logs []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE generations SET logs=$1, updated_at=NOW() WHERE id=$2`, nullableBytes(logs), id)
	return err
}

// GetLastSuccessfulByDomain возвращает последнюю успешную генерацию домена.
func (s *GenerationStore) GetLastSuccessfulByDomain(ctx context.Context, domainID string) (Generation, error) {
	var g Generation
	var logs, artifacts, summary, checkpoint sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, requested_by, status, progress, error, logs, artifacts, artifacts_summary, checkpoint_data, attempts, retryable, next_retry_at, last_error_at, started_at, finished_at, prompt_id, created_at, updated_at
		FROM generations WHERE domain_id=$1 AND status='success' ORDER BY created_at DESC LIMIT 1`, domainID).
		Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &summary, &checkpoint, &g.Attempts, &g.Retryable, &g.NextRetryAt, &g.LastErrorAt, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.GenerationType, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return Generation{}, err
	}
	if logs.Valid {
		g.Logs = []byte(logs.String)
	}
	if artifacts.Valid {
		g.Artifacts = []byte(artifacts.String)
	}
	if summary.Valid {
		g.ArtifactsSummary = []byte(summary.String)
	}
	if checkpoint.Valid {
		g.CheckpointData = []byte(checkpoint.String)
	}
	return g, nil
}

// GetLastByDomain возвращает последнюю генерацию домена независимо от статуса.
func (s *GenerationStore) GetLastByDomain(ctx context.Context, domainID string) (Generation, error) {
	var g Generation
	var logs, artifacts, summary, checkpoint sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, requested_by, status, progress, error, logs, artifacts, artifacts_summary, checkpoint_data, attempts, retryable, next_retry_at, last_error_at, started_at, finished_at, prompt_id, created_at, updated_at
		FROM generations WHERE domain_id=$1 ORDER BY created_at DESC LIMIT 1`, domainID).
		Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &summary, &checkpoint, &g.Attempts, &g.Retryable, &g.NextRetryAt, &g.LastErrorAt, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.GenerationType, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return Generation{}, err
	}
	if logs.Valid {
		g.Logs = []byte(logs.String)
	}
	if artifacts.Valid {
		g.Artifacts = []byte(artifacts.String)
	}
	if summary.Valid {
		g.ArtifactsSummary = []byte(summary.String)
	}
	if checkpoint.Valid {
		g.CheckpointData = []byte(checkpoint.String)
	}
	return g, nil
}

// SaveCheckpoint сохраняет чекпоинт для задачи
func (s *GenerationStore) SaveCheckpoint(ctx context.Context, id string, checkpointData []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE generations SET checkpoint_data=$1, updated_at=NOW() WHERE id=$2`,
		nullableBytes(checkpointData), id)
	return err
}

// ClearCheckpoint очищает чекпоинт
func (s *GenerationStore) ClearCheckpoint(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE generations SET checkpoint_data=NULL, updated_at=NOW() WHERE id=$1`, id)
	return err
}

// UpdateStatusWithCheckpoint обновляет статус и чекпоинт одновременно
func (s *GenerationStore) UpdateStatusWithCheckpoint(ctx context.Context, id, status string, progress int, checkpointData []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE generations SET status=$1, progress=$2, checkpoint_data=$3, updated_at=NOW() WHERE id=$4`,
		status, progress, nullableBytes(checkpointData), id)
	return err
}

func (s *GenerationStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM generations WHERE id=$1`, id)
	return err
}

func (s *GenerationStore) ListRecentByUser(ctx context.Context, email string, limit, offset int, search string) ([]Generation, error) {
	clauses := []string{
		`(p.user_email = $1
   OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_email = $1))`,
		`p.deleted_at IS NULL`,
		`d.deleted_at IS NULL`,
	}
	args := []interface{}{email}
	idx := 2
	if term := strings.TrimSpace(search); term != "" {
		clauses = append(clauses, fmt.Sprintf("(LOWER(COALESCE(d.url, '')) LIKE $%d OR LOWER(g.domain_id) LIKE $%d)", idx, idx))
		args = append(args, "%"+strings.ToLower(term)+"%")
		idx++
	}
	query := fmt.Sprintf(`SELECT g.id, g.domain_id, g.requested_by, g.status, g.progress, g.error, g.logs, g.artifacts, g.artifacts_summary, g.checkpoint_data, g.attempts, g.retryable, g.next_retry_at, g.last_error_at, g.started_at, g.finished_at, g.prompt_id, g.generation_type, g.created_at, g.updated_at
FROM generations g
JOIN domains d ON g.domain_id = d.id
JOIN projects p ON d.project_id = p.id
WHERE %s
ORDER BY g.created_at DESC
LIMIT $%d OFFSET $%d`, strings.Join(clauses, " AND "), idx, idx+1)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Generation
	for rows.Next() {
		var g Generation
		var logs, artifacts, summary, checkpoint sql.NullString
		if err := rows.Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &summary, &checkpoint, &g.Attempts, &g.Retryable, &g.NextRetryAt, &g.LastErrorAt, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.GenerationType, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		if logs.Valid {
			g.Logs = []byte(logs.String)
		}
		if artifacts.Valid {
			g.Artifacts = []byte(artifacts.String)
		}
		if summary.Valid {
			g.ArtifactsSummary = []byte(summary.String)
		}
		if checkpoint.Valid {
			g.CheckpointData = []byte(checkpoint.String)
		}
		res = append(res, g)
	}
	return res, rows.Err()
}

func (s *GenerationStore) ListRecentByUserLite(ctx context.Context, email string, limit, offset int, search string) ([]Generation, error) {
	clauses := []string{
		`(p.user_email = $1
   OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_email = $1))`,
		`p.deleted_at IS NULL`,
		`d.deleted_at IS NULL`,
	}
	args := []interface{}{email}
	idx := 2
	if term := strings.TrimSpace(search); term != "" {
		clauses = append(clauses, fmt.Sprintf("(LOWER(COALESCE(d.url, '')) LIKE $%d OR LOWER(g.domain_id) LIKE $%d)", idx, idx))
		args = append(args, "%"+strings.ToLower(term)+"%")
		idx++
	}
	query := fmt.Sprintf(`SELECT g.id, g.domain_id, g.requested_by, g.status, g.progress, g.error, g.attempts, g.retryable, g.next_retry_at, g.last_error_at, g.started_at, g.finished_at, g.prompt_id, g.generation_type, g.created_at, g.updated_at
FROM generations g
JOIN domains d ON g.domain_id = d.id
JOIN projects p ON d.project_id = p.id
WHERE %s
ORDER BY g.created_at DESC
LIMIT $%d OFFSET $%d`, strings.Join(clauses, " AND "), idx, idx+1)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Generation
	for rows.Next() {
		var g Generation
		if err := rows.Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &g.Attempts, &g.Retryable, &g.NextRetryAt, &g.LastErrorAt, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.GenerationType, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, g)
	}
	return res, rows.Err()
}

func (s *GenerationStore) ListRecentAll(ctx context.Context, limit, offset int, search string) ([]Generation, error) {
	args := []interface{}{}
	idx := 1
	query := `SELECT g.id, g.domain_id, g.requested_by, g.status, g.progress, g.error, g.logs, g.artifacts, g.artifacts_summary, g.checkpoint_data, g.attempts, g.retryable, g.next_retry_at, g.last_error_at, g.started_at, g.finished_at, g.prompt_id, g.generation_type, g.created_at, g.updated_at
FROM generations g`
	if term := strings.TrimSpace(search); term != "" {
		query += " LEFT JOIN domains d ON d.id = g.domain_id"
		query += fmt.Sprintf(" WHERE (LOWER(COALESCE(d.url, '')) LIKE $%d OR LOWER(g.domain_id) LIKE $%d)", idx, idx)
		args = append(args, "%"+strings.ToLower(term)+"%")
		idx++
	}
	query += fmt.Sprintf(" ORDER BY g.created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Generation
	for rows.Next() {
		var g Generation
		var logs, artifacts, summary, checkpoint sql.NullString
		if err := rows.Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &summary, &checkpoint, &g.Attempts, &g.Retryable, &g.NextRetryAt, &g.LastErrorAt, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.GenerationType, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		if logs.Valid {
			g.Logs = []byte(logs.String)
		}
		if artifacts.Valid {
			g.Artifacts = []byte(artifacts.String)
		}
		if summary.Valid {
			g.ArtifactsSummary = []byte(summary.String)
		}
		if checkpoint.Valid {
			g.CheckpointData = []byte(checkpoint.String)
		}
		res = append(res, g)
	}
	return res, rows.Err()
}

func (s *GenerationStore) ListRecentAllLite(ctx context.Context, limit, offset int, search string) ([]Generation, error) {
	args := []interface{}{}
	idx := 1
	query := `SELECT g.id, g.domain_id, g.requested_by, g.status, g.progress, g.error, g.attempts, g.retryable, g.next_retry_at, g.last_error_at, g.started_at, g.finished_at, g.prompt_id, g.generation_type, g.created_at, g.updated_at
FROM generations g`
	if term := strings.TrimSpace(search); term != "" {
		query += " LEFT JOIN domains d ON d.id = g.domain_id"
		query += fmt.Sprintf(" WHERE (LOWER(COALESCE(d.url, '')) LIKE $%d OR LOWER(g.domain_id) LIKE $%d)", idx, idx)
		args = append(args, "%"+strings.ToLower(term)+"%")
		idx++
	}
	query += fmt.Sprintf(" ORDER BY g.created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Generation
	for rows.Next() {
		var g Generation
		if err := rows.Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &g.Attempts, &g.Retryable, &g.NextRetryAt, &g.LastErrorAt, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.GenerationType, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, g)
	}
	return res, rows.Err()
}

func (s *GenerationStore) CountsByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM generations GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		res[status] = count
	}
	return res, rows.Err()
}

// DeleteOldGenerations удаляет генерации старше указанного количества дней с определенными статусами
// Возвращает количество удаленных записей
func (s *GenerationStore) DeleteOldGenerations(ctx context.Context, olderThanDays int, statuses []string) (int, error) {
	if len(statuses) == 0 {
		// По умолчанию удаляем только cancelled и error
		statuses = []string{"cancelled", "error"}
	}

	// Строим SQL запрос с использованием IN для списка статусов
	// Создаем плейсхолдеры для каждого статуса
	placeholders := make([]string, len(statuses))
	args := make([]interface{}, len(statuses))
	for i, status := range statuses {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = status
	}

	// Используем параметризованный запрос для интервала
	query := fmt.Sprintf(
		`DELETE FROM generations WHERE created_at < NOW() - INTERVAL '%d days' AND status IN (%s)`,
		olderThanDays,
		strings.Join(placeholders, ","),
	)

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old generations: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(count), nil
}

func nullableString(ns sql.NullString) interface{} {
	if ns.Valid {
		return ns.String
	}
	return nil
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func nullableTime(t sql.NullTime) interface{} {
	if t.Valid {
		return t.Time
	}
	return nil
}

func NullableString(val string) sql.NullString {
	return sql.NullString{String: val, Valid: true}
}

func nullString(ptr *string) sql.NullString {
	if ptr == nil || *ptr == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *ptr, Valid: true}
}

func nullableBytes(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}

// Project member helpers

func (s *ProjectStore) AddMember(ctx context.Context, projectID, email, role string) error {
	if role == "" {
		role = "editor"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO project_members(project_id, user_email, role) VALUES($1,$2,$3)
		ON CONFLICT (project_id, user_email) DO UPDATE SET role=$3`, projectID, email, role)
	return err
}

func (s *ProjectStore) RemoveMember(ctx context.Context, projectID, email string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1 AND user_email=$2`, projectID, email)
	return err
}

func (s *ProjectStore) ListMembers(ctx context.Context, projectID string) ([]ProjectMember, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, user_email, role, created_at FROM project_members WHERE project_id=$1`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ProjectMember
	for rows.Next() {
		var pm ProjectMember
		if err := rows.Scan(&pm.ProjectID, &pm.UserEmail, &pm.Role, &pm.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, pm)
	}
	return res, rows.Err()
}

func (s *ProjectStore) IsMember(ctx context.Context, projectID, email string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=$1 AND user_email=$2)`, projectID, email).Scan(&exists)
	return exists, err
}
