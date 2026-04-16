package config

import (
	"fmt"
	"strings"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/tui/styles"
)

type rowKind int

const (
	rowSection rowKind = iota
	rowSetting
)

type configRow struct {
	Kind  rowKind
	Label string
	Key   string
	Value string
}

// Model is the config view model.
type Model struct {
	Rows    []configRow
	Cursor  int
	Loading bool
	Err     error
	Width   int
	Height  int
}

// New creates a new config model.
func New() Model {
	return Model{Loading: true}
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// SetConfig rebuilds the row list from a parsed config.
func (m *Model) SetConfig(cfg *config.Config) {
	m.Rows = buildRows(cfg)
	m.Loading = false
	m.Err = nil
	// Clamp cursor
	if m.Cursor >= len(m.Rows) {
		m.Cursor = 0
	}
	m.snapCursorToSetting()
}

// CursorUp moves the cursor to the previous setting row.
func (m *Model) CursorUp() {
	for i := m.Cursor - 1; i >= 0; i-- {
		if m.Rows[i].Kind == rowSetting {
			m.Cursor = i
			return
		}
	}
	// Wrap to last setting
	for i := len(m.Rows) - 1; i > m.Cursor; i-- {
		if m.Rows[i].Kind == rowSetting {
			m.Cursor = i
			return
		}
	}
}

// CursorDown moves the cursor to the next setting row.
func (m *Model) CursorDown() {
	for i := m.Cursor + 1; i < len(m.Rows); i++ {
		if m.Rows[i].Kind == rowSetting {
			m.Cursor = i
			return
		}
	}
	// Wrap to first setting
	for i := 0; i < m.Cursor; i++ {
		if m.Rows[i].Kind == rowSetting {
			m.Cursor = i
			return
		}
	}
}

// SelectedRow returns the row at the cursor, or nil.
func (m *Model) SelectedRow() *configRow {
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) && m.Rows[m.Cursor].Kind == rowSetting {
		return &m.Rows[m.Cursor]
	}
	return nil
}

func (m *Model) snapCursorToSetting() {
	if m.Cursor < len(m.Rows) && m.Rows[m.Cursor].Kind == rowSetting {
		return
	}
	for i, r := range m.Rows {
		if r.Kind == rowSetting {
			m.Cursor = i
			return
		}
	}
}

// View renders the config view.
func (m *Model) View() string {
	if m.Loading {
		return "  Loading config..."
	}
	if m.Err != nil {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}
	if len(m.Rows) == 0 {
		return "  No config found.\n"
	}

	// Find max label width for alignment
	maxLabel := 0
	for _, r := range m.Rows {
		if r.Kind == rowSetting && len(r.Label) > maxLabel {
			maxLabel = len(r.Label)
		}
	}

	var lines []string

	for i, r := range m.Rows {
		switch r.Kind {
		case rowSection:
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, "  "+styles.SectionFmt.Render(r.Label))
		case rowSetting:
			marker := "  "
			label := r.Label
			value := r.Value
			if value == "" {
				value = styles.HintStyle.Render("(empty)")
			}
			padded := label + strings.Repeat(" ", maxLabel-len(label))
			line := fmt.Sprintf("    %s  %s", padded, value)
			if i == m.Cursor {
				marker = styles.SelectedStyle.Render("> ")
				line = styles.SelectedStyle.Render(fmt.Sprintf("    %s  %s", padded, r.Value))
				if r.Value == "" {
					line = styles.SelectedStyle.Render(fmt.Sprintf("    %s  ", padded)) + styles.HintStyle.Render("(empty)")
				}
			}
			lines = append(lines, marker+line)
		}
	}

	lines = append(lines, "")
	hints := "j/k:navigate  enter:edit  r:refresh"
	lines = append(lines, styles.HintStyle.Render("  "+hints))

	// Viewport scrolling
	visibleHeight := m.Height - 2
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	// Calculate scroll offset to keep cursor visible
	// Map cursor to rendered line index
	cursorLine := m.cursorLineIndex(lines)
	start := 0
	if len(lines) > visibleHeight {
		margin := 3
		if cursorLine-margin < start {
			start = cursorLine - margin
		}
		if cursorLine+margin >= start+visibleHeight {
			start = cursorLine + margin - visibleHeight + 1
		}
		if start < 0 {
			start = 0
		}
		if start+visibleHeight > len(lines) {
			start = len(lines) - visibleHeight
		}
		if start < 0 {
			start = 0
		}
	}

	end := start + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

func (m *Model) cursorLineIndex(lines []string) int {
	// The cursor row index maps to a specific line in the rendered output.
	// Lines: header, blank, then for each row: section adds blank+header, setting adds one line.
	lineIdx := 2 // header + blank
	for i, r := range m.Rows {
		if i == m.Cursor {
			return lineIdx
		}
		switch r.Kind {
		case rowSection:
			if i > 0 {
				lineIdx++ // blank before section
			}
			lineIdx++ // section header
		case rowSetting:
			lineIdx++
		}
	}
	return lineIdx
}

