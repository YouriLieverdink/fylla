package commands

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

type mockTaskDeleter struct {
	calls []string
	err   error
}

func (m *mockTaskDeleter) DeleteTask(_ context.Context, taskKey string) error {
	m.calls = append(m.calls, taskKey)
	return m.err
}

func TestRunDelete(t *testing.T) {
	t.Run("deletes task", func(t *testing.T) {
		deleter := &mockTaskDeleter{}
		result, err := RunDelete(context.Background(), DeleteParams{
			TaskKey: "PROJ-123",
			Deleter: deleter,
		})
		if err != nil {
			t.Fatalf("RunDelete: %v", err)
		}

		if len(deleter.calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(deleter.calls))
		}
		if deleter.calls[0] != "PROJ-123" {
			t.Errorf("task key = %q, want PROJ-123", deleter.calls[0])
		}
		if result.TaskKey != "PROJ-123" {
			t.Errorf("result.TaskKey = %q, want PROJ-123", result.TaskKey)
		}
	})

	t.Run("returns error from deleter", func(t *testing.T) {
		deleter := &mockTaskDeleter{err: fmt.Errorf("not found")}
		_, err := RunDelete(context.Background(), DeleteParams{
			TaskKey: "PROJ-456",
			Deleter: deleter,
		})
		if err == nil {
			t.Fatal("expected error from deleter")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error = %q, want to contain 'not found'", err.Error())
		}
	})

	t.Run("calls DeleteTask even with provider set for non-multi source", func(t *testing.T) {
		deleter := &mockTaskDeleter{}
		result, err := RunDelete(context.Background(), DeleteParams{
			TaskKey:  "PROJ-123",
			Provider: "kendo",
			Deleter:  deleter,
		})
		if err != nil {
			t.Fatalf("RunDelete: %v", err)
		}
		// For non-MultiTaskSource, routedSource returns the source unchanged.
		if len(deleter.calls) != 1 {
			t.Errorf("expected 1 DeleteTask call, got %d", len(deleter.calls))
		}
		if result.TaskKey != "PROJ-123" {
			t.Errorf("result.TaskKey = %q, want PROJ-123", result.TaskKey)
		}
	})

	t.Run("confirmation message displayed", func(t *testing.T) {
		var buf bytes.Buffer
		PrintDeleteResult(&buf, &DeleteResult{TaskKey: "PROJ-789"})
		out := buf.String()
		if !strings.Contains(out, "PROJ-789") {
			t.Errorf("output = %q, want to contain PROJ-789", out)
		}
		if !strings.Contains(out, "Deleted") {
			t.Errorf("output = %q, want to contain 'Deleted'", out)
		}
	})
}
