package worklog

import (
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/tui/msg"
	"github.com/iruoy/fylla/internal/tui/styles"
)

func defaultBiz() []config.BusinessHoursConfig {
	return []config.BusinessHoursConfig{
		{Start: "09:00", End: "17:00", WorkDays: []int{1, 2, 3, 4, 5}},
	}
}

func newModel(t *testing.T, date time.Time, holidays []config.HolidayConfig) Model {
	t.Helper()
	idx, err := config.BuildHolidayIndex(holidays)
	if err != nil {
		t.Fatalf("build holiday index: %v", err)
	}
	m := New(8, 40, 0.7, []int{1, 2, 3, 4, 5}, defaultBiz(), idx, config.HolidayIndex{})
	m.Date = date
	return m
}

func TestDailyTarget(t *testing.T) {
	mon := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC) // Monday

	tests := []struct {
		name     string
		holidays []config.HolidayConfig
		want     time.Duration
	}{
		{"no holiday", nil, 8 * time.Hour},
		{"full day off", []config.HolidayConfig{{Date: "2026-05-04"}}, 0},
		{"4h afternoon", []config.HolidayConfig{{Date: "2026-05-04", Start: "13:00", End: "17:00"}}, 4 * time.Hour},
		{"two 1h blocks", []config.HolidayConfig{
			{Date: "2026-05-04", Start: "09:00", End: "10:00"},
			{Date: "2026-05-04", Start: "16:00", End: "17:00"},
		}, 6 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModel(t, mon, tt.holidays)
			if got := m.dailyTarget(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWeeklyTarget(t *testing.T) {
	wed := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC) // Wednesday in week of 2026-05-04..05-10

	tests := []struct {
		name     string
		holidays []config.HolidayConfig
		want     time.Duration
	}{
		{"clean week", nil, 40 * time.Hour},
		{"one full holiday", []config.HolidayConfig{{Date: "2026-05-04"}}, 32 * time.Hour},
		{"4h afternoon off", []config.HolidayConfig{{Date: "2026-05-05", Start: "13:00", End: "17:00"}}, 36 * time.Hour},
		{"two 1h blocks one day", []config.HolidayConfig{
			{Date: "2026-05-06", Start: "09:00", End: "10:00"},
			{Date: "2026-05-06", Start: "16:00", End: "17:00"},
		}, 38 * time.Hour},
		{"holiday on weekend ignored", []config.HolidayConfig{{Date: "2026-05-09"}}, 40 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModel(t, wed, tt.holidays)
			if got := m.weeklyTarget(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalmModeEntryLineHasNoTime(t *testing.T) {
	d := time.Date(2026, 5, 4, 14, 30, 0, 0, time.UTC)
	m := New(8, 40, 0.7, []int{1, 2, 3, 4, 5}, defaultBiz(), config.HolidayIndex{}, config.HolidayIndex{})
	m.Width = 120
	m.CalmMode = true
	e := msg.WorklogEntry{IssueKey: "PRJ-1", IssueSummary: "Fix login", Description: "review", Started: d, TimeSpent: 90 * time.Minute}

	line := m.renderCalmEntry(e, false)

	if strings.Contains(line, "14:30") || strings.Contains(line, "1h") || strings.Contains(line, "16:00") {
		t.Errorf("calm entry line leaked a time/duration: %q", line)
	}
	for _, want := range []string{"PRJ-1", "review"} {
		if !strings.Contains(line, want) {
			t.Errorf("calm entry line missing %q: %q", want, line)
		}
	}
	if strings.Contains(line, "Fix login") {
		t.Errorf("calm entry line should show key + note only, not the summary: %q", line)
	}
}

func TestCalmModeEntryLineWrapsWithinWidth(t *testing.T) {
	m := New(8, 40, 0.7, []int{1, 2, 3, 4, 5}, defaultBiz(), config.HolidayIndex{}, config.HolidayIndex{})
	m.Width = 60
	m.CalmMode = true
	note := strings.Repeat("woord ", 40) // long enough to wrap several times
	e := msg.WorklogEntry{IssueKey: "PRJ-1", Description: note}

	out := m.renderCalmEntry(e, false)
	rows := strings.Split(out, "\n")
	if len(rows) < 2 {
		t.Fatalf("expected the long note to wrap onto multiple lines, got %d: %q", len(rows), out)
	}
	for i, r := range rows {
		// renderCalmEntry already includes the 2-col cursor on the first line.
		if w := styles.StringWidth(r); w > m.Width {
			t.Errorf("wrapped line %d width %d exceeds pane width %d: %q", i, w, m.Width, r)
		}
	}
}

func TestWrapText(t *testing.T) {
	got := wrapText("alpha beta gamma", 11)
	want := []string{"alpha beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
	// Over-long word hard-breaks.
	if hb := wrapText("xxxxxxxx", 3); len(hb) != 3 || hb[0] != "xxx" {
		t.Errorf("hard-break = %v, want 3 lines starting xxx", hb)
	}
}

func TestCalmModeSelectionIsPerEntry(t *testing.T) {
	d := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	m := New(8, 40, 0.7, []int{1, 2, 3, 4, 5}, defaultBiz(), config.HolidayIndex{}, config.HolidayIndex{})
	m.Loading = false
	m.CalmMode = true
	m.Entries = []msg.WorklogEntry{
		{ID: "1", IssueKey: "PRJ-1", Description: "a", Started: d, TimeSpent: time.Hour},
		{ID: "2", IssueKey: "PRJ-1", Description: "b", Started: d, TimeSpent: time.Hour},
		{ID: "3", IssueKey: "PRJ-2", Description: "c", Started: d, TimeSpent: time.Hour},
	}

	// Same-task entries stay separate rows; selection maps 1:1 to entries.
	m.Cursor = 0
	m.CursorDown()
	if e := m.SelectedEntry(); e == nil || e.ID != "2" {
		t.Fatalf("SelectedEntry = %+v, want entry ID 2", e)
	}
	m.CursorDown()
	if e := m.SelectedEntry(); e == nil || e.ID != "3" {
		t.Fatalf("SelectedEntry = %+v, want entry ID 3", e)
	}
}
