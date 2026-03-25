package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/iruoy/fylla/internal/tui/components"
	"github.com/iruoy/fylla/internal/tui/msg"
	"github.com/iruoy/fylla/internal/tui/styles"
	configView "github.com/iruoy/fylla/internal/tui/views/config"
	"github.com/iruoy/fylla/internal/tui/views/tasks"
	timerView "github.com/iruoy/fylla/internal/tui/views/timer"
	"github.com/iruoy/fylla/internal/tui/views/worklog"
)

const (
	tabTasks = iota
	tabTimer
	tabWorklog
	tabConfig
	tabCount
)

// Deps holds the dependencies needed by the TUI.
type Deps struct {
	CB               Callbacks
	DailyHours       float64
	WeeklyHours      float64
	EfficiencyTarget float64
}

type confirmAction int

const (
	confirmNone confirmAction = iota
	confirmDeleteTask
	confirmSyncApply
	confirmSyncForce
	confirmClearEvents
	confirmAbortTimer
	confirmDeleteWorklog
)

type formKind int

const (
	formNone formKind = iota
	formAddTask
	formAddTaskPending // waiting for project/section options to load
	formEditTask
	formSetConfig
	formEditTaskPending // waiting for epic options to load for edit form
	formSnoozeTask
	formStopTimer
	formStopTimerPending // waiting for tasks to load for picker
	formAddWorklog
	formAddWorklogPending // waiting for tasks to load for picker
	formEditWorklog
	formMoveTaskPending  // waiting for transitions to load
	formTimerComment
	formTimerStartTime
)

type pendingEditData struct {
	summary      string
	estimate     string
	dueDate      string
	priority     string
	upNext       string
	noSplit      string
	notBefore    string
	parentKey    string // current parent key
	section      string
}

type model struct {
	cb           Callbacks
	activeTab    int
	width        int
	height       int
	tasks        tasks.Model
	timer        timerView.Model
	worklog      worklog.Model
	config       *configView.Model
	timerKey       string
	timerSummary   string
	timerComment   string
	timerStartTime time.Time
	timerElapsed   time.Duration
	timerRunning   bool
	tickGen      int
	toast        string
	toastIsError bool
	showHelp     bool
	ready        bool
	confirm      components.ConfirmDialog
	confirmType  confirmAction
	confirmKey      string
	confirmProvider string
	form         components.Form
	picker       components.Picker
	formKind         formKind
	formConfigKey    string
	formTaskKey      string
	formTaskProvider string
	formWorklogID       string
	formWorklogKey      string
	formWorklogProvider string
	formOptions      *msg.FormOptionsMsg
	pickerFieldLabel string
	pendingEdit      *pendingEditData
	viewDetail *msg.ViewResult
	spinner    spinner.Model
	saving       string // non-empty shows spinner in status bar with this label

	// Background prefetch cache
	cachedTasks       []msg.ScoredTask
	cachedFormOptions *msg.FormOptionsMsg
	cachedFallback    []msg.FallbackIssue
}

func initialModel(deps Deps) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	return model{
		cb:       deps.CB,
		tasks:    tasks.New(),
		timer:    timerView.New(),
		worklog:  worklog.New(deps.DailyHours, deps.WeeklyHours, deps.EfficiencyTarget),
		config:   ptrConfig(configView.New()),
		spinner:  s,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		timerStatusCmd(m.cb),
		loadTasksCmd(m.cb),
		loadFormOptionsCmd(m.cb),
		prefetchFallbackCmd(m.cb),
		autoRefreshCmd(),
	)
}

