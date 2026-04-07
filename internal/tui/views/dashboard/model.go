package dashboard

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/iruoy/fylla/internal/tui/msg"
	"github.com/iruoy/fylla/internal/tui/styles"
)

// Model is the dashboard view model.
type Model struct {
	Entries          []msg.WorklogEntry
	Loading          bool
	Err              error
	Width            int
	Height           int
	Month            time.Time // first day of the selected month
	DailyHours       float64
	WeeklyHours      float64
	EfficiencyTarget float64
	WorkDays         map[int]bool // ISO weekday numbers (1=Mon..7=Sun)
	ScrollOffset     int
}

// New creates a new dashboard model.
func New(dailyHours, weeklyHours, efficiencyTarget float64, workDays []int) Model {
	now := time.Now()
	wd := make(map[int]bool, len(workDays))
	for _, d := range workDays {
		wd[d] = true
	}
	if len(wd) == 0 {
		// Default Mon-Fri
		for i := 1; i <= 5; i++ {
			wd[i] = true
		}
	}
	return Model{
		Loading:          true,
		Month:            time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()),
		DailyHours:       dailyHours,
		WeeklyHours:      weeklyHours,
		EfficiencyTarget: efficiencyTarget,
		WorkDays:         wd,
	}
}

// DateRange returns the since/until dates for the current month.
func (m *Model) DateRange() (time.Time, time.Time) {
	since := m.Month
	until := since.AddDate(0, 1, -1)
	return since, until
}

// PrevMonth navigates to the previous month.
func (m *Model) PrevMonth() {
	m.Month = m.Month.AddDate(0, -1, 0)
	m.ScrollOffset = 0
}

// NextMonth navigates to the next month, clamped to the current month.
func (m *Model) NextMonth() {
	now := time.Now()
	current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	next := m.Month.AddDate(0, 1, 0)
	if !next.After(current) {
		m.Month = next
	}
	m.ScrollOffset = 0
}

// GoToCurrentMonth resets to the current month.
func (m *Model) GoToCurrentMonth() {
	now := time.Now()
	m.Month = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	m.ScrollOffset = 0
}

