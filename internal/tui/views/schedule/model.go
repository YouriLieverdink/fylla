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
	Result  *msg.SyncResult
	Loading bool
	Err     error
	Width   int
	Height  int
	Cursor  int

	// Flat list of selectable entry indices (into entries slice).
	entries []scheduleEntry
}

type scheduleEntry struct {
	Start      time.Time
	End        time.Time
	Summary    string
	Project    string
	Section    string
	AtRisk     bool
	IsCalEvent bool
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

// CursorUp moves the cursor up.
func (m *Model) CursorUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// CursorDown moves the cursor down.
func (m *Model) CursorDown() {
	if m.Cursor < len(m.entries)-1 {
		m.Cursor++
	}
}

// ScrollUp is an alias for CursorUp for backwards compatibility.
func (m *Model) ScrollUp() { m.CursorUp() }

// ScrollDown is an alias for CursorDown for backwards compatibility.
func (m *Model) ScrollDown() { m.CursorDown() }

func (m *Model) buildEntries() {
	m.entries = nil
	if m.Result == nil {
		return
	}
	for _, a := range m.Result.Allocations {
		m.entries = append(m.entries, scheduleEntry{
			Start: a.Start, End: a.End, Summary: a.Summary,
			Project: a.Project, Section: a.Section, AtRisk: a.AtRisk,
		})
	}
	for _, e := range m.Result.CalendarEvents {
		m.entries = append(m.entries, scheduleEntry{
			Start: e.Start, End: e.End, Summary: e.Summary, IsCalEvent: true,
		})
	}
	sort.Slice(m.entries, func(i, j int) bool { return m.entries[i].Start.Before(m.entries[j].Start) })
	if m.Cursor >= len(m.entries) {
		m.Cursor = len(m.entries) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
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

	m.buildEntries()

	atRisk := make([]msg.Allocation, len(m.Result.AtRisk))
	copy(atRisk, m.Result.AtRisk)
	sort.Slice(atRisk, func(i, j int) bool { return atRisk[i].Start.Before(atRisk[j].Start) })

	// Build display lines.
	type displayLine struct {
		entryIdx int    // index into m.entries, or -1 for header/separator
		header   string // non-empty for section headers
	}
	var lines []displayLine

	// Group entries by day.
	if len(m.entries) > 0 {
		currentDay := ""
		for i, e := range m.entries {
			day := e.Start.Format("Mon Jan 2")
			if day != currentDay {
				lines = append(lines, displayLine{entryIdx: -1, header: day})
				currentDay = day
			}
			lines = append(lines, displayLine{entryIdx: i})
		}
		lines = append(lines, displayLine{entryIdx: -1}) // blank separator
	}

	// At-risk section (non-selectable).
	if len(atRisk) > 0 {
		lines = append(lines, displayLine{entryIdx: -1, header: "At Risk"})
		for _, a := range atRisk {
			dot := styles.FormatProjectDot(a.Project)
			line := styles.AtRiskStyle.Render(fmt.Sprintf("%s%s (%s - %s)",
				styles.FormatPrefix(a.Project, a.Section), a.Summary,
				a.Start.Format("15:04"), a.End.Format("15:04")))
			lines = append(lines, displayLine{entryIdx: -1, header: "  " + dot + line})
		}
		lines = append(lines, displayLine{entryIdx: -1}) // blank separator
	}

	// Unscheduled section (non-selectable).
	if len(m.Result.Unscheduled) > 0 {
		lines = append(lines, displayLine{entryIdx: -1, header: "Unscheduled"})
		for _, u := range m.Result.Unscheduled {
			dot := styles.FormatProjectDot(u.Project)
			est := styles.FormatDurationOrDash(u.Estimate)
			line := styles.WarnStyle.Render(fmt.Sprintf("%s%s  %s  (%s)",
				styles.FormatPrefix(u.Project, u.Section), u.Summary, est, u.Reason))
			lines = append(lines, displayLine{entryIdx: -1, header: "  " + dot + line})
		}
		lines = append(lines, displayLine{entryIdx: -1}) // blank separator
	}

	if len(m.entries) == 0 && len(m.Result.Unscheduled) == 0 {
		lines = append(lines, displayLine{entryIdx: -1, header: "No tasks to schedule."})
	}

	var b strings.Builder
	b.WriteString(styles.HeaderFmt.Render("  Schedule Preview (Dry Run)"))
	b.WriteString("\n\n")

	// Calculate visible range, keeping cursor in view.
	cursorDisplayIdx := 0
	for di, dl := range lines {
		if dl.entryIdx == m.Cursor {
			cursorDisplayIdx = di
			break
		}
	}

	visibleHeight := m.Height - 6
	if visibleHeight < 3 {
		visibleHeight = 3
	}

	startIdx := cursorDisplayIdx - visibleHeight/2
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > len(lines)-visibleHeight {
		startIdx = len(lines) - visibleHeight
	}
	if startIdx < 0 {
		startIdx = 0
	}

	endIdx := startIdx + visibleHeight
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	for di := startIdx; di < endIdx; di++ {
		dl := lines[di]
		if dl.entryIdx == -1 {
			if dl.header != "" {
				// Check if it's a raw pre-formatted line (at-risk/unscheduled items).
				if strings.HasPrefix(dl.header, "  ") {
					b.WriteString(dl.header)
				} else {
					b.WriteString(styles.HeaderFmt.Render("  " + dl.header))
				}
			}
			b.WriteString("\n")
			continue
		}

		e := m.entries[dl.entryIdx]
		isSelected := dl.entryIdx == m.Cursor

		dot := styles.FormatProjectDot(e.Project)
		timeRange := fmt.Sprintf("%s - %s", e.Start.Format("15:04"), e.End.Format("15:04"))
		prefix := styles.FormatPrefix(e.Project, e.Section)
		line := fmt.Sprintf("%s  %s%s", timeRange, prefix, e.Summary)

		switch {
		case e.IsCalEvent:
			line = styles.CalEventStyle.Render(styles.Truncate(line, m.Width-4))
		case e.AtRisk:
			line = styles.AtRiskStyle.Render(styles.Truncate(line, m.Width-4))
		default:
			line = styles.Truncate(line, m.Width-4)
		}

		cursor := "  "
		if isSelected {
			cursor = "> "
			if !e.IsCalEvent && !e.AtRisk {
				line = styles.SelectedStyle.Render(styles.Truncate(
					fmt.Sprintf("%s  %s%s", timeRange, prefix, e.Summary), m.Width-4))
			}
		}

		b.WriteString(cursor)
		b.WriteString(dot)
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(lines) > visibleHeight {
		b.WriteString(styles.HintStyle.Render(fmt.Sprintf("\n  Showing %d-%d of %d lines", startIdx+1, endIdx, len(lines))))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	hints := "j/k:navigate  enter/a:apply  f:force  c:clear  r:refresh"
	b.WriteString(styles.HintStyle.Render("  " + hints))

	return b.String()
}
