package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/db"
	"obzornik-pbn-generator/internal/store/sqlstore"
	"obzornik-pbn-generator/internal/tasks"
)

const (
	schedulerBatchSize = 100
)

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

	scheduleStore := sqlstore.NewScheduleStore(dbConn)
	linkScheduleStore := sqlstore.NewLinkScheduleStore(dbConn)
	genQueueStore := sqlstore.NewGenQueueStore(dbConn)
	linkTaskStore := sqlstore.NewLinkTaskStore(dbConn)
	domainStore := sqlstore.NewDomainStore(dbConn)
	genStore := sqlstore.NewGenerationStore(dbConn)
	projectStore := sqlstore.NewProjectStore(dbConn)

	taskClient, err := tasks.NewClient(cfg)
	if err != nil {
		sugar.Fatalf("failed to init task client: %v", err)
	}
	defer taskClient.Close()

	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	scheduler := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{})
	if _, err := scheduler.Register("@every 1m", tasks.NewSchedulerTickTask()); err != nil {
		sugar.Fatalf("failed to register scheduler tick: %v", err)
	}
	go func() {
		if err := scheduler.Run(); err != nil {
			sugar.Fatalf("scheduler stopped: %v", err)
		}
	}()

	server := tasks.NewServer(cfg, 1)
	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TaskSchedulerTick, func(ctx context.Context, t *asynq.Task) error {
		return runSchedulerTick(ctx, scheduleStore, linkScheduleStore, genQueueStore, linkTaskStore, domainStore, genStore, projectStore, taskClient, sugar)
	})

	sugar.Infof("starting scheduler on redis %s", cfg.RedisAddr)
	if err := server.Run(mux); err != nil {
		sugar.Fatalf("scheduler worker stopped: %v", err)
	}
}

type ScheduleStore interface {
	ListActive(ctx context.Context) ([]sqlstore.Schedule, error)
	Get(ctx context.Context, scheduleID string) (*sqlstore.Schedule, error)
	Update(ctx context.Context, scheduleID string, updates sqlstore.ScheduleUpdates) error
}

type LinkScheduleStore interface {
	ListActive(ctx context.Context) ([]sqlstore.LinkSchedule, error)
	Update(ctx context.Context, scheduleID string, updates sqlstore.LinkScheduleUpdates) error
}

type GenQueueStore interface {
	ListByProject(ctx context.Context, projectID string) ([]sqlstore.QueueItem, error)
	Enqueue(ctx context.Context, item sqlstore.QueueItem) error
	GetPending(ctx context.Context, limit int) ([]sqlstore.QueueItem, error)
	MarkProcessed(ctx context.Context, itemID, status string, errorMsg *string) error
}

type DomainStore interface {
	ListByProject(ctx context.Context, projectID string) ([]sqlstore.Domain, error)
	Get(ctx context.Context, id string) (sqlstore.Domain, error)
	UpdateStatus(ctx context.Context, id, status string) error
	SetLastGeneration(ctx context.Context, id, genID string) error
}

type GenerationStore interface {
	ListByDomain(ctx context.Context, domainID string) ([]sqlstore.Generation, error)
	Create(ctx context.Context, g sqlstore.Generation) error
	UpdateStatus(ctx context.Context, id, status string, progress int, errText *string) error
}

type ProjectStore interface {
	GetByID(ctx context.Context, id string) (sqlstore.Project, error)
}

func runSchedulerTick(
	ctx context.Context,
	scheduleStore ScheduleStore,
	linkScheduleStore LinkScheduleStore,
	genQueueStore GenQueueStore,
	linkTaskStore LinkTaskStore,
	domainStore DomainStore,
	genStore GenerationStore,
	projectStore ProjectStore,
	taskClient tasks.Enqueuer,
	logger *zap.SugaredLogger,
) error {
	if err := applySchedules(ctx, scheduleStore, genQueueStore, domainStore, logger); err != nil {
		return err
	}
	if err := applyLinkSchedules(ctx, linkScheduleStore, linkTaskStore, domainStore, logger); err != nil {
		return err
	}
	if err := processPendingQueue(ctx, scheduleStore, genQueueStore, domainStore, genStore, projectStore, taskClient, logger); err != nil {
		return err
	}
	if err := processPendingLinkTasks(ctx, linkTaskStore, taskClient, logger); err != nil {
		return err
	}
	return nil
}

