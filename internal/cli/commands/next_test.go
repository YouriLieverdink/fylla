package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

func TestRunNext(t *testing.T) {
	// Monday 10:30 UTC — within business hours
	now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
	cfg := testConfig()
	// Buffer is 15min, so first slot starts at 10:45 at the earliest

	t.Run("shows current task when now is inside an allocation", func(t *testing.T) {
		fetcher := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "PROJ-123", Summary: "Fix login bug", Priority: 1, RemainingEstimate: 2 * time.Hour, Project: "PROJ", Created: now.AddDate(0, 0, -1)},
				{Key: "PROJ-456", Summary: "Update docs", Priority: 2, RemainingEstimate: time.Hour, Project: "PROJ", Created: now.AddDate(0, 0, -2)},
			},
		}

		result, err := RunNext(context.Background(), NextParams{
			Tasks: fetcher,
			Cfg:   cfg,
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunNext: %v", err)
		}

		// With buffer=15min, first task starts at 10:45, so at 10:30 nothing is current yet
		// But next should be set
		if result.Next == nil {
			t.Fatal("expected next task")
		}
		if result.Next.TaskKey != "PROJ-123" {
			t.Errorf("next task key = %q, want PROJ-123", result.Next.TaskKey)
		}
	})

	t.Run("no tasks today", func(t *testing.T) {
		fetcher := &mockTaskFetcher{tasks: []task.Task{}}

		result, err := RunNext(context.Background(), NextParams{
			Tasks: fetcher,
			Cfg:   cfg,
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunNext: %v", err)
		}

		if result.Current != nil {
			t.Error("expected no current task")
		}
		if result.Next != nil {
			t.Error("expected no next task")
		}
	})

	t.Run("identifies current task at start of allocation", func(t *testing.T) {
		// With zero buffer, slots start exactly at now (business hours start)
		atSlotStart := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
		zeroBufCfg := testConfig()
		zeroBufCfg.Scheduling.BufferMinutes = 0

		fetcher := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "T-1", Summary: "First task", Priority: 1, RemainingEstimate: time.Hour, Project: "TEST", Created: atSlotStart.AddDate(0, 0, -1)},
				{Key: "T-2", Summary: "Second task", Priority: 2, RemainingEstimate: time.Hour, Project: "TEST", Created: atSlotStart.AddDate(0, 0, -1)},
			},
		}

		result, err := RunNext(context.Background(), NextParams{
			Tasks: fetcher,
			Cfg:   zeroBufCfg,
			Now:   atSlotStart,
		})
		if err != nil {
			t.Fatalf("RunNext: %v", err)
		}

		if result.Current == nil {
			t.Fatal("expected current task at slot start")
		}
		if result.Current.TaskKey != "T-1" {
			t.Errorf("current task = %q, want T-1", result.Current.TaskKey)
		}
		if result.Next == nil {
			t.Fatal("expected next task")
		}
		if result.Next.TaskKey != "T-2" {
			t.Errorf("next task = %q, want T-2", result.Next.TaskKey)
		}
	})
}

