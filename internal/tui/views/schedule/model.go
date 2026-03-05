package schedule

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/iruoy/fylla/internal/tui/msg"
)

var (
	headerFmt     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	sectionFmt    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})
	atRiskStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"})
	calEventStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#555555"})
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#F2A900", Dark: "#FDCB58"})
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"})
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

	// Build unified schedule entries (tasks + calendar events)
	type scheduleEntry struct {
		Start       time.Time
		End         time.Time
		Summary     string
		Project     string
		Section     string
		AtRisk      bool
		IsCalEvent  bool
	}

	var entries []scheduleEntry
	for _, a := range m.Result.Allocations {
		entries = append(entries, scheduleEntry{
			Start: a.Start, End: a.End, Summary: a.Summary,
			Project: a.Project, Section: a.Section, AtRisk: a.AtRisk,
		})
	}
	for _, e := range m.Result.CalendarEvents {
		entries = append(entries, scheduleEntry{
			Start: e.Start, End: e.End, Summary: e.Summary, IsCalEvent: true,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Start.Before(entries[j].Start) })

	atRisk := make([]msg.Allocation, len(m.Result.AtRisk))
	copy(atRisk, m.Result.AtRisk)
	sort.Slice(atRisk, func(i, j int) bool { return atRisk[i].Start.Before(atRisk[j].Start) })

	// Group entries by day
	if len(entries) > 0 {
		b.WriteString(sectionFmt.Render("Schedule"))
		b.WriteString("\n")
		currentDay := ""
		for _, e := range entries {
			day := e.Start.Format("Mon Jan 2")
			if day != currentDay {
				b.WriteString("\n  " + headerFmt.Render(day) + "\n")
				currentDay = day
			}
			line := fmt.Sprintf("    %s - %s  %s%s",
				e.Start.Format("15:04"), e.End.Format("15:04"),
				formatPrefix(e.Project, e.Section), e.Summary)
			switch {
			case e.IsCalEvent:
				line = calEventStyle.Render(line)
			case e.AtRisk:
				line = atRiskStyle.Render(line)
			}
			b.WriteString(truncate(line, m.Width) + "\n")
		}
		b.WriteString("\n")
	}

	// At-risk
	if len(atRisk) > 0 {
		b.WriteString(atRiskStyle.Render("At Risk"))
		b.WriteString("\n")
		for _, a := range atRisk {
			line := atRiskStyle.Render(fmt.Sprintf("    %s%s (%s - %s)",
				formatPrefix(a.Project, a.Section), a.Summary,
				a.Start.Format("15:04"), a.End.Format("15:04")))
			b.WriteString(truncate(line, m.Width) + "\n")
		}
		b.WriteString("\n")
	}

	// Unscheduled
	if len(m.Result.Unscheduled) > 0 {
		b.WriteString(warnStyle.Render("Unscheduled"))
		b.WriteString("\n")
		for _, u := range m.Result.Unscheduled {
			est := formatDuration(u.Estimate)
			line := warnStyle.Render(fmt.Sprintf("    %s%s  %s  (%s)",
				formatPrefix(u.Project, u.Section), u.Summary, est, u.Reason))
			b.WriteString(truncate(line, m.Width) + "\n")
		}
		b.WriteString("\n")
	}

	if len(entries) == 0 && len(m.Result.Unscheduled) == 0 {
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

func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Truncate(s, width, "…")
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
