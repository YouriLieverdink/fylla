package commands

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/timer"
)

func TestStop_CalendarEventUpdated(t *testing.T) {
	now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
	startTime := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

	timerPath := filepath.Join(t.TempDir(), "timer.json")
	_, err := timer.Start("PROJ-1", startTime, timerPath)
	if err != nil {
		t.Fatalf("timer.Start: %v", err)
	}

	cal := &mockCalendar{
		fyllaEvents: []calendar.Event{
			{
				ID:          "evt-1",
				Title:       "[PROJ] Fix bug",
				Description: "fylla: PROJ-1\nhttps://test.atlassian.net/browse/PROJ-1",
				Start:       time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
				End:         time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	jira := &mockWorklogPoster{}

	result, err := RunStop(context.Background(), StopParams{
		TimerPath:    timerPath,
		RoundMinutes: 5,
		Now:          now,
		Description:  "Fixed login issue",
		Jira:         jira,
		Cal:          cal,
		Cfg:          testConfig(),
	})
	if err != nil {
		t.Fatalf("RunStop: %v", err)
	}

	if !result.CalendarUpdated {
		t.Error("expected CalendarUpdated to be true")
	}
	if len(cal.updated) != 1 {
		t.Fatalf("expected 1 calendar update, got %d", len(cal.updated))
	}
	if cal.updated[0].input.Done != true {
		t.Error("expected updated event to have Done=true")
	}
	if result.TaskKey != "PROJ-1" {
		t.Errorf("TaskKey = %q, want PROJ-1", result.TaskKey)
	}
}

func TestStop_NoCalendarGracefullySkipped(t *testing.T) {
	now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
	startTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)

	timerPath := filepath.Join(t.TempDir(), "timer.json")
	_, err := timer.Start("PROJ-2", startTime, timerPath)
	if err != nil {
		t.Fatalf("timer.Start: %v", err)
	}

	jira := &mockWorklogPoster{}

	result, err := RunStop(context.Background(), StopParams{
		TimerPath:    timerPath,
		RoundMinutes: 5,
		Now:          now,
		Description:  "Work done",
		Jira:         jira,
		Cal:          nil, // No calendar
		Cfg:          testConfig(),
	})
	if err != nil {
		t.Fatalf("RunStop: %v", err)
	}

	if result.CalendarUpdated {
		t.Error("expected CalendarUpdated to be false when Cal is nil")
	}
	if result.TaskKey != "PROJ-2" {
		t.Errorf("TaskKey = %q, want PROJ-2", result.TaskKey)
	}
	if len(jira.calls) != 1 {
		t.Fatalf("expected 1 worklog call, got %d", len(jira.calls))
	}
}

func TestStop_RemainingEstimateMessages(t *testing.T) {
	t.Run("remaining > 0 message printed", func(t *testing.T) {
		now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
		startTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)

		timerPath := filepath.Join(t.TempDir(), "timer.json")
		_, err := timer.Start("PROJ-3", startTime, timerPath)
		if err != nil {
			t.Fatalf("timer.Start: %v", err)
		}

		jira := &mockWorklogPoster{}
		estimate := &mockEstimateGetter{estimate: 2 * time.Hour}

		result, err := RunStop(context.Background(), StopParams{
			TimerPath:    timerPath,
			RoundMinutes: 5,
			Now:          now,
			Description:  "Work",
			Jira:         jira,
			Estimate:     estimate,
			Cfg:          testConfig(),
		})
		if err != nil {
			t.Fatalf("RunStop: %v", err)
		}

		if !result.HasRemaining {
			t.Error("expected HasRemaining to be true")
		}
		if result.RemainingEstimate != 2*time.Hour {
			t.Errorf("RemainingEstimate = %v, want 2h", result.RemainingEstimate)
		}

		var buf bytes.Buffer
		PrintStopResult(&buf, result)
		out := buf.String()
		if !bytes.Contains([]byte(out), []byte("remaining")) {
			t.Errorf("output should mention remaining, got:\n%s", out)
		}
		if !bytes.Contains([]byte(out), []byte("rescheduled")) {
			t.Errorf("output should mention rescheduled, got:\n%s", out)
		}
	})

	t.Run("remaining == 0 warning printed", func(t *testing.T) {
		now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
		startTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)

		timerPath := filepath.Join(t.TempDir(), "timer.json")
		_, err := timer.Start("PROJ-4", startTime, timerPath)
		if err != nil {
			t.Fatalf("timer.Start: %v", err)
		}

		jira := &mockWorklogPoster{}
		estimate := &mockEstimateGetter{estimate: 0}

		result, err := RunStop(context.Background(), StopParams{
			TimerPath:    timerPath,
			RoundMinutes: 5,
			Now:          now,
			Description:  "Work",
			Jira:         jira,
			Estimate:     estimate,
			Cfg:          testConfig(),
		})
		if err != nil {
			t.Fatalf("RunStop: %v", err)
		}

		if !result.HasRemaining {
			t.Error("expected HasRemaining to be true")
		}
		if result.RemainingEstimate != 0 {
			t.Errorf("RemainingEstimate = %v, want 0", result.RemainingEstimate)
		}

		var buf bytes.Buffer
		PrintStopResult(&buf, result)
		out := buf.String()
		if !bytes.Contains([]byte(out), []byte("Warning")) {
			t.Errorf("output should contain Warning, got:\n%s", out)
		}
		if !bytes.Contains([]byte(out), []byte("no time remaining")) {
			t.Errorf("output should mention no time remaining, got:\n%s", out)
		}
	})
}
