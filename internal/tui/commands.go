package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/iruoy/fylla/internal/tui/msg"
)

// EditTaskParams holds all parameters for editing a task from the TUI.
type EditTaskParams struct {
	TaskKey   string
	Summary   string
	Estimate  string
	Due       string
	Priority  string
	UpNext    *bool
	NoSplit   *bool
	NotBefore string
}

// Callbacks holds function references that the TUI uses to invoke business logic.
type Callbacks struct {
	LoadToday   func() ([]msg.FyllaEvent, error)
	LoadTasks   func() ([]msg.ScoredTask, error)
	DoneTask    func(taskKey string) error
	DeleteTask  func(taskKey string) error
	StartTimer  func(taskKey string) error
	TimerStatus func() (taskKey string, elapsed time.Duration, running bool, err error)
	SyncPreview func() (*msg.SyncResult, error)
	SyncApply   func(force bool) (*msg.SyncResult, error)
	ClearEvents func() (int, error)
	LoadConfig  func() (string, error)
	SetConfig   func(key, value string) error
	AddTask      func(summary, project, section, issueType, description, estimate, dueDate, priority string) (key, summaryOut string, err error)
	EditTask     func(params EditTaskParams) error
	StopTimer    func(description string) (taskKey string, elapsed time.Duration, err error)
	ListProjects func() ([]string, error)
	Provider     func() string
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

func startTimerCmd(cb Callbacks, taskKey, summary string) tea.Cmd {
	return func() tea.Msg {
		err := cb.StartTimer(taskKey)
		return msg.TimerStartedMsg{TaskKey: taskKey, Summary: summary, Err: err}
	}
}

func timerStatusCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		taskKey, elapsed, running, err := cb.TimerStatus()
		return msg.TimerStatusMsg{
			TaskKey: taskKey,
			Elapsed: elapsed,
			Running: running,
			Err:     err,
		}
	}
}

func timerTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return msg.TimerTickMsg{}
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

func addTaskCmd(cb Callbacks, summary, project, section, issueType, description, estimate, dueDate, priority string) tea.Cmd {
	return func() tea.Msg {
		key, summaryOut, err := cb.AddTask(summary, project, section, issueType, description, estimate, dueDate, priority)
		return msg.TaskAddedMsg{Key: key, Summary: summaryOut, Err: err}
	}
}

func editTaskCmd(cb Callbacks, params EditTaskParams) tea.Cmd {
	return func() tea.Msg {
		err := cb.EditTask(params)
		return msg.TaskEditedMsg{TaskKey: params.TaskKey, Err: err}
	}
}

func stopTimerCmd(cb Callbacks, description string) tea.Cmd {
	return func() tea.Msg {
		taskKey, elapsed, err := cb.StopTimer(description)
		return msg.TimerStoppedMsg{TaskKey: taskKey, Elapsed: elapsed, Err: err}
	}
}

func loadFormOptionsCmd(cb Callbacks) tea.Cmd {
	return func() tea.Msg {
		var projects []string
		if cb.ListProjects != nil {
			p, err := cb.ListProjects()
			if err == nil {
				projects = p
			}
		}
		var provider string
		if cb.Provider != nil {
			provider = cb.Provider()
		}
		return msg.FormOptionsMsg{Projects: projects, Provider: provider}
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
