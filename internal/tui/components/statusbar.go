package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}).
			Padding(0, 1)

	timerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}).
			Bold(true)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"})

	successMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})

	pomodoroStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FF6347", Dark: "#FF6347"}).
			Padding(0, 1)

	pomodoroAlertStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FF6347", Dark: "#FF6347"}).
				Bold(true).
				Padding(0, 1)

	statusBarBorder = lipgloss.NewStyle().
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
)

// PomodoroState represents the current state of the pomodoro timer.
type PomodoroState int

const (
	PomodoroOff     PomodoroState = iota
	PomodoroRunning
	PomodoroPaused
	PomodoroBreak
)

// StatusBar holds the state for the bottom status bar.
type StatusBar struct {
	TimerKey     string
	TimerSummary string
	TimerElapsed time.Duration
	TimerRunning bool
	Toast        string
	ToastIsError bool
	HelpHints    string
	Width        int
	LoadingText  string
	Pomodoro          PomodoroState
	PomodoroRemaining time.Duration
}

// Render renders the status bar.
func (s StatusBar) Render() string {
	var left string

	// Pomodoro indicator (leftmost)
	pomo := s.renderPomodoro()
	if pomo != "" {
		left = pomo
	}

	if s.TimerRunning {
		label := s.TimerSummary
		if label == "" {
			label = s.TimerKey
		}
		timer := timerStyle.Render(fmt.Sprintf(" %s %s", label, formatElapsed(s.TimerElapsed)))
		if left != "" {
			left = left + "  " + timer
		} else {
			left = timer
		}
	}

	if s.Toast != "" {
		var toast string
		if s.ToastIsError {
			toast = errorMsgStyle.Render(s.Toast)
		} else {
			toast = successMsgStyle.Render(s.Toast)
		}
		if left != "" {
			left = left + "  " + toast
		} else {
			left = toast
		}
	}

	if s.LoadingText != "" {
		loading := statusStyle.Render(s.LoadingText)
		if left != "" {
			left = left + "  " + loading
		} else {
			left = loading
		}
	}

	right := statusStyle.Render(s.HelpHints)

	gap := s.Width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	filler := lipgloss.NewStyle().Width(gap).Render("")

	row := lipgloss.JoinHorizontal(lipgloss.Top, left, filler, right)
	return statusBarBorder.Width(s.Width).Render(row)
}

func (s StatusBar) renderPomodoro() string {
	switch s.Pomodoro {
	case PomodoroOff:
		return statusStyle.Render("\U0001F345 b:start")
	case PomodoroRunning:
		m := int(s.PomodoroRemaining.Minutes())
		sec := int(s.PomodoroRemaining.Seconds()) % 60
		return pomodoroStyle.Render(fmt.Sprintf("\U0001F345 %d:%02d", m, sec))
	case PomodoroPaused:
		m := int(s.PomodoroRemaining.Minutes())
		sec := int(s.PomodoroRemaining.Seconds()) % 60
		return statusStyle.Render(fmt.Sprintf("\U0001F345 %d:%02d", m, sec))
	case PomodoroBreak:
		return pomodoroAlertStyle.Render("\U0001F345 break!")
	}
	return ""
}

func formatElapsed(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}
