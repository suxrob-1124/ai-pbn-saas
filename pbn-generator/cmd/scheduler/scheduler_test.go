package main

import (
	"testing"
	"time"
)

func TestComputeScheduleNextRunImmediate(t *testing.T) {
	now := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	next, due, err := computeScheduleNextRun("immediate", scheduleConfig{}, now, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !due {
		t.Fatalf("expected immediate schedule to run")
	}
	if !next.IsZero() {
		t.Fatalf("expected empty next run for immediate, got %s", next.Format(time.RFC3339))
	}

	lastRun := now.Add(-time.Hour)
	next, due, err = computeScheduleNextRun("immediate", scheduleConfig{}, now, lastRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if due {
		t.Fatalf("expected immediate schedule to skip after first run")
	}
	if !next.IsZero() {
		t.Fatalf("expected empty next run for immediate after run")
	}
}

func TestComputeScheduleNextRunDaily(t *testing.T) {
	cfg := scheduleConfig{Time: "10:00"}
	now := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)

	next, due, err := computeScheduleNextRun("daily", cfg, now, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !due {
		t.Fatalf("expected daily schedule to run")
	}
	expected := time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected next run %s, got %s", expected, next)
	}

	early := time.Date(2026, 2, 4, 9, 0, 0, 0, time.UTC)
	next, due, err = computeScheduleNextRun("daily", cfg, early, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if due {
		t.Fatalf("expected daily schedule to wait before time")
	}
	expected = time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected next run %s, got %s", expected, next)
	}
}

func TestComputeScheduleNextRunWeeklyDay(t *testing.T) {
	cfg := scheduleConfig{Time: "10:00", Day: 3}        // Wednesday
	now := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC) // Wednesday
	next, due, err := computeScheduleNextRun("weekly", cfg, now, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !due {
		t.Fatalf("expected weekly schedule to run")
	}
	expected := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected next run %s, got %s", expected, next)
	}
}

func TestComputeScheduleNextRunCustomCron(t *testing.T) {
	cfg := scheduleConfig{Cron: "0 10 * * *"}
	now := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	next, due, err := computeScheduleNextRun("custom", cfg, now, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !due {
		t.Fatalf("expected custom schedule to run")
	}
	expected := time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected next run %s, got %s", expected, next)
	}
}

func TestParseInterval(t *testing.T) {
	dur, err := parseInterval("1d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dur != 24*time.Hour {
		t.Fatalf("expected 24h, got %v", dur)
	}

	if _, err := parseInterval("bad"); err == nil {
		t.Fatalf("expected error for invalid interval")
	}
}