func (m model) Update(mssg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch mssg := mssg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(mssg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = mssg.Width
		m.height = mssg.Height
		contentHeight := m.height - 5
		m.tasks.SetSize(m.width, contentHeight)
		m.timer.SetSize(m.width, contentHeight)
		m.worklog.SetSize(m.width, contentHeight)
		m.config.SetSize(m.width, contentHeight)
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		// Pending form states — only allow escape
		if m.formKind == formAddTaskPending || m.formKind == formEditTaskPending {
			if key.Matches(mssg, keys.Escape) {
				m.formKind = formNone
				m.pendingEdit = nil
			}
			return m, nil
		}
		if m.formKind == formAddWorklogPending {
			if key.Matches(mssg, keys.Escape) {
				m.formKind = formAddWorklog // return to form
			}
			return m, nil
		}
		if m.formKind == formStopTimerPending {
			if key.Matches(mssg, keys.Escape) {
				m.formKind = formStopTimer // return to form
			}
			return m, nil
		}
		if m.formKind == formMoveTaskPending {
			if key.Matches(mssg, keys.Escape) {
				m.formKind = formNone
			}
			return m, nil
		}

		// Picker overlay takes priority
		if m.picker.Active {
			return m.updatePicker(mssg)
		}

		// Form overlay takes priority
		if m.form.Active {
			return m.updateForm(mssg)
		}

		// Confirm dialog takes priority
		if m.confirm.Active {
			return m.updateConfirm(mssg)
		}

		// Help overlay
		if key.Matches(mssg, keys.Help) {
			m.showHelp = !m.showHelp
			return m, nil
		}
		if m.showHelp {
			if key.Matches(mssg, keys.Escape) || key.Matches(mssg, keys.Help) {
				m.showHelp = false
			}
			return m, nil
		}

		// View detail overlay
		if m.viewDetail != nil {
			if key.Matches(mssg, keys.Escape) || mssg.Type == tea.KeyRunes {
				m.viewDetail = nil
			}
			return m, nil
		}

		// Filter mode in tasks view
		if m.activeTab == tabTasks && m.tasks.IsFilterMode() {
			return m.updateTasksFilter(mssg)
		}

		// Quit
		if key.Matches(mssg, keys.Quit) {
			return m, tea.Quit
		}

		// Tab switching
		switch {
		case key.Matches(mssg, keys.Tab1):
			return m.switchTab(tabTasks)
		case key.Matches(mssg, keys.Tab2):
			return m.switchTab(tabTimer)
		case key.Matches(mssg, keys.Tab3):
			return m.switchTab(tabWorklog)
		case key.Matches(mssg, keys.Tab4):
			return m.switchTab(tabConfig)
		case key.Matches(mssg, keys.NextTab):
			return m.switchTab((m.activeTab + 1) % tabCount)
		case key.Matches(mssg, keys.PrevTab):
			return m.switchTab((m.activeTab + tabCount - 1) % tabCount)
		}

		// Route to active view
		switch m.activeTab {
		case tabTasks:
			return m.updateTasks(mssg)
		case tabTimer:
			return m.updateTimer(mssg)
		case tabWorklog:
			return m.updateWorklog(mssg)
		case tabConfig:
			return m.updateConfig(mssg)
		}

	case msg.TasksLoadedMsg:
		// Always update cache on successful fetch
		if mssg.Err == nil {
			m.cachedTasks = mssg.Tasks
		}
		if m.formKind == formAddWorklogPending || m.formKind == formStopTimerPending {
			returnKind := formAddWorklog
			if m.formKind == formStopTimerPending {
				returnKind = formStopTimer
			}
			if mssg.Err != nil {
				m.formKind = returnKind
				m.setToast(fmt.Sprintf("Failed to load tasks: %v", mssg.Err), true)
				cmds = append(cmds, clearToastCmd())
				return m, tea.Batch(cmds...)
			}
			m.openTaskPicker(mssg.Tasks, returnKind)
			return m, nil
		}
		m.tasks.Loading = false
		if mssg.Err != nil {
			m.tasks.Err = mssg.Err
		} else {
			m.tasks.Tasks = mssg.Tasks
			m.tasks.Err = nil
		}
		return m, nil

	case msg.JiraKeyResolvedMsg:
		if mssg.Err == nil && mssg.Key != "" && m.formKind == formStopTimer && m.form.Active {
			current := m.form.ValueByLabel("Issue Key")
			if current == "" || strings.Contains(current, "#") {
				m.form.SetValueByLabel("Issue Key", mssg.Key)
			}
		}
		return m, nil

	case msg.AllTasksLoadedMsg:
		if m.picker.Active && m.picker.Mode == components.PickerModeAllTasks {
			// Only apply results if the query still matches what the user typed
			if mssg.Query == m.picker.Filter.Value() {
				m.picker.Loading = false
				if mssg.Err != nil {
					m.setToast(fmt.Sprintf("Failed to search tasks: %v", mssg.Err), true)
					return m, clearToastCmd()
				}
				m.rebuildPickerItems(mssg.Tasks)
			}
		}
		return m, nil

	case msg.PickerSearchDebounceMsg:
		// Only fire search if picker is still in all-tasks mode and query matches
		if m.picker.Active && m.picker.Mode == components.PickerModeAllTasks && mssg.Query == m.picker.Filter.Value() {
			if mssg.Query == "" {
				m.picker.Items = nil
				m.picker.Loading = false
				return m, nil
			}
			m.picker.Loading = true
			return m, searchAllTasksCmd(m.cb, mssg.Query)
		}
		return m, nil

	case msg.TimerStatusMsg:
		m.timer.Loading = false
		if mssg.Err == nil {
			m.timerKey = mssg.TaskKey
			m.timerSummary = mssg.Summary
			m.timerComment = mssg.Comment
			m.timerStartTime = mssg.StartTime
			m.timerElapsed = mssg.Elapsed
			m.timerRunning = mssg.Running
			m.timer.TaskKey = mssg.TaskKey
			m.timer.Summary = mssg.Summary
			m.timer.Project = mssg.Project
			m.timer.Section = mssg.Section
			m.timer.Comment = mssg.Comment
			m.timer.StartTime = mssg.StartTime
			m.timer.Elapsed = mssg.Elapsed
			m.timer.TotalElapsed = mssg.TotalElapsed
			m.timer.Segments = nil
			for _, s := range mssg.Segments {
				m.timer.Segments = append(m.timer.Segments, timerView.SegmentInfo{Duration: s.Duration, Comment: s.Comment})
			}
			m.timer.Running = mssg.Running
			m.timer.Err = nil
			m.timer.Paused = nil
			for _, p := range mssg.Paused {
				m.timer.Paused = append(m.timer.Paused, timerView.PausedInfo{
					TaskKey:      p.TaskKey,
					Project:      p.Project,
					SegmentCount: p.SegmentCount,
				})
			}
		} else {
			m.timer.Err = mssg.Err
		}
		m.worklog.TimerRunning = m.timerRunning
		m.worklog.TimerElapsed = m.timerElapsed
		if m.timerRunning {
			m.tickGen++
			cmds = append(cmds, timerTickCmd(m.tickGen))
		}
		return m, tea.Batch(cmds...)

	case msg.TimerTickMsg:
		if m.timerRunning && mssg.Gen == m.tickGen {
			m.timerElapsed += time.Second
			m.timer.Elapsed = m.timerElapsed
			m.timer.TotalElapsed += time.Second
			m.worklog.TimerElapsed = m.timerElapsed
			cmds = append(cmds, timerTickCmd(m.tickGen))
		}
		return m, tea.Batch(cmds...)

	case msg.TimerStartedMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Timer error: %v", mssg.Err), true)
		} else {
			m.timerKey = mssg.TaskKey
			m.timerSummary = mssg.Summary
			m.timerElapsed = 0
			m.timerRunning = true
			m.timer.TaskKey = mssg.TaskKey
			m.timer.Summary = mssg.Summary
			m.timer.Project = mssg.Project
			m.timer.Section = mssg.Section
			m.timer.Elapsed = 0
			m.timer.TotalElapsed = 0
			m.timer.Segments = nil
			m.timer.Running = true
			m.worklog.TimerRunning = true
			m.worklog.TimerElapsed = 0
			label := mssg.Summary
			if label == "" {
				label = mssg.TaskKey
			}
			m.setToast(fmt.Sprintf("Timer started for %s", label), false)
			m.tickGen++
			cmds = append(cmds, timerTickCmd(m.tickGen))
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TransitionsLoadedMsg:
		if m.formKind != formMoveTaskPending {
			return m, nil
		}
		if mssg.Err != nil {
			m.formKind = formNone
			m.setToast(fmt.Sprintf("Transitions error: %v", mssg.Err), true)
			cmds = append(cmds, clearToastCmd())
			return m, tea.Batch(cmds...)
		}
		if len(mssg.Transitions) == 0 {
			m.formKind = formNone
			m.setToast("No transitions available", true)
			cmds = append(cmds, clearToastCmd())
			return m, tea.Batch(cmds...)
		}
		items := make([]components.PickerItem, len(mssg.Transitions))
		for i, t := range mssg.Transitions {
			items[i] = components.PickerItem{Key: t, Label: t}
		}
		m.picker = components.NewPicker(fmt.Sprintf("Move %s to:", m.formTaskKey), items)
		m.pickerFieldLabel = "move"
		m.formKind = formNone
		return m, nil

	case msg.TaskMovedMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Move error: %v", mssg.Err), true)
		} else {
			m.setToast(fmt.Sprintf("Moved %s to %s", mssg.TaskKey, mssg.Target), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TaskDoneMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Done error: %v", mssg.Err), true)
		} else {
			m.setToast(fmt.Sprintf("Marked %s as done", mssg.TaskKey), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TaskDeletedMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Delete error: %v", mssg.Err), true)
		} else {
			m.setToast(fmt.Sprintf("Deleted %s", mssg.TaskKey), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TaskAddedMsg:
		m.saving = ""
		if mssg.Err != nil {
			if m.formKind == formAddTask {
				m.form.Error = fmt.Sprintf("Add error: %v", mssg.Err)
				return m, nil
			}
			m.setToast(fmt.Sprintf("Add error: %v", mssg.Err), true)
		} else {
			m.form.Active = false
			m.formKind = formNone
			m.formOptions = nil
			m.setToast(fmt.Sprintf("Added %s: %s", mssg.Key, mssg.Summary), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TaskEditedMsg:
		m.saving = ""
		if mssg.Err != nil {
			if m.formKind == formEditTask {
				m.form.Error = fmt.Sprintf("Edit error: %v", mssg.Err)
				return m, nil
			}
			m.setToast(fmt.Sprintf("Edit error: %v", mssg.Err), true)
		} else {
			m.form.Active = false
			m.formKind = formNone
			m.pendingEdit = nil
			m.setToast(fmt.Sprintf("Edited %s", mssg.TaskKey), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TimerStoppedMsg:
		m.saving = ""
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Stop error: %v", mssg.Err), true)
		} else {
			stoppedLabel := m.timerSummary
			if stoppedLabel == "" {
				stoppedLabel = mssg.TaskKey
			}
			if stoppedLabel == "" {
				stoppedLabel = "(anonymous)"
			}
			elapsed := styles.FormatDuration(mssg.Elapsed)
			if mssg.ResumedKey != "" {
				m.setToast(fmt.Sprintf("Stopped %s (%s), resumed %s", stoppedLabel, elapsed, mssg.ResumedKey), false)
				// Refresh timer status to pick up resumed timer
				cmds = append(cmds, timerStatusCmd(m.cb))
			} else {
				m.timerRunning = false
				m.timerKey = ""
				m.timerSummary = ""
				m.timerComment = ""
				m.timerElapsed = 0
				m.timer.Running = false
				m.timer.TaskKey = ""
				m.timer.Summary = ""
				m.timer.Project = ""
				m.timer.Section = ""
				m.timer.Elapsed = 0
				m.timer.Paused = nil
				m.worklog.TimerRunning = false
				m.worklog.TimerElapsed = 0
				m.setToast(fmt.Sprintf("Stopped %s (%s)", stoppedLabel, elapsed), false)
			}
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TimerCommentSavedMsg:
		m.saving = ""
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Comment error: %v", mssg.Err), true)
		} else {
			m.setToast("Comment saved", false)
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TimerStartTimeSavedMsg:
		m.saving = ""
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Start time error: %v", mssg.Err), true)
		} else {
			m.setToast("Start time updated", false)
			cmds = append(cmds, timerStatusCmd(m.cb))
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TimerAbortedMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Abort error: %v", mssg.Err), true)
		} else {
			stoppedLabel := m.timerSummary
			if stoppedLabel == "" {
				stoppedLabel = mssg.TaskKey
			}
			if mssg.ResumedKey != "" {
				toastMsg := fmt.Sprintf("Aborted %s, resumed %s", stoppedLabel, mssg.ResumedKey)
				m.setToast(toastMsg, false)
				cmds = append(cmds, timerStatusCmd(m.cb))
			} else {
				m.timerRunning = false
				m.timerKey = ""
				m.timerSummary = ""
				m.timerElapsed = 0
				m.timer.Running = false
				m.timer.TaskKey = ""
				m.timer.Summary = ""
				m.timer.Project = ""
				m.timer.Section = ""
				m.timer.Elapsed = 0
				m.timer.Paused = nil
				m.worklog.TimerRunning = false
				m.worklog.TimerElapsed = 0
				m.setToast(fmt.Sprintf("Timer aborted for %s", stoppedLabel), false)
			}
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TimerInterruptedMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Interrupt error: %v", mssg.Err), true)
		} else {
			m.setToast("Interrupted, anonymous timer started", false)
			cmds = append(cmds, timerStatusCmd(m.cb))
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.ClearDoneMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Clear error: %v", mssg.Err), true)
		} else {
			m.setToast(fmt.Sprintf("Cleared %d events", mssg.Count), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.SyncDoneMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Sync error: %v", mssg.Err), true)
		} else {
			m.setToast(fmt.Sprintf("Sync done: %d created, %d updated, %d deleted",
				mssg.Result.Created, mssg.Result.Updated, mssg.Result.Deleted), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.ConfigLoadedMsg:
		m.config.Loading = false
		if mssg.Err != nil {
			m.config.Err = mssg.Err
		} else {
			m.config.SetConfig(mssg.Config)
		}
		return m, nil

	case msg.ConfigSetMsg:
		m.saving = ""
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Config error: %v", mssg.Err), true)
		} else {
			m.setToast(fmt.Sprintf("Config key %q updated", mssg.Key), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.ToastMsg:
		m.setToast(mssg.Message, mssg.IsError)
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.ClearToastMsg:
		m.toast = ""
		m.toastIsError = false
		return m, nil

	case msg.FormOptionsMsg:
		if m.formKind == formEditTaskPending && m.pendingEdit != nil {
			m.pendingEdit.parentKey = mssg.ParentKey
			m.form = buildEditForm(m.formTaskKey, m.formTaskProvider, *m.pendingEdit, mssg.Epics, mssg.Sections)
			m.formKind = formEditTask
			return m, nil
		}
		if m.formKind == formAddTaskPending {
			m.formOptions = &mssg
			m.cachedFormOptions = &mssg
			m.form = buildAddForm(mssg.Provider, &mssg)
			m.formKind = formAddTask
			if mssg.Provider == "kendo" {
				project := ""
				if len(mssg.Projects) > 0 {
					project = mssg.Projects[0]
				}
				return m, loadSprintsCmd(m.cb, mssg.Provider, project)
			}
		} else {
			// Background prefetch — just cache it
			m.cachedFormOptions = &mssg
		}
		return m, nil

	case msg.TaskSnoozedMsg:
		m.saving = ""
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Snooze error: %v", mssg.Err), true)
		} else {
			m.setToast(fmt.Sprintf("Snoozed %s", mssg.TaskKey), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TaskViewedMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("View error: %v", mssg.Err), true)
			cmds = append(cmds, clearToastCmd())
		} else {
			m.viewDetail = mssg.Result
		}
		return m, tea.Batch(cmds...)

	case msg.ProjectsLoadedMsg:
		if m.form.Active && m.formKind == formAddTask && m.formOptions != nil {
			m.formOptions.Projects = mssg.Projects
			if len(mssg.Projects) > 0 {
				m.form.UpdateSelectByLabel("Project", mssg.Projects, mssg.Projects[0])
				project := mssg.Projects[0]
				provider := m.form.ValueByLabel("Provider")
				if provider == "" {
					provider = m.formOptions.Provider
				}
				if provider == "kendo" {
					cmds = append(cmds, loadLanesCmd(m.cb, provider, project))
					cmds = append(cmds, loadSprintsCmd(m.cb, provider, project))
				}
				if provider == "jira" {
					cmds = append(cmds, loadIssueTypesCmd(m.cb, provider, project))
				}
			} else {
				m.form.ConvertToTextByLabel("Project", "Project key")
			}
		}
		return m, tea.Batch(cmds...)

	case msg.SectionsLoadedMsg:
		if m.form.Active && (m.formKind == formAddTask || m.formKind == formEditTask) {
			if len(mssg.Sections) > 0 {
				sectionOptions := append([]string{"None"}, mssg.Sections...)
				current := m.form.ValueByLabel("Section")
				if current == "" {
					current = "None"
				}
				m.form.UpdateSelectByLabel("Section", sectionOptions, current)
				m.form.ConvertToSelectByLabel("Section", sectionOptions, current)
			} else {
				m.form.ConvertToTextByLabel("Section", "Section name")
			}
		}
		return m, nil

	case msg.LanesLoadedMsg:
		if m.form.Active && (m.formKind == formAddTask || m.formKind == formEditTask) {
			if len(mssg.Lanes) > 0 {
				current := m.form.ValueByLabel("Issue Type")
				if current == "" {
					current = mssg.Lanes[0]
				}
				m.form.ConvertToSelectByLabel("Issue Type", mssg.Lanes, current)
				m.form.UpdateSelectByLabel("Issue Type", mssg.Lanes, current)
			}
			if m.formOptions != nil {
				m.formOptions.Lanes = mssg.Lanes
			}
		}
		return m, nil

	case msg.SprintsLoadedMsg:
		if m.form.Active && m.formKind == formAddTask {
			if m.formOptions != nil {
				m.formOptions.Sprints = mssg.Sprints
			}
			if len(mssg.Sprints) > 0 {
				sprintOptions := make([]string, 0, len(mssg.Sprints)+1)
				defaultSprint := "Backlog"
				for _, s := range mssg.Sprints {
					sprintOptions = append(sprintOptions, s.Label)
					if s.Active {
						defaultSprint = s.Label
					}
				}
				sprintOptions = append(sprintOptions, "Backlog")
				m.form.ConvertToSelectByLabel("Sprint", sprintOptions, defaultSprint)
				m.form.UpdateSelectByLabel("Sprint", sprintOptions, defaultSprint)
			}
		}
		return m, nil

	case msg.IssueTypesLoadedMsg:
		if m.form.Active && (m.formKind == formAddTask || m.formKind == formEditTask) {
			if len(mssg.IssueTypes) > 0 {
				current := m.form.ValueByLabel("Issue Type")
				if current == "" {
					current = mssg.IssueTypes[0]
				}
				m.form.ConvertToSelectByLabel("Issue Type", mssg.IssueTypes, current)
				m.form.UpdateSelectByLabel("Issue Type", mssg.IssueTypes, current)
			}
			if m.formOptions != nil {
				m.formOptions.IssueTypes = mssg.IssueTypes
			}
		}
		return m, nil

	case msg.EpicsLoadedMsg:
		if m.form.Active && (m.formKind == formAddTask || m.formKind == formEditTask) {
			epicOptions := []string{"None"}
			for _, e := range mssg.Epics {
				epicOptions = append(epicOptions, e.Label)
			}
			m.form.UpdateSelectByLabel("Parent", epicOptions, "None")
		}
		return m, nil

	case msg.WorklogsLoadedMsg:
		m.worklog.Loading = false
		if mssg.Err != nil {
			m.worklog.Err = mssg.Err
		} else {
			m.worklog.Entries = mssg.Entries
			m.worklog.Err = nil
		}
		return m, nil

	case msg.WorklogUpdatedMsg:
		m.saving = ""
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Worklog update error: %v", mssg.Err), true)
		} else {
			m.setToast("Worklog updated", false)
			cmds = append(cmds, loadWorklogsCmd(m.cb, m.worklog.WeekView, m.worklog.Date))
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.WorklogDeletedMsg:
		m.saving = ""
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Worklog delete error: %v", mssg.Err), true)
		} else {
			m.setToast("Worklog deleted", false)
			cmds = append(cmds, loadWorklogsCmd(m.cb, m.worklog.WeekView, m.worklog.Date))
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.WorklogAddedMsg:
		m.saving = ""
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Worklog add error: %v", mssg.Err), true)
		} else {
			m.setToast("Worklog added", false)
			cmds = append(cmds, loadWorklogsCmd(m.cb, m.worklog.WeekView, m.worklog.Date))
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.FallbackLoadedMsg:
		m.cachedFallback = mssg.Issues
		return m, nil

	case msg.AutoRefreshMsg:
		cmds = append(cmds, m.refreshActiveView(), autoRefreshCmd())
		return m, tea.Batch(cmds...)
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateTasks(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Up):
		m.tasks.CursorUp()
	case key.Matches(mssg, keys.Down):
		m.tasks.CursorDown()
	case key.Matches(mssg, keys.Refresh):
		m.tasks.Loading = true
		return m, loadTasksCmd(m.cb)
	case key.Matches(mssg, keys.Enter), key.Matches(mssg, keys.Timer):
		if t := m.tasks.SelectedTask(); t != nil {
			return m, startTimerCmd(m.cb, t.Key, t.Summary, t.Project, t.Section)
		}
	case key.Matches(mssg, keys.Done):
		if t := m.tasks.SelectedTask(); t != nil {
			return m, doneTaskCmd(m.cb, t.Key, t.Provider)
		}
	case key.Matches(mssg, keys.Delete):
		if t := m.tasks.SelectedTask(); t != nil {
			m.confirm = components.NewConfirm(fmt.Sprintf("Delete %s?", t.Key))
			m.confirmType = confirmDeleteTask
			m.confirmKey = t.Key
			m.confirmProvider = t.Provider
		}
	case key.Matches(mssg, keys.Search):
		m.tasks.ToggleFilter()
	case key.Matches(mssg, keys.Add):
		if m.cachedFormOptions != nil {
			m.formOptions = m.cachedFormOptions
			m.form = buildAddForm(m.cachedFormOptions.Provider, m.cachedFormOptions)
			m.formKind = formAddTask
			return m, loadFormOptionsCmd(m.cb) // refresh cache in background
		}
		m.formKind = formAddTaskPending
		return m, loadFormOptionsCmd(m.cb)
	case key.Matches(mssg, keys.Edit):
		if t := m.tasks.SelectedTask(); t != nil {
			ed := buildPendingEditData(t)
			m.formTaskKey = t.Key
			m.formTaskProvider = t.Provider
			m.pendingEdit = &ed
			m.formKind = formEditTaskPending
			return m, loadEditFormOptionsCmd(m.cb, t.Project, t.Key)
		}
	case key.Matches(mssg, keys.Move):
		if t := m.tasks.SelectedTask(); t != nil {
			m.formKind = formMoveTaskPending
			m.formTaskKey = t.Key
			m.formTaskProvider = t.Provider
			return m, listTransitionsCmd(m.cb, t.Key, t.Provider)
		}
	case key.Matches(mssg, keys.Snooze):
		if t := m.tasks.SelectedTask(); t != nil {
			m.form = components.NewForm(fmt.Sprintf("Snooze %s", t.Key), []components.FormFieldDef{
				{Label: "Duration", Placeholder: "e.g. 3d, 1w, Monday"},
			})
			m.formKind = formSnoozeTask
			m.formTaskKey = t.Key
		}
	case key.Matches(mssg, keys.ViewTask):
		if t := m.tasks.SelectedTask(); t != nil {
			return m, viewTaskCmd(m.cb, t.Key)
		}
	}
	return m, nil
}

func (m model) updateTasksFilter(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Escape):
		m.tasks.ClearFilter()
	case key.Matches(mssg, keys.Enter):
		m.tasks.ToggleFilter()
	case mssg.Type == tea.KeyBackspace:
		m.tasks.BackspaceFilter()
	case mssg.Type == tea.KeyRunes:
		for _, r := range mssg.Runes {
			m.tasks.AppendFilter(r)
		}
	}
	return m, nil
}

func (m model) updateConfirm(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Escape):
		m.confirm.Active = false
		m.confirmType = confirmNone
	case mssg.Type == tea.KeyLeft, mssg.Type == tea.KeyRight,
		mssg.Type == tea.KeyTab:
		m.confirm.Toggle()
	case key.Matches(mssg, keys.Enter):
		m.confirm.Active = false
		if m.confirm.Selected {
			switch m.confirmType {
			case confirmDeleteTask:
				m.confirmType = confirmNone
				return m, deleteTaskCmd(m.cb, m.confirmKey, m.confirmProvider)
			case confirmSyncApply:
				m.confirmType = confirmNone
				return m, syncApplyCmd(m.cb, false)
			case confirmSyncForce:
				m.confirmType = confirmNone
				return m, syncApplyCmd(m.cb, true)
			case confirmClearEvents:
				m.confirmType = confirmNone
				return m, clearEventsCmd(m.cb)
			case confirmAbortTimer:
				m.confirmType = confirmNone
				return m, abortTimerCmd(m.cb)
			case confirmDeleteWorklog:
				m.confirmType = confirmNone
				return m, deleteWorklogCmd(m.cb, m.formWorklogKey, m.confirmKey, m.formWorklogProvider)
			}
		}
		m.confirmType = confirmNone
	}
	return m, nil
}

func (m model) updateWorklog(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Up):
		m.worklog.CursorUp()
	case key.Matches(mssg, keys.Down):
		m.worklog.CursorDown()
	case key.Matches(mssg, keys.Refresh):
		m.worklog.Loading = true
		return m, loadWorklogsCmd(m.cb, m.worklog.WeekView, m.worklog.Date)
	case key.Matches(mssg, keys.WeekToggle):
		m.worklog.ToggleWeekView()
		m.worklog.Loading = true
		return m, loadWorklogsCmd(m.cb, m.worklog.WeekView, m.worklog.Date)
	case key.Matches(mssg, keys.DatePrev):
		m.worklog.PrevDate()
		m.worklog.Loading = true
		return m, loadWorklogsCmd(m.cb, m.worklog.WeekView, m.worklog.Date)
	case key.Matches(mssg, keys.DateNext):
		m.worklog.NextDate()
		m.worklog.Loading = true
		return m, loadWorklogsCmd(m.cb, m.worklog.WeekView, m.worklog.Date)
	case key.Matches(mssg, keys.GoToday):
		m.worklog.GoToToday()
		m.worklog.Loading = true
		return m, loadWorklogsCmd(m.cb, m.worklog.WeekView, m.worklog.Date)
	case key.Matches(mssg, keys.Add):
		startedDefault := time.Now().Format("15:04")
		if !m.worklog.IsToday() {
			startedDefault = m.worklog.Date.Format("2006-01-02T") + "09:00"
		}
		issueField := components.FormFieldDef{
			Label:       "Issue Key",
			Placeholder: "e.g. PROJ-123 (/ to search)",
		}
		if len(m.cachedFallback) > 0 {
			options := make([]string, len(m.cachedFallback))
			for i, fb := range m.cachedFallback {
				if fb.Summary != "" {
					options[i] = fb.Key + "  " + fb.Summary
				} else {
					options[i] = fb.Key
				}
			}
			issueField.Kind = components.FieldSelect
			issueField.Options = options
		}
		m.form = components.NewForm("Add Worklog", []components.FormFieldDef{
			issueField,
			{Label: "Duration", Placeholder: "e.g. 1h30m, 45m"},
			{Label: "Description", Placeholder: "What did you work on?"},
			{Label: "Started", Placeholder: "e.g. 09:00, 2006-01-02T15:04", Value: startedDefault},
		})
		m.formKind = formAddWorklog
		m.formWorklogProvider = ""
	case key.Matches(mssg, keys.Edit):
		if e := m.worklog.SelectedEntry(); e != nil {
			m.form = components.NewForm(fmt.Sprintf("Edit Worklog — %s", e.IssueKey), []components.FormFieldDef{
				{Label: "Duration", Placeholder: "e.g. 1h30m, 45m", Value: styles.FormatDuration(e.TimeSpent)},
				{Label: "Description", Placeholder: "What did you work on?", Value: e.Description},
				{Label: "Started", Placeholder: "e.g. 09:00, 2006-01-02T15:04", Value: e.Started.Format("15:04")},
			})
			m.formKind = formEditWorklog
			m.formWorklogID = e.ID
			m.formWorklogKey = e.IssueKey
			m.formWorklogProvider = e.Provider
		}
	case key.Matches(mssg, keys.Delete):
		if e := m.worklog.SelectedEntry(); e != nil {
			m.confirm = components.NewConfirm(fmt.Sprintf("Delete worklog on %s?", e.IssueKey))
			m.confirmType = confirmDeleteWorklog
			m.confirmKey = e.ID
			m.formWorklogKey = e.IssueKey
			m.formWorklogProvider = e.Provider
		}
	}
	return m, nil
}

func (m model) updateConfig(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Up):
		m.config.CursorUp()
	case key.Matches(mssg, keys.Down):
		m.config.CursorDown()
	case key.Matches(mssg, keys.Refresh):
		m.config.Loading = true
		return m, loadConfigCmd(m.cb)
	case key.Matches(mssg, keys.Enter):
		if row := m.config.SelectedRow(); row != nil {
			m.formConfigKey = row.Key
			m.form = components.NewForm("Edit: "+row.Label, []components.FormFieldDef{
				{Label: "Value", Placeholder: "new value", Value: row.Value},
			})
			m.formKind = formSetConfig
		}
	}
	return m, nil
}

func (m model) updateTimer(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Refresh):
		return m, timerStatusCmd(m.cb)
	case key.Matches(mssg, keys.Interrupt):
		if m.timerRunning {
			return m, interruptTimerCmd(m.cb)
		}
	case key.Matches(mssg, keys.Comment):
		if m.timerRunning {
			fields := []components.FormFieldDef{
				{Label: "Comment", Placeholder: "What are you working on?", Value: m.timerComment},
			}
			m.form = components.NewForm("Timer Comment", fields)
			m.formKind = formTimerComment
			return m, nil
		}
	case key.Matches(mssg, keys.EditStart):
		if m.timerRunning {
			fields := []components.FormFieldDef{
				{Label: "Start time", Placeholder: "HH:MM", Value: m.timerStartTime.Local().Format("15:04")},
			}
			m.form = components.NewForm("Edit Start Time", fields)
			m.formKind = formTimerStartTime
			return m, nil
		}
	case key.Matches(mssg, keys.Stop):
		if m.timerRunning {
			// Build form title with task info and elapsed time
			titleLabel := m.timerSummary
			if titleLabel == "" && m.timerKey != "" {
				titleLabel = m.timerKey
			}
			if titleLabel == "" {
				titleLabel = "(anonymous)"
			}
			formTitle := fmt.Sprintf("Stop Timer — %s (%s)", titleLabel, styles.FormatDuration(m.timerElapsed))

			fields := []components.FormFieldDef{
				{Label: "Comment", Placeholder: "What did you work on?", Value: m.timerComment},
				{Label: "Mark done", Kind: components.FieldToggle},
			}
			if needsWorklogIssue(m.timerKey) {
				issueField := components.FormFieldDef{
					Label:       "Issue Key",
					Placeholder: "PROJ-123 (/ to search)",
				}
				if len(m.cachedFallback) > 0 {
					options := make([]string, len(m.cachedFallback))
					for i, fb := range m.cachedFallback {
						if fb.Summary != "" {
							options[i] = fb.Key + "  " + fb.Summary
						} else {
							options[i] = fb.Key
						}
					}
					issueField.Kind = components.FieldSelect
					issueField.Options = options
				}
				fields = append([]components.FormFieldDef{issueField}, fields...)
			} else {
				issueField := components.FormFieldDef{
					Label:       "Issue Key",
					Placeholder: "PROJ-123 (/ to search)",
					Value:       m.timerKey,
				}
				fields = append([]components.FormFieldDef{issueField}, fields...)
			}
			m.form = components.NewForm(formTitle, fields)
			m.formKind = formStopTimer
			// Auto-resolve Jira key from GitHub PR branch name
			if strings.Contains(m.timerKey, "#") {
				return m, resolveJiraKeyCmd(m.cb, m.timerKey)
			}
			return m, nil
		}
		// No timer running — start anonymous timer
		return m, startTimerCmd(m.cb, "", "", "", "")
	case key.Matches(mssg, keys.Abort):
		if m.timerRunning {
			m.confirm = components.NewConfirm("Abort timer? Work will not be logged.")
			m.confirmType = confirmAbortTimer
			return m, nil
		}
	}
	return m, nil
}

