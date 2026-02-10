package jira

import "time"

// Task represents a Jira issue with fields relevant to scheduling.
type Task struct {
	Key               string
	Summary           string
	Priority          int // 1 (Highest) to 5 (Lowest)
	DueDate           *time.Time
	OriginalEstimate  time.Duration
	RemainingEstimate time.Duration
	IssueType         string // Bug, Task, Story, etc.
	Created           time.Time
	Project           string
}
