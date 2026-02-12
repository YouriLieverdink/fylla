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

func timePtr(t time.Time) *time.Time { return &t }