func (m *model) openTaskPicker(tasks []msg.ScoredTask, returnKind formKind) {
	var items []components.PickerItem
	if returnKind == formStopTimer || returnKind == formAddWorklog {
		for _, fb := range m.cachedFallback {
			label := fb.Key
			if fb.Summary != "" {
				label = fmt.Sprintf("%-10s  %s", fb.Key, fb.Summary)
			}
			items = append(items, components.PickerItem{
				Key:   fb.Key,
				Label: label,
			})
		}
	}
	for _, t := range tasks {
		if (returnKind == formStopTimer || returnKind == formAddWorklog) && !isJiraKeyPattern(t.Key) {
			continue
		}
		items = append(items, components.PickerItem{
			Key:      t.Key,
			Label:    fmt.Sprintf("%-10s  %s", t.Key, t.Summary),
			Provider: t.Provider,
		})
	}
	m.picker = components.NewPicker("Search Tasks (Enter to select, Esc to cancel)", items)
	m.pickerFieldLabel = "Issue Key"
	m.formKind = returnKind
}

func (m *model) rebuildMyTaskPickerItems() {
	var items []components.PickerItem
	for _, fb := range m.cachedFallback {
		label := fb.Key
		if fb.Summary != "" {
			label = fmt.Sprintf("%-10s  %s", fb.Key, fb.Summary)
		}
		items = append(items, components.PickerItem{
			Key:   fb.Key,
			Label: label,
		})
	}
	tasks := m.cachedTasks
	if tasks == nil {
		tasks = []msg.ScoredTask{}
	}
	for _, t := range tasks {
		if !isJiraKeyPattern(t.Key) {
			continue
		}
		items = append(items, components.PickerItem{
			Key:      t.Key,
			Label:    fmt.Sprintf("%-10s  %s", t.Key, t.Summary),
			Provider: t.Provider,
		})
	}
	filter := m.picker.Filter.Value()
	m.picker.Items = items
	m.picker.Filter.SetValue(filter)
	m.picker.ResetCursor()
}

