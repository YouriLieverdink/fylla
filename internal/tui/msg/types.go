package msg

import "time"

// FyllaEvent represents a scheduled Fylla task event or a calendar event.
type FyllaEvent struct {
	TaskKey         string
	Project         string
	Section         string
	Summary         string
	Start           time.Time
	End             time.Time
	AtRisk          bool
	IsCalendarEvent bool
}

// ScoredTask holds a task with its computed score for display.
type ScoredTask struct {
	Key       string
	Summary   string
	Priority  int
	DueDate   *time.Time
	Estimate  time.Duration
	IssueType string
	Score     float64
	Project   string
	Section   string
	UpNext    bool
	NoSplit   bool
	NotBefore *time.Time
}

// CalendarEvent represents a non-task calendar event (meeting, etc.).
type CalendarEvent struct {
	Summary string
	Start   time.Time
	End     time.Time
}

// SyncResult holds the result of a sync operation for display.
type SyncResult struct {
	Allocations    []Allocation
	AtRisk         []Allocation
	Unscheduled    []UnscheduledTask
	CalendarEvents []CalendarEvent
	Created        int
	Updated        int
	Deleted        int
	Unchanged      int
}

// Allocation represents a scheduled task allocation.
type Allocation struct {
	TaskKey string
	Summary string
	Project string
	Section string
	Start   time.Time
	End     time.Time
	AtRisk  bool
}

// UnscheduledTask represents a task that could not be scheduled.
type UnscheduledTask struct {
	TaskKey  string
	Project  string
	Section  string
	Summary  string
	Estimate time.Duration
	Reason   string
}
