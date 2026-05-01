package msg

import (
	"time"

	"github.com/iruoy/fylla/internal/config"
)

// TodayLoadedMsg carries the result of loading today's events.
type TodayLoadedMsg struct {
	Events []FyllaEvent
	Err    error
}

// TasksLoadedMsg carries the result of loading tasks.
type TasksLoadedMsg struct {
	Tasks []ScoredTask
	Err   error
}

// TasksPartialMsg carries tasks from a single provider, enabling progressive loading.
type TasksPartialMsg struct {
	Provider string
	Tasks    []ScoredTask
	Err      error
}

// TaskDoneMsg is sent after marking a task done.
type TaskDoneMsg struct {
	TaskKey string
	Err     error
}

// TaskDeletedMsg is sent after deleting a task.
type TaskDeletedMsg struct {
	TaskKey string
	Err     error
}

// TaskOpenedMsg is sent after opening a task in the browser.
type TaskOpenedMsg struct {
	TaskKey string
	URL     string
	Err     error
}

// TaskAddedMsg is sent after adding a task.
type TaskAddedMsg struct {
	Key     string
	Summary string
	Err     error
}

// TaskEditedMsg is sent after editing a task.
type TaskEditedMsg struct {
	TaskKey string
	Err     error
}

// PausedTimerInfo describes a paused timer in the stack.
type PausedTimerInfo struct {
	TaskKey      string
	Project      string
	SegmentCount int
}

// TimerSegmentInfo describes a completed segment in the timer status.
type TimerSegmentInfo struct {
	Duration time.Duration
	Comment  string
}

// TimerStatusMsg carries the current timer status.
type TimerStatusMsg struct {
	TaskKey      string
	Summary      string
	Project      string
	Section      string
	Comment      string
	StartTime    time.Time
	Elapsed      time.Duration
	TotalElapsed time.Duration
	Segments     []TimerSegmentInfo
	Running      bool
	Paused       []PausedTimerInfo
	Err          error
}

// TimerCommentSavedMsg is sent after saving a comment to the running timer.
type TimerCommentSavedMsg struct {
	Err error
}

// TimerStartTimeSavedMsg is sent after changing the start time of the running timer.
type TimerStartTimeSavedMsg struct {
	Err error
}

// TimerStartedMsg is sent after starting a timer.
type TimerStartedMsg struct {
	TaskKey string
	Summary string
	Project string
	Section string
	Err     error
}

// TimerStoppedMsg is sent after stopping a timer.
type TimerStoppedMsg struct {
	TaskKey    string
	Elapsed    time.Duration
	ResumedKey string
	Err        error
}

// TimerAbortedMsg is sent after aborting a timer.
type TimerAbortedMsg struct {
	TaskKey    string
	ResumedKey string
	Err        error
}

// TimerInterruptedMsg is sent after interrupting a timer.
type TimerInterruptedMsg struct {
	Err error
}

// TimerTickMsg triggers timer display updates.
// Gen is a generation counter to deduplicate concurrent tick chains.
type TimerTickMsg struct {
	Gen int
}

// SyncPreviewMsg carries the dry-run sync result.
type SyncPreviewMsg struct {
	Result *SyncResult
	Err    error
}

// SyncDoneMsg is sent after applying a sync.
type SyncDoneMsg struct {
	Result *SyncResult
	Err    error
}

// ClearDoneMsg is sent after clearing events.
type ClearDoneMsg struct {
	Count int
	Err   error
}

// ConfigLoadedMsg carries the parsed config.
type ConfigLoadedMsg struct {
	Config *config.Config
	Err    error
}

// ConfigSetMsg is sent after setting a config value.
type ConfigSetMsg struct {
	Key string
	Err error
}

// ToastMsg displays a temporary notification.
type ToastMsg struct {
	Message string
	IsError bool
}

// ClearToastMsg clears the current toast.
type ClearToastMsg struct{}

// FormOptionsMsg carries project/section lists for populating add form selectors.
type FormOptionsMsg struct {
	Projects   []string
	Sections   []string
	Lanes      []string // lane names (Kendo issue type / board column)
	IssueTypes []string // issue type names (e.g. Kendo types)
	Provider   string   // primary provider name (e.g. "kendo", "todoist")
	Providers  []string // all active provider names
	Epics      []EpicOption
	Sprints    []SprintOption
	ParentKey  string // current parent key (for edit form pre-population)
	Err        error
}

