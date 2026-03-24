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
	Elapsed      time.Duration
	TotalElapsed time.Duration
	Segments     []SegmentInfo // prior completed segments
	Running      bool
	Loading      bool
	Err          error
	Width        int
	Height       int
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
		if len(m.Paused) > 0 {
			b.WriteString("\n\n")
			for _, p := range m.Paused {
				label := p.TaskKey
				if label == "" {
					label = "(anonymous)"
				}
				if p.Project != "" {
					label = fmt.Sprintf("[%s] %s", p.Project, label)
				}
				b.WriteString(fmt.Sprintf("  Paused: %s (%d segments)\n", styles.TaskStyle.Render(label), p.SegmentCount))
			}
		}
		return b.String()
	}

	b.WriteString("  " + styles.RunningStyle.Render("RUNNING") + "\n\n")
	label := styles.FormatPrefix(m.Project, m.Section) + m.Summary
	if m.Summary == "" {
		if m.TaskKey != "" {
			label = m.TaskKey
		} else {
			label = "(anonymous)"
		}
	}
	b.WriteString("  Task: " + styles.TaskStyle.Render(label) + "\n\n")

	// Big elapsed display
	if len(m.Segments) > 0 {
		// Show total time (all segments + current) as the main display
		th := int(m.TotalElapsed.Hours())
		tmin := int(m.TotalElapsed.Minutes()) % 60
		tsec := int(m.TotalElapsed.Seconds()) % 60
		totalStr := fmt.Sprintf("  %02d:%02d:%02d", th, tmin, tsec)
		b.WriteString(styles.TimerBig.Render(totalStr))
		b.WriteString("\n")
		// Show each prior segment
		for i, seg := range m.Segments {
			segLabel := fmt.Sprintf("  segment %d: %s", i+1, styles.FormatDuration(seg.Duration))
			if seg.Comment != "" {
				segLabel += " — " + seg.Comment
			}
			b.WriteString(styles.HintStyle.Render(segLabel))
			b.WriteString("\n")
		}
		// Show current segment
		sh := int(m.Elapsed.Hours())
		smin := int(m.Elapsed.Minutes()) % 60
		ssec := int(m.Elapsed.Seconds()) % 60
		curStr := fmt.Sprintf("  segment %d: %02d:%02d:%02d", len(m.Segments)+1, sh, smin, ssec)
		if m.Comment != "" {
			curStr += " — " + m.Comment
		}
		b.WriteString(styles.HintStyle.Render(curStr))
		b.WriteString("\n\n")
	} else {
		h := int(m.Elapsed.Hours())
		min := int(m.Elapsed.Minutes()) % 60
		sec := int(m.Elapsed.Seconds()) % 60
		elapsed := fmt.Sprintf("  %02d:%02d:%02d", h, min, sec)
		b.WriteString(styles.TimerBig.Render(elapsed))
		b.WriteString("\n")
		if m.Comment != "" {
			b.WriteString(styles.HintStyle.Render("  " + m.Comment))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	hints := "s:stop  c:comment  x:abort  i:interrupt  r:refresh"
	b.WriteString(styles.HintStyle.Render("  " + hints))

	if len(m.Paused) > 0 {
		b.WriteString("\n\n")
		for _, p := range m.Paused {
			plabel := p.TaskKey
			if plabel == "" {
				plabel = "(anonymous)"
			}
			if p.Project != "" {
				plabel = fmt.Sprintf("[%s] %s", p.Project, plabel)
			}
			segments := "segment"
			if p.SegmentCount != 1 {
				segments = "segments"
			}
			b.WriteString(fmt.Sprintf("  Paused: %s (%d %s)\n", styles.TaskStyle.Render(plabel), p.SegmentCount, segments))
		}
	}

	return b.String()
}
