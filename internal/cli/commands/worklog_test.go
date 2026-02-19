package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
)

func worklogConfig() *config.Config {
	cfg := testConfig()
	cfg.Worklog = config.WorklogConfig{
		FallbackIssues: []string{"ADMIN-1", "MEETING-1"},
	}
	return cfg
}

func TestWorklog_EmptyCalendarFullFallback(t *testing.T) {
	// Monday with no events — should prompt for full target as fallback
	date := time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC) // Monday

	cal := &mockCalendar{
		fyllaEvents: []calendar.Event{},
		events:      []calendar.Event{},
	}
	jira := &mockWorklogPoster{}
	survey := &mockSurveyor{
		// Fallback: select issue, input duration, confirm
		selectAnswers:           []string{"ADMIN-1", "Yes"},
		inputWithDefaultAnswers: []string{"8h"},
	}

	var buf bytes.Buffer
	result, err := RunWorklog(context.Background(), WorklogParams{
		Cal:    cal,
		Jira:   jira,
		Cfg:    worklogConfig(),
		Survey: survey,
		Date:   date,
		W:      &buf,
	})
	if err != nil {
		t.Fatalf("RunWorklog: %v", err)
	}

	if result.Target != 8*time.Hour {
		t.Errorf("Target = %v, want 8h", result.Target)
	}
	if result.Posted != 1 {
		t.Errorf("Posted = %d, want 1", result.Posted)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("Entries = %d, want 1", len(result.Entries))
	}
	if result.Entries[0].TaskKey != "ADMIN-1" {
		t.Errorf("TaskKey = %q, want ADMIN-1", result.Entries[0].TaskKey)
	}
}

func TestWorklog_EventsFillTarget(t *testing.T) {
	// Monday with fylla events covering the full 8h target
	date := time.Date(2025, 1, 20, 17, 0, 0, 0, time.UTC) // Monday

	cal := &mockCalendar{
		fyllaEvents: []calendar.Event{
			{
				ID:          "f1",
				Title:       "[TEST] Task 1",
				Description: "fylla: TEST-1\nhttps://test.atlassian.net/browse/TEST-1",
				Start:       time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
				End:         time.Date(2025, 1, 20, 13, 0, 0, 0, time.UTC),
			},
			{
				ID:          "f2",
				Title:       "[TEST] Task 2",
				Description: "fylla: TEST-2\nhttps://test.atlassian.net/browse/TEST-2",
				Start:       time.Date(2025, 1, 20, 13, 0, 0, 0, time.UTC),
				End:         time.Date(2025, 1, 20, 17, 0, 0, 0, time.UTC),
			},
		},
		events: []calendar.Event{},
	}
	jira := &mockWorklogPoster{}
	survey := &mockSurveyor{
		// Accept default durations for both events, then confirm
		inputWithDefaultAnswers: []string{"4h", "4h"},
		selectAnswers:           []string{"Yes"},
	}

	var buf bytes.Buffer
	result, err := RunWorklog(context.Background(), WorklogParams{
		Cal:    cal,
		Jira:   jira,
		Cfg:    worklogConfig(),
		Survey: survey,
		Date:   date,
		W:      &buf,
	})
	if err != nil {
		t.Fatalf("RunWorklog: %v", err)
	}

	// Should not prompt for fallback since target is met
	if result.Posted != 2 {
		t.Errorf("Posted = %d, want 2", result.Posted)
	}
	out := buf.String()
	if strings.Contains(out, "Remaining:") {
		t.Errorf("should not show Remaining when target is met, got:\n%s", out)
	}
}

