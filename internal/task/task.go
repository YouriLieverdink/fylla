package task

import "time"

// Task represents a schedulable task from any source (Jira, Todoist, etc.).
type Task struct {
	Key               string
	Summary           string
	Priority          int // 1 (Highest) to 5 (Lowest)
	DueDate           *time.Time
	OriginalEstimate  time.Duration
	RemainingEstimate time.Duration
	IssueType         string // Bug, Task, Story (Jira) or label (Todoist)
	Created           time.Time
	Project           string
	Section           string
	NotBefore         *time.Time
	UpNext            bool
	NoSplit           bool
}

// CreateInput holds the fields for creating a new task.
type CreateInput struct {
	Project     string
	Section     string
	IssueType   string
	Summary     string
	Description string
	Estimate    time.Duration
	DueDate     *time.Time
	Priority    string // Priority name (Highest, High, Medium, Low, Lowest)
}