// IsCurrentMonth reports whether the selected month is the current month.
func (m *Model) IsCurrentMonth() bool {
	now := time.Now()
	return m.Month.Year() == now.Year() && m.Month.Month() == now.Month()
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// ScrollUp scrolls the dashboard content up.
func (m *Model) ScrollUp() {
	if m.ScrollOffset > 0 {
		m.ScrollOffset--
	}
}

// ScrollDown scrolls the dashboard content down.
func (m *Model) ScrollDown() {
	m.ScrollOffset++
}

// projectStats holds aggregated stats for one project.
type projectStats struct {
	project string
	total   time.Duration
	entries int
}

// View renders the dashboard.
func (m Model) View() string {
	if m.Loading {
		return "  Loading dashboard..."
	}
	if m.Err != nil {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	var b strings.Builder

	// Title
	var monthLabel string
	if m.IsCurrentMonth() {
		monthLabel = "This Month"
	} else {
		monthLabel = m.Month.Format("January 2006")
	}
	monthTotal := totalTime(m.Entries)
	title := fmt.Sprintf("Dashboard — %s (%d entries, %s)", monthLabel, len(m.Entries), styles.FormatDuration(monthTotal))
	b.WriteString(styles.HeaderFmt.Render(title))
	b.WriteString("\n\n")

	if len(m.Entries) == 0 {
		b.WriteString("  No worklogs found for this month.\n")
	} else {
		m.renderMonthSummary(&b, monthTotal)
		b.WriteString("\n")
		m.renderCalendarGrid(&b)
		b.WriteString("\n")
		m.renderProjectBreakdown(&b, monthTotal)
	}

	b.WriteString("\n")
	hints := "h/l:prev/next month  T:current month  j/k:scroll  r:refresh"
	b.WriteString(styles.HintStyle.Render("  " + hints))

	// Apply scroll
	lines := strings.Split(b.String(), "\n")
	visibleHeight := m.Height - 3
	if visibleHeight < 5 {
		visibleHeight = 5
	}
	if m.ScrollOffset > len(lines)-visibleHeight {
		m.ScrollOffset = len(lines) - visibleHeight
	}
	if m.ScrollOffset < 0 {
		m.ScrollOffset = 0
	}
	end := m.ScrollOffset + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[m.ScrollOffset:end], "\n")
}

func (m Model) renderMonthSummary(b *strings.Builder, monthTotal time.Duration) {
	// Count working days (Mon-Fri) in the month up to today
	now := time.Now()
	endDate := time.Date(m.Month.Year(), m.Month.Month()+1, 0, 0, 0, 0, 0, m.Month.Location())
	if endDate.After(now) {
		endDate = now
	}
	workingDays := countWorkingDays(m.Month, endDate, m.WorkDays)

	dailyTarget := time.Duration(m.DailyHours * float64(time.Hour))
	expectedTotal := time.Duration(workingDays) * dailyTarget

	b.WriteString(styles.HeaderFmt.Render("  Summary"))
	b.WriteString("\n")

	avgDaily := time.Duration(0)
	if workingDays > 0 {
		avgDaily = monthTotal / time.Duration(workingDays)
	}

	b.WriteString(fmt.Sprintf("  Working days: %d", workingDays))
	b.WriteString(fmt.Sprintf("    Avg daily: %s", styles.FormatDuration(avgDaily)))
	if dailyTarget > 0 {
		pct := float64(avgDaily) / float64(dailyTarget) * 100
		b.WriteString(fmt.Sprintf(" / %s", styles.FormatDuration(dailyTarget)))
		b.WriteString("  " + colorPct(pct, m.EfficiencyTarget))
	}
	b.WriteString("\n")

	if expectedTotal > 0 {
		efficiency := float64(monthTotal) / float64(expectedTotal) * 100
		b.WriteString(fmt.Sprintf("  Month total: %s / %s", styles.FormatDuration(monthTotal), styles.FormatDuration(expectedTotal)))
		b.WriteString("  " + colorPct(efficiency, m.EfficiencyTarget))
		b.WriteString("\n")
	}
}

func (m Model) renderCalendarGrid(b *strings.Builder) {
	dailyTotals := make(map[string]time.Duration)
	for _, e := range m.Entries {
		day := e.Started.Format("2006-01-02")
		dailyTotals[day] += e.TimeSpent
	}

	dailyTarget := time.Duration(m.DailyHours * float64(time.Hour))
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	b.WriteString(styles.HeaderFmt.Render("  Calendar"))
	b.WriteString("\n")

	// Day-of-week headers (Mon–Sun)
	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	cellWidth := 10
	if m.Width > 0 {
		cellWidth = (m.Width - 4) / 7
		if cellWidth < 8 {
			cellWidth = 8
		}
		if cellWidth > 12 {
			cellWidth = 12
		}
	}

	b.WriteString("  ")
	for i, name := range dayNames {
		iso := i + 1 // 1=Mon..7=Sun
		label := styles.PadOrTruncate(name, cellWidth)
		if m.WorkDays[iso] {
			b.WriteString(styles.HeaderFmt.Render(label))
		} else {
			b.WriteString(styles.HintStyle.Render(label))
		}
	}
	b.WriteString("\n")

	// Find the Monday on or before the 1st of the month
	first := m.Month
	wd := isoWeekday(first.Weekday())
	gridStart := first.AddDate(0, 0, -(wd - 1))

	// Find the last day of the month
	lastDay := m.Month.AddDate(0, 1, -1)
	// Extend grid to Sunday after last day
	lastWd := isoWeekday(lastDay.Weekday())
	gridEnd := lastDay.AddDate(0, 0, 7-lastWd)

	for d := gridStart; !d.After(gridEnd); {
		b.WriteString("  ")
		for col := 0; col < 7; col++ {
			inMonth := d.Month() == m.Month.Month() && d.Year() == m.Month.Year()
			iso := col + 1

			if !inMonth {
				// Outside current month — blank cell
				b.WriteString(styles.PadOrTruncate("", cellWidth))
			} else {
				key := d.Format("2006-01-02")
				total := dailyTotals[key]
				dayNum := fmt.Sprintf("%d", d.Day())

				if !m.WorkDays[iso] {
					// Non-work day
					if total > 0 {
						cell := fmt.Sprintf("%s %s", dayNum, styles.FormatDuration(total))
						b.WriteString(styles.HintStyle.Render(styles.PadOrTruncate(cell, cellWidth)))
					} else {
						b.WriteString(styles.HintStyle.Render(styles.PadOrTruncate(dayNum, cellWidth)))
					}
				} else if d.After(today) {
					// Future day
					b.WriteString(styles.HintStyle.Render(styles.PadOrTruncate(dayNum, cellWidth)))
				} else if total == 0 {
					// Work day with no logged time
					b.WriteString(styles.ErrStyle.Render(styles.PadOrTruncate(dayNum+" -", cellWidth)))
				} else {
					cell := fmt.Sprintf("%s %s", dayNum, styles.FormatDuration(total))
					var style lipgloss.Style
					if dailyTarget > 0 {
						ratio := float64(total) / float64(dailyTarget)
						switch {
						case ratio >= m.EfficiencyTarget:
							style = styles.CurrentStyle
						case ratio >= m.EfficiencyTarget-0.1:
							style = styles.WarnStyle
						default:
							style = styles.ErrStyle
						}
					} else {
						style = styles.CurrentStyle
					}
					b.WriteString(style.Render(styles.PadOrTruncate(cell, cellWidth)))
				}
			}
			d = d.AddDate(0, 0, 1)
		}
		b.WriteString("\n")
	}
}

func (m Model) renderProjectBreakdown(b *strings.Builder, monthTotal time.Duration) {
	projects := m.computeProjectStats()

	b.WriteString(styles.HeaderFmt.Render("  Project Breakdown"))
	b.WriteString("\n")

	b.WriteString(styles.HintStyle.Render("  Project                    Hours     Entries  Share"))
	b.WriteString("\n")

	for _, ps := range projects {
		pct := float64(0)
		if monthTotal > 0 {
			pct = float64(ps.total) / float64(monthTotal) * 100
		}
		dot := styles.FormatProjectDot(ps.project)
		name := styles.PadOrTruncate(ps.project, 25)
		line := fmt.Sprintf("  %s%s  %-10s%-9d%.1f%%",
			dot, name, styles.FormatDuration(ps.total), ps.entries, pct)
		b.WriteString(line)
		b.WriteString("\n")

		// Proportion bar
		if monthTotal > 0 {
			bar := renderProportionBar(ps.total, monthTotal, ps.project, m.Width-6)
			b.WriteString("      " + bar)
			b.WriteString("\n")
		}
	}
}

func (m Model) renderDailyHeatmap(b *strings.Builder) {
	b.WriteString(styles.HeaderFmt.Render("  Daily Activity"))
	b.WriteString("\n")

	dailyTotals := make(map[string]time.Duration)
	for _, e := range m.Entries {
		day := e.Started.Format("2006-01-02")
		dailyTotals[day] += e.TimeSpent
	}

	dailyTarget := time.Duration(m.DailyHours * float64(time.Hour))

	// Iterate through all days of the month
	endDate := m.Month.AddDate(0, 1, -1)
	now := time.Now()
	if endDate.After(now) {
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	for d := m.Month; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		iso := isoWeekday(d.Weekday())
		if !m.WorkDays[iso] {
			continue
		}

		key := d.Format("2006-01-02")
		total := dailyTotals[key]

		dayLabel := d.Format("Mon 02")
		hours := styles.FormatDuration(total)

		var indicator string
		if dailyTarget > 0 {
			ratio := float64(total) / float64(dailyTarget)
			indicator = heatBlock(ratio, m.EfficiencyTarget)
		}

		b.WriteString(fmt.Sprintf("  %s  %s  %s", dayLabel, styles.PadOrTruncate(hours, 6), indicator))
		b.WriteString("\n")
	}
}

func (m Model) computeWeekStats() []weekStats {
	weekMap := make(map[int]*weekStats)
	var weekNums []int

	for _, e := range m.Entries {
		_, wk := e.Started.ISOWeek()
		ws, ok := weekMap[wk]
		if !ok {
			// Find Monday of this ISO week
			weekStart := isoWeekStart(e.Started)
			ws = &weekStats{
				weekNum:   wk,
				weekStart: weekStart,
				weekEnd:   weekStart.AddDate(0, 0, 6),
				byProject: make(map[string]time.Duration),
			}
			weekMap[wk] = ws
			weekNums = append(weekNums, wk)
		}
		ws.total += e.TimeSpent
		ws.entries++
		ws.byProject[e.Project] += e.TimeSpent
	}

	sort.Ints(weekNums)
	result := make([]weekStats, 0, len(weekNums))
	for _, wk := range weekNums {
		ws := weekMap[wk]
		ws.workingDays = countWorkingDays(ws.weekStart, ws.weekEnd, m.WorkDays)
		result = append(result, *ws)
	}
	return result
}

func (m Model) computeProjectStats() []projectStats {
	projectMap := make(map[string]*projectStats)
	var projects []string

	for _, e := range m.Entries {
		proj := e.Project
		if proj == "" {
			proj = "(no project)"
		}
		ps, ok := projectMap[proj]
		if !ok {
			ps = &projectStats{project: proj}
			projectMap[proj] = ps
			projects = append(projects, proj)
		}
		ps.total += e.TimeSpent
		ps.entries++
	}

	result := make([]projectStats, 0, len(projects))
	for _, p := range projects {
		result = append(result, *projectMap[p])
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].total > result[j].total
	})
	return result
}

