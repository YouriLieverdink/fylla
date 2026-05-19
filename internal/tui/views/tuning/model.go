package tuning

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/tui/msg"
	"github.com/iruoy/fylla/internal/tui/styles"
)

// fieldKind classifies a tunable row by how its value is rendered/adjusted.
type fieldKind int

const (
	fieldWeight        fieldKind = iota // 0-1 fraction (priority, dueDate, estimate, age)
	fieldFlat                           // flat additive (upNext)
	fieldPriorityLevel                  // priorityLevels[i], 0-100
	fieldTypeBonus                      // typeBonus[key], flat additive
)

// row is one tunable parameter shown in the left pane.
type row struct {
	Kind    fieldKind
	Label   string
	Index   int    // for priorityLevel: 1..5; for typeBonus: position in sorted keys
	MapKey  string // for typeBonus
	Section string // section header above the row (rendered when changes)
}

// Model is the tuning view state.
type Model struct {
	Width   int
	Height  int
	Cursor  int
	Loading bool
	Err     error

	// Working copy of weights — edits apply here until saved.
	Working config.WeightsConfig
	// Original weights at last load/save — used for dirty check + reset.
	Original config.WeightsConfig

	// Tasks used for live preview (snapshot of cachedTasks at view open).
	Tasks []msg.ScoredTask

	// Saved flash message shown briefly after save.
	Saved bool

	rows []row
}

// New creates a fresh tuning model.
func New() Model {
	return Model{Loading: true}
}

// SetSize updates dimensions.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

// SetWeights replaces both Working and Original from a config load.
func (m *Model) SetWeights(w config.WeightsConfig) {
	if len(w.PriorityLevels) != 5 {
		w.PriorityLevels = append([]float64(nil), config.DefaultPriorityLevels...)
	} else {
		w.PriorityLevels = append([]float64(nil), w.PriorityLevels...)
	}
	w.TypeBonus = copyFloatMap(w.TypeBonus)
	m.Working = w
	m.Original = cloneWeights(w)
	m.Loading = false
	m.Err = nil
	m.rebuildRows()
	if m.Cursor >= len(m.rows) {
		m.Cursor = 0
	}
}

// SetTasks replaces the preview task list.
func (m *Model) SetTasks(tasks []msg.ScoredTask) {
	m.Tasks = tasks
}

// Dirty reports whether Working differs from Original.
func (m *Model) Dirty() bool {
	return !weightsEqual(m.Working, m.Original)
}

// MarkSaved updates Original to match Working (call after a successful save).
func (m *Model) MarkSaved() {
	m.Original = cloneWeights(m.Working)
	m.Saved = true
}

// ClearSaved clears the saved-flash flag.
func (m *Model) ClearSaved() { m.Saved = false }

// Reset reverts Working to Original.
func (m *Model) Reset() {
	m.Working = cloneWeights(m.Original)
}

// CursorUp moves to the previous row.
func (m *Model) CursorUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// CursorDown moves to the next row.
func (m *Model) CursorDown() {
	if m.Cursor < len(m.rows)-1 {
		m.Cursor++
	}
}

// Adjust applies a delta to the current row. Sign determines direction;
// step magnitude is chosen per field kind.
func (m *Model) Adjust(direction int, large bool) {
	if direction == 0 || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return
	}
	r := m.rows[m.Cursor]
	step := stepFor(r.Kind, large)
	delta := step
	if direction < 0 {
		delta = -delta
	}
	m.applyDelta(r, delta)
}

// SetCurrent overrides the current row to the given numeric value (clamped).
func (m *Model) SetCurrent(value float64) {
	if m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return
	}
	r := m.rows[m.Cursor]
	switch r.Kind {
	case fieldWeight:
		v := clamp(value, 0, 1)
		switch r.Label {
		case "priority":
			m.Working.Priority = v
		case "dueDate":
			m.Working.DueDate = v
		case "estimate":
			m.Working.Estimate = v
		case "age":
			m.Working.Age = v
		}
	case fieldFlat:
		m.Working.UpNext = clamp(value, 0, 500)
	case fieldPriorityLevel:
		if r.Index >= 1 && r.Index <= 5 {
			m.Working.PriorityLevels[r.Index-1] = clamp(value, 0, 200)
		}
	case fieldTypeBonus:
		if m.Working.TypeBonus == nil {
			m.Working.TypeBonus = map[string]float64{}
		}
		m.Working.TypeBonus[r.MapKey] = clamp(value, -100, 200)
	}
}

