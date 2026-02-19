package schedule

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/iruoy/fylla/internal/tui/msg"
)

var (
	headerFmt   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	sectionFmt  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})
	atRiskStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"})
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#F2A900", Dark: "#FDCB58"})
	hintStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"})
)

// Model is the schedule view model.
type Model struct {
	Result    *msg.SyncResult
	Loading   bool
	Err       error
	Width     int
	Height    int
	ScrollPos int
}

// New creates a new schedule model.
func New() Model {
	return Model{Loading: true}
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// ScrollUp scrolls the content up.
func (m *Model) ScrollUp() {
	if m.ScrollPos > 0 {
		m.ScrollPos--
	}
}

// ScrollDown scrolls the content down.
func (m *Model) ScrollDown() {
	m.ScrollPos++
}

// View renders the schedule view.
func (m Model) View() string {
	if m.Loading {
		return "  Loading schedule preview..."
	}
	if m.Err != nil {
		return errStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}
	if m.Result == nil {
		return "  No schedule data."
	}

	var b strings.Builder
	b.WriteString(headerFmt.Render("Schedule Preview (Dry Run)"))
	b.WriteString("\n\n")

	// Group allocations by day
	if len(m.Result.Allocations) > 0 {
		b.WriteString(sectionFmt.Render("Scheduled Tasks"))
		b.WriteString("\n")
		currentDay := ""
		for _, a := range m.Result.Allocations {
			day := a.Start.Format("Mon Jan 2")
			if day != currentDay {
				b.WriteString("\n  " + headerFmt.Render(day) + "\n")
				currentDay = day
			}
			line := fmt.Sprintf("    %s - %s  %s: %s",
				a.Start.Format("15:04"), a.End.Format("15:04"),
				a.TaskKey, a.Summary)
			if a.AtRisk {
				line = atRiskStyle.Render(line)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	// At-risk
	if len(m.Result.AtRisk) > 0 {
		b.WriteString(atRiskStyle.Render("At Risk"))
		b.WriteString("\n")
		for _, a := range m.Result.AtRisk {
			b.WriteString(atRiskStyle.Render(fmt.Sprintf("    %s: %s (%s - %s)",
				a.TaskKey, a.Summary,
				a.Start.Format("15:04"), a.End.Format("15:04"))))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Unscheduled
	if len(m.Result.Unscheduled) > 0 {
		b.WriteString(warnStyle.Render("Unscheduled"))
		b.WriteString("\n")
		for _, u := range m.Result.Unscheduled {
			est := formatDuration(u.Estimate)
			b.WriteString(warnStyle.Render(fmt.Sprintf("    %s: %s  %s  (%s)",
				u.TaskKey, u.Summary, est, u.Reason)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(m.Result.Allocations) == 0 && len(m.Result.Unscheduled) == 0 {
		b.WriteString("  No tasks to schedule.\n")
	}

	b.WriteString("\n")
	hints := "j/k:scroll  enter/a:apply  f:force  c:clear  r:refresh"
	b.WriteString(hintStyle.Render("  " + hints))

	// Apply scrolling
	lines := strings.Split(b.String(), "\n")
	visibleHeight := m.Height - 2
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	start := m.ScrollPos
	if start >= len(lines) {
		start = len(lines) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "--"
	}
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
