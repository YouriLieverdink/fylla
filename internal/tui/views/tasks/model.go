package tasks

import (
	"fmt"
	"strings"
	"time"

	"github.com/iruoy/fylla/internal/tui/msg"
	"github.com/iruoy/fylla/internal/tui/styles"
)

// Model is the tasks view model.
type Model struct {
	Tasks   []msg.ScoredTask
	Cursor  int
	Loading bool
	Err     error
	Width   int
	Height  int
	Filter  string
	filterMode bool
}

// New creates a new tasks model.
func New() Model {
	return Model{Loading: true}
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// SelectedTask returns the currently selected task, or nil.
func (m *Model) SelectedTask() *msg.ScoredTask {
	filtered := m.filteredTasks()
	if len(filtered) == 0 || m.Cursor < 0 || m.Cursor >= len(filtered) {
		return nil
	}
	return &filtered[m.Cursor]
}

// CursorUp moves the cursor up.
func (m *Model) CursorUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// CursorDown moves the cursor down.
func (m *Model) CursorDown() {
	filtered := m.filteredTasks()
	if m.Cursor < len(filtered)-1 {
		m.Cursor++
	}
}

// IsFilterMode returns true if filter input is active.
func (m *Model) IsFilterMode() bool {
	return m.filterMode
}

// ToggleFilter enters or exits filter mode.
func (m *Model) ToggleFilter() {
	m.filterMode = !m.filterMode
	if !m.filterMode {
		m.Filter = ""
		m.Cursor = 0
	}
}

// ClearFilter clears the filter and exits filter mode.
func (m *Model) ClearFilter() {
	m.Filter = ""
	m.filterMode = false
	m.Cursor = 0
}

// AppendFilter adds a character to the filter.
func (m *Model) AppendFilter(ch rune) {
	m.Filter += string(ch)
	m.Cursor = 0
}

// BackspaceFilter removes the last character from the filter.
func (m *Model) BackspaceFilter() {
	if len(m.Filter) > 0 {
		m.Filter = m.Filter[:len(m.Filter)-1]
		m.Cursor = 0
	}
}

func (m *Model) filteredTasks() []msg.ScoredTask {
	now := time.Now()
	var source []msg.ScoredTask
	if m.Filter == "" {
		source = m.Tasks
	} else {
		lower := strings.ToLower(m.Filter)
		for _, t := range m.Tasks {
			if strings.Contains(strings.ToLower(t.Summary), lower) ||
				strings.Contains(strings.ToLower(t.Key), lower) ||
				strings.Contains(strings.ToLower(t.Project), lower) ||
				strings.Contains(strings.ToLower(t.Section), lower) ||
				strings.Contains(strings.ToLower(t.Status), lower) {
				source = append(source, t)
			}
		}
	}
	// Return actionable tasks first, then deferred (not-before in the future).
	var actionable, deferred []msg.ScoredTask
	for _, t := range source {
		if t.NotBefore != nil && t.NotBefore.After(now) {
			deferred = append(deferred, t)
		} else {
			actionable = append(actionable, t)
		}
	}
	return append(actionable, deferred...)
}

func (m *Model) splitPoint(filtered []msg.ScoredTask) int {
	now := time.Now()
	for i, t := range filtered {
		if t.NotBefore != nil && t.NotBefore.After(now) {
			return i
		}
	}
	return len(filtered)
}

// View renders the tasks view.
func (m Model) View() string {
	if m.Loading {
		return "  Loading tasks..."
	}
	if m.Err != nil {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	filtered := m.filteredTasks()

	var b strings.Builder

	title := fmt.Sprintf("Tasks (%d)", len(m.Tasks))
	if m.Filter != "" {
		title = fmt.Sprintf("Tasks (%d/%d) filter: %s", len(filtered), len(m.Tasks), m.Filter)
	}
	b.WriteString(styles.HeaderFmt.Render(title))
	b.WriteString("\n\n")

	if len(filtered) == 0 {
		if m.Filter != "" {
			b.WriteString("  No tasks match the filter.")
		} else {
			b.WriteString("  No tasks found.")
		}
		b.WriteString("\n")
	} else {
		split := m.splitPoint(filtered)
		nActionable := split
		nDeferred := len(filtered) - split

		// Build display lines: each line is either a task (with its flat index) or a header.
		type displayLine struct {
			taskIdx int    // index into filtered, or -1 for header
			header  string // non-empty for header lines
		}
		var lines []displayLine
		if nActionable > 0 {
			lines = append(lines, displayLine{taskIdx: -1, header: fmt.Sprintf("Actionable (%d)", nActionable)})
			for i := 0; i < nActionable; i++ {
				lines = append(lines, displayLine{taskIdx: i})
			}
		}
		if nDeferred > 0 {
			if nActionable > 0 {
				lines = append(lines, displayLine{taskIdx: -1}) // blank separator
			}
			lines = append(lines, displayLine{taskIdx: -1, header: fmt.Sprintf("Not yet (%d)", nDeferred)})
			for i := split; i < len(filtered); i++ {
				lines = append(lines, displayLine{taskIdx: i})
			}
		}

		// Calculate visible range for scrolling based on cursor position in display lines.
		// Find the display line index for the current cursor.
		cursorDisplayIdx := 0
		for di, dl := range lines {
			if dl.taskIdx == m.Cursor {
				cursorDisplayIdx = di
				break
			}
		}

		visibleHeight := m.Height - 6
		if visibleHeight < 3 {
			visibleHeight = 3
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
			if dl.taskIdx == -1 {
				if dl.header != "" {
					b.WriteString(styles.HeaderFmt.Render("  " + dl.header))
				}
				b.WriteString("\n")
				continue
			}

			t := filtered[dl.taskIdx]
			isSelected := dl.taskIdx == m.Cursor

			dot := styles.FormatProjectDot(t.Project)
			rank := fmt.Sprintf("%2d.", dl.taskIdx+1)
			est := styles.FormatDurationPadded(t.Estimate)
			score := fmt.Sprintf("%5.1f", t.Score)

			label := styles.FormatPrefix(t.Project, t.Section) + styles.Truncate(t.Summary, 50)
			line := fmt.Sprintf("%s %s  %s  %s", rank, label, est, score)

			if t.Status != "" {
				line += styles.HintStyle.Render(fmt.Sprintf(" [%s]", t.Status))
			}
			if t.UpNext {
				line += styles.UpNextStyle.Render(" [UP NEXT]")
			}
			if t.NotBefore != nil && t.NotBefore.After(time.Now()) {
				line += styles.HintStyle.Render(fmt.Sprintf(" [not before %s]", t.NotBefore.Format("Jan 2")))
			}

			cursor := "  "
			if isSelected {
				cursor = "> "
				line = styles.SelectedStyle.Render(line)
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
	}

	b.WriteString("\n")
	if m.filterMode {
		b.WriteString(styles.HintStyle.Render("  Type to filter, Esc to clear"))
	} else {
		hints := "j/k:navigate  t/enter:timer  d:done  D:delete  m:move  a:add  e:edit  S:snooze  /:filter  r:refresh"
		b.WriteString(styles.HintStyle.Render("  " + hints))
	}

	return b.String()
}
