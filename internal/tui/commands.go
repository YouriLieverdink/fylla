package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/iruoy/fylla/internal/tui/msg"
)

// EditTaskParams holds all parameters for editing a task from the TUI.
type EditTaskParams struct {
	TaskKey   string
	Provider  string
	Summary   string
	Estimate  string
	Due       string
	Priority  string
	UpNext    *bool
	NoSplit   *bool
	NotBefore    string
	HadNotBefore bool
	Parent       string
	Section      string
	HadDue       bool
	HadEstimate  bool
	HadPriority  bool
	HadParent    bool
	HadSection   bool
}

// FallbackIssue pairs a Jira key with its summary for display.
type FallbackIssue struct {
	Key     string
	Summary string
}

// Callbacks holds function references that the TUI uses to invoke business logic.
type Callbacks struct {
	LoadToday   func() ([]msg.FyllaEvent, error)
	LoadTasks   func() ([]msg.ScoredTask, error)
	DoneTask    func(taskKey string) error
	DeleteTask  func(taskKey string) error
	StartTimer  func(taskKey, project, section string) error
	TimerStatus func() (taskKey, summary, project, section string, elapsed time.Duration, running bool, err error)
	SyncPreview func() (*msg.SyncResult, error)
	SyncApply   func(force bool) (*msg.SyncResult, error)
	ClearEvents func() (int, error)
	LoadConfig  func() (string, error)
	SetConfig   func(key, value string) error
	AddTask      func(provider, summary, project, section, issueType, description, estimate, dueDate, priority, parent string) (key, summaryOut string, err error)
	EditTask     func(params EditTaskParams) error
	StopTimer    func(description string, done bool, fallbackIssue string) (taskKey string, elapsed time.Duration, err error)
	AbortTimer   func() (taskKey string, err error)
	ListProjects func(provider string) ([]string, error)
	ListSections func(provider, project string) ([]string, error)
	ListLanes    func(provider, project string) ([]string, error)
	ListEpics    func(project string) ([]msg.EpicOption, error)
	GetParent    func(taskKey string) (string, error)
	Provider     func() string
	Providers    func() []string
	SnoozeTask     func(taskKey, target string) error
	ViewTask       func(taskKey string) (*msg.ViewResult, error)
	LoadReport     func(days int) (*msg.ReportResult, error)
	LoadWorklogs   func(weekView bool, date time.Time) ([]msg.WorklogEntry, error)
	UpdateWorklog  func(issueKey, worklogID, provider string, timeSpent time.Duration, description string, started time.Time) error
	DeleteWorklog  func(issueKey, worklogID, provider string) error
	AddWorklog     func(issueKey, provider string, timeSpent time.Duration, description string, started time.Time) error
	FallbackIssues func() []FallbackIssue
}

func loadTodayCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		events, err := cb.LoadToday()
		return msg.TodayLoadedMsg{Events: events, Err: err}
	}
}

func loadTasksCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		tasks, err := cb.LoadTasks()
		return msg.TasksLoadedMsg{Tasks: tasks, Err: err}
	}
}

func doneTaskCmd(cb Callbacks, taskKey string) tea.Cmd {
	return func() tea.Msg {
		err := cb.DoneTask(taskKey)
		return msg.TaskDoneMsg{TaskKey: taskKey, Err: err}
	}
}

func deleteTaskCmd(cb Callbacks, taskKey string) tea.Cmd {
	return func() tea.Msg {
		err := cb.DeleteTask(taskKey)
		return msg.TaskDeletedMsg{TaskKey: taskKey, Err: err}
	}
}

func startTimerCmd(cb Callbacks, taskKey, summary, project, section string) tea.Cmd {
	return func() tea.Msg {
		err := cb.StartTimer(taskKey, project, section)
		return msg.TimerStartedMsg{TaskKey: taskKey, Summary: summary, Project: project, Section: section, Err: err}
	}
}

func timerStatusCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		taskKey, summary, project, section, elapsed, running, err := cb.TimerStatus()
		return msg.TimerStatusMsg{
			TaskKey: taskKey,
			Summary: summary,
			Project: project,
			Section: section,
			Elapsed: elapsed,
			Running: running,
			Err:     err,
		}
	}
}

