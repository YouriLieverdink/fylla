package task

import (
	"testing"
	"time"
)

func TestParseTitleEstimate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantDur  time.Duration
		wantText string
	}{
		{"hours only", "Fix bug [2h]", 2 * time.Hour, "Fix bug"},
		{"minutes only", "Fix bug [30m]", 30 * time.Minute, "Fix bug"},
		{"hours and minutes", "Fix bug [1h30m]", time.Hour + 30*time.Minute, "Fix bug"},
		{"no match", "Fix bug", 0, "Fix bug"},
		{"empty brackets", "Fix bug []", 0, "Fix bug []"},
		{"bracket at start", "[2h] Fix bug", 2 * time.Hour, "Fix bug"},
		{"bracket in middle", "Fix [2h] bug", 2 * time.Hour, "Fix bug"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dur, text := ParseTitleEstimate(tc.input)
			if dur != tc.wantDur {
				t.Errorf("duration = %v, want %v", dur, tc.wantDur)
			}
			if text != tc.wantText {
				t.Errorf("text = %q, want %q", text, tc.wantText)
			}
		})
	}
}

func TestParseTitleDueDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantDate *time.Time
		wantText string
	}{
		{
			"valid date",
			"Fix bug {2025-02-15}",
			timePtr(time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC)),
			"Fix bug",
		},
		{"no match", "Fix bug", nil, "Fix bug"},
		{
			"date at start",
			"{2025-06-01} Fix bug",
			timePtr(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
			"Fix bug",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			date, text := ParseTitleDueDate(tc.input)
			if tc.wantDate == nil && date != nil {
				t.Errorf("date = %v, want nil", date)
			}
			if tc.wantDate != nil {
				if date == nil {
					t.Fatalf("date = nil, want %v", *tc.wantDate)
				}
				if !date.Equal(*tc.wantDate) {
					t.Errorf("date = %v, want %v", *date, *tc.wantDate)
				}
			}
			if text != tc.wantText {
				t.Errorf("text = %q, want %q", text, tc.wantText)
			}
		})
	}
}

func TestSetTitleEstimate(t *testing.T) {
	tests := []struct {
		name string
		text string
		dur  time.Duration
		want string
	}{
		{"append to plain text", "Fix bug", 2 * time.Hour, "Fix bug [2h]"},
		{"replace existing", "Fix bug [1h]", 2 * time.Hour, "Fix bug [2h]"},
		{"minutes", "Fix bug", 30 * time.Minute, "Fix bug [30m]"},
		{"mixed", "Fix bug", time.Hour + 15*time.Minute, "Fix bug [1h15m]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SetTitleEstimate(tc.text, tc.dur)
			if got != tc.want {
				t.Errorf("SetTitleEstimate(%q, %v) = %q, want %q", tc.text, tc.dur, got, tc.want)
			}
		})
	}
}

func TestParseInput(t *testing.T) {
	// Use a fixed reference time for deterministic date assertions
	ref := time.Date(2025, 2, 12, 12, 0, 0, 0, time.UTC)

	// Helper to get the weekday date for the next occurrence of a weekday
	nextWeekday := func(wd time.Weekday) time.Time {
		d := ref
		for {
			d = d.AddDate(0, 0, 1)
			if d.Weekday() == wd {
				return d
			}
		}
	}

	tests := []struct {
		name         string
		input        string
		wantSummary  string
		wantEstimate time.Duration
		wantPriority string
		wantDueDay   *time.Weekday
		wantDesc     string
	}{
		{
			name:         "full syntax with scheduling hints left in title",
			input:        "Write the docs [30m] (due Friday not before Monday priority:critical upnext nosplit)",
			wantSummary:  "Write the docs not before Monday upnext nosplit",
			wantEstimate: 30 * time.Minute,
			wantPriority: "Highest",
			wantDueDay:   weekdayPtr(time.Friday),
		},
		{
			name:         "only estimate",
			input:        "Fix bug [2h]",
			wantSummary:  "Fix bug",
			wantEstimate: 2 * time.Hour,
		},
		{
			name:        "only due in parens",
			input:       "Fix bug (due Friday)",
			wantSummary: "Fix bug",
			wantDueDay:  weekdayPtr(time.Friday),
		},
		{
			name:         "only priority in parens",
			input:        "Fix bug (priority:p1)",
			wantSummary:  "Fix bug",
			wantPriority: "Highest",
		},
		{
			name:        "not before stays in title",
			input:       "Task (not before next Monday)",
			wantSummary: "Task not before next Monday",
		},
		{
			name:        "upnext and nosplit stay in title",
			input:       "Task (upnext nosplit)",
			wantSummary: "Task upnext nosplit",
		},
		{
			name:        "no attributes",
			input:       "Just a plain task",
			wantSummary: "Just a plain task",
		},
		{
			name:         "estimate and parens",
			input:        "Fix bug [1h] (due Friday priority:high)",
			wantSummary:  "Fix bug",
			wantEstimate: time.Hour,
			wantPriority: "High",
			wantDueDay:   weekdayPtr(time.Friday),
		},
		{
			name:         "case insensitive priority",
			input:        "Fix bug (priority:HIGH)",
			wantSummary:  "Fix bug",
			wantPriority: "High",
		},
		{
			name:         "priority aliases",
			input:        "Fix bug (priority:p2)",
			wantSummary:  "Fix bug",
			wantPriority: "High",
		},
		{
			name:        "due and not before together",
			input:       "Write report (not before Monday due Friday)",
			wantSummary: "Write report not before Monday",
			wantDueDay:  weekdayPtr(time.Friday),
		},
		{
			name:        "desc extracted from parens",
			input:       "Fix bug (due Friday desc:detailed description here)",
			wantSummary: "Fix bug",
			wantDueDay:  weekdayPtr(time.Friday),
			wantDesc:    "detailed description here",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseInput(tc.input, ref)

			if got.Summary != tc.wantSummary {
				t.Errorf("Summary = %q, want %q", got.Summary, tc.wantSummary)
			}
			if got.Estimate != tc.wantEstimate {
				t.Errorf("Estimate = %v, want %v", got.Estimate, tc.wantEstimate)
			}
			if got.Priority != tc.wantPriority {
				t.Errorf("Priority = %q, want %q", got.Priority, tc.wantPriority)
			}
			if got.Description != tc.wantDesc {
				t.Errorf("Description = %q, want %q", got.Description, tc.wantDesc)
			}

			if tc.wantDueDay != nil {
				if got.DueDate == nil {
					t.Fatalf("DueDate = nil, want %v", *tc.wantDueDay)
				}
				wantDate := nextWeekday(*tc.wantDueDay)
				if got.DueDate.Weekday() != wantDate.Weekday() {
					t.Errorf("DueDate weekday = %v, want %v", got.DueDate.Weekday(), wantDate.Weekday())
				}
			} else if got.DueDate != nil {
				t.Errorf("DueDate = %v, want nil", got.DueDate)
			}
		})
	}
}

func weekdayPtr(wd time.Weekday) *time.Weekday { return &wd }

func timePtr(t time.Time) *time.Time { return &t }
