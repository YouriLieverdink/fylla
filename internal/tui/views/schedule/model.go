package schedule

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/iruoy/fylla/internal/tui/msg"
	"github.com/iruoy/fylla/internal/tui/styles"
)

// Model is the schedule view model.
type Model struct {
	Result       *msg.SyncResult
	Loading      bool
	Err          error
	Width        int
	Height       int
	ScrollPos    int
	contentLines int
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
	visibleHeight := m.Height - 2
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	maxScroll := m.contentLines - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.ScrollPos < maxScroll {
		m.ScrollPos++
	}
}

// View renders the schedule view.
func (m *Model) View() string {
	if m.Loading {
		return "  Loading schedule preview..."
	}
	if m.Err != nil {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}
	if m.Result == nil {
		return "  No schedule data."
	}

	var b strings.Builder
	b.WriteString(styles.HeaderFmt.Render("Schedule Preview (Dry Run)"))
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
		b.WriteString(styles.SectionFmt.Render("Schedule"))
		b.WriteString("\n")
		currentDay := ""
		for _, e := range entries {
			day := e.Start.Format("Mon Jan 2")
			if day != currentDay {
				b.WriteString("\n  " + styles.HeaderFmt.Render(day) + "\n")
				currentDay = day
			}
			line := fmt.Sprintf("    %s - %s  %s%s",
				e.Start.Format("15:04"), e.End.Format("15:04"),
				styles.FormatPrefix(e.Project, e.Section), e.Summary)
			switch {
			case e.IsCalEvent:
				line = styles.CalEventStyle.Render(line)
			case e.AtRisk:
				line = styles.AtRiskStyle.Render(line)
			}
			b.WriteString(styles.Truncate(line, m.Width) + "\n")
		}
		b.WriteString("\n")
	}

	// At-risk
	if len(atRisk) > 0 {
		b.WriteString(styles.AtRiskStyle.Render("At Risk"))
		b.WriteString("\n")
		for _, a := range atRisk {
			line := styles.AtRiskStyle.Render(fmt.Sprintf("    %s%s (%s - %s)",
				styles.FormatPrefix(a.Project, a.Section), a.Summary,
				a.Start.Format("15:04"), a.End.Format("15:04")))
			b.WriteString(styles.Truncate(line, m.Width) + "\n")
		}
		b.WriteString("\n")
	}

	// Unscheduled
	if len(m.Result.Unscheduled) > 0 {
		b.WriteString(styles.WarnStyle.Render("Unscheduled"))
		b.WriteString("\n")
		for _, u := range m.Result.Unscheduled {
			est := styles.FormatDurationOrDash(u.Estimate)
			line := styles.WarnStyle.Render(fmt.Sprintf("    %s%s  %s  (%s)",
				styles.FormatPrefix(u.Project, u.Section), u.Summary, est, u.Reason))
			b.WriteString(styles.Truncate(line, m.Width) + "\n")
		}
		b.WriteString("\n")
	}

	if len(entries) == 0 && len(m.Result.Unscheduled) == 0 {
		b.WriteString("  No tasks to schedule.\n")
	}

	b.WriteString("\n")
	hints := "j/k:scroll  enter/a:apply  f:force  c:clear  r:refresh"
	b.WriteString(styles.HintStyle.Render("  " + hints))

	// Apply scrolling
	lines := strings.Split(b.String(), "\n")
	m.contentLines = len(lines)
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
