package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/iruoy/fylla/internal/tui/msg"
)

// TimerSegmentInfo describes a completed segment.
type TimerSegmentInfo struct {
	Duration time.Duration
	Comment  string
}

// TimerStatusInfo holds all timer status fields returned by the TimerStatus callback.
type TimerStatusInfo struct {
	TaskKey      string
	Summary      string
	Project      string
	Section      string
	Comment      string
	Elapsed      time.Duration
	TotalElapsed time.Duration
	Segments     []TimerSegmentInfo
	Running      bool
	Paused       []PausedTimerInfo
}

// PausedTimerInfo describes a paused timer in the stack.
type PausedTimerInfo struct {
	TaskKey      string
	Project      string
	SegmentCount int
}

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
	DoneTask    func(taskKey, provider string) error
	DeleteTask  func(taskKey, provider string) error
	StartTimer     func(taskKey, project, section string) error
	InterruptTimer func() error
	TimerStatus    func() (*TimerStatusInfo, error)
	SaveTimerComment func(comment string) error
	SyncPreview func() (*msg.SyncResult, error)
	SyncApply   func(force bool) (*msg.SyncResult, error)
	ClearEvents func() (int, error)
	LoadConfig  func() (string, error)
	SetConfig   func(key, value string) error
	AddTask      func(provider, summary, project, section, issueType, description, estimate, dueDate, priority, parent string, sprintID *int) (key, summaryOut string, err error)
	EditTask     func(params EditTaskParams) error
	StopTimer    func(description string, done bool, fallbackIssue string) (taskKey string, elapsed time.Duration, resumedKey string, err error)
	AbortTimer   func() (taskKey string, resumedKey string, err error)
	ListProjects func(provider string) ([]string, error)
	ListSections func(provider, project string) ([]string, error)
	ListLanes    func(provider, project string) ([]string, error)
	ListEpics    func(project string) ([]msg.EpicOption, error)
	GetParent    func(taskKey string) (string, error)
	Provider     func() string
	Providers    func() []string
	SnoozeTask   func(taskKey, target string) error
	ViewTask     func(taskKey string) (*msg.ViewResult, error)
	LoadWorklogs func(weekView bool, date time.Time) ([]msg.WorklogEntry, error)
	UpdateWorklog  func(issueKey, worklogID, provider string, timeSpent time.Duration, description string, started time.Time) error
	DeleteWorklog  func(issueKey, worklogID, provider string) error
	AddWorklog     func(issueKey, provider string, timeSpent time.Duration, description string, started time.Time) error
	FallbackIssues   func() []FallbackIssue
	ListTransitions  func(taskKey, provider string) ([]string, error)
	MoveTask         func(taskKey, provider, target string) error
	ListIssueTypes   func(provider, project string) ([]string, error)
	ListSprints      func(provider, project string) ([]msg.SprintOption, error)
	ResolveJiraKey   func(prKey string) (string, error)
	SearchAllTasks   func(query string) ([]msg.ScoredTask, error)
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

func doneTaskCmd(cb Callbacks, taskKey, provider string) tea.Cmd {
	return func() tea.Msg {
		err := cb.DoneTask(taskKey, provider)
		return msg.TaskDoneMsg{TaskKey: taskKey, Err: err}
	}
}

func deleteTaskCmd(cb Callbacks, taskKey, provider string) tea.Cmd {
	return func() tea.Msg {
		err := cb.DeleteTask(taskKey, provider)
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
		info, err := cb.TimerStatus()
		if err != nil {
			return msg.TimerStatusMsg{Err: err}
		}
		if info == nil {
			return msg.TimerStatusMsg{}
		}
		m := msg.TimerStatusMsg{
			TaskKey:      info.TaskKey,
			Summary:      info.Summary,
			Project:      info.Project,
			Section:      info.Section,
			Comment:      info.Comment,
			Elapsed:      info.Elapsed,
			TotalElapsed: info.TotalElapsed,
			Running:      info.Running,
		}
		for _, s := range info.Segments {
			m.Segments = append(m.Segments, msg.TimerSegmentInfo{Duration: s.Duration, Comment: s.Comment})
		}
		for _, p := range info.Paused {
			m.Paused = append(m.Paused, msg.PausedTimerInfo{
				TaskKey:      p.TaskKey,
				Project:      p.Project,
				SegmentCount: p.SegmentCount,
			})
		}
		return m
	}
}

