package httpserver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

func computeLinkScheduleNextRun(cfg scheduleConfig, now time.Time, lastRun time.Time) (time.Time, bool, error) {
	strategy := "daily"
	if strings.TrimSpace(cfg.Cron) != "" || strings.TrimSpace(cfg.Interval) != "" {
		strategy = "custom"
	} else if strings.TrimSpace(cfg.Weekday) != "" || cfg.Day > 0 {
		strategy = "weekly"
	}
	return computeScheduleNextRun(strategy, cfg, now, lastRun)
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
		if cfg.Cron == "" && cfg.Interval == "" {
			return time.Time{}, false, fmt.Errorf("cron or interval is required for custom strategy")
		}
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

func enqueueScheduleDomains(
	ctx context.Context,
	genQueue GenQueueStore,
	sched sqlstore.Schedule,
	cfg scheduleConfig,
	domains []sqlstore.Domain,
	queueItems []sqlstore.QueueItem,
	now time.Time,
) (int, error) {
	eligibleDomains := filterGenerationDomains(domains)
	eligibleIDs := map[string]bool{}
	for _, d := range eligibleDomains {
		eligibleIDs[d.ID] = true
	}

	queuedDomains := map[string]bool{}
	queuedForSchedule := 0
	for _, item := range queueItems {
		if item.Status != "pending" && item.Status != "queued" {
			continue
		}
		if !eligibleIDs[item.DomainID] {
			continue
		}
		queuedDomains[item.DomainID] = true
		if item.ScheduleID.Valid && item.ScheduleID.String == sched.ID {
			queuedForSchedule++
		}
	}

	limit := cfg.Limit
	if limit <= 0 {
		limit = len(eligibleDomains)
	}
	remaining := limit - queuedForSchedule
	if remaining <= 0 {
		return 0, nil
	}

	count := 0
	for _, d := range eligibleDomains {
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
		if err := genQueue.Enqueue(ctx, item); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
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

func scheduleRunAtToday(now time.Time, cfg scheduleConfig) (time.Time, error) {
	return scheduleAtTime(now, cfg.Time)
}

func effectiveLinkReadyAt(domain sqlstore.Domain, scheduleRunAt time.Time) time.Time {
	effective := scheduleRunAt
	if effective.IsZero() && domain.LinkReadyAt.Valid {
		return domain.LinkReadyAt.Time
	}
	if domain.LinkReadyAt.Valid && domain.LinkReadyAt.Time.After(effective) {
		return domain.LinkReadyAt.Time
	}
	return effective
}

func isLinkStatusEligible(domain sqlstore.Domain) bool {
	if !domain.LinkStatus.Valid {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(domain.LinkStatus.String))
	if status == "" || status == "ready" {
		return true
	}
	return status == "needs_relink" || status == "pending"
}

func filterLinkDomains(domains []sqlstore.Domain, now time.Time, scheduleRunAt time.Time) []sqlstore.Domain {
	res := make([]sqlstore.Domain, 0, len(domains))
	for _, d := range domains {
		if !isDomainPublished(d) {
			continue
		}
		if !isLinkStatusEligible(d) {
			continue
		}
		anchor := strings.TrimSpace(d.LinkAnchorText.String)
		target := strings.TrimSpace(d.LinkAcceptorURL.String)
		if !d.LinkAnchorText.Valid || !d.LinkAcceptorURL.Valid || anchor == "" || target == "" {
			continue
		}
		effective := effectiveLinkReadyAt(d, scheduleRunAt)
		if !effective.IsZero() && effective.After(now) {
			continue
		}
		res = append(res, d)
	}
	return res
}

func filterGenerationDomains(domains []sqlstore.Domain) []sqlstore.Domain {
	res := make([]sqlstore.Domain, 0, len(domains))
	for _, d := range domains {
		if !isDomainWaiting(d) {
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

func isDomainWaiting(d sqlstore.Domain) bool {
	return strings.EqualFold(strings.TrimSpace(d.Status), "waiting")
}

func listActiveLinkTasksByProject(ctx context.Context, linkTaskStore LinkTaskStore, projectID string) ([]sqlstore.LinkTask, error) {
	activeStatuses := []string{"pending", "searching", "removing"}
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
