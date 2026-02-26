package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/db"
	"obzornik-pbn-generator/internal/inventory"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

func main() {
	manifest := flag.String("manifest", "", "path to CSV manifest")
	mode := flag.String("mode", "dry-run", "inventory mode: dry-run|apply")
	target := flag.String("target", "", "target alias/server_id from manifest")
	batchSize := flag.Int("batch-size", 50, "batch size")
	batchNumber := flag.Int("batch-number", 1, "batch number (1-based)")
	concurrency := flag.Int("concurrency", 10, "number of concurrent workers")
	retries := flag.Int("retries", 3, "ssh probe retries")
	retryDelay := flag.Duration("retry-delay", 2*time.Second, "retry delay")
	jitterMin := flag.Duration("jitter-min", 100*time.Millisecond, "minimum jitter before ssh dial")
	jitterMax := flag.Duration("jitter-max", 500*time.Millisecond, "maximum jitter before ssh dial")
	reportPath := flag.String("report", "inventory_legacy_sites_report.json", "output path for JSON report")
	flag.Parse()

	zl, _ := zap.NewProduction()
	defer zl.Sync()
	logger := zl.Sugar()

	cfg := config.Load()
	if err := validateConfig(cfg); err != nil {
		logger.Fatalf("invalid config: %v", err)
	}

	targetCfg, err := resolveTarget(cfg, strings.TrimSpace(*target))
	if err != nil {
		logger.Fatalf("resolve target: %v", err)
	}

	database := db.Open(cfg, logger)
	defer database.Close()

	domainStore := sqlstore.NewDomainStore(database)
	runner := inventory.Runner{
		Domains: domainStore,
		Prober: inventory.SSHProber{
			Timeout:        cfg.DeployTimeout,
			KnownHostsPath: cfg.DeployKnownHostsPath,
		},
	}

	report, err := runner.Run(context.Background(), inventory.RunOptions{
		ManifestPath: *manifest,
		TargetServer: *target,
		Mode:         inventory.Mode(*mode),
		BatchSize:    *batchSize,
		BatchNumber:  *batchNumber,
		Concurrency:  *concurrency,
		Retries:      *retries,
		RetryDelay:   *retryDelay,
		JitterMin:    *jitterMin,
		JitterMax:    *jitterMax,
		Target:       targetCfg,
	})
	if err != nil {
		logger.Fatalf("inventory failed: %v", err)
	}
	if err := writeReport(*reportPath, report); err != nil {
		logger.Fatalf("write report failed: %v", err)
	}

	logger.Infow("inventory finished",
		"mode", report.Mode,
		"target", *target,
		"processed", report.Summary.Processed,
		"success", report.Summary.Success,
		"warned", report.Summary.Warned,
		"failed", report.Summary.Failed,
		"found", report.Summary.Found,
		"not_found", report.Summary.NotFound,
		"ambiguous", report.Summary.Ambiguous,
		"permission_denied", report.Summary.PermissionDenied,
		"domain_missing_in_db", report.Summary.DomainMissingInDB,
		"applied_updated", report.Summary.AppliedUpdated,
		"applied_skipped", report.Summary.AppliedSkipped,
		"report", *reportPath,
	)

	fmt.Printf(
		"inventory completed: mode=%s target=%s processed=%d success=%d warned=%d failed=%d found=%d not_found=%d ambiguous=%d permission_denied=%d updated=%d skipped=%d report=%s\n",
		report.Mode, *target,
		report.Summary.Processed, report.Summary.Success, report.Summary.Warned, report.Summary.Failed,
		report.Summary.Found, report.Summary.NotFound, report.Summary.Ambiguous, report.Summary.PermissionDenied,
		report.Summary.AppliedUpdated, report.Summary.AppliedSkipped,
		*reportPath,
	)
}

func writeReport(path string, report inventory.Report) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func validateConfig(cfg config.Config) error {
	if strings.TrimSpace(cfg.DBDriver) == "" {
		return fmt.Errorf("DB_DRIVER is required")
	}
	if strings.TrimSpace(cfg.DSN) == "" {
		return fmt.Errorf("DB_DSN is required")
	}
	return nil
}

func resolveTarget(cfg config.Config, alias string) (inventory.Target, error) {
	if alias == "" {
		return inventory.Target{}, fmt.Errorf("--target is required")
	}
	targets, err := parseTargetsJSON(cfg.DeployTargetsJSON)
	if err != nil {
		return inventory.Target{}, err
	}
	if t, ok := targets[alias]; ok {
		return t, nil
	}
	// Legacy single target fallback for transition period.
	host := strings.TrimSpace(os.Getenv("DEPLOY_SSH_HOST"))
	user := strings.TrimSpace(os.Getenv("DEPLOY_SSH_USER"))
	key := strings.TrimSpace(os.Getenv("DEPLOY_SSH_KEY_PATH"))
	if host != "" && user != "" {
		return inventory.Target{Alias: alias, Host: host, User: user, KeyPath: key}, nil
	}
	return inventory.Target{}, fmt.Errorf("target %q not found in DEPLOY_TARGETS_JSON and legacy DEPLOY_SSH_* is empty", alias)
}

func parseTargetsJSON(raw string) (map[string]inventory.Target, error) {
	out := map[string]inventory.Target{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out, nil
	}
	var payload map[string]map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("parse DEPLOY_TARGETS_JSON: %w", err)
	}
	for alias, item := range payload {
		host := pickString(item, "ssh_host", "host")
		user := pickString(item, "ssh_user", "user")
		key := pickString(item, "ssh_key_path", "key_path")
		if host == "" || user == "" {
			continue
		}
		out[alias] = inventory.Target{
			Alias:   alias,
			Host:    host,
			User:    user,
			KeyPath: key,
		}
	}
	return out, nil
}

func pickString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch tv := v.(type) {
			case string:
				if strings.TrimSpace(tv) != "" {
					return strings.TrimSpace(tv)
				}
			case float64:
				return strconv.FormatFloat(tv, 'f', -1, 64)
			}
		}
	}
	return ""
}
