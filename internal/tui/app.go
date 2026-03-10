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
	configView "github.com/iruoy/fylla/internal/tui/views/config"
	"github.com/iruoy/fylla/internal/tui/views/schedule"
	"github.com/iruoy/fylla/internal/tui/views/tasks"
	"github.com/iruoy/fylla/internal/tui/views/timeline"
	timerView "github.com/iruoy/fylla/internal/tui/views/timer"
	"github.com/iruoy/fylla/internal/tui/views/worklog"
)

const (
	tabTimeline = iota
	tabTasks
	tabSchedule
	tabTimer
	tabWorklog
	tabConfig
	tabCount
)

// Deps holds the dependencies needed by the TUI.
type Deps struct {
	CB Callbacks
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
	formAddWorklog
	formAddWorklogPending // waiting for tasks to load for picker
	formEditWorklog
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
	timeline     timeline.Model
	tasks        tasks.Model
	schedule     schedule.Model
	timer        timerView.Model
	worklog      worklog.Model
	config       configView.Model
	timerKey     string
	timerSummary string
	timerElapsed time.Duration
	timerRunning bool
	tickGen      int
	toast        string
	toastIsError bool
	showHelp     bool
	ready        bool
	confirm      components.ConfirmDialog
	confirmType  confirmAction
	confirmKey   string
	form         components.Form
	picker       components.Picker
	formKind      formKind
	formTaskKey     string
	formWorklogID   string
	formWorklogKey  string
	formOptions   *msg.FormOptionsMsg
	pendingEdit   *pendingEditData
	viewDetail    *msg.ViewResult
	reportResult *msg.ReportResult
	spinner      spinner.Model
	saving       string // non-empty shows spinner in status bar with this label
}

