package commands

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/timer"
)

// mockWorklogPoster records PostWorklog calls for assertion.
type mockWorklogPoster struct {
	calls []worklogCall
	err   error
}

type worklogCall struct {
	issueKey    string
	timeSpent   time.Duration
	description string
	started     time.Time
}

func (m *mockWorklogPoster) PostWorklog(_ context.Context, issueKey string, timeSpent time.Duration, description string, started time.Time) error {
	m.calls = append(m.calls, worklogCall{issueKey, timeSpent, description, started})
	return m.err
}

func TestCLI010_start_begins_timer(t *testing.T) {
	t.Run("start stores task key and returns confirmation", func(t *testing.T) {
		dir := t.TempDir()
		timerPath := filepath.Join(dir, "timer.json")
		now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

		state, err := RunStart(StartParams{
			TaskKey:   "PROJ-123",
			TimerPath: timerPath,
			Now:       now,
		})
		if err != nil {
			t.Fatalf("RunStart: %v", err)
		}

		if state.TaskKey != "PROJ-123" {
			t.Errorf("TaskKey = %q, want PROJ-123", state.TaskKey)
		}
		if !state.StartTime.Equal(now) {
			t.Errorf("StartTime = %v, want %v", state.StartTime, now)
		}
	})

	t.Run("confirmation message includes task key", func(t *testing.T) {
		dir := t.TempDir()
		timerPath := filepath.Join(dir, "timer.json")
		now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

		state, err := RunStart(StartParams{
			TaskKey:   "PROJ-123",
			TimerPath: timerPath,
			Now:       now,
		})
		if err != nil {
			t.Fatalf("RunStart: %v", err)
		}

		var buf bytes.Buffer
		PrintStartResult(&buf, state)
		out := buf.String()

		if !strings.Contains(out, "Started timer for PROJ-123") {
			t.Errorf("output = %q, want to contain 'Started timer for PROJ-123'", out)
		}
	})

	t.Run("timer state is persisted to disk", func(t *testing.T) {
		dir := t.TempDir()
		timerPath := filepath.Join(dir, "timer.json")
		now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

		_, err := RunStart(StartParams{
			TaskKey:   "PROJ-123",
			TimerPath: timerPath,
			Now:       now,
		})
		if err != nil {
			t.Fatalf("RunStart: %v", err)
		}

		// Verify state was persisted by loading it back
		loaded, err := timer.Load(timerPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if loaded == nil {
			t.Fatal("expected timer state to be persisted")
		}
		if loaded.TaskKey != "PROJ-123" {
			t.Errorf("persisted TaskKey = %q, want PROJ-123", loaded.TaskKey)
		}
	})
}

func TestCLI011_stop_ends_timer_and_logs(t *testing.T) {
	t.Run("stop calculates elapsed and posts worklog", func(t *testing.T) {
		dir := t.TempDir()
		timerPath := filepath.Join(dir, "timer.json")
		startTime := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
		stopTime := startTime.Add(1*time.Hour + 25*time.Minute)

		// Start timer first
		_, err := timer.Start("PROJ-123", "", "", "", startTime, timerPath)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}

		mock := &mockWorklogPoster{}
		result, err := RunStop(context.Background(), StopParams{
			TimerPath:    timerPath,
			RoundMinutes: 5,
			Now:          stopTime,
			Description:  "Fixed the auth bug",
			Jira:         mock,
		})
		if err != nil {
			t.Fatalf("RunStop: %v", err)
		}

		if result.TaskKey != "PROJ-123" {
			t.Errorf("TaskKey = %q, want PROJ-123", result.TaskKey)
		}
		// 1h25m rounded to nearest 5 minutes = 1h25m
		if result.Rounded != 1*time.Hour+25*time.Minute {
			t.Errorf("Rounded = %v, want 1h25m", result.Rounded)
		}
	})

	t.Run("worklog is posted to Jira", func(t *testing.T) {
		dir := t.TempDir()
		timerPath := filepath.Join(dir, "timer.json")
		startTime := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
		stopTime := startTime.Add(45 * time.Minute)

		_, err := timer.Start("PROJ-456", "", "", "", startTime, timerPath)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}

		mock := &mockWorklogPoster{}
		_, err = RunStop(context.Background(), StopParams{
			TimerPath:    timerPath,
			RoundMinutes: 5,
			Now:          stopTime,
			Description:  "Worked on feature",
			Jira:         mock,
		})
		if err != nil {
			t.Fatalf("RunStop: %v", err)
		}

		if len(mock.calls) != 1 {
			t.Fatalf("expected 1 worklog call, got %d", len(mock.calls))
		}
		call := mock.calls[0]
		if call.issueKey != "PROJ-456" {
			t.Errorf("issueKey = %q, want PROJ-456", call.issueKey)
		}
		if call.description != "Worked on feature" {
			t.Errorf("description = %q, want 'Worked on feature'", call.description)
		}
		if call.timeSpent != 45*time.Minute {
			t.Errorf("timeSpent = %v, want 45m", call.timeSpent)
		}
	})

	t.Run("output shows stopped time and worklog confirmation", func(t *testing.T) {
		result := &StopResult{
			TaskKey: "PROJ-123",
			Rounded: 1*time.Hour + 25*time.Minute,
		}
		var buf bytes.Buffer
		PrintStopResult(&buf, result)
		out := buf.String()

		if !strings.Contains(out, "Timer stopped") {
			t.Errorf("output missing 'Timer stopped', got:\n%s", out)
		}
		if !strings.Contains(out, "Worklog added to PROJ-123") {
			t.Errorf("output missing worklog confirmation, got:\n%s", out)
		}
	})
}