func (m *Model) applyDelta(r row, delta float64) {
	switch r.Kind {
	case fieldWeight:
		switch r.Label {
		case "priority":
			m.Working.Priority = clamp(m.Working.Priority+delta, 0, 1)
		case "dueDate":
			m.Working.DueDate = clamp(m.Working.DueDate+delta, 0, 1)
		case "estimate":
			m.Working.Estimate = clamp(m.Working.Estimate+delta, 0, 1)
		case "age":
			m.Working.Age = clamp(m.Working.Age+delta, 0, 1)
		}
	case fieldFlat:
		m.Working.UpNext = clamp(m.Working.UpNext+delta, 0, 500)
	case fieldPriorityLevel:
		if r.Index >= 1 && r.Index <= 5 {
			m.Working.PriorityLevels[r.Index-1] = clamp(m.Working.PriorityLevels[r.Index-1]+delta, 0, 200)
		}
	case fieldTypeBonus:
		if m.Working.TypeBonus == nil {
			m.Working.TypeBonus = map[string]float64{}
		}
		m.Working.TypeBonus[r.MapKey] = clamp(m.Working.TypeBonus[r.MapKey]+delta, -100, 200)
	}
}

// WeightSum returns priority+dueDate+estimate+age.
func (m *Model) WeightSum() float64 {
	return m.Working.Priority + m.Working.DueDate + m.Working.Estimate + m.Working.Age
}

// SumOK reports whether the current weight sum is acceptable (within 0.005 of 1.0).
func (m *Model) SumOK() bool { return sumOK(m.WeightSum()) }

func sumOK(sum float64) bool { return math.Abs(sum-1.0) < 0.005 }

func (m *Model) rebuildRows() {
	rows := []row{
		{Kind: fieldWeight, Label: "priority", Section: "Weights (must sum to 1.0)"},
		{Kind: fieldWeight, Label: "dueDate"},
		{Kind: fieldWeight, Label: "estimate"},
		{Kind: fieldWeight, Label: "age"},
		{Kind: fieldFlat, Label: "upNext", Section: "Boosts"},
	}
	rows = append(rows, row{Kind: fieldPriorityLevel, Label: "P1 Highest", Index: 1, Section: "Priority levels (raw 0-100)"})
	rows = append(rows, row{Kind: fieldPriorityLevel, Label: "P2 High", Index: 2})
	rows = append(rows, row{Kind: fieldPriorityLevel, Label: "P3 Medium", Index: 3})
	rows = append(rows, row{Kind: fieldPriorityLevel, Label: "P4 Low", Index: 4})
	rows = append(rows, row{Kind: fieldPriorityLevel, Label: "P5 Lowest", Index: 5})
	if len(m.Working.TypeBonus) > 0 {
		keys := make([]string, 0, len(m.Working.TypeBonus))
		for k := range m.Working.TypeBonus {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			r := row{Kind: fieldTypeBonus, Label: k, MapKey: k, Index: i}
			if i == 0 {
				r.Section = "Type bonuses"
			}
			rows = append(rows, r)
		}
	}
	m.rows = rows
}

// View renders the tuning view (left controls + right preview).
func (m *Model) View() string {
	if m.Loading {
		return "  Loading tuning..."
	}
	if m.Err != nil {
		return styles.ErrStyle.Render(fmt.Sprintf("  Error: %v", m.Err))
	}

	leftWidth := m.Width / 2
	if leftWidth < 38 {
		leftWidth = 38
	}
	rightWidth := m.Width - leftWidth - 4
	if rightWidth < 30 {
		rightWidth = 30
	}

	left := m.renderControls(leftWidth)
	right := m.renderPreview(rightWidth)

	row := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	hints := "j/k:nav  h/l or -/+:adjust  Shift:×5  =:reset row  s:save  r:reset all"
	if m.Dirty() {
		hints = "● unsaved  " + hints
	}
	if m.Saved {
		hints = "✓ saved  " + hints
	}
	return row + "\n\n" + styles.HintStyle.Render("  "+hints)
}

