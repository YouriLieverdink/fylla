package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

var (
	formBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}).
			Padding(1, 2)

	formLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}).
			Width(12)

	formTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})

	formHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
)

// FormField defines a field in a form.
type FormField struct {
	Label       string
	Placeholder string
	Value       string
}

// Form represents a modal form overlay with multiple text inputs.
type Form struct {
	Title    string
	Fields   []textinput.Model
	Labels   []string
	Focus    int
	Active   bool
}

// NewForm creates a new form with the given title and fields.
func NewForm(title string, fields []FormField) Form {
	inputs := make([]textinput.Model, len(fields))
	labels := make([]string, len(fields))
	for i, f := range fields {
		ti := textinput.New()
		ti.Placeholder = f.Placeholder
		ti.SetValue(f.Value)
		if i == 0 {
			ti.Focus()
		}
		inputs[i] = ti
		labels[i] = f.Label
	}
	return Form{
		Title:  title,
		Fields: inputs,
		Labels: labels,
		Active: true,
	}
}

// FocusNext moves focus to the next field.
func (f *Form) FocusNext() {
	f.Fields[f.Focus].Blur()
	f.Focus = (f.Focus + 1) % len(f.Fields)
	f.Fields[f.Focus].Focus()
}

// FocusPrev moves focus to the previous field.
func (f *Form) FocusPrev() {
	f.Fields[f.Focus].Blur()
	f.Focus = (f.Focus + len(f.Fields) - 1) % len(f.Fields)
	f.Fields[f.Focus].Focus()
}

// Values returns all field values.
func (f *Form) Values() []string {
	vals := make([]string, len(f.Fields))
	for i, field := range f.Fields {
		vals[i] = field.Value()
	}
	return vals
}

// UpdateField passes a key message to the currently focused field.
func (f *Form) UpdateField(tmsg interface{ String() string }) {
	// We need the tea.Msg interface - the caller should pass tea.KeyMsg
}

// View renders the form overlay.
func (f Form) View(width, height int) string {
	if !f.Active {
		return ""
	}

	var b strings.Builder
	b.WriteString(formTitleStyle.Render(f.Title))
	b.WriteString("\n\n")

	for i, field := range f.Fields {
		label := formLabelStyle.Render(f.Labels[i] + ":")
		b.WriteString(label + " " + field.View() + "\n")
		_ = i
	}

	b.WriteString("\n")
	b.WriteString(formHintStyle.Render("Tab:next field  Enter:submit  Esc:cancel"))

	box := formBorder.Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
