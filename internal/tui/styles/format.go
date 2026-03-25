package styles

import (
	"fmt"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// FormatPrefix formats a project/section prefix for display.
func FormatPrefix(project, section string) string {
	if project != "" && section != "" {
		return project + " / " + section + ": "
	}
	if project != "" {
		return project + ": "
	}
	return ""
}

// FormatProjectDot renders a colored dot for a project.
func FormatProjectDot(project string) string {
	if project == "" {
		return ""
	}
	return ProjectBadgeStyle(project).Render("●") + " "
}

// FormatDuration formats a duration as "1h30m", "1h", or "30m". Zero returns "0m".
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	return formatDur(d)
}

// FormatDurationOrDash formats a duration, returning "--" for zero.
func FormatDurationOrDash(d time.Duration) string {
	if d <= 0 {
		return "--"
	}
	return formatDur(d)
}

// FormatDurationPadded formats a duration, returning "  --" for zero.
func FormatDurationPadded(d time.Duration) string {
	if d <= 0 {
		return "  --"
	}
	return formatDur(d)
}

// FormatDurationParens formats a duration in parentheses, e.g. "(1h30m)".
func FormatDurationParens(d time.Duration) string {
	return "(" + formatDur(d) + ")"
}

func formatDur(d time.Duration) string {
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

// Truncate truncates a string (ANSI-aware) to the given width with an ellipsis.
func Truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Truncate(s, width, "…")
}

// PriorityName returns the human-readable name for a priority level.
func PriorityName(level int) string {
	if name, ok := priorityLevelNames[level]; ok {
		return name
	}
	return "Medium"
}

var priorityLevelNames = map[int]string{
	1: "Highest",
	2: "High",
	3: "Medium",
	4: "Low",
	5: "Lowest",
}
