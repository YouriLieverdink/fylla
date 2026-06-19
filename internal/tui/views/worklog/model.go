package worklog

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/iruoy/fylla/internal/config"
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
	WorkDays         map[int]bool // ISO weekday (1=Mon..7=Sun)
	BusinessHours    []config.BusinessHoursConfig
	Holidays         config.HolidayIndex
	SickDays         config.HolidayIndex
	CalmMode         bool // hide durations/efficiency; collapse entries per task
}

// New creates a new worklog model.
func New(dailyHours, weeklyHours, efficiencyTarget float64, workDays []int, businessHours []config.BusinessHoursConfig, holidays, sickDays config.HolidayIndex) Model {
	wd := make(map[int]bool, len(workDays))
	for _, d := range workDays {
		wd[d] = true
	}
	if len(wd) == 0 {
		for i := 1; i <= 5; i++ {
			wd[i] = true
		}
	}
	return Model{
		Loading:          true,
		Date:             today(),
		DailyHours:       dailyHours,
		WeeklyHours:      weeklyHours,
		EfficiencyTarget: efficiencyTarget,
		WorkDays:         wd,
		BusinessHours:    businessHours,
		Holidays:         holidays,
		SickDays:         sickDays,
	}
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
	// Total, deterministic order. Date-only entries (e.g. Jibble) all share the
	// same midnight Started, so without a tiebreak sort.Slice would order them
	// arbitrarily — and differently between the renderer and SelectedEntry,
	// causing the cursor to act on a different row than the one highlighted.
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].Started.Equal(sorted[j].Started) {
			return sorted[i].Started.After(sorted[j].Started)
		}
		if sorted[i].ID != sorted[j].ID {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Description < sorted[j].Description
	})
	return sorted
}
// wrapText word-wraps s to the given column width, hard-breaking any word longer
// than width. It always returns at least one line.
func wrapText(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	var cur string
	for _, word := range strings.Fields(s) {
		for styles.StringWidth(word) > width {
			// Hard-break an over-long word so it never overflows the column.
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			r := []rune(word)
			lines = append(lines, string(r[:width]))
			word = string(r[width:])
		}
		switch {
		case cur == "":
			cur = word
		case styles.StringWidth(cur)+1+styles.StringWidth(word) <= width:
			cur += " " + word
		default:
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	return lines
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
	if !m.WeekView {
		if mark := m.holidayLabel(m.Date); mark != "" {
			viewLabel += "  " + mark
		}
	}
	var title string
	if m.CalmMode {
		title = fmt.Sprintf("  Worklogs — %s (%d entries)", viewLabel, len(sorted))
	} else {
		total := totalTime(sorted)
		title = fmt.Sprintf("  Worklogs — %s (%d entries, %s)", viewLabel, len(sorted), styles.FormatDuration(total))
	}
	b.WriteString(styles.HeaderFmt.Render(title))
	b.WriteString("\n")

	if !m.CalmMode {
		if line := m.efficiencyLine(totalTime(sorted)); line != "" {
			b.WriteString(line)
		}
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

// holidayLabel returns a short marker (e.g. "holiday", "sick" or "4h off") if
// the given date has any holiday or sick-day entry, or "" otherwise.
func (m Model) holidayLabel(date time.Time) string {
	if m.Holidays.IsFullDay(date) {
		return "holiday"
	}
	if m.SickDays.IsFullDay(date) {
		return "sick"
	}
	if !m.Holidays.HasHoliday(date) && !m.SickDays.HasHoliday(date) {
		return ""
	}
	if m.CalmMode {
		return "partial off"
	}
	full := m.DailyHours
	if full <= 0 {
		return "partial off"
	}
	eff := m.SickDays.EffectiveDailyHours(date, m.Holidays.EffectiveDailyHours(date, full, m.BusinessHours), m.BusinessHours)
	off := full - eff
	if off <= 0 {
		return ""
	}
	return fmt.Sprintf("%s off", styles.FormatDuration(time.Duration(off*float64(time.Hour))))
}

func (m Model) dailyTarget() time.Duration {
	return m.dailyTargetFor(m.Date)
}

func (m Model) dailyTargetFor(date time.Time) time.Duration {
	if m.DailyHours <= 0 {
		return 0
	}
	eff := m.SickDays.EffectiveDailyHours(date, m.Holidays.EffectiveDailyHours(date, m.DailyHours, m.BusinessHours), m.BusinessHours)
	return time.Duration(eff * float64(time.Hour))
}

func (m Model) weeklyTarget() time.Duration {
	if m.DailyHours <= 0 || len(m.WorkDays) == 0 {
		if m.WeeklyHours <= 0 {
			return 0
		}
		return time.Duration(m.WeeklyHours * float64(time.Hour))
	}
	weekday := int(m.Date.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := time.Date(m.Date.Year(), m.Date.Month(), m.Date.Day()-weekday+1, 0, 0, 0, 0, m.Date.Location())
	var total time.Duration
	for i := 0; i < 7; i++ {
		d := monday.AddDate(0, 0, i)
		iso := int(d.Weekday())
		if iso == 0 {
			iso = 7
		}
		if !m.WorkDays[iso] {
			continue
		}
		total += m.dailyTargetFor(d)
	}
	return total
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
		remainingStr = style.Render(styles.FormatDuration(remaining) + " to target")
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

	// Render rows up front so we can window by actual terminal-line height
	// (calm-mode notes wrap), keeping the cursor row fully visible.
	rows := make([]string, len(sorted))
	heights := make([]int, len(sorted))
	for i, e := range sorted {
		rows[i] = m.renderEntryRow(e, i == m.Cursor)
		heights[i] = lineHeight(rows[i])
	}

	startIdx, endIdx := windowAround(heights, m.Cursor, visibleHeight)

	for i := startIdx; i < endIdx; i++ {
		b.WriteString(rows[i])
		b.WriteString("\n")
	}

	if startIdx > 0 || endIdx < len(sorted) {
		b.WriteString(styles.HintStyle.Render(fmt.Sprintf("\n  Showing %d-%d of %d", startIdx+1, endIdx, len(sorted))))
		b.WriteString("\n")
	}
}

func (m Model) renderWeekView(b *strings.Builder, sorted []msg.WorklogEntry) {
	// Group entries by day
	groups := make(map[string][]msg.WorklogEntry)
	for _, e := range sorted {
		day := e.Started.Format("2006-01-02")
		groups[day] = append(groups[day], e)
	}

	// Build the full Mon–Sun day list anchored on m.Date so empty days still
	// render with their target / holiday marker.
	weekday := int(m.Date.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := time.Date(m.Date.Year(), m.Date.Month(), m.Date.Day()-weekday+1, 0, 0, 0, 0, m.Date.Location())

	// Pre-render every display line (headers, entry rows, blank separators) to
	// its final text so the window can be sized by actual terminal-line height —
	// calm-mode notes wrap, so an entry row is not always one line.
	type displayLine struct {
		text   string // final rendered content, no trailing newline; "" = blank separator
		height int
	}
	var lines []displayLine
	cursorDisplayIdx := 0
	flatIdx := 0

	// Iterate Sun→Mon so the most recent day renders at the top, matching the
	// descending sort. The flatIdx→sorted coupling holds because both walk
	// entries in descending-time order.
	for i := 6; i >= 0; i-- {
		t := monday.AddDate(0, 0, i)
		key := t.Format("2006-01-02")
		entries := groups[key]
		iso := int(t.Weekday())
		if iso == 0 {
			iso = 7
		}
		// Only render days configured as work days in businessHours.
		if !m.WorkDays[iso] {
			continue
		}
		var header string
		if m.CalmMode {
			header = t.Format("Mon Jan 2")
		} else {
			dayTotal := totalTime(entries)
			header = fmt.Sprintf("%s  %s", t.Format("Mon Jan 2"), styles.FormatDuration(dayTotal))
			if dt := m.dailyTargetFor(t); dt > 0 {
				header += "  " + m.formatEfficiency(dayTotal, dt)
			}
		}
		if mark := m.holidayLabel(t); mark != "" {
			header += "  " + mark
		}
		lines = append(lines, displayLine{text: styles.HeaderFmt.Render("  " + header), height: 1})
		for range entries {
			row := m.renderEntryRow(sorted[flatIdx], flatIdx == m.Cursor)
			if flatIdx == m.Cursor {
				cursorDisplayIdx = len(lines)
			}
			lines = append(lines, displayLine{text: row, height: lineHeight(row)})
			flatIdx++
		}
		lines = append(lines, displayLine{text: "", height: 1}) // blank separator
	}

	visibleHeight := m.Height - 8
	if visibleHeight < 3 {
		visibleHeight = 3
	}

	heights := make([]int, len(lines))
	for di, dl := range lines {
		heights[di] = dl.height
	}
	startIdx, endIdx := windowAround(heights, cursorDisplayIdx, visibleHeight)

	for di := startIdx; di < endIdx; di++ {
		b.WriteString(lines[di].text)
		b.WriteString("\n")
	}
}

// renderEntryRow returns one worklog row's content (leading cursor included, no
// trailing newline). Calm mode delegates to renderCalmEntry, which may wrap a
// long note across several lines; otherwise it is the timed single-line render
// with the selected row highlighted.
func (m Model) renderEntryRow(e msg.WorklogEntry, isSelected bool) string {
	if m.CalmMode {
		return m.renderCalmEntry(e, isSelected)
	}
	line := m.formatEntryLine(e)
	cursor := "  "
	if isSelected {
		cursor = "> "
		line = styles.SelectedStyle.Render(line)
	}
	return cursor + line
}

// writeEntryLine writes one worklog row plus a trailing newline.
func (m Model) writeEntryLine(b *strings.Builder, e msg.WorklogEntry, isSelected bool) {
	b.WriteString(m.renderEntryRow(e, isSelected))
	b.WriteString("\n")
}

// lineHeight reports how many terminal lines a rendered row occupies.
func lineHeight(s string) int {
	return strings.Count(s, "\n") + 1
}

// windowAround returns the [start, end) index range over heights that keeps the
// focus index visible within at most visible terminal lines, expanding outward
// from focus (up then down) so the focused row stays roughly centered. It
// accounts for variable per-row heights, so calm-mode rows that wrap don't push
// the cursor off-screen.
func windowAround(heights []int, focus, visible int) (int, int) {
	n := len(heights)
	if n == 0 {
		return 0, 0
	}
	if focus < 0 {
		focus = 0
	}
	if focus >= n {
		focus = n - 1
	}
	start := focus
	used := heights[focus]
	up, down := focus-1, focus+1
	for {
		grew := false
		if up >= 0 && used+heights[up] <= visible {
			used += heights[up]
			start = up
			up--
			grew = true
		}
		if down < n && used+heights[down] <= visible {
			used += heights[down]
			down++
			grew = true
		}
		if !grew {
			break
		}
	}
	return start, down
}

// renderCalmEntry renders a calm-mode worklog row: cursor + project dot + issue
// key + note, with long notes wrapped onto indented continuation lines. The row
// style is applied per line *after* the dot so the dot's ANSI reset cannot clip
// the highlight (which otherwise leaves the first line unstyled while wrapped
// lines stay bold).
func (m Model) renderCalmEntry(e msg.WorklogEntry, isSelected bool) string {
	dot := styles.FormatProjectDot(e.Project)
	key := e.IssueKey
	if key == "" {
		key = e.Project
	}
	if key == "" {
		key = e.IssueSummary
	}
	note := e.Description
	if note == "" {
		note = "-"
	}
	// noteCol = cursor(2) + dot(2) + key(10) + gaps(2) = 16.
	const noteCol = 16
	width := m.Width - noteCol
	if width < 10 {
		width = 10
	}
	keyCol := styles.PadOrTruncate(key, 10)
	wrapped := wrapText(note, width)
	indent := strings.Repeat(" ", noteCol)
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	var sb strings.Builder
	if isSelected {
		sb.WriteString(cursor + dot + styles.SelectedStyle.Render(keyCol+"  "+wrapped[0]))
		for _, l := range wrapped[1:] {
			sb.WriteString("\n" + indent + styles.SelectedStyle.Render(l))
		}
	} else {
		sb.WriteString(cursor + dot + keyCol + "  " + styles.HintStyle.Render(wrapped[0]))
		for _, l := range wrapped[1:] {
			sb.WriteString("\n" + indent + styles.HintStyle.Render(l))
		}
	}
	return sb.String()
}

func (m Model) formatEntryLine(e msg.WorklogEntry) string {
	dot := styles.FormatProjectDot(e.Project)

	dur := styles.FormatDurationPadded(e.TimeSpent)

	// Date-only entries (e.g. Jibble) have no clock time and no issue title.
	// Render a compact line — "Client / Project" label, duration, note — instead
	// of a misleading 00:00 range and an empty, space-hungry summary column.
	if e.DateOnly {
		label := e.IssueSummary
		if label == "" {
			label = e.Project
		}
		if label == "" {
			label = e.IssueKey
		}
		labelW := 28
		if maxW := m.Width / 3; labelW > maxW {
			labelW = maxW
		}
		if labelW < 10 {
			labelW = 10
		}
		labelCol := styles.PadOrTruncate(label, labelW)
		note := e.Description
		if note == "" {
			note = "-"
		}
		remaining := m.Width - (labelW + 15) // cursor(2)+dot(2)+dur(5)+gaps(6)
		if remaining < 10 {
			remaining = 10
		}
		note = styles.Truncate(note, remaining)
		return fmt.Sprintf("%s%s  %s  %s", dot, labelCol, dur, styles.HintStyle.Render(note))
	}

	timeRange := e.Started.Format("15:04") + "–" + e.Started.Add(e.TimeSpent).Format("15:04")
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
