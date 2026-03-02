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

const defaultStaleCheckingTimeout = 20 * time.Minute

// ProjectStore описывает доступ к проектам для index checker worker.
type ProjectStore interface {
	ListAll(ctx context.Context) ([]sqlstore.Project, error)
	GetByID(ctx context.Context, id string) (sqlstore.Project, error)
}

// DomainStore описывает доступ к доменам для index checker worker.
type DomainStore interface {
	ListByProject(ctx context.Context, projectID string) ([]sqlstore.Domain, error)
	Get(ctx context.Context, id string) (sqlstore.Domain, error)
}

// IndexCheckStore описывает доступ к проверкам индексации для index checker worker.
type IndexCheckStore interface {
	Create(ctx context.Context, check sqlstore.IndexCheck) error
	Get(ctx context.Context, checkID string) (*sqlstore.IndexCheck, error)
	GetByDomainAndDate(ctx context.Context, domainID string, date time.Time) (*sqlstore.IndexCheck, error)
	ListPendingRetries(ctx context.Context) ([]sqlstore.IndexCheck, error)
	ListStaleChecking(ctx context.Context, olderThan time.Time) ([]sqlstore.IndexCheck, error)
	TryMarkChecking(ctx context.Context, checkID string) (bool, sqlstore.IndexCheck, error)
	UpdateStatus(ctx context.Context, checkID string, status string, isIndexed *bool, errMsg *string) error
	IncrementAttempts(ctx context.Context, checkID string) error
	SetNextRetry(ctx context.Context, checkID string, nextRetry time.Time) error
	UpdateContentResult(ctx context.Context, checkID string, quote string, isContentIndexed *bool) error
	ListSuccessWithoutQuote(ctx context.Context, limit int) ([]sqlstore.IndexCheck, error)
}

// CheckHistoryStore описывает доступ к истории попыток.
type CheckHistoryStore interface {
	Create(ctx context.Context, history sqlstore.CheckHistory) error
}

// ProcessCheckResult возвращает результат одиночной проверки.
type ProcessCheckResult struct {
	Started bool
	Status  string
	Skipped string
}

// RunIndexCheckerTick выполняет одну итерацию index checker.
func RunIndexCheckerTick(
	ctx context.Context,
	now time.Time,
	staleTimeout time.Duration,
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
	if staleTimeout <= 0 {
		staleTimeout = defaultStaleCheckingTimeout
	}

	if err := failStaleChecks(ctx, now, staleTimeout, checkStore, historyStore, logger); err != nil {
		logger.Errorf("index checker: stale checks pre-step failed: %v", err)
	}

	today := dateOnlyUTC(now)
	projects, err := projectStore.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}

	dueChecks := make(map[string]sqlstore.IndexCheck)
	gatherErrors := 0

	for _, p := range projects {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		domains, err := domainStore.ListByProject(ctx, p.ID)
		if err != nil {
			logger.Errorf("index checker: list domains failed for project %s: %v", p.ID, err)
			gatherErrors++
			continue
		}
		for _, d := range domains {
			if !isDomainPublished(d) || strings.TrimSpace(d.URL) == "" {
				continue
			}
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
						if isUniqueViolation(err) {
							existing, getErr := checkStore.GetByDomainAndDate(ctx, d.ID, today)
							if getErr != nil {
								logger.Errorf("index checker: load existing check failed for domain %s: %v", d.ID, getErr)
								gatherErrors++
								continue
							}
							check = existing
						} else {
							logger.Errorf("index checker: create check failed for domain %s: %v", d.ID, err)
							gatherErrors++
							continue
						}
					} else {
						check = &newCheck
					}
				} else {
					logger.Errorf("index checker: get check failed for domain %s: %v", d.ID, err)
					gatherErrors++
					continue
				}
			}
			if check != nil && isDuePending(*check, now) {
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

	var (
		processed int
		successes int
		failures  int
		skipped   int
		runErrors int
	)
	for _, check := range dueChecks {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		processed++
		result, err := ProcessCheckByID(ctx, now, check.ID, projectStore, domainStore, checkStore, historyStore, checker, logger)
		if err != nil {
			runErrors++
			logger.Errorf("index checker: process check %s failed: %v", check.ID, err)
			continue
		}
		if !result.Started {
			skipped++
			continue
		}
		switch result.Status {
		case "success":
			successes++
		case "failed_investigation":
			failures++
		default:
			// checking = retry scheduled
		}
	}

	logger.Infow(
		"index checker tick summary",
		"processed", processed,
		"success", successes,
		"failed", failures,
		"skipped", skipped,
		"errors", runErrors+gatherErrors,
	)

	// Backfill content checks for already-completed checks that have no quote yet.
	if err := backfillContentChecks(ctx, checkStore, domainStore, checker, logger); err != nil {
		logger.Warnf("index checker: content backfill step failed: %v", err)
	}

	return nil
}

const contentBackfillLimit = 3