func applySchedules(ctx context.Context, scheduleStore ScheduleStore, genQueueStore GenQueueStore, domainStore DomainStore, logger *zap.SugaredLogger) error {
	now := time.Now().UTC().Truncate(time.Minute)
	return applySchedulesAt(ctx, scheduleStore, genQueueStore, domainStore, logger, now)
}

func applySchedulesAt(ctx context.Context, scheduleStore ScheduleStore, genQueueStore GenQueueStore, domainStore DomainStore, logger *zap.SugaredLogger, now time.Time) error {
	schedules, err := scheduleStore.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list schedules: %w", err)
	}
	if len(schedules) == 0 {
		return nil
	}

	for _, sched := range schedules {
		cfg, err := parseScheduleConfig(sched.Config)
		if err != nil {
			logger.Warnf("invalid schedule config %s: %v", sched.ID, err)
			continue
		}

		queueItems, err := genQueueStore.ListByProject(ctx, sched.ProjectID)
		if err != nil {
			logger.Warnf("could not list queue for schedule %s: %v", sched.ID, err)
			continue
		}
		loc := time.UTC
		if sched.Timezone.Valid {
			if tz, err := time.LoadLocation(strings.TrimSpace(sched.Timezone.String)); err == nil {
				loc = tz
			} else if logger != nil {
				logger.Warnf("invalid schedule timezone %s: %v", sched.ID, err)
			}
		}
		localNow := now.In(loc)
		lastRunLocal := time.Time{}
		if sched.LastRunAt.Valid {
			lastRunLocal = sched.LastRunAt.Time.In(loc)
		}
		nextRunLocal, shouldRun, err := computeScheduleNextRun(sched.Strategy, cfg, localNow, lastRunLocal)
		if err != nil {
			if logger != nil {
				logger.Warnf("schedule %s skipped: %v", sched.ID, err)
			}
			continue
		}
		nextRunUTC := sql.NullTime{}
		if !nextRunLocal.IsZero() {
			nextRunUTC = sql.NullTime{Time: nextRunLocal.In(time.UTC), Valid: true}
		}
		if !sched.NextRunAt.Valid || (nextRunUTC.Valid && !sched.NextRunAt.Time.Equal(nextRunUTC.Time)) {
			if err := scheduleStore.Update(ctx, sched.ID, sqlstore.ScheduleUpdates{NextRunAt: &nextRunUTC}); err != nil && logger != nil {
				logger.Warnf("failed to update next_run_at for %s: %v", sched.ID, err)
			}
		}
		if !shouldRun {
			continue
		}

		domains, err := domainStore.ListByProject(ctx, sched.ProjectID)
		if err != nil {
			logger.Warnf("could not list domains for schedule %s: %v", sched.ID, err)
			continue
		}

		enqueued, err := enqueueScheduleDomains(ctx, genQueueStore, sched, cfg, domains, queueItems, now)
		if err != nil {
			logger.Warnf("failed to enqueue schedule %s: %v", sched.ID, err)
			continue
		}
		lastRunUTC := sql.NullTime{Time: now, Valid: true}
		updates := sqlstore.ScheduleUpdates{LastRunAt: &lastRunUTC, NextRunAt: &nextRunUTC}
		if err := scheduleStore.Update(ctx, sched.ID, updates); err != nil && logger != nil {
			logger.Warnf("failed to update schedule run times %s: %v", sched.ID, err)
		}
		if enqueued > 0 {
			logger.Infof("schedule %s enqueued %d domains", sched.ID, enqueued)
		}
	}

	return nil
}

