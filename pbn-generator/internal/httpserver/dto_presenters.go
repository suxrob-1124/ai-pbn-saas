package httpserver

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"time"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

type projectDTO struct {
	ID              string          `json:"id"`
	OwnerEmail      string          `json:"owner_email,omitempty"`
	Name            string          `json:"name"`
	Status          string          `json:"status"`
	TargetCountry   string          `json:"target_country,omitempty"`
	TargetLanguage  string          `json:"target_language,omitempty"`
	Timezone        *string         `json:"timezone,omitempty"`
	DefaultServerID *string         `json:"default_server_id,omitempty"`
	GlobalBlacklist json.RawMessage `json:"global_blacklist,omitempty"`
	IndexCheckEnabled bool            `json:"index_check_enabled"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	OwnerHasApiKey    bool            `json:"ownerHasApiKey,omitempty"` // Есть ли API ключ у владельца проекта
}

type domainDTO struct {
	ID                  string     `json:"id"`
	ProjectID           string     `json:"project_id"`
	ServerID            *string    `json:"server_id,omitempty"`
	URL                 string     `json:"url"`
	MainKeyword         string     `json:"main_keyword,omitempty"`
	TargetCountry       string     `json:"target_country,omitempty"`
	TargetLanguage      string     `json:"target_language,omitempty"`
	ExcludeDomains      *string    `json:"exclude_domains,omitempty"`
	Status              string     `json:"status"`
	LastAttemptGenID    *string    `json:"last_attempt_generation_id,omitempty"`
	LastSuccessGenID    *string    `json:"last_success_generation_id,omitempty"`
	PublishedAt         *time.Time `json:"published_at,omitempty"`
	PublishedPath       *string    `json:"published_path,omitempty"`
	FileCount           int        `json:"file_count,omitempty"`
	TotalSizeBytes      int64      `json:"total_size_bytes,omitempty"`
	DeploymentMode      *string    `json:"deployment_mode,omitempty"`
	LinkAnchorText      *string    `json:"link_anchor_text,omitempty"`
	LinkAcceptorURL     *string    `json:"link_acceptor_url,omitempty"`
	LinkStatus          *string    `json:"link_status,omitempty"`
	LinkStatusEffective *string    `json:"link_status_effective,omitempty"`
	LinkStatusSource    string     `json:"link_status_source,omitempty"`
	LinkUpdatedAt       *time.Time `json:"link_updated_at,omitempty"`
	LinkLastTaskID      *string    `json:"link_last_task_id,omitempty"`
	LinkFilePath        *string    `json:"link_file_path,omitempty"`
	LinkAnchorSnapshot  *string    `json:"link_anchor_snapshot,omitempty"`
	LinkReadyAt         *time.Time `json:"link_ready_at,omitempty"`
	IndexCheckEnabled   bool       `json:"index_check_enabled"`
	GenerationType      string     `json:"generation_type"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type fileDTO struct {
	ID           string     `json:"id"`
	Path         string     `json:"path"`
	Size         int64      `json:"size"`
	MimeType     string     `json:"mimeType"`
	Version      int        `json:"version"`
	IsEditable   bool       `json:"isEditable"`
	IsBinary     bool       `json:"isBinary"`
	Width        *int       `json:"width,omitempty"`
	Height       *int       `json:"height,omitempty"`
	LastEditedBy *string    `json:"lastEditedBy,omitempty"`
	DeletedAt    *time.Time `json:"deletedAt,omitempty"`
	DeletedBy    *string    `json:"deletedBy,omitempty"`
	DeleteReason *string    `json:"deleteReason,omitempty"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type fileEditDTO struct {
	ID          string    `json:"id"`
	EditedBy    string    `json:"editedBy"`
	EditType    string    `json:"editType"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type fileRevisionDTO struct {
	ID          string    `json:"id"`
	FileID      string    `json:"file_id"`
	Version     int       `json:"version"`
	EditedBy    string    `json:"edited_by"`
	Source      string    `json:"source"`
	Description string    `json:"description,omitempty"`
	ContentHash string    `json:"content_hash"`
	SizeBytes   int64     `json:"size_bytes"`
	MimeType    string    `json:"mime_type"`
	Content     string    `json:"content,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type linkTaskDTO struct {
	ID               string     `json:"id"`
	DomainID         string     `json:"domain_id"`
	AnchorText       string     `json:"anchor_text"`
	TargetURL        string     `json:"target_url"`
	ScheduledFor     time.Time  `json:"scheduled_for"`
	Action           string     `json:"action"`
	Status           string     `json:"status"`
	FoundLocation    *string    `json:"found_location,omitempty"`
	GeneratedContent *string    `json:"generated_content,omitempty"`
	ErrorMessage     *string    `json:"error_message,omitempty"`
	LogLines         []string   `json:"log_lines,omitempty"`
	Attempts         int        `json:"attempts"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

type indexCheckDTO struct {
	ID               string     `json:"id"`
	DomainID         string     `json:"domain_id"`
	ProjectID        *string    `json:"project_id,omitempty"`
	DomainURL        *string    `json:"domain_url,omitempty"`
	CheckDate        time.Time  `json:"check_date"`
	Status           string     `json:"status"`
	IsIndexed        *bool      `json:"is_indexed,omitempty"`
	ContentQuote     *string    `json:"content_quote,omitempty"`
	IsContentIndexed *bool      `json:"is_content_indexed,omitempty"`
	Attempts         int        `json:"attempts"`
	LastAttemptAt    *time.Time `json:"last_attempt_at,omitempty"`
	NextRetryAt      *time.Time `json:"next_retry_at,omitempty"`
	ErrorMessage     *string    `json:"error_message,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	RunNowEnqueued   *bool      `json:"run_now_enqueued,omitempty"`
	RunNowError      *string    `json:"run_now_error,omitempty"`
}

type indexCheckListDTO struct {
	Items []indexCheckDTO `json:"items"`
	Total int             `json:"total"`
}

type indexCheckHistoryDTO struct {
	ID            string          `json:"id"`
	CheckID       string          `json:"check_id"`
	AttemptNumber int             `json:"attempt_number"`
	Result        *string         `json:"result,omitempty"`
	ResponseData  json.RawMessage `json:"response_data,omitempty"`
	ErrorMessage  *string         `json:"error_message,omitempty"`
	DurationMS    *int64          `json:"duration_ms,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type indexCheckDailyDTO struct {
	Date                string `json:"date"`
	Total               int    `json:"total"`
	IndexedTrue         int    `json:"indexed_true"`
	IndexedFalse        int    `json:"indexed_false"`
	Pending             int    `json:"pending"`
	Checking            int    `json:"checking"`
	FailedInvestigation int    `json:"failed_investigation"`
	Success             int    `json:"success"`
}

type indexCheckStatsDTO struct {
	From                 string               `json:"from"`
	To                   string               `json:"to"`
	TotalChecks          int                  `json:"total_checks"`
	TotalResolved        int                  `json:"total_resolved"`
	IndexedTrue          int                  `json:"indexed_true"`
	PercentIndexed       int                  `json:"percent_indexed"`
	AvgAttemptsToSuccess float64              `json:"avg_attempts_to_success"`
	FailedInvestigation  int                  `json:"failed_investigation"`
	Daily                []indexCheckDailyDTO `json:"daily"`
}

type llmUsageEventDTO struct {
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	Provider         string    `json:"provider"`
	Operation        string    `json:"operation"`
	Stage            *string   `json:"stage,omitempty"`
	Model            string    `json:"model"`
	Status           string    `json:"status"`
	RequesterEmail   string    `json:"requester_email"`
	KeyOwnerEmail    *string   `json:"key_owner_email,omitempty"`
	KeyType          *string   `json:"key_type,omitempty"`
	ProjectID        *string   `json:"project_id,omitempty"`
	DomainID         *string   `json:"domain_id,omitempty"`
	GenerationID     *string   `json:"generation_id,omitempty"`
	LinkTaskID       *string   `json:"link_task_id,omitempty"`
	FilePath         *string   `json:"file_path,omitempty"`
	PromptTokens     *int64    `json:"prompt_tokens,omitempty"`
	CompletionTokens *int64    `json:"completion_tokens,omitempty"`
	TotalTokens      *int64    `json:"total_tokens,omitempty"`
	TokenSource      string    `json:"token_source"`
	EstimatedCostUSD *float64  `json:"estimated_cost_usd,omitempty"`
}

type llmUsageListDTO struct {
	Items []llmUsageEventDTO `json:"items"`
	Total int                `json:"total"`
}

type llmUsageBucketDTO struct {
	Key      string  `json:"key"`
	Requests int     `json:"requests"`
	Tokens   int64   `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
}

type llmUsageStatsDTO struct {
	TotalRequests int                 `json:"total_requests"`
	TotalTokens   int64               `json:"total_tokens"`
	TotalCostUSD  float64             `json:"total_cost_usd"`
	ByDay         []llmUsageBucketDTO `json:"by_day"`
	ByModel       []llmUsageBucketDTO `json:"by_model"`
	ByOperation   []llmUsageBucketDTO `json:"by_operation"`
	ByUser        []llmUsageBucketDTO `json:"by_user"`
}

type llmPricingDTO struct {
	ID                  string     `json:"id"`
	Provider            string     `json:"provider"`
	Model               string     `json:"model"`
	InputUSDPerMillion  float64    `json:"input_usd_per_million"`
	OutputUSDPerMillion float64    `json:"output_usd_per_million"`
	ActiveFrom          time.Time  `json:"active_from"`
	ActiveTo            *time.Time `json:"active_to,omitempty"`
	IsActive            bool       `json:"is_active"`
	UpdatedBy           string     `json:"updated_by"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type generationDTO struct {
	ID               string     `json:"id"`
	DomainID         string     `json:"domain_id"`
	DomainURL        *string    `json:"domain_url,omitempty"`
	Status           string     `json:"status"`
	Progress         int        `json:"progress"`
	Error            *string    `json:"error,omitempty"`
	PromptID         *string    `json:"prompt_id,omitempty"`
	GenerationType   string     `json:"generation_type,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	Logs             any        `json:"logs,omitempty"`
	Artifacts        any        `json:"artifacts,omitempty"`
	ArtifactsSummary any        `json:"artifacts_summary,omitempty"`
}

type promptOverrideDTO struct {
	ID            string    `json:"id"`
	ScopeType     string    `json:"scope_type"`
	ScopeID       string    `json:"scope_id"`
	Stage         string    `json:"stage"`
	Body          string    `json:"body"`
	Model         *string   `json:"model,omitempty"`
	BasedOnPrompt *string   `json:"based_on_prompt_id,omitempty"`
	UpdatedBy     string    `json:"updated_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type resolvedPromptDTO struct {
	Stage           string  `json:"stage"`
	Source          string  `json:"source"`
	PromptID        *string `json:"prompt_id,omitempty"`
	OverrideID      *string `json:"override_id,omitempty"`
	Body            string  `json:"body"`
	Model           *string `json:"model,omitempty"`
	BasedOnPromptID *string `json:"based_on_prompt_id,omitempty"`
}

type domainPromptsDTO struct {
	Overrides []promptOverrideDTO `json:"overrides"`
	Resolved  []resolvedPromptDTO `json:"resolved"`
}

type deploymentAttemptDTO struct {
	ID             string     `json:"id"`
	DomainID       string     `json:"domain_id"`
	GenerationID   string     `json:"generation_id"`
	Mode           string     `json:"mode"`
	TargetPath     string     `json:"target_path"`
	OwnerBefore    *string    `json:"owner_before,omitempty"`
	OwnerAfter     *string    `json:"owner_after,omitempty"`
	Status         string     `json:"status"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	FileCount      int        `json:"file_count"`
	TotalSizeBytes int64      `json:"total_size_bytes"`
	CreatedAt      time.Time  `json:"created_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

type dashboardStatsDTO struct {
	Pending    int  `json:"pending"`
	Processing int  `json:"processing"`
	Error      int  `json:"error"`
	AvgMinutes *int `json:"avg_minutes,omitempty"`
	AvgSample  int  `json:"avg_sample"`
}

type dashboardDTO struct {
	Projects     []projectDTO      `json:"projects"`
	Stats        dashboardStatsDTO `json:"stats"`
	RecentErrors []generationDTO   `json:"recent_errors"`
}

type adminUserDTO struct {
	Email           string     `json:"email"`
	Name            string     `json:"name,omitempty"`
	Role            string     `json:"role"`
	IsApproved      bool       `json:"isApproved"`
	Verified        bool       `json:"verified"`
	CreatedAt       time.Time  `json:"createdAt"`
	APIKeyUpdatedAt *time.Time `json:"apiKeyUpdatedAt,omitempty"`
}

type adminPromptDTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Body        string    `json:"body"`
	Stage       *string   `json:"stage,omitempty"`
	Model       *string   `json:"model,omitempty"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type projectMemberDTO struct {
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type scheduleDTO struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"project_id"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Strategy    string          `json:"strategy"`
	Config      json.RawMessage `json:"config"`
	IsActive    bool            `json:"isActive"`
	CreatedBy   string          `json:"createdBy"`
	LastRunAt   *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time      `json:"next_run_at,omitempty"`
	Timezone    *string         `json:"timezone,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type linkScheduleDTO struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	IsActive  bool            `json:"isActive"`
	CreatedBy string          `json:"createdBy"`
	LastRunAt *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt *time.Time      `json:"next_run_at,omitempty"`
	Timezone  *string         `json:"timezone,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type queueItemDTO struct {
	ID           string     `json:"id"`
	DomainID     string     `json:"domain_id"`
	DomainURL    *string    `json:"domain_url,omitempty"`
	ScheduleID   *string    `json:"schedule_id,omitempty"`
	Priority     int        `json:"priority"`
	ScheduledFor time.Time  `json:"scheduled_for"`
	Status       string     `json:"status"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
}

type projectSummaryDTO struct {
	Project projectDTO         `json:"project"`
	Domains []domainDTO        `json:"domains"`
	Members []projectMemberDTO `json:"members"`
	MyRole  string             `json:"my_role"`
}

type domainSummaryDTO struct {
	Domain        domainDTO       `json:"domain"`
	ProjectName   string          `json:"project_name"`
	Generations   []generationDTO `json:"generations"`
	LatestAttempt *generationDTO  `json:"latest_attempt,omitempty"`
	LatestSuccess *generationDTO  `json:"latest_success,omitempty"`
	LinkTasks     []linkTaskDTO   `json:"link_tasks"`
	MyRole        string          `json:"my_role"`
}

type adminAuditRuleDTO struct {
	Code        string    `json:"code"`
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	Severity    string    `json:"severity"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (s *Server) toProjectDTO(ctx context.Context, p sqlstore.Project) projectDTO {
	var gb json.RawMessage
	if len(p.GlobalBlacklist) > 0 {
		gb = json.RawMessage(p.GlobalBlacklist)
	}

	// Проверяем наличие API ключа у владельца проекта
	ownerHasApiKey := false
	if encKey, err := s.svc.GetUserAPIKeyEncrypted(ctx, p.UserEmail); err == nil && len(encKey) > 0 {
		ownerHasApiKey = true
	}

	return projectDTO{
		ID:                p.ID,
		OwnerEmail:        p.UserEmail,
		Name:              p.Name,
		Status:            p.Status,
		TargetCountry:     p.TargetCountry,
		TargetLanguage:    p.TargetLanguage,
		Timezone:          nullableStringPtr(p.Timezone),
		DefaultServerID:   nullableStringPtr(p.DefaultServerID),
		GlobalBlacklist:   gb,
		IndexCheckEnabled: p.IndexCheckEnabled,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
		OwnerHasApiKey:    ownerHasApiKey,
	}
}

func toProjectDTO(p sqlstore.Project) projectDTO {
	// Deprecated: используйте s.toProjectDTO для получения ownerHasApiKey
	var gb json.RawMessage
	if len(p.GlobalBlacklist) > 0 {
		gb = json.RawMessage(p.GlobalBlacklist)
	}
	return projectDTO{
		ID:                p.ID,
		OwnerEmail:        p.UserEmail,
		Name:              p.Name,
		Status:            p.Status,
		TargetCountry:     p.TargetCountry,
		TargetLanguage:    p.TargetLanguage,
		Timezone:          nullableStringPtr(p.Timezone),
		DefaultServerID:   nullableStringPtr(p.DefaultServerID),
		GlobalBlacklist:   gb,
		IndexCheckEnabled: p.IndexCheckEnabled,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}
}

func toProjectDTOs(list []sqlstore.Project) []projectDTO {
	out := make([]projectDTO, 0, len(list))
	for _, p := range list {
		out = append(out, toProjectDTO(p))
	}
	return out
}

func toDomainDTO(d sqlstore.Domain) domainDTO {
	dto := domainDTO{
		ID:                 d.ID,
		ProjectID:          d.ProjectID,
		ServerID:           nullableStringPtr(d.ServerID),
		URL:                d.URL,
		MainKeyword:        d.MainKeyword,
		TargetCountry:      d.TargetCountry,
		TargetLanguage:     d.TargetLanguage,
		ExcludeDomains:     nullableStringPtr(d.ExcludeDomains),
		Status:             stableDomainStatusFromDomain(d),
		LastAttemptGenID:   nullableStringPtr(d.LastGenerationID),
		LastSuccessGenID:   nullableStringPtr(d.LastSuccessGenID),
		PublishedAt:        nullableTimePtr(d.PublishedAt),
		PublishedPath:      nullableStringPtr(d.PublishedPath),
		FileCount:          d.FileCount,
		TotalSizeBytes:     d.TotalSizeBytes,
		DeploymentMode:     nullableStringPtr(d.DeploymentMode),
		LinkAnchorText:     nullableStringPtr(d.LinkAnchorText),
		LinkAcceptorURL:    nullableStringPtr(d.LinkAcceptorURL),
		LinkStatus:         nullableStringPtr(d.LinkStatus),
		LinkUpdatedAt:      nullableTimePtr(d.LinkUpdatedAt),
		LinkLastTaskID:     nullableStringPtr(d.LinkLastTaskID),
		LinkFilePath:       nullableStringPtr(d.LinkFilePath),
		LinkAnchorSnapshot: nullableStringPtr(d.LinkAnchorSnapshot),
		LinkReadyAt:        nullableTimePtr(d.LinkReadyAt),
		IndexCheckEnabled:  d.IndexCheckEnabled,
		GenerationType:     d.GenerationType,
		CreatedAt:          d.CreatedAt,
		UpdatedAt:          d.UpdatedAt,
	}
	return dto
}

func toDomainDTOs(list []sqlstore.Domain) []domainDTO {
	out := make([]domainDTO, 0, len(list))
	for _, d := range list {
		out = append(out, toDomainDTO(d))
	}
	return out
}

func linkTaskStatusPriority(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "removing":
		return 3
	case "searching":
		return 2
	case "pending":
		return 1
	default:
		return 0
	}
}

func stringPtr(value string) *string {
	return &value
}

// domains.link_status может временно отставать между scheduler и worker,
// поэтому UI должен опираться на вычисленный link_status_effective.
func resolveEffectiveLinkStatus(rawStatus *string, activeTaskStatus *string) (*string, string) {
	var raw string
	if rawStatus != nil {
		raw = strings.TrimSpace(*rawStatus)
	}
	var active string
	if activeTaskStatus != nil {
		active = strings.TrimSpace(*activeTaskStatus)
	}
	if active != "" && linkTaskStatusPriority(active) > 0 {
		return stringPtr(active), "active_task"
	}
	if raw != "" {
		return stringPtr(raw), "domain"
	}
	return nil, "domain"
}

func applyEffectiveLinkStatus(dto *domainDTO, activeTask *sqlstore.LinkTask) {
	if dto == nil {
		return
	}
	var active *string
	if activeTask != nil {
		active = stringPtr(activeTask.Status)
	}
	effective, source := resolveEffectiveLinkStatus(dto.LinkStatus, active)
	dto.LinkStatusEffective = effective
	dto.LinkStatusSource = source
}

func toIndexCheckDTO(check sqlstore.IndexCheck) indexCheckDTO {
	dto := indexCheckDTO{
		ID:        check.ID,
		DomainID:  check.DomainID,
		CheckDate: check.CheckDate,
		Status:    check.Status,
		Attempts:  check.Attempts,
		CreatedAt: check.CreatedAt,
	}
	if check.IsIndexed.Valid {
		val := check.IsIndexed.Bool
		dto.IsIndexed = &val
	}
	if check.ContentQuote.Valid && check.ContentQuote.String != "" {
		val := check.ContentQuote.String
		dto.ContentQuote = &val
	}
	if check.IsContentIndexed.Valid {
		val := check.IsContentIndexed.Bool
		dto.IsContentIndexed = &val
	}
	if check.LastAttemptAt.Valid {
		val := check.LastAttemptAt.Time
		dto.LastAttemptAt = &val
	}
	if check.NextRetryAt.Valid {
		val := check.NextRetryAt.Time
		dto.NextRetryAt = &val
	}
	if check.ErrorMessage.Valid {
		val := check.ErrorMessage.String
		dto.ErrorMessage = &val
	}
	if check.CompletedAt.Valid {
		val := check.CompletedAt.Time
		dto.CompletedAt = &val
	}
	return dto
}

func toIndexCheckHistoryDTO(item sqlstore.CheckHistory) indexCheckHistoryDTO {
	dto := indexCheckHistoryDTO{
		ID:            item.ID,
		CheckID:       item.CheckID,
		AttemptNumber: item.AttemptNumber,
		CreatedAt:     item.CreatedAt,
	}
	if item.Result.Valid {
		val := item.Result.String
		dto.Result = &val
	}
	if len(item.ResponseData) > 0 {
		dto.ResponseData = json.RawMessage(item.ResponseData)
	}
	if item.ErrorMessage.Valid {
		val := item.ErrorMessage.String
		dto.ErrorMessage = &val
	}
	if item.DurationMS.Valid {
		val := item.DurationMS.Int64
		dto.DurationMS = &val
	}
	return dto
}

func toIndexCheckDailyDTO(item sqlstore.IndexCheckDailySummary) indexCheckDailyDTO {
	return indexCheckDailyDTO{
		Date:                item.Date.Format("2006-01-02"),
		Total:               item.Total,
		IndexedTrue:         item.IndexedTrue,
		IndexedFalse:        item.IndexedFalse,
		Pending:             item.Pending,
		Checking:            item.Checking,
		FailedInvestigation: item.FailedInvestigation,
		Success:             item.Success,
	}
}

func buildIndexCheckStatsDTO(
	stats sqlstore.IndexCheckStats,
	daily []sqlstore.IndexCheckDailySummary,
	from time.Time,
	to time.Time,
) indexCheckStatsDTO {
	percent := 0
	if stats.TotalResolved > 0 {
		percent = int(math.Round(float64(stats.IndexedTrue) / float64(stats.TotalResolved) * 100))
	}
	resp := indexCheckStatsDTO{
		From:                 from.Format("2006-01-02"),
		To:                   to.Format("2006-01-02"),
		TotalChecks:          stats.TotalChecks,
		TotalResolved:        stats.TotalResolved,
		IndexedTrue:          stats.IndexedTrue,
		PercentIndexed:       percent,
		AvgAttemptsToSuccess: stats.AvgAttemptsToSuccess,
		FailedInvestigation:  stats.FailedInvestigation,
		Daily:                []indexCheckDailyDTO{},
	}
	if len(daily) > 0 {
		resp.Daily = make([]indexCheckDailyDTO, 0, len(daily))
		for _, item := range daily {
			resp.Daily = append(resp.Daily, toIndexCheckDailyDTO(item))
		}
	}
	return resp
}

func toLLMUsageEventDTO(item sqlstore.LLMUsageEvent) llmUsageEventDTO {
	dto := llmUsageEventDTO{
		ID:             item.ID,
		CreatedAt:      item.CreatedAt,
		Provider:       item.Provider,
		Operation:      item.Operation,
		Model:          item.Model,
		Status:         item.Status,
		RequesterEmail: item.RequesterEmail,
		TokenSource:    item.TokenSource,
	}
	if item.Stage.Valid {
		v := strings.TrimSpace(item.Stage.String)
		if v != "" {
			dto.Stage = &v
		}
	}
	if item.KeyOwnerEmail.Valid {
		v := strings.TrimSpace(item.KeyOwnerEmail.String)
		if v != "" {
			dto.KeyOwnerEmail = &v
		}
	}
	if item.KeyType.Valid {
		v := strings.TrimSpace(item.KeyType.String)
		if v != "" {
			dto.KeyType = &v
		}
	}
	if item.ProjectID.Valid {
		v := strings.TrimSpace(item.ProjectID.String)
		if v != "" {
			dto.ProjectID = &v
		}
	}
	if item.DomainID.Valid {
		v := strings.TrimSpace(item.DomainID.String)
		if v != "" {
			dto.DomainID = &v
		}
	}
	if item.GenerationID.Valid {
		v := strings.TrimSpace(item.GenerationID.String)
		if v != "" {
			dto.GenerationID = &v
		}
	}
	if item.LinkTaskID.Valid {
		v := strings.TrimSpace(item.LinkTaskID.String)
		if v != "" {
			dto.LinkTaskID = &v
		}
	}
	if item.FilePath.Valid {
		v := strings.TrimSpace(item.FilePath.String)
		if v != "" {
			dto.FilePath = &v
		}
	}
	if item.PromptTokens.Valid {
		v := item.PromptTokens.Int64
		dto.PromptTokens = &v
	}
	if item.CompletionTokens.Valid {
		v := item.CompletionTokens.Int64
		dto.CompletionTokens = &v
	}
	if item.TotalTokens.Valid {
		v := item.TotalTokens.Int64
		dto.TotalTokens = &v
	}
	if item.EstimatedCostUSD.Valid {
		v := item.EstimatedCostUSD.Float64
		dto.EstimatedCostUSD = &v
	}
	return dto
}

func toLLMUsageStatsDTO(stats sqlstore.LLMUsageStats) llmUsageStatsDTO {
	dto := llmUsageStatsDTO{
		TotalRequests: stats.Totals.TotalRequests,
		TotalTokens:   stats.Totals.TotalTokens,
		TotalCostUSD:  stats.Totals.TotalCostUSD,
		ByDay:         make([]llmUsageBucketDTO, 0, len(stats.ByDay)),
		ByModel:       make([]llmUsageBucketDTO, 0, len(stats.ByModel)),
		ByOperation:   make([]llmUsageBucketDTO, 0, len(stats.ByOperation)),
		ByUser:        make([]llmUsageBucketDTO, 0, len(stats.ByUser)),
	}
	for _, item := range stats.ByDay {
		dto.ByDay = append(dto.ByDay, llmUsageBucketDTO{Key: item.Key, Requests: item.Requests, Tokens: item.Tokens, CostUSD: item.CostUSD})
	}
	for _, item := range stats.ByModel {
		dto.ByModel = append(dto.ByModel, llmUsageBucketDTO{Key: item.Key, Requests: item.Requests, Tokens: item.Tokens, CostUSD: item.CostUSD})
	}
	for _, item := range stats.ByOperation {
		dto.ByOperation = append(dto.ByOperation, llmUsageBucketDTO{Key: item.Key, Requests: item.Requests, Tokens: item.Tokens, CostUSD: item.CostUSD})
	}
	for _, item := range stats.ByUser {
		dto.ByUser = append(dto.ByUser, llmUsageBucketDTO{Key: item.Key, Requests: item.Requests, Tokens: item.Tokens, CostUSD: item.CostUSD})
	}
	return dto
}

func toLLMPricingDTO(item sqlstore.LLMModelPricing) llmPricingDTO {
	dto := llmPricingDTO{
		ID:                  item.ID,
		Provider:            item.Provider,
		Model:               item.Model,
		InputUSDPerMillion:  item.InputUSDPerMillion,
		OutputUSDPerMillion: item.OutputUSDPerMillion,
		ActiveFrom:          item.ActiveFrom,
		IsActive:            item.IsActive,
		UpdatedBy:           item.UpdatedBy,
		UpdatedAt:           item.UpdatedAt,
	}
	if item.ActiveTo.Valid {
		v := item.ActiveTo.Time
		dto.ActiveTo = &v
	}
	return dto
}

func toGenerationDTO(g sqlstore.Generation) generationDTO {
	return generationDTO{
		ID:               g.ID,
		DomainID:         g.DomainID,
		Status:           g.Status,
		Progress:         g.Progress,
		Error:            nullableStringPtr(g.Error),
		PromptID:         nullableStringPtr(g.PromptID),
		GenerationType:   g.GenerationType,
		CreatedAt:        g.CreatedAt,
		UpdatedAt:        g.UpdatedAt,
		StartedAt:        nullableTimePtr(g.StartedAt),
		FinishedAt:       nullableTimePtr(g.FinishedAt),
		Logs:             rawJSONOrNil(g.Logs),
		Artifacts:        rawJSONOrNil(g.Artifacts),
		ArtifactsSummary: rawJSONOrNil(g.ArtifactsSummary),
	}
}

func toGenerationLightDTO(g sqlstore.Generation) generationDTO {
	return generationDTO{
		ID:               g.ID,
		DomainID:         g.DomainID,
		Status:           g.Status,
		Progress:         g.Progress,
		Error:            nullableStringPtr(g.Error),
		PromptID:         nullableStringPtr(g.PromptID),
		GenerationType:   g.GenerationType,
		CreatedAt:        g.CreatedAt,
		UpdatedAt:        g.UpdatedAt,
		StartedAt:        nullableTimePtr(g.StartedAt),
		FinishedAt:       nullableTimePtr(g.FinishedAt),
		ArtifactsSummary: rawJSONOrNil(g.ArtifactsSummary),
	}
}

func toGenerationDTOs(list []sqlstore.Generation) []generationDTO {
	out := make([]generationDTO, 0, len(list))
	for _, g := range list {
		out = append(out, toGenerationDTO(g))
	}
	return out
}

func toGenerationDTOWithDomain(g sqlstore.Generation, domainURL *string) generationDTO {
	dto := toGenerationDTO(g)
	dto.DomainURL = domainURL
	return dto
}

func toPromptOverrideDTO(item sqlstore.PromptOverride) promptOverrideDTO {
	return promptOverrideDTO{
		ID:            item.ID,
		ScopeType:     item.ScopeType,
		ScopeID:       item.ScopeID,
		Stage:         item.Stage,
		Body:          item.Body,
		Model:         nullableStringPtr(item.Model),
		BasedOnPrompt: nullableStringPtr(item.BasedOnPrompt),
		UpdatedBy:     item.UpdatedBy,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func toPromptOverrideDTOs(list []sqlstore.PromptOverride) []promptOverrideDTO {
	out := make([]promptOverrideDTO, 0, len(list))
	for _, item := range list {
		out = append(out, toPromptOverrideDTO(item))
	}
	return out
}

func toResolvedPromptDTO(item sqlstore.ResolvedPrompt) resolvedPromptDTO {
	return resolvedPromptDTO{
		Stage:           item.Stage,
		Source:          item.Source,
		PromptID:        nullableStringPtr(item.PromptID),
		OverrideID:      nullableStringPtr(item.OverrideID),
		Body:            item.Body,
		Model:           nullableStringPtr(item.Model),
		BasedOnPromptID: nullableStringPtr(item.BasedOnPromptID),
	}
}

func toDeploymentAttemptDTO(item sqlstore.DeploymentAttempt) deploymentAttemptDTO {
	return deploymentAttemptDTO{
		ID:             item.ID,
		DomainID:       item.DomainID,
		GenerationID:   item.GenerationID,
		Mode:           item.Mode,
		TargetPath:     item.TargetPath,
		OwnerBefore:    nullableStringPtr(item.OwnerBefore),
		OwnerAfter:     nullableStringPtr(item.OwnerAfter),
		Status:         item.Status,
		ErrorMessage:   nullableStringPtr(item.ErrorMessage),
		FileCount:      item.FileCount,
		TotalSizeBytes: item.TotalSizeBytes,
		CreatedAt:      item.CreatedAt,
		FinishedAt:     nullableTimePtr(item.FinishedAt),
	}
}

func toAdminUserDTO(u auth.User) adminUserDTO {
	return adminUserDTO{
		Email:           u.Email,
		Name:            u.Name,
		Role:            u.Role,
		IsApproved:      u.IsApproved,
		Verified:        u.Verified,
		CreatedAt:       u.CreatedAt,
		APIKeyUpdatedAt: u.APIKeySetAt,
	}
}

func toAdminPromptDTO(p sqlstore.SystemPrompt) adminPromptDTO {
	return adminPromptDTO{
		ID:          p.ID,
		Name:        p.Name,
		Description: nullableStringPtr(p.Description),
		Body:        p.Body,
		Stage:       nullableStringPtr(p.Stage),
		Model:       nullableStringPtr(p.Model),
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func toScheduleDTO(s sqlstore.Schedule) scheduleDTO {
	var cfg json.RawMessage
	if len(s.Config) > 0 {
		cfg = json.RawMessage(s.Config)
	}
	return scheduleDTO{
		ID:          s.ID,
		ProjectID:   s.ProjectID,
		Name:        s.Name,
		Description: nullableStringPtr(s.Description),
		Strategy:    s.Strategy,
		Config:      cfg,
		IsActive:    s.IsActive,
		CreatedBy:   s.CreatedBy,
		LastRunAt:   nullableTimePtr(s.LastRunAt),
		NextRunAt:   nullableTimePtr(s.NextRunAt),
		Timezone:    nullableStringPtr(s.Timezone),
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func toLinkScheduleDTO(s sqlstore.LinkSchedule) linkScheduleDTO {
	return linkScheduleDTO{
		ID:        s.ID,
		ProjectID: s.ProjectID,
		Name:      s.Name,
		Config:    s.Config,
		IsActive:  s.IsActive,
		CreatedBy: s.CreatedBy,
		LastRunAt: nullableTimePtr(s.LastRunAt),
		NextRunAt: nullableTimePtr(s.NextRunAt),
		Timezone:  nullableStringPtr(s.Timezone),
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func toQueueItemDTO(item sqlstore.QueueItem) queueItemDTO {
	return queueItemDTO{
		ID:           item.ID,
		DomainID:     item.DomainID,
		ScheduleID:   nullableStringPtr(item.ScheduleID),
		Priority:     item.Priority,
		ScheduledFor: item.ScheduledFor,
		Status:       item.Status,
		ErrorMessage: nullableStringPtr(item.ErrorMessage),
		CreatedAt:    item.CreatedAt,
		ProcessedAt:  nullableTimePtr(item.ProcessedAt),
	}
}

func toLinkTaskDTO(task sqlstore.LinkTask) linkTaskDTO {
	return linkTaskDTO{
		ID:               task.ID,
		DomainID:         task.DomainID,
		AnchorText:       task.AnchorText,
		TargetURL:        task.TargetURL,
		ScheduledFor:     task.ScheduledFor,
		Action:           task.Action,
		Status:           task.Status,
		FoundLocation:    nullableStringPtr(task.FoundLocation),
		GeneratedContent: nullableStringPtr(task.GeneratedContent),
		ErrorMessage:     nullableStringPtr(task.ErrorMessage),
		LogLines:         task.LogLines,
		Attempts:         task.Attempts,
		CreatedBy:        task.CreatedBy,
		CreatedAt:        task.CreatedAt,
		CompletedAt:      nullableTimePtr(task.CompletedAt),
	}
}

func toAdminAuditRuleDTO(r sqlstore.AuditRule) adminAuditRuleDTO {
	var desc *string
	if strings.TrimSpace(r.Description) != "" {
		desc = &r.Description
	}
	return adminAuditRuleDTO{
		Code:        r.Code,
		Title:       r.Title,
		Description: desc,
		Severity:    r.Severity,
		IsActive:    r.IsActive,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func toFileDTO(domain sqlstore.Domain, f sqlstore.SiteFile) fileDTO {
	item := fileDTO{
		ID:         f.ID,
		Path:       f.Path,
		Size:       f.SizeBytes,
		MimeType:   f.MimeType,
		Version:    f.Version,
		IsEditable: isEditableMimeType(f.MimeType),
		IsBinary:   isBinaryMimeType(f.MimeType),
		UpdatedAt:  f.UpdatedAt,
	}
	if item.Version <= 0 {
		item.Version = 1
	}
	if f.LastEditedBy.Valid {
		v := strings.TrimSpace(f.LastEditedBy.String)
		if v != "" {
			item.LastEditedBy = &v
		}
	}
	if f.DeletedAt.Valid {
		v := f.DeletedAt.Time
		item.DeletedAt = &v
	}
	if f.DeletedBy.Valid {
		v := strings.TrimSpace(f.DeletedBy.String)
		if v != "" {
			item.DeletedBy = &v
		}
	}
	if f.DeleteReason.Valid {
		v := strings.TrimSpace(f.DeleteReason.String)
		if v != "" {
			item.DeleteReason = &v
		}
	}
	if strings.HasPrefix(strings.ToLower(baseMimeType(f.MimeType)), "image/") {
		if w, h, ok := detectImageDimensions(domain, f.Path); ok {
			item.Width = &w
			item.Height = &h
		}
	}
	return item
}
