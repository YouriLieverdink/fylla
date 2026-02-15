package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

func TestCLI009_list_shows_sorted_tasks(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

	t.Run("tasks displayed in priority order", func(t *testing.T) {
		due := now.AddDate(0, 0, 10)
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "LOW-1", Summary: "Low task", Priority: 5, DueDate: &due, RemainingEstimate: time.Hour, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
				{Key: "HIGH-1", Summary: "High task", Priority: 1, DueDate: &due, RemainingEstimate: time.Hour, Project: "TEST", IssueType: "Bug", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunList(context.Background(), ListParams{
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunList: %v", err)
		}

		if len(result.Tasks) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(result.Tasks))
		}
		// Higher priority task should be first
		if result.Tasks[0].Task.Key != "HIGH-1" {
			t.Errorf("first task = %q, want HIGH-1", result.Tasks[0].Task.Key)
		}
		if result.Tasks[1].Task.Key != "LOW-1" {
			t.Errorf("second task = %q, want LOW-1", result.Tasks[1].Task.Key)
		}
	})

	t.Run("no calendar events are created", func(t *testing.T) {
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "T-1", Summary: "Task", Priority: 1, RemainingEstimate: time.Hour, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		// RunList does not accept a CalendarClient — no calendar interaction possible
		_, err := RunList(context.Background(), ListParams{
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunList: %v", err)
		}
		// No calendar mock needed — RunList doesn't touch calendar at all
	})

	t.Run("output includes task details", func(t *testing.T) {
		due := now.AddDate(0, 0, 5)
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "PROJ-42", Summary: "Fix login bug", Priority: 1, DueDate: &due, RemainingEstimate: 2 * time.Hour, Project: "PROJ", IssueType: "Bug", Created: now.AddDate(0, 0, -3)},
			},
		}

		result, err := RunList(context.Background(), ListParams{
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = PROJ",
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunList: %v", err)
		}

		var buf bytes.Buffer
		PrintListResult(&buf, result, false)
		out := buf.String()

		if !strings.Contains(out, "[PROJ] PROJ-42: Fix login bug") {
			t.Errorf("output missing formatted task line, got:\n%s", out)
		}
		// Default mode should not include detail tags like issue type
		if strings.Contains(out, "Bug") {
			t.Errorf("default output should not include issue type, got:\n%s", out)
		}
	})

	t.Run("verbose output includes detail line", func(t *testing.T) {
		due := now.AddDate(0, 0, 5)
		notBefore := now.AddDate(0, 0, 2)
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "PROJ-42", Summary: "Fix login bug", Priority: 2, DueDate: &due, RemainingEstimate: 2 * time.Hour, Project: "PROJ", IssueType: "Bug", Created: now.AddDate(0, 0, -3), NotBefore: &notBefore, UpNext: true},
			},
		}

		result, err := RunList(context.Background(), ListParams{
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = PROJ",
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunList: %v", err)
		}

		var buf bytes.Buffer
		PrintListResult(&buf, result, true)
		out := buf.String()

		// Project is now in the prefix line, not in verbose details
		if !strings.Contains(out, "[PROJ] PROJ-42: Fix login bug") {
			t.Errorf("verbose output missing formatted prefix, got:\n%s", out)
		}
		for _, want := range []string{"Bug", "2h", "Due: Jan 25", "Priority: High", "Not Before: Jan 22", "Up Next"} {
			if !strings.Contains(out, want) {
				t.Errorf("verbose output missing %q, got:\n%s", want, out)
			}
		}
	})

	t.Run("section displayed in prefix", func(t *testing.T) {
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "12345678", Summary: "Buy groceries", Priority: 3, RemainingEstimate: 30 * time.Minute, Project: "Personal", Section: "Shopping", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunList(context.Background(), ListParams{
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "",
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunList: %v", err)
		}

		var buf bytes.Buffer
		PrintListResult(&buf, result, false)
		out := buf.String()

		if !strings.Contains(out, "[Personal / Shopping] 12345678: Buy groceries") {
			t.Errorf("output missing section prefix, got:\n%s", out)
		}
	})

	t.Run("displayWidth handles emoji correctly", func(t *testing.T) {
		tests := []struct {
			s    string
			want int
		}{
			{"hello", 5},
			{"🎈", 2},                // single emoji
			{"⛺️", 2},               // wide base + FE0F
			{"❣️", 2},               // narrow base + FE0F promotion
			{"🫰🏻", 2},              // emoji + skin tone (combined)
			{"[⛺️ Thuis]", 10},      // mixed with FE0F
			{"[🪷 P / ❣️ J]", 13},  // mixed with FE0F promotion
			{"[🪷 P / 🫰🏻 F]", 13}, // mixed with skin tone
		}
		for _, tt := range tests {
			got := displayWidth(tt.s)
			if got != tt.want {
				t.Errorf("displayWidth(%q) = %d, want %d", tt.s, got, tt.want)
			}
		}
	})

	t.Run("emoji alignment", func(t *testing.T) {
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "a1", Summary: "Task A", Priority: 1, RemainingEstimate: time.Hour, Project: "⛺️ Thuis", Created: now.AddDate(0, 0, -1)},
				{Key: "a2", Summary: "Task B", Priority: 2, RemainingEstimate: time.Hour, Project: "🪷 Persoonlijk", Section: "🫰🏻 Financiën", Created: now.AddDate(0, 0, -1)},
				{Key: "a3", Summary: "Task C", Priority: 3, RemainingEstimate: time.Hour, Project: "🪷 Persoonlijk", Section: "❣️ Jasmijn", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunList(context.Background(), ListParams{
			Tasks: jr,
			Cfg:   testConfig(),
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunList: %v", err)
		}

		var buf bytes.Buffer
		PrintListResult(&buf, result, false)
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

		// All score values should end at the same column.
		var scoreEnds []int
		for _, line := range lines[1:] { // skip header
			trimmed := strings.TrimRight(line, " \t")
			scoreEnds = append(scoreEnds, displayWidth(trimmed))
		}
		for i := 1; i < len(scoreEnds); i++ {
			if scoreEnds[i] != scoreEnds[0] {
				t.Errorf("score column misaligned: line 1 ends at %d, line %d ends at %d\noutput:\n%s",
					scoreEnds[0], i+1, scoreEnds[i], buf.String())
			}
		}
	})

	t.Run("empty task list handled", func(t *testing.T) {
		jr := &mockTaskFetcher{}

		result, err := RunList(context.Background(), ListParams{
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = EMPTY",
			Now:   now,
		})
		if err != nil {
			t.Fatalf("RunList: %v", err)
		}

		var buf bytes.Buffer
		PrintListResult(&buf, result, false)
		if !strings.Contains(buf.String(), "No tasks found") {
			t.Errorf("expected 'No tasks found' message, got:\n%s", buf.String())
		}
	})
}