func TestCLI012_stop_description_flag(t *testing.T) {
	t.Run("inline description used in worklog", func(t *testing.T) {
		dir := t.TempDir()
		timerPath := filepath.Join(dir, "timer.json")
		startTime := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
		stopTime := startTime.Add(30 * time.Minute)

		_, err := timer.Start("PROJ-789", "", "", "", startTime, timerPath)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}

		mock := &mockWorklogPoster{}
		result, err := RunStop(context.Background(), StopParams{
			TimerPath:    timerPath,
			RoundMinutes: 5,
			Now:          stopTime,
			Description:  "Fixed the bug",
			Jira:         mock,
		})
		if err != nil {
			t.Fatalf("RunStop: %v", err)
		}

		if result.Description != "Fixed the bug" {
			t.Errorf("Description = %q, want 'Fixed the bug'", result.Description)
		}
		if len(mock.calls) != 1 {
			t.Fatalf("expected 1 worklog call, got %d", len(mock.calls))
		}
		if mock.calls[0].description != "Fixed the bug" {
			t.Errorf("worklog description = %q, want 'Fixed the bug'", mock.calls[0].description)
		}
	})

	t.Run("cobra command has description flag", func(t *testing.T) {
		root := newTestRootCmd()
		stopCmd, _, err := root.Find([]string{"timer", "stop"})
		if err != nil {
			t.Fatalf("find stop command: %v", err)
		}
		flag := stopCmd.Flags().Lookup("description")
		if flag == nil {
			t.Fatal("stop command missing --description flag")
		}
		if flag.Shorthand != "d" {
			t.Errorf("description shorthand = %q, want 'd'", flag.Shorthand)
		}
	})
}

func TestCLI013_status_shows_running_task(t *testing.T) {
	t.Run("status shows task key and elapsed time", func(t *testing.T) {
		dir := t.TempDir()
		timerPath := filepath.Join(dir, "timer.json")
		startTime := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
		checkTime := startTime.Add(1*time.Hour + 23*time.Minute)

		_, err := timer.Start("PROJ-123", "", "", "", startTime, timerPath)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}

		result, err := RunStatus(StatusParams{
			TimerPath: timerPath,
			Now:       checkTime,
		})
		if err != nil {
			t.Fatalf("RunStatus: %v", err)
		}

		if result == nil {
			t.Fatal("expected status result, got nil")
		}
		if result.TaskKey != "PROJ-123" {
			t.Errorf("TaskKey = %q, want PROJ-123", result.TaskKey)
		}
		expectedElapsed := 1*time.Hour + 23*time.Minute
		if result.Elapsed != expectedElapsed {
			t.Errorf("Elapsed = %v, want %v", result.Elapsed, expectedElapsed)
		}
	})

	t.Run("output displays task key and elapsed", func(t *testing.T) {
		result := &StatusResult{
			TaskKey: "PROJ-123",
			Elapsed: 1*time.Hour + 23*time.Minute,
		}
		var buf bytes.Buffer
		PrintStatusResult(&buf, result)
		out := buf.String()

		if !strings.Contains(out, "PROJ-123") {
			t.Errorf("output missing task key, got:\n%s", out)
		}
		if !strings.Contains(out, "Running for:") {
			t.Errorf("output missing 'Running for:', got:\n%s", out)
		}
		if !strings.Contains(out, "1h 23m") {
			t.Errorf("output missing elapsed time '1h 23m', got:\n%s", out)
		}
	})

	t.Run("no timer running returns nil", func(t *testing.T) {
		dir := t.TempDir()
		timerPath := filepath.Join(dir, "timer.json")

		result, err := RunStatus(StatusParams{
			TimerPath: timerPath,
			Now:       time.Now(),
		})
		if err != nil {
			t.Fatalf("RunStatus: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil result, got %+v", result)
		}
	})

	t.Run("no timer running output message", func(t *testing.T) {
		var buf bytes.Buffer
		PrintStatusResult(&buf, nil)
		out := buf.String()
		if !strings.Contains(out, "No timer running") {
			t.Errorf("output = %q, want 'No timer running'", out)
		}
	})
}