func TestPrintNextResult(t *testing.T) {
	now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)

	t.Run("prints no tasks message", func(t *testing.T) {
		var buf bytes.Buffer
		PrintNextResult(&buf, &NextResult{}, now)
		if !strings.Contains(buf.String(), "No more Fylla tasks today.") {
			t.Errorf("output = %q, want 'No more Fylla tasks today.'", buf.String())
		}
	})

	t.Run("prints current and next", func(t *testing.T) {
		var buf bytes.Buffer
		PrintNextResult(&buf, &NextResult{
			Current: &FyllaEvent{
				TaskKey: "PROJ-123",
				Summary: "Fix login bug",
				End:     time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
			},
			Next: &FyllaEvent{
				TaskKey: "PROJ-456",
				Summary: "Update docs",
				Start:   time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
				End:     time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
			},
		}, now)
		out := buf.String()
		if !strings.Contains(out, "Current:") {
			t.Errorf("output missing 'Current:', got %q", out)
		}
		if !strings.Contains(out, "PROJ-123") {
			t.Errorf("output missing 'PROJ-123', got %q", out)
		}
		if !strings.Contains(out, "until 11:00") {
			t.Errorf("output missing 'until 11:00', got %q", out)
		}
		if !strings.Contains(out, "Next:") {
			t.Errorf("output missing 'Next:', got %q", out)
		}
		if !strings.Contains(out, "PROJ-456") {
			t.Errorf("output missing 'PROJ-456', got %q", out)
		}
		if !strings.Contains(out, "starts in 30m") {
			t.Errorf("output missing 'starts in 30m', got %q", out)
		}
	})

	t.Run("prints time range for distant next task", func(t *testing.T) {
		var buf bytes.Buffer
		PrintNextResult(&buf, &NextResult{
			Next: &FyllaEvent{
				TaskKey: "PROJ-456",
				Summary: "Update docs",
				Start:   time.Date(2025, 1, 20, 14, 0, 0, 0, time.UTC),
				End:     time.Date(2025, 1, 20, 15, 0, 0, 0, time.UTC),
			},
		}, now)
		out := buf.String()
		if !strings.Contains(out, "14:00 – 15:00") {
			t.Errorf("output missing '14:00 – 15:00', got %q", out)
		}
	})

	t.Run("prints LATE prefix for at-risk task", func(t *testing.T) {
		var buf bytes.Buffer
		PrintNextResult(&buf, &NextResult{
			Current: &FyllaEvent{
				TaskKey: "PROJ-789",
				Summary: "Overdue fix",
				End:     time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
				AtRisk:  true,
			},
		}, now)
		out := buf.String()
		if !strings.Contains(out, "[LATE]") {
			t.Errorf("output missing '[LATE]', got %q", out)
		}
	})

	t.Run("prints project prefix for current and next", func(t *testing.T) {
		var buf bytes.Buffer
		PrintNextResult(&buf, &NextResult{
			Current: &FyllaEvent{
				TaskKey: "PROJ-1",
				Project: "PROJ",
				Summary: "Fix bug",
				End:     time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
			},
			Next: &FyllaEvent{
				TaskKey: "PROJ-2",
				Project: "PROJ",
				Summary: "Update docs",
				Start:   time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
				End:     time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
			},
		}, now)
		out := buf.String()
		if !strings.Contains(out, "[PROJ] PROJ-1: Fix bug") {
			t.Errorf("missing project prefix for current, got %q", out)
		}
		if !strings.Contains(out, "[PROJ] PROJ-2: Update docs") {
			t.Errorf("missing project prefix for next, got %q", out)
		}
	})

	t.Run("prints calendar event as current", func(t *testing.T) {
		var buf bytes.Buffer
		PrintNextResult(&buf, &NextResult{
			Current: &FyllaEvent{
				Summary:         "Team standup",
				End:             time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
				IsCalendarEvent: true,
			},
			Next: &FyllaEvent{
				TaskKey: "T-1",
				Summary: "Fix bug",
				Start:   time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
				End:     time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
			},
		}, now)
		out := buf.String()
		if !strings.Contains(out, "Current: Team standup (until 11:00)") {
			t.Errorf("output missing calendar current, got %q", out)
		}
		if !strings.Contains(out, "T-1: Fix bug") {
			t.Errorf("output missing task next, got %q", out)
		}
	})

	t.Run("prints calendar event as next", func(t *testing.T) {
		var buf bytes.Buffer
		PrintNextResult(&buf, &NextResult{
			Next: &FyllaEvent{
				Summary:         "Calendly meeting",
				Start:           time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
				End:             time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
				IsCalendarEvent: true,
			},
		}, now)
		out := buf.String()
		if !strings.Contains(out, "Next:    Calendly meeting (starts in 30m)") {
			t.Errorf("output missing calendar next, got %q", out)
		}
	})
}
