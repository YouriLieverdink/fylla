package task

import "time"

// Recurrence describes a repeating schedule for a task.
type Recurrence struct {
	Freq string // "daily", "weekly", "biweekly", "monthly"
	Days []int  // ISO weekdays: 1=Mon..7=Sun; empty = all applicable
}

// Task represents a schedulable task from any source (Kendo, Todoist, GitHub, etc.).
type Task struct {
	Key               string
	Provider          string
	Summary           string
	Priority          int // 1 (Highest) to 5 (Lowest)
	DueDate           *time.Time
	OriginalEstimate  time.Duration
	RemainingEstimate time.Duration
	IssueType         string // Bug, Task, Feature (Kendo) or label (Todoist)
	Created           time.Time
	Project           string
	Section           string
	NotBefore         *time.Time
	NotBeforeRaw      string // raw value from summary (e.g. "-3d") for round-trip editing
	UpNext            bool
	NoSplit           bool
	Recurrence        *Recurrence
	RecurrenceRaw     string // human-readable recurrence (e.g. "every monday"), provider-native
	Status            string
	SprintID          *int
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
	DueString   string // raw recurrence/natural-language due (e.g. "every monday"), provider-native
	Priority    string // Priority name (Highest, High, Medium, Low, Lowest)
	Parent      string // Parent issue key (e.g. Epic key)
	SprintID    *int   // Sprint/iteration ID (Kendo)
	Lane        string // Board column / lane name (Kendo)
}

// BatchUpdate carries the subset of fields a provider can set in a single
// request. A nil pointer means "field unchanged"; for the removable fields a
// non-nil zero value means "remove". Title is the bracket-free summary portion
// — the provider owns estimate-bracket placement, so callers must never embed
// an [estimate] clause in Title.
type BatchUpdate struct {
	Title     *string        // composed summary text (no [estimate] bracket)
	Estimate  *time.Duration // nil = unchanged, 0 = remove, >0 = set
	Priority  *int           // fylla level 1..5; nil = unchanged, 0 = remove
	DueDate   *time.Time     // absolute due date set
	DueString *string        // natural-language / recurrence set
	RemoveDue bool           // clear due date
}
