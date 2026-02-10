package calendar

import "time"

// Event represents a calendar event with fields relevant to scheduling.
type Event struct {
	ID        string
	Title     string
	Start     time.Time
	End       time.Time
	EventType string // e.g. "outOfOffice", "default"
	AllDay    bool
}

// IsOOO returns true if the event represents an out-of-office period.
// It checks both the Google Calendar eventType field and common title patterns.
func (e Event) IsOOO() bool {
	if e.EventType == "outOfOffice" {
		return true
	}
	return isOOOTitle(e.Title)
}

// oooPatterns contains title substrings that indicate out-of-office events.
var oooPatterns = []string{
	"ooo",
	"out of office",
	"pto",
	"vacation",
}

func isOOOTitle(title string) bool {
	lower := toLower(title)
	for _, pattern := range oooPatterns {
		if containsStr(lower, pattern) {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func containsStr(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
