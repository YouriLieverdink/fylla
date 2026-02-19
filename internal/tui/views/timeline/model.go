package timeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/iruoy/fylla/internal/tui/msg"
)

var (
	currentStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})
	atRiskStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"}).Bold(true)
	calEventStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#555555"})
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})
	pastStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#555555"})
	headerFmt     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"})
)

// Model is the timeline view model.
type Model struct {
	Events  []msg.FyllaEvent
	Cursor  int
	Loading bool
	Err     error
	Width   int
	Height  int
}

// New creates a new timeline model.
func New() Model {
	return Model{Loading: true}
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// SelectedEvent returns the currently selected event, or nil.
func (m *Model) SelectedEvent() *msg.FyllaEvent {
	if len(m.Events) == 0 || m.Cursor < 0 || m.Cursor >= len(m.Events) {
		return nil
	}
	return &m.Events[m.Cursor]
}

// CursorUp moves the cursor up.
func (m *Model) CursorUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// CursorDown moves the cursor down.
func (m *Model) CursorDown() {
	if m.Cursor < len(m.Events)-1 {
		m.Cursor++
	}
}

// View renders the timeline view.
func (m Model) View() string {
	if m.Loading {
		return "  Loading today's schedule..."
	}
	if m.Err != nil {
		return errStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}
	if len(m.Events) == 0 {
		return "  No events scheduled for today."
	}

	now := time.Now()
	var b strings.Builder
	b.WriteString(headerFmt.Render("Today's Schedule"))
	b.WriteString("\n\n")

	for i, e := range m.Events {
		isCurrent := !now.Before(e.Start) && now.Before(e.End)
		isPast := now.After(e.End)
		isSelected := i == m.Cursor

		timeRange := fmt.Sprintf("%s - %s", e.Start.Format("15:04"), e.End.Format("15:04"))
		dur := e.End.Sub(e.Start)
		durStr := formatDuration(dur)

		var label string
		if e.IsCalendarEvent {
			label = fmt.Sprintf("%s  %s  %s", timeRange, e.Summary, durStr)
			switch {
			case isPast:
				label = pastStyle.Render(label)
			case isSelected:
				label = selectedStyle.Render(label)
			default:
				label = calEventStyle.Render(label)
			}
		} else {
			prefix := ""
			if e.AtRisk {
				prefix = "[LATE] "
			}
			taskLabel := e.TaskKey
			if e.Summary != "" {
				taskLabel = fmt.Sprintf("%s: %s", e.TaskKey, e.Summary)
			}
			label = fmt.Sprintf("%s  %s%s  %s", timeRange, prefix, taskLabel, durStr)
			switch {
			case isSelected:
				label = selectedStyle.Render(label)
			case isCurrent:
				label = currentStyle.Render(label)
			case e.AtRisk:
				label = atRiskStyle.Render(label)
			case isPast:
				label = pastStyle.Render(label)
			}
		}

		cursor := "  "
		if isSelected {
			cursor = "> "
		}
		if isCurrent && !isSelected {
			cursor = "* "
		}

		b.WriteString(cursor)
		b.WriteString(label)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	hints := "j/k:navigate  enter/t:timer  d:done  s:sync  r:refresh"
	b.WriteString(hintStyle.Render("  " + hints))

	return b.String()
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("(%dh%dm)", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("(%dh)", h)
	}
	return fmt.Sprintf("(%dm)", m)
}
