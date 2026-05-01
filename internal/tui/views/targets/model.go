package targets

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/iruoy/fylla/internal/tui/msg"
	"github.com/iruoy/fylla/internal/tui/styles"
)

// Model is the targets view model.
type Model struct {
	Items    []msg.TargetProgress
	Cursor   int
	Loading  bool
	Err      error
	Width    int
	Height   int
	Provider string
	// Offset is the relative cycle offset applied to recurring targets
	// (0 = current period, -1 = previous, +1 = next). Fixed-range targets
	// ignore the offset.
	Offset int
}

// New creates a new targets model.
func New(provider string) Model {
	return Model{Loading: true, Provider: provider}
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// SetItems replaces the current items.
func (m *Model) SetItems(items []msg.TargetProgress) {
	m.Items = items
	if m.Cursor >= len(items) {
		m.Cursor = 0
	}
}

// CursorUp moves the selection up.
func (m *Model) CursorUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// CursorDown moves the selection down.
func (m *Model) CursorDown() {
	if m.Cursor < len(m.Items)-1 {
		m.Cursor++
	}
}

// SelectedIndex returns the cursor index, or -1 if there is nothing selected.
func (m Model) SelectedIndex() int {
	if len(m.Items) == 0 {
		return -1
	}
	if m.Cursor < 0 || m.Cursor >= len(m.Items) {
		return -1
	}
	return m.Cursor
}

// Selected returns the selected item, or nil.
func (m Model) Selected() *msg.TargetProgress {
	idx := m.SelectedIndex()
	if idx == -1 {
		return nil
	}
	return &m.Items[idx]
}

// View renders the targets view.
func (m Model) View() string {
	if m.Loading {
		return "  Loading targets..."
	}

	var b strings.Builder

	header := "  Targets"
	if m.Provider != "" {
		header = fmt.Sprintf("  Targets — %s", m.Provider)
	}
	switch {
	case m.Offset < 0:
		header += fmt.Sprintf("   ← %d cycle(s) back", -m.Offset)
	case m.Offset > 0:
		header += fmt.Sprintf("   → %d cycle(s) forward", m.Offset)
	}
	b.WriteString(styles.HeaderFmt.Render(header))
	b.WriteString("\n\n")

	if m.Err != nil {
		b.WriteString(styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err)))
		b.WriteString("\n")
	}

	if m.Provider == "" {
		b.WriteString("  ")
		b.WriteString(styles.HintStyle.Render("No worklog provider configured. Set worklog.provider in config."))
		b.WriteString("\n")
	} else if len(m.Items) == 0 {
		b.WriteString("  ")
		b.WriteString(styles.HintStyle.Render("No targets defined. Press 'a' to add one."))
		b.WriteString("\n")
	} else {
		for i, item := range m.Items {
			b.WriteString(m.renderItem(i, item))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	hints := "j/k:navigate  h/l:prev/next cycle  T:current  a:add  e:edit  d:delete  r:refresh"
	b.WriteString(styles.HintStyle.Render("  " + hints))

	return b.String()
}

func (m Model) renderItem(i int, item msg.TargetProgress) string {
	cursor := "  "
	if i == m.Cursor {
		cursor = "> "
	}

	scope := item.Target.Scope
	if scope == "" {
		scope = "me"
	}
	title := fmt.Sprintf("%s (%s) [%s]", item.Target.Project, item.PeriodLabel, scope)
	if i == m.Cursor {
		title = styles.SelectedStyle.Render(title)
	}

	target := time.Duration(item.Target.Hours * float64(time.Hour))
	var pct float64
	if target > 0 {
		pct = float64(item.Logged) / float64(target)
	}

	bar := renderBar(pct, m.barWidth())
	stats := fmt.Sprintf("%s / %s (%.0f%%)",
		styles.FormatDuration(item.Logged),
		styles.FormatDuration(target),
		pct*100)
	stats = colorize(pct).Render(stats)

	line := fmt.Sprintf("%s%s\n  %s  %s", cursor, title, bar, stats)
	if item.Err != nil {
		line += "\n  " + styles.ErrStyle.Render(fmt.Sprintf("error: %v", item.Err))
	}
	return line
}

func (m Model) barWidth() int {
	w := m.Width - 30
	if w < 10 {
		w = 10
	}
	if w > 50 {
		w = 50
	}
	return w
}

func renderBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(float64(width) * pct)
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return colorize(pct).Render(bar)
}

func colorize(pct float64) lipgloss.Style {
	switch {
	case pct >= 1:
		return styles.CurrentStyle
	case pct >= 0.5:
		return styles.WarnStyle
	default:
		return styles.ErrStyle
	}
}
