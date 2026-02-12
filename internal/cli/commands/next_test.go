package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
)

func TestRunNext(t *testing.T) {
	// Monday 10:30 UTC
	now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)

	t.Run("shows current task when now is inside an event", func(t *testing.T) {
		cal := &mockCalendar{
			events: []calendar.Event{
				{
					Title: "[Fylla] PROJ-123: Fix login bug",
					Start: time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
				},
				{
					Title: "[Fylla] PROJ-456: Update docs",
					Start: time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
				},
			},
		}

		result, err := RunNext(context.Background(), NextParams{
			Cal: cal,
			Now: now,
		})
		if err != nil {
			t.Fatalf("RunNext: %v", err)
		}

		if result.Current == nil {
			t.Fatal("expected current task")
		}
		if result.Current.TaskKey != "PROJ-123" {
			t.Errorf("current task key = %q, want PROJ-123", result.Current.TaskKey)
		}
		if result.Current.Summary != "Fix login bug" {
			t.Errorf("current summary = %q, want 'Fix login bug'", result.Current.Summary)
		}

		if result.Next == nil {
			t.Fatal("expected next task")
		}
		if result.Next.TaskKey != "PROJ-456" {
			t.Errorf("next task key = %q, want PROJ-456", result.Next.TaskKey)
		}
	})

	t.Run("shows next task when between events", func(t *testing.T) {
		betweenEvents := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
		cal := &mockCalendar{
			events: []calendar.Event{
				{
					Title: "[Fylla] PROJ-123: Fix login bug",
					Start: time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
				},
				{
					Title: "[Fylla] PROJ-456: Update docs",
					Start: time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
				},
			},
		}

		result, err := RunNext(context.Background(), NextParams{
			Cal: cal,
			Now: betweenEvents,
		})
		if err != nil {
			t.Fatalf("RunNext: %v", err)
		}

		if result.Current != nil {
			t.Error("expected no current task")
		}
		if result.Next == nil {
			t.Fatal("expected next task")
		}
		if result.Next.TaskKey != "PROJ-456" {
			t.Errorf("next task key = %q, want PROJ-456", result.Next.TaskKey)
		}
	})

	t.Run("no tasks today", func(t *testing.T) {
		cal := &mockCalendar{events: []calendar.Event{}}

		result, err := RunNext(context.Background(), NextParams{
			Cal: cal,
			Now: now,
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

	t.Run("handles LATE prefix", func(t *testing.T) {
		cal := &mockCalendar{
			events: []calendar.Event{
				{
					Title: "[LATE] [Fylla] PROJ-789: Overdue fix",
					Start: time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
				},
			},
		}

		result, err := RunNext(context.Background(), NextParams{
			Cal: cal,
			Now: now,
		})
		if err != nil {
			t.Fatalf("RunNext: %v", err)
		}

		if result.Current == nil {
			t.Fatal("expected current task")
		}
		if result.Current.TaskKey != "PROJ-789" {
			t.Errorf("task key = %q, want PROJ-789", result.Current.TaskKey)
		}
		if !result.Current.AtRisk {
			t.Error("expected AtRisk to be true")
		}
	})

	t.Run("ignores non-Fylla events", func(t *testing.T) {
		cal := &mockCalendar{
			events: []calendar.Event{
				{
					Title: "Team standup",
					Start: time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC),
				},
				{
					Title: "[Fylla] PROJ-123: Real task",
					Start: time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
				},
			},
		}

		result, err := RunNext(context.Background(), NextParams{
			Cal: cal,
			Now: now,
		})
		if err != nil {
			t.Fatalf("RunNext: %v", err)
		}

		if result.Current != nil {
			t.Error("expected no current task (standup is not a Fylla event)")
		}
		if result.Next == nil {
			t.Fatal("expected next task")
		}
		if result.Next.TaskKey != "PROJ-123" {
			t.Errorf("next task key = %q, want PROJ-123", result.Next.TaskKey)
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
}
