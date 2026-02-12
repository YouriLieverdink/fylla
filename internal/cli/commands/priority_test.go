package commands

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockPriorityUpdater records UpdatePriority calls.
type mockPriorityUpdater struct {
	calls []priorityCall
	err   error
}

type priorityCall struct {
	issueKey string
	priority int
}

func (m *mockPriorityUpdater) UpdatePriority(_ context.Context, issueKey string, priority int) error {
	m.calls = append(m.calls, priorityCall{issueKey, priority})
	return m.err
}

// mockPriorityGetter returns a preset current priority.
type mockPriorityGetter struct {
	priority int
	err      error
}

func (m *mockPriorityGetter) GetPriority(_ context.Context, _ string) (int, error) {
	return m.priority, m.err
}

func TestPriority_absolute_name(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLevel int
		wantName  string
	}{
		{"Highest", "Highest", 1, "Highest"},
		{"High", "High", 2, "High"},
		{"Medium", "Medium", 3, "Medium"},
		{"Low", "Low", 4, "Low"},
		{"Lowest", "Lowest", 5, "Lowest"},
		{"case insensitive", "high", 2, "High"},
		{"numeric 1", "1", 1, "Highest"},
		{"numeric 5", "5", 5, "Lowest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updater := &mockPriorityUpdater{}
			result, err := RunPriority(context.Background(), PriorityParams{
				TaskKey:  "PROJ-123",
				Priority: tt.input,
				Updater:  updater,
			})
			if err != nil {
				t.Fatalf("RunPriority: %v", err)
			}

			if len(updater.calls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(updater.calls))
			}
			if updater.calls[0].issueKey != "PROJ-123" {
				t.Errorf("issueKey = %q, want PROJ-123", updater.calls[0].issueKey)
			}
			if updater.calls[0].priority != tt.wantLevel {
				t.Errorf("priority = %d, want %d", updater.calls[0].priority, tt.wantLevel)
			}

			if result.Priority != tt.wantLevel {
				t.Errorf("result.Priority = %d, want %d", result.Priority, tt.wantLevel)
			}
			if result.Name != tt.wantName {
				t.Errorf("result.Name = %q, want %q", result.Name, tt.wantName)
			}
		})
	}
}

func TestPriority_relative_adjustments(t *testing.T) {
	t.Run("increase priority by 1 from Medium", func(t *testing.T) {
		updater := &mockPriorityUpdater{}
		getter := &mockPriorityGetter{priority: 3}
		result, err := RunPriority(context.Background(), PriorityParams{
			TaskKey:  "PROJ-123",
			Priority: "-1",
			Updater:  updater,
			Getter:   getter,
		})
		if err != nil {
			t.Fatalf("RunPriority: %v", err)
		}

		if result.Priority != 2 {
			t.Errorf("result.Priority = %d, want 2 (High)", result.Priority)
		}
	})

	t.Run("decrease priority by 1 from Medium", func(t *testing.T) {
		updater := &mockPriorityUpdater{}
		getter := &mockPriorityGetter{priority: 3}
		result, err := RunPriority(context.Background(), PriorityParams{
			TaskKey:  "PROJ-123",
			Priority: "+1",
			Updater:  updater,
			Getter:   getter,
		})
		if err != nil {
			t.Fatalf("RunPriority: %v", err)
		}

		if result.Priority != 4 {
			t.Errorf("result.Priority = %d, want 4 (Low)", result.Priority)
		}
	})

	t.Run("clamp at Highest when going below 1", func(t *testing.T) {
		updater := &mockPriorityUpdater{}
		getter := &mockPriorityGetter{priority: 1}
		result, err := RunPriority(context.Background(), PriorityParams{
			TaskKey:  "PROJ-123",
			Priority: "-3",
			Updater:  updater,
			Getter:   getter,
		})
		if err != nil {
			t.Fatalf("RunPriority: %v", err)
		}

		if result.Priority != 1 {
			t.Errorf("result.Priority = %d, want 1 (Highest)", result.Priority)
		}
	})

	t.Run("clamp at Lowest when going above 5", func(t *testing.T) {
		updater := &mockPriorityUpdater{}
		getter := &mockPriorityGetter{priority: 5}
		result, err := RunPriority(context.Background(), PriorityParams{
			TaskKey:  "PROJ-123",
			Priority: "+3",
			Updater:  updater,
			Getter:   getter,
		})
		if err != nil {
			t.Fatalf("RunPriority: %v", err)
		}

		if result.Priority != 5 {
			t.Errorf("result.Priority = %d, want 5 (Lowest)", result.Priority)
		}
	})

	t.Run("returns error when getter fails", func(t *testing.T) {
		updater := &mockPriorityUpdater{}
		getter := &mockPriorityGetter{err: fmt.Errorf("jira error")}
		_, err := RunPriority(context.Background(), PriorityParams{
			TaskKey:  "PROJ-123",
			Priority: "+1",
			Updater:  updater,
			Getter:   getter,
		})
		if err == nil {
			t.Fatal("expected error from getter")
		}
	})
}

func TestPriority_invalid_input(t *testing.T) {
	t.Run("empty priority", func(t *testing.T) {
		updater := &mockPriorityUpdater{}
		_, err := RunPriority(context.Background(), PriorityParams{
			TaskKey:  "PROJ-123",
			Priority: "",
			Updater:  updater,
		})
		if err == nil {
			t.Fatal("expected error for empty priority")
		}
	})

	t.Run("invalid name", func(t *testing.T) {
		updater := &mockPriorityUpdater{}
		_, err := RunPriority(context.Background(), PriorityParams{
			TaskKey:  "PROJ-123",
			Priority: "Critical",
			Updater:  updater,
		})
		if err == nil {
			t.Fatal("expected error for invalid name")
		}
	})

	t.Run("numeric out of range", func(t *testing.T) {
		updater := &mockPriorityUpdater{}
		_, err := RunPriority(context.Background(), PriorityParams{
			TaskKey:  "PROJ-123",
			Priority: "6",
			Updater:  updater,
		})
		if err == nil {
			t.Fatal("expected error for out of range numeric")
		}
	})

	t.Run("returns error from updater", func(t *testing.T) {
		updater := &mockPriorityUpdater{err: fmt.Errorf("jira error")}
		_, err := RunPriority(context.Background(), PriorityParams{
			TaskKey:  "PROJ-123",
			Priority: "High",
			Updater:  updater,
		})
		if err == nil {
			t.Fatal("expected error from updater")
		}
	})
}

func TestPrintPriorityResult(t *testing.T) {
	var buf bytes.Buffer
	PrintPriorityResult(&buf, &PriorityResult{
		TaskKey:  "PROJ-123",
		Priority: 2,
		Name:     "High",
	})
	out := buf.String()
	if !strings.Contains(out, "PROJ-123") {
		t.Errorf("output = %q, want to contain PROJ-123", out)
	}
	if !strings.Contains(out, "High") {
		t.Errorf("output = %q, want to contain High", out)
	}
}
