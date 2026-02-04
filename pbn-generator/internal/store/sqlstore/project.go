package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Project struct {
	ID              string
	UserEmail       string
	Name            string
	TargetCountry   string
	TargetLanguage  string
	GlobalBlacklist []byte
	DefaultServerID sql.NullString
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Domain struct {
	ID                string
	ProjectID         string
	ServerID          sql.NullString
	URL               string
	MainKeyword       string
	TargetCountry     string
	TargetLanguage    string
	ExcludeDomains    sql.NullString
	SpecificBlacklist []byte
	Status            string
	LastGenerationID  sql.NullString
	PublishedAt       sql.NullTime
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Generation struct {
	ID             string
	DomainID       string
	RequestedBy    sql.NullString
	Status         string
	Progress       int
	Error          sql.NullString
	Logs           []byte
	Artifacts      []byte
	CheckpointData []byte // JSONB checkpoint data
	StartedAt      sql.NullTime
	FinishedAt     sql.NullTime
	PromptID       sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO projects(id, user_email, name, target_country, target_language, global_blacklist, default_server_id, status, created_at, updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())`, p.ID, p.UserEmail, p.Name, p.TargetCountry, p.TargetLanguage, nullableBytes(p.GlobalBlacklist), nullableString(p.DefaultServerID), p.Status)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	return nil
}

func (s *ProjectStore) ListByUser(ctx context.Context, email string) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_email, name, target_country, target_language, global_blacklist, default_server_id, status, created_at, updated_at
FROM projects
WHERE user_email=$1
   OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = projects.id AND pm.user_email = $1)
ORDER BY updated_at DESC`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Project
	for rows.Next() {
		var p Project
		var gb sql.NullString
		if err := rows.Scan(&p.ID, &p.UserEmail, &p.Name, &p.TargetCountry, &p.TargetLanguage, &gb, &p.DefaultServerID, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
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
	err := s.db.QueryRowContext(ctx, `SELECT id, user_email, name, target_country, target_language, global_blacklist, default_server_id, status, created_at, updated_at
FROM projects
WHERE id=$1 AND (user_email=$2 OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = projects.id AND pm.user_email = $2))`, id, email).
		Scan(&p.ID, &p.UserEmail, &p.Name, &p.TargetCountry, &p.TargetLanguage, &gb, &p.DefaultServerID, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return Project{}, err
	}
	if gb.Valid {
		p.GlobalBlacklist = []byte(gb.String)
	}
	return p, nil
}

func (s *ProjectStore) Update(ctx context.Context, p Project) error {
	_, err := s.db.ExecContext(ctx, `UPDATE projects SET name=$1, target_country=$2, target_language=$3, global_blacklist=$4, default_server_id=$5, status=$6, updated_at=NOW() WHERE id=$7 AND user_email=$8`,
		p.Name, p.TargetCountry, p.TargetLanguage, nullableBytes(p.GlobalBlacklist), nullableString(p.DefaultServerID), p.Status, p.ID, p.UserEmail)
	return err
}

func (s *ProjectStore) Delete(ctx context.Context, id, email string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1 AND user_email=$2`, id, email)
	return err
}

func (s *ProjectStore) ListAll(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_email, name, target_country, target_language, global_blacklist, default_server_id, status, created_at, updated_at FROM projects ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Project
	for rows.Next() {
		var p Project
		var gb sql.NullString
		if err := rows.Scan(&p.ID, &p.UserEmail, &p.Name, &p.TargetCountry, &p.TargetLanguage, &gb, &p.DefaultServerID, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
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
	err := s.db.QueryRowContext(ctx, `SELECT id, user_email, name, target_country, target_language, global_blacklist, default_server_id, status, created_at, updated_at FROM projects WHERE id=$1`, id).
		Scan(&p.ID, &p.UserEmail, &p.Name, &p.TargetCountry, &p.TargetLanguage, &gb, &p.DefaultServerID, &p.Status, &p.CreatedAt, &p.UpdatedAt)
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO domains(id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, created_at, updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())`,
		d.ID, d.ProjectID, nullableString(d.ServerID), d.URL, d.MainKeyword, d.TargetCountry, d.TargetLanguage, nullableString(d.ExcludeDomains), nullableBytes(d.SpecificBlacklist), d.Status)
	if err != nil {
		return fmt.Errorf("failed to create domain: %w", err)
	}
	return nil
}

func (s *DomainStore) ListByProject(ctx context.Context, projectID string) ([]Domain, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, published_at, created_at, updated_at FROM domains WHERE project_id=$1 ORDER BY updated_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Domain
	for rows.Next() {
		var d Domain
		var sb sql.NullString
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.ServerID, &d.URL, &d.MainKeyword, &d.TargetCountry, &d.TargetLanguage, &d.ExcludeDomains, &sb, &d.Status, &d.LastGenerationID, &d.PublishedAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
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
	err := s.db.QueryRowContext(ctx, `SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, published_at, created_at, updated_at FROM domains WHERE id=$1`, id).
		Scan(&d.ID, &d.ProjectID, &d.ServerID, &d.URL, &d.MainKeyword, &d.TargetCountry, &d.TargetLanguage, &d.ExcludeDomains, &sb, &d.Status, &d.LastGenerationID, &d.PublishedAt, &d.CreatedAt, &d.UpdatedAt)
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

func (s *DomainStore) UpdateKeyword(ctx context.Context, id, keyword string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains SET main_keyword=$1, updated_at=NOW() WHERE id=$2`, keyword, id)
	return err
}

func (s *DomainStore) SetLastGeneration(ctx context.Context, id, genID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE domains SET last_generation_id=$1, updated_at=NOW() WHERE id=$2`, genID, id)
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

type GenerationStore struct {
	db *sql.DB
}

func NewGenerationStore(db *sql.DB) *GenerationStore {
	return &GenerationStore{db: db}
}

func (s *GenerationStore) Get(ctx context.Context, id string) (Generation, error) {
	var g Generation
	var logs, artifacts, checkpoint sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, requested_by, status, progress, error, logs, artifacts, checkpoint_data, started_at, finished_at, prompt_id, created_at, updated_at FROM generations WHERE id=$1`, id).
		Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &checkpoint, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return Generation{}, err
	}
	if logs.Valid {
		g.Logs = []byte(logs.String)
	}
	if artifacts.Valid {
		g.Artifacts = []byte(artifacts.String)
	}
	if checkpoint.Valid {
		g.CheckpointData = []byte(checkpoint.String)
	}
	return g, nil
}

func (s *GenerationStore) Create(ctx context.Context, g Generation) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO generations(id, domain_id, requested_by, status, progress, error, logs, artifacts, checkpoint_data, started_at, finished_at, created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())`,
		g.ID, g.DomainID, nullableString(g.RequestedBy), g.Status, g.Progress, nullableString(g.Error), nullableBytes(g.Logs), nullableBytes(g.Artifacts), nullableBytes(g.CheckpointData), nullableTime(g.StartedAt), nullableTime(g.FinishedAt))
	if err != nil {
		return fmt.Errorf("failed to create generation: %w", err)
	}
	return nil
}

func (s *GenerationStore) ListByDomain(ctx context.Context, domainID string) ([]Generation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, requested_by, status, progress, error, logs, artifacts, checkpoint_data, started_at, finished_at, prompt_id, created_at, updated_at FROM generations WHERE domain_id=$1 ORDER BY created_at DESC`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Generation
	for rows.Next() {
		var g Generation
		var logs, artifacts, checkpoint sql.NullString
		if err := rows.Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &checkpoint, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		if logs.Valid {
			g.Logs = []byte(logs.String)
		}
		if artifacts.Valid {
			g.Artifacts = []byte(artifacts.String)
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

func (s *GenerationStore) UpdateFull(ctx context.Context, id, status string, progress int, errText *string, logs, artifacts []byte, started, finished *time.Time, promptID *string) error {
	var errVal sql.NullString
	if errText != nil {
		errVal = sql.NullString{String: *errText, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `UPDATE generations SET status=$1, progress=$2, error=$3, logs=$4, artifacts=$5, started_at=$6, finished_at=$7, prompt_id=$8, updated_at=NOW() WHERE id=$9`,
		status, progress, errVal, nullableBytes(logs), nullableBytes(artifacts), nullableTime(nullTime(started)), nullableTime(nullTime(finished)), nullableString(nullString(promptID)), id)
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
	var logs, artifacts, checkpoint sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, requested_by, status, progress, error, logs, artifacts, checkpoint_data, started_at, finished_at, prompt_id, created_at, updated_at
		FROM generations WHERE domain_id=$1 AND status='success' ORDER BY created_at DESC LIMIT 1`, domainID).
		Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &checkpoint, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return Generation{}, err
	}
	if logs.Valid {
		g.Logs = []byte(logs.String)
	}
	if artifacts.Valid {
		g.Artifacts = []byte(artifacts.String)
	}
	if checkpoint.Valid {
		g.CheckpointData = []byte(checkpoint.String)
	}
	return g, nil
}

// GetLastByDomain возвращает последнюю генерацию домена независимо от статуса.
func (s *GenerationStore) GetLastByDomain(ctx context.Context, domainID string) (Generation, error) {
	var g Generation
	var logs, artifacts, checkpoint sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, domain_id, requested_by, status, progress, error, logs, artifacts, checkpoint_data, started_at, finished_at, prompt_id, created_at, updated_at
		FROM generations WHERE domain_id=$1 ORDER BY created_at DESC LIMIT 1`, domainID).
		Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &checkpoint, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return Generation{}, err
	}
	if logs.Valid {
		g.Logs = []byte(logs.String)
	}
	if artifacts.Valid {
		g.Artifacts = []byte(artifacts.String)
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

func (s *GenerationStore) ListRecentByUser(ctx context.Context, email string, limit int) ([]Generation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT g.id, g.domain_id, g.requested_by, g.status, g.progress, g.error, g.logs, g.artifacts, g.checkpoint_data, g.started_at, g.finished_at, g.prompt_id, g.created_at, g.updated_at
FROM generations g
JOIN domains d ON g.domain_id = d.id
JOIN projects p ON d.project_id = p.id
WHERE p.user_email = $1
   OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_email = $1)
ORDER BY g.created_at DESC
LIMIT $2`, email, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Generation
	for rows.Next() {
		var g Generation
		var logs, artifacts, checkpoint sql.NullString
		if err := rows.Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &checkpoint, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		if logs.Valid {
			g.Logs = []byte(logs.String)
		}
		if artifacts.Valid {
			g.Artifacts = []byte(artifacts.String)
		}
		if checkpoint.Valid {
			g.CheckpointData = []byte(checkpoint.String)
		}
		res = append(res, g)
	}
	return res, rows.Err()
}

func (s *GenerationStore) ListRecentAll(ctx context.Context, limit int) ([]Generation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT g.id, g.domain_id, g.requested_by, g.status, g.progress, g.error, g.logs, g.artifacts, g.checkpoint_data, g.started_at, g.finished_at, g.prompt_id, g.created_at, g.updated_at
FROM generations g
ORDER BY g.created_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Generation
	for rows.Next() {
		var g Generation
		var logs, artifacts, checkpoint sql.NullString
		if err := rows.Scan(&g.ID, &g.DomainID, &g.RequestedBy, &g.Status, &g.Progress, &g.Error, &logs, &artifacts, &checkpoint, &g.StartedAt, &g.FinishedAt, &g.PromptID, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		if logs.Valid {
			g.Logs = []byte(logs.String)
		}
		if artifacts.Valid {
			g.Artifacts = []byte(artifacts.String)
		}
		if checkpoint.Valid {
			g.CheckpointData = []byte(checkpoint.String)
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
