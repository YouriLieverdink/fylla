package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	confirmBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#F2A900", Dark: "#FDCB58"}).
			Padding(1, 2)

	confirmHighlight = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})
)

// ConfirmDialog represents a yes/no confirmation dialog.
type ConfirmDialog struct {
	Message  string
	Selected bool // true = Yes, false = No
	Active   bool
}

// NewConfirm creates a new confirmation dialog.
func NewConfirm(message string) ConfirmDialog {
	return ConfirmDialog{Message: message, Active: true}
}

// Toggle switches between Yes and No.
func (c *ConfirmDialog) Toggle() {
	c.Selected = !c.Selected
}

// View renders the confirmation dialog.
func (c ConfirmDialog) View(width, height int) string {
	if !c.Active {
		return ""
	}

	yes := "  Yes  "
	no := "  No  "
	if c.Selected {
		yes = confirmHighlight.Render("> Yes <")
		no = "  No  "
	} else {
		yes = "  Yes  "
		no = confirmHighlight.Render("> No <")
	}

	content := fmt.Sprintf("%s\n\n%s    %s", c.Message, yes, no)
	box := confirmBorder.Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
