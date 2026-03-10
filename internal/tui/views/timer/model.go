package timer

import (
	"fmt"
	"strings"
	"time"

	"github.com/iruoy/fylla/internal/tui/styles"
)

// Model is the timer view model.
type Model struct {
	TaskKey  string
	Summary  string
	Project  string
	Section  string
	Elapsed  time.Duration
	Running  bool
	Loading  bool
	Err      error
	Width    int
	Height   int
}

// New creates a new timer model.
func New() Model {
	return Model{Loading: true}
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// View renders the timer view.
func (m Model) View() string {
	if m.Loading {
		return "  Loading timer status..."
	}
	if m.Err != nil {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	var b strings.Builder
	b.WriteString(styles.HeaderFmt.Render("Timer"))
	b.WriteString("\n\n")

	if !m.Running {
		b.WriteString("  No timer running.\n\n")
		b.WriteString(styles.HintStyle.Render("  Start a timer from the Timeline or Tasks tab using 't' or 'Enter'."))
		return b.String()
	}

	b.WriteString("  " + styles.RunningStyle.Render("RUNNING") + "\n\n")
	label := styles.FormatPrefix(m.Project, m.Section) + m.Summary
	if m.Summary == "" {
		label = m.TaskKey
	}
	b.WriteString("  Task: " + styles.TaskStyle.Render(label) + "\n\n")

	// Big elapsed display
	h := int(m.Elapsed.Hours())
	min := int(m.Elapsed.Minutes()) % 60
	sec := int(m.Elapsed.Seconds()) % 60
	elapsed := fmt.Sprintf("  %02d:%02d:%02d", h, min, sec)
	b.WriteString(styles.TimerBig.Render(elapsed))
	b.WriteString("\n\n")

	hints := "s:stop  x:abort  r:refresh"
	b.WriteString(styles.HintStyle.Render("  " + hints))

	return b.String()
}
