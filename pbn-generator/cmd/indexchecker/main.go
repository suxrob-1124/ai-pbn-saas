package main

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/db"
	"obzornik-pbn-generator/internal/indexchecker"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

const indexCheckerSchedule = "@every 1h"

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()

	dbConn := db.Open(cfg, sugar)
	defer dbConn.Close()

	projectStore := sqlstore.NewProjectStore(dbConn)
	domainStore := sqlstore.NewDomainStore(dbConn)
	checkStore := sqlstore.NewIndexCheckStore(dbConn)
	historyStore := sqlstore.NewCheckHistoryStore(dbConn)
	checker := &indexchecker.SerpChecker{}

	run := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		now := time.Now().UTC()
		if err := indexchecker.RunIndexCheckerTick(ctx, now, projectStore, domainStore, checkStore, historyStore, checker, sugar); err != nil {
			sugar.Errorf("index checker tick failed: %v", err)
		} else {
			sugar.Infof("index checker tick completed")
		}
	}

	run()

	c := cron.New(cron.WithLocation(time.UTC))
	if _, err := c.AddFunc(indexCheckerSchedule, run); err != nil {
		sugar.Fatalf("failed to register index checker cron: %v", err)
	}
	c.Start()

	sugar.Infof("index checker started with schedule %s", indexCheckerSchedule)
	select {}
}
