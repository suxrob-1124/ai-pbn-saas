package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/db"
	"obzornik-pbn-generator/internal/importer/legacy"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

func main() {
	manifest := flag.String("manifest", "", "path to CSV manifest")
	mode := flag.String("mode", "", "import mode: dry-run|apply")
	batchSize := flag.Int("batch-size", 50, "batch size")
	batchNumber := flag.Int("batch-number", 1, "batch number (1-based)")
	serverDir := flag.String("server-dir", "server", "directory with published sites")
	reportPath := flag.String("report", "import_legacy_report.json", "output path for JSON report")
	force := flag.Bool("force", false, "allow rewrite synthetic legacy artifacts even if non-legacy generations exist")
	decodeSource := flag.String("decode-source", "import_legacy", "decode source marker: import_legacy|decode_backfill")
	flag.Parse()

	zl, _ := zap.NewProduction()
	defer zl.Sync()
	logger := zl.Sugar()

	cfg := config.Load()
	if err := validateImportConfig(cfg); err != nil {
		logger.Fatalf("invalid config: %v", err)
	}

	database := db.Open(cfg, logger)
	defer database.Close()

	userStore := sqlstore.NewUserStore(database)
	projectStore := sqlstore.NewProjectStore(database)
	domainStore := sqlstore.NewDomainStore(database)
	siteFileStore := sqlstore.NewSiteFileStore(database)
	linkTaskStore := sqlstore.NewLinkTaskStore(database)
	generationStore := sqlstore.NewGenerationStore(database)
	promptStore := sqlstore.NewPromptStore(database)

	imp := legacy.NewImporter(userStore, projectStore, domainStore, siteFileStore, linkTaskStore, generationStore, promptStore)

	opts := legacy.RunOptions{
		ManifestPath: *manifest,
		ServerDir:    *serverDir,
		Mode:         legacy.Mode(*mode),
		Batch: legacy.BatchConfig{
			BatchSize:   *batchSize,
			BatchNumber: *batchNumber,
		},
		Force:        *force,
		DecodeSource: *decodeSource,
	}

	report, err := imp.Run(context.Background(), opts)
	if err != nil {
		logger.Fatalf("import failed: %v", err)
	}

	if err := writeReport(*reportPath, report); err != nil {
		logger.Fatalf("write report failed: %v", err)
	}

	logger.Infow("import finished",
		"mode", report.Mode,
		"processed", report.Summary.Processed,
		"success", report.Summary.Success,
		"warned", report.Summary.Warned,
		"failed", report.Summary.Failed,
		"decoded", report.Summary.Decoded,
		"updated", report.Summary.Updated,
		"skipped", report.Summary.Skipped,
		"unchanged", report.Summary.Unchanged,
		"report", *reportPath,
	)

	fmt.Printf("import completed: processed=%d success=%d warned=%d failed=%d decoded=%d updated=%d skipped=%d unchanged=%d report=%s\n",
		report.Summary.Processed,
		report.Summary.Success,
		report.Summary.Warned,
		report.Summary.Failed,
		report.Summary.Decoded,
		report.Summary.Updated,
		report.Summary.Skipped,
		report.Summary.Unchanged,
		*reportPath,
	)
}

func writeReport(path string, report legacy.Report) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func validateImportConfig(cfg config.Config) error {
	if strings.TrimSpace(cfg.DBDriver) == "" {
		return fmt.Errorf("DB_DRIVER is required")
	}
	if strings.TrimSpace(cfg.DSN) == "" {
		return fmt.Errorf("DB_DSN is required")
	}
	return nil
}