func (m *model) rebuildPickerItems(tasks []msg.ScoredTask) {
	var items []components.PickerItem
	for _, t := range tasks {
		items = append(items, components.PickerItem{
			Key:      t.Key,
			Label:    fmt.Sprintf("%-10s  %s", t.Key, t.Summary),
			Provider: t.Provider,
		})
	}
	filter := m.picker.Filter.Value()
	m.picker.Items = items
	m.picker.Filter.SetValue(filter)
	m.picker.ResetCursor()
}

func (m *model) openPickerForSelect() {
	label := m.form.FocusedLabel()
	opts := m.form.FocusedSelectOptions()
	items := make([]components.PickerItem, len(opts))
	for i, opt := range opts {
		items[i] = components.PickerItem{Key: opt, Label: opt}
	}
	m.picker = components.NewPicker("Select "+label+" (Enter to select, Esc to cancel)", items)
	m.pickerFieldLabel = label
}

func (m model) pickerSideEffect(label string) tea.Cmd {
	switch label {
	case "Project":
		project := m.form.ValueByLabel("Project")
		provider := m.form.ValueByLabel("Provider")
		if provider == "" && m.formOptions != nil {
			provider = m.formOptions.Provider
		}
		cmds := []tea.Cmd{loadEpicsCmd(m.cb, provider, project), loadSectionsCmd(m.cb, provider, project)}
		if provider == "kendo" {
			cmds = append(cmds, loadLanesCmd(m.cb, provider, project))
			cmds = append(cmds, loadSprintsCmd(m.cb, provider, project))
		}
		if provider == "jira" {
			cmds = append(cmds, loadIssueTypesCmd(m.cb, provider, project))
		}
		return tea.Batch(cmds...)
	}
	return nil
}

