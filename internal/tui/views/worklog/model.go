package worklog

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/iruoy/fylla/internal/tui/msg"
	"github.com/iruoy/fylla/internal/tui/styles"
)

// Model is the worklog view model.
type Model struct {
	Entries          []msg.WorklogEntry
	Cursor           int
	Loading          bool
	Err              error
	Width            int
	Height           int
	WeekView         bool
	Date             time.Time
	DailyHours       float64
	WeeklyHours      float64
	EfficiencyTarget float64
}

// New creates a new worklog model.
func New(dailyHours, weeklyHours, efficiencyTarget float64) Model {
	return Model{Loading: true, Date: today(), DailyHours: dailyHours, WeeklyHours: weeklyHours, EfficiencyTarget: efficiencyTarget}
}

func today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// PrevDate moves the date backward by 1 day (day view) or 7 days (week view).
func (m *Model) PrevDate() {
	if m.WeekView {
		m.Date = m.Date.AddDate(0, 0, -7)
	} else {
		m.Date = m.Date.AddDate(0, 0, -1)
	}
	m.Cursor = 0
}

// NextDate moves the date forward by 1 day or 7 days, clamped to today.
func (m *Model) NextDate() {
	t := today()
	if m.WeekView {
		next := m.Date.AddDate(0, 0, 7)
		if !next.After(t) {
			m.Date = next
		} else {
			m.Date = t
		}
	} else {
		next := m.Date.AddDate(0, 0, 1)
		if !next.After(t) {
			m.Date = next
		} else {
			m.Date = t
		}
	}
	m.Cursor = 0
}

// GoToToday resets the date to today.
func (m *Model) GoToToday() {
	m.Date = today()
	m.Cursor = 0
}

// IsToday reports whether the selected date is today.
func (m *Model) IsToday() bool {
	return m.Date.Equal(today())
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
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	sorted := m.sortedEntries()
	var b strings.Builder

	var viewLabel string
	if m.IsToday() {
		if m.WeekView {
			viewLabel = "This Week"
		} else {
			viewLabel = "Today"
		}
	} else {
		if m.WeekView {
			viewLabel = "Week of " + m.Date.Format("Mon Jan 2, 2006")
		} else {
			viewLabel = m.Date.Format("Mon Jan 2, 2006")
		}
	}
	total := totalTime(sorted)
	title := fmt.Sprintf("Worklogs — %s (%d entries, %s)", viewLabel, len(sorted), styles.FormatDuration(total))
	b.WriteString(styles.HeaderFmt.Render(title))
	b.WriteString("\n")

	if line := m.efficiencyLine(total); line != "" {
		b.WriteString(line)
	}
	b.WriteString("\n")

	if len(sorted) == 0 {
		b.WriteString("  No worklogs found.\n")
	} else if m.WeekView {
		m.renderWeekView(&b, sorted)
	} else {
		m.renderDayView(&b, sorted)
	}

	b.WriteString("\n")
	hints := "j/k:navigate  h/l:prev/next day  T:today  a:add  e:edit  D:delete  w:toggle week  s:standup  r:refresh"
	b.WriteString(styles.HintStyle.Render("  " + hints))

	return b.String()
}

func (m Model) dailyTarget() time.Duration {
	if m.DailyHours <= 0 {
		return 0
	}
	return time.Duration(m.DailyHours * float64(time.Hour))
}

func (m Model) weeklyTarget() time.Duration {
	if m.WeeklyHours <= 0 {
		return 0
	}
	return time.Duration(m.WeeklyHours * float64(time.Hour))
}

func (m Model) efficiencyLine(total time.Duration) string {
	var target time.Duration
	if m.WeekView {
		target = m.weeklyTarget()
	} else {
		target = m.dailyTarget()
	}
	if target <= 0 {
		return ""
	}

	remaining := time.Duration(float64(target)*m.EfficiencyTarget) - total
	var remainingStr string
	if remaining <= 0 {
		remainingStr = styles.CurrentStyle.Render("✓ Target reached")
	} else {
		var style lipgloss.Style
		efficiency := float64(total) / float64(target)
		switch {
		case efficiency >= m.EfficiencyTarget-0.1:
			style = styles.WarnStyle
		default:
			style = styles.ErrStyle
		}
		remainingStr = style.Render(styles.FormatDuration(remaining)+" to target")
	}

	return fmt.Sprintf("  Efficiency: %s  Target: %.0f%%  %s\n",
		m.formatEfficiency(total, target),
		m.EfficiencyTarget*100,
		remainingStr,
	)
}

