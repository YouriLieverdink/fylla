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
		name          string
		input         string
		wantSummary   string
		wantEstimate  time.Duration
		wantPriority  string
		wantDueDay    *time.Weekday
		wantNotBefore *time.Weekday
		wantUpNext    bool
		wantNoSplit   bool
	}{
		{
			name:          "full syntax with scheduling constraints extracted",
			input:         "Write the docs [30m] (due Friday not before Monday priority:p1 upnext nosplit)",
			wantSummary:   "Write the docs",
			wantEstimate:  30 * time.Minute,
			wantPriority:  "Highest",
			wantDueDay:    weekdayPtr(time.Friday),
			wantNotBefore: weekdayPtr(time.Monday),
			wantUpNext:    true,
			wantNoSplit:   true,
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
			name:         "priority p1",
			input:        "Fix bug (priority:p1)",
			wantSummary:  "Fix bug",
			wantPriority: "Highest",
		},
		{
			name:         "priority p2",
			input:        "Fix bug (priority:p2)",
			wantSummary:  "Fix bug",
			wantPriority: "High",
		},
		{
			name:        "unknown priority alias ignored",
			input:       "Fix bug (priority:critical)",
			wantSummary: "Fix bug",
		},
		{
			name:          "not before extracted",
			input:         "Task (not before next Monday)",
			wantSummary:   "Task",
			wantNotBefore: weekdayPtr(time.Monday),
		},
		{
			name:        "upnext and nosplit extracted",
			input:       "Task (upnext nosplit)",
			wantSummary: "Task",
			wantUpNext:  true,
			wantNoSplit: true,
		},
		{
			name:        "no attributes",
			input:       "Just a plain task",
			wantSummary: "Just a plain task",
		},
		{
			name:         "estimate and due with priority",
			input:        "Fix bug [1h] (due Friday priority:p2)",
			wantSummary:  "Fix bug",
			wantEstimate: time.Hour,
			wantPriority: "High",
			wantDueDay:   weekdayPtr(time.Friday),
		},
		{
			name:          "due and not before together",
			input:         "Write report (not before Monday due Friday)",
			wantSummary:   "Write report",
			wantDueDay:    weekdayPtr(time.Friday),
			wantNotBefore: weekdayPtr(time.Monday),
		},
		{
			name:        "upnext only",
			input:       "Deploy (upnext)",
			wantSummary: "Deploy",
			wantUpNext:  true,
		},
		{
			name:        "nosplit only",
			input:       "Deep work (nosplit)",
			wantSummary: "Deep work",
			wantNoSplit: true,
		},
		{
			name:          "not before with ISO date",
			input:         "Task (not before 2025-03-01)",
			wantSummary:   "Task",
			wantNotBefore: nil, // checked separately below
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
			if got.UpNext != tc.wantUpNext {
				t.Errorf("UpNext = %v, want %v", got.UpNext, tc.wantUpNext)
			}
			if got.NoSplit != tc.wantNoSplit {
				t.Errorf("NoSplit = %v, want %v", got.NoSplit, tc.wantNoSplit)
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

			if tc.wantNotBefore != nil {
				if got.NotBefore == nil {
					t.Fatalf("NotBefore = nil, want %v", *tc.wantNotBefore)
				}
				wantDate := nextWeekday(*tc.wantNotBefore)
				if got.NotBefore.Weekday() != wantDate.Weekday() {
					t.Errorf("NotBefore weekday = %v, want %v", got.NotBefore.Weekday(), wantDate.Weekday())
				}
			}
		})
	}
}

func TestParseInput_not_before_iso(t *testing.T) {
	ref := time.Date(2025, 2, 12, 12, 0, 0, 0, time.UTC)
	got := ParseInput("Task (not before 2025-03-01)", ref)

	if got.Summary != "Task" {
		t.Errorf("Summary = %q, want %q", got.Summary, "Task")
	}
	if got.NotBefore == nil {
		t.Fatal("NotBefore = nil, want 2025-03-01")
	}
	want := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	if !got.NotBefore.Equal(want) {
		t.Errorf("NotBefore = %v, want %v", *got.NotBefore, want)
	}
}

func TestExtractConstraints(t *testing.T) {
	ref := time.Date(2025, 2, 12, 12, 0, 0, 0, time.UTC)

	t.Run("all constraints", func(t *testing.T) {
		cleaned, notBefore, upNext, noSplit := ExtractConstraints(
			"Write docs not before 2025-03-01 upnext nosplit", ref,
		)
		if cleaned != "Write docs" {
			t.Errorf("cleaned = %q, want %q", cleaned, "Write docs")
		}
		if notBefore == nil {
			t.Fatal("notBefore = nil")
		}
		want := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
		if !notBefore.Equal(want) {
			t.Errorf("notBefore = %v, want %v", *notBefore, want)
		}
		if !upNext {
			t.Error("upNext = false, want true")
		}
		if !noSplit {
			t.Error("noSplit = false, want true")
		}
	})

	t.Run("parenthesized not before", func(t *testing.T) {
		cleaned, notBefore, upNext, noSplit := ExtractConstraints(
			"Write docs (not before 2025-03-01)", ref,
		)
		if cleaned != "Write docs" {
			t.Errorf("cleaned = %q, want %q", cleaned, "Write docs")
		}
		if notBefore == nil {
			t.Fatal("notBefore = nil")
		}
		want := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
		if !notBefore.Equal(want) {
			t.Errorf("notBefore = %v, want %v", *notBefore, want)
		}
		if upNext {
			t.Error("upNext = true, want false")
		}
		if noSplit {
			t.Error("noSplit = true, want false")
		}
	})

	t.Run("no constraints", func(t *testing.T) {
		cleaned, notBefore, upNext, noSplit := ExtractConstraints("Plain task", ref)
		if cleaned != "Plain task" {
			t.Errorf("cleaned = %q, want %q", cleaned, "Plain task")
		}
		if notBefore != nil {
			t.Errorf("notBefore = %v, want nil", notBefore)
		}
		if upNext {
			t.Error("upNext = true, want false")
		}
		if noSplit {
			t.Error("noSplit = true, want false")
		}
	})
}

func TestParseInput_not_before_iso_in_parens(t *testing.T) {
	ref := time.Date(2025, 2, 12, 12, 0, 0, 0, time.UTC)
	got := ParseInput("Task [5m] (not before 2025-03-01)", ref)

	if got.Summary != "Task" {
		t.Errorf("Summary = %q, want %q", got.Summary, "Task")
	}
	if got.Estimate != 5*time.Minute {
		t.Errorf("Estimate = %v, want %v", got.Estimate, 5*time.Minute)
	}
	if got.NotBefore == nil {
		t.Fatal("NotBefore = nil, want 2025-03-01")
	}
	want := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	if !got.NotBefore.Equal(want) {
		t.Errorf("NotBefore = %v, want %v", *got.NotBefore, want)
	}
}

func TestParseInput_due_in_parens(t *testing.T) {
	ref := time.Date(2025, 2, 12, 12, 0, 0, 0, time.UTC)
	got := ParseInput("Write docs (due 2025-03-01)", ref)

	if got.Summary != "Write docs" {
		t.Errorf("Summary = %q, want %q", got.Summary, "Write docs")
	}
	if got.DueDate == nil {
		t.Fatal("DueDate = nil, want 2025-03-01")
	}
	want := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	if !got.DueDate.Equal(want) {
		t.Errorf("DueDate = %v, want %v", *got.DueDate, want)
	}
}

func weekdayPtr(wd time.Weekday) *time.Weekday { return &wd }

func timePtr(t time.Time) *time.Time { return &t }