func (m *Model) renderControls(width int) string {
	var b strings.Builder
	maxLabel := 14
	for _, r := range m.rows {
		if len(r.Label) > maxLabel {
			maxLabel = len(r.Label)
		}
	}
	for i, r := range m.rows {
		if r.Section != "" {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("  " + styles.SectionFmt.Render(r.Section) + "\n")
		}
		marker := "  "
		valueStr := m.renderValue(r)
		bar := m.renderBar(r, 14)
		padded := r.Label + strings.Repeat(" ", maxLabel-len(r.Label))
		line := fmt.Sprintf("    %s  %s  %s", padded, valueStr, bar)
		if i == m.Cursor {
			marker = styles.SelectedStyle.Render("> ")
			line = styles.SelectedStyle.Render(fmt.Sprintf("    %s  %s  %s", padded, valueStr, bar))
		}
		b.WriteString(marker + line + "\n")
	}
	sum := m.WeightSum()
	sumLine := fmt.Sprintf("  Sum: %.2f", sum)
	if !sumOK(sum) {
		sumLine += "  " + styles.ErrStyle.Render("(must be 1.00)")
	} else {
		sumLine += "  ✓"
	}
	b.WriteString("\n" + sumLine + "\n")
	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m *Model) renderValue(r row) string {
	switch r.Kind {
	case fieldWeight:
		var v float64
		switch r.Label {
		case "priority":
			v = m.Working.Priority
		case "dueDate":
			v = m.Working.DueDate
		case "estimate":
			v = m.Working.Estimate
		case "age":
			v = m.Working.Age
		}
		return fmt.Sprintf("%5.2f", v)
	case fieldFlat:
		return fmt.Sprintf("%5.0f", m.Working.UpNext)
	case fieldPriorityLevel:
		if r.Index >= 1 && r.Index <= 5 {
			return fmt.Sprintf("%5.0f", m.Working.PriorityLevels[r.Index-1])
		}
	case fieldTypeBonus:
		return fmt.Sprintf("%5.0f", m.Working.TypeBonus[r.MapKey])
	}
	return ""
}

