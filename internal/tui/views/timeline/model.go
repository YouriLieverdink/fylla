package timeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/iruoy/fylla/internal/tui/msg"
	"github.com/iruoy/fylla/internal/tui/styles"
)

// Model is the timeline view model.
type Model struct {
	Events  []msg.FyllaEvent
	Cursor  int
	Loading bool
	Err     error
	Width   int
	Height  int
}

// New creates a new timeline model.
func New() Model {
	return Model{Loading: true}
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// SelectedEvent returns the currently selected event, or nil.
func (m *Model) SelectedEvent() *msg.FyllaEvent {
	if len(m.Events) == 0 || m.Cursor < 0 || m.Cursor >= len(m.Events) {
		return nil
	}
	return &m.Events[m.Cursor]
}

// CursorUp moves the cursor up.
func (m *Model) CursorUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// CursorDown moves the cursor down.
func (m *Model) CursorDown() {
	if m.Cursor < len(m.Events)-1 {
		m.Cursor++
	}
}

// View renders the timeline view.
func (m Model) View() string {
	if m.Loading {
		return "  Loading today's schedule..."
	}
	if m.Err != nil {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}
	if len(m.Events) == 0 {
		return "  No events scheduled for today."
	}

	now := time.Now()
	var b strings.Builder
	b.WriteString(styles.HeaderFmt.Render("Today's Schedule"))
	b.WriteString("\n\n")

	for i, e := range m.Events {
		isCurrent := !now.Before(e.Start) && now.Before(e.End)
		isPast := now.After(e.End)
		isSelected := i == m.Cursor

		timeRange := fmt.Sprintf("%s - %s", e.Start.Format("15:04"), e.End.Format("15:04"))
		dur := e.End.Sub(e.Start)
		durStr := styles.FormatDurationParens(dur)

		var label string
		if e.IsCalendarEvent {
			label = fmt.Sprintf("%s  %s  %s", timeRange, e.Summary, durStr)
			switch {
			case isPast:
				label = styles.PastStyle.Render(label)
			case isSelected:
				label = styles.SelectedStyle.Render(label)
			default:
				label = styles.CalEventStyle.Render(label)
			}
		} else {
			prefix := ""
			if e.AtRisk {
				prefix = "[LATE] "
			}
			taskLabel := styles.FormatPrefix(e.Project, e.Section) + e.Summary
			label = fmt.Sprintf("%s  %s%s  %s", timeRange, prefix, taskLabel, durStr)
			switch {
			case isSelected:
				label = styles.SelectedStyle.Render(label)
			case isCurrent:
				label = styles.CurrentStyle.Render(label)
			case e.AtRisk:
				label = styles.AtRiskStyle.Render(label)
			case isPast:
				label = styles.PastStyle.Render(label)
			}
		}

		cursor := "  "
		if isSelected {
			cursor = "> "
		}
		if isCurrent && !isSelected {
			cursor = "* "
		}

		b.WriteString(cursor)
		b.WriteString(label)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	hints := "j/k:navigate  enter/t:timer  d:done  D:delete  a:add  S:snooze  v:view  s:sync  R:report  r:refresh"
	b.WriteString(styles.HintStyle.Render("  " + hints))

	return b.String()
}
