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
	now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)

	t.Run("returns all Fylla events for today", func(t *testing.T) {
		cal := &mockCalendar{
			events: []calendar.Event{
				{
					Title: "[Fylla] PROJ-1: Write the docs",
					Start: time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC),
				},
				{
					Title: "[Fylla] PROJ-2: Fix login bug",
					Start: time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
				},
				{
					Title: "Team standup",
					Start: time.Date(2025, 1, 20, 9, 30, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 9, 45, 0, 0, time.UTC),
				},
				{
					Title: "[Fylla] PROJ-3: Review PR",
					Start: time.Date(2025, 1, 20, 13, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 14, 0, 0, 0, time.UTC),
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
		if result.Events[1].TaskKey != "PROJ-2" {
			t.Errorf("second event = %q, want PROJ-2", result.Events[1].TaskKey)
		}
		if result.Events[2].TaskKey != "PROJ-3" {
			t.Errorf("third event = %q, want PROJ-3", result.Events[2].TaskKey)
		}
	})

	t.Run("empty day", func(t *testing.T) {
		cal := &mockCalendar{events: []calendar.Event{}}

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

	t.Run("handles LATE prefix", func(t *testing.T) {
		cal := &mockCalendar{
			events: []calendar.Event{
				{
					Title: "[LATE] [Fylla] PROJ-4: Overdue task",
					Start: time.Date(2025, 1, 20, 14, 0, 0, 0, time.UTC),
					End:   time.Date(2025, 1, 20, 15, 30, 0, 0, time.UTC),
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
}
