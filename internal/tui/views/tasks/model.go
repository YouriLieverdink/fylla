package tasks

import (
	"fmt"
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
	upNextStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})
)

var priorityNames = map[int]string{
	1: "Highest",
	2: "High",
	3: "Medium",
	4: "Low",
	5: "Lowest",
}

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
	if m.Filter == "" {
		return m.Tasks
	}
	lower := strings.ToLower(m.Filter)
	var result []msg.ScoredTask
	for _, t := range m.Tasks {
		if strings.Contains(strings.ToLower(t.Summary), lower) ||
			strings.Contains(strings.ToLower(t.Key), lower) ||
			strings.Contains(strings.ToLower(t.Project), lower) {
			result = append(result, t)
		}
	}
	return result
}

// View renders the tasks view.
func (m Model) View() string {
	if m.Loading {
		return "  Loading tasks..."
	}
	if m.Err != nil {
		return errStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	filtered := m.filteredTasks()

	var b strings.Builder

	title := fmt.Sprintf("Tasks (%d)", len(m.Tasks))
	if m.Filter != "" {
		title = fmt.Sprintf("Tasks (%d/%d) filter: %s", len(filtered), len(m.Tasks), m.Filter)
	}
	b.WriteString(headerFmt.Render(title))
	b.WriteString("\n\n")

	if len(filtered) == 0 {
		if m.Filter != "" {
			b.WriteString("  No tasks match the filter.")
		} else {
			b.WriteString("  No tasks found.")
		}
		b.WriteString("\n")
	} else {
		// Calculate visible range for scrolling
		visibleHeight := m.Height - 6
		if visibleHeight < 3 {
			visibleHeight = 3
		}
		startIdx := 0
		if m.Cursor >= visibleHeight {
			startIdx = m.Cursor - visibleHeight + 1
		}
		endIdx := startIdx + visibleHeight
		if endIdx > len(filtered) {
			endIdx = len(filtered)
		}

		for i := startIdx; i < endIdx; i++ {
			t := filtered[i]
			isSelected := i == m.Cursor

			rank := fmt.Sprintf("%2d.", i+1)
			est := formatDuration(t.Estimate)
			score := fmt.Sprintf("%5.1f", t.Score)

			label := formatPrefix(t.Project, t.Section) + truncate(t.Summary, 50)
			line := fmt.Sprintf("%s %s  %s  %s", rank, label, est, score)

			if t.UpNext {
				line += upNextStyle.Render(" [UP NEXT]")
			}
			if t.NotBefore != nil && t.NotBefore.After(time.Now()) {
				line += hintStyle.Render(fmt.Sprintf(" [not before %s]", t.NotBefore.Format("Jan 2")))
			}

			cursor := "  "
			if isSelected {
				cursor = "> "
				line = selectedStyle.Render(line)
			}

			b.WriteString(cursor)
			b.WriteString(line)
			b.WriteString("\n")
		}

		if len(filtered) > visibleHeight {
			b.WriteString(hintStyle.Render(fmt.Sprintf("\n  Showing %d-%d of %d", startIdx+1, endIdx, len(filtered))))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.filterMode {
		b.WriteString(hintStyle.Render("  Type to filter, Esc to clear"))
	} else {
		hints := "j/k:navigate  t/enter:timer  d:done  D:delete  a:add  e:edit  /:filter  r:refresh"
		b.WriteString(hintStyle.Render("  " + hints))
	}

	return b.String()
}

func formatPrefix(project, section string) string {
	if project != "" && section != "" {
		return project + " / " + section + ": "
	}
	if project != "" {
		return project + ": "
	}
	return ""
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "  --"
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