func (m model) updatePicker(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Escape):
		m.picker.Active = false
		// Return to the form underneath
		return m, nil
	case key.Matches(mssg, keys.Up):
		m.picker.CursorUp()
		return m, nil
	case key.Matches(mssg, keys.Down):
		m.picker.CursorDown()
		return m, nil
	case key.Matches(mssg, keys.Enter):
		selected := m.picker.Selected()
		m.picker.Active = false
		if selected != nil && m.pickerFieldLabel == "move" {
			return m, moveTaskCmd(m.cb, m.formTaskKey, m.formTaskProvider, selected.Key)
		}
		if selected != nil {
			m.form.SetValueByLabel(m.pickerFieldLabel, selected.Key)
			if m.pickerFieldLabel == "Issue Key" && (m.formKind == formAddWorklog || m.formKind == formStopTimer) {
				m.formWorklogProvider = selected.Provider
			}
		}
		if m.pickerFieldLabel == "Provider" && m.formKind == formAddTask && m.formOptions != nil {
			cmd := m.rebuildAddFormForProvider()
			return m, cmd
		}
		return m, m.pickerSideEffect(m.pickerFieldLabel)
	case mssg.Type == tea.KeyTab && m.pickerFieldLabel == "Issue Key":
		if m.picker.Mode == components.PickerModeMyTasks {
			m.picker.Mode = components.PickerModeAllTasks
			m.picker.Items = nil
			m.picker.ResetCursor()
			// If there's already text, trigger a search immediately
			if q := m.picker.Filter.Value(); q != "" {
				m.picker.Loading = true
				return m, searchAllTasksCmd(m.cb, q)
			}
			return m, nil
		}
		m.picker.Mode = components.PickerModeMyTasks
		m.rebuildMyTaskPickerItems()
		return m, nil
	default:
		var cmd tea.Cmd
		m.picker.Filter, cmd = m.picker.Filter.Update(mssg)
		m.picker.ResetCursor()
		// In all-tasks mode, debounce server-side search on each keystroke
		if m.picker.Mode == components.PickerModeAllTasks && m.pickerFieldLabel == "Issue Key" {
			q := m.picker.Filter.Value()
			return m, tea.Batch(cmd, pickerSearchDebounceCmd(q))
		}
		return m, cmd
	}
}

