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

	statusBarBorder = lipgloss.NewStyle().
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
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
}

// Render renders the status bar.
func (s StatusBar) Render() string {
	var left string

	if s.TimerRunning {
		label := s.TimerSummary
		if label == "" {
			label = s.TimerKey
		}
		left = timerStyle.Render(fmt.Sprintf(" %s %s", label, formatElapsed(s.TimerElapsed)))
	}

	if s.Toast != "" {
		if s.ToastIsError {
			left = errorMsgStyle.Render(s.Toast)
		} else {
			left = successMsgStyle.Render(s.Toast)
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

func formatElapsed(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}
