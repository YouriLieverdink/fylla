package msg

import (
	"time"

	"github.com/iruoy/fylla/internal/task"
)

// FyllaEvent represents a scheduled Fylla task event or a calendar event.
type FyllaEvent struct {
	TaskKey         string
	Provider        string
	Project         string
	Section         string
	Summary         string
	Start           time.Time
	End             time.Time
	AtRisk          bool
	Status          string
	IsCalendarEvent bool
}

// ScoreBreakdown holds the individual components of a composite score.
type ScoreBreakdown struct {
	PriorityRaw      float64
	PriorityWeight   float64
	PriorityWeighted float64
	PriorityReason   string
	DueDateRaw       float64
	DueDateWeight    float64
	DueDateWeighted  float64
	DueDateReason    string
	EstimateRaw      float64
	EstimateWeight   float64
	EstimateWeighted float64
	EstimateReason   string
	AgeRaw           float64
	AgeWeight        float64
	AgeWeighted      float64
	AgeReason        string
	CrunchBoost      float64
	CrunchReason     string
	TypeBonus        float64
	TypeBonusReason  string
	UpNextBoost      float64
	NotBeforeMult    float64
	NotBeforeReason  string
	Total            float64
}

// ScoredTask holds a task with its computed score for display.
type ScoredTask struct {
	Key       string
	Provider  string
	Summary   string
	Priority  int
	DueDate   *time.Time
	Estimate  time.Duration
	IssueType string
	Score     float64
	Breakdown ScoreBreakdown
	Project   string
	Section   string
	Status       string
	UpNext       bool
	NoSplit      bool
	NotBefore    *time.Time
	NotBeforeRaw string
	SprintID     *int
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
	Warnings       []string
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

// SprintOption is an alias for the provider-neutral task.SprintOption type.
type SprintOption = task.SprintOption

// WorklogEntry is an alias for the provider-neutral task.WorklogEntry type.
type WorklogEntry = task.WorklogEntry