func initialModel(deps Deps) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	return model{
		cb:       deps.CB,
		timeline: timeline.New(),
		tasks:    tasks.New(),
		schedule: schedule.New(),
		timer:    timerView.New(),
		worklog:  worklog.New(),
		config:   configView.New(),
		spinner:  s,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadTodayCmd(m.cb),
		timerStatusCmd(m.cb),
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
		m.timeline.SetSize(m.width, contentHeight)
		m.tasks.SetSize(m.width, contentHeight)
		m.schedule.SetSize(m.width, contentHeight)
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

		// Report overlay
		if m.reportResult != nil {
			if key.Matches(mssg, keys.Escape) || mssg.Type == tea.KeyRunes {
				m.reportResult = nil
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
			return m.switchTab(tabTimeline)
		case key.Matches(mssg, keys.Tab2):
			return m.switchTab(tabTasks)
		case key.Matches(mssg, keys.Tab3):
			return m.switchTab(tabSchedule)
		case key.Matches(mssg, keys.Tab4):
			return m.switchTab(tabTimer)
		case key.Matches(mssg, keys.Tab5):
			return m.switchTab(tabWorklog)
		case key.Matches(mssg, keys.Tab6):
			return m.switchTab(tabConfig)
		case key.Matches(mssg, keys.NextTab):
			return m.switchTab((m.activeTab + 1) % tabCount)
		case key.Matches(mssg, keys.PrevTab):
			return m.switchTab((m.activeTab + tabCount - 1) % tabCount)
		}

		// Route to active view
		switch m.activeTab {
		case tabTimeline:
			return m.updateTimeline(mssg)
		case tabTasks:
			return m.updateTasks(mssg)
		case tabSchedule:
			return m.updateSchedule(mssg)
		case tabTimer:
			return m.updateTimer(mssg)
		case tabWorklog:
			return m.updateWorklog(mssg)
		case tabConfig:
			return m.updateConfig(mssg)
		}

	case msg.TodayLoadedMsg:
		m.timeline.Loading = false
		if mssg.Err != nil {
			m.timeline.Err = mssg.Err
		} else {
			m.timeline.Events = mssg.Events
		}
		return m, nil

	case msg.TasksLoadedMsg:
		if m.formKind == formAddWorklogPending {
			if mssg.Err != nil {
				m.formKind = formAddWorklog // go back to form
				m.setToast(fmt.Sprintf("Failed to load tasks: %v", mssg.Err), true)
				cmds = append(cmds, clearToastCmd())
				return m, tea.Batch(cmds...)
			}
			items := make([]components.PickerItem, len(mssg.Tasks))
			for i, t := range mssg.Tasks {
				items[i] = components.PickerItem{
					Key:   t.Key,
					Label: fmt.Sprintf("%-10s  %s", t.Key, t.Summary),
				}
			}
			m.picker = components.NewPicker("Search Tasks (Enter to select, Esc to cancel)", items)
			m.formKind = formAddWorklog
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

	case msg.TimerStatusMsg:
		m.timer.Loading = false
		if mssg.Err == nil {
			m.timerKey = mssg.TaskKey
			m.timerSummary = mssg.Summary
			m.timerElapsed = mssg.Elapsed
			m.timerRunning = mssg.Running
			m.timer.TaskKey = mssg.TaskKey
			m.timer.Summary = mssg.Summary
			m.timer.Project = mssg.Project
			m.timer.Section = mssg.Section
			m.timer.Elapsed = mssg.Elapsed
			m.timer.Running = mssg.Running
			m.timer.Err = nil
		} else {
			m.timer.Err = mssg.Err
		}
		if m.timerRunning {
			m.tickGen++
			cmds = append(cmds, timerTickCmd(m.tickGen))
		}
		return m, tea.Batch(cmds...)

	case msg.TimerTickMsg:
		if m.timerRunning && mssg.Gen == m.tickGen {
			m.timerElapsed += time.Second
			m.timer.Elapsed = m.timerElapsed
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
			m.timer.Running = true
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
			m.setToast(fmt.Sprintf("Add error: %v", mssg.Err), true)
		} else {
			m.setToast(fmt.Sprintf("Added %s: %s", mssg.Key, mssg.Summary), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TaskEditedMsg:
		m.saving = ""
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Edit error: %v", mssg.Err), true)
		} else {
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
			m.setToast(fmt.Sprintf("Timer stopped for %s", stoppedLabel), false)
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
			m.setToast(fmt.Sprintf("Timer aborted for %s", stoppedLabel), false)
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.SyncPreviewMsg:
		m.schedule.Loading = false
		if mssg.Err != nil {
			m.schedule.Err = mssg.Err
		} else {
			m.schedule.Result = mssg.Result
			m.schedule.Err = nil
		}
		return m, nil

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
			m.config.Content = mssg.Content
			m.config.Err = nil
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
			m.form = buildEditForm(m.formTaskKey, *m.pendingEdit, mssg.Epics, mssg.Sections)
			m.formKind = formEditTask
			return m, nil
		}
		if m.formKind == formAddTaskPending {
			m.formOptions = &mssg
			m.form = buildAddForm(mssg.Provider, &mssg)
			m.formKind = formAddTask
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

	case msg.ReportLoadedMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Report error: %v", mssg.Err), true)
			cmds = append(cmds, clearToastCmd())
		} else {
			m.reportResult = mssg.Result
		}
		return m, tea.Batch(cmds...)

	case msg.ProjectsLoadedMsg:
		if m.form.Active && m.formKind == formAddTask && m.formOptions != nil {
			m.formOptions.Projects = mssg.Projects
			if len(mssg.Projects) > 0 {
				m.form.UpdateSelectByLabel("Project", mssg.Projects, mssg.Projects[0])
			} else {
				m.form.ConvertToTextByLabel("Project", "Project key")
			}
		}
		return m, nil

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
			cmds = append(cmds, loadWorklogsCmd(m.cb, m.worklog.WeekView))
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.WorklogDeletedMsg:
		m.saving = ""
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Worklog delete error: %v", mssg.Err), true)
		} else {
			m.setToast("Worklog deleted", false)
			cmds = append(cmds, loadWorklogsCmd(m.cb, m.worklog.WeekView))
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.WorklogAddedMsg:
		m.saving = ""
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Worklog add error: %v", mssg.Err), true)
		} else {
			m.setToast("Worklog added", false)
			cmds = append(cmds, loadWorklogsCmd(m.cb, m.worklog.WeekView))
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.AutoRefreshMsg:
		cmds = append(cmds, m.refreshActiveView(), autoRefreshCmd())
		return m, tea.Batch(cmds...)
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateTimeline(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Up):
		m.timeline.CursorUp()
	case key.Matches(mssg, keys.Down):
		m.timeline.CursorDown()
	case key.Matches(mssg, keys.Refresh):
		m.timeline.Loading = true
		return m, loadTodayCmd(m.cb)
	case key.Matches(mssg, keys.Enter), key.Matches(mssg, keys.Timer):
		if e := m.timeline.SelectedEvent(); e != nil && !e.IsCalendarEvent && e.TaskKey != "" {
			return m, startTimerCmd(m.cb, e.TaskKey, e.Summary, e.Project, e.Section)
		}
	case key.Matches(mssg, keys.Done):
		if e := m.timeline.SelectedEvent(); e != nil && !e.IsCalendarEvent && e.TaskKey != "" {
			return m, doneTaskCmd(m.cb, e.TaskKey)
		}
	case key.Matches(mssg, keys.Sync):
		return m, syncApplyCmd(m.cb, false)
	case key.Matches(mssg, keys.Report):
		return m, loadReportCmd(m.cb, 1)
	}
	return m, nil
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
			return m, doneTaskCmd(m.cb, t.Key)
		}
	case key.Matches(mssg, keys.Delete):
		if t := m.tasks.SelectedTask(); t != nil {
			m.confirm = components.NewConfirm(fmt.Sprintf("Delete %s?", t.Key))
			m.confirmType = confirmDeleteTask
			m.confirmKey = t.Key
		}
	case key.Matches(mssg, keys.Search):
		m.tasks.ToggleFilter()
	case key.Matches(mssg, keys.Add):
		m.formKind = formAddTaskPending
		return m, loadFormOptionsCmd(m.cb)
	case key.Matches(mssg, keys.Edit):
		if t := m.tasks.SelectedTask(); t != nil {
			ed := buildPendingEditData(t)
			m.formTaskKey = t.Key
			m.pendingEdit = &ed
			m.formKind = formEditTaskPending
			return m, loadEditFormOptionsCmd(m.cb, t.Project, t.Key)
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
				return m, deleteTaskCmd(m.cb, m.confirmKey)
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
				return m, deleteWorklogCmd(m.cb, m.formWorklogKey, m.confirmKey)
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
		return m, loadWorklogsCmd(m.cb, m.worklog.WeekView)
	case key.Matches(mssg, keys.WeekToggle):
		m.worklog.ToggleWeekView()
		m.worklog.Loading = true
		return m, loadWorklogsCmd(m.cb, m.worklog.WeekView)
	case key.Matches(mssg, keys.Add):
		m.form = components.NewForm("Add Worklog", []components.FormFieldDef{
			{Label: "Issue Key", Placeholder: "e.g. PROJ-123 (/ to search)"},
			{Label: "Duration", Placeholder: "e.g. 1h30m, 45m"},
			{Label: "Description", Placeholder: "What did you work on?"},
			{Label: "Started", Placeholder: "e.g. 09:00, 2006-01-02T15:04", Value: time.Now().Format("15:04")},
		})
		m.formKind = formAddWorklog
	case key.Matches(mssg, keys.Edit):
		if e := m.worklog.SelectedEntry(); e != nil {
			m.form = components.NewForm(fmt.Sprintf("Edit Worklog — %s", e.IssueKey), []components.FormFieldDef{
				{Label: "Duration", Placeholder: "e.g. 1h30m, 45m", Value: formatDurationShort(e.TimeSpent)},
				{Label: "Description", Placeholder: "What did you work on?", Value: e.Description},
				{Label: "Started", Placeholder: "e.g. 09:00, 2006-01-02T15:04", Value: e.Started.Format("15:04")},
			})
			m.formKind = formEditWorklog
			m.formWorklogID = e.ID
			m.formWorklogKey = e.IssueKey
		}
	case key.Matches(mssg, keys.Delete):
		if e := m.worklog.SelectedEntry(); e != nil {
			m.confirm = components.NewConfirm(fmt.Sprintf("Delete worklog on %s?", e.IssueKey))
			m.confirmType = confirmDeleteWorklog
			m.confirmKey = e.ID
			m.formWorklogKey = e.IssueKey
		}
	}
	return m, nil
}

func (m model) updateConfig(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Up):
		m.config.ScrollUp()
	case key.Matches(mssg, keys.Down):
		m.config.ScrollDown()
	case key.Matches(mssg, keys.Refresh):
		m.config.Loading = true
		return m, loadConfigCmd(m.cb)
	case key.Matches(mssg, keys.Edit):
		m.form = components.NewForm("Set Config", []components.FormFieldDef{
			{Label: "Key", Placeholder: "e.g. scheduling.windowDays"},
			{Label: "Value", Placeholder: "new value"},
		})
		m.formKind = formSetConfig
	}
	return m, nil
}

