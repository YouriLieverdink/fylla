package config

import (
	"fmt"
	"strings"

	"github.com/iruoy/fylla/internal/tui/styles"
)

// Model is the config view model.
type Model struct {
	Content      string
	Loading      bool
	Err          error
	Width        int
	Height       int
	ScrollPos    int
	contentLines int
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
	visibleHeight := m.Height - 2
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	maxScroll := m.contentLines - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.ScrollPos < maxScroll {
		m.ScrollPos++
	}
}

// View renders the config view.
func (m *Model) View() string {
	if m.Loading {
		return "  Loading config..."
	}
	if m.Err != nil {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	var b strings.Builder
	b.WriteString(styles.HeaderFmt.Render("Configuration"))
	b.WriteString("\n\n")

	if m.Content == "" {
		b.WriteString("  No config found.\n")
	} else {
		for _, line := range strings.Split(m.Content, "\n") {
			b.WriteString("  " + styles.YamlStyle.Render(line) + "\n")
		}
	}

	b.WriteString("\n")
	hints := "j/k:scroll  e:edit value  r:refresh"
	b.WriteString(styles.HintStyle.Render("  " + hints))

	// Apply scrolling
	lines := strings.Split(b.String(), "\n")
	m.contentLines = len(lines)
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