func (m model) updateForm(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.saving != "" {
		return m, nil
	}
	m.form.Error = ""

	// "/" on Issue Key field opens the task picker
	if (m.formKind == formAddWorklog || m.formKind == formStopTimer) && m.form.FocusedLabel() == "Issue Key" && key.Matches(mssg, keys.Search) {
		returnKind := formAddWorklog
		if m.formKind == formStopTimer {
			returnKind = formStopTimer
		}
		if m.cachedTasks != nil {
			m.openTaskPicker(m.cachedTasks, returnKind)
			return m, loadTasksCmd(m.cb) // refresh cache in background
		}
		if m.formKind == formAddWorklog {
			m.formKind = formAddWorklogPending
		} else {
			m.formKind = formStopTimerPending
		}
		return m, loadTasksCmd(m.cb)
	}

	// "/" on any select field with many options opens the picker
	if key.Matches(mssg, keys.Search) && m.form.IsSelectField() {
		m.openPickerForSelect()
		return m, nil
	}

	switch {
	case key.Matches(mssg, keys.Escape):
		m.form.Active = false
		m.formKind = formNone
		m.formOptions = nil
		return m, nil
	case mssg.Type == tea.KeyTab:
		m.form.FocusNext()
		return m, nil
	case mssg.Type == tea.KeyShiftTab:
		m.form.FocusPrev()
		return m, nil
	case mssg.Type == tea.KeyLeft:
		if m.form.IsSelectField() {
			m.form.CycleSelectLeft()
			if m.form.FocusedLabel() == "Provider" && m.formKind == formAddTask && m.formOptions != nil {
				return m, m.rebuildAddFormForProvider()
			}
			if m.form.FocusedLabel() == "Project" {
				project := m.form.ValueByLabel("Project")
				provider := m.form.ValueByLabel("Provider")
				if provider == "" && m.formOptions != nil {
					provider = m.formOptions.Provider
				}
				cmds := []tea.Cmd{loadEpicsCmd(m.cb, provider, project), loadSectionsCmd(m.cb, provider, project)}
				if provider == "kendo" {
					cmds = append(cmds, loadLanesCmd(m.cb, provider, project))
					cmds = append(cmds, loadSprintsCmd(m.cb, provider, project))
				}
				if provider == "jira" {
					cmds = append(cmds, loadIssueTypesCmd(m.cb, provider, project))
				}
				return m, tea.Batch(cmds...)
			}
			return m, nil
		}
		if m.form.IsToggleField() {
			m.form.ToggleValue()
			return m, nil
		}
		if ti := m.form.FocusedTextInput(); ti != nil {
			var cmd tea.Cmd
			*ti, cmd = ti.Update(mssg)
			return m, cmd
		}
		return m, nil
	case mssg.Type == tea.KeyRight:
		if m.form.IsSelectField() {
			m.form.CycleSelectRight()
			if m.form.FocusedLabel() == "Provider" && m.formKind == formAddTask && m.formOptions != nil {
				return m, m.rebuildAddFormForProvider()
			}
			if m.form.FocusedLabel() == "Project" {
				project := m.form.ValueByLabel("Project")
				provider := m.form.ValueByLabel("Provider")
				if provider == "" && m.formOptions != nil {
					provider = m.formOptions.Provider
				}
				cmds := []tea.Cmd{loadEpicsCmd(m.cb, provider, project), loadSectionsCmd(m.cb, provider, project)}
				if provider == "kendo" {
					cmds = append(cmds, loadLanesCmd(m.cb, provider, project))
					cmds = append(cmds, loadSprintsCmd(m.cb, provider, project))
				}
				if provider == "jira" {
					cmds = append(cmds, loadIssueTypesCmd(m.cb, provider, project))
				}
				return m, tea.Batch(cmds...)
			}
			return m, nil
		}
		if m.form.IsToggleField() {
			m.form.ToggleValue()
			return m, nil
		}
		if ti := m.form.FocusedTextInput(); ti != nil {
			var cmd tea.Cmd
			*ti, cmd = ti.Update(mssg)
			return m, cmd
		}
		return m, nil
	case mssg.Type == tea.KeySpace:
		if m.form.IsToggleField() {
			m.form.ToggleValue()
			return m, nil
		}
		if ti := m.form.FocusedTextInput(); ti != nil {
			var cmd tea.Cmd
			*ti, cmd = ti.Update(mssg)
			return m, cmd
		}
		return m, nil
	case key.Matches(mssg, keys.Enter):
		vals := m.form.Values()
		switch m.formKind {
		case formAddTask:
			summary := m.form.ValueByLabel("Summary")
			if summary == "" {
				m.form.Active = false
				m.formKind = formNone
				return m, nil
			}
			if nb := m.form.ValueByLabel("Not Before"); nb != "" {
				summary += " not before " + nb
			}
			if m.form.ValueByLabel("Up Next") == "true" {
				summary += " upnext"
			}
			if m.form.ValueByLabel("No Split") == "true" {
				summary += " nosplit"
			}
			parent := m.form.ValueByLabel("Parent")
			if parent == "None" || parent == "" {
				parent = ""
			} else {
				// Extract key from "KEY — Summary" label
				if idx := strings.Index(parent, " — "); idx > 0 {
					parent = parent[:idx]
				}
			}
			provider := m.form.ValueByLabel("Provider")
			if provider == "" && m.formOptions != nil {
				provider = m.formOptions.Provider
			}
			section := m.form.ValueByLabel("Section")
			if section == "None" {
				section = ""
			}
			var sprintID *int
			if sprintLabel := m.form.ValueByLabel("Sprint"); sprintLabel != "" && sprintLabel != "Backlog" {
				if opts := m.formOptions; opts != nil {
					for _, s := range opts.Sprints {
						if s.Label == sprintLabel {
							id := s.ID
							sprintID = &id
							break
						}
					}
				}
			}
			m.saving = "Adding task"
			return m, addTaskCmd(m.cb, provider, summary,
				m.form.ValueByLabel("Project"), section,
				m.form.ValueByLabel("Issue Type"),
				m.form.ValueByLabel("Description"),
				m.form.ValueByLabel("Estimate"),
				m.form.ValueByLabel("Due Date"),
				m.form.ValueByLabel("Priority"),
				parent,
				sprintID,
			)
		case formEditTask:
			upNextVal := m.form.ValueByLabel("Up Next")
			var upNext *bool
			if upNextVal == "true" {
				v := true
				upNext = &v
			} else {
				v := false
				upNext = &v
			}
			noSplitVal := m.form.ValueByLabel("No Split")
			var noSplit *bool
			if noSplitVal == "true" {
				v := true
				noSplit = &v
			} else {
				v := false
				noSplit = &v
			}
			parent := m.form.ValueByLabel("Parent")
			if parent == "None" || parent == "" {
				parent = ""
			} else {
				if idx := strings.Index(parent, " — "); idx > 0 {
					parent = parent[:idx]
				}
			}
			editSection := m.form.ValueByLabel("Section")
			if editSection == "None" {
				editSection = ""
			}
			hadNotBefore := m.pendingEdit != nil && m.pendingEdit.notBefore != ""
			hadDue := m.pendingEdit != nil && m.pendingEdit.dueDate != ""
			hadEstimate := m.pendingEdit != nil && m.pendingEdit.estimate != ""
			hadPriority := m.pendingEdit != nil && m.pendingEdit.priority != ""
			hadParent := m.pendingEdit != nil && m.pendingEdit.parentKey != ""
			hadSection := m.pendingEdit != nil && m.pendingEdit.section != ""
			priority := m.form.ValueByLabel("Priority")
			if priority == "None" {
				priority = ""
			}
			m.saving = "Saving task"
			return m, editTaskCmd(m.cb, EditTaskParams{
				TaskKey:      m.formTaskKey,
				Provider:     m.formTaskProvider,
				Summary:      m.form.ValueByLabel("Summary"),
				Estimate:     m.form.ValueByLabel("Estimate"),
				Due:          m.form.ValueByLabel("Due Date"),
				Priority:     priority,
				UpNext:       upNext,
				NoSplit:      noSplit,
				NotBefore:    m.form.ValueByLabel("Not Before"),
				HadNotBefore: hadNotBefore,
				Parent:       parent,
				Section:      editSection,
				HadDue:       hadDue,
				HadEstimate:  hadEstimate,
				HadPriority:  hadPriority,
				HadParent:    hadParent,
				HadSection:   hadSection,
			})
		case formSnoozeTask:
			duration := m.form.ValueByLabel("Duration")
			if duration == "" {
				m.form.Active = false
				m.formKind = formNone
				return m, nil
			}
			m.form.Active = false
			m.formKind = formNone
			m.saving = "Snoozing task"
			return m, snoozeTaskCmd(m.cb, m.formTaskKey, duration)
		case formTimerComment:
			comment := m.form.ValueByLabel("Comment")
			m.timerComment = comment
			m.timer.Comment = comment
			m.form.Active = false
			m.formKind = formNone
			m.saving = "Saving comment"
			return m, saveTimerCommentCmd(m.cb, comment)
		case formTimerStartTime:
			raw := m.form.ValueByLabel("Start time")
			m.form.Active = false
			m.formKind = formNone
			parsed, err := time.ParseInLocation("15:04", raw, time.Now().Location())
			if err != nil {
				m.setToast(fmt.Sprintf("Invalid time: %v", err), true)
				return m, clearToastCmd()
			}
			now := time.Now()
			newStart := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
			m.saving = "Saving start time"
			return m, saveTimerStartTimeCmd(m.cb, newStart)
		case formStopTimer:
			comment := m.form.ValueByLabel("Comment")
			done := m.form.ValueByLabel("Mark done") == "true"
			fallbackIssue := extractIssueKey(m.form.ValueByLabel("Issue Key"))
			m.form.Active = false
			m.formKind = formNone
			m.saving = "Stopping timer"
			return m, stopTimerCmd(m.cb, comment, done, fallbackIssue, m.formWorklogProvider)
		case formAddWorklog:
			issueKey := extractIssueKey(m.form.ValueByLabel("Issue Key"))
			durationStr := m.form.ValueByLabel("Duration")
			description := m.form.ValueByLabel("Description")
			startedStr := m.form.ValueByLabel("Started")
			if issueKey == "" || durationStr == "" {
				m.form.Active = false
				m.formKind = formNone
				return m, nil
			}
			dur := parseDurationInput(durationStr)
			started := parseStartedInput(startedStr, m.worklog.Date)
			m.form.Active = false
			m.formKind = formNone
			m.saving = "Adding worklog"
			return m, addWorklogCmd(m.cb, issueKey, m.formWorklogProvider, dur, description, started)
		case formEditWorklog:
			durationStr := m.form.ValueByLabel("Duration")
			description := m.form.ValueByLabel("Description")
			startedStr := m.form.ValueByLabel("Started")
			if durationStr == "" {
				m.form.Active = false
				m.formKind = formNone
				return m, nil
			}
			dur := parseDurationInput(durationStr)
			started := parseStartedInput(startedStr, m.worklog.Date)
			m.form.Active = false
			m.formKind = formNone
			m.saving = "Updating worklog"
			return m, updateWorklogCmd(m.cb, m.formWorklogKey, m.formWorklogID, m.formWorklogProvider, dur, description, started)
		case formSetConfig:
			cfgVal := vals[0]
			m.form.Active = false
			m.formKind = formNone
			m.saving = "Saving config"
			return m, setConfigCmd(m.cb, m.formConfigKey, cfgVal)
		}
		m.form.Active = false
		m.formKind = formNone
		return m, nil
	default:
		// Pass key to focused text input
		if ti := m.form.FocusedTextInput(); ti != nil {
			var cmd tea.Cmd
			*ti, cmd = ti.Update(mssg)
			return m, cmd
		}
		return m, nil
	}
}

func (m *model) switchTab(tab int) (tea.Model, tea.Cmd) {
	m.activeTab = tab
	// Serve cached tasks instantly, then refresh in background
	if tab == tabTasks && m.cachedTasks != nil {
		m.tasks.Tasks = m.cachedTasks
		m.tasks.Loading = false
		m.tasks.Err = nil
		return *m, loadTasksCmd(m.cb) // background refresh
	}
	// Keep showing current timer state, refresh in background
	if tab == tabTimer && m.timer.Running {
		m.timer.Loading = false
		return *m, timerStatusCmd(m.cb)
	}
	return *m, m.refreshActiveView()
}

