package timer

import (
	"fmt"
	"strings"
	"time"

	"github.com/iruoy/fylla/internal/tui/styles"
)

// PausedInfo describes a paused timer shown in the timer view.
type PausedInfo struct {
	TaskKey      string
	Project      string
	SegmentCount int
}

// SegmentInfo describes a completed segment shown in the timer view.
type SegmentInfo struct {
	Duration time.Duration
	Comment  string
}

// Model is the timer view model.
type Model struct {
	TaskKey      string
	Summary      string
	Project      string
	Section      string
	Comment      string
	StartTime    time.Time
	Elapsed      time.Duration
	TotalElapsed time.Duration
	Segments     []SegmentInfo // prior completed segments
	Running      bool
	Loading      bool
	Err          error
	Width        int
	Height       int
	Focused      bool
	Paused       []PausedInfo
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

// View renders the timer as a compact side panel.
func (m Model) View() string {
	if m.Loading {
		return "  Loading..."
	}
	if m.Err != nil {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	var b strings.Builder

	// Panel title
	titleStyle := styles.HeaderFmt
	if m.Focused {
		titleStyle = styles.SectionFmt
	}
	b.WriteString("  " + titleStyle.Render("Timer") + "\n\n")

	if !m.Running {
		b.WriteString("  No timer running.\n")
		if len(m.Paused) > 0 {
			b.WriteString("\n")
			m.writePaused(&b)
		}
		return b.String()
	}

	// Task info
	dot := styles.FormatProjectDot(m.Project)
	label := m.Summary
	if label == "" {
		if m.TaskKey != "" {
			label = m.TaskKey
		} else {
			label = "(anonymous)"
		}
	}
	wrapWidth := m.Width - 4
	if wrapWidth < 10 {
		wrapWidth = 10
	}
	wrapped := wordWrap(label, wrapWidth-2) // -2 for dot
	for i, line := range wrapped {
		if i == 0 {
			b.WriteString("  " + dot + styles.TaskStyle.Render(line) + "\n")
		} else {
			b.WriteString("    " + styles.TaskStyle.Render(line) + "\n")
		}
	}
	if !m.StartTime.IsZero() {
		b.WriteString(styles.HintStyle.Render("  Started at "+m.StartTime.Local().Format("15:04")) + "\n")
	}
	b.WriteString("\n")

	// Big elapsed display
	dur := m.Elapsed
	if len(m.Segments) > 0 {
		dur = m.TotalElapsed
	}
	h := int(dur.Hours())
	min := int(dur.Minutes()) % 60
	sec := int(dur.Seconds()) % 60
	b.WriteString("  " + styles.TimerBig.Render(fmt.Sprintf("%02d:%02d:%02d", h, min, sec)) + "\n")
	b.WriteString("\n")

	// Segments
	if len(m.Segments) > 0 {
		for i, seg := range m.Segments {
			line := fmt.Sprintf("  seg %d: %s", i+1, styles.FormatDuration(seg.Duration))
			if seg.Comment != "" {
				line += " — " + seg.Comment
			}
			if m.Width > 0 {
				line = styles.Truncate(line, m.Width-2)
			}
			b.WriteString(styles.HintStyle.Render(line) + "\n")
		}
		curLine := fmt.Sprintf("  seg %d: %s", len(m.Segments)+1, formatSegmentDuration(m.Elapsed))
		if m.Comment != "" {
			curLine += " — " + m.Comment
		}
		if m.Width > 0 {
			curLine = styles.Truncate(curLine, m.Width-2)
		}
		b.WriteString(styles.HintStyle.Render(curLine) + "\n")
		b.WriteString("\n")
	} else if m.Comment != "" {
		wrapWidth := m.Width - 4
		if wrapWidth < 10 {
			wrapWidth = 10
		}
		for _, line := range wordWrap(m.Comment, wrapWidth) {
			b.WriteString(styles.HintStyle.Render("  "+line) + "\n")
		}
		b.WriteString("\n")
	}

	// Key hints
	b.WriteString(styles.HintStyle.Render("  s:stop  c:comment") + "\n")
	b.WriteString(styles.HintStyle.Render("  e:edit  x:abort") + "\n")
	b.WriteString(styles.HintStyle.Render("  i:interrupt") + "\n")

	if len(m.Paused) > 0 {
		b.WriteString("\n")
		m.writePaused(&b)
	}

	return b.String()
}

func (m Model) writePaused(b *strings.Builder) {
	for _, p := range m.Paused {
		plabel := p.TaskKey
		if plabel == "" {
			plabel = "(anonymous)"
		}
		if p.Project != "" {
			plabel = fmt.Sprintf("[%s] %s", p.Project, plabel)
		}
		segments := "seg"
		if p.SegmentCount != 1 {
			segments = "segs"
		}
		b.WriteString(fmt.Sprintf("  Paused: %s (%d %s)\n", styles.TaskStyle.Render(plabel), p.SegmentCount, segments))
	}
}

func wordWrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var lines []string
	for len(s) > width {
		// Find last space within width
		cut := width
		if i := strings.LastIndex(s[:cut], " "); i > 0 {
			cut = i
		}
		lines = append(lines, s[:cut])
		s = strings.TrimLeft(s[cut:], " ")
	}
	if s != "" {
		lines = append(lines, s)
	}
	return lines
}

func formatSegmentDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
