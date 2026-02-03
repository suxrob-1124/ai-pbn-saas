package main

import (
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/db"
)

func main() {
	zl, _ := zap.NewProduction()
	defer zl.Sync()
	logger := zl.Sugar()

	cfg := config.Load()

	if err := cfg.Validate(); err != nil {
		logger.Fatalf("invalid config: %v", err)
	}

	cfg.MigrateOnStart = true

	conn := db.Open(cfg, logger)

	if conn != nil {
		conn.Close()
	}
	logger.Info("migrations applied")
}
