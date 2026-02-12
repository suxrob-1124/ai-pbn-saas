package indexchecker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

// ProjectStore описывает доступ к проектам для index checker worker.
type ProjectStore interface {
	ListAll(ctx context.Context) ([]sqlstore.Project, error)
}

// DomainStore описывает доступ к доменам для index checker worker.
type DomainStore interface {
	ListByProject(ctx context.Context, projectID string) ([]sqlstore.Domain, error)
}

// IndexCheckStore описывает доступ к проверкам индексации для index checker worker.
type IndexCheckStore interface {
	Create(ctx context.Context, check sqlstore.IndexCheck) error
	GetByDomainAndDate(ctx context.Context, domainID string, date time.Time) (*sqlstore.IndexCheck, error)
	ListPendingRetries(ctx context.Context) ([]sqlstore.IndexCheck, error)
	UpdateStatus(ctx context.Context, checkID string, status string, isIndexed *bool, errMsg *string) error
	IncrementAttempts(ctx context.Context, checkID string) error
	SetNextRetry(ctx context.Context, checkID string, nextRetry time.Time) error
}

// CheckHistoryStore описывает доступ к истории попыток.
type CheckHistoryStore interface {
	Create(ctx context.Context, history sqlstore.CheckHistory) error
}

// RunIndexCheckerTick выполняет одну итерацию index checker.
func RunIndexCheckerTick(
	ctx context.Context,
	now time.Time,
	projectStore ProjectStore,
	domainStore DomainStore,
	checkStore IndexCheckStore,
	historyStore CheckHistoryStore,
	checker IndexChecker,
	logger *zap.SugaredLogger,
) error {
	if checker == nil {
		return errors.New("index checker is required")
	}
	if projectStore == nil || domainStore == nil || checkStore == nil || historyStore == nil {
		return errors.New("stores are required")
	}
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	today := dateOnlyUTC(now)

	projects, err := projectStore.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}
	projectByID := make(map[string]sqlstore.Project, len(projects))
	for _, p := range projects {
		projectByID[p.ID] = p
	}

	domainByID := make(map[string]sqlstore.Domain)
	dueChecks := make(map[string]sqlstore.IndexCheck)

	for _, p := range projects {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		domains, err := domainStore.ListByProject(ctx, p.ID)
		if err != nil {
			return fmt.Errorf("list domains for project %s: %w", p.ID, err)
		}
		for _, d := range domains {
			domainByID[d.ID] = d
			check, err := checkStore.GetByDomainAndDate(ctx, d.ID, today)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					newCheck := sqlstore.IndexCheck{
						ID:        uuid.NewString(),
						DomainID:  d.ID,
						CheckDate: today,
						Status:    "pending",
						Attempts:  0,
						CreatedAt: now.UTC(),
					}
					if err := checkStore.Create(ctx, newCheck); err != nil {
						return fmt.Errorf("create index check: %w", err)
					}
					check = &newCheck
				} else {
					return fmt.Errorf("get index check: %w", err)
				}
			}
			if isDuePending(*check, now) {
				dueChecks[check.ID] = *check
			}
		}
	}

	retryChecks, err := checkStore.ListPendingRetries(ctx)
	if err != nil {
		return fmt.Errorf("list pending retries: %w", err)
	}
	for _, check := range retryChecks {
		dueChecks[check.ID] = check
	}

	for _, check := range dueChecks {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		domain, ok := domainByID[check.DomainID]
		if !ok {
			logger.Warnf("domain %s not found for check %s", check.DomainID, check.ID)
			continue
		}
		if strings.TrimSpace(domain.URL) == "" {
			logger.Warnf("domain url is empty for check %s", check.ID)
			continue
		}
		project := projectByID[domain.ProjectID]
		geo := resolveGeo(domain, project)
		if err := processCheck(ctx, now, check, domain.URL, geo, checkStore, historyStore, checker, logger); err != nil {
			return err
		}
	}

	return nil
}

func processCheck(
	ctx context.Context,
	now time.Time,
	check sqlstore.IndexCheck,
	domain string,
	geo string,
	checkStore IndexCheckStore,
	historyStore CheckHistoryStore,
	checker IndexChecker,
	logger *zap.SugaredLogger,
) error {
	startedAt := time.Now()
	indexed, err := checker.Check(ctx, domain, geo)
	duration := time.Since(startedAt)
	attemptNumber := check.Attempts + 1
	durationMS := sql.NullInt64{Int64: duration.Milliseconds(), Valid: true}

	if err == nil {
		payload, encErr := json.Marshal(map[string]any{"indexed": indexed})
		if encErr != nil {
			return fmt.Errorf("encode response data: %w", encErr)
		}
		history := sqlstore.CheckHistory{
			ID:            uuid.NewString(),
			CheckID:       check.ID,
			AttemptNumber: attemptNumber,
			Result:        sqlstore.NullableString("success"),
			ResponseData:  payload,
			ErrorMessage:  sql.NullString{},
			DurationMS:    durationMS,
		}
		if err := historyStore.Create(ctx, history); err != nil {
			return fmt.Errorf("create check history: %w", err)
		}
		if err := checkStore.UpdateStatus(ctx, check.ID, "success", &indexed, nil); err != nil {
			return fmt.Errorf("update index check status: %w", err)
		}
		return nil
	}

	errMsg := err.Error()
	result := classifyCheckError(err)
	history := sqlstore.CheckHistory{
		ID:            uuid.NewString(),
		CheckID:       check.ID,
		AttemptNumber: attemptNumber,
		Result:        sqlstore.NullableString(result),
		ResponseData:  nil,
		ErrorMessage:  sqlstore.NullableString(errMsg),
		DurationMS:    durationMS,
	}
	if err := historyStore.Create(ctx, history); err != nil {
		return fmt.Errorf("create check history: %w", err)
	}
	if err := checkStore.IncrementAttempts(ctx, check.ID); err != nil {
		return fmt.Errorf("increment attempts: %w", err)
	}
	check.Attempts++

	if ShouldRetry(check) {
		nextRetry := now.Add(CalculateNextRetry(check.Attempts))
		if err := checkStore.SetNextRetry(ctx, check.ID, nextRetry); err != nil {
			return fmt.Errorf("set next retry: %w", err)
		}
		if err := checkStore.UpdateStatus(ctx, check.ID, "checking", nil, &errMsg); err != nil {
			return fmt.Errorf("update index check status: %w", err)
		}
		return nil
	}

	if err := checkStore.UpdateStatus(ctx, check.ID, "failed_investigation", nil, &errMsg); err != nil {
		return fmt.Errorf("update index check status: %w", err)
	}
	if logger != nil {
		logger.Warnf("index check %s failed without retry: %s", check.ID, errMsg)
	}
	return nil
}

func resolveGeo(domain sqlstore.Domain, project sqlstore.Project) string {
	if v := strings.TrimSpace(domain.TargetCountry); v != "" {
		return strings.ToLower(v)
	}
	if v := strings.TrimSpace(project.TargetCountry); v != "" {
		return strings.ToLower(v)
	}
	return "se"
}

func dateOnlyUTC(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func isDuePending(check sqlstore.IndexCheck, now time.Time) bool {
	if strings.TrimSpace(check.Status) != "pending" {
		return false
	}
	if check.NextRetryAt.Valid {
		return !check.NextRetryAt.Time.After(now)
	}
	return true
}

func classifyCheckError(err error) string {
	if err == nil {
		return "success"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	if strings.Contains(strings.ToLower(err.Error()), "timeout") {
		return "timeout"
	}
	return "error"
}
