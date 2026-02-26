package inventory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"crypto/rand"

	"golang.org/x/net/idna"

	"obzornik-pbn-generator/internal/importer/legacy"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

type domainRepo interface {
	GetByURL(ctx context.Context, url string) (sqlstore.Domain, error)
	UpdateInventoryState(
		ctx context.Context,
		id string,
		publishedPath sql.NullString,
		siteOwner sql.NullString,
		inventoryStatus sql.NullString,
		inventoryError sql.NullString,
		checkedAt time.Time,
	) error
}

type prober interface {
	Probe(ctx context.Context, target Target, domainASCII string) (ProbeResult, error)
}

type Runner struct {
	Domains domainRepo
	Prober  prober
	Sleep   func(time.Duration)
}

func (r Runner) Run(ctx context.Context, opts RunOptions) (Report, error) {
	if err := validateRunOptions(opts); err != nil {
		return Report{}, err
	}
	rows, err := legacy.ParseManifestCSV(opts.ManifestPath)
	if err != nil {
		return Report{}, err
	}
	batched, err := selectBatch(rows, opts.BatchSize, opts.BatchNumber)
	if err != nil {
		return Report{}, err
	}
	filtered := make([]legacy.ManifestRow, 0, len(batched))
	for _, row := range batched {
		if strings.TrimSpace(opts.TargetServer) != "" && !strings.EqualFold(strings.TrimSpace(row.ServerID), strings.TrimSpace(opts.TargetServer)) {
			continue
		}
		filtered = append(filtered, row)
	}

	report := Report{
		Mode:         opts.Mode,
		ManifestPath: opts.ManifestPath,
		TargetServer: opts.TargetServer,
		StartedAt:    time.Now().UTC(),
		Rows:         make([]RowReport, 0, len(filtered)),
	}
	if len(filtered) == 0 {
		report.FinishedAt = time.Now().UTC()
		return report, nil
	}

	workerCount := opts.Concurrency
	if workerCount <= 0 {
		workerCount = 10
	}
	if workerCount > len(filtered) {
		workerCount = len(filtered)
	}

	if r.Sleep == nil {
		r.Sleep = time.Sleep
	}

	jobs := make(chan legacy.ManifestRow)
	out := make(chan RowReport)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for row := range jobs {
				out <- r.processRow(ctx, row, opts)
			}
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	go func() {
		defer close(jobs)
		for _, row := range filtered {
			select {
			case <-ctx.Done():
				return
			case jobs <- row:
			}
		}
	}()

	for rr := range out {
		report.Rows = append(report.Rows, rr)
		applySummary(&report.Summary, rr)
	}
	report.FinishedAt = time.Now().UTC()
	return report, nil
}

func (r Runner) processRow(ctx context.Context, row legacy.ManifestRow, opts RunOptions) RowReport {
	rr := RowReport{
		RowNumber: row.RowNumber,
		DomainURL: row.DomainURL,
		ServerID:  row.ServerID,
	}
	host, err := normalizeDomainHost(row.DomainURL)
	if err != nil {
		rr.ProbeStatus = ProbeError
		rr.Error = err.Error()
		return rr
	}
	rr.NormalizedHost = host

	res, attempts, probeErr := r.probeWithRetry(ctx, host, opts)
	rr.Attempts = attempts
	rr.ProbeStatus = res.Status
	rr.PublishedPath = res.PublishedPath
	rr.SiteOwner = res.SiteOwner
	if len(res.Candidates) > 0 {
		rr.Warnings = append(rr.Warnings, "ambiguous paths: "+strings.Join(res.Candidates, ", "))
	}
	if probeErr != nil {
		rr.Error = probeErr.Error()
		return rr
	}
	rr.InventoryStatus = inventoryStatusFromProbe(res.Status)

	if opts.Mode == ModeDryRun {
		rr.ApplyAction = "would_update"
		return rr
	}

	domain, err := r.findDomain(ctx, row.DomainURL, host)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			rr.ApplyAction = "skipped_domain_missing_in_db"
			rr.Warnings = append(rr.Warnings, "domain not found in DB")
			return rr
		}
		rr.Error = fmt.Sprintf("load domain from DB: %v", err)
		return rr
	}
	inventoryError := strings.TrimSpace(res.Message)
	if len(res.Candidates) > 0 && inventoryError == "" {
		inventoryError = "ambiguous paths: " + strings.Join(res.Candidates, ", ")
	}
	if err := r.Domains.UpdateInventoryState(
		ctx,
		domain.ID,
		nullString(res.PublishedPath),
		nullString(res.SiteOwner),
		nullString(rr.InventoryStatus),
		nullString(inventoryError),
		time.Now().UTC(),
	); err != nil {
		rr.Error = fmt.Sprintf("update inventory state: %v", err)
		return rr
	}
	rr.ApplyAction = "updated"
	return rr
}

