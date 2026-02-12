package commands

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

type mockTaskCompleter struct {
	calls []string
	err   error
}

func (m *mockTaskCompleter) CompleteTask(_ context.Context, taskKey string) error {
	m.calls = append(m.calls, taskKey)
	return m.err
}

func TestRunDone(t *testing.T) {
	t.Run("marks task as done", func(t *testing.T) {
		completer := &mockTaskCompleter{}
		result, err := RunDone(context.Background(), DoneParams{
			TaskKey:   "PROJ-123",
			Completer: completer,
		})
		if err != nil {
			t.Fatalf("RunDone: %v", err)
		}

		if len(completer.calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(completer.calls))
		}
		if completer.calls[0] != "PROJ-123" {
			t.Errorf("task key = %q, want PROJ-123", completer.calls[0])
		}
		if result.TaskKey != "PROJ-123" {
			t.Errorf("result.TaskKey = %q, want PROJ-123", result.TaskKey)
		}
	})

	t.Run("returns error from completer", func(t *testing.T) {
		completer := &mockTaskCompleter{err: fmt.Errorf("transition failed")}
		_, err := RunDone(context.Background(), DoneParams{
			TaskKey:   "PROJ-456",
			Completer: completer,
		})
		if err == nil {
			t.Fatal("expected error from completer")
		}
		if !strings.Contains(err.Error(), "transition failed") {
			t.Errorf("error = %q, want to contain 'transition failed'", err.Error())
		}
	})

	t.Run("confirmation message displayed", func(t *testing.T) {
		var buf bytes.Buffer
		PrintDoneResult(&buf, &DoneResult{TaskKey: "PROJ-789"})
		out := buf.String()
		if !strings.Contains(out, "PROJ-789") {
			t.Errorf("output = %q, want to contain PROJ-789", out)
		}
		if !strings.Contains(out, "done") {
			t.Errorf("output = %q, want to contain 'done'", out)
		}
	})
}
