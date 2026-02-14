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

// mockTaskCreator records CreateTask calls.
type mockTaskCreator struct {
	calls []task.CreateInput
	key   string
	err   error
}

func (m *mockTaskCreator) CreateTask(_ context.Context, input task.CreateInput) (string, error) {
	m.calls = append(m.calls, input)
	return m.key, m.err
}

func TestCLI017_add_interactive_creation(t *testing.T) {
	t.Run("creates issue with all fields", func(t *testing.T) {
		mock := &mockTaskCreator{key: "PROJ-456"}
		result, err := RunAdd(context.Background(), AddParams{
			Project:     "PROJ",
			IssueType:   "Bug",
			Summary:     "Fix the login timeout issue",
			Description: "Users are being logged out after 5 minutes",
			Estimate:    "2h",
			Priority:    "High",
			Creator:     mock,
		})
		if err != nil {
			t.Fatalf("RunAdd: %v", err)
		}

		if len(mock.calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.calls))
		}
		c := mock.calls[0]
		if c.Project != "PROJ" {
			t.Errorf("Project = %q, want PROJ", c.Project)
		}
		if c.IssueType != "Bug" {
			t.Errorf("IssueType = %q, want Bug", c.IssueType)
		}
		if c.Summary != "Fix the login timeout issue" {
			t.Errorf("Summary = %q", c.Summary)
		}
		if c.Description != "Users are being logged out after 5 minutes" {
			t.Errorf("Description = %q", c.Description)
		}
		if c.Estimate != 2*time.Hour {
			t.Errorf("Estimate = %v, want 2h", c.Estimate)
		}
		if c.Priority != "High" {
			t.Errorf("Priority = %q, want High", c.Priority)
		}

		if result.Key != "PROJ-456" {
			t.Errorf("result.Key = %q, want PROJ-456", result.Key)
		}
		if result.Summary != "Fix the login timeout issue" {
			t.Errorf("result.Summary = %q", result.Summary)
		}
	})

	t.Run("interactive mode requires all fields for jira", func(t *testing.T) {
		fields := RequiredFields(AddParams{}, "jira")
		expected := []string{"project", "issueType", "summary", "description", "estimate", "dueDate", "priority"}
		if len(fields) != len(expected) {
			t.Fatalf("fields = %v, want %v", fields, expected)
		}
		for i, f := range expected {
			if fields[i] != f {
				t.Errorf("fields[%d] = %q, want %q", i, fields[i], f)
			}
		}
	})

	t.Run("interactive mode omits issueType for todoist", func(t *testing.T) {
		fields := RequiredFields(AddParams{}, "todoist")
		for _, f := range fields {
			if f == "issueType" {
				t.Error("issueType should not be in required fields for todoist provider")
			}
		}
	})

	t.Run("confirmation message shows key and summary", func(t *testing.T) {
		var buf bytes.Buffer
		PrintAddResult(&buf, &AddResult{
			Key:     "PROJ-456",
			Summary: "Fix the login timeout issue",
		})
		out := buf.String()
		if !strings.Contains(out, "PROJ-456") {
			t.Errorf("output = %q, want to contain PROJ-456", out)
		}
		if !strings.Contains(out, "Fix the login timeout issue") {
			t.Errorf("output = %q, want to contain summary", out)
		}
	})

	t.Run("returns error from creator", func(t *testing.T) {
		mock := &mockTaskCreator{err: fmt.Errorf("create error")}
		_, err := RunAdd(context.Background(), AddParams{
			Project:   "PROJ",
			IssueType: "Task",
			Summary:   "Test",
			Estimate:  "1h",
			Priority:  "Medium",
			Creator:   mock,
		})
		if err == nil {
			t.Fatal("expected error from creator")
		}
	})

	t.Run("returns error for invalid estimate", func(t *testing.T) {
		mock := &mockTaskCreator{key: "PROJ-456"}
		_, err := RunAdd(context.Background(), AddParams{
			Project:   "PROJ",
			IssueType: "Task",
			Summary:   "Test",
			Estimate:  "invalid",
			Priority:  "Medium",
			Creator:   mock,
		})
		if err == nil {
			t.Fatal("expected error for invalid estimate")
		}
	})
}

