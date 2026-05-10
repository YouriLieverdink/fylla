// Package focus is the TUI view model for the Focus tab — a curated subset
// of tasks with manual ordering. Rendering mirrors the tasks tab compact
// view; filter and multi-select are intentionally omitted.
package focus

import (
	"fmt"
	"strings"

	"github.com/iruoy/fylla/internal/tui/msg"
	"github.com/iruoy/fylla/internal/tui/styles"
)

// Model holds the focus tab state.
type Model struct {
	Tasks   []msg.ScoredTask
	Cursor  int
	Loading bool
	Err     error
	Width   int
	Height  int
}

// New returns a fresh focus model.
func New() Model {
	return Model{Loading: true}
}

// SetSize updates view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// SetTasks replaces the ordered task list.
func (m *Model) SetTasks(tasks []msg.ScoredTask) {
	m.Tasks = tasks
	if m.Cursor >= len(tasks) {
		m.Cursor = len(tasks) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
}

// SelectedTask returns the task under the cursor, or nil.
func (m *Model) SelectedTask() *msg.ScoredTask {
	if len(m.Tasks) == 0 || m.Cursor < 0 || m.Cursor >= len(m.Tasks) {
		return nil
	}
	return &m.Tasks[m.Cursor]
}

// CursorUp moves the cursor up one row.
func (m *Model) CursorUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// CursorDown moves the cursor down one row.
func (m *Model) CursorDown() {
	if m.Cursor < len(m.Tasks)-1 {
		m.Cursor++
	}
}

// View renders the focus list.
func (m Model) View() string {
	if m.Loading && len(m.Tasks) == 0 {
		return "  Loading focus..."
	}
	if m.Err != nil && len(m.Tasks) == 0 {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	var b strings.Builder

	loadingSuffix := ""
	if m.Loading && len(m.Tasks) > 0 {
		loadingSuffix = " ..."
	}
	title := fmt.Sprintf("  Focus (%d)%s", len(m.Tasks), loadingSuffix)
	b.WriteString(styles.HeaderFmt.Render(title))
	b.WriteString("\n\n")

	if len(m.Tasks) == 0 {
		b.WriteString(styles.HintStyle.Render("  No focused tasks. Press 'f' on a task in the Tasks tab to add it here."))
		b.WriteString("\n")
		b.WriteString("\n")
		b.WriteString(styles.HintStyle.Render("  j/k:navigate  3:Tasks tab  r:refresh"))
		return b.String()
	}

	visibleHeight := m.Height - 6
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	startIdx := m.Cursor - visibleHeight/2
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > len(m.Tasks)-visibleHeight {
		startIdx = len(m.Tasks) - visibleHeight
	}
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + visibleHeight
	if endIdx > len(m.Tasks) {
		endIdx = len(m.Tasks)
	}

	for i := startIdx; i < endIdx; i++ {
		t := m.Tasks[i]
		isSelected := i == m.Cursor
		cursor := "  "
		if isSelected {
			cursor = "> "
		}
		b.WriteString(m.renderRow(t, i, cursor, isSelected))
	}

	if len(m.Tasks) > visibleHeight {
		b.WriteString(styles.HintStyle.Render(fmt.Sprintf("\n  Showing %d-%d of %d", startIdx+1, endIdx, len(m.Tasks))))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	hints := "j/k:navigate  shift+↑/↓:reorder  f:remove  t/enter:timer  d:done  e:edit  r:refresh"
	b.WriteString(styles.HintStyle.Render("  " + hints))

	return b.String()
}

// renderRow renders a stripped-down focus row: cursor, rank, summary.
// Project, estimate, due date, and tags are intentionally omitted to keep
// the view distraction-free.
func (m Model) renderRow(t msg.ScoredTask, idx int, cursor string, isSelected bool) string {
	rank := fmt.Sprintf("%2d.", idx+1)

	// Fixed parts: cursor(2) + rank(3) + gap(2) = 7
	summaryWidth := m.Width - 7
	if summaryWidth < 10 {
		summaryWidth = 10
	}
	summary := styles.Truncate(t.Summary, summaryWidth)

	line := fmt.Sprintf("%s  %s", rank, summary)
	if isSelected {
		line = styles.SelectedStyle.Render(line)
	}
	return cursor + line + "\n"
}
