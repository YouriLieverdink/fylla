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
	UpNext       bool
	NoSplit      bool
	NotBefore    *time.Time
	NotBeforeRaw string
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

// ViewResult holds task details for the view overlay.
type ViewResult struct {
	Key       string
	Summary   string
	Priority  int
	Estimate  time.Duration
	DueDate   *time.Time
	NotBefore *time.Time
	UpNext    bool
	NoSplit   bool
}

// EpicOption represents an epic for form select fields.
type EpicOption struct {
	Key   string
	Label string // "KEY — Summary"
}

// WorklogEntry represents a worklog entry for TUI display.
type WorklogEntry struct {
	ID           string
	IssueKey     string
	IssueSummary string
	Description  string
	Started      time.Time
	TimeSpent    time.Duration
}

// ReportResult holds summary stats for the report overlay.
type ReportResult struct {
	Start       time.Time
	End         time.Time
	TasksDone   int
	TaskTime    time.Duration
	MeetingTime time.Duration
	TotalEvents int
}
