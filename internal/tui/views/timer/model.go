package timer

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerFmt    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	runningStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})
	timerBig     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})
	taskStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})
	hintStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"})
)

// Model is the timer view model.
type Model struct {
	TaskKey  string
	Summary  string
	Project  string
	Section  string
	Elapsed  time.Duration
	Running  bool
	Loading  bool
	Err      error
	Width    int
	Height   int
}

// New creates a new timer model.
func New() Model {
	return Model{Loading: true}
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// View renders the timer view.
func (m Model) View() string {
	if m.Loading {
		return "  Loading timer status..."
	}
	if m.Err != nil {
		return errStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	var b strings.Builder
	b.WriteString(headerFmt.Render("Timer"))
	b.WriteString("\n\n")

	if !m.Running {
		b.WriteString("  No timer running.\n\n")
		b.WriteString(hintStyle.Render("  Start a timer from the Timeline or Tasks tab using 't' or 'Enter'."))
		return b.String()
	}

	b.WriteString("  " + runningStyle.Render("RUNNING") + "\n\n")
	label := formatPrefix(m.Project, m.Section) + m.Summary
	if m.Summary == "" {
		label = m.TaskKey
	}
	b.WriteString("  Task: " + taskStyle.Render(label) + "\n\n")

	// Big elapsed display
	h := int(m.Elapsed.Hours())
	min := int(m.Elapsed.Minutes()) % 60
	sec := int(m.Elapsed.Seconds()) % 60
	elapsed := fmt.Sprintf("  %02d:%02d:%02d", h, min, sec)
	b.WriteString(timerBig.Render(elapsed))
	b.WriteString("\n\n")

	hints := "s:stop  r:refresh"
	b.WriteString(hintStyle.Render("  " + hints))

	return b.String()
}

func formatPrefix(project, section string) string {
	if project != "" && section != "" {
		return project + " / " + section + ": "
	}
	if project != "" {
		return project + ": "
	}
	return ""
}
