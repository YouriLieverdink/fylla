package components

import (
	"fmt"
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
			Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#AAAAAA"}).
			Width(12)

	formTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})

	formHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})

	formSelectStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"})
)

// FieldKind distinguishes the type of form field.
type FieldKind int

const (
	FieldText   FieldKind = iota
	FieldSelect
	FieldToggle
)

// FormFieldDef defines a field in a form.
type FormFieldDef struct {
	Label       string
	Placeholder string
	Value       string
	Kind        FieldKind
	Options     []string // for FieldSelect
}

type selectField struct {
	options  []string
	selected int
}

type toggleField struct {
	value bool
}

// Form represents a modal form overlay with multiple fields of varying types.
type Form struct {
	Title    string
	kinds    []FieldKind
	texts    []textinput.Model
	selects  []selectField
	toggles  []toggleField
	Labels   []string
	textIdx  []int // maps field index → texts slice index (-1 if not text)
	selIdx   []int // maps field index → selects slice index (-1 if not select)
	togIdx   []int // maps field index → toggles slice index (-1 if not toggle)
	Focus    int
	Active   bool
}

// NewForm creates a new form with the given title and field definitions.
func NewForm(title string, fields []FormFieldDef) Form {
	f := Form{
		Title:   title,
		kinds:   make([]FieldKind, len(fields)),
		Labels:  make([]string, len(fields)),
		textIdx: make([]int, len(fields)),
		selIdx:  make([]int, len(fields)),
		togIdx:  make([]int, len(fields)),
		Active:  true,
	}

	for i, fd := range fields {
		f.kinds[i] = fd.Kind
		f.Labels[i] = fd.Label
		f.textIdx[i] = -1
		f.selIdx[i] = -1
		f.togIdx[i] = -1

		switch fd.Kind {
		case FieldText:
			ti := textinput.New()
			ti.Placeholder = fd.Placeholder
			ti.PlaceholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#BBBBBB", Dark: "#555555"})
			ti.SetValue(fd.Value)
			if i == 0 {
				ti.Focus()
			}
			f.textIdx[i] = len(f.texts)
			f.texts = append(f.texts, ti)

		case FieldSelect:
			sel := selectField{options: fd.Options}
			for j, opt := range fd.Options {
				if strings.EqualFold(opt, fd.Value) {
					sel.selected = j
					break
				}
			}
			f.selIdx[i] = len(f.selects)
			f.selects = append(f.selects, sel)

		case FieldToggle:
			f.togIdx[i] = len(f.toggles)
			f.toggles = append(f.toggles, toggleField{value: fd.Value == "true"})
		}
	}

	return f
}

// FocusNext moves focus to the next field.
func (f *Form) FocusNext() {
	f.blurCurrent()
	f.Focus = (f.Focus + 1) % len(f.kinds)
	f.focusCurrent()
}

// FocusPrev moves focus to the previous field.
func (f *Form) FocusPrev() {
	f.blurCurrent()
	f.Focus = (f.Focus + len(f.kinds) - 1) % len(f.kinds)
	f.focusCurrent()
}

func (f *Form) blurCurrent() {
	if idx := f.textIdx[f.Focus]; idx >= 0 {
		f.texts[idx].Blur()
	}
}

func (f *Form) focusCurrent() {
	if idx := f.textIdx[f.Focus]; idx >= 0 {
		f.texts[idx].Focus()
	}
}

// IsSelectField returns true if the focused field is a select.
func (f *Form) IsSelectField() bool {
	return f.kinds[f.Focus] == FieldSelect
}

// IsToggleField returns true if the focused field is a toggle.
func (f *Form) IsToggleField() bool {
	return f.kinds[f.Focus] == FieldToggle
}

// IsTextField returns true if the focused field is a text input.
func (f *Form) IsTextField() bool {
	return f.kinds[f.Focus] == FieldText
}

// CycleSelectRight cycles the focused select field to the next option.
func (f *Form) CycleSelectRight() {
	if idx := f.selIdx[f.Focus]; idx >= 0 {
		s := &f.selects[idx]
		s.selected = (s.selected + 1) % len(s.options)
	}
}

// CycleSelectLeft cycles the focused select field to the previous option.
func (f *Form) CycleSelectLeft() {
	if idx := f.selIdx[f.Focus]; idx >= 0 {
		s := &f.selects[idx]
		s.selected = (s.selected + len(s.options) - 1) % len(s.options)
	}
}

// ToggleValue toggles the focused toggle field.
func (f *Form) ToggleValue() {
	if idx := f.togIdx[f.Focus]; idx >= 0 {
		f.toggles[idx].value = !f.toggles[idx].value
	}
}

// FocusedTextInput returns a pointer to the focused text input, or nil.
func (f *Form) FocusedTextInput() *textinput.Model {
	if idx := f.textIdx[f.Focus]; idx >= 0 {
		return &f.texts[idx]
	}
	return nil
}

// Values returns all field values as strings.
func (f *Form) Values() []string {
	vals := make([]string, len(f.kinds))
	for i := range f.kinds {
		switch f.kinds[i] {
		case FieldText:
			vals[i] = f.texts[f.textIdx[i]].Value()
		case FieldSelect:
			s := f.selects[f.selIdx[i]]
			if len(s.options) > 0 {
				vals[i] = s.options[s.selected]
			}
		case FieldToggle:
			if f.toggles[f.togIdx[i]].value {
				vals[i] = "true"
			} else {
				vals[i] = "false"
			}
		}
	}
	return vals
}

// ValueByLabel returns the value of the field with the given label, or empty string if not found.
func (f *Form) ValueByLabel(label string) string {
	vals := f.Values()
	for i, l := range f.Labels {
		if l == label {
			return vals[i]
		}
	}
	return ""
}

// View renders the form overlay.
func (f Form) View(width, height int) string {
	if !f.Active {
		return ""
	}

	var b strings.Builder
	b.WriteString(formTitleStyle.Render(f.Title))
	b.WriteString("\n\n")

	for i := range f.kinds {
		label := formLabelStyle.Render(f.Labels[i] + ":")
		focused := i == f.Focus

		switch f.kinds[i] {
		case FieldText:
			b.WriteString(label + " " + f.texts[f.textIdx[i]].View() + "\n")

		case FieldSelect:
			s := f.selects[f.selIdx[i]]
			var val string
			if len(s.options) > 0 {
				val = s.options[s.selected]
			}
			if focused {
				val = formSelectStyle.Render(fmt.Sprintf("< %s >", val))
			}
			b.WriteString(label + " " + val + "\n")

		case FieldToggle:
			t := f.toggles[f.togIdx[i]]
			check := "[ ]"
			if t.value {
				check = "[x]"
			}
			if focused {
				check = formSelectStyle.Render(check)
			}
			b.WriteString(label + " " + check + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(formHintStyle.Render("Tab:next  \u2190/\u2192:select  Space:toggle  Enter:submit  Esc:cancel"))

	box := formBorder.Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
