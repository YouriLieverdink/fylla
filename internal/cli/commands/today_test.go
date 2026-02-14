package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/task"
)

func TestRunToday(t *testing.T) {
	// Monday 09:00 UTC — business hours start
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

	t.Run("allocates tasks into today's business hours", func(t *testing.T) {
		fetcher := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "PROJ-1", Summary: "Write the docs", Priority: 1, RemainingEstimate: 30 * time.Minute, Project: "PROJ", Created: now.AddDate(0, 0, -1)},
				{Key: "PROJ-2", Summary: "Fix login bug", Priority: 2, RemainingEstimate: time.Hour, Project: "PROJ", Created: now.AddDate(0, 0, -2)},
				{Key: "PROJ-3", Summary: "Review PR", Priority: 3, RemainingEstimate: 30 * time.Minute, Project: "PROJ", Created: now.AddDate(0, 0, -3)},
			},
		}

		result, err := RunToday(context.Background(), TodayParams{
			Tasks: fetcher,
			Cfg:   testConfig(),
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

		if len(result.Events) != 3 {
			t.Fatalf("got %d events, want 3", len(result.Events))
		}
		// Highest priority task should be first
		if result.Events[0].TaskKey != "PROJ-1" {
			t.Errorf("first event = %q, want PROJ-1", result.Events[0].TaskKey)
		}
	})

	t.Run("empty task list", func(t *testing.T) {
		fetcher := &mockTaskFetcher{tasks: []task.Task{}}

		result, err := RunToday(context.Background(), TodayParams{
			Tasks: fetcher,
			Cfg:   testConfig(),
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

		if len(result.Events) != 0 {
			t.Fatalf("got %d events, want 0", len(result.Events))
		}
	})

	t.Run("marks at-risk tasks with overdue due dates", func(t *testing.T) {
		dueYesterday := time.Date(2025, 1, 19, 17, 0, 0, 0, time.UTC)
		fetcher := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "PROJ-4", Summary: "Overdue task", Priority: 1, DueDate: &dueYesterday, RemainingEstimate: 30 * time.Minute, Project: "PROJ", Created: now.AddDate(0, 0, -5)},
			},
		}

		result, err := RunToday(context.Background(), TodayParams{
			Tasks: fetcher,
			Cfg:   testConfig(),
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

		if len(result.Events) != 1 {
			t.Fatalf("got %d events, want 1", len(result.Events))
		}
		if !result.Events[0].AtRisk {
			t.Error("expected AtRisk to be true")
		}
		if result.Events[0].TaskKey != "PROJ-4" {
			t.Errorf("task key = %q, want PROJ-4", result.Events[0].TaskKey)
		}
	})

	t.Run("events fall within business hours", func(t *testing.T) {
		fetcher := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "T-1", Summary: "Task", Priority: 1, RemainingEstimate: time.Hour, Project: "TEST", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunToday(context.Background(), TodayParams{
			Tasks: fetcher,
			Cfg:   testConfig(),
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

		for _, ev := range result.Events {
			if ev.IsCalendarEvent {
				continue
			}
			if ev.Start.Hour() < 9 || ev.End.Hour() > 17 {
				t.Errorf("event %v-%v outside business hours 09:00-17:00", ev.Start, ev.End)
			}
		}
	})

	t.Run("merges calendar events into timeline", func(t *testing.T) {
		cal := &mockCalendar{
			events: []calendar.Event{
				{
					Title: "Team standup",
					Start: time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC),
				},
			},
		}
		fetcher := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "T-1", Summary: "Task", Priority: 1, RemainingEstimate: 30 * time.Minute, Project: "TEST", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunToday(context.Background(), TodayParams{
			Cal:   cal,
			Tasks: fetcher,
			Cfg:   testConfig(),
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

		// Should have both the task and the calendar event
		if len(result.Events) != 2 {
			t.Fatalf("got %d events, want 2", len(result.Events))
		}

		calCount := 0
		for _, ev := range result.Events {
			if ev.IsCalendarEvent {
				calCount++
				if ev.Summary != "Team standup" {
					t.Errorf("calendar event summary = %q, want Team standup", ev.Summary)
				}
			}
		}
		if calCount != 1 {
			t.Errorf("got %d calendar events, want 1", calCount)
		}
	})

	t.Run("excludes Fylla-created events from calendar merge", func(t *testing.T) {
		cal := &mockCalendar{
			events: []calendar.Event{
				{
					Title:       "[Fylla] T-1: Some task",
					Description: "fylla: T-1\nhttps://example.com",
					Start:       time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
					End:         time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
				},
				{
					Title: "Real meeting",
					Start: time.Date(2025, 1, 20, 14, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 15, 0, 0, 0, time.UTC),
				},
			},
		}
		fetcher := &mockTaskFetcher{tasks: []task.Task{}}

		result, err := RunToday(context.Background(), TodayParams{
			Cal:   cal,
			Tasks: fetcher,
			Cfg:   testConfig(),
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

		// Only the real meeting should appear, not the Fylla event
		if len(result.Events) != 1 {
			t.Fatalf("got %d events, want 1", len(result.Events))
		}
		if result.Events[0].Summary != "Real meeting" {
			t.Errorf("event summary = %q, want Real meeting", result.Events[0].Summary)
		}
	})

	t.Run("excludes all-day events from calendar merge", func(t *testing.T) {
		cal := &mockCalendar{
			events: []calendar.Event{
				{
					Title:  "Holiday",
					Start:  time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC),
					End:    time.Date(2025, 1, 21, 0, 0, 0, 0, time.UTC),
					AllDay: true,
				},
			},
		}
		fetcher := &mockTaskFetcher{tasks: []task.Task{}}

		result, err := RunToday(context.Background(), TodayParams{
			Cal:   cal,
			Tasks: fetcher,
			Cfg:   testConfig(),
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

		if len(result.Events) != 0 {
			t.Fatalf("got %d events, want 0 (all-day should be excluded)", len(result.Events))
		}
	})

	t.Run("timeline is sorted by start time", func(t *testing.T) {
		cal := &mockCalendar{
			events: []calendar.Event{
				{
					Title: "Afternoon meeting",
					Start: time.Date(2025, 1, 20, 14, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 15, 0, 0, 0, time.UTC),
				},
			},
		}
		fetcher := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "T-1", Summary: "Morning task", Priority: 1, RemainingEstimate: 30 * time.Minute, Project: "TEST", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunToday(context.Background(), TodayParams{
			Cal:   cal,
			Tasks: fetcher,
			Cfg:   testConfig(),
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

		for i := 1; i < len(result.Events); i++ {
			if result.Events[i].Start.Before(result.Events[i-1].Start) {
				t.Errorf("events not sorted: %v before %v", result.Events[i].Start, result.Events[i-1].Start)
			}
		}
	})
}

func TestPrintTodayResult(t *testing.T) {
	now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)

	t.Run("prints empty state", func(t *testing.T) {
		var buf bytes.Buffer
		PrintTodayResult(&buf, &TodayResult{}, now)
		if !strings.Contains(buf.String(), "No Fylla tasks scheduled for today.") {
			t.Errorf("output = %q, want empty state message", buf.String())
		}
	})

	t.Run("prints full schedule with current marker", func(t *testing.T) {
		var buf bytes.Buffer
		PrintTodayResult(&buf, &TodayResult{
			Events: []FyllaEvent{
				{
					TaskKey: "PROJ-1",
					Summary: "Write the docs",
					Start:   time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
					End:     time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC),
				},
				{
					TaskKey: "PROJ-2",
					Summary: "Fix login bug",
					Start:   time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC),
					End:     time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
				},
				{
					TaskKey: "PROJ-3",
					Summary: "Review PR",
					Start:   time.Date(2025, 1, 20, 13, 0, 0, 0, time.UTC),
					End:     time.Date(2025, 1, 20, 14, 0, 0, 0, time.UTC),
				},
			},
		}, now)

		out := buf.String()
		if !strings.Contains(out, "Today's schedule:") {
			t.Errorf("missing header, got:\n%s", out)
		}
		// First event should not be current (ended at 10:30, now is 10:30)
		if !strings.Contains(out, "  09:00 – 10:30  PROJ-1: Write the docs") {
			t.Errorf("missing first event, got:\n%s", out)
		}
		// Second event should be current
		if !strings.Contains(out, "> 10:30 – 12:00  PROJ-2: Fix login bug") {
			t.Errorf("missing current marker on second event, got:\n%s", out)
		}
		if !strings.Contains(out, "(current)") {
			t.Errorf("missing (current) suffix, got:\n%s", out)
		}
		// Third event should not be current
		if !strings.Contains(out, "  13:00 – 14:00  PROJ-3: Review PR") {
			t.Errorf("missing third event, got:\n%s", out)
		}
	})

	t.Run("prints LATE prefix", func(t *testing.T) {
		var buf bytes.Buffer
		PrintTodayResult(&buf, &TodayResult{
			Events: []FyllaEvent{
				{
					TaskKey: "PROJ-4",
					Summary: "Update tests",
					Start:   time.Date(2025, 1, 20, 14, 0, 0, 0, time.UTC),
					End:     time.Date(2025, 1, 20, 15, 30, 0, 0, time.UTC),
					AtRisk:  true,
				},
			},
		}, now)

		out := buf.String()
		if !strings.Contains(out, "[LATE]") {
			t.Errorf("missing [LATE] prefix, got:\n%s", out)
		}
		if !strings.Contains(out, "PROJ-4: Update tests") {
			t.Errorf("missing task details, got:\n%s", out)
		}
	})

	t.Run("prints calendar events without task key", func(t *testing.T) {
		var buf bytes.Buffer
		PrintTodayResult(&buf, &TodayResult{
			Events: []FyllaEvent{
				{
					TaskKey: "T-1",
					Summary: "Fix bug",
					Start:   time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
					End:     time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
				},
				{
					Summary:         "Team standup",
					Start:           time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
					End:             time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC),
					IsCalendarEvent: true,
				},
			},
		}, now)

		out := buf.String()
		if !strings.Contains(out, "T-1: Fix bug") {
			t.Errorf("missing task event, got:\n%s", out)
		}
		if !strings.Contains(out, "10:00 – 10:30  Team standup") {
			t.Errorf("missing calendar event, got:\n%s", out)
		}
		// Calendar event should NOT have ":" prefix pattern like task events
		if strings.Contains(out, ": Team standup") {
			t.Errorf("calendar event should not have task key prefix, got:\n%s", out)
		}
	})
}
