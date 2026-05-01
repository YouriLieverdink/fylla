package tui

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/iruoy/fylla/internal/config"
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
	StartTime    time.Time
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
	TaskKey      string
	Provider     string
	Summary      string
	Estimate     string
	Due          string
	Priority     string
	UpNext       *bool
	NoSplit      *bool
	NotBefore    string
	HadNotBefore bool
	Project      string
	Parent       string
	Section      string
	HadDue       bool
	HadEstimate  bool
	HadPriority  bool
	HadProject   bool
	HadParent    bool
	HadSection   bool
	SprintID     *int
	HadSprint    bool
}

// FallbackIssue pairs an issue key with its summary for display.
type FallbackIssue struct {
	Key     string
	Summary string
}

// Callbacks holds function references that the TUI uses to invoke business logic.
type Callbacks struct {
	LoadToday           func() ([]msg.FyllaEvent, error)
	LoadTasks           func() ([]msg.ScoredTask, error)
	DoneTask            func(taskKey, provider string) error
	DeleteTask          func(taskKey, provider string) error
	OpenTaskURL         func(taskKey, provider, project, issueType string) (string, error)
	StartTimer          func(taskKey, summary, project, section, provider string) error
	InterruptTimer      func() error
	TimerStatus         func() (*TimerStatusInfo, error)
	SaveTimerComment    func(comment string) error
	SaveTimerStartTime  func(startTime time.Time) error
	SyncPreview         func() (*msg.SyncResult, error)
	SyncApply           func(force bool) (*msg.SyncResult, error)
	ClearEvents         func() (int, error)
	LoadConfig          func() (*config.Config, error)
	SetConfig           func(key, value string) error
	AddTask             func(provider, summary, project, section, issueType, lane, description, estimate, dueDate, priority, parent string, sprintID *int) (key, summaryOut string, err error)
	EditTask            func(params EditTaskParams) error
	StopTimer           func(description string, done bool, fallbackIssue, fallbackProvider string) (taskKey string, elapsed time.Duration, resumedKey string, err error)
	AbortTimer          func() (taskKey string, resumedKey string, err error)
	ListProjects        func(provider string) ([]string, error)
	ListSections        func(provider, project string) ([]string, error)
	ListLanes           func(provider, project string) ([]string, error)
	ListEpics           func(provider, project string) ([]msg.EpicOption, error)
	GetParent           func(taskKey, provider string) (string, error)
	Provider            func() string
	Providers           func() []string
	SnoozeTask          func(taskKey, target string) error
	ViewTask            func(taskKey string) (*msg.ViewResult, error)
	LoadWorklogs        func(weekView bool, date time.Time) ([]msg.WorklogEntry, error)
	LoadDashboard       func(month time.Time) ([]msg.WorklogEntry, error)
	UpdateWorklog       func(issueKey, worklogID, provider string, timeSpent time.Duration, description string, started time.Time) error
	DeleteWorklog       func(issueKey, worklogID, provider string) error
	AddWorklog          func(issueKey, provider string, timeSpent time.Duration, description string, started time.Time) error
	FallbackIssues      func() []FallbackIssue
	ListTransitions     func(taskKey, provider string) ([]string, error)
	MoveTask            func(taskKey, provider, target string) error
	ListIssueTypes      func(provider, project string) ([]string, error)
	ListSprints         func(provider, project string) ([]msg.SprintOption, error)
	ResolveIssueKey     func(prKey string) (string, error)
	SearchAllTasks      func(query string) ([]msg.ScoredTask, error)
	LoadTasksByProvider func(provider string) ([]msg.ScoredTask, error)
	BulkDone            func(taskKeys []string) (succeeded []string, failed map[string]error, err error)
	BulkDelete          func(taskKeys []string) (succeeded []string, failed map[string]error, err error)
	BulkMove            func(taskKeys []string, target string) (succeeded []string, failed map[string]error, err error)
	BulkSnooze          func(taskKeys []string, target string) (succeeded []string, failed map[string]error, err error)
	WorklogProvider     func() string
	LoadTargets         func(offset int) ([]msg.TargetProgress, error)
	AddTarget           func(target config.TargetConfig) error
	UpdateTarget        func(index int, target config.TargetConfig) error
	DeleteTarget        func(index int) error
}

func loadTodayCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		events, err := cb.LoadToday()
		return msg.TodayLoadedMsg{Events: events, Err: err}
	}
}

func loadTasksCmd(cb Callbacks) tea.Cmd {
	// When per-provider loading is available with multiple providers,
	// fire one command per provider so results trickle in progressively.
	if cb.LoadTasksByProvider != nil && cb.Providers != nil {
		providers := cb.Providers()
		if len(providers) > 1 {
			cmds := make([]tea.Cmd, len(providers))
			for i, p := range providers {
				provider := p // capture
				cmds[i] = func() tea.Msg {
					tasks, err := cb.LoadTasksByProvider(provider)
					return msg.TasksPartialMsg{Provider: provider, Tasks: tasks, Err: err}
				}
			}
			return tea.Batch(cmds...)
		}
	}
	return func() tea.Msg {
		tasks, err := cb.LoadTasks()
		return msg.TasksLoadedMsg{Tasks: tasks, Err: err}
	}
}

func bulkDoneCmd(cb Callbacks, taskKeys []string) tea.Cmd {
	return func() tea.Msg {
		if cb.BulkDone == nil {
			return msg.BulkActionMsg{Action: "done", Err: fmt.Errorf("bulk done not available")}
		}
		succeeded, failed, err := cb.BulkDone(taskKeys)
		return msg.BulkActionMsg{Action: "done", Succeeded: succeeded, Failed: failed, Err: err}
	}
}

func bulkDeleteCmd(cb Callbacks, taskKeys []string) tea.Cmd {
	return func() tea.Msg {
		if cb.BulkDelete == nil {
			return msg.BulkActionMsg{Action: "delete", Err: fmt.Errorf("bulk delete not available")}
		}
		succeeded, failed, err := cb.BulkDelete(taskKeys)
		return msg.BulkActionMsg{Action: "delete", Succeeded: succeeded, Failed: failed, Err: err}
	}
}

func bulkMoveCmd(cb Callbacks, taskKeys []string, target string) tea.Cmd {
	return func() tea.Msg {
		if cb.BulkMove == nil {
			return msg.BulkActionMsg{Action: "move", Err: fmt.Errorf("bulk move not available")}
		}
		succeeded, failed, err := cb.BulkMove(taskKeys, target)
		return msg.BulkActionMsg{Action: "move", Succeeded: succeeded, Failed: failed, Err: err}
	}
}

func bulkSnoozeCmd(cb Callbacks, taskKeys []string, target string) tea.Cmd {
	return func() tea.Msg {
		if cb.BulkSnooze == nil {
			return msg.BulkActionMsg{Action: "snooze", Err: fmt.Errorf("bulk snooze not available")}
		}
		succeeded, failed, err := cb.BulkSnooze(taskKeys, target)
		return msg.BulkActionMsg{Action: "snooze", Succeeded: succeeded, Failed: failed, Err: err}
	}
}

