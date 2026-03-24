package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// PickerItem represents an item in the picker list.
type PickerItem struct {
	Key      string
	Label    string // displayed text (e.g. "PROJ-123  Fix the bug")
	Provider string // task provider (e.g. "jira", "kendo")
}

// Picker mode constants.
const (
	PickerModeMyTasks  = 0
	PickerModeAllTasks = 1
)

// Picker is a searchable list overlay for selecting from a list of items.
type Picker struct {
	Title   string
	Items   []PickerItem
	Filter  textinput.Model
	Cursor  int
	Active  bool
	Mode    int  // PickerModeMyTasks or PickerModeAllTasks
	Loading bool // true while fetching tasks for mode switch
}

// NewPicker creates a new picker with the given title and items.
func NewPicker(title string, items []PickerItem) Picker {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.PlaceholderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#BBBBBB", Dark: "#555555"})
	ti.Focus()
	ti.Width = 50
	return Picker{
		Title:  title,
		Items:  items,
		Filter: ti,
		Active: true,
	}
}

// Filtered returns the items matching the current filter.
func (p *Picker) Filtered() []PickerItem {
	q := strings.ToLower(p.Filter.Value())
	if q == "" {
		return p.Items
	}
	var out []PickerItem
	for _, item := range p.Items {
		if strings.Contains(strings.ToLower(item.Key), q) ||
			strings.Contains(strings.ToLower(item.Label), q) {
			out = append(out, item)
		}
	}
	return out
}

// Selected returns the currently highlighted item, or nil.
func (p *Picker) Selected() *PickerItem {
	filtered := p.Filtered()
	if len(filtered) == 0 || p.Cursor < 0 || p.Cursor >= len(filtered) {
		return nil
	}
	return &filtered[p.Cursor]
}

// CursorUp moves the cursor up.
func (p *Picker) CursorUp() {
	if p.Cursor > 0 {
		p.Cursor--
	}
}

// CursorDown moves the cursor down.
func (p *Picker) CursorDown() {
	filtered := p.Filtered()
	if p.Cursor < len(filtered)-1 {
		p.Cursor++
	}
}

// ResetCursor resets the cursor to 0 (call after filter changes).
func (p *Picker) ResetCursor() {
	p.Cursor = 0
}

// View renders the picker overlay.
func (p Picker) View(width, height int) string {
	if !p.Active {
		return ""
	}

	// Fixed box dimensions based on terminal size with padding
	boxWidth := width - 6
	boxHeight := height - 4

	// Content area inside box: subtract border(2) + padding(2 top/bottom, 4 left/right)
	contentWidth := boxWidth - 6
	contentHeight := boxHeight - 4

	if contentWidth < 20 {
		contentWidth = 20
	}
	if contentHeight < 6 {
		contentHeight = 6
	}

	// Lines budget: title(1) + blank(1) + filter(1) + blank(1) + hints(1) + blank before hints(1) = 6
	listHeight := contentHeight - 6
	if listHeight < 1 {
		listHeight = 1
	}

	// Max chars per item line (2 for cursor prefix "  " or "> ")
	maxLineLen := contentWidth - 2

	p.Filter.Width = contentWidth
	filtered := p.Filtered()

	if listHeight > len(filtered) {
		listHeight = len(filtered)
	}

	var b strings.Builder
	title := p.Title
	if p.Mode == PickerModeAllTasks {
		title = "Search All Tasks (Enter to select, Esc to cancel)"
	}
	b.WriteString(formTitleStyle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(p.Filter.View())
	b.WriteString("\n\n")

	if p.Loading {
		b.WriteString(formHintStyle.Render("  Searching..."))
	} else if len(filtered) == 0 && p.Mode == PickerModeAllTasks && p.Filter.Value() == "" {
		b.WriteString(formHintStyle.Render("  Type to search all tasks"))
	} else if len(filtered) == 0 {
		b.WriteString(formHintStyle.Render("  No matching tasks"))
	} else {
		startIdx := 0
		if p.Cursor >= listHeight {
			startIdx = p.Cursor - listHeight + 1
		}
		endIdx := startIdx + listHeight
		if endIdx > len(filtered) {
			endIdx = len(filtered)
		}

		for i := startIdx; i < endIdx; i++ {
			item := filtered[i]
			line := item.Label
			if len(line) > maxLineLen {
				line = line[:maxLineLen-3] + "..."
			}
			cursor := "  "
			if i == p.Cursor {
				cursor = "> "
				line = formSelectStyle.Render(line)
			}
			b.WriteString(cursor + line)
			if i < endIdx-1 {
				b.WriteString("\n")
			}
		}

		if len(filtered) > listHeight {
			b.WriteString("\n")
			b.WriteString(formHintStyle.Render(fmt.Sprintf("  %d/%d shown", listHeight, len(filtered))))
		}
	}

	b.WriteString("\n\n")
	modeToggle := "Tab:all tasks"
	if p.Mode == PickerModeAllTasks {
		modeToggle = "Tab:my tasks"
	}
	b.WriteString(formHintStyle.Render("↑/↓:navigate  Enter:select  " + modeToggle + "  Esc:cancel"))

	box := formBorder.
		Width(contentWidth).
		Height(contentHeight).
		Render(b.String())

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