func (m model) refreshActiveView() tea.Cmd {
	switch m.activeTab {
	case tabTasks:
		return loadTasksCmd(m.cb)
	case tabTimer:
		return timerStatusCmd(m.cb)
	case tabWorklog:
		return loadWorklogsCmd(m.cb, m.worklog.WeekView, m.worklog.Date)
	case tabConfig:
		return loadConfigCmd(m.cb)
	}
	return nil
}

func (m *model) setToast(text string, isError bool) {
	m.toast = text
	m.toastIsError = isError
}

func (m model) isLoading() bool {
	switch m.activeTab {
	case tabTasks:
		if m.tasks.Loading {
			return true
		}
	case tabTimer:
		if m.timer.Loading {
			return true
		}
	case tabWorklog:
		if m.worklog.Loading {
			return true
		}
	case tabConfig:
		if m.config.Loading {
			return true
		}
	}
	return m.formKind == formAddTaskPending || m.formKind == formEditTaskPending || m.formKind == formAddWorklogPending || m.formKind == formStopTimerPending || m.formKind == formMoveTaskPending
}

func (m model) loadingLabel() string {
	switch m.activeTab {
	case tabTasks:
		if m.tasks.Loading {
			return "Loading tasks"
		}
	case tabTimer:
		if m.timer.Loading {
			return "Loading timer"
		}
	case tabWorklog:
		if m.worklog.Loading {
			return "Loading worklogs"
		}
	case tabConfig:
		if m.config.Loading {
			return "Loading config"
		}
	}
	if m.formKind == formAddTaskPending {
		return "Loading form options"
	}
	if m.formKind == formEditTaskPending {
		return "Loading edit options"
	}
	if m.formKind == formAddWorklogPending || m.formKind == formStopTimerPending {
		return "Loading tasks"
	}
	if m.formKind == formMoveTaskPending {
		return "Loading transitions"
	}
	if m.saving != "" {
		return m.saving
	}
	return ""
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Picker overlay
	if m.picker.Active {
		return m.picker.View(m.width, m.height)
	}

	// Form overlay
	if m.form.Active {
		return m.form.View(m.width, m.height)
	}

	// Confirm dialog overlay
	if m.confirm.Active {
		return m.confirm.View(m.width, m.height)
	}

	if m.viewDetail != nil {
		return m.renderViewDetail()
	}

	if m.showHelp {
		return m.renderHelp()
	}

	tabBar := components.RenderTabBar(components.TabNames(), m.activeTab, m.width)

	contentHeight := m.height - lipgloss.Height(tabBar) - 3
	var content string
	switch m.activeTab {
	case tabTasks:
		content = m.tasks.View()
	case tabTimer:
		content = m.timer.View()
	case tabWorklog:
		content = m.worklog.View()
	case tabConfig:
		content = m.config.View()
	}

	contentArea := lipgloss.NewStyle().
		Height(contentHeight).
		MaxHeight(contentHeight).
		Width(m.width).
		Render(content)

	hints := "1-4:tabs  ?:help  q:quit"
	var loadingText string
	if label := m.loadingLabel(); label != "" {
		loadingText = m.spinner.View() + " " + label
	}
	statusBar := components.StatusBar{
		TimerKey:     m.timerKey,
		TimerSummary: m.timerSummary,
		TimerElapsed: m.timer.TotalElapsed,
		TimerRunning: m.timerRunning,
		Toast:        m.toast,
		ToastIsError: m.toastIsError,
		HelpHints:    hints,
		Width:        m.width,
		LoadingText:  loadingText,
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		tabBar,
		contentArea,
		statusBar.Render(),
	)
}

func (m model) renderHelp() string {
	bold := lipgloss.NewStyle().Bold(true)
	hint := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})

	var b strings.Builder
	b.WriteString(bold.Render("Keyboard Shortcuts") + "\n\n")

	b.WriteString(bold.Render("Global") + "\n")
	b.WriteString("  1-4           Switch tabs\n")
	b.WriteString("  Tab           Next tab\n")
	b.WriteString("  Shift+Tab     Previous tab\n")
	b.WriteString("  q/Ctrl+C      Quit\n")
	b.WriteString("  ?             Toggle help\n")
	b.WriteString("  Esc           Close overlay\n")
	b.WriteString("  r             Refresh\n\n")

	b.WriteString(bold.Render("Navigation") + "\n")
	b.WriteString("  j/k/arrows    Move cursor\n")
	b.WriteString("  Enter         Primary action\n\n")

	b.WriteString(bold.Render("Tasks") + "\n")
	b.WriteString("  a             Add task\n")
	b.WriteString("  e             Edit task\n")
	b.WriteString("  d             Mark done\n")
	b.WriteString("  D             Delete\n")
	b.WriteString("  S             Snooze\n")
	b.WriteString("  v             View details\n")
	b.WriteString("  t             Start timer\n")
	b.WriteString("  /             Search\n\n")

	b.WriteString(bold.Render("Timer") + "\n")
	b.WriteString("  s             Stop timer / start anonymous\n")
	b.WriteString("  x             Abort timer\n")
	b.WriteString("  i             Interrupt (pause + new timer)\n")
	b.WriteString("  c             Set comment\n\n")

	b.WriteString(bold.Render("Worklog") + "\n")
	b.WriteString("  a             Add worklog\n")
	b.WriteString("  e             Edit worklog\n")
	b.WriteString("  D             Delete worklog\n")
	b.WriteString("  w             Toggle week view\n\n")

	b.WriteString(bold.Render("Config") + "\n")
	b.WriteString("  e             Edit value\n\n")

	b.WriteString(hint.Render("Press ? or Esc to close"))

	content := lipgloss.NewStyle().
		Padding(1, 3).
		Render(b.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m model) renderViewDetail() string {
	bold := lipgloss.NewStyle().Bold(true)
	hint := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})

	v := m.viewDetail
	var b strings.Builder
	b.WriteString(bold.Render(fmt.Sprintf("Task %s", v.Key)) + "\n\n")
	b.WriteString(fmt.Sprintf("  Summary:    %s\n", v.Summary))
	if v.Priority >= 1 && v.Priority <= 5 {
		name := styles.PriorityName(v.Priority)
		b.WriteString(fmt.Sprintf("  Priority:   %s\n", name))
	}
	if v.Estimate > 0 {
		b.WriteString(fmt.Sprintf("  Estimate:   %s\n", styles.FormatDuration(v.Estimate)))
	} else {
		b.WriteString("  Estimate:   none\n")
	}
	if v.DueDate != nil {
		b.WriteString(fmt.Sprintf("  Due:        %s\n", v.DueDate.Format("Mon Jan 2, 2006")))
	} else {
		b.WriteString("  Due:        none\n")
	}
	if v.NotBefore != nil {
		b.WriteString(fmt.Sprintf("  Not Before: %s\n", v.NotBefore.Format("Mon Jan 2, 2006")))
	}
	if v.UpNext {
		b.WriteString("  Up Next:    yes\n")
	}
	if v.NoSplit {
		b.WriteString("  No Split:   yes\n")
	}
	b.WriteString("\n")
	b.WriteString(hint.Render("Press any key to close"))

	content := lipgloss.NewStyle().Padding(1, 3).Render(b.String())
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}).
		Padding(1, 2).
		Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func ptrConfig(m configView.Model) *configView.Model { return &m }

var jiraKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

func isJiraKeyPattern(key string) bool {
	return jiraKeyPattern.MatchString(key)
}

func needsWorklogIssue(key string) bool {
	return key == "" || !isJiraKeyPattern(key)
}

func extractIssueKey(val string) string {
	if i := strings.IndexByte(val, ' '); i > 0 {
		return val[:i]
	}
	return val
}

func buildPendingEditData(t *msg.ScoredTask) pendingEditData {
	ed := pendingEditData{}
	ed.section = t.Section
	if t.Estimate > 0 {
		h := int(t.Estimate.Hours())
		mins := int(t.Estimate.Minutes()) % 60
		if h > 0 && mins > 0 {
			ed.estimate = fmt.Sprintf("%dh%dm", h, mins)
		} else if h > 0 {
			ed.estimate = fmt.Sprintf("%dh", h)
		} else {
			ed.estimate = fmt.Sprintf("%dm", mins)
		}
	}
	if t.DueDate != nil {
		ed.dueDate = t.DueDate.Format("2006-01-02")
	}
	ed.priority = styles.PriorityName(t.Priority)
	ed.notBefore = t.NotBeforeRaw
	if ed.notBefore == "" && t.NotBefore != nil {
		ed.notBefore = t.NotBefore.Format("2006-01-02")
	}
	ed.summary = stripConstraints(t.Summary)
	ed.upNext = "false"
	if t.UpNext {
		ed.upNext = "true"
	}
	ed.noSplit = "false"
	if t.NoSplit {
		ed.noSplit = "true"
	}
	return ed
}

