package dashboard

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/iruoy/fylla/internal/config"
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
	Month            time.Time
	DailyHours       float64
	WeeklyHours      float64
	EfficiencyTarget float64
	WorkDays         map[int]bool
	BusinessHours    []config.BusinessHoursConfig
	Holidays         config.HolidayIndex
	ScrollOffset     int
}

// New creates a new dashboard model.
func New(dailyHours, weeklyHours, efficiencyTarget float64, workDays []int, businessHours []config.BusinessHoursConfig, holidays config.HolidayIndex) Model {
	now := time.Now()
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
		Month:            time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()),
		DailyHours:       dailyHours,
		WeeklyHours:      weeklyHours,
		EfficiencyTarget: efficiencyTarget,
		WorkDays:         wd,
		BusinessHours:    businessHours,
		Holidays:         holidays,
	}
}

// DateRange returns the since/until dates for the current month.
func (m *Model) DateRange() (time.Time, time.Time) {
	since := m.Month
	until := since.AddDate(0, 1, -1)
	return since, until
}

func (m *Model) PrevMonth() {
	m.Month = m.Month.AddDate(0, -1, 0)
	m.ScrollOffset = 0
}

func (m *Model) NextMonth() {
	now := time.Now()
	current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	next := m.Month.AddDate(0, 1, 0)
	if !next.After(current) {
		m.Month = next
	}
	m.ScrollOffset = 0
}

func (m *Model) GoToCurrentMonth() {
	now := time.Now()
	m.Month = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	m.ScrollOffset = 0
}

func (m *Model) IsCurrentMonth() bool {
	now := time.Now()
	return m.Month.Year() == now.Year() && m.Month.Month() == now.Month()
}

func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

func (m *Model) ScrollUp() {
	if m.ScrollOffset > 0 {
		m.ScrollOffset--
	}
}

func (m *Model) ScrollDown() {
	m.ScrollOffset++
}

// dailyTargetFor returns the holiday-adjusted target for a single date.
// Returns 0 if the day is not a workday or is a full-day holiday.
func (m Model) dailyTargetFor(d time.Time) time.Duration {
	iso := isoWeekday(d.Weekday())
	if !m.WorkDays[iso] {
		return 0
	}
	if m.DailyHours <= 0 {
		return 0
	}
	eff := m.Holidays.EffectiveDailyHours(d, m.DailyHours, m.BusinessHours)
	if eff <= 0 {
		return 0
	}
	return time.Duration(eff * float64(time.Hour))
}

type projectStats struct {
	project string
	total   time.Duration
	entries int
}

type monthStats struct {
	logged       time.Duration
	expected     time.Duration
	workingDays  int
	loggedDays   int
	missedDays   int
	holidayDays  int
	dailyTotals  map[string]time.Duration
	dailyTargets map[string]time.Duration
}

