package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

type mockViewSource struct {
	summary  string
	estimate time.Duration
	dueDate  *time.Time
	priority int
}

func (m *mockViewSource) GetSummary(_ context.Context, _ string) (string, error) {
	return m.summary, nil
}
func (m *mockViewSource) GetEstimate(_ context.Context, _ string) (time.Duration, error) {
	return m.estimate, nil
}
func (m *mockViewSource) GetDueDate(_ context.Context, _ string) (*time.Time, error) {
	return m.dueDate, nil
}
func (m *mockViewSource) GetPriority(_ context.Context, _ string) (int, error) {
	return m.priority, nil
}
func (m *mockViewSource) FetchTasks(_ context.Context, _ string) ([]task.Task, error) {
	return nil, nil
}
func (m *mockViewSource) CreateTask(_ context.Context, _ task.CreateInput) (string, error) {
	return "", nil
}
func (m *mockViewSource) CompleteTask(_ context.Context, _ string) error              { return nil }
func (m *mockViewSource) DeleteTask(_ context.Context, _ string) error                { return nil }
func (m *mockViewSource) UpdateEstimate(_ context.Context, _ string, _ time.Duration) error {
	return nil
}
func (m *mockViewSource) UpdateDueDate(_ context.Context, _ string, _ time.Time) error { return nil }
func (m *mockViewSource) RemoveDueDate(_ context.Context, _ string) error              { return nil }
func (m *mockViewSource) UpdatePriority(_ context.Context, _ string, _ int) error      { return nil }
func (m *mockViewSource) UpdateSummary(_ context.Context, _ string, _ string) error    { return nil }
func (m *mockViewSource) PostWorklog(_ context.Context, _ string, _ time.Duration, _ string, _ time.Time) error {
	return nil
}

func TestRunView(t *testing.T) {
	due := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	source := &mockViewSource{
		summary:  "Write docs upnext",
		estimate: 2 * time.Hour,
		dueDate:  &due,
		priority: 2,
	}

	result, err := RunView(context.Background(), ViewParams{
		TaskKey: "L-1",
		Source:  source,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Key != "L-1" {
		t.Errorf("key = %s", result.Key)
	}
	if result.Summary != "Write docs" {
		t.Errorf("summary = %q, want %q", result.Summary, "Write docs")
	}
	if !result.UpNext {
		t.Error("expected UpNext to be true")
	}
	if result.Estimate != 2*time.Hour {
		t.Errorf("estimate = %v", result.Estimate)
	}

	var buf bytes.Buffer
	PrintViewResult(&buf, result)
	output := buf.String()
	if !strings.Contains(output, "L-1") {
		t.Error("output should contain key")
	}
	if !strings.Contains(output, "Write docs") {
		t.Error("output should contain summary")
	}
	if !strings.Contains(output, "High") {
		t.Error("output should contain priority name")
	}
}
