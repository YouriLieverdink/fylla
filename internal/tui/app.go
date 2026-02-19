package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/iruoy/fylla/internal/tui/components"
	"github.com/iruoy/fylla/internal/tui/msg"
	configView "github.com/iruoy/fylla/internal/tui/views/config"
	"github.com/iruoy/fylla/internal/tui/views/schedule"
	"github.com/iruoy/fylla/internal/tui/views/tasks"
	"github.com/iruoy/fylla/internal/tui/views/timeline"
	timerView "github.com/iruoy/fylla/internal/tui/views/timer"
)

const (
	tabTimeline = iota
	tabTasks
	tabSchedule
	tabTimer
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
)

type formKind int

const (
	formNone formKind = iota
	formAddTask
	formEditTask
	formSetConfig
)

type model struct {
	cb           Callbacks
	activeTab    int
	width        int
	height       int
	timeline     timeline.Model
	tasks        tasks.Model
	schedule     schedule.Model
	timer        timerView.Model
	config       configView.Model
	timerKey     string
	timerElapsed time.Duration
	timerRunning bool
	toast        string
	toastIsError bool
	showHelp     bool
	ready        bool
	confirm      components.ConfirmDialog
	confirmType  confirmAction
	confirmKey   string
	form         components.Form
	formKind     formKind
	formTaskKey  string
}