func timerTickCmd(gen int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return msg.TimerTickMsg{Gen: gen}
	})
}

func syncPreviewCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		result, err := cb.SyncPreview()
		return msg.SyncPreviewMsg{Result: result, Err: err}
	}
}

func syncApplyCmd(cb Callbacks, force bool) tea.Cmd {
	return func() tea.Msg {
		result, err := cb.SyncApply(force)
		return msg.SyncDoneMsg{Result: result, Err: err}
	}
}

func clearEventsCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		count, err := cb.ClearEvents()
		return msg.ClearDoneMsg{Count: count, Err: err}
	}
}

func loadConfigCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		content, err := cb.LoadConfig()
		return msg.ConfigLoadedMsg{Content: content, Err: err}
	}
}

func setConfigCmd(cb Callbacks, key, value string) tea.Cmd {
	return func() tea.Msg {
		err := cb.SetConfig(key, value)
		return msg.ConfigSetMsg{Key: key, Err: err}
	}
}

func addTaskCmd(cb Callbacks, provider, summary, project, section, issueType, description, estimate, dueDate, priority, parent string) tea.Cmd {
	return func() tea.Msg {
		key, summaryOut, err := cb.AddTask(provider, summary, project, section, issueType, description, estimate, dueDate, priority, parent)
		return msg.TaskAddedMsg{Key: key, Summary: summaryOut, Err: err}
	}
}

func editTaskCmd(cb Callbacks, params EditTaskParams) tea.Cmd {
	return func() tea.Msg {
		err := cb.EditTask(params)
		return msg.TaskEditedMsg{TaskKey: params.TaskKey, Err: err}
	}
}

func stopTimerCmd(cb Callbacks, description string, done bool, fallbackIssue string) tea.Cmd {
	return func() tea.Msg {
		taskKey, elapsed, err := cb.StopTimer(description, done, fallbackIssue)
		return msg.TimerStoppedMsg{TaskKey: taskKey, Elapsed: elapsed, Err: err}
	}
}

func abortTimerCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		taskKey, err := cb.AbortTimer()
		return msg.TimerAbortedMsg{TaskKey: taskKey, Err: err}
	}
}

func loadFormOptionsCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		var provider string
		if cb.Provider != nil {
			provider = cb.Provider()
		}
		var providers []string
		if cb.Providers != nil {
			providers = cb.Providers()
		}
		var projects []string
		if cb.ListProjects != nil {
			p, err := cb.ListProjects(provider)
			if err == nil {
				projects = p
			}
		}
		var sections []string
		if cb.ListSections != nil {
			project := ""
			if len(projects) > 0 {
				project = projects[0]
			}
			s, err := cb.ListSections(provider, project)
			if err == nil {
				sections = s
			}
		}
		var epics []msg.EpicOption
		if cb.ListEpics != nil && (provider == "jira" || provider == "kendo") {
			project := ""
			if len(projects) > 0 {
				project = projects[0]
			}
			e, err := cb.ListEpics(project)
			if err == nil && e != nil {
				epics = e
			} else {
				epics = []msg.EpicOption{}
			}
		}
		var lanes []string
		if cb.ListLanes != nil {
			project := ""
			if len(projects) > 0 {
				project = projects[0]
			}
			l, err := cb.ListLanes(provider, project)
			if err == nil {
				lanes = l
			}
		}
		return msg.FormOptionsMsg{Projects: projects, Sections: sections, Lanes: lanes, Provider: provider, Providers: providers, Epics: epics}
	}
}

func loadEditFormOptionsCmd(cb Callbacks, project, taskKey string) tea.Cmd {
	return func() tea.Msg {
		var epics []msg.EpicOption
		if cb.ListEpics != nil {
			e, err := cb.ListEpics(project)
			if err == nil && e != nil {
				epics = e
			} else {
				epics = []msg.EpicOption{}
			}
		}
		var provider string
		if cb.Provider != nil {
			provider = cb.Provider()
		}
		var sections []string
		if cb.ListSections != nil {
			s, err := cb.ListSections(provider, project)
			if err == nil {
				sections = s
			}
		}
		var parentKey string
		if cb.GetParent != nil {
			p, err := cb.GetParent(taskKey)
			if err == nil {
				parentKey = p
			}
		}
		return msg.FormOptionsMsg{Provider: provider, Sections: sections, Epics: epics, ParentKey: parentKey}
	}
}

