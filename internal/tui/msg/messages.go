package msg

import "time"

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

// TimerStatusMsg carries the current timer status.
type TimerStatusMsg struct {
	TaskKey string
	Summary string
	Project string
	Section string
	Elapsed time.Duration
	Running bool
	Err     error
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
	TaskKey string
	Elapsed time.Duration
	Err     error
}

// TimerAbortedMsg is sent after aborting a timer.
type TimerAbortedMsg struct {
	TaskKey string
	Err     error
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

// ConfigLoadedMsg carries the config display string.
type ConfigLoadedMsg struct {
	Content string
	Err     error
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
	Projects  []string
	Sections  []string
	Lanes     []string // lane names (Kendo issue type / board column)
	Provider  string   // primary provider name (e.g. "jira", "todoist")
	Providers []string // all active provider names
	Epics     []EpicOption
	ParentKey string // current parent key (for edit form pre-population)
	Err       error
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

// ReportLoadedMsg carries the result of loading a report.
type ReportLoadedMsg struct {
	Result *ReportResult
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

// FallbackIssue pairs a Jira key with its summary for display.
type FallbackIssue struct {
	Key     string
	Summary string
}

// AutoRefreshMsg triggers an auto-refresh of the current view.
type AutoRefreshMsg struct{}
