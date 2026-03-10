package worklog

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/iruoy/fylla/internal/tui/msg"
)

var (
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})
	headerFmt     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"})
)

// Model is the worklog view model.
type Model struct {
	Entries  []msg.WorklogEntry
	Cursor   int
	Loading  bool
	Err      error
	Width    int
	Height   int
	WeekView bool
}

// New creates a new worklog model.
func New() Model {
	return Model{Loading: true}
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// SelectedEntry returns the currently selected entry, or nil.
func (m *Model) SelectedEntry() *msg.WorklogEntry {
	if len(m.Entries) == 0 || m.Cursor < 0 || m.Cursor >= len(m.Entries) {
		return nil
	}
	sorted := m.sortedEntries()
	if m.Cursor >= len(sorted) {
		return nil
	}
	return &sorted[m.Cursor]
}

// CursorUp moves the cursor up.
func (m *Model) CursorUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// CursorDown moves the cursor down.
func (m *Model) CursorDown() {
	sorted := m.sortedEntries()
	if m.Cursor < len(sorted)-1 {
		m.Cursor++
	}
}

// ToggleWeekView toggles between today and week view.
func (m *Model) ToggleWeekView() {
	m.WeekView = !m.WeekView
	m.Cursor = 0
}

func (m *Model) sortedEntries() []msg.WorklogEntry {
	sorted := make([]msg.WorklogEntry, len(m.Entries))
	copy(sorted, m.Entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Started.Before(sorted[j].Started)
	})
	return sorted
}

func totalTime(entries []msg.WorklogEntry) time.Duration {
	var total time.Duration
	for _, e := range entries {
		total += e.TimeSpent
	}
	return total
}

// View renders the worklog view.
func (m Model) View() string {
	if m.Loading {
		return "  Loading worklogs..."
	}
	if m.Err != nil {
		return errStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	sorted := m.sortedEntries()
	var b strings.Builder

	viewLabel := "Today"
	if m.WeekView {
		viewLabel = "This Week"
	}
	total := totalTime(sorted)
	title := fmt.Sprintf("Worklogs — %s (%d entries, %s)", viewLabel, len(sorted), formatDuration(total))
	b.WriteString(headerFmt.Render(title))
	b.WriteString("\n\n")

	if len(sorted) == 0 {
		b.WriteString("  No worklogs found.\n")
	} else if m.WeekView {
		m.renderWeekView(&b, sorted)
	} else {
		m.renderDayView(&b, sorted)
	}

	b.WriteString("\n")
	hints := "j/k:navigate  a:add  e:edit  D:delete  w:toggle week  r:refresh"
	b.WriteString(hintStyle.Render("  " + hints))

	return b.String()
}

func (m Model) renderDayView(b *strings.Builder, sorted []msg.WorklogEntry) {
	visibleHeight := m.Height - 8
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	startIdx := 0
	if m.Cursor >= visibleHeight {
		startIdx = m.Cursor - visibleHeight + 1
	}
	endIdx := startIdx + visibleHeight
	if endIdx > len(sorted) {
		endIdx = len(sorted)
	}

	for i := startIdx; i < endIdx; i++ {
		e := sorted[i]
		isSelected := i == m.Cursor
		line := formatEntryLine(e)
		cursor := "  "
		if isSelected {
			cursor = "> "
			line = selectedStyle.Render(line)
		}
		b.WriteString(cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(sorted) > visibleHeight {
		b.WriteString(hintStyle.Render(fmt.Sprintf("\n  Showing %d-%d of %d", startIdx+1, endIdx, len(sorted))))
		b.WriteString("\n")
	}
}

func (m Model) renderWeekView(b *strings.Builder, sorted []msg.WorklogEntry) {
	// Group by day
	type dayGroup struct {
		date    string
		entries []msg.WorklogEntry
	}
	groups := make(map[string]*dayGroup)
	var dayOrder []string
	for _, e := range sorted {
		day := e.Started.Format("2006-01-02")
		if _, ok := groups[day]; !ok {
			groups[day] = &dayGroup{date: day}
			dayOrder = append(dayOrder, day)
		}
		groups[day].entries = append(groups[day].entries, e)
	}

	// Build display lines
	type displayLine struct {
		entryIdx int // -1 for header/separator
		header   string
	}
	var lines []displayLine
	flatIdx := 0
	entryToFlat := make(map[int]int) // sorted index -> flat index

	for _, day := range dayOrder {
		g := groups[day]
		t, _ := time.Parse("2006-01-02", g.date)
		dayTotal := totalTime(g.entries)
		lines = append(lines, displayLine{entryIdx: -1, header: fmt.Sprintf("%s  %s", t.Format("Mon Jan 2"), formatDuration(dayTotal))})
		for _, e := range g.entries {
			entryToFlat[flatIdx] = len(lines)
			lines = append(lines, displayLine{entryIdx: flatIdx})
			_ = e
			flatIdx++
		}
		lines = append(lines, displayLine{entryIdx: -1}) // blank separator
	}

	visibleHeight := m.Height - 8
	if visibleHeight < 3 {
		visibleHeight = 3
	}

	// Find display line for cursor
	cursorDisplayIdx := 0
	for _, dl := range lines {
		if dl.entryIdx == m.Cursor {
			break
		}
		cursorDisplayIdx++
	}

	startIdx := 0
	if cursorDisplayIdx >= visibleHeight {
		startIdx = cursorDisplayIdx - visibleHeight + 1
	}
	endIdx := startIdx + visibleHeight
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	for di := startIdx; di < endIdx; di++ {
		dl := lines[di]
		if dl.entryIdx == -1 {
			if dl.header != "" {
				b.WriteString(headerFmt.Render("  " + dl.header))
			}
			b.WriteString("\n")
			continue
		}

		e := sorted[dl.entryIdx]
		isSelected := dl.entryIdx == m.Cursor
		line := formatEntryLine(e)
		cursor := "  "
		if isSelected {
			cursor = "> "
			line = selectedStyle.Render(line)
		}
		b.WriteString(cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}
}

func formatEntryLine(e msg.WorklogEntry) string {
	timeStr := e.Started.Format("15:04")
	dur := formatDuration(e.TimeSpent)
	desc := e.Description
	if desc == "" {
		desc = "-"
	}
	if len(desc) > 40 {
		desc = desc[:37] + "..."
	}
	return fmt.Sprintf("%s  %-10s  %6s  %s", timeStr, e.IssueKey, dur, desc)
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
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