// View renders the dashboard.
func (m Model) View() string {
	if m.Loading {
		return "  Loading dashboard..."
	}
	if m.Err != nil {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	stats := m.computeMonthStats()

	var sections []string
	sections = append(sections, m.renderTitle())
	sections = append(sections, m.renderKPIs(stats))

	if len(m.Entries) == 0 && stats.workingDays == 0 {
		sections = append(sections, styles.HintStyle.Render("No worklogs and no working days in this month."))
	} else {
		sections = append(sections, m.renderBody(stats))
	}

	sections = append(sections, m.renderHints())

	rendered := indentLines(strings.Join(sections, "\n"), "  ")
	return m.applyScroll(rendered)
}

func (m Model) renderTitle() string {
	var monthLabel string
	if m.IsCurrentMonth() {
		monthLabel = "This Month — " + m.Month.Format("January 2006")
	} else {
		monthLabel = m.Month.Format("January 2006")
	}
	return styles.SectionFmt.Render(monthLabel)
}

func (m Model) renderKPIs(stats monthStats) string {
	available := m.Width - 3
	if available < 40 {
		available = 40
	}
	tileCount := 3
	tileTotal := available / tileCount
	if tileTotal < 18 {
		tileTotal = 18
	}
	// lipgloss Width includes padding but excludes border, so border adds 2.
	contentWidth := tileTotal - 2
	remainder := available - tileTotal*tileCount

	w0 := contentWidth
	w1 := contentWidth
	w2 := contentWidth
	if remainder > 0 {
		w0++
	}
	if remainder > 1 {
		w1++
	}

	tiles := []string{
		m.kpiThisMonth(stats, w0),
		m.kpiAvgPerDay(stats, w1),
		m.kpiMissed(stats, w2),
	}

	border := lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}
	rendered := make([]string, len(tiles))
	for i, t := range tiles {
		rendered[i] = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(0, 1).
			Width(w(i, w0, w1, w2)).
			Render(t)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func w(i, a, b, c int) int {
	switch i {
	case 0:
		return a
	case 1:
		return b
	default:
		return c
	}
}

func (m Model) kpiThisMonth(stats monthStats, w int) string {
	value := styles.FormatDuration(stats.logged)
	sub := ""
	if stats.expected > 0 {
		sub = "/ " + styles.FormatDuration(stats.expected) + "  " + colorPct(float64(stats.logged)/float64(stats.expected)*100, m.EfficiencyTarget)
	}
	return kpiBlock("Month", value, sub, w)
}

func (m Model) kpiAvgPerDay(stats monthStats, w int) string {
	avg := time.Duration(0)
	if stats.loggedDays > 0 {
		avg = stats.logged / time.Duration(stats.loggedDays)
	}
	dailyTarget := time.Duration(m.DailyHours * float64(time.Hour))
	value := styles.FormatDuration(avg)
	sub := ""
	if dailyTarget > 0 && avg > 0 {
		sub = "/ " + styles.FormatDuration(dailyTarget) + "  " + colorPct(float64(avg)/float64(dailyTarget)*100, m.EfficiencyTarget)
	} else {
		sub = styles.HintStyle.Render(fmt.Sprintf("%d days logged", stats.loggedDays))
	}
	return kpiBlock("Avg / day", value, sub, w)
}

func (m Model) kpiMissed(stats monthStats, w int) string {
	value := fmt.Sprintf("%d", stats.missedDays)
	sub := styles.HintStyle.Render(fmt.Sprintf("%d work · %d hol", stats.workingDays, stats.holidayDays))
	style := styles.CurrentStyle
	if stats.missedDays > 0 {
		style = styles.WarnStyle
	}
	if stats.missedDays > 2 {
		style = styles.ErrStyle
	}
	return kpiBlock("Missed", style.Render(value), sub, w)
}

func kpiBlock(label, value, sub string, _ int) string {
	h := styles.HintStyle.Render(strings.ToUpper(label))
	if sub == "" {
		return h + "\n" + value
	}
	return h + "\n" + value + "\n" + sub
}

func (m Model) renderBody(stats monthStats) string {
	calWidth, projWidth := m.bodyWidths()

	calendar := m.renderCalendarPanel(stats, calWidth)
	projects := m.renderProjectsPanel(stats, projWidth)

	if calWidth+projWidth+3 > m.Width || m.Width < 100 {
		return calendar + "\n" + projects
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, calendar, "  ", projects)
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func (m Model) bodyWidths() (int, int) {
	w := m.Width
	if w < 100 {
		full := w - 3
		if full < 40 {
			full = 40
		}
		return full, full
	}
	avail := w - 5
	cal := int(float64(avail) * 0.55)
	proj := avail - cal
	if cal < 50 {
		cal = 50
	}
	if proj < 30 {
		proj = 30
	}
	return cal, proj
}

func (m Model) renderCalendarPanel(stats monthStats, width int) string {
	var b strings.Builder
	b.WriteString(styles.HeaderFmt.Render("Calendar — heatmap"))
	b.WriteString("\n\n")

	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	cellWidth := (width - 6) / 8 // 7 days + 1 week-total column
	if cellWidth < 6 {
		cellWidth = 6
	}
	if cellWidth > 12 {
		cellWidth = 12
	}

	// Header row
	for i, name := range dayNames {
		iso := i + 1
		label := styles.PadOrTruncate(name, cellWidth)
		if m.WorkDays[iso] {
			b.WriteString(styles.HeaderFmt.Render(label))
		} else {
			b.WriteString(styles.HintStyle.Render(label))
		}
	}
	b.WriteString(styles.HeaderFmt.Render(styles.PadOrTruncate("Σ", cellWidth)))
	b.WriteString("\n")

	first := m.Month
	gridStart := first.AddDate(0, 0, -(isoWeekday(first.Weekday()) - 1))
	lastDay := m.Month.AddDate(0, 1, -1)
	gridEnd := lastDay.AddDate(0, 0, 7-isoWeekday(lastDay.Weekday()))

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for d := gridStart; !d.After(gridEnd); {
		var weekTotal time.Duration
		hadInMonth := false
		var weekRow strings.Builder
		for col := 0; col < 7; col++ {
			weekRow.WriteString(m.renderCalendarCell(d, today, stats, cellWidth))
			if d.Month() == m.Month.Month() && d.Year() == m.Month.Year() {
				hadInMonth = true
				key := d.Format("2006-01-02")
				weekTotal += stats.dailyTotals[key]
			}
			d = d.AddDate(0, 0, 1)
		}
		b.WriteString(weekRow.String())
		if hadInMonth {
			b.WriteString(styles.HintStyle.Render(styles.PadOrTruncate(styles.FormatDuration(weekTotal), cellWidth)))
		} else {
			b.WriteString(styles.PadOrTruncate("", cellWidth))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHeatmapLegend())

	return panelBox(b.String(), width)
}

func (m Model) renderCalendarCell(d, today time.Time, stats monthStats, cellWidth int) string {
	inMonth := d.Month() == m.Month.Month() && d.Year() == m.Month.Year()
	if !inMonth {
		return styles.PadOrTruncate("", cellWidth)
	}

	iso := isoWeekday(d.Weekday())
	key := d.Format("2006-01-02")
	total := stats.dailyTotals[key]
	target := stats.dailyTargets[key]
	dayNum := fmt.Sprintf("%d", d.Day())

	isToday := d.Equal(today)
	isFullHoliday := m.Holidays.IsFullDay(d)
	isPartialHoliday := !isFullHoliday && m.Holidays.HasHoliday(d) && m.WorkDays[iso]
	isFuture := d.After(today)
	isWorkday := m.WorkDays[iso] && !isFullHoliday

	var cellText string
	switch {
	case isFullHoliday:
		cellText = dayNum + " ⛱"
	case !isWorkday:
		if total > 0 {
			cellText = fmt.Sprintf("%s %s", dayNum, styles.FormatDuration(total))
		} else {
			cellText = dayNum
		}
	case isFuture:
		if isPartialHoliday {
			cellText = dayNum + " ½"
		} else {
			cellText = dayNum
		}
	case total == 0:
		cellText = dayNum + " ·"
	default:
		cellText = fmt.Sprintf("%s %s", dayNum, styles.FormatDuration(total))
	}

	cellStr := styles.PadOrTruncate(cellText, cellWidth)

	style := m.cellStyle(d, total, target, isWorkday, isFuture, isFullHoliday, isPartialHoliday)
	if isToday {
		style = style.Underline(true)
	}
	return style.Render(cellStr)
}

func (m Model) cellStyle(d time.Time, total, target time.Duration, isWorkday, isFuture, isFullHoliday, isPartialHoliday bool) lipgloss.Style {
	if isFullHoliday {
		return styles.HintStyle.Italic(true)
	}
	if !isWorkday {
		if total > 0 {
			return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"})
		}
		return styles.HintStyle
	}
	if isFuture {
		if isPartialHoliday {
			return styles.HintStyle.Italic(true)
		}
		return styles.HintStyle
	}
	if total == 0 {
		return styles.ErrStyle
	}
	if target == 0 {
		return styles.CurrentStyle
	}
	ratio := float64(total) / float64(target)
	return heatmapStyle(ratio, m.EfficiencyTarget, isPartialHoliday)
}

// heatmapStyle maps logged/target ratio to a color, using the same thresholds
// as colorPct: red < target-10%, amber < target, green < 110%, blue ≥ 110%.
func heatmapStyle(ratio, target float64, isPartialHoliday bool) lipgloss.Style {
	red := lipgloss.AdaptiveColor{Light: "#FFD9DD", Dark: "#5A2A33"}
	amber := lipgloss.AdaptiveColor{Light: "#FFE2A8", Dark: "#6A4A1A"}
	green := lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	blue := lipgloss.AdaptiveColor{Light: "#1E88E5", Dark: "#42A5F5"}

	var bg lipgloss.AdaptiveColor
	switch {
	case ratio >= 1.1:
		bg = blue
	case ratio >= target:
		bg = green
	case ratio >= target-0.1:
		bg = amber
	default:
		bg = red
	}
	style := lipgloss.NewStyle().
		Background(bg).
		Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
		Bold(true)
	if isPartialHoliday {
		style = style.Italic(true)
	}
	return style
}

func (m Model) renderHeatmapLegend() string {
	t := m.EfficiencyTarget
	amber := int((t - 0.1) * 100)
	hit := int(t * 100)
	swatches := []struct {
		label string
		ratio float64
	}{
		{fmt.Sprintf("<%d%%", amber), t - 0.2},
		{fmt.Sprintf("%d–%d%%", amber, hit-1), t - 0.05},
		{fmt.Sprintf("≥%d%%", hit), t},
		{">110%", 1.2},
	}
	hint := styles.HintStyle
	var b strings.Builder
	b.WriteString("\n")
	for _, s := range swatches {
		b.WriteString(heatmapStyle(s.ratio, t, false).Render("  "))
		b.WriteString(" " + hint.Render(s.label) + "  ")
	}
	b.WriteString("\n")
	b.WriteString(hint.Render("⛱ holiday   · no log   ½ partial   _ today"))
	return b.String()
}

func (m Model) renderProjectsPanel(stats monthStats, width int) string {
	projects := m.computeProjectStats()

	var b strings.Builder
	b.WriteString(styles.HeaderFmt.Render("Projects"))
	b.WriteString("\n\n")

	if len(projects) == 0 {
		b.WriteString(styles.HintStyle.Render("  No projects yet."))
		return panelBox(b.String(), width)
	}

	nameW := width - 31
	if nameW < 12 {
		nameW = 12
	}

	header := fmt.Sprintf("  %s  %-7s  %-4s  %-7s",
		styles.PadOrTruncate("Project", nameW),
		"Hours", "Cnt", "Share")
	b.WriteString(styles.HintStyle.Render(header))
	b.WriteString("\n")

	for _, ps := range projects {
		pct := float64(0)
		if stats.logged > 0 {
			pct = float64(ps.total) / float64(stats.logged) * 100
		}
		dot := styles.FormatProjectDot(ps.project)
		name := styles.PadOrTruncate(ps.project, nameW)
		line := fmt.Sprintf("  %s%s  %-7s  %-4d  %5.1f%%",
			dot, name, styles.FormatDuration(ps.total), ps.entries, pct)
		b.WriteString(line)
		b.WriteString("\n")

		barWidth := width - 8
		if barWidth > 6 {
			b.WriteString("    " + renderProportionBar(ps.total, stats.logged, ps.project, barWidth))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.HintStyle.Render(fmt.Sprintf("  %d projects · %s total", len(projects), styles.FormatDuration(stats.logged))))

	return panelBox(b.String(), width)
}

func panelBox(content string, width int) string {
	inner := width - 2
	if inner < 10 {
		inner = 10
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}).
		Padding(0, 1).
		Width(inner).
		Render(content)
}

func (m Model) renderHints() string {
	hints := "h/l:prev/next month  T:current  j/k:scroll  r:refresh"
	return styles.HintStyle.Render(hints)
}

func (m Model) applyScroll(rendered string) string {
	lines := strings.Split(rendered, "\n")
	visibleHeight := m.Height - 2
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
	if m.ScrollOffset >= len(lines) {
		return ""
	}
	return strings.Join(lines[m.ScrollOffset:end], "\n")
}

func (m Model) computeMonthStats() monthStats {
	stats := monthStats{
		dailyTotals:  make(map[string]time.Duration),
		dailyTargets: make(map[string]time.Duration),
	}
	for _, e := range m.Entries {
		key := e.Started.Format("2006-01-02")
		stats.dailyTotals[key] += e.TimeSpent
		stats.logged += e.TimeSpent
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	lastDay := m.Month.AddDate(0, 1, -1)
	endDate := lastDay
	if endDate.After(today) {
		endDate = today
	}

	for d := m.Month; !d.After(lastDay); d = d.AddDate(0, 0, 1) {
		iso := isoWeekday(d.Weekday())
		target := m.dailyTargetFor(d)
		stats.dailyTargets[d.Format("2006-01-02")] = target

		if !d.After(endDate) {
			if m.Holidays.IsFullDay(d) {
				stats.holidayDays++
			} else if m.WorkDays[iso] && target > 0 {
				stats.workingDays++
				stats.expected += target
				key := d.Format("2006-01-02")
				if stats.dailyTotals[key] > 0 {
					stats.loggedDays++
				} else if d.Before(today) {
					stats.missedDays++
				}
			}
		} else {
			if m.Holidays.IsFullDay(d) {
				stats.holidayDays++
			}
		}
	}
	return stats
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

func isoWeekday(wd time.Weekday) int {
	d := int(wd)
	if d == 0 {
		d = 7
	}
	return d
}

func colorPct(pct, target float64) string {
	label := fmt.Sprintf("%.0f%%", pct)
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

func renderProportionBar(value, total time.Duration, project string, maxWidth int) string {
	if maxWidth < 10 {
		maxWidth = 10
	}
	barWidth := maxWidth - 2
	if barWidth < 5 {
		barWidth = 5
	}
	if total <= 0 {
		return ""
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
