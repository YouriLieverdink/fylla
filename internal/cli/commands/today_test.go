package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
)

func TestRunToday(t *testing.T) {
	// Monday 09:00 UTC — business hours start
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

	t.Run("reads fylla events from calendar", func(t *testing.T) {
		cal := &mockCalendar{
			fyllaEvents: []calendar.Event{
				{
					Title:       "[PROJ] Write the docs",
					Description: "fylla: PROJ-1\nhttps://test.atlassian.net/browse/PROJ-1",
					Start:       time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
					End:         time.Date(2025, 1, 20, 9, 30, 0, 0, time.UTC),
				},
				{
					Title:       "[PROJ] Fix login bug",
					Description: "fylla: PROJ-2\nhttps://test.atlassian.net/browse/PROJ-2",
					Start:       time.Date(2025, 1, 20, 9, 30, 0, 0, time.UTC),
					End:         time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC),
				},
				{
					Title:       "[PROJ] Review PR",
					Description: "fylla: PROJ-3\nhttps://test.atlassian.net/browse/PROJ-3",
					Start:       time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC),
					End:         time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC),
				},
			},
		}

		result, err := RunToday(context.Background(), TodayParams{
			Cal: cal,
			Now: now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

		if len(result.Events) != 3 {
			t.Fatalf("got %d events, want 3", len(result.Events))
		}
		if result.Events[0].TaskKey != "PROJ-1" {
			t.Errorf("first event = %q, want PROJ-1", result.Events[0].TaskKey)
		}
		if result.Events[0].Project != "PROJ" {
			t.Errorf("first event project = %q, want PROJ", result.Events[0].Project)
		}
		if result.Events[0].Summary != "Write the docs" {
			t.Errorf("first event summary = %q, want Write the docs", result.Events[0].Summary)
		}
	})

	t.Run("empty calendar", func(t *testing.T) {
		cal := &mockCalendar{}

		result, err := RunToday(context.Background(), TodayParams{
			Cal: cal,
			Now: now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

		if len(result.Events) != 0 {
			t.Fatalf("got %d events, want 0", len(result.Events))
		}
	})

	t.Run("parses at-risk events", func(t *testing.T) {
		cal := &mockCalendar{
			fyllaEvents: []calendar.Event{
				{
					Title:       "⚠️ [PROJ] Overdue task",
					Description: "fylla: PROJ-4\nhttps://test.atlassian.net/browse/PROJ-4",
					Start:       time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
					End:         time.Date(2025, 1, 20, 9, 30, 0, 0, time.UTC),
				},
			},
		}

		result, err := RunToday(context.Background(), TodayParams{
			Cal: cal,
			Now: now,
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

	t.Run("merges calendar events into timeline", func(t *testing.T) {
		cal := &mockCalendar{
			fyllaEvents: []calendar.Event{
				{
					Title:       "Fix bug",
					Description: "fylla: T-1\nhttps://test.atlassian.net/browse/T-1",
					Start:       time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
					End:         time.Date(2025, 1, 20, 9, 30, 0, 0, time.UTC),
				},
			},
			events: []calendar.Event{
				{
					Title: "Team standup",
					Start: time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC),
				},
			},
		}

		result, err := RunToday(context.Background(), TodayParams{
			Cal: cal,
			Now: now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

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

	t.Run("excludes Fylla-created events from source calendar", func(t *testing.T) {
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

		result, err := RunToday(context.Background(), TodayParams{
			Cal: cal,
			Now: now,
		})
		if err != nil {
			t.Fatalf("RunToday: %v", err)
		}

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

		result, err := RunToday(context.Background(), TodayParams{
			Cal: cal,
			Now: now,
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
			fyllaEvents: []calendar.Event{
				{
					Title:       "Morning task",
					Description: "fylla: T-1\nhttps://test.atlassian.net/browse/T-1",
					Start:       time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
					End:         time.Date(2025, 1, 20, 9, 30, 0, 0, time.UTC),
				},
			},
			events: []calendar.Event{
				{
					Title: "Afternoon meeting",
					Start: time.Date(2025, 1, 20, 14, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 15, 0, 0, 0, time.UTC),
				},
			},
		}

		result, err := RunToday(context.Background(), TodayParams{
			Cal: cal,
			Now: now,
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
		PrintTodayResult(&buf, &TodayResult{}, now, false)
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
		}, now, false)

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
		}, now, false)

		out := buf.String()
		if !strings.Contains(out, "[LATE]") {
			t.Errorf("missing [LATE] prefix, got:\n%s", out)
		}
		if !strings.Contains(out, "PROJ-4: Update tests") {
			t.Errorf("missing task details, got:\n%s", out)
		}
	})

	t.Run("prints project prefix when set", func(t *testing.T) {
		var buf bytes.Buffer
		PrintTodayResult(&buf, &TodayResult{
			Events: []FyllaEvent{
				{
					TaskKey: "PROJ-1",
					Project: "PROJ",
					Summary: "Fix login bug",
					Start:   time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
					End:     time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
				},
			},
		}, now, true)

		out := buf.String()
		if !strings.Contains(out, "[PROJ] PROJ-1: Fix login bug") {
			t.Errorf("missing project prefix, got:\n%s", out)
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
		}, now, false)

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