func bulkEditCmd(cb Callbacks, tasks []msg.ScoredTask, estimate, due, priority string, upNext, noSplit *bool, notBefore string) tea.Cmd {
	return func() tea.Msg {
		if cb.EditTask == nil {
			return msg.BulkActionMsg{Action: "edit", Err: fmt.Errorf("edit not available")}
		}
		var succeeded []string
		failed := make(map[string]error)
		for _, t := range tasks {
			err := cb.EditTask(EditTaskParams{
				TaskKey:   t.Key,
				Provider:  t.Provider,
				Estimate:  estimate,
				Due:       due,
				Priority:  priority,
				UpNext:    upNext,
				NoSplit:   noSplit,
				NotBefore: notBefore,
			})
			if err != nil {
				failed[t.Key] = err
			} else {
				succeeded = append(succeeded, t.Key)
			}
		}
		return msg.BulkActionMsg{Action: "edit", Succeeded: succeeded, Failed: failed}
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

func openTaskURLCmd(cb Callbacks, taskKey, provider, project, issueType string) tea.Cmd {
	return func() tea.Msg {
		if cb.OpenTaskURL == nil {
			return msg.TaskOpenedMsg{TaskKey: taskKey, Err: fmt.Errorf("open in browser not available")}
		}
		openedURL, err := cb.OpenTaskURL(taskKey, provider, project, issueType)
		return msg.TaskOpenedMsg{TaskKey: taskKey, URL: openedURL, Err: err}
	}
}

func startTimerCmd(cb Callbacks, taskKey, summary, project, section, provider string) tea.Cmd {
	return func() tea.Msg {
		err := cb.StartTimer(taskKey, summary, project, section, provider)
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
			StartTime:    info.StartTime,
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

func saveTimerStartTimeCmd(cb Callbacks, startTime time.Time) tea.Cmd {
	return func() tea.Msg {
		err := cb.SaveTimerStartTime(startTime)
		return msg.TimerStartTimeSavedMsg{Err: err}
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
		cfg, err := cb.LoadConfig()
		return msg.ConfigLoadedMsg{Config: cfg, Err: err}
	}
}

func setConfigCmd(cb Callbacks, key, value string) tea.Cmd {
	return func() tea.Msg {
		err := cb.SetConfig(key, value)
		return msg.ConfigSetMsg{Key: key, Err: err}
	}
}

func addTaskCmd(cb Callbacks, provider, summary, project, section, issueType, lane, description, estimate, dueDate, priority, parent string, sprintID *int) tea.Cmd {
	return func() tea.Msg {
		key, summaryOut, err := cb.AddTask(provider, summary, project, section, issueType, lane, description, estimate, dueDate, priority, parent, sprintID)
		return msg.TaskAddedMsg{Key: key, Summary: summaryOut, Err: err}
	}
}

func editTaskCmd(cb Callbacks, params EditTaskParams) tea.Cmd {
	return func() tea.Msg {
		err := cb.EditTask(params)
		return msg.TaskEditedMsg{TaskKey: params.TaskKey, Err: err}
	}
}

func stopTimerCmd(cb Callbacks, description string, done bool, fallbackIssue, fallbackProvider string) tea.Cmd {
	return func() tea.Msg {
		taskKey, elapsed, resumedKey, err := cb.StopTimer(description, done, fallbackIssue, fallbackProvider)
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
		if cb.ListEpics != nil && (provider == "kendo") {
			project := ""
			if len(projects) > 0 {
				project = projects[0]
			}
			e, err := cb.ListEpics(provider, project)
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
		if cb.ListIssueTypes != nil && (provider == "kendo") {
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

func loadEditFormOptionsCmd(cb Callbacks, project, taskKey, taskProvider string) tea.Cmd {
	return func() tea.Msg {
		provider := taskProvider
		if provider == "" && cb.Provider != nil {
			provider = cb.Provider()
		}
		var projects []string
		if cb.ListProjects != nil {
			p, err := cb.ListProjects(provider)
			if err == nil {
				projects = p
			}
		}
		var epics []msg.EpicOption
		if cb.ListEpics != nil {
			e, err := cb.ListEpics(provider, project)
			if err == nil && e != nil {
				epics = e
			} else {
				epics = []msg.EpicOption{}
			}
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
			p, err := cb.GetParent(taskKey, provider)
			if err == nil {
				parentKey = p
			}
		}
		var sprints []msg.SprintOption
		if provider == "kendo" && cb.ListSprints != nil {
			s, err := cb.ListSprints(provider, project)
			if err == nil {
				sprints = s
			}
		}
		return msg.FormOptionsMsg{Provider: provider, Projects: projects, Sections: sections, Epics: epics, ParentKey: parentKey, Sprints: sprints}
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

func loadEpicsCmd(cb Callbacks, provider, project string) tea.Cmd {
	return func() tea.Msg {
		if cb.ListEpics == nil {
			return msg.EpicsLoadedMsg{}
		}
		epics, err := cb.ListEpics(provider, project)
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

func loadTargetsCmd(cb Callbacks, offset int) tea.Cmd {
	return func() tea.Msg {
		if cb.LoadTargets == nil {
			return msg.TargetsLoadedMsg{Err: fmt.Errorf("targets not available")}
		}
		items, err := cb.LoadTargets(offset)
		return msg.TargetsLoadedMsg{Offset: offset, Items: items, Err: err}
	}
}

func addTargetCmd(cb Callbacks, target config.TargetConfig) tea.Cmd {
	return func() tea.Msg {
		if cb.AddTarget == nil {
			return msg.TargetSavedMsg{Action: "add", Err: fmt.Errorf("add target not available")}
		}
		err := cb.AddTarget(target)
		return msg.TargetSavedMsg{Action: "add", Err: err}
	}
}

func updateTargetCmd(cb Callbacks, index int, target config.TargetConfig) tea.Cmd {
	return func() tea.Msg {
		if cb.UpdateTarget == nil {
			return msg.TargetSavedMsg{Action: "edit", Err: fmt.Errorf("update target not available")}
		}
		err := cb.UpdateTarget(index, target)
		return msg.TargetSavedMsg{Action: "edit", Err: err}
	}
}

func deleteTargetCmd(cb Callbacks, index int) tea.Cmd {
	return func() tea.Msg {
		if cb.DeleteTarget == nil {
			return msg.TargetSavedMsg{Action: "delete", Err: fmt.Errorf("delete target not available")}
		}
		err := cb.DeleteTarget(index)
		return msg.TargetSavedMsg{Action: "delete", Err: err}
	}
}

func loadDashboardCmd(cb Callbacks, month time.Time) tea.Cmd {
	return func() tea.Msg {
		entries, err := cb.LoadDashboard(month)
		return msg.DashboardLoadedMsg{Entries: entries, Err: err}
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

func resolveIssueKeyCmd(cb Callbacks, prKey string) tea.Cmd {
	return func() tea.Msg {
		if cb.ResolveIssueKey == nil {
			return msg.IssueKeyResolvedMsg{Err: fmt.Errorf("no resolver available")}
		}
		key, err := cb.ResolveIssueKey(prKey)
		return msg.IssueKeyResolvedMsg{Key: key, Err: err}
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

func clearToastCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return msg.ClearToastMsg{}
	})
}

func generateStandupCmd(entries []msg.WorklogEntry, date time.Time) tea.Cmd {
	return func() tea.Msg {
		if len(entries) == 0 {
			return msg.StandupGeneratedMsg{Content: "No worklogs to summarize."}
		}

		// Merge entries by issue key: sum durations, collect descriptions.
		type merged struct {
			key          string
			summary      string
			duration     time.Duration
			descriptions []string
		}
		byKey := make(map[string]*merged)
		var order []string
		for _, e := range entries {
			m, ok := byKey[e.IssueKey]
			if !ok {
				m = &merged{key: e.IssueKey, summary: e.IssueSummary}
				byKey[e.IssueKey] = m
				order = append(order, e.IssueKey)
			}
			m.duration += e.TimeSpent
			if e.Description != "" {
				m.descriptions = append(m.descriptions, e.Description)
			}
		}

		// Sort by total duration descending.
		sort.Slice(order, func(i, j int) bool {
			return byKey[order[i]].duration > byKey[order[j]].duration
		})

		// Build context for Claude.
		var sb strings.Builder
		for _, key := range order {
			m := byKey[key]
			fmt.Fprintf(&sb, "- %s: %s (%s)", m.key, m.summary, formatDur(m.duration))
			if len(m.descriptions) > 0 {
				fmt.Fprintf(&sb, " — %s", strings.Join(m.descriptions, "; "))
			}
			sb.WriteString("\n")
		}

		prompt := fmt.Sprintf(
			"Hier zijn mijn worklogs van %s:\n\n%s\nSchrijf een korte stand-up samenvatting in het Nederlands als één doorlopende paragraaf (geen opsommingstekens, geen lijsten). Houd het natuurlijk en beknopt — dit is om in een teamchat te plakken. Geen tijdsduren, begroetingen of afsluitingen.",
			date.Format("Monday, January 2"),
			sb.String(),
		)

		out, err := exec.Command("claude", "-p", "--model", "haiku", prompt).Output()
		if err != nil {
			return msg.StandupGeneratedMsg{Err: fmt.Errorf("claude: %w", err)}
		}
		return msg.StandupGeneratedMsg{Content: strings.TrimSpace(string(out))}
	}
}

func formatDur(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}
