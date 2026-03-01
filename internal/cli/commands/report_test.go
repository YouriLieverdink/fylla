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

type mockReportCal struct {
	events      []calendar.Event
	fyllaEvents []calendar.Event
}

func (m *mockReportCal) FetchEvents(_ context.Context, _, _ time.Time) ([]calendar.Event, error) {
	return m.events, nil
}
func (m *mockReportCal) FetchFyllaEvents(_ context.Context, _, _ time.Time) ([]calendar.Event, error) {
	return m.fyllaEvents, nil
}
func (m *mockReportCal) DeleteFyllaEvents(_ context.Context, _, _ time.Time) error { return nil }
func (m *mockReportCal) CreateEvent(_ context.Context, _ calendar.CreateEventInput) error {
	return nil
}
func (m *mockReportCal) UpdateEvent(_ context.Context, _ string, _ calendar.CreateEventInput) error {
	return nil
}
func (m *mockReportCal) DeleteEvent(_ context.Context, _ string) error { return nil }

func TestRunReport(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	cal := &mockReportCal{
		events: []calendar.Event{
			{
				ID:    "meeting1",
				Title: "Team standup",
				Start: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 3, 1, 9, 30, 0, 0, time.UTC),
			},
			{
				ID:    "fylla1",
				Title: "[PROJ] Some task",
				Start: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC),
			},
			{
				ID:    "fylla2",
				Title: calendar.DoneMarker + "[PROJ] Done task",
				Start: time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			},
		},
		fyllaEvents: []calendar.Event{
			{
				ID:    "fylla1",
				Title: "[PROJ] Some task",
				Start: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC),
			},
			{
				ID:    "fylla2",
				Title: calendar.DoneMarker + "[PROJ] Done task",
				Start: time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	result, err := RunReport(context.Background(), ReportParams{
		Cal:  cal,
		Cfg:  &config.Config{},
		Now:  now,
		Days: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.TasksDone != 1 {
		t.Errorf("tasks done = %d, want 1", result.TasksDone)
	}
	if result.TaskTime != 2*time.Hour {
		t.Errorf("task time = %v, want 2h", result.TaskTime)
	}
	if result.MeetingTime != 30*time.Minute {
		t.Errorf("meeting time = %v, want 30m", result.MeetingTime)
	}

	var buf bytes.Buffer
	PrintReportResult(&buf, result)
	output := buf.String()
	if !strings.Contains(output, "Tasks completed:  1") {
		t.Errorf("output missing tasks completed: %s", output)
	}
}
