package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type LLMUsageEvent struct {
	ID                      string
	Provider                string
	Operation               string
	Stage                   sql.NullString
	Model                   string
	Status                  string
	RequesterEmail          string
	KeyOwnerEmail           sql.NullString
	KeyType                 sql.NullString
	ProjectID               sql.NullString
	DomainID                sql.NullString
	GenerationID            sql.NullString
	LinkTaskID              sql.NullString
	FilePath                sql.NullString
	PromptTokens            sql.NullInt64
	CompletionTokens        sql.NullInt64
	TotalTokens             sql.NullInt64
	TokenSource             string
	InputPriceUSDPerMillion sql.NullFloat64
	OutputPriceUSDPerMillion sql.NullFloat64
	EstimatedCostUSD        sql.NullFloat64
	ErrorMessage            sql.NullString
	CreatedAt               time.Time
}

type LLMUsageFilters struct {
	From      *time.Time
	To        *time.Time
	UserEmail *string
	ProjectID *string
	DomainID  *string
	Model     *string
	Operation *string
	Status    *string
	Limit     int
	Offset    int
}

type LLMUsageTotalStats struct {
	TotalRequests int     `json:"total_requests"`
	TotalTokens   int64   `json:"total_tokens"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
}

type LLMUsageStatsBucket struct {
	Key       string  `json:"key"`
	Requests  int     `json:"requests"`
	Tokens    int64   `json:"tokens"`
	CostUSD   float64 `json:"cost_usd"`
}

type LLMUsageStats struct {
	Totals      LLMUsageTotalStats   `json:"totals"`
	ByDay       []LLMUsageStatsBucket `json:"by_day"`
	ByModel     []LLMUsageStatsBucket `json:"by_model"`
	ByOperation []LLMUsageStatsBucket `json:"by_operation"`
	ByUser      []LLMUsageStatsBucket `json:"by_user"`
}

type LLMUsageStore struct {
	db *sql.DB
}

func NewLLMUsageStore(db *sql.DB) *LLMUsageStore {
	return &LLMUsageStore{db: db}
}

func (s *LLMUsageStore) CreateEvent(ctx context.Context, item LLMUsageEvent) error {
	if strings.TrimSpace(item.ID) == "" {
		item.ID = uuid.NewString()
	}
	createdAt := item.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if strings.TrimSpace(item.Provider) == "" {
		item.Provider = "gemini"
	}
	if strings.TrimSpace(item.TokenSource) == "" {
		item.TokenSource = "estimated"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO llm_usage_events(
	id, provider, operation, stage, model, status, requester_email, key_owner_email, key_type, project_id, domain_id, generation_id, link_task_id, file_path,
	prompt_tokens, completion_tokens, total_tokens, token_source, input_price_usd_per_million, output_price_usd_per_million, estimated_cost_usd, error_message, created_at
) VALUES(
	$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,
	$15,$16,$17,$18,$19,$20,$21,$22,$23
)`,
		item.ID,
		item.Provider,
		item.Operation,
		nullableString(item.Stage),
		item.Model,
		item.Status,
		item.RequesterEmail,
		nullableString(item.KeyOwnerEmail),
		nullableString(item.KeyType),
		nullableString(item.ProjectID),
		nullableString(item.DomainID),
		nullableString(item.GenerationID),
		nullableString(item.LinkTaskID),
		nullableString(item.FilePath),
		nullableLLMInt64(item.PromptTokens),
		nullableLLMInt64(item.CompletionTokens),
		nullableLLMInt64(item.TotalTokens),
		item.TokenSource,
		nullableFloat64(item.InputPriceUSDPerMillion),
		nullableFloat64(item.OutputPriceUSDPerMillion),
		nullableFloat64(item.EstimatedCostUSD),
		nullableString(item.ErrorMessage),
		createdAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create llm usage event: %w", err)
	}
	return nil
}