func (m model) updateTimer(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Refresh):
		m.timer.Loading = true
		return m, timerStatusCmd(m.cb)
	case key.Matches(mssg, keys.Stop):
		if m.timerRunning {
			m.form = components.NewForm("Stop Timer", []components.FormFieldDef{
				{Label: "Comment", Placeholder: "What did you work on?"},
				{Label: "Mark done", Kind: components.FieldToggle},
			})
			m.formKind = formStopTimer
			return m, nil
		}
	case key.Matches(mssg, keys.Abort):
		if m.timerRunning {
			m.confirm = components.NewConfirm("Abort timer? Work will not be logged.")
			m.confirmType = confirmAbortTimer
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateSchedule(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Up):
		m.schedule.ScrollUp()
	case key.Matches(mssg, keys.Down):
		m.schedule.ScrollDown()
	case key.Matches(mssg, keys.Refresh):
		m.schedule.Loading = true
		return m, syncPreviewCmd(m.cb)
	case key.Matches(mssg, keys.Enter), key.Matches(mssg, keys.Add):
		m.confirm = components.NewConfirm("Apply sync to calendar?")
		m.confirmType = confirmSyncApply
	case key.Matches(mssg, keys.Force):
		m.confirm = components.NewConfirm("Force sync? This will recreate all events.")
		m.confirmType = confirmSyncForce
	case key.Matches(mssg, keys.Clear):
		m.confirm = components.NewConfirm("Clear all Fylla events from calendar?")
		m.confirmType = confirmClearEvents
	}
	return m, nil
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
		if selected != nil {
			m.form.SetValueByLabel("Issue Key", selected.Key)
		}
		// Return to the form underneath
		return m, nil
	default:
		var cmd tea.Cmd
		m.picker.Filter, cmd = m.picker.Filter.Update(mssg)
		m.picker.ResetCursor()
		return m, cmd
	}
}

