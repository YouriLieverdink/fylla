package config

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerFmt = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	hintStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"})
	yamlStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#CCCCCC"})
)

// Model is the config view model.
type Model struct {
	Content   string
	Loading   bool
	Err       error
	Width     int
	Height    int
	ScrollPos int
}

// New creates a new config model.
func New() Model {
	return Model{Loading: true}
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// ScrollUp scrolls the content up.
func (m *Model) ScrollUp() {
	if m.ScrollPos > 0 {
		m.ScrollPos--
	}
}

// ScrollDown scrolls the content down.
func (m *Model) ScrollDown() {
	m.ScrollPos++
}

// View renders the config view.
func (m Model) View() string {
	if m.Loading {
		return "  Loading config..."
	}
	if m.Err != nil {
		return errStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	var b strings.Builder
	b.WriteString(headerFmt.Render("Configuration"))
	b.WriteString("\n\n")

	if m.Content == "" {
		b.WriteString("  No config found.\n")
	} else {
		for _, line := range strings.Split(m.Content, "\n") {
			b.WriteString("  " + yamlStyle.Render(line) + "\n")
		}
	}

	b.WriteString("\n")
	hints := "j/k:scroll  e:edit value  r:refresh"
	b.WriteString(hintStyle.Render("  " + hints))

	// Apply scrolling
	lines := strings.Split(b.String(), "\n")
	visibleHeight := m.Height - 2
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	start := m.ScrollPos
	if start >= len(lines) {
		start = len(lines) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}