func enqueueScheduleDomains(
	ctx context.Context,
	genQueueStore GenQueueStore,
	sched sqlstore.Schedule,
	cfg scheduleConfig,
	domains []sqlstore.Domain,
	queueItems []sqlstore.QueueItem,
	now time.Time,
) (int, error) {
	queuedDomains := map[string]bool{}
	queuedForSchedule := 0
	for _, item := range queueItems {
		if item.Status != "pending" && item.Status != "queued" {
			continue
		}
		queuedDomains[item.DomainID] = true
		if item.ScheduleID.Valid && item.ScheduleID.String == sched.ID {
			queuedForSchedule++
		}
	}

	limit := cfg.Limit
	if limit <= 0 {
		limit = len(domains)
	}
	remaining := limit - queuedForSchedule
	if remaining <= 0 {
		return 0, nil
	}

	count := 0
	for _, d := range domains {
		if count >= remaining {
			break
		}
		if queuedDomains[d.ID] {
			continue
		}
		item := sqlstore.QueueItem{
			ID:           uuid.NewString(),
			DomainID:     d.ID,
			ScheduleID:   sql.NullString{String: sched.ID, Valid: true},
			Priority:     0,
			ScheduledFor: now,
			Status:       "pending",
		}
		if err := genQueueStore.Enqueue(ctx, item); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func processPendingQueue(
	ctx context.Context,
	scheduleStore ScheduleStore,
	genQueueStore GenQueueStore,
	domainStore DomainStore,
	genStore GenerationStore,
	projectStore ProjectStore,
	taskClient tasks.Enqueuer,
	logger *zap.SugaredLogger,
) error {
	for {
		items, err := genQueueStore.GetPending(ctx, schedulerBatchSize)
		if err != nil {
			return fmt.Errorf("get pending queue: %w", err)
		}
		if len(items) == 0 {
			return nil
		}

		for _, item := range items {
			if err := processQueueItem(ctx, scheduleStore, genQueueStore, domainStore, genStore, projectStore, taskClient, item, logger); err != nil {
				logger.Warnf("queue item %s failed: %v", item.ID, err)
			}
		}
	}
}

func processQueueItem(
	ctx context.Context,
	scheduleStore ScheduleStore,
	genQueueStore GenQueueStore,
	domainStore DomainStore,
	genStore GenerationStore,
	projectStore ProjectStore,
	taskClient tasks.Enqueuer,
	item sqlstore.QueueItem,
	logger *zap.SugaredLogger,
) error {
	domain, err := domainStore.Get(ctx, item.DomainID)
	if err != nil {
		_ = genQueueStore.MarkProcessed(ctx, item.ID, "failed", strPtr("domain not found"))
		return fmt.Errorf("domain not found")
	}

	if strings.TrimSpace(domain.MainKeyword) == "" {
		_ = genQueueStore.MarkProcessed(ctx, item.ID, "failed", strPtr("keyword is required"))
		return fmt.Errorf("missing keyword")
	}

	gens, err := genStore.ListByDomain(ctx, domain.ID)
	if err == nil && len(gens) > 0 {
		last := gens[0]
		if isGenerationActive(last.Status) {
			return nil
		}
	}

	project, err := projectStore.GetByID(ctx, domain.ProjectID)
	if err != nil {
		_ = genQueueStore.MarkProcessed(ctx, item.ID, "failed", strPtr("project not found"))
		return fmt.Errorf("project not found")
	}

	requestedBy := project.UserEmail
	if item.ScheduleID.Valid {
		if sched, err := scheduleStore.Get(ctx, item.ScheduleID.String); err == nil && sched.CreatedBy != "" {
			requestedBy = sched.CreatedBy
		}
	}

	genID := uuid.NewString()
	gen := sqlstore.Generation{
		ID:          genID,
		DomainID:    domain.ID,
		RequestedBy: sqlstore.NullableString(requestedBy),
		Status:      "pending",
		Progress:    0,
		Logs:        json.RawMessage(`[]`),
		Artifacts:   json.RawMessage(`{}`),
	}
	if err := genStore.Create(ctx, gen); err != nil {
		_ = genQueueStore.MarkProcessed(ctx, item.ID, "failed", strPtr("could not create generation"))
		return fmt.Errorf("create generation: %w", err)
	}
	_ = domainStore.SetLastGeneration(ctx, domain.ID, genID)
	_ = domainStore.UpdateStatus(ctx, domain.ID, "processing")

	task := tasks.NewGenerateTask(genID, domain.ID, "")
	if _, err := taskClient.Enqueue(ctx, task); err != nil {
		errMsg := fmt.Sprintf("enqueue failed: %v", err)
		_ = genStore.UpdateStatus(ctx, genID, "error", 0, &errMsg)
		_ = domainStore.UpdateStatus(ctx, domain.ID, "waiting")
		_ = genQueueStore.MarkProcessed(ctx, item.ID, "failed", &errMsg)
		return fmt.Errorf("enqueue: %w", err)
	}

	if err := genQueueStore.MarkProcessed(ctx, item.ID, "queued", nil); err != nil {
		logger.Warnf("failed to mark queue item queued: %v", err)
	}
	return nil
}

type LinkTaskStore interface {
	ListPending(ctx context.Context, limit int) ([]sqlstore.LinkTask, error)
	ListByProject(ctx context.Context, projectID string, filters sqlstore.LinkTaskFilters) ([]sqlstore.LinkTask, error)
	Create(ctx context.Context, task sqlstore.LinkTask) error
	Update(ctx context.Context, taskID string, updates sqlstore.LinkTaskUpdates) error
}

func applyLinkSchedules(ctx context.Context, linkScheduleStore LinkScheduleStore, linkTaskStore LinkTaskStore, domainStore DomainStore, logger *zap.SugaredLogger) error {
	now := time.Now().UTC().Truncate(time.Minute)
	return applyLinkSchedulesAt(ctx, linkScheduleStore, linkTaskStore, domainStore, logger, now)
}

func applyLinkSchedulesAt(
	ctx context.Context,
	linkScheduleStore LinkScheduleStore,
	linkTaskStore LinkTaskStore,
	domainStore DomainStore,
	logger *zap.SugaredLogger,
	now time.Time,
) error {
	if linkScheduleStore == nil {
		return fmt.Errorf("link schedule store is nil")
	}
	if linkTaskStore == nil {
		return fmt.Errorf("link task store is nil")
	}
	if domainStore == nil {
		return fmt.Errorf("domain store is nil")
	}

	schedules, err := linkScheduleStore.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list link schedules: %w", err)
	}
	if len(schedules) == 0 {
		return nil
	}

	for _, sched := range schedules {
		cfg, err := parseScheduleConfig(sched.Config)
		if err != nil {
			if logger != nil {
				logger.Warnf("invalid link schedule config %s: %v", sched.ID, err)
			}
			continue
		}
		loc := time.UTC
		if sched.Timezone.Valid {
			if tz, err := time.LoadLocation(strings.TrimSpace(sched.Timezone.String)); err == nil {
				loc = tz
			} else if logger != nil {
				logger.Warnf("invalid link schedule timezone %s: %v", sched.ID, err)
			}
		}
		localNow := now.In(loc)
		lastRunLocal := time.Time{}
		if sched.LastRunAt.Valid {
			lastRunLocal = sched.LastRunAt.Time.In(loc)
		}
		nextRunLocal, shouldRun, err := computeLinkScheduleNextRun(cfg, localNow, lastRunLocal)
		if err != nil {
			if logger != nil {
				logger.Warnf("link schedule %s skipped: %v", sched.ID, err)
			}
			continue
		}
		nextRunUTC := sql.NullTime{}
		if !nextRunLocal.IsZero() {
			nextRunUTC = sql.NullTime{Time: nextRunLocal.In(time.UTC), Valid: true}
		}
		if !sched.NextRunAt.Valid || (nextRunUTC.Valid && !sched.NextRunAt.Time.Equal(nextRunUTC.Time)) {
			if err := linkScheduleStore.Update(ctx, sched.ID, sqlstore.LinkScheduleUpdates{NextRunAt: &nextRunUTC}); err != nil && logger != nil {
				logger.Warnf("failed to update link next_run_at for %s: %v", sched.ID, err)
			}
		}
		if !shouldRun {
			continue
		}
		domains, err := domainStore.ListByProject(ctx, sched.ProjectID)
		if err != nil {
			if logger != nil {
				logger.Warnf("could not list domains for link schedule %s: %v", sched.ID, err)
			}
			continue
		}

		eligible := filterLinkDomains(domains)
		if len(eligible) == 0 {
			continue
		}

		activeTasks, err := listActiveLinkTasksByProject(ctx, linkTaskStore, sched.ProjectID)
		if err != nil {
			if logger != nil {
				logger.Warnf("could not list link tasks for schedule %s: %v", sched.ID, err)
			}
			continue
		}

		activeByDomain := map[string]sqlstore.LinkTask{}
		for _, task := range activeTasks {
			activeByDomain[task.DomainID] = task
		}

		limit := cfg.Limit
		if limit <= 0 {
			limit = len(eligible)
		}

		activeEligible := 0
		for _, d := range eligible {
			if _, ok := activeByDomain[d.ID]; ok {
				activeEligible++
			}
		}
		remaining := limit - activeEligible
		if remaining < 0 {
			remaining = 0
		}

		for _, d := range eligible {
			anchor := strings.TrimSpace(d.LinkAnchorText.String)
			target := strings.TrimSpace(d.LinkAcceptorURL.String)
			if task, ok := activeByDomain[d.ID]; ok {
				status := "pending"
				attempts := 0
				nullStr := sql.NullString{}
				nullTime := sql.NullTime{}
				updates := sqlstore.LinkTaskUpdates{
					AnchorText:       &anchor,
					TargetURL:        &target,
					Status:           &status,
					Attempts:         &attempts,
					ScheduledFor:     &now,
					FoundLocation:    &nullStr,
					GeneratedContent: &nullStr,
					ErrorMessage:     &nullStr,
					CompletedAt:      &nullTime,
				}
				if err := linkTaskStore.Update(ctx, task.ID, updates); err != nil && logger != nil {
					logger.Warnf("failed to update link task %s: %v", task.ID, err)
				}
				continue
			}

			if remaining == 0 {
				continue
			}

			createdBy := strings.TrimSpace(sched.CreatedBy)
			if createdBy == "" {
				createdBy = "scheduler"
			}
			task := sqlstore.LinkTask{
				ID:           uuid.NewString(),
				DomainID:     d.ID,
				AnchorText:   anchor,
				TargetURL:    target,
				ScheduledFor: now,
				Status:       "pending",
				Attempts:     0,
				CreatedBy:    createdBy,
				CreatedAt:    now,
			}
			if err := linkTaskStore.Create(ctx, task); err != nil {
				if logger != nil {
					logger.Warnf("failed to create link task for domain %s: %v", d.ID, err)
				}
				continue
			}
			remaining--
		}

		lastRunUTC := sql.NullTime{Time: now, Valid: true}
		updates := sqlstore.LinkScheduleUpdates{LastRunAt: &lastRunUTC, NextRunAt: &nextRunUTC}
		if err := linkScheduleStore.Update(ctx, sched.ID, updates); err != nil && logger != nil {
			logger.Warnf("failed to update link schedule run times %s: %v", sched.ID, err)
		}
	}

	return nil
}

func computeLinkScheduleNextRun(cfg scheduleConfig, now time.Time, lastRun time.Time) (time.Time, bool, error) {
	strategy := "daily"
	if strings.TrimSpace(cfg.Cron) != "" || strings.TrimSpace(cfg.Interval) != "" {
		strategy = "custom"
	} else if strings.TrimSpace(cfg.Weekday) != "" || cfg.Day > 0 {
		strategy = "weekly"
	}
	return computeScheduleNextRun(strategy, cfg, now, lastRun)
}

func filterLinkDomains(domains []sqlstore.Domain) []sqlstore.Domain {
	res := make([]sqlstore.Domain, 0, len(domains))
	for _, d := range domains {
		if !isDomainPublished(d) {
			continue
		}
		anchor := strings.TrimSpace(d.LinkAnchorText.String)
		target := strings.TrimSpace(d.LinkAcceptorURL.String)
		if !d.LinkAnchorText.Valid || !d.LinkAcceptorURL.Valid || anchor == "" || target == "" {
			continue
		}
		res = append(res, d)
	}
	return res
}

func isDomainPublished(d sqlstore.Domain) bool {
	if d.PublishedAt.Valid {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(d.Status), "published")
}

func listActiveLinkTasksByProject(ctx context.Context, linkTaskStore LinkTaskStore, projectID string) ([]sqlstore.LinkTask, error) {
	activeStatuses := []string{"pending", "searching", "found"}
	var res []sqlstore.LinkTask
	for _, status := range activeStatuses {
		status := status
		filters := sqlstore.LinkTaskFilters{Status: &status}
		list, err := linkTaskStore.ListByProject(ctx, projectID, filters)
		if err != nil {
			return nil, err
		}
		res = append(res, list...)
	}
	return res, nil
}

func processPendingLinkTasks(ctx context.Context, linkTaskStore LinkTaskStore, taskClient tasks.Enqueuer, logger *zap.SugaredLogger) error {
	if linkTaskStore == nil {
		return fmt.Errorf("link task store is nil")
	}
	if taskClient == nil {
		return fmt.Errorf("task client is nil")
	}
	items, err := linkTaskStore.ListPending(ctx, schedulerBatchSize)
	if err != nil {
		return fmt.Errorf("get pending link tasks: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	for _, item := range items {
		task := tasks.NewLinkTaskTask(item.ID)
		if _, err := taskClient.Enqueue(ctx, task); err != nil {
			errMsg := fmt.Sprintf("enqueue failed: %v", err)
			updateErr := linkTaskStore.Update(ctx, item.ID, sqlstore.LinkTaskUpdates{
				ErrorMessage: &sql.NullString{String: errMsg, Valid: true},
			})
			if updateErr != nil && logger != nil {
				logger.Warnf("failed to update link task error message: %v", updateErr)
			}
			continue
		}
		status := "searching"
		clearErr := sql.NullString{}
		if err := linkTaskStore.Update(ctx, item.ID, sqlstore.LinkTaskUpdates{
			Status:       &status,
			ErrorMessage: &clearErr,
		}); err != nil && logger != nil {
			logger.Warnf("failed to mark link task searching: %v", err)
		}
	}
	return nil
}

func isGenerationActive(status string) bool {
	active := map[string]bool{
		"pending":         true,
		"processing":      true,
		"pause_requested": true,
		"cancelling":      true,
	}
	return active[status]
}

func lastScheduledAt(items []sqlstore.QueueItem, scheduleID string) time.Time {
	var last time.Time
	for _, item := range items {
		if !item.ScheduleID.Valid || item.ScheduleID.String != scheduleID {
			continue
		}
		if item.ScheduledFor.After(last) {
			last = item.ScheduledFor
		}
	}
	return last
}

type scheduleConfig struct {
	Limit    int
	Cron     string
	Time     string
	Weekday  string
	Day      int
	Interval string
}

func parseScheduleConfig(raw json.RawMessage) (scheduleConfig, error) {
	if len(raw) == 0 {
		return scheduleConfig{}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return scheduleConfig{}, err
	}
	cfg := scheduleConfig{}
	if v, ok := data["limit"].(float64); ok {
		cfg.Limit = int(v)
	}
	if v, ok := data["cron"].(string); ok {
		cfg.Cron = strings.TrimSpace(v)
	}
	if v, ok := data["time"].(string); ok {
		cfg.Time = strings.TrimSpace(v)
	}
	if v, ok := data["weekday"].(string); ok {
		cfg.Weekday = strings.TrimSpace(v)
	}
	if v, ok := data["day"].(float64); ok {
		cfg.Day = int(v)
	}
	if v, ok := data["day"].(string); ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.Day = parsed
		}
	}
	if v, ok := data["interval"].(string); ok {
		cfg.Interval = strings.TrimSpace(v)
	}
	return cfg, nil
}