func initialModel(deps Deps) model {
	return model{
		cb:       deps.CB,
		timeline: timeline.New(),
		tasks:    tasks.New(),
		schedule: schedule.New(),
		timer:    timerView.New(),
		config:   configView.New(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		loadTodayCmd(m.cb),
		timerStatusCmd(m.cb),
		autoRefreshCmd(),
	)
}

func (m model) Update(mssg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch mssg := mssg.(type) {
	case tea.WindowSizeMsg:
		m.width = mssg.Width
		m.height = mssg.Height
		contentHeight := m.height - 4
		m.timeline.SetSize(m.width, contentHeight)
		m.tasks.SetSize(m.width, contentHeight)
		m.schedule.SetSize(m.width, contentHeight)
		m.timer.SetSize(m.width, contentHeight)
		m.config.SetSize(m.width, contentHeight)
		m.ready = true
		return m, nil

	case tea.KeyMsg:
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
			m.timerElapsed = mssg.Elapsed
			m.timerRunning = mssg.Running
			m.timer.TaskKey = mssg.TaskKey
			m.timer.Elapsed = mssg.Elapsed
			m.timer.Running = mssg.Running
			m.timer.Err = nil
		} else {
			m.timer.Err = mssg.Err
		}
		if m.timerRunning {
			cmds = append(cmds, timerTickCmd())
		}
		return m, tea.Batch(cmds...)

	case msg.TimerTickMsg:
		if m.timerRunning {
			m.timerElapsed += time.Second
			m.timer.Elapsed = m.timerElapsed
			cmds = append(cmds, timerTickCmd())
		}
		return m, tea.Batch(cmds...)

	case msg.TimerStartedMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Timer error: %v", mssg.Err), true)
		} else {
			m.timerKey = mssg.TaskKey
			m.timerElapsed = 0
			m.timerRunning = true
			m.timer.TaskKey = mssg.TaskKey
			m.timer.Elapsed = 0
			m.timer.Running = true
			m.setToast(fmt.Sprintf("Timer started for %s", mssg.TaskKey), false)
			cmds = append(cmds, timerTickCmd())
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
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Add error: %v", mssg.Err), true)
		} else {
			m.setToast(fmt.Sprintf("Added %s: %s", mssg.Key, mssg.Summary), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TaskEditedMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Edit error: %v", mssg.Err), true)
		} else {
			m.setToast(fmt.Sprintf("Edited %s", mssg.TaskKey), false)
			cmds = append(cmds, m.refreshActiveView())
		}
		cmds = append(cmds, clearToastCmd())
		return m, tea.Batch(cmds...)

	case msg.TimerStoppedMsg:
		if mssg.Err != nil {
			m.setToast(fmt.Sprintf("Stop error: %v", mssg.Err), true)
		} else {
			m.timerRunning = false
			m.timerKey = ""
			m.timerElapsed = 0
			m.timer.Running = false
			m.timer.TaskKey = ""
			m.timer.Elapsed = 0
			m.setToast(fmt.Sprintf("Timer stopped for %s", mssg.TaskKey), false)
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
			return m, startTimerCmd(m.cb, e.TaskKey)
		}
	case key.Matches(mssg, keys.Done):
		if e := m.timeline.SelectedEvent(); e != nil && !e.IsCalendarEvent && e.TaskKey != "" {
			return m, doneTaskCmd(m.cb, e.TaskKey)
		}
	case key.Matches(mssg, keys.Sync):
		return m, syncApplyCmd(m.cb, false)
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
			return m, startTimerCmd(m.cb, t.Key)
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
		m.form = components.NewForm("Add Task", []components.FormField{
			{Label: "Summary", Placeholder: "Task summary"},
			{Label: "Project", Placeholder: "Project key"},
			{Label: "Estimate", Placeholder: "e.g. 2h, 30m"},
			{Label: "Due Date", Placeholder: "e.g. 2025-03-01"},
			{Label: "Priority", Placeholder: "Highest/High/Medium/Low/Lowest"},
		})
		m.formKind = formAddTask
	case key.Matches(mssg, keys.Edit):
		if t := m.tasks.SelectedTask(); t != nil {
			estStr := ""
			if t.Estimate > 0 {
				h := int(t.Estimate.Hours())
				mins := int(t.Estimate.Minutes()) % 60
				if h > 0 && mins > 0 {
					estStr = fmt.Sprintf("%dh%dm", h, mins)
				} else if h > 0 {
					estStr = fmt.Sprintf("%dh", h)
				} else {
					estStr = fmt.Sprintf("%dm", mins)
				}
			}
			dueStr := ""
			if t.DueDate != nil {
				dueStr = t.DueDate.Format("2006-01-02")
			}
			m.form = components.NewForm(fmt.Sprintf("Edit %s", t.Key), []components.FormField{
				{Label: "Estimate", Placeholder: "e.g. 2h, 30m", Value: estStr},
				{Label: "Due Date", Placeholder: "e.g. 2025-03-01", Value: dueStr},
				{Label: "Priority", Placeholder: "Highest/High/Medium/Low/Lowest"},
			})
			m.formKind = formEditTask
			m.formTaskKey = t.Key
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
			}
		}
		m.confirmType = confirmNone
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
		m.form = components.NewForm("Set Config", []components.FormField{
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
			return m, stopTimerCmd(m.cb, "")
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

func (m model) updateForm(mssg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(mssg, keys.Escape):
		m.form.Active = false
		m.formKind = formNone
		return m, nil
	case mssg.Type == tea.KeyTab:
		m.form.FocusNext()
		return m, nil
	case mssg.Type == tea.KeyShiftTab:
		m.form.FocusPrev()
		return m, nil
	case key.Matches(mssg, keys.Enter):
		m.form.Active = false
		vals := m.form.Values()
		switch m.formKind {
		case formAddTask:
			summary := vals[0]
			if summary == "" {
				m.formKind = formNone
				return m, nil
			}
			m.formKind = formNone
			return m, addTaskCmd(m.cb, summary, vals[1], "", vals[2], vals[3], vals[4])
		case formEditTask:
			m.formKind = formNone
			return m, editTaskCmd(m.cb, m.formTaskKey, vals[0], vals[1], vals[2])
		case formSetConfig:
			cfgKey := vals[0]
			cfgVal := vals[1]
			if cfgKey == "" {
				m.formKind = formNone
				return m, nil
			}
			m.formKind = formNone
			return m, setConfigCmd(m.cb, cfgKey, cfgVal)
		}
		m.formKind = formNone
		return m, nil
	default:
		// Pass key to focused input
		var cmd tea.Cmd
		m.form.Fields[m.form.Focus], cmd = m.form.Fields[m.form.Focus].Update(mssg)
		return m, cmd
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
	case tabConfig:
		return loadConfigCmd(m.cb)
	}
	return nil
}

func (m *model) setToast(text string, isError bool) {
	m.toast = text
	m.toastIsError = isError
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Form overlay
	if m.form.Active {
		return m.form.View(m.width, m.height)
	}

	// Confirm dialog overlay
	if m.confirm.Active {
		return m.confirm.View(m.width, m.height)
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
	case tabConfig:
		content = m.config.View()
	}

	contentArea := lipgloss.NewStyle().
		Height(contentHeight).
		Width(m.width).
		Render(content)

	hints := "1-5:tabs  ?:help  q:quit"
	statusBar := components.StatusBar{
		TimerKey:     m.timerKey,
		TimerElapsed: m.timerElapsed,
		TimerRunning: m.timerRunning,
		Toast:        m.toast,
		ToastIsError: m.toastIsError,
		HelpHints:    hints,
		Width:        m.width,
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
	b.WriteString("  1-5           Switch tabs\n")
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
	b.WriteString("  s             Sync\n\n")

	b.WriteString(bold.Render("Tasks") + "\n")
	b.WriteString("  a             Add task\n")
	b.WriteString("  e             Edit task\n")
	b.WriteString("  d             Mark done\n")
	b.WriteString("  D             Delete\n")
	b.WriteString("  t             Start timer\n")
	b.WriteString("  /             Search\n\n")

	b.WriteString(bold.Render("Schedule") + "\n")
	b.WriteString("  Enter/a       Apply sync\n")
	b.WriteString("  f             Force sync\n")
	b.WriteString("  c             Clear events\n\n")

	b.WriteString(bold.Render("Timer") + "\n")
	b.WriteString("  s             Stop timer\n\n")

	b.WriteString(bold.Render("Config") + "\n")
	b.WriteString("  e             Edit value\n\n")

	b.WriteString(hint.Render("Press ? or Esc to close"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}).
		Padding(1, 3).
		Render(b.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
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
