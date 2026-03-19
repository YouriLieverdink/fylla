package commands

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/timer"
)

func worklogConfig() *config.Config {
	cfg := testConfig()
	cfg.Worklog = config.WorklogConfig{
		FallbackIssues: []string{"ADMIN-1", "MEETING-1"},
	}
	return cfg
}

type mockSurveyor struct {
	selectAnswers           []string
	multiSelectAnswers      [][]string
	inputAnswers            []string
	inputWithDefaultAnswers []string
	passwordAnswer          []string
	selectIdx               int
	multiSelectIdx          int
	inputIdx                int
	inputWithDefaultIdx     int
	passwordIdx             int
}

func (m *mockSurveyor) Select(message string, options []string) (string, error) {
	if m.selectIdx >= len(m.selectAnswers) {
		return "", fmt.Errorf("unexpected Select call: %s", message)
	}
	answer := m.selectAnswers[m.selectIdx]
	m.selectIdx++
	return answer, nil
}

func (m *mockSurveyor) MultiSelect(message string, options []string) ([]string, error) {
	if m.multiSelectIdx >= len(m.multiSelectAnswers) {
		return nil, fmt.Errorf("unexpected MultiSelect call: %s", message)
	}
	answer := m.multiSelectAnswers[m.multiSelectIdx]
	m.multiSelectIdx++
	return answer, nil
}

func (m *mockSurveyor) Input(message string) (string, error) {
	if m.inputIdx >= len(m.inputAnswers) {
		return "", fmt.Errorf("unexpected Input call: %s", message)
	}
	answer := m.inputAnswers[m.inputIdx]
	m.inputIdx++
	return answer, nil
}

func (m *mockSurveyor) InputWithDefault(message, defaultVal string) (string, error) {
	if m.inputWithDefaultIdx >= len(m.inputWithDefaultAnswers) {
		return defaultVal, nil
	}
	answer := m.inputWithDefaultAnswers[m.inputWithDefaultIdx]
	m.inputWithDefaultIdx++
	return answer, nil
}

func (m *mockSurveyor) Password(message string) (string, error) {
	if m.passwordIdx >= len(m.passwordAnswer) {
		return "", fmt.Errorf("unexpected Password call: %s", message)
	}
	answer := m.passwordAnswer[m.passwordIdx]
	m.passwordIdx++
	return answer, nil
}

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

// mockJiraKeyResolver returns a predefined Jira key for testing.
type mockJiraKeyResolver struct {
	key string
	err error
}

func (m *mockJiraKeyResolver) ResolveJiraKey(_ context.Context, _ string) (string, error) {
	return m.key, m.err
}