// TaskSnoozedMsg is sent after snoozing a task.
type TaskSnoozedMsg struct {
	TaskKey string
	Err     error
}

// TaskViewedMsg carries the result of viewing a task's details.
type TaskViewedMsg struct {
	Result *ViewResult
	Err    error
}

// EpicsLoadedMsg carries the result of loading epics for a project.
type EpicsLoadedMsg struct {
	Epics []EpicOption
	Err   error
}

// ProjectsLoadedMsg carries the result of loading projects for a specific provider.
type ProjectsLoadedMsg struct {
	Projects []string
	Err      error
}

// SectionsLoadedMsg carries the result of loading sections for a specific project.
type SectionsLoadedMsg struct {
	Sections []string
	Err      error
}

// LanesLoadedMsg carries the result of loading lanes for a specific project.
type LanesLoadedMsg struct {
	Lanes []string
	Err   error
}

// SprintsLoadedMsg carries the result of loading sprints for a specific project.
type SprintsLoadedMsg struct {
	Sprints []SprintOption
	Err     error
}

// IssueTypesLoadedMsg carries the result of loading issue types for a project.
type IssueTypesLoadedMsg struct {
	IssueTypes []string
	Err        error
}

// WorklogsLoadedMsg carries the result of loading worklogs.
type WorklogsLoadedMsg struct {
	Entries []WorklogEntry
	Err     error
}

// WorklogUpdatedMsg is sent after updating a worklog.
type WorklogUpdatedMsg struct {
	Err error
}

// WorklogDeletedMsg is sent after deleting a worklog.
type WorklogDeletedMsg struct {
	Err error
}

// WorklogAddedMsg is sent after adding a worklog.
type WorklogAddedMsg struct {
	Err error
}

// FallbackLoadedMsg carries prefetched fallback issue summaries.
type FallbackLoadedMsg struct {
	Issues []FallbackIssue
}

// FallbackIssue pairs an issue key with its summary for display.
type FallbackIssue struct {
	Key     string
	Summary string
}

// TaskMovedMsg is sent after moving a task to a new status/lane.
type TaskMovedMsg struct {
	TaskKey string
	Target  string
	Err     error
}

// TransitionsLoadedMsg carries available transitions for a task.
type TransitionsLoadedMsg struct {
	TaskKey     string
	Provider    string
	Transitions []string
	Err         error
}

// IssueKeyResolvedMsg carries the result of resolving a GitHub PR to an issue key.
type IssueKeyResolvedMsg struct {
	Key string
	Err error
}

// BulkActionMsg carries the result of a bulk operation.
type BulkActionMsg struct {
	Action    string
	Succeeded []string
	Failed    map[string]error
	Err       error
}

// AllTasksLoadedMsg carries the result of searching all tasks (not just assigned).
type AllTasksLoadedMsg struct {
	Tasks []ScoredTask
	Query string
	Err   error
}

// PickerSearchDebounceMsg triggers a server-side search after typing pauses.
type PickerSearchDebounceMsg struct {
	Query string
}

// StandupGeneratedMsg carries the AI-generated stand-up summary.
type StandupGeneratedMsg struct {
	Content string
	Err     error
}

// DashboardLoadedMsg carries the result of loading dashboard worklogs.
type DashboardLoadedMsg struct {
	Entries []WorklogEntry
	Err     error
}

// AutoRefreshMsg triggers an auto-refresh of the current view.
type AutoRefreshMsg struct{}

// TargetProgress carries the loaded progress for a single target.
type TargetProgress struct {
	Target      config.TargetConfig
	Logged      time.Duration
	PeriodLabel string
	PeriodStart time.Time
	PeriodEnd   time.Time
	Err         error
}

// TargetsLoadedMsg carries the result of loading target progress.
type TargetsLoadedMsg struct {
	Items []TargetProgress
	Err   error
}

// TargetSavedMsg is sent after a target add/update/delete persists to config.
type TargetSavedMsg struct {
	Action string // "add", "edit", "delete"
	Err    error
}