// backfillContentChecks запускает проверку цитаты для успешных проверок без цитаты.
func backfillContentChecks(
	ctx context.Context,
	checkStore IndexCheckStore,
	domainStore DomainStore,
	checker IndexChecker,
	logger *zap.SugaredLogger,
) error {
	checks, err := checkStore.ListSuccessWithoutQuote(ctx, contentBackfillLimit)
	if err != nil {
		return fmt.Errorf("list success without quote: %w", err)
	}
	for _, check := range checks {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		domain, err := domainStore.Get(ctx, check.DomainID)
		if err != nil {
			logger.Warnf("content backfill: load domain %s failed: %v", check.DomainID, err)
			continue
		}
		siteURL := strings.TrimSpace(domain.URL)
		if siteURL == "" {
			continue
		}
		quote, extractErr := ExtractSiteQuote(ctx, siteURL)
		if extractErr != nil || quote == "" {
			continue
		}
		geo := strings.ToLower(strings.TrimSpace(domain.TargetCountry))
		if geo == "" {
			geo = "se"
		}
		var contentIndexedPtr *bool
		if contentIndexed, contentErr := checker.CheckWithQuote(ctx, siteURL, quote, geo); contentErr == nil {
			contentIndexedPtr = &contentIndexed
		}
		if updateErr := checkStore.UpdateContentResult(ctx, check.ID, quote, contentIndexedPtr); updateErr != nil {
			logger.Warnf("content backfill: save result for check %s failed: %v", check.ID, updateErr)
		}
	}
	return nil
}

// ProcessCheckByID запускает проверку индексации для конкретного check_id.
func ProcessCheckByID(
	ctx context.Context,
	now time.Time,
	checkID string,
	projectStore ProjectStore,
	domainStore DomainStore,
	checkStore IndexCheckStore,
	historyStore CheckHistoryStore,
	checker IndexChecker,
	logger *zap.SugaredLogger,
) (ProcessCheckResult, error) {
	if checker == nil {
		return ProcessCheckResult{}, errors.New("index checker is required")
	}
	check, err := checkStore.Get(ctx, checkID)
	if err != nil {
		return ProcessCheckResult{}, fmt.Errorf("load check: %w", err)
	}

	domain, err := domainStore.Get(ctx, check.DomainID)
	if err != nil {
		return ProcessCheckResult{}, fmt.Errorf("load domain: %w", err)
	}
	if !isDomainPublished(domain) {
		return failInvalidCheck(
			ctx,
			checkStore,
			historyStore,
			check.ID,
			"domain is not published",
			logger,
		)
	}
	if strings.TrimSpace(domain.URL) == "" {
		return failInvalidCheck(
			ctx,
			checkStore,
			historyStore,
			check.ID,
			"domain url is empty",
			logger,
		)
	}

	project, err := projectStore.GetByID(ctx, domain.ProjectID)
	if err != nil {
		return ProcessCheckResult{}, fmt.Errorf("load project: %w", err)
	}

	started, current, err := checkStore.TryMarkChecking(ctx, check.ID)
	if err != nil {
		return ProcessCheckResult{}, fmt.Errorf("mark checking: %w", err)
	}
	if !started {
		return ProcessCheckResult{Started: false, Skipped: "already_in_progress"}, nil
	}

	geo := resolveGeo(domain, project)
	status, err := processCheck(ctx, now, current, domain.URL, geo, checkStore, historyStore, checker, logger)
	if err != nil {
		return ProcessCheckResult{}, err
	}
	return ProcessCheckResult{Started: true, Status: status}, nil
}