func (s *LLMUsageStore) ListEvents(ctx context.Context, filters LLMUsageFilters) ([]LLMUsageEvent, error) {
	where, args := buildLLMUsageWhere(filters)
	limit := filters.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filters.Offset
	query := `SELECT
	id, provider, operation, stage, model, status, requester_email, key_owner_email, key_type, project_id, domain_id, generation_id, link_task_id, file_path,
	prompt_tokens, completion_tokens, total_tokens, token_source, input_price_usd_per_million, output_price_usd_per_million, estimated_cost_usd, error_message, created_at
FROM llm_usage_events` + where + `
ORDER BY created_at DESC
LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list llm usage events: %w", err)
	}
	defer rows.Close()

	items := make([]LLMUsageEvent, 0)
	for rows.Next() {
		var item LLMUsageEvent
		if err := rows.Scan(
			&item.ID,
			&item.Provider,
			&item.Operation,
			&item.Stage,
			&item.Model,
			&item.Status,
			&item.RequesterEmail,
			&item.KeyOwnerEmail,
			&item.KeyType,
			&item.ProjectID,
			&item.DomainID,
			&item.GenerationID,
			&item.LinkTaskID,
			&item.FilePath,
			&item.PromptTokens,
			&item.CompletionTokens,
			&item.TotalTokens,
			&item.TokenSource,
			&item.InputPriceUSDPerMillion,
			&item.OutputPriceUSDPerMillion,
			&item.EstimatedCostUSD,
			&item.ErrorMessage,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan llm usage event: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *LLMUsageStore) CountEvents(ctx context.Context, filters LLMUsageFilters) (int, error) {
	where, args := buildLLMUsageWhere(filters)
	query := `SELECT COUNT(1) FROM llm_usage_events` + where
	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to count llm usage events: %w", err)
	}
	return total, nil
}

func (s *LLMUsageStore) AggregateStats(ctx context.Context, filters LLMUsageFilters) (LLMUsageStats, error) {
	where, args := buildLLMUsageWhere(filters)
	stats := LLMUsageStats{}

	totalQuery := `SELECT COUNT(1), COALESCE(SUM(total_tokens),0), COALESCE(SUM(estimated_cost_usd),0) FROM llm_usage_events` + where
	if err := s.db.QueryRowContext(ctx, totalQuery, args...).Scan(
		&stats.Totals.TotalRequests,
		&stats.Totals.TotalTokens,
		&stats.Totals.TotalCostUSD,
	); err != nil {
		return LLMUsageStats{}, fmt.Errorf("failed to aggregate llm usage totals: %w", err)
	}

	var err error
	stats.ByDay, err = s.aggregateBy(ctx, where, args, "to_char(created_at, 'YYYY-MM-DD')")
	if err != nil {
		return LLMUsageStats{}, err
	}
	stats.ByModel, err = s.aggregateBy(ctx, where, args, "model")
	if err != nil {
		return LLMUsageStats{}, err
	}
	stats.ByOperation, err = s.aggregateBy(ctx, where, args, "operation")
	if err != nil {
		return LLMUsageStats{}, err
	}
	stats.ByUser, err = s.aggregateBy(ctx, where, args, "requester_email")
	if err != nil {
		return LLMUsageStats{}, err
	}
	return stats, nil
}

func (s *LLMUsageStore) aggregateBy(ctx context.Context, where string, args []interface{}, keyExpr string) ([]LLMUsageStatsBucket, error) {
	query := `SELECT ` + keyExpr + ` AS grp, COUNT(1), COALESCE(SUM(total_tokens),0), COALESCE(SUM(estimated_cost_usd),0)
FROM llm_usage_events` + where + `
GROUP BY grp
ORDER BY grp`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate llm usage by %s: %w", keyExpr, err)
	}
	defer rows.Close()

	out := make([]LLMUsageStatsBucket, 0)
	for rows.Next() {
		var item LLMUsageStatsBucket
		if err := rows.Scan(&item.Key, &item.Requests, &item.Tokens, &item.CostUSD); err != nil {
			return nil, fmt.Errorf("failed to scan llm usage aggregate: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func buildLLMUsageWhere(filters LLMUsageFilters) (string, []interface{}) {
	conds := make([]string, 0, 8)
	args := make([]interface{}, 0, 8)
	add := func(cond string, value interface{}) {
		args = append(args, value)
		conds = append(conds, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if filters.From != nil {
		add("created_at >= ?", filters.From.UTC())
	}
	if filters.To != nil {
		add("created_at <= ?", filters.To.UTC())
	}
	if filters.UserEmail != nil && strings.TrimSpace(*filters.UserEmail) != "" {
		add("requester_email = ?", strings.TrimSpace(*filters.UserEmail))
	}
	if filters.ProjectID != nil && strings.TrimSpace(*filters.ProjectID) != "" {
		add("project_id = ?", strings.TrimSpace(*filters.ProjectID))
	}
	if filters.DomainID != nil && strings.TrimSpace(*filters.DomainID) != "" {
		add("domain_id = ?", strings.TrimSpace(*filters.DomainID))
	}
	if filters.Model != nil && strings.TrimSpace(*filters.Model) != "" {
		add("model = ?", strings.TrimSpace(*filters.Model))
	}
	if filters.Operation != nil && strings.TrimSpace(*filters.Operation) != "" {
		add("operation = ?", strings.TrimSpace(*filters.Operation))
	}
	if filters.Status != nil && strings.TrimSpace(*filters.Status) != "" {
		add("status = ?", strings.TrimSpace(*filters.Status))
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// PurgeOlderThan удаляет записи об использовании LLM старше retentionDays дней.
func (s *LLMUsageStore) PurgeOlderThan(ctx context.Context, retentionDays int) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM llm_usage_events WHERE created_at < NOW() - make_interval(days => $1)`,
		retentionDays)
	if err != nil {
		return 0, fmt.Errorf("purge llm_usage_events: %w", err)
	}
	return res.RowsAffected()
}

func nullableLLMInt64(v sql.NullInt64) interface{} {
	if v.Valid {
		return v.Int64
	}
	return nil
}

func nullableFloat64(v sql.NullFloat64) interface{} {
	if v.Valid {
		return v.Float64
	}
	return nil
}
