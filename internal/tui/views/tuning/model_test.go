package tuning

import (
	"testing"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/tui/msg"
)

func defaultWeights() config.WeightsConfig {
	return config.WeightsConfig{
		Priority:       0.45,
		DueDate:        0.30,
		Estimate:       0.15,
		Age:            0.10,
		UpNext:         50,
		PriorityLevels: []float64{100, 80, 60, 40, 20},
	}
}

func TestSetWeightsResetClonesSlices(t *testing.T) {
	m := New()
	m.SetWeights(defaultWeights())

	m.Working.PriorityLevels[0] = 999
	if m.Original.PriorityLevels[0] == 999 {
		t.Fatal("Original.PriorityLevels aliased Working — must be cloned")
	}
	m.Reset()
	if m.Working.PriorityLevels[0] != 100 {
		t.Fatalf("Reset should restore P1 to 100, got %v", m.Working.PriorityLevels[0])
	}
}

func TestDirtyTracksEdits(t *testing.T) {
	m := New()
	m.SetWeights(defaultWeights())
	if m.Dirty() {
		t.Fatal("fresh model must not be dirty")
	}
	m.Adjust(1, false) // priority weight +0.01
	if !m.Dirty() {
		t.Fatal("after Adjust the model must be dirty")
	}
	m.Reset()
	if m.Dirty() {
		t.Fatal("Reset must clear dirty state")
	}
}

func TestChangedKeysIncludesPriorityLevels(t *testing.T) {
	m := New()
	m.SetWeights(defaultWeights())

	// Move cursor to P1 row and bump it.
	for i := 0; i < 5; i++ {
		m.CursorDown()
	}
	m.Adjust(1, false) // P1 +5

	changes := m.ChangedKeys()
	var found bool
	for _, c := range changes {
		if c.Key == "weights.priorityLevels" {
			found = true
			if c.Value != "[105,80,60,40,20]" {
				t.Fatalf("unexpected priorityLevels value: %s", c.Value)
			}
		}
	}
	if !found {
		t.Fatalf("expected priorityLevels key in changes, got %+v", changes)
	}
}

func TestRescoreReshuffles(t *testing.T) {
	tasks := []msg.ScoredTask{
		{
			Key:      "A",
			Priority: 1,
			Breakdown: msg.ScoreBreakdown{
				PriorityRaw: 100, DueDateRaw: 0, EstimateRaw: 0, AgeRaw: 0,
				NotBeforeMult: 1,
			},
		},
		{
			Key:      "B",
			Priority: 5,
			DueDate:  nil,
			Breakdown: msg.ScoreBreakdown{
				PriorityRaw: 20, DueDateRaw: 80, EstimateRaw: 0, AgeRaw: 0,
				NotBeforeMult: 1,
			},
		},
	}
	w := defaultWeights()
	// With default weights: A=0.45*100+0.30*0=45; B=0.45*20+0.30*80=9+24=33. A wins.
	out := Rescore(tasks, w)
	if out[0].Key != "A" {
		t.Fatalf("expected A first, got %s", out[0].Key)
	}

	// Boost due date weight, drop priority. B should overtake.
	w.Priority = 0.05
	w.DueDate = 0.70
	w.Estimate = 0.15
	w.Age = 0.10
	out = Rescore(tasks, w)
	if out[0].Key != "B" {
		t.Fatalf("expected B first after re-weighting, got %s", out[0].Key)
	}
}

func TestViewDoesNotPanicWithMultibyteSummary(t *testing.T) {
	m := New()
	m.SetWeights(defaultWeights())
	m.SetSize(60, 20)
	m.SetTasks([]msg.ScoredTask{
		{
			Key:       "verylongkeythatoverflows",
			Summary:   "Émoji-laden summary 🚀 with multibyte runes ✓✓✓✓✓✓✓✓✓✓✓",
			Priority:  1,
			Breakdown: msg.ScoreBreakdown{PriorityRaw: 100, NotBeforeMult: 1},
		},
	})
	_ = m.View() // must not panic
}

func TestViewDoesNotPanicWithZeroWidth(t *testing.T) {
	m := New()
	m.SetWeights(defaultWeights())
	// Width=0 simulates the first render before WindowSizeMsg arrives.
	m.SetSize(0, 0)
	m.SetTasks([]msg.ScoredTask{{
		Key: "K", Summary: "hi", Breakdown: msg.ScoreBreakdown{NotBeforeMult: 1},
	}})
	_ = m.View()
}

func TestSetWeightsFallsBackToDefaultLevels(t *testing.T) {
	w := defaultWeights()
	w.PriorityLevels = nil
	m := New()
	m.SetWeights(w)
	if len(m.Working.PriorityLevels) != 5 || m.Working.PriorityLevels[0] != 100 {
		t.Fatalf("expected default priority levels, got %+v", m.Working.PriorityLevels)
	}
}