func TestStop_CalendarEventUpdated(t *testing.T) {
	now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
	startTime := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

	timerPath := filepath.Join(t.TempDir(), "timer.json")
	_, err := timer.Start("PROJ-1", "", "", "", startTime, timerPath)
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
	_, err := timer.Start("PROJ-2", "", "", "", startTime, timerPath)
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
		_, err := timer.Start("PROJ-3", "", "", "", startTime, timerPath)
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
		_, err := timer.Start("PROJ-4", "", "", "", startTime, timerPath)
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

func TestStop_GitHubKeyResolvesToJira(t *testing.T) {
	t.Run("resolved key used for worklog", func(t *testing.T) {
		now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
		startTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)

		timerPath := filepath.Join(t.TempDir(), "timer.json")
		_, err := timer.Start("fylla#42", "", "", "", startTime, timerPath)
		if err != nil {
			t.Fatalf("timer.Start: %v", err)
		}

		jira := &mockWorklogPoster{}
		survey := &mockSurveyor{
			inputWithDefaultAnswers: []string{"PROJ-123"},
		}

		result, err := RunStop(context.Background(), StopParams{
			TimerPath:    timerPath,
			RoundMinutes: 5,
			Now:          now,
			Description:  "PR review",
			Jira:         jira,
			Cfg:          worklogConfig(),
			Resolver:     &mockJiraKeyResolver{key: "PROJ-123"},
			Survey:       survey,
		})
		if err != nil {
			t.Fatalf("RunStop: %v", err)
		}

		if result.TaskKey != "PROJ-123" {
			t.Errorf("TaskKey = %q, want PROJ-123", result.TaskKey)
		}
		if len(jira.calls) != 1 {
			t.Fatalf("expected 1 worklog call, got %d", len(jira.calls))
		}
		if jira.calls[0].issueKey != "PROJ-123" {
			t.Errorf("worklog issueKey = %q, want PROJ-123", jira.calls[0].issueKey)
		}
	})

	t.Run("fallback prompt when no key found", func(t *testing.T) {
		now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
		startTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)

		timerPath := filepath.Join(t.TempDir(), "timer.json")
		_, err := timer.Start("fylla#99", "", "", "", startTime, timerPath)
		if err != nil {
			t.Fatalf("timer.Start: %v", err)
		}

		jira := &mockWorklogPoster{}
		survey := &mockSurveyor{
			selectAnswers: []string{"ADMIN-1"},
		}

		result, err := RunStop(context.Background(), StopParams{
			TimerPath:    timerPath,
			RoundMinutes: 5,
			Now:          now,
			Description:  "PR review",
			Jira:         jira,
			Cfg:          worklogConfig(),
			Resolver:     &mockJiraKeyResolver{key: ""},
			Survey:       survey,
		})
		if err != nil {
			t.Fatalf("RunStop: %v", err)
		}

		if result.TaskKey != "ADMIN-1" {
			t.Errorf("TaskKey = %q, want ADMIN-1", result.TaskKey)
		}
	})

	t.Run("resolver error falls back to prompt", func(t *testing.T) {
		now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
		startTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)

		timerPath := filepath.Join(t.TempDir(), "timer.json")
		_, err := timer.Start("fylla#99", "", "", "", startTime, timerPath)
		if err != nil {
			t.Fatalf("timer.Start: %v", err)
		}

		jira := &mockWorklogPoster{}
		survey := &mockSurveyor{
			selectAnswers: []string{"MEETING-1"},
		}

		result, err := RunStop(context.Background(), StopParams{
			TimerPath:    timerPath,
			RoundMinutes: 5,
			Now:          now,
			Description:  "PR review",
			Jira:         jira,
			Cfg:          worklogConfig(),
			Resolver:     &mockJiraKeyResolver{err: fmt.Errorf("API error")},
			Survey:       survey,
		})
		if err != nil {
			t.Fatalf("RunStop: %v", err)
		}

		if result.TaskKey != "MEETING-1" {
			t.Errorf("TaskKey = %q, want MEETING-1", result.TaskKey)
		}
	})

	t.Run("no resolver falls back to prompt for GitHub key", func(t *testing.T) {
		now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
		startTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)

		timerPath := filepath.Join(t.TempDir(), "timer.json")
		_, err := timer.Start("fylla#42", "", "", "", startTime, timerPath)
		if err != nil {
			t.Fatalf("timer.Start: %v", err)
		}

		jira := &mockWorklogPoster{}
		survey := &mockSurveyor{
			selectAnswers: []string{"ADMIN-1"},
		}

		// Without resolver, should show fallback prompt instead of passing raw key
		result, err := RunStop(context.Background(), StopParams{
			TimerPath:    timerPath,
			RoundMinutes: 5,
			Now:          now,
			Description:  "PR review",
			Jira:         jira,
			Cfg:          worklogConfig(),
			Survey:       survey,
		})
		if err != nil {
			t.Fatalf("RunStop: %v", err)
		}

		if result.TaskKey != "ADMIN-1" {
			t.Errorf("TaskKey = %q, want ADMIN-1", result.TaskKey)
		}
		if len(jira.calls) != 1 {
			t.Fatalf("expected 1 worklog call, got %d", len(jira.calls))
		}
		if jira.calls[0].issueKey != "ADMIN-1" {
			t.Errorf("worklog posted to %q, want ADMIN-1", jira.calls[0].issueKey)
		}
	})
}

func TestStop_TodoistKeyResolvesToJiraFallback(t *testing.T) {
	now := time.Date(2025, 1, 20, 10, 30, 0, 0, time.UTC)
	startTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)

	timerPath := filepath.Join(t.TempDir(), "timer.json")
	_, err := timer.Start("12345", "", "", "", startTime, timerPath)
	if err != nil {
		t.Fatalf("timer.Start: %v", err)
	}

	jira := &mockWorklogPoster{}
	cfg := worklogConfig()
	cfg.Worklog.Provider = "jira"

	survey := &mockSurveyor{
		selectAnswers: []string{"ADMIN-1"},
	}

	result, err := RunStop(context.Background(), StopParams{
		TimerPath:    timerPath,
		RoundMinutes: 5,
		Now:          now,
		Description:  "Todoist task work",
		Jira:         jira,
		Cfg:          cfg,
		Survey:       survey,
	})
	if err != nil {
		t.Fatalf("RunStop: %v", err)
	}

	if result.TaskKey != "ADMIN-1" {
		t.Errorf("TaskKey = %q, want ADMIN-1", result.TaskKey)
	}
	if len(jira.calls) != 1 {
		t.Fatalf("expected 1 worklog call, got %d", len(jira.calls))
	}
	if jira.calls[0].issueKey != "ADMIN-1" {
		t.Errorf("worklog posted to %q, want ADMIN-1", jira.calls[0].issueKey)
	}
}
