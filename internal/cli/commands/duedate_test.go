package commands

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// mockDueDateUpdater records UpdateDueDate calls.
type mockDueDateUpdater struct {
	calls []dueDateCall
	err   error
}

type dueDateCall struct {
	issueKey string
	dueDate  time.Time
}

func (m *mockDueDateUpdater) UpdateDueDate(_ context.Context, issueKey string, dueDate time.Time) error {
	m.calls = append(m.calls, dueDateCall{issueKey, dueDate})
	return m.err
}

// mockDueDateGetter returns a preset current due date.
type mockDueDateGetter struct {
	dueDate *time.Time
	err     error
}

func (m *mockDueDateGetter) GetDueDate(_ context.Context, _ string) (*time.Time, error) {
	return m.dueDate, m.err
}

func TestDueDate_sets_absolute(t *testing.T) {
	t.Run("sets due date to specific date", func(t *testing.T) {
		updater := &mockDueDateUpdater{}
		result, err := RunDueDate(context.Background(), DueDateParams{
			TaskKey: "PROJ-123",
			Date:    "2025-03-15",
			Jira:    updater,
		})
		if err != nil {
			t.Fatalf("RunDueDate: %v", err)
		}

		if len(updater.calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(updater.calls))
		}
		if updater.calls[0].issueKey != "PROJ-123" {
			t.Errorf("issueKey = %q, want PROJ-123", updater.calls[0].issueKey)
		}
		want := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
		if !updater.calls[0].dueDate.Equal(want) {
			t.Errorf("dueDate = %v, want %v", updater.calls[0].dueDate, want)
		}

		if result.TaskKey != "PROJ-123" {
			t.Errorf("result.TaskKey = %q, want PROJ-123", result.TaskKey)
		}
		if !result.DueDate.Equal(want) {
			t.Errorf("result.DueDate = %v, want %v", result.DueDate, want)
		}
	})

	t.Run("confirmation message displayed", func(t *testing.T) {
		var buf bytes.Buffer
		PrintDueDateResult(&buf, &DueDateResult{
			TaskKey: "PROJ-123",
			DueDate: time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		})
		out := buf.String()
		if !strings.Contains(out, "PROJ-123") {
			t.Errorf("output = %q, want to contain PROJ-123", out)
		}
		if !strings.Contains(out, "2025-03-15") {
			t.Errorf("output = %q, want to contain 2025-03-15", out)
		}
	})

	t.Run("returns error for invalid date", func(t *testing.T) {
		updater := &mockDueDateUpdater{}
		_, err := RunDueDate(context.Background(), DueDateParams{
			TaskKey: "PROJ-123",
			Date:    "invalid",
			Jira:    updater,
		})
		if err == nil {
			t.Fatal("expected error for invalid date")
		}
	})

	t.Run("returns error for empty date", func(t *testing.T) {
		updater := &mockDueDateUpdater{}
		_, err := RunDueDate(context.Background(), DueDateParams{
			TaskKey: "PROJ-123",
			Date:    "",
			Jira:    updater,
		})
		if err == nil {
			t.Fatal("expected error for empty date")
		}
	})

	t.Run("returns error from Jira update", func(t *testing.T) {
		updater := &mockDueDateUpdater{err: fmt.Errorf("jira error")}
		_, err := RunDueDate(context.Background(), DueDateParams{
			TaskKey: "PROJ-123",
			Date:    "2025-03-15",
			Jira:    updater,
		})
		if err == nil {
			t.Fatal("expected error from Jira")
		}
	})
}

func TestDueDate_relative_adjustments(t *testing.T) {
	t.Run("add 7 days to current due date", func(t *testing.T) {
		current := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
		updater := &mockDueDateUpdater{}
		getter := &mockDueDateGetter{dueDate: &current}
		result, err := RunDueDate(context.Background(), DueDateParams{
			TaskKey: "PROJ-123",
			Date:    "+7d",
			Jira:    updater,
			Getter:  getter,
		})
		if err != nil {
			t.Fatalf("RunDueDate: %v", err)
		}

		want := time.Date(2025, 3, 17, 0, 0, 0, 0, time.UTC)
		if !result.DueDate.Equal(want) {
			t.Errorf("result.DueDate = %v, want %v", result.DueDate, want)
		}
	})

	t.Run("subtract 3 days from current due date", func(t *testing.T) {
		current := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
		updater := &mockDueDateUpdater{}
		getter := &mockDueDateGetter{dueDate: &current}
		result, err := RunDueDate(context.Background(), DueDateParams{
			TaskKey: "PROJ-123",
			Date:    "-3d",
			Jira:    updater,
			Getter:  getter,
		})
		if err != nil {
			t.Fatalf("RunDueDate: %v", err)
		}

		want := time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC)
		if !result.DueDate.Equal(want) {
			t.Errorf("result.DueDate = %v, want %v", result.DueDate, want)
		}
	})

	t.Run("relative adjustment with no current due date uses today", func(t *testing.T) {
		updater := &mockDueDateUpdater{}
		getter := &mockDueDateGetter{dueDate: nil}
		result, err := RunDueDate(context.Background(), DueDateParams{
			TaskKey: "PROJ-123",
			Date:    "+7d",
			Jira:    updater,
			Getter:  getter,
		})
		if err != nil {
			t.Fatalf("RunDueDate: %v", err)
		}

		// Should be approximately 7 days from now
		expected := time.Now().AddDate(0, 0, 7)
		diff := result.DueDate.Sub(expected)
		if diff < -time.Minute || diff > time.Minute {
			t.Errorf("result.DueDate = %v, want ~%v", result.DueDate, expected)
		}
	})

	t.Run("returns error when getter fails", func(t *testing.T) {
		updater := &mockDueDateUpdater{}
		getter := &mockDueDateGetter{err: fmt.Errorf("jira error")}
		_, err := RunDueDate(context.Background(), DueDateParams{
			TaskKey: "PROJ-123",
			Date:    "+7d",
			Jira:    updater,
			Getter:  getter,
		})
		if err == nil {
			t.Fatal("expected error from getter")
		}
	})

	t.Run("returns error for invalid relative offset", func(t *testing.T) {
		updater := &mockDueDateUpdater{}
		getter := &mockDueDateGetter{}
		_, err := RunDueDate(context.Background(), DueDateParams{
			TaskKey: "PROJ-123",
			Date:    "+invalid",
			Jira:    updater,
			Getter:  getter,
		})
		if err == nil {
			t.Fatal("expected error for invalid relative offset")
		}
	})
}

func TestParseDate(t *testing.T) {
	t.Run("valid date", func(t *testing.T) {
		d, err := ParseDate("2025-03-15")
		if err != nil {
			t.Fatalf("ParseDate: %v", err)
		}
		want := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
		if !d.Equal(want) {
			t.Errorf("ParseDate = %v, want %v", d, want)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := ParseDate("15-03-2025")
		if err == nil {
			t.Fatal("expected error for invalid format")
		}
	})

	t.Run("invalid date", func(t *testing.T) {
		_, err := ParseDate("not-a-date")
		if err == nil {
			t.Fatal("expected error for invalid date")
		}
	})
}