func saveTimerCommentCmd(cb Callbacks, comment string) tea.Cmd {
	return func() tea.Msg {
		err := cb.SaveTimerComment(comment)
		return msg.TimerCommentSavedMsg{Err: err}
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

func addTaskCmd(cb Callbacks, provider, summary, project, section, issueType, description, estimate, dueDate, priority, parent string, sprintID *int) tea.Cmd {
	return func() tea.Msg {
		key, summaryOut, err := cb.AddTask(provider, summary, project, section, issueType, description, estimate, dueDate, priority, parent, sprintID)
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
		taskKey, elapsed, resumedKey, err := cb.StopTimer(description, done, fallbackIssue)
		return msg.TimerStoppedMsg{TaskKey: taskKey, Elapsed: elapsed, ResumedKey: resumedKey, Err: err}
	}
}

func abortTimerCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		taskKey, resumedKey, err := cb.AbortTimer()
		return msg.TimerAbortedMsg{TaskKey: taskKey, ResumedKey: resumedKey, Err: err}
	}
}

func interruptTimerCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		err := cb.InterruptTimer()
		return msg.TimerInterruptedMsg{Err: err}
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
		var issueTypes []string
		if cb.ListIssueTypes != nil && provider == "jira" {
			project := ""
			if len(projects) > 0 {
				project = projects[0]
			}
			it, err := cb.ListIssueTypes(provider, project)
			if err == nil {
				issueTypes = it
			}
		}
		return msg.FormOptionsMsg{Projects: projects, Sections: sections, Lanes: lanes, IssueTypes: issueTypes, Provider: provider, Providers: providers, Epics: epics}
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

func loadIssueTypesCmd(cb Callbacks, provider, project string) tea.Cmd {
	return func() tea.Msg {
		if cb.ListIssueTypes == nil {
			return msg.IssueTypesLoadedMsg{}
		}
		types, err := cb.ListIssueTypes(provider, project)
		return msg.IssueTypesLoadedMsg{IssueTypes: types, Err: err}
	}
}

func loadSprintsCmd(cb Callbacks, provider, project string) tea.Cmd {
	return func() tea.Msg {
		if cb.ListSprints == nil {
			return msg.SprintsLoadedMsg{}
		}
		sprints, err := cb.ListSprints(provider, project)
		return msg.SprintsLoadedMsg{Sprints: sprints, Err: err}
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

func listTransitionsCmd(cb Callbacks, taskKey, provider string) tea.Cmd {
	return func() tea.Msg {
		if cb.ListTransitions == nil {
			return msg.TransitionsLoadedMsg{TaskKey: taskKey, Provider: provider, Err: fmt.Errorf("transitions not supported")}
		}
		transitions, err := cb.ListTransitions(taskKey, provider)
		return msg.TransitionsLoadedMsg{TaskKey: taskKey, Provider: provider, Transitions: transitions, Err: err}
	}
}

func moveTaskCmd(cb Callbacks, taskKey, provider, target string) tea.Cmd {
	return func() tea.Msg {
		err := cb.MoveTask(taskKey, provider, target)
		return msg.TaskMovedMsg{TaskKey: taskKey, Target: target, Err: err}
	}
}

func resolveJiraKeyCmd(cb Callbacks, prKey string) tea.Cmd {
	return func() tea.Msg {
		if cb.ResolveJiraKey == nil {
			return msg.JiraKeyResolvedMsg{Err: fmt.Errorf("no resolver available")}
		}
		key, err := cb.ResolveJiraKey(prKey)
		return msg.JiraKeyResolvedMsg{Key: key, Err: err}
	}
}

func searchAllTasksCmd(cb Callbacks, query string) tea.Cmd {
	return func() tea.Msg {
		if cb.SearchAllTasks == nil {
			return msg.AllTasksLoadedMsg{Query: query, Err: fmt.Errorf("search not available")}
		}
		tasks, err := cb.SearchAllTasks(query)
		return msg.AllTasksLoadedMsg{Tasks: tasks, Query: query, Err: err}
	}
}

func pickerSearchDebounceCmd(query string) tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg {
		return msg.PickerSearchDebounceMsg{Query: query}
	})
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