func (m *Model) renderBar(r row, width int) string {
	frac := 0.0
	switch r.Kind {
	case fieldWeight:
		switch r.Label {
		case "priority":
			frac = m.Working.Priority
		case "dueDate":
			frac = m.Working.DueDate
		case "estimate":
			frac = m.Working.Estimate
		case "age":
			frac = m.Working.Age
		}
	case fieldFlat:
		frac = m.Working.UpNext / 100.0
	case fieldPriorityLevel:
		if r.Index >= 1 && r.Index <= 5 {
			frac = m.Working.PriorityLevels[r.Index-1] / 100.0
		}
	case fieldTypeBonus:
		frac = (m.Working.TypeBonus[r.MapKey] + 50) / 150.0
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(math.Round(frac * float64(width)))
	if filled > width {
		filled = width
	}
	return strings.Repeat("▮", filled) + strings.Repeat("▯", width-filled)
}

func (m *Model) renderPreview(width int) string {
	var b strings.Builder
	b.WriteString("  " + styles.SectionFmt.Render("Preview — top tasks with current weights") + "\n\n")
	if len(m.Tasks) == 0 {
		b.WriteString(styles.HintStyle.Render("    no tasks loaded yet (Tasks tab to fetch)\n"))
		return lipgloss.NewStyle().Width(width).Render(b.String())
	}
	scored := Rescore(m.Tasks, m.Working)
	limit := 15
	if limit > len(scored) {
		limit = len(scored)
	}
	// Mirror tasks-tab compact row: dot(2) + rank(3) + 2 + label + 2 + score(6).
	const fixed = 15
	labelWidth := width - fixed
	if labelWidth < 20 {
		labelWidth = 20
	}
	for i, t := range scored[:limit] {
		dot := styles.FormatProjectDot(t.Project)
		rank := fmt.Sprintf("%2d.", i+1)
		prefix := ""
		if t.Section != "" {
			prefix = t.Section + ": "
		} else if t.Project != "" {
			prefix = t.Project + ": "
		}
		summary := t.Summary
		if summary == "" {
			summary = t.Key
		}
		if summary == "" {
			summary = "(untitled)"
		}
		summaryWidth := labelWidth - runeLen(prefix)
		if summaryWidth < 10 {
			summaryWidth = 10
		}
		label := styles.PadOrTruncate(prefix+styles.Truncate(summary, summaryWidth), labelWidth)
		b.WriteString(fmt.Sprintf("%s%s  %s  %6.1f\n", dot, rank, label, t.Score))
	}
	return lipgloss.NewStyle().Width(width).Render(b.String())
}

// Rescore recomputes scores for the given tasks under the supplied weights,
// reusing the stable raw scores (due date, estimate, age, crunch, not-before)
// stored in each task's existing breakdown. Returns a sorted descending copy.
func Rescore(tasks []msg.ScoredTask, w config.WeightsConfig) []msg.ScoredTask {
	levels := w.PriorityLevelsOrDefault()
	out := make([]msg.ScoredTask, len(tasks))
	for i, t := range tasks {
		bd := t.Breakdown
		priRaw := priorityRaw(t.Priority, levels)
		bd.PriorityRaw = priRaw
		bd.PriorityWeight = w.Priority
		bd.PriorityWeighted = w.Priority * priRaw
		bd.DueDateWeight = w.DueDate
		bd.DueDateWeighted = w.DueDate * bd.DueDateRaw
		bd.EstimateWeight = w.Estimate
		bd.EstimateWeighted = w.Estimate * bd.EstimateRaw
		bd.AgeWeight = w.Age
		bd.AgeWeighted = w.Age * bd.AgeRaw
		bd.TypeBonus = typeBonus(t.IssueType, w.TypeBonus)
		score := bd.PriorityWeighted + bd.DueDateWeighted + bd.EstimateWeighted + bd.AgeWeighted
		score += bd.CrunchBoost + bd.TypeBonus
		if t.UpNext {
			bd.UpNextBoost = w.UpNext
			score += bd.UpNextBoost
		} else {
			if bd.NotBeforeMult == 0 {
				bd.NotBeforeMult = 1
			}
			score *= bd.NotBeforeMult
		}
		bd.Total = score
		out[i] = t
		out[i].Breakdown = bd
		out[i].Score = score
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func priorityRaw(p int, levels []float64) float64 {
	if len(levels) != 5 {
		levels = config.DefaultPriorityLevels
	}
	if p < 1 || p > 5 {
		return levels[2]
	}
	return levels[p-1]
}

func typeBonus(t string, m map[string]float64) float64 {
	if t == "" || len(m) == 0 {
		return 0
	}
	return m[t]
}

func stepFor(k fieldKind, large bool) float64 {
	switch k {
	case fieldWeight:
		if large {
			return 0.05
		}
		return 0.01
	case fieldFlat, fieldTypeBonus:
		if large {
			return 25
		}
		return 5
	case fieldPriorityLevel:
		if large {
			return 25
		}
		return 5
	}
	return 1
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func cloneWeights(w config.WeightsConfig) config.WeightsConfig {
	out := w
	out.PriorityLevels = append([]float64(nil), w.PriorityLevels...)
	out.TypeBonus = copyFloatMap(w.TypeBonus)
	return out
}

func copyFloatMap(m map[string]float64) map[string]float64 {
	if m == nil {
		return nil
	}
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func weightsEqual(a, b config.WeightsConfig) bool {
	if a.Priority != b.Priority || a.DueDate != b.DueDate || a.Estimate != b.Estimate ||
		a.Age != b.Age || a.UpNext != b.UpNext {
		return false
	}
	if len(a.PriorityLevels) != len(b.PriorityLevels) {
		return false
	}
	for i := range a.PriorityLevels {
		if a.PriorityLevels[i] != b.PriorityLevels[i] {
			return false
		}
	}
	if len(a.TypeBonus) != len(b.TypeBonus) {
		return false
	}
	for k, v := range a.TypeBonus {
		if b.TypeBonus[k] != v {
			return false
		}
	}
	return true
}

// ChangedKeys returns the (key, value) pairs from Working that differ from
// Original, in the format expected by config.SetIn.
func (m *Model) ChangedKeys() []KeyValue {
	var out []KeyValue
	if m.Working.Priority != m.Original.Priority {
		out = append(out, KeyValue{Key: "weights.priority", Value: formatFloat(m.Working.Priority)})
	}
	if m.Working.DueDate != m.Original.DueDate {
		out = append(out, KeyValue{Key: "weights.dueDate", Value: formatFloat(m.Working.DueDate)})
	}
	if m.Working.Estimate != m.Original.Estimate {
		out = append(out, KeyValue{Key: "weights.estimate", Value: formatFloat(m.Working.Estimate)})
	}
	if m.Working.Age != m.Original.Age {
		out = append(out, KeyValue{Key: "weights.age", Value: formatFloat(m.Working.Age)})
	}
	if m.Working.UpNext != m.Original.UpNext {
		out = append(out, KeyValue{Key: "weights.upNext", Value: formatFloat(m.Working.UpNext)})
	}
	if !floatsEqual(m.Working.PriorityLevels, m.Original.PriorityLevels) {
		parts := make([]string, len(m.Working.PriorityLevels))
		for i, v := range m.Working.PriorityLevels {
			parts[i] = formatFloat(v)
		}
		out = append(out, KeyValue{Key: "weights.priorityLevels", Value: "[" + strings.Join(parts, ",") + "]"})
	}
	for k, v := range m.Working.TypeBonus {
		if m.Original.TypeBonus[k] != v {
			out = append(out, KeyValue{Key: "weights.typeBonus." + k, Value: formatFloat(v)})
		}
	}
	return out
}

// KeyValue is one config key + value pair.
type KeyValue struct {
	Key   string
	Value string
}

func formatFloat(v float64) string {
	if v == float64(int(v)) {
		return fmt.Sprintf("%d", int(v))
	}
	return fmt.Sprintf("%g", v)
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}

func padRight(s string, width int) string {
	pad := width - utf8.RuneCountInString(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func floatsEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
