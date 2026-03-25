package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}).
				Padding(0, 2)

	tabBarStyle = lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
)

// RenderTabBar renders a tab bar with the given labels and active index.
func RenderTabBar(tabs []string, active int, width int) string {
	var rendered []string
	for i, tab := range tabs {
		label := tab
		if i == active {
			rendered = append(rendered, activeTabStyle.Render(label))
		} else {
			rendered = append(rendered, inactiveTabStyle.Render(label))
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	return tabBarStyle.Width(width).Render(row)
}

// TabNames returns the default tab labels.
func TabNames() []string {
	return []string{"Timeline", "Tasks", "Timer", "Worklog", "Config"}
}

// RenderHelp renders a key hint string.
func RenderHelp(hints []string) string {
	subtle := lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}
	style := lipgloss.NewStyle().Foreground(subtle)
	return style.Render(strings.Join(hints, "  "))
}