func TestWorklog_MeetingsPromptForIssueKey(t *testing.T) {
	date := time.Date(2025, 1, 20, 17, 0, 0, 0, time.UTC) // Monday

	cal := &mockCalendar{
		fyllaEvents: []calendar.Event{},
		events: []calendar.Event{
			{
				Title: "Sprint planning",
				Start: time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	jira := &mockWorklogPoster{}
	// Meeting: accept duration, select issue MEETING-1
	// Then fallback for remaining 7h: select ADMIN-1, accept duration
	// Then confirm
	survey := &mockSurveyor{
		inputWithDefaultAnswers: []string{"1h", "7h"},
		selectAnswers:           []string{"MEETING-1", "ADMIN-1", "Yes"},
	}

	var buf bytes.Buffer
	result, err := RunWorklog(context.Background(), WorklogParams{
		Cal:    cal,
		Jira:   jira,
		Cfg:    worklogConfig(),
		Survey: survey,
		Date:   date,
		W:      &buf,
	})
	if err != nil {
		t.Fatalf("RunWorklog: %v", err)
	}

	if result.Posted != 2 {
		t.Errorf("Posted = %d, want 2", result.Posted)
	}
	// First entry should be the meeting
	if len(result.Entries) < 1 || result.Entries[0].TaskKey != "MEETING-1" {
		t.Errorf("first entry key = %q, want MEETING-1", result.Entries[0].TaskKey)
	}
}

func TestWorklog_OverLoggedWarning(t *testing.T) {
	date := time.Date(2025, 1, 20, 17, 0, 0, 0, time.UTC) // Monday

	cal := &mockCalendar{
		fyllaEvents: []calendar.Event{
			{
				ID:          "f1",
				Title:       "[TEST] Big task",
				Description: "fylla: TEST-1\nhttps://test.atlassian.net/browse/TEST-1",
				Start:       time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
				End:         time.Date(2025, 1, 20, 17, 0, 0, 0, time.UTC),
			},
		},
		events: []calendar.Event{
			{
				Title: "Extra meeting",
				Start: time.Date(2025, 1, 20, 17, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 1, 20, 18, 0, 0, 0, time.UTC),
			},
		},
	}
	jira := &mockWorklogPoster{}
	survey := &mockSurveyor{
		inputWithDefaultAnswers: []string{"8h", "1h"},
		selectAnswers:           []string{"ADMIN-1", "Yes"},
	}

	var buf bytes.Buffer
	_, err := RunWorklog(context.Background(), WorklogParams{
		Cal:    cal,
		Jira:   jira,
		Cfg:    worklogConfig(),
		Survey: survey,
		Date:   date,
		W:      &buf,
	})
	if err != nil {
		t.Fatalf("RunWorklog: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Warning") || !strings.Contains(out, "exceeds") {
		t.Errorf("expected over-logged warning, got:\n%s", out)
	}
}

func TestWorklog_NonWorkDay(t *testing.T) {
	// Saturday — target should be 0
	date := time.Date(2025, 1, 25, 12, 0, 0, 0, time.UTC) // Saturday

	cal := &mockCalendar{
		fyllaEvents: []calendar.Event{},
		events:      []calendar.Event{},
	}
	jira := &mockWorklogPoster{}
	survey := &mockSurveyor{}

	var buf bytes.Buffer
	result, err := RunWorklog(context.Background(), WorklogParams{
		Cal:    cal,
		Jira:   jira,
		Cfg:    worklogConfig(),
		Survey: survey,
		Date:   date,
		W:      &buf,
	})
	if err != nil {
		t.Fatalf("RunWorklog: %v", err)
	}

	if result.Target != 0 {
		t.Errorf("Target = %v, want 0 for Saturday", result.Target)
	}
	out := buf.String()
	if !strings.Contains(out, "Non-work day") {
		t.Errorf("expected non-work day message, got:\n%s", out)
	}
}

func TestWorklog_UserCancels(t *testing.T) {
	date := time.Date(2025, 1, 20, 17, 0, 0, 0, time.UTC) // Monday

	cal := &mockCalendar{
		fyllaEvents: []calendar.Event{
			{
				ID:          "f1",
				Title:       "[TEST] Task 1",
				Description: "fylla: TEST-1\nhttps://test.atlassian.net/browse/TEST-1",
				Start:       time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
				End:         time.Date(2025, 1, 20, 17, 0, 0, 0, time.UTC),
			},
		},
		events: []calendar.Event{},
	}
	jira := &mockWorklogPoster{}
	survey := &mockSurveyor{
		inputWithDefaultAnswers: []string{"8h"},
		selectAnswers:           []string{"No"}, // User cancels
	}

	var buf bytes.Buffer
	result, err := RunWorklog(context.Background(), WorklogParams{
		Cal:    cal,
		Jira:   jira,
		Cfg:    worklogConfig(),
		Survey: survey,
		Date:   date,
		W:      &buf,
	})
	if err != nil {
		t.Fatalf("RunWorklog: %v", err)
	}

	if result.Posted != 0 {
		t.Errorf("Posted = %d, want 0 (user cancelled)", result.Posted)
	}
	if len(jira.calls) != 0 {
		t.Errorf("expected 0 worklog calls, got %d", len(jira.calls))
	}
}
