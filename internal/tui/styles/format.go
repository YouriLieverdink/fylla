package styles

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// FormatPrefix formats a project/section prefix for display.
// Uses the last path segment of the project name to save horizontal space.
func FormatPrefix(project, section string) string {
	short := abbreviateProject(project)
	if short != "" && section != "" {
		return short + "/" + section + ": "
	}
	if short != "" {
		return short + ": "
	}
	return ""
}

// abbreviateProject returns the project name as-is. GitHub tasks use
// `owner/repo` form and the org is worth keeping for disambiguation.
func abbreviateProject(project string) string {
	return project
}

// PadOrTruncate pads or truncates s (ANSI-aware) to exactly the given width.
func PadOrTruncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w > width {
		return ansi.Truncate(s, width, "…")
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
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

// FormatDurationPadded formats a duration right-aligned in a 5-char field, returning "   --" for zero.
func FormatDurationPadded(d time.Duration) string {
	s := "--"
	if d > 0 {
		s = formatDur(d)
	}
	const w = 5
	if len(s) < w {
		return strings.Repeat(" ", w-len(s)) + s
	}
	return s
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

// StringWidth returns the visible width of an ANSI string.
func StringWidth(s string) int {
	return ansi.StringWidth(s)
}

var statusAbbrev = map[string]string{
	"to do":       "TD",
	"in progress": "IP",
	"in review":   "IR",
	"done":        "DN",
	"blocked":     "BL",
	"on hold":     "OH",
}

// AbbrevStatus returns a short abbreviation for a task status.
func AbbrevStatus(status string) string {
	if a, ok := statusAbbrev[strings.ToLower(status)]; ok {
		return HintStyle.Render(a)
	}
	// Fallback: first 2 uppercase letters.
	up := strings.ToUpper(status)
	if len(up) > 2 {
		up = up[:2]
	}
	return HintStyle.Render(up)
}
