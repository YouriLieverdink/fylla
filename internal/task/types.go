package task

import "time"

// WorklogEntry represents a single worklog entry from any provider.
type WorklogEntry struct {
	ID           string
	IssueKey     string
	Provider     string
	Project      string
	IssueSummary string
	Description  string
	Started      time.Time
	TimeSpent    time.Duration
}

// Epic represents an epic or parent issue.
type Epic struct {
	Key     string
	Summary string
}

// SprintOption represents a selectable sprint.
type SprintOption struct {
	ID     int
	Label  string
	Active bool
}
