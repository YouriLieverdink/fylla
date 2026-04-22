package styles

import (
	"hash/fnv"

	"github.com/charmbracelet/lipgloss"
)

var (
	HeaderFmt     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	HintStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	ErrStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"})
	SelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})
	CurrentStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})
	UpNextStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})
	AtRiskStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"}).Bold(true)
	CalEventStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#555555"})
	PastStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#555555"})
	SectionFmt    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})
	WarnStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#F2A900", Dark: "#FDCB58"})
	RunningStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})
	TimerBig      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})
	TaskStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})
	YamlStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#CCCCCC"})
	PRTagStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8E24AA", Dark: "#BA68C8"})
	IssueTagStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1E88E5", Dark: "#64B5F6"})

	projectPalette = []lipgloss.AdaptiveColor{
		{Light: "#1E88E5", Dark: "#42A5F5"}, // blue
		{Light: "#43A047", Dark: "#66BB6A"}, // green
		{Light: "#FB8C00", Dark: "#FFA726"}, // orange
		{Light: "#8E24AA", Dark: "#AB47BC"}, // purple
		{Light: "#00897B", Dark: "#26A69A"}, // teal
		{Light: "#E53935", Dark: "#EF5350"}, // red
		{Light: "#3949AB", Dark: "#5C6BC0"}, // indigo
		{Light: "#6D4C41", Dark: "#8D6E63"}, // brown
	}
)

// ProjectBadgeStyle returns a lipgloss style with a deterministic foreground
// color based on the project name.
func ProjectBadgeStyle(project string) lipgloss.Style {
	h := fnv.New32a()
	h.Write([]byte(project))
	idx := int(h.Sum32()) % len(projectPalette)
	return lipgloss.NewStyle().Foreground(projectPalette[idx])
}
