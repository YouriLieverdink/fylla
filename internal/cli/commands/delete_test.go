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

type mockProviderTaskDeleter struct {
	calls       []string
	onCalls     []string
	onProviders []string
	err         error
}

func (m *mockProviderTaskDeleter) DeleteTask(_ context.Context, taskKey string) error {
	m.calls = append(m.calls, taskKey)
	return m.err
}

func (m *mockProviderTaskDeleter) DeleteTaskOn(_ context.Context, taskKey, provider string) error {
	m.onCalls = append(m.onCalls, taskKey)
	m.onProviders = append(m.onProviders, provider)
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

	t.Run("uses DeleteTaskOn when provider is set", func(t *testing.T) {
		deleter := &mockProviderTaskDeleter{}
		result, err := RunDelete(context.Background(), DeleteParams{
			TaskKey:  "PROJ-123",
			Provider: "kendo",
			Deleter:  deleter,
		})
		if err != nil {
			t.Fatalf("RunDelete: %v", err)
		}
		if len(deleter.calls) != 0 {
			t.Errorf("expected 0 DeleteTask calls, got %d", len(deleter.calls))
		}
		if len(deleter.onCalls) != 1 {
			t.Fatalf("expected 1 DeleteTaskOn call, got %d", len(deleter.onCalls))
		}
		if deleter.onCalls[0] != "PROJ-123" {
			t.Errorf("task key = %q, want PROJ-123", deleter.onCalls[0])
		}
		if deleter.onProviders[0] != "kendo" {
			t.Errorf("provider = %q, want kendo", deleter.onProviders[0])
		}
		if result.TaskKey != "PROJ-123" {
			t.Errorf("result.TaskKey = %q, want PROJ-123", result.TaskKey)
		}
	})

	t.Run("falls back to DeleteTask when provider is empty", func(t *testing.T) {
		deleter := &mockProviderTaskDeleter{}
		_, err := RunDelete(context.Background(), DeleteParams{
			TaskKey: "PROJ-123",
			Deleter: deleter,
		})
		if err != nil {
			t.Fatalf("RunDelete: %v", err)
		}
		if len(deleter.calls) != 1 {
			t.Errorf("expected 1 DeleteTask call, got %d", len(deleter.calls))
		}
		if len(deleter.onCalls) != 0 {
			t.Errorf("expected 0 DeleteTaskOn calls, got %d", len(deleter.onCalls))
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