func (m model) updateForm(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// "/" on Issue Key field in add worklog form opens the task picker
	if m.formKind == formAddWorklog && m.form.FocusedLabel() == "Issue Key" && key.Matches(mssg, keys.Search) {
		m.formKind = formAddWorklogPending
		return m, loadTasksCmd(m.cb)
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
				return m, tea.Batch(loadEpicsCmd(m.cb, project), loadSectionsCmd(m.cb, provider, project))
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
				return m, tea.Batch(loadEpicsCmd(m.cb, project), loadSectionsCmd(m.cb, provider, project))
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
		m.form.Active = false
		vals := m.form.Values()
		switch m.formKind {
		case formAddTask:
			summary := m.form.ValueByLabel("Summary")
			if summary == "" {
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
			m.formKind = formNone
			m.formOptions = nil
			m.saving = "Adding task"
			return m, addTaskCmd(m.cb, provider, summary,
				m.form.ValueByLabel("Project"), section,
				m.form.ValueByLabel("Issue Type"),
				m.form.ValueByLabel("Description"),
				m.form.ValueByLabel("Estimate"),
				m.form.ValueByLabel("Due Date"),
				m.form.ValueByLabel("Priority"),
				parent,
			)
		case formEditTask:
			m.formKind = formNone
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
			m.pendingEdit = nil
			m.saving = "Saving task"
			return m, editTaskCmd(m.cb, EditTaskParams{
				TaskKey:      m.formTaskKey,
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
				m.formKind = formNone
				return m, nil
			}
			m.formKind = formNone
			m.saving = "Snoozing task"
			return m, snoozeTaskCmd(m.cb, m.formTaskKey, duration)
		case formStopTimer:
			comment := m.form.ValueByLabel("Comment")
			done := m.form.ValueByLabel("Mark done") == "true"
			m.formKind = formNone
			m.saving = "Stopping timer"
			return m, stopTimerCmd(m.cb, comment, done)
		case formAddWorklog:
			issueKey := m.form.ValueByLabel("Issue Key")
			durationStr := m.form.ValueByLabel("Duration")
			description := m.form.ValueByLabel("Description")
			startedStr := m.form.ValueByLabel("Started")
			if issueKey == "" || durationStr == "" {
				m.formKind = formNone
				return m, nil
			}
			dur := parseDurationInput(durationStr)
			started := parseStartedInput(startedStr)
			m.formKind = formNone
			m.saving = "Adding worklog"
			return m, addWorklogCmd(m.cb, issueKey, dur, description, started)
		case formEditWorklog:
			durationStr := m.form.ValueByLabel("Duration")
			description := m.form.ValueByLabel("Description")
			startedStr := m.form.ValueByLabel("Started")
			if durationStr == "" {
				m.formKind = formNone
				return m, nil
			}
			dur := parseDurationInput(durationStr)
			started := parseStartedInput(startedStr)
			m.formKind = formNone
			m.saving = "Updating worklog"
			return m, updateWorklogCmd(m.cb, m.formWorklogKey, m.formWorklogID, dur, description, started)
		case formSetConfig:
			cfgKey := vals[0]
			cfgVal := vals[1]
			if cfgKey == "" {
				m.formKind = formNone
				return m, nil
			}
			m.formKind = formNone
			m.saving = "Saving config"
			return m, setConfigCmd(m.cb, cfgKey, cfgVal)
		}
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
	return *m, m.refreshActiveView()
}

func (m model) refreshActiveView() tea.Cmd {
	switch m.activeTab {
	case tabTimeline:
		return loadTodayCmd(m.cb)
	case tabTasks:
		return loadTasksCmd(m.cb)
	case tabSchedule:
		return syncPreviewCmd(m.cb)
	case tabTimer:
		return timerStatusCmd(m.cb)
	case tabWorklog:
		return loadWorklogsCmd(m.cb, m.worklog.WeekView)
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
	case tabTimeline:
		if m.timeline.Loading {
			return true
		}
	case tabTasks:
		if m.tasks.Loading {
			return true
		}
	case tabSchedule:
		if m.schedule.Loading {
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
	return m.formKind == formAddTaskPending || m.formKind == formEditTaskPending || m.formKind == formAddWorklogPending
}

func (m model) loadingLabel() string {
	switch m.activeTab {
	case tabTimeline:
		if m.timeline.Loading {
			return "Loading timeline"
		}
	case tabTasks:
		if m.tasks.Loading {
			return "Loading tasks"
		}
	case tabSchedule:
		if m.schedule.Loading {
			return "Loading schedule"
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
	if m.formKind == formAddWorklogPending {
		return "Loading tasks"
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

	if m.reportResult != nil {
		return m.renderReport()
	}

	if m.showHelp {
		return m.renderHelp()
	}

	tabBar := components.RenderTabBar(components.TabNames(), m.activeTab, m.width)

	contentHeight := m.height - lipgloss.Height(tabBar) - 3
	var content string
	switch m.activeTab {
	case tabTimeline:
		content = m.timeline.View()
	case tabTasks:
		content = m.tasks.View()
	case tabSchedule:
		content = m.schedule.View()
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

	hints := "1-6:tabs  ?:help  q:quit"
	var loadingText string
	if label := m.loadingLabel(); label != "" {
		loadingText = m.spinner.View() + " " + label
	}
	statusBar := components.StatusBar{
		TimerKey:     m.timerKey,
		TimerSummary: m.timerSummary,
		TimerElapsed: m.timerElapsed,
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
	b.WriteString("  1-6           Switch tabs\n")
	b.WriteString("  Tab           Next tab\n")
	b.WriteString("  Shift+Tab     Previous tab\n")
	b.WriteString("  q/Ctrl+C      Quit\n")
	b.WriteString("  ?             Toggle help\n")
	b.WriteString("  Esc           Close overlay\n")
	b.WriteString("  r             Refresh\n\n")

	b.WriteString(bold.Render("Navigation") + "\n")
	b.WriteString("  j/k/arrows    Move cursor\n")
	b.WriteString("  Enter         Primary action\n\n")

	b.WriteString(bold.Render("Timeline") + "\n")
	b.WriteString("  t/Enter       Start timer\n")
	b.WriteString("  d             Mark done\n")
	b.WriteString("  s             Sync\n")
	b.WriteString("  R             Report\n\n")

	b.WriteString(bold.Render("Tasks") + "\n")
	b.WriteString("  a             Add task\n")
	b.WriteString("  e             Edit task\n")
	b.WriteString("  d             Mark done\n")
	b.WriteString("  D             Delete\n")
	b.WriteString("  S             Snooze\n")
	b.WriteString("  v             View details\n")
	b.WriteString("  t             Start timer\n")
	b.WriteString("  /             Search\n\n")

	b.WriteString(bold.Render("Schedule") + "\n")
	b.WriteString("  Enter/a       Apply sync\n")
	b.WriteString("  f             Force sync\n")
	b.WriteString("  c             Clear events\n\n")

	b.WriteString(bold.Render("Timer") + "\n")
	b.WriteString("  s             Stop timer\n")
	b.WriteString("  x             Abort timer\n\n")

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
	if name, ok := priorityLevelNames[v.Priority]; ok {
		b.WriteString(fmt.Sprintf("  Priority:   %s\n", name))
	}
	if v.Estimate > 0 {
		b.WriteString(fmt.Sprintf("  Estimate:   %s\n", formatDurationShort(v.Estimate)))
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

func (m model) renderReport() string {
	bold := lipgloss.NewStyle().Bold(true)
	hint := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})

	r := m.reportResult
	var b strings.Builder
	if r.Start.Format("2006-01-02") == r.End.Format("2006-01-02") {
		b.WriteString(bold.Render(fmt.Sprintf("Report for %s", r.Start.Format("Mon Jan 2, 2006"))) + "\n\n")
	} else {
		b.WriteString(bold.Render(fmt.Sprintf("Report for %s — %s",
			r.Start.Format("Mon Jan 2"), r.End.Format("Mon Jan 2, 2006"))) + "\n\n")
	}
	b.WriteString(fmt.Sprintf("  Tasks completed:  %d\n", r.TasksDone))
	b.WriteString(fmt.Sprintf("  Time on tasks:    %s\n", formatDurationShort(r.TaskTime)))
	b.WriteString(fmt.Sprintf("  Meeting time:     %s\n", formatDurationShort(r.MeetingTime)))
	b.WriteString(fmt.Sprintf("  Total events:     %d\n", r.TotalEvents))
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

func formatDurationShort(d time.Duration) string {
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

var priorityLevelNames = map[int]string{
	1: "Highest",
	2: "High",
	3: "Medium",
	4: "Low",
	5: "Lowest",
}

func priorityName(level int) string {
	if name, ok := priorityLevelNames[level]; ok {
		return name
	}
	return "Medium"
}

var jiraKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

func isJiraKeyPattern(key string) bool {
	return jiraKeyPattern.MatchString(key)
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
	ed.priority = priorityName(t.Priority)
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
		fields = append(fields, components.FormFieldDef{
			Label: "Issue Type", Kind: components.FieldSelect,
			Options: []string{"Task", "Bug", "Story", "Epic"}, Value: "Task",
		})
	}
	if provider != "jira" && provider != "github" {
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
		components.FormFieldDef{Label: "Due Date", Placeholder: "e.g. 2025-03-01"},
		components.FormFieldDef{Label: "Priority", Kind: components.FieldSelect, Options: []string{"Highest", "High", "Medium", "Low", "Lowest"}, Value: "Medium"},
	)
	if provider == "jira" {
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
	if newProvider == "jira" {
		cmds = append(cmds, loadEpicsCmd(m.cb, m.form.ValueByLabel("Project")))
	}
	cmds = append(cmds, loadSectionsCmd(m.cb, newProvider, m.form.ValueByLabel("Project")))
	return tea.Batch(cmds...)
}

func buildEditForm(taskKey string, ed pendingEditData, epics []msg.EpicOption, sections []string) components.Form {
	fields := []components.FormFieldDef{
		{Label: "Summary", Placeholder: "Task summary", Value: ed.summary},
		{Label: "Estimate", Placeholder: "e.g. 2h, 30m", Value: ed.estimate},
		{Label: "Due Date", Placeholder: "e.g. 2025-03-01", Value: ed.dueDate},
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

func parseStartedInput(s string) time.Time {
	s = strings.TrimSpace(s)
	now := time.Now()

	// Try full datetime
	if t, err := time.ParseInLocation("2006-01-02T15:04", s, now.Location()); err == nil {
		return t
	}
	// Try time only (HH:MM) — use today's date
	if t, err := time.Parse("15:04", s); err == nil {
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
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
