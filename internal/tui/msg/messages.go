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
	Projects []string
	Provider string // primary provider name (e.g. "jira", "todoist")
	Err      error
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

// AutoRefreshMsg triggers an auto-refresh of the current view.
type AutoRefreshMsg struct{}