func computeScheduleNextRun(strategy string, cfg scheduleConfig, now time.Time, lastRun time.Time) (time.Time, bool, error) {
	strategy = strings.ToLower(strings.TrimSpace(strategy))
	if strategy == "" {
		return time.Time{}, false, fmt.Errorf("strategy is required")
	}

	if strategy == "immediate" {
		return time.Time{}, lastRun.IsZero(), nil
	}

	if cfg.Interval != "" {
		interval, err := parseInterval(cfg.Interval)
		if err != nil {
			return time.Time{}, false, err
		}
		if lastRun.IsZero() || now.Sub(lastRun) >= interval {
			return now.Add(interval), true, nil
		}
		return lastRun.Add(interval), false, nil
	}

	switch strategy {
	case "daily":
		scheduled, err := scheduleAtTime(now, cfg.Time)
		if err != nil {
			return time.Time{}, false, err
		}
		if now.Before(scheduled) {
			return scheduled, false, nil
		}
		if !lastRun.IsZero() && sameDay(lastRun, now) {
			return scheduled.Add(24 * time.Hour), false, nil
		}
		return scheduled.Add(24 * time.Hour), true, nil
	case "weekly":
		targetWeekday, ok := resolveWeekday(cfg.Weekday, cfg.Day)
		if !ok {
			return time.Time{}, false, fmt.Errorf("weekday is required")
		}
		scheduled, err := scheduleAtWeekday(now, targetWeekday, cfg.Time)
		if err != nil {
			return time.Time{}, false, err
		}
		if now.Before(scheduled) {
			return scheduled, false, nil
		}
		if !lastRun.IsZero() && sameWeek(lastRun, now) {
			return scheduled.Add(7 * 24 * time.Hour), false, nil
		}
		return scheduled.Add(7 * 24 * time.Hour), true, nil
	case "custom":
		if cfg.Cron == "" {
			return time.Time{}, false, fmt.Errorf("cron is required for custom strategy")
		}
		next, due, err := cronNext(cfg.Cron, now)
		return next, due, err
	default:
		return time.Time{}, false, fmt.Errorf("unknown strategy: %s", strategy)
	}
}