func buildAddForm(provider string, opts *msg.FormOptionsMsg) components.Form {
	var fields []components.FormFieldDef
	if len(opts.Providers) > 1 {
		fields = append(fields, components.FormFieldDef{
			Label: "Provider", Kind: components.FieldSelect,
			Options: opts.Providers, Value: provider,
		})
	}
	projectField := components.FormFieldDef{Label: "Project", Placeholder: "Project key"}
	if len(opts.Projects) > 0 {
		projectField = components.FormFieldDef{
			Label:   "Project",
			Kind:    components.FieldSelect,
			Options: opts.Projects,
			Value:   opts.Projects[0],
		}
	}
	fields = append(fields,
		components.FormFieldDef{Label: "Summary", Placeholder: "Task summary"},
		projectField,
	)
	if provider == "jira" {
		if len(opts.IssueTypes) > 0 {
			fields = append(fields, components.FormFieldDef{
				Label: "Issue Type", Kind: components.FieldSelect,
				Options: opts.IssueTypes, Value: opts.IssueTypes[0],
			})
		} else {
			fields = append(fields, components.FormFieldDef{
				Label: "Issue Type", Placeholder: "Loading issue types...",
			})
		}
	} else if provider == "kendo" {
		if len(opts.Lanes) > 0 {
			fields = append(fields, components.FormFieldDef{
				Label: "Issue Type", Kind: components.FieldSelect,
				Options: opts.Lanes, Value: opts.Lanes[0],
			})
		} else {
			fields = append(fields, components.FormFieldDef{
				Label: "Issue Type", Placeholder: "Loading lanes...",
			})
		}
		if len(opts.Sprints) > 0 {
			sprintOptions := make([]string, 0, len(opts.Sprints)+1)
			defaultSprint := "Backlog"
			for _, s := range opts.Sprints {
				sprintOptions = append(sprintOptions, s.Label)
				if s.Active {
					defaultSprint = s.Label
				}
			}
			sprintOptions = append(sprintOptions, "Backlog")
			fields = append(fields, components.FormFieldDef{
				Label: "Sprint", Kind: components.FieldSelect,
				Options: sprintOptions, Value: defaultSprint,
			})
		} else {
			fields = append(fields, components.FormFieldDef{
				Label: "Sprint", Kind: components.FieldSelect,
				Options: []string{"Backlog"}, Value: "Backlog",
			})
		}
	}
	if provider != "jira" && provider != "kendo" && provider != "github" {
		if len(opts.Sections) > 0 {
			sectionOptions := append([]string{"None"}, opts.Sections...)
			fields = append(fields, components.FormFieldDef{
				Label: "Section", Kind: components.FieldSelect,
				Options: sectionOptions, Value: "None",
			})
		} else {
			fields = append(fields, components.FormFieldDef{Label: "Section", Placeholder: "Section name"})
		}
	}
	fields = append(fields,
		components.FormFieldDef{Label: "Description", Placeholder: "Description"},
		components.FormFieldDef{Label: "Estimate", Placeholder: "e.g. 2h, 30m"},
		components.FormFieldDef{Label: "Due Date", Placeholder: "e.g. YYYY-MM-DD"},
		components.FormFieldDef{Label: "Priority", Kind: components.FieldSelect, Options: []string{"Highest", "High", "Medium", "Low", "Lowest"}, Value: "Medium"},
	)
	if provider == "jira" || provider == "kendo" {
		epicOptions := []string{"None"}
		for _, e := range opts.Epics {
			epicOptions = append(epicOptions, e.Label)
		}
		fields = append(fields, components.FormFieldDef{
			Label: "Parent", Kind: components.FieldSelect,
			Options: epicOptions, Value: "None",
		})
	}
	fields = append(fields,
		components.FormFieldDef{Label: "Up Next", Kind: components.FieldToggle},
		components.FormFieldDef{Label: "No Split", Kind: components.FieldToggle},
		components.FormFieldDef{Label: "Not Before", Placeholder: "e.g. -3d, 2025-03-01"},
	)
	return components.NewForm("Add Task", fields)
}

func (m *model) rebuildAddFormForProvider() tea.Cmd {
	newProvider := m.form.ValueByLabel("Provider")
	// Preserve user-entered values
	preserved := map[string]string{
		"Summary":     m.form.ValueByLabel("Summary"),
		"Description": m.form.ValueByLabel("Description"),
		"Estimate":    m.form.ValueByLabel("Estimate"),
		"Due Date":    m.form.ValueByLabel("Due Date"),
		"Priority":    m.form.ValueByLabel("Priority"),
		"Section":     m.form.ValueByLabel("Section"),
		"Up Next":     m.form.ValueByLabel("Up Next"),
		"No Split":    m.form.ValueByLabel("No Split"),
		"Not Before":  m.form.ValueByLabel("Not Before"),
	}
	m.form = buildAddForm(newProvider, m.formOptions)
	for label, val := range preserved {
		if val != "" {
			m.form.SetValueByLabel(label, val)
		}
	}
	// Focus the Provider field after rebuild
	m.form.FocusByLabel("Provider")
	var cmds []tea.Cmd
	cmds = append(cmds, loadProjectsCmd(m.cb, newProvider))
	if newProvider == "jira" || newProvider == "kendo" {
		cmds = append(cmds, loadEpicsCmd(m.cb, newProvider, m.form.ValueByLabel("Project")))
	}
	if newProvider == "kendo" {
		cmds = append(cmds, loadLanesCmd(m.cb, newProvider, m.form.ValueByLabel("Project")))
		cmds = append(cmds, loadSprintsCmd(m.cb, newProvider, m.form.ValueByLabel("Project")))
	}
	if newProvider == "jira" {
		cmds = append(cmds, loadIssueTypesCmd(m.cb, newProvider, m.form.ValueByLabel("Project")))
	}
	cmds = append(cmds, loadSectionsCmd(m.cb, newProvider, m.form.ValueByLabel("Project")))
	return tea.Batch(cmds...)
}

func buildEditForm(taskKey, provider string, ed pendingEditData, epics []msg.EpicOption, sections []string) components.Form {
	fields := []components.FormFieldDef{
		{Label: "Provider", Value: provider, Disabled: true},
		{Label: "Summary", Placeholder: "Task summary", Value: ed.summary},
		{Label: "Estimate", Placeholder: "e.g. 2h, 30m", Value: ed.estimate},
		{Label: "Due Date", Placeholder: "e.g. YYYY-MM-DD", Value: ed.dueDate},
		{Label: "Priority", Kind: components.FieldSelect, Options: []string{"None", "Highest", "High", "Medium", "Low", "Lowest"}, Value: ed.priority},
	}
	if epics != nil {
		epicOptions := []string{"None"}
		currentVal := "None"
		for _, e := range epics {
			epicOptions = append(epicOptions, e.Label)
			if e.Key == ed.parentKey {
				currentVal = e.Label
			}
		}
		fields = append(fields, components.FormFieldDef{
			Label: "Parent", Kind: components.FieldSelect,
			Options: epicOptions, Value: currentVal,
		})
	}
	if sections != nil {
		sectionOptions := []string{"None"}
		sectionOptions = append(sectionOptions, sections...)
		currentVal := "None"
		if ed.section != "" {
			for _, s := range sections {
				if s == ed.section {
					currentVal = ed.section
					break
				}
			}
			if currentVal == "None" {
				// Section exists but not in the list; add it
				sectionOptions = append(sectionOptions, ed.section)
				currentVal = ed.section
			}
		}
		fields = append(fields, components.FormFieldDef{
			Label: "Section", Kind: components.FieldSelect,
			Options: sectionOptions, Value: currentVal,
		})
	} else if epics == nil {
		// No sections loaded and not Jira — show a text input
		fields = append(fields, components.FormFieldDef{
			Label: "Section", Placeholder: "Section name", Value: ed.section,
		})
	}
	fields = append(fields,
		components.FormFieldDef{Label: "Up Next", Kind: components.FieldToggle, Value: ed.upNext},
		components.FormFieldDef{Label: "No Split", Kind: components.FieldToggle, Value: ed.noSplit},
		components.FormFieldDef{Label: "Not Before", Placeholder: "e.g. -3d, 2025-03-01", Value: ed.notBefore},
	)
	return components.NewForm(fmt.Sprintf("Edit %s", taskKey), fields)
}

func stripConstraints(summary string) string {
	s := regexp.MustCompile(`(?i)\bupnext\b`).ReplaceAllString(summary, "")
	s = regexp.MustCompile(`(?i)\bnosplit\b`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)\bnot before \S+`).ReplaceAllString(s, "")
	s = strings.TrimSpace(regexp.MustCompile(`\s{2,}`).ReplaceAllString(s, " "))
	return s
}

func parseDurationInput(s string) time.Duration {
	s = strings.TrimSpace(s)
	// Try standard Go duration first (e.g. "1h30m", "45m")
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	// Try "Xh Ym" format with space
	s = strings.ReplaceAll(s, " ", "")
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return 0
}

func parseStartedInput(s string, refDate time.Time) time.Time {
	s = strings.TrimSpace(s)
	now := time.Now()

	// Try full datetime
	if t, err := time.ParseInLocation("2006-01-02T15:04", s, now.Location()); err == nil {
		return t
	}
	// Try time only (HH:MM) — use reference date
	if t, err := time.Parse("15:04", s); err == nil {
		return time.Date(refDate.Year(), refDate.Month(), refDate.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	}
	return now
}

// Run starts the TUI application.
func Run(deps Deps) error {
	p := tea.NewProgram(
		initialModel(deps),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