func TestCLI014_log_manual_worklog(t *testing.T) {
	t.Run("creates worklog in Jira", func(t *testing.T) {
		mock := &mockWorklogPoster{}
		err := RunLog(context.Background(), LogParams{
			TaskKey:     "PROJ-123",
			Duration:    2 * time.Hour,
			Description: "Worked on feature",
			Jira:        mock,
		})
		if err != nil {
			t.Fatalf("RunLog: %v", err)
		}

		if len(mock.calls) != 1 {
			t.Fatalf("expected 1 worklog call, got %d", len(mock.calls))
		}
		call := mock.calls[0]
		if call.issueKey != "PROJ-123" {
			t.Errorf("issueKey = %q, want PROJ-123", call.issueKey)
		}
		if call.timeSpent != 2*time.Hour {
			t.Errorf("timeSpent = %v, want 2h", call.timeSpent)
		}
		if call.description != "Worked on feature" {
			t.Errorf("description = %q, want 'Worked on feature'", call.description)
		}
	})

	t.Run("output shows confirmation", func(t *testing.T) {
		var buf bytes.Buffer
		PrintLogResult(&buf, "PROJ-123", 2*time.Hour)
		out := buf.String()

		if !strings.Contains(out, "Worklog added to PROJ-123") {
			t.Errorf("output missing confirmation, got:\n%s", out)
		}
		if !strings.Contains(out, "2h") {
			t.Errorf("output missing duration, got:\n%s", out)
		}
	})

	t.Run("ParseDuration handles various formats", func(t *testing.T) {
		tests := []struct {
			input string
			want  time.Duration
		}{
			{"2h", 2 * time.Hour},
			{"30m", 30 * time.Minute},
			{"1h30m", 1*time.Hour + 30*time.Minute},
			{"4h", 4 * time.Hour},
			{"15m", 15 * time.Minute},
		}
		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				got, err := ParseDuration(tt.input)
				if err != nil {
					t.Fatalf("ParseDuration(%q): %v", tt.input, err)
				}
				if got != tt.want {
					t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
				}
			})
		}
	})

	t.Run("ParseDuration rejects invalid input", func(t *testing.T) {
		invalid := []string{"", "abc", "2x", "h30", "2h 30m"}
		for _, s := range invalid {
			t.Run(s, func(t *testing.T) {
				_, err := ParseDuration(s)
				if err == nil {
					t.Errorf("ParseDuration(%q) should return error", s)
				}
			})
		}
	})

	t.Run("cobra command accepts three args", func(t *testing.T) {
		root := newTestRootCmd()
		logCmd, _, err := root.Find([]string{"timer", "log"})
		if err != nil {
			t.Fatalf("find log command: %v", err)
		}
		// Verify Args validator accepts exactly 3 args
		if err := logCmd.Args(logCmd, []string{"PROJ-123", "2h", "Worked on feature"}); err != nil {
			t.Fatalf("log command should accept 3 args: %v", err)
		}
	})

	t.Run("cobra command rejects wrong arg count", func(t *testing.T) {
		root := newTestRootCmd()
		_, err := executeCommand(root, "timer", "log", "PROJ-123", "2h")
		if err == nil {
			t.Fatal("expected error with 2 args instead of 3")
		}
	})
}
