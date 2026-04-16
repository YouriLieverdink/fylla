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
	Tasks      []msg.ScoredTask
	Cursor     int
	Loading    bool
	Err        error
	Width      int
	Height     int
	Filter     string
	filterMode bool
	Selected   map[string]bool // multi-select: key → selected
	SelectMode bool            // true when multi-select is active
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

// ToggleFilter enters or exits filter input mode.
// Exiting preserves the filter text so the list stays filtered.
func (m *Model) ToggleFilter() {
	m.filterMode = !m.filterMode
	if m.filterMode {
		m.Cursor = 0
	}
}

// HasFilter returns true if a non-empty filter is active.
func (m *Model) HasFilter() bool {
	return m.Filter != ""
}

// ToggleSelect toggles the selection of the currently focused task.
func (m *Model) ToggleSelect() {
	t := m.SelectedTask()
	if t == nil {
		return
	}
	if m.Selected == nil {
		m.Selected = make(map[string]bool)
	}
	if m.Selected[t.Key] {
		delete(m.Selected, t.Key)
	} else {
		m.Selected[t.Key] = true
	}
}

// ToggleSelectMode enters or exits multi-select mode.
func (m *Model) ToggleSelectMode() {
	m.SelectMode = !m.SelectMode
	if !m.SelectMode {
		m.Selected = nil
	} else if m.Selected == nil {
		m.Selected = make(map[string]bool)
	}
}

// SelectedKeys returns the keys of all selected tasks.
func (m *Model) SelectedKeys() []string {
	keys := make([]string, 0, len(m.Selected))
	for k := range m.Selected {
		keys = append(keys, k)
	}
	return keys
}

// SelectionCount returns the number of selected tasks.
func (m *Model) SelectionCount() int {
	return len(m.Selected)
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
	if m.Loading && len(m.Tasks) == 0 {
		return "  Loading tasks..."
	}
	if m.Err != nil && len(m.Tasks) == 0 {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	filtered := m.filteredTasks()

	var b strings.Builder

	loadingSuffix := ""
	if m.Loading && len(m.Tasks) > 0 {
		loadingSuffix = " ..."
	}
	title := fmt.Sprintf("  Tasks (%d)%s", len(m.Tasks), loadingSuffix)
	if m.Filter != "" {
		title = fmt.Sprintf("  Tasks (%d/%d) filter: %s%s", len(filtered), len(m.Tasks), m.Filter, loadingSuffix)
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

		// Calculate visible range for scrolling, keeping cursor in view.
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

		// Center the cursor in the visible window.
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

			// Build compact tags.
			var tags string
			if t.Status != "" {
				tags += " " + styles.AbbrevStatus(t.Status)
			}
			if t.UpNext {
				tags += styles.UpNextStyle.Render(" ↑")
			}
			if t.NotBefore != nil && t.NotBefore.After(time.Now()) {
				tags += styles.HintStyle.Render(" ≥" + t.NotBefore.Format("Jan 2"))
			}

			// Fixed parts: cursor(2) + dot(2) + rank(3) + gaps(6) + est(5) + score(5) = 23
			// Tags and label share the remaining space.
			tagsWidth := styles.StringWidth(tags)
			labelWidth := m.Width - 23 - tagsWidth
			if labelWidth < 20 {
				labelWidth = 20
			}

			prefix := styles.FormatPrefix(t.Project, t.Section)
			summaryWidth := labelWidth - len(prefix)
			if summaryWidth < 10 {
				summaryWidth = 10
			}
			label := styles.PadOrTruncate(prefix+styles.Truncate(t.Summary, summaryWidth), labelWidth)

			line := fmt.Sprintf("%s  %s  %s  %s%s", rank, label, est, score, tags)

			cursor := "  "
			if isSelected {
				cursor = "> "
				line = styles.SelectedStyle.Render(line)
			}

			// Multi-select checkbox
			check := ""
			if m.SelectMode {
				if m.Selected != nil && m.Selected[t.Key] {
					check = "[x] "
				} else {
					check = "[ ] "
				}
			}

			b.WriteString(cursor)
			b.WriteString(check)
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
		b.WriteString(styles.HintStyle.Render("  Type to filter, Enter/Esc to confirm"))
	} else if m.SelectMode {
		count := m.SelectionCount()
		hints := fmt.Sprintf("  space:toggle  d:done %d  D:delete %d  Esc:exit select", count, count)
		b.WriteString(styles.HintStyle.Render(hints))
	} else {
		filterHint := "/:filter"
		if m.HasFilter() {
			filterHint = "/:clear filter"
		}
		hints := fmt.Sprintf("j/k:navigate  t/enter:timer  d:done  D:delete  m:move  a:add  e:edit  S:snooze  space:select  %s  r:refresh", filterHint)
		b.WriteString(styles.HintStyle.Render("  " + hints))
	}

	return b.String()
}
