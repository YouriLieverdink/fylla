package commands

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

type mockMoveSource struct {
	listCalls      []string
	transitionKeys []string
	transitionTgts []string
	transitions    []string
	listErr        error
	transitionErr  error
}

func (m *mockMoveSource) ListTransitions(_ context.Context, taskKey string) ([]string, error) {
	m.listCalls = append(m.listCalls, taskKey)
	return m.transitions, m.listErr
}

func (m *mockMoveSource) TransitionTask(_ context.Context, taskKey, target string) error {
	m.transitionKeys = append(m.transitionKeys, taskKey)
	m.transitionTgts = append(m.transitionTgts, target)
	return m.transitionErr
}

// TaskSource interface stubs
func (m *mockMoveSource) FetchTasks(context.Context, string) ([]task.Task, error) { return nil, nil }
func (m *mockMoveSource) CreateTask(context.Context, task.CreateInput) (string, error) {
	return "", nil
}
func (m *mockMoveSource) CompleteTask(context.Context, string) error        { return nil }
func (m *mockMoveSource) DeleteTask(context.Context, string) error          { return nil }
func (m *mockMoveSource) PostWorklog(context.Context, string, time.Duration, string, time.Time) error {
	return nil
}
func (m *mockMoveSource) GetEstimate(context.Context, string) (time.Duration, error)   { return 0, nil }
func (m *mockMoveSource) UpdateEstimate(context.Context, string, time.Duration) error   { return nil }
func (m *mockMoveSource) GetDueDate(context.Context, string) (*time.Time, error)        { return nil, nil }
func (m *mockMoveSource) UpdateDueDate(context.Context, string, time.Time) error        { return nil }
func (m *mockMoveSource) RemoveDueDate(context.Context, string) error                   { return nil }
func (m *mockMoveSource) GetPriority(context.Context, string) (int, error)              { return 3, nil }
func (m *mockMoveSource) UpdatePriority(context.Context, string, int) error             { return nil }
func (m *mockMoveSource) GetSummary(context.Context, string) (string, error)            { return "", nil }
func (m *mockMoveSource) UpdateSummary(context.Context, string, string) error           { return nil }

type mockMoveSurveyor struct {
	selected string
	err      error
}

func (m *mockMoveSurveyor) Select(message string, options []string) (string, error) {
	return m.selected, m.err
}
func (m *mockMoveSurveyor) MultiSelect(message string, options []string) ([]string, error) {
	return nil, nil
}
func (m *mockMoveSurveyor) Input(message string) (string, error)                     { return "", nil }
func (m *mockMoveSurveyor) InputWithDefault(message, defaultVal string) (string, error) { return "", nil }
func (m *mockMoveSurveyor) Password(message string) (string, error)                   { return "", nil }

func TestRunMove(t *testing.T) {
	t.Run("moves task with explicit target", func(t *testing.T) {
		src := &mockMoveSource{transitions: []string{"To Do", "In Progress", "Done"}}
		result, err := RunMove(context.Background(), MoveParams{
			TaskKey: "PROJ-123",
			Target:  "In Progress",
			Source:  src,
		})
		if err != nil {
			t.Fatalf("RunMove: %v", err)
		}
		if result.TaskKey != "PROJ-123" {
			t.Errorf("TaskKey = %q, want PROJ-123", result.TaskKey)
		}
		if result.Target != "In Progress" {
			t.Errorf("Target = %q, want In Progress", result.Target)
		}
		if len(src.transitionKeys) != 1 || src.transitionKeys[0] != "PROJ-123" {
			t.Errorf("unexpected transition calls: %v", src.transitionKeys)
		}
	})

	t.Run("prompts when no target given", func(t *testing.T) {
		src := &mockMoveSource{transitions: []string{"To Do", "In Progress", "Done"}}
		result, err := RunMove(context.Background(), MoveParams{
			TaskKey:  "PROJ-456",
			Source:   src,
			Surveyor: &mockMoveSurveyor{selected: "Done"},
		})
		if err != nil {
			t.Fatalf("RunMove: %v", err)
		}
		if result.Target != "Done" {
			t.Errorf("Target = %q, want Done", result.Target)
		}
		if len(src.listCalls) != 1 {
			t.Errorf("expected 1 ListTransitions call, got %d", len(src.listCalls))
		}
	})

	t.Run("returns error from transition", func(t *testing.T) {
		src := &mockMoveSource{transitionErr: fmt.Errorf("forbidden")}
		_, err := RunMove(context.Background(), MoveParams{
			TaskKey: "PROJ-789",
			Target:  "Done",
			Source:  src,
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "forbidden") {
			t.Errorf("error = %q, want to contain 'forbidden'", err.Error())
		}
	})

	t.Run("strips recurrence suffix", func(t *testing.T) {
		src := &mockMoveSource{transitions: []string{"Done"}}
		_, err := RunMove(context.Background(), MoveParams{
			TaskKey: "PROJ-123@2026-03-18",
			Target:  "Done",
			Source:  src,
		})
		if err != nil {
			t.Fatalf("RunMove: %v", err)
		}
		if len(src.transitionKeys) != 1 || src.transitionKeys[0] != "PROJ-123" {
			t.Errorf("expected stripped key PROJ-123, got %v", src.transitionKeys)
		}
	})

	t.Run("confirmation message displayed", func(t *testing.T) {
		var buf bytes.Buffer
		PrintMoveResult(&buf, &MoveResult{TaskKey: "PROJ-123", Target: "In Progress"})
		out := buf.String()
		if !strings.Contains(out, "PROJ-123") || !strings.Contains(out, "In Progress") {
			t.Errorf("output = %q, want to contain PROJ-123 and In Progress", out)
		}
	})
}