func (m Model) formatEfficiency(logged, target time.Duration) string {
	efficiency := float64(logged) / float64(target)
	pct := fmt.Sprintf("%.1f%%", efficiency*100)

	var style lipgloss.Style
	switch {
	case efficiency >= m.EfficiencyTarget:
		style = styles.CurrentStyle
	case efficiency >= m.EfficiencyTarget-0.1:
		style = styles.WarnStyle
	default:
		style = styles.ErrStyle
	}

	return fmt.Sprintf("%s (%s / %s)",
		style.Render(pct),
		styles.FormatDuration(logged),
		styles.FormatDuration(target),
	)
}

func (m Model) renderDayView(b *strings.Builder, sorted []msg.WorklogEntry) {
	visibleHeight := m.Height - 8
	if visibleHeight < 3 {
		visibleHeight = 3
	}

	// Center cursor in visible window.
	startIdx := m.Cursor - visibleHeight/2
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > len(sorted)-visibleHeight {
		startIdx = len(sorted) - visibleHeight
	}
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + visibleHeight
	if endIdx > len(sorted) {
		endIdx = len(sorted)
	}

	for i := startIdx; i < endIdx; i++ {
		e := sorted[i]
		isSelected := i == m.Cursor
		line := m.formatEntryLine(e)
		cursor := "  "
		if isSelected {
			cursor = "> "
			line = styles.SelectedStyle.Render(line)
		}
		b.WriteString(cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(sorted) > visibleHeight {
		b.WriteString(styles.HintStyle.Render(fmt.Sprintf("\n  Showing %d-%d of %d", startIdx+1, endIdx, len(sorted))))
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
		header := fmt.Sprintf("%s  %s", t.Format("Mon Jan 2"), styles.FormatDuration(dayTotal))
		if dt := m.dailyTarget(); dt > 0 {
			header += "  " + m.formatEfficiency(dayTotal, dt)
		}
		lines = append(lines, displayLine{entryIdx: -1, header: header})
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

	// Find display line for cursor and center it.
	cursorDisplayIdx := 0
	for di, dl := range lines {
		if dl.entryIdx == m.Cursor {
			cursorDisplayIdx = di
			break
		}
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
				b.WriteString(styles.HeaderFmt.Render("  " + dl.header))
			}
			b.WriteString("\n")
			continue
		}

		e := sorted[dl.entryIdx]
		isSelected := dl.entryIdx == m.Cursor
		line := m.formatEntryLine(e)
		cursor := "  "
		if isSelected {
			cursor = "> "
			line = styles.SelectedStyle.Render(line)
		}
		b.WriteString(cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}
}

func (m Model) formatEntryLine(e msg.WorklogEntry) string {
	dot := styles.FormatProjectDot(e.Project)
	timeRange := e.Started.Format("15:04") + "–" + e.Started.Add(e.TimeSpent).Format("15:04")
	dur := styles.FormatDurationPadded(e.TimeSpent)
	key := styles.PadOrTruncate(e.IssueKey, 10)

	// Fixed parts: cursor(2) + dot(2) + time(11) + gaps(6) + key(10) + dur(5) = 36
	const fixedCols = 36
	remaining := m.Width - fixedCols
	if remaining < 10 {
		remaining = 10
	}

	// Split remaining space: ~60% summary, ~40% description.
	summaryW := remaining * 6 / 10
	descW := remaining - summaryW - 2 // 2 for gap

	summary := e.IssueSummary
	if summary == "" {
		summary = "-"
	}
	summary = styles.PadOrTruncate(summary, summaryW)

	desc := e.Description
	if desc == "" {
		desc = "-"
	}
	desc = styles.Truncate(desc, descW)

	return fmt.Sprintf("%s%s  %s  %s  %s  %s", dot, timeRange, key, dur, summary, styles.HintStyle.Render(desc))
}