func buildRows(cfg *config.Config) []configRow {
	var rows []configRow

	// Providers (always shown)
	rows = append(rows, configRow{Kind: rowSection, Label: "Providers"})
	rows = append(rows, configRow{Kind: rowSetting, Label: "providers", Key: "providers", Value: formatStringSlice(cfg.Providers)})

	// Todoist
	rows = append(rows, configRow{Kind: rowSection, Label: "Todoist"})
	rows = append(rows, configRow{Kind: rowSetting, Label: "defaultFilter", Key: "todoist.defaultFilter", Value: cfg.Todoist.DefaultFilter})
	rows = append(rows, configRow{Kind: rowSetting, Label: "defaultProject", Key: "todoist.defaultProject", Value: cfg.Todoist.DefaultProject})
	rows = append(rows, configRow{Kind: rowSetting, Label: "credentials", Key: "todoist.credentials", Value: cfg.Todoist.Credentials})

	// GitHub
	rows = append(rows, configRow{Kind: rowSection, Label: "GitHub"})
	rows = append(rows, configRow{Kind: rowSetting, Label: "defaultQuery", Key: "github.defaultQuery", Value: cfg.GitHub.DefaultQuery})
	rows = append(rows, configRow{Kind: rowSetting, Label: "repos", Key: "github.repos", Value: formatStringSlice(cfg.GitHub.Repos)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "credentials", Key: "github.credentials", Value: cfg.GitHub.Credentials})

	// Local
	rows = append(rows, configRow{Kind: rowSection, Label: "Local"})
	rows = append(rows, configRow{Kind: rowSetting, Label: "storePath", Key: "local.storePath", Value: cfg.Local.StorePath})
	rows = append(rows, configRow{Kind: rowSetting, Label: "defaultFilter", Key: "local.defaultFilter", Value: cfg.Local.DefaultFilter})
	rows = append(rows, configRow{Kind: rowSetting, Label: "defaultProject", Key: "local.defaultProject", Value: cfg.Local.DefaultProject})

	// Kendo
	rows = append(rows, configRow{Kind: rowSection, Label: "Kendo"})
	rows = append(rows, configRow{Kind: rowSetting, Label: "url", Key: "kendo.url", Value: cfg.Kendo.URL})
	rows = append(rows, configRow{Kind: rowSetting, Label: "defaultFilter", Key: "kendo.defaultFilter", Value: cfg.Kendo.DefaultFilter})
	rows = append(rows, configRow{Kind: rowSetting, Label: "defaultProject", Key: "kendo.defaultProject", Value: cfg.Kendo.DefaultProject})
	rows = append(rows, configRow{Kind: rowSetting, Label: "doneLane", Key: "kendo.doneLane", Value: cfg.Kendo.DoneLane})
	rows = append(rows, configRow{Kind: rowSetting, Label: "credentials", Key: "kendo.credentials", Value: cfg.Kendo.Credentials})

	// Calendar
	rows = append(rows, configRow{Kind: rowSection, Label: "Calendar"})
	rows = append(rows, configRow{Kind: rowSetting, Label: "sourceCalendars", Key: "calendar.sourceCalendars", Value: formatStringSlice(cfg.Calendar.SourceCalendars)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "fyllaCalendar", Key: "calendar.fyllaCalendar", Value: cfg.Calendar.FyllaCalendar})
	rows = append(rows, configRow{Kind: rowSetting, Label: "credentials", Key: "calendar.credentials", Value: cfg.Calendar.Credentials})

	// Scheduling
	rows = append(rows, configRow{Kind: rowSection, Label: "Scheduling"})
	rows = append(rows, configRow{Kind: rowSetting, Label: "windowDays", Key: "scheduling.windowDays", Value: fmt.Sprintf("%d", cfg.Scheduling.WindowDays)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "minTaskDuration", Key: "scheduling.minTaskDurationMinutes", Value: fmt.Sprintf("%dm", cfg.Scheduling.MinTaskDurationMinutes)})
	maxDur := "unlimited"
	if cfg.Scheduling.MaxTaskDurationMinutes > 0 {
		maxDur = fmt.Sprintf("%dm", cfg.Scheduling.MaxTaskDurationMinutes)
	}
	rows = append(rows, configRow{Kind: rowSetting, Label: "maxTaskDuration", Key: "scheduling.maxTaskDurationMinutes", Value: maxDur})
	rows = append(rows, configRow{Kind: rowSetting, Label: "bufferMinutes", Key: "scheduling.bufferMinutes", Value: fmt.Sprintf("%d", cfg.Scheduling.BufferMinutes)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "travelBuffer", Key: "scheduling.travelBufferMinutes", Value: fmt.Sprintf("%d", cfg.Scheduling.TravelBufferMinutes)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "snapMinutes", Key: "scheduling.snapMinutes", Value: formatIntSlice(cfg.Scheduling.SnapMinutes)})

	// Business Hours
	rows = append(rows, configRow{Kind: rowSection, Label: "Business Hours"})
	for i, bh := range cfg.BusinessHours {
		rows = append(rows, configRow{Kind: rowSetting, Label: fmt.Sprintf("window %d", i+1), Key: fmt.Sprintf("businessHours[%d]", i), Value: formatBusinessHours(bh)})
	}

	// Project Rules
	if len(cfg.ProjectRules) > 0 {
		rows = append(rows, configRow{Kind: rowSection, Label: "Project Rules"})
		for name, windows := range cfg.ProjectRules {
			for i, bh := range windows {
				label := name
				if len(windows) > 1 {
					label = fmt.Sprintf("%s [%d]", name, i+1)
				}
				rows = append(rows, configRow{Kind: rowSetting, Label: label, Key: fmt.Sprintf("projectRules.%s[%d]", name, i), Value: formatBusinessHours(bh)})
			}
		}
	}

	// Weights
	rows = append(rows, configRow{Kind: rowSection, Label: "Weights"})
	rows = append(rows, configRow{Kind: rowSetting, Label: "priority", Key: "weights.priority", Value: formatFloat(cfg.Weights.Priority)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "dueDate", Key: "weights.dueDate", Value: formatFloat(cfg.Weights.DueDate)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "estimate", Key: "weights.estimate", Value: formatFloat(cfg.Weights.Estimate)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "age", Key: "weights.age", Value: formatFloat(cfg.Weights.Age)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "upNext", Key: "weights.upNext", Value: formatFloat(cfg.Weights.UpNext)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "typeBonus", Key: "weights.typeBonus", Value: formatFloatMap(cfg.Weights.TypeBonus)})

	// Worklog
	rows = append(rows, configRow{Kind: rowSection, Label: "Worklog"})
	rows = append(rows, configRow{Kind: rowSetting, Label: "provider", Key: "worklog.provider", Value: cfg.Worklog.Provider})
	rows = append(rows, configRow{Kind: rowSetting, Label: "fallbackIssues", Key: "worklog.fallbackIssues", Value: formatStringSlice(cfg.Worklog.FallbackIssues)})

	// Efficiency
	rows = append(rows, configRow{Kind: rowSection, Label: "Efficiency"})
	rows = append(rows, configRow{Kind: rowSetting, Label: "weeklyHours", Key: "efficiency.weeklyHours", Value: formatFloat(cfg.Efficiency.WeeklyHours)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "dailyHours", Key: "efficiency.dailyHours", Value: formatFloat(cfg.Efficiency.DailyHours)})
	rows = append(rows, configRow{Kind: rowSetting, Label: "target", Key: "efficiency.target", Value: fmt.Sprintf("%.0f%%", cfg.Efficiency.Target*100)})

	return rows
}