func TestCLI018_add_inline_mode(t *testing.T) {
	t.Run("inline mode only prompts project", func(t *testing.T) {
		fields := RequiredFields(AddParams{Inline: true, Summary: "Task"}, "jira")
		expected := []string{"project"}
		if len(fields) != len(expected) {
			t.Fatalf("fields = %v, want %v", fields, expected)
		}
		for i, f := range expected {
			if fields[i] != f {
				t.Errorf("fields[%d] = %q, want %q", i, fields[i], f)
			}
		}
	})

	t.Run("inline mode skips project prompt when pre-selected", func(t *testing.T) {
		fields := RequiredFields(AddParams{Inline: true, Project: "PROJ", Summary: "Task"}, "jira")
		if len(fields) != 0 {
			t.Fatalf("fields = %v, want empty", fields)
		}
	})

	t.Run("inline mode defaults priority to Medium", func(t *testing.T) {
		mock := &mockTaskCreator{key: "PROJ-457"}
		result, err := RunAdd(context.Background(), AddParams{
			Project: "PROJ",
			Summary: "Quick bugfix",
			Inline:  true,
			Creator: mock,
		})
		if err != nil {
			t.Fatalf("RunAdd: %v", err)
		}

		c := mock.calls[0]
		if c.Priority != "Medium" {
			t.Errorf("Priority = %q, want Medium (default)", c.Priority)
		}

		if result.Key != "PROJ-457" {
			t.Errorf("result.Key = %q, want PROJ-457", result.Key)
		}
	})

	t.Run("inline mode does not override provided type and priority", func(t *testing.T) {
		mock := &mockTaskCreator{key: "PROJ-458"}
		_, err := RunAdd(context.Background(), AddParams{
			Project:   "PROJ",
			IssueType: "Bug",
			Summary:   "Important bug",
			Priority:  "High",
			Inline:    true,
			Creator:   mock,
		})
		if err != nil {
			t.Fatalf("RunAdd: %v", err)
		}

		c := mock.calls[0]
		if c.IssueType != "Bug" {
			t.Errorf("IssueType = %q, want Bug (not overridden)", c.IssueType)
		}
		if c.Priority != "High" {
			t.Errorf("Priority = %q, want High (not overridden)", c.Priority)
		}
	})

	t.Run("description flows through to CreateInput", func(t *testing.T) {
		mock := &mockTaskCreator{key: "PROJ-460"}
		_, err := RunAdd(context.Background(), AddParams{
			Project:     "PROJ",
			Summary:     "Task with desc",
			Description: "Some details",
			Inline:      true,
			Creator:     mock,
		})
		if err != nil {
			t.Fatalf("RunAdd: %v", err)
		}
		if mock.calls[0].Description != "Some details" {
			t.Errorf("Description = %q, want %q", mock.calls[0].Description, "Some details")
		}
	})
}

func TestRequiredFields_interactive_populated(t *testing.T) {
	t.Run("summary populated skips summary prompt", func(t *testing.T) {
		fields := RequiredFields(AddParams{Summary: "Already set"}, "jira")
		for _, f := range fields {
			if f == "summary" {
				t.Error("summary should not be in required fields when already set")
			}
		}
	})

	t.Run("estimate populated skips estimate prompt", func(t *testing.T) {
		fields := RequiredFields(AddParams{Estimate: "2h"}, "jira")
		for _, f := range fields {
			if f == "estimate" {
				t.Error("estimate should not be in required fields when already set")
			}
		}
	})

	t.Run("due date populated skips due date prompt", func(t *testing.T) {
		fields := RequiredFields(AddParams{DueDate: "2025-02-15"}, "jira")
		for _, f := range fields {
			if f == "dueDate" {
				t.Error("dueDate should not be in required fields when already set")
			}
		}
	})

	t.Run("priority populated skips priority prompt", func(t *testing.T) {
		fields := RequiredFields(AddParams{Priority: "High"}, "jira")
		for _, f := range fields {
			if f == "priority" {
				t.Error("priority should not be in required fields when already set")
			}
		}
	})
}

func TestCLI019_add_project_preselect(t *testing.T) {
	t.Run("project flag skips project prompt", func(t *testing.T) {
		fields := RequiredFields(AddParams{Project: "PROJ"}, "jira")
		for _, f := range fields {
			if f == "project" {
				t.Error("project should not be in required fields when pre-selected")
			}
		}
	})

	t.Run("pre-selected project used in issue creation", func(t *testing.T) {
		mock := &mockTaskCreator{key: "PROJ-459"}
		result, err := RunAdd(context.Background(), AddParams{
			Project:   "PROJ",
			IssueType: "Task",
			Summary:   "Task with pre-selected project",
			Estimate:  "1h",
			Priority:  "Medium",
			Creator:   mock,
		})
		if err != nil {
			t.Fatalf("RunAdd: %v", err)
		}

		if mock.calls[0].Project != "PROJ" {
			t.Errorf("Project = %q, want PROJ", mock.calls[0].Project)
		}
		if result.Key != "PROJ-459" {
			t.Errorf("result.Key = %q, want PROJ-459", result.Key)
		}
	})

	t.Run("project flag with inline mode", func(t *testing.T) {
		fields := RequiredFields(AddParams{Project: "PROJ", Inline: true}, "jira")
		if len(fields) != 0 {
			t.Fatalf("fields = %v, want empty", fields)
		}
	})
}