func cronNext(expr string, now time.Time) (time.Time, bool, error) {
	sched, err := cron.ParseStandard(expr)
	if err != nil {
		return time.Time{}, false, err
	}
	windowStart := now.Add(-time.Minute)
	next := sched.Next(windowStart)
	if !next.After(now) {
		return sched.Next(now), true, nil
	}
	return next, false, nil
}

func scheduleAtTime(now time.Time, value string) (time.Time, error) {
	hour, minute, err := parseHourMinute(value, now)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location()), nil
}

func scheduleAtWeekday(now time.Time, weekday time.Weekday, value string) (time.Time, error) {
	hour, minute, err := parseHourMinute(value, now)
	if err != nil {
		return time.Time{}, err
	}
	diff := int(weekday - now.Weekday())
	if diff < 0 {
		diff += 7
	}
	target := now.AddDate(0, 0, diff)
	return time.Date(target.Year(), target.Month(), target.Day(), hour, minute, 0, 0, now.Location()), nil
}

func parseHourMinute(value string, now time.Time) (int, int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, 0, nil
	}
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minute")
	}
	return hour, minute, nil
}

func resolveWeekday(value string, day int) (time.Weekday, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "mon", "monday":
		return time.Monday, true
	case "tue", "tues", "tuesday":
		return time.Tuesday, true
	case "wed", "wednesday":
		return time.Wednesday, true
	case "thu", "thurs", "thursday":
		return time.Thursday, true
	case "fri", "friday":
		return time.Friday, true
	case "sat", "saturday":
		return time.Saturday, true
	case "sun", "sunday":
		return time.Sunday, true
	}
	if day >= 1 && day <= 7 {
		return time.Weekday(day % 7), true
	}
	return time.Weekday(0), false
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func sameWeek(a, b time.Time) bool {
	ay, aw := a.ISOWeek()
	by, bw := b.ISOWeek()
	return ay == by && aw == bw
}

func parseInterval(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("interval is empty")
	}
	if strings.HasSuffix(value, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(value, "w") {
		n, err := strconv.Atoi(strings.TrimSuffix(value, "w"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return time.ParseDuration(value)
}

func strPtr(val string) *string {
	return &val
}