func (r Runner) findDomain(ctx context.Context, rawDomainURL string, normalizedHost string) (sqlstore.Domain, error) {
	domain, err := r.Domains.GetByURL(ctx, rawDomainURL)
	if err == nil {
		return domain, nil
	}
	if normalizedHost == rawDomainURL {
		return sqlstore.Domain{}, err
	}
	return r.Domains.GetByURL(ctx, normalizedHost)
}

func (r Runner) probeWithRetry(ctx context.Context, host string, opts RunOptions) (ProbeResult, int, error) {
	maxAttempts := opts.Retries + 1
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			backoff := opts.RetryDelay
			if backoff <= 0 {
				backoff = 2 * time.Second
			}
			r.Sleep(backoff)
		}
		jitter := randomDuration(opts.JitterMin, opts.JitterMax)
		if jitter > 0 {
			r.Sleep(jitter)
		}
		res, err := r.Prober.Probe(ctx, opts.Target, host)
		if err == nil {
			if res.Message == "" && res.Status == ProbeError {
				res.Message = "probe returned error status"
			}
			return res, attempt, nil
		}
		lastErr = err
		if strings.Contains(strings.ToLower(err.Error()), "permission denied") {
			return ProbeResult{
				Status:  ProbePermissionDenied,
				Message: err.Error(),
			}, attempt, nil
		}
	}
	return ProbeResult{
		Status:  ProbeError,
		Message: errorString(lastErr),
	}, maxAttempts, fmt.Errorf("probe failed after retries: %w", lastErr)
}

func validateRunOptions(opts RunOptions) error {
	if strings.TrimSpace(opts.ManifestPath) == "" {
		return fmt.Errorf("--manifest is required")
	}
	if opts.Mode != ModeDryRun && opts.Mode != ModeApply {
		return fmt.Errorf("--mode must be dry-run or apply")
	}
	if strings.TrimSpace(opts.TargetServer) == "" {
		return fmt.Errorf("--target is required")
	}
	if strings.TrimSpace(opts.Target.Host) == "" || strings.TrimSpace(opts.Target.User) == "" {
		return fmt.Errorf("target config must include ssh host and user")
	}
	if opts.BatchSize <= 0 {
		return fmt.Errorf("--batch-size must be > 0")
	}
	if opts.BatchNumber <= 0 {
		return fmt.Errorf("--batch-number must be >= 1")
	}
	if opts.Concurrency <= 0 {
		return fmt.Errorf("--concurrency must be > 0")
	}
	if opts.Retries < 0 {
		return fmt.Errorf("--retries must be >= 0")
	}
	return nil
}

func selectBatch(rows []legacy.ManifestRow, batchSize, batchNumber int) ([]legacy.ManifestRow, error) {
	start := (batchNumber - 1) * batchSize
	if start >= len(rows) {
		return []legacy.ManifestRow{}, nil
	}
	end := start + batchSize
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end], nil
}

func normalizeDomainHost(input string) (string, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", fmt.Errorf("domain is empty")
	}
	host := raw
	if strings.Contains(raw, "://") {
		parts := strings.SplitN(raw, "://", 2)
		host = parts[1]
	}
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "//")
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
	if idx := strings.Index(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	host = strings.TrimSpace(strings.Trim(host, "."))
	if host == "" {
		return "", fmt.Errorf("invalid domain: %q", input)
	}
	ascii, err := idna.Lookup.ToASCII(strings.ToLower(host))
	if err != nil {
		return "", fmt.Errorf("normalize domain %q: %w", host, err)
	}
	return ascii, nil
}

func applySummary(s *Summary, rr RowReport) {
	s.Processed++
	switch rr.ProbeStatus {
	case ProbeFound:
		s.Found++
	case ProbeNotFound:
		s.NotFound++
	case ProbeAmbiguous:
		s.Ambiguous++
	case ProbePermissionDenied:
		s.PermissionDenied++
	}
	if rr.Error != "" {
		s.Failed++
	} else if len(rr.Warnings) > 0 || rr.ProbeStatus != ProbeFound {
		s.Warned++
	} else {
		s.Success++
	}
	if rr.ApplyAction == "skipped_domain_missing_in_db" {
		s.DomainMissingInDB++
		s.AppliedSkipped++
	}
	if rr.ApplyAction == "updated" {
		s.AppliedUpdated++
	}
}

func inventoryStatusFromProbe(status ProbeStatus) string {
	switch status {
	case ProbeFound:
		return "ok"
	case ProbeAmbiguous:
		return "partial"
	default:
		return "failed"
	}
}

func nullString(v string) sql.NullString {
	v = strings.TrimSpace(v)
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func randomDuration(min, max time.Duration) time.Duration {
	if min <= 0 && max <= 0 {
		return 0
	}
	if max <= min {
		if min > 0 {
			return min
		}
		return max
	}
	diff := max - min
	n, err := rand.Int(rand.Reader, big.NewInt(int64(diff+1)))
	if err != nil {
		return min
	}
	return min + time.Duration(n.Int64())
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