func failInvalidCheck(
	ctx context.Context,
	checkStore IndexCheckStore,
	historyStore CheckHistoryStore,
	checkID string,
	reason string,
	logger *zap.SugaredLogger,
) (ProcessCheckResult, error) {
	started, current, err := checkStore.TryMarkChecking(ctx, checkID)
	if err != nil {
		return ProcessCheckResult{}, fmt.Errorf("mark checking: %w", err)
	}
	if !started {
		return ProcessCheckResult{Started: false, Skipped: "already_in_progress"}, nil
	}
	attemptNumber := current.Attempts + 1
	history := sqlstore.CheckHistory{
		ID:            uuid.NewString(),
		CheckID:       current.ID,
		AttemptNumber: attemptNumber,
		Result:        sqlstore.NullableString("failed_investigation"),
		ErrorMessage:  sqlstore.NullableString(reason),
		DurationMS:    sql.NullInt64{},
	}
	if err := historyStore.Create(ctx, history); err != nil {
		return ProcessCheckResult{}, fmt.Errorf("create check history: %w", err)
	}
	if err := checkStore.IncrementAttempts(ctx, current.ID); err != nil {
		return ProcessCheckResult{}, fmt.Errorf("increment attempts: %w", err)
	}
	if err := checkStore.UpdateStatus(ctx, current.ID, "failed_investigation", nil, &reason); err != nil {
		return ProcessCheckResult{}, fmt.Errorf("update index check status: %w", err)
	}
	if logger != nil {
		logger.Warnf("index check %s terminated: %s", current.ID, reason)
	}
	return ProcessCheckResult{Started: true, Status: "failed_investigation"}, nil
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
) (string, error) {
	startedAt := time.Now()
	indexed, err := checker.Check(ctx, domain, geo)
	duration := time.Since(startedAt)
	attemptNumber := check.Attempts + 1
	durationMS := sql.NullInt64{Int64: duration.Milliseconds(), Valid: true}

	if err == nil {
		// Проверка цитаты (best-effort): не влияет на статус основной проверки.
		quote := strings.TrimSpace(check.ContentQuote.String)
		if !check.ContentQuote.Valid || quote == "" {
			if extracted, extractErr := ExtractSiteQuote(ctx, domain); extractErr == nil && extracted != "" {
				quote = extracted
			}
		}
		var contentIndexedPtr *bool
		if quote != "" {
			if contentIndexed, contentErr := checker.CheckWithQuote(ctx, domain, quote, geo); contentErr == nil {
				contentIndexedPtr = &contentIndexed
			}
			if updateErr := checkStore.UpdateContentResult(ctx, check.ID, quote, contentIndexedPtr); updateErr != nil && logger != nil {
				logger.Warnf("index check %s: failed to save content result: %v", check.ID, updateErr)
			}
		}

		payload, encErr := json.Marshal(map[string]any{
			"indexed":           indexed,
			"content_quote":     quote,
			"content_indexed":   contentIndexedPtr,
		})
		if encErr != nil {
			return "", fmt.Errorf("encode response data: %w", encErr)
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
			return "", fmt.Errorf("create check history: %w", err)
		}
		if err := checkStore.IncrementAttempts(ctx, check.ID); err != nil {
			return "", fmt.Errorf("increment attempts: %w", err)
		}
		if err := checkStore.UpdateStatus(ctx, check.ID, "success", &indexed, nil); err != nil {
			return "", fmt.Errorf("update index check status: %w", err)
		}
		return "success", nil
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
		return "", fmt.Errorf("create check history: %w", err)
	}
	if err := checkStore.IncrementAttempts(ctx, check.ID); err != nil {
		return "", fmt.Errorf("increment attempts: %w", err)
	}
	check.Attempts++

	if ShouldRetry(check) {
		nextRetry := now.Add(CalculateNextRetry(check.Attempts))
		if err := checkStore.SetNextRetry(ctx, check.ID, nextRetry); err != nil {
			return "", fmt.Errorf("set next retry: %w", err)
		}
		if err := checkStore.UpdateStatus(ctx, check.ID, "checking", nil, &errMsg); err != nil {
			return "", fmt.Errorf("update index check status: %w", err)
		}
		return "checking", nil
	}

	if err := checkStore.UpdateStatus(ctx, check.ID, "failed_investigation", nil, &errMsg); err != nil {
		return "", fmt.Errorf("update index check status: %w", err)
	}
	if logger != nil {
		logger.Warnf("index check %s failed without retry: %s", check.ID, errMsg)
	}
	return "failed_investigation", nil
}

func failStaleChecks(
	ctx context.Context,
	now time.Time,
	staleTimeout time.Duration,
	checkStore IndexCheckStore,
	historyStore CheckHistoryStore,
	logger *zap.SugaredLogger,
) error {
	olderThan := now.Add(-staleTimeout)
	staleChecks, err := checkStore.ListStaleChecking(ctx, olderThan)
	if err != nil {
		return fmt.Errorf("list stale checking: %w", err)
	}

	for _, check := range staleChecks {
		msg := "stale checking timeout"
		attempt := check.Attempts
		if attempt < 1 {
			attempt = 1
		}
		history := sqlstore.CheckHistory{
			ID:            uuid.NewString(),
			CheckID:       check.ID,
			AttemptNumber: attempt,
			Result:        sqlstore.NullableString("failed_investigation"),
			ErrorMessage:  sqlstore.NullableString(msg),
			DurationMS:    sql.NullInt64{},
		}
		if err := historyStore.Create(ctx, history); err != nil {
			if logger != nil {
				logger.Errorf("index checker: stale history create failed for %s: %v", check.ID, err)
			}
			continue
		}
		if err := checkStore.UpdateStatus(ctx, check.ID, "failed_investigation", nil, &msg); err != nil {
			if logger != nil {
				logger.Errorf("index checker: stale status update failed for %s: %v", check.ID, err)
			}
			continue
		}
		if logger != nil {
			logger.Warnf("index checker: check %s marked failed_investigation due to stale checking", check.ID)
		}
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

func isDomainPublished(d sqlstore.Domain) bool {
	if d.PublishedAt.Valid {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(d.Status), "published")
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "duplicate key value") || strings.Contains(text, "unique constraint")
}