func formatStringSlice(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return "[" + strings.Join(s, ", ") + "]"
}

func formatStringMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, k+": "+v)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatFloatMap(m map[string]float64) string {
	if len(m) == 0 {
		return ""
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s: %g", k, v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatIntSlice(s []int) string {
	if len(s) == 0 {
		return ""
	}
	parts := make([]string, len(s))
	for i, v := range s {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatFloat(f float64) string {
	if f == float64(int(f)) {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%g", f)
}

func formatBusinessHours(bh config.BusinessHoursConfig) string {
	days := ""
	if len(bh.WorkDays) > 0 {
		dayNames := []string{"", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
		first := bh.WorkDays[0]
		last := bh.WorkDays[len(bh.WorkDays)-1]
		// Check if days are consecutive
		consecutive := true
		for i := 1; i < len(bh.WorkDays); i++ {
			if bh.WorkDays[i] != bh.WorkDays[i-1]+1 {
				consecutive = false
				break
			}
		}
		if consecutive && len(bh.WorkDays) > 2 && first >= 1 && first <= 7 && last >= 1 && last <= 7 {
			days = dayNames[first] + "–" + dayNames[last]
		} else {
			parts := make([]string, len(bh.WorkDays))
			for i, d := range bh.WorkDays {
				if d >= 1 && d <= 7 {
					parts[i] = dayNames[d]
				} else {
					parts[i] = fmt.Sprintf("%d", d)
				}
			}
			days = strings.Join(parts, ", ")
		}
	}
	return fmt.Sprintf("%s–%s  %s", bh.Start, bh.End, days)
}