func totalTime(entries []msg.WorklogEntry) time.Duration {
	var total time.Duration
	for _, e := range entries {
		total += e.TimeSpent
	}
	return total
}

func countWorkingDays(from, to time.Time, workDays map[int]bool) int {
	count := 0
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		iso := isoWeekday(d.Weekday())
		if workDays[iso] {
			count++
		}
	}
	return count
}

func isoWeekday(wd time.Weekday) int {
	d := int(wd)
	if d == 0 {
		d = 7
	}
	return d
}

func isoWeekStart(t time.Time) time.Time {
	wd := t.Weekday()
	if wd == time.Sunday {
		wd = 7
	}
	return time.Date(t.Year(), t.Month(), t.Day()-int(wd)+1, 0, 0, 0, 0, t.Location())
}

func colorPct(pct, target float64) string {
	label := fmt.Sprintf("%.1f%%", pct)
	targetPct := target * 100
	switch {
	case pct >= targetPct:
		return styles.CurrentStyle.Render(label)
	case pct >= targetPct-10:
		return styles.WarnStyle.Render(label)
	default:
		return styles.ErrStyle.Render(label)
	}
}

func renderBar(value, target time.Duration, effTarget float64, maxWidth int) string {
	if maxWidth < 10 {
		maxWidth = 10
	}
	barWidth := maxWidth - 2 // leave room for brackets
	if barWidth < 5 {
		barWidth = 5
	}

	ratio := float64(value) / float64(target)
	filled := int(math.Round(ratio * float64(barWidth)))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}

	// Target marker position
	targetPos := int(math.Round(effTarget * float64(barWidth)))
	if targetPos > barWidth {
		targetPos = barWidth
	}

	var barColor lipgloss.Style
	switch {
	case ratio >= effTarget:
		barColor = styles.CurrentStyle
	case ratio >= effTarget-0.1:
		barColor = styles.WarnStyle
	default:
		barColor = styles.ErrStyle
	}

	bar := make([]rune, barWidth)
	for i := range bar {
		if i < filled {
			bar[i] = '█'
		} else if i == targetPos {
			bar[i] = '│'
		} else {
			bar[i] = '░'
		}
	}

	return barColor.Render(string(bar))
}

func renderProportionBar(value, total time.Duration, project string, maxWidth int) string {
	if maxWidth < 10 {
		maxWidth = 10
	}
	barWidth := maxWidth - 2
	if barWidth < 5 {
		barWidth = 5
	}

	ratio := float64(value) / float64(total)
	filled := int(math.Round(ratio * float64(barWidth)))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 1 {
		filled = 1
	}

	bar := make([]rune, barWidth)
	for i := range bar {
		if i < filled {
			bar[i] = '█'
		} else {
			bar[i] = '░'
		}
	}

	return styles.ProjectBadgeStyle(project).Render(string(bar))
}

func heatBlock(ratio, target float64) string {
	switch {
	case ratio >= target:
		return styles.CurrentStyle.Render("████")
	case ratio >= target-0.1:
		return styles.WarnStyle.Render("███░")
	case ratio >= 0.5:
		return styles.WarnStyle.Render("██░░")
	case ratio > 0:
		return styles.ErrStyle.Render("█░░░")
	default:
		return styles.HintStyle.Render("░░░░")
	}
}