func loadProjectsCmd(cb Callbacks, provider string) tea.Cmd {
	return func() tea.Msg {
		if cb.ListProjects == nil {
			return msg.ProjectsLoadedMsg{}
		}
		projects, err := cb.ListProjects(provider)
		return msg.ProjectsLoadedMsg{Projects: projects, Err: err}
	}
}

func loadSectionsCmd(cb Callbacks, provider, project string) tea.Cmd {
	return func() tea.Msg {
		if cb.ListSections == nil {
			return msg.SectionsLoadedMsg{}
		}
		sections, err := cb.ListSections(provider, project)
		return msg.SectionsLoadedMsg{Sections: sections, Err: err}
	}
}

func loadLanesCmd(cb Callbacks, provider, project string) tea.Cmd {
	return func() tea.Msg {
		if cb.ListLanes == nil {
			return msg.LanesLoadedMsg{}
		}
		lanes, err := cb.ListLanes(provider, project)
		return msg.LanesLoadedMsg{Lanes: lanes, Err: err}
	}
}

func loadEpicsCmd(cb Callbacks, project string) tea.Cmd {
	return func() tea.Msg {
		if cb.ListEpics == nil {
			return msg.EpicsLoadedMsg{}
		}
		epics, err := cb.ListEpics(project)
		return msg.EpicsLoadedMsg{Epics: epics, Err: err}
	}
}

func snoozeTaskCmd(cb Callbacks, taskKey, target string) tea.Cmd {
	return func() tea.Msg {
		err := cb.SnoozeTask(taskKey, target)
		return msg.TaskSnoozedMsg{TaskKey: taskKey, Err: err}
	}
}

func viewTaskCmd(cb Callbacks, taskKey string) tea.Cmd {
	return func() tea.Msg {
		result, err := cb.ViewTask(taskKey)
		return msg.TaskViewedMsg{Result: result, Err: err}
	}
}

func loadReportCmd(cb Callbacks, days int) tea.Cmd {
	return func() tea.Msg {
		result, err := cb.LoadReport(days)
		return msg.ReportLoadedMsg{Result: result, Err: err}
	}
}

func loadWorklogsCmd(cb Callbacks, weekView bool, date time.Time) tea.Cmd {
	return func() tea.Msg {
		entries, err := cb.LoadWorklogs(weekView, date)
		return msg.WorklogsLoadedMsg{Entries: entries, Err: err}
	}
}

func updateWorklogCmd(cb Callbacks, issueKey, worklogID, provider string, timeSpent time.Duration, description string, started time.Time) tea.Cmd {
	return func() tea.Msg {
		err := cb.UpdateWorklog(issueKey, worklogID, provider, timeSpent, description, started)
		return msg.WorklogUpdatedMsg{Err: err}
	}
}

func deleteWorklogCmd(cb Callbacks, issueKey, worklogID, provider string) tea.Cmd {
	return func() tea.Msg {
		err := cb.DeleteWorklog(issueKey, worklogID, provider)
		return msg.WorklogDeletedMsg{Err: err}
	}
}

func addWorklogCmd(cb Callbacks, issueKey, provider string, timeSpent time.Duration, description string, started time.Time) tea.Cmd {
	return func() tea.Msg {
		err := cb.AddWorklog(issueKey, provider, timeSpent, description, started)
		return msg.WorklogAddedMsg{Err: err}
	}
}

func prefetchFallbackCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		if cb.FallbackIssues == nil {
			return msg.FallbackLoadedMsg{}
		}
		issues := cb.FallbackIssues()
		result := make([]msg.FallbackIssue, len(issues))
		for i, fb := range issues {
			result[i] = msg.FallbackIssue{Key: fb.Key, Summary: fb.Summary}
		}
		return msg.FallbackLoadedMsg{Issues: result}
	}
}

func autoRefreshCmd() tea.Cmd {
	return tea.Tick(60*time.Second, func(time.Time) tea.Msg {
		return msg.AutoRefreshMsg{}
	})
}

func clearToastCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return msg.ClearToastMsg{}
	})
}
