package local

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	return NewClient(filepath.Join(t.TempDir(), "local_tasks.yaml"))
}

func TestCreateAndFetch(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	key, err := c.CreateTask(ctx, task.CreateInput{
		Summary:  "Buy groceries",
		Project:  "Personal",
		Section:  "Errands",
		Priority: "High",
		Estimate: 30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if key != "L-1" {
		t.Fatalf("expected L-1, got %s", key)
	}

	tasks, err := c.FetchTasks(ctx, "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Key != "L-1" {
		t.Errorf("key = %s, want L-1", tasks[0].Key)
	}
	if tasks[0].Summary != "Buy groceries" {
		t.Errorf("summary = %q, want %q", tasks[0].Summary, "Buy groceries")
	}
	if tasks[0].Project != "Personal" {
		t.Errorf("project = %q, want %q", tasks[0].Project, "Personal")
	}
	if tasks[0].Priority != 2 {
		t.Errorf("priority = %d, want 2", tasks[0].Priority)
	}
	if tasks[0].RemainingEstimate != 30*time.Minute {
		t.Errorf("estimate = %v, want 30m", tasks[0].RemainingEstimate)
	}
}

func TestAutoIncrement(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	k1, _ := c.CreateTask(ctx, task.CreateInput{Summary: "First"})
	k2, _ := c.CreateTask(ctx, task.CreateInput{Summary: "Second"})

	if k1 != "L-1" || k2 != "L-2" {
		t.Errorf("keys = %s, %s; want L-1, L-2", k1, k2)
	}
}

func TestComplete(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	c.CreateTask(ctx, task.CreateInput{Summary: "Do something"})

	if err := c.CompleteTask(ctx, "L-1"); err != nil {
		t.Fatalf("complete: %v", err)
	}

	tasks, _ := c.FetchTasks(ctx, "")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after complete, got %d", len(tasks))
	}
}

func TestDelete(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	c.CreateTask(ctx, task.CreateInput{Summary: "To delete"})

	if err := c.DeleteTask(ctx, "L-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	tasks, _ := c.FetchTasks(ctx, "")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
	}
}

func TestFilterByProject(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	c.CreateTask(ctx, task.CreateInput{Summary: "Work task", Project: "Work"})
	c.CreateTask(ctx, task.CreateInput{Summary: "Home task", Project: "Home"})

	tasks, _ := c.FetchTasks(ctx, "project:Work")
	if len(tasks) != 1 || tasks[0].Summary != "Work task" {
		t.Errorf("project filter: got %d tasks", len(tasks))
	}
}

func TestFilterBySection(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	c.CreateTask(ctx, task.CreateInput{Summary: "Bug fix", Section: "Bugs"})
	c.CreateTask(ctx, task.CreateInput{Summary: "Feature", Section: "Features"})

	tasks, _ := c.FetchTasks(ctx, "section:Bugs")
	if len(tasks) != 1 || tasks[0].Summary != "Bug fix" {
		t.Errorf("section filter: got %d tasks", len(tasks))
	}
}

func TestTextFilter(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	c.CreateTask(ctx, task.CreateInput{Summary: "Buy groceries"})
	c.CreateTask(ctx, task.CreateInput{Summary: "Read book"})

	tasks, _ := c.FetchTasks(ctx, "groceries")
	if len(tasks) != 1 || tasks[0].Summary != "Buy groceries" {
		t.Errorf("text filter: got %d tasks", len(tasks))
	}
}

func TestGettersAndUpdaters(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	c.CreateTask(ctx, task.CreateInput{
		Summary:  "Test task",
		Priority: "Low",
		Estimate: time.Hour,
	})

	t.Run("GetEstimate", func(t *testing.T) {
		est, err := c.GetEstimate(ctx, "L-1")
		if err != nil {
			t.Fatal(err)
		}
		if est != time.Hour {
			t.Errorf("estimate = %v, want 1h", est)
		}
	})

	t.Run("UpdateEstimate", func(t *testing.T) {
		if err := c.UpdateEstimate(ctx, "L-1", 2*time.Hour); err != nil {
			t.Fatal(err)
		}
		est, _ := c.GetEstimate(ctx, "L-1")
		if est != 2*time.Hour {
			t.Errorf("estimate = %v, want 2h", est)
		}
	})

	t.Run("GetPriority", func(t *testing.T) {
		p, err := c.GetPriority(ctx, "L-1")
		if err != nil {
			t.Fatal(err)
		}
		if p != 4 {
			t.Errorf("priority = %d, want 4", p)
		}
	})

	t.Run("UpdatePriority", func(t *testing.T) {
		if err := c.UpdatePriority(ctx, "L-1", 1); err != nil {
			t.Fatal(err)
		}
		p, _ := c.GetPriority(ctx, "L-1")
		if p != 1 {
			t.Errorf("priority = %d, want 1", p)
		}
	})

	t.Run("DueDate", func(t *testing.T) {
		d, _ := c.GetDueDate(ctx, "L-1")
		if d != nil {
			t.Errorf("expected nil due date")
		}

		due := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
		if err := c.UpdateDueDate(ctx, "L-1", due); err != nil {
			t.Fatal(err)
		}
		d, _ = c.GetDueDate(ctx, "L-1")
		if d == nil || d.Format("2006-01-02") != "2026-03-15" {
			t.Errorf("due date = %v, want 2026-03-15", d)
		}

		if err := c.RemoveDueDate(ctx, "L-1"); err != nil {
			t.Fatal(err)
		}
		d, _ = c.GetDueDate(ctx, "L-1")
		if d != nil {
			t.Errorf("expected nil due date after remove")
		}
	})

	t.Run("Summary", func(t *testing.T) {
		s, _ := c.GetSummary(ctx, "L-1")
		if s != "Test task" {
			t.Errorf("summary = %q, want %q", s, "Test task")
		}

		if err := c.UpdateSummary(ctx, "L-1", "Updated task"); err != nil {
			t.Fatal(err)
		}
		s, _ = c.GetSummary(ctx, "L-1")
		if s != "Updated task" {
			t.Errorf("summary = %q, want %q", s, "Updated task")
		}
	})
}

func TestListProjects(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	c.CreateTask(ctx, task.CreateInput{Summary: "A", Project: "Work"})
	c.CreateTask(ctx, task.CreateInput{Summary: "B", Project: "Home"})
	c.CreateTask(ctx, task.CreateInput{Summary: "C", Project: "Work"})

	projects, err := c.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

func TestNotFoundErrors(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	if err := c.CompleteTask(ctx, "L-999"); err == nil {
		t.Error("expected error for non-existent task")
	}
	if err := c.DeleteTask(ctx, "L-999"); err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestPostWorklogNoOp(t *testing.T) {
	c := newTestClient(t)
	if err := c.PostWorklog(context.Background(), "L-1", time.Hour, "test", time.Now()); err != nil {
		t.Errorf("expected no-op, got error: %v", err)
	}
}

func TestEstimateFromSummaryBracket(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	c.CreateTask(ctx, task.CreateInput{Summary: "Do thing [45m]"})

	tasks, _ := c.FetchTasks(ctx, "")
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].RemainingEstimate != 45*time.Minute {
		t.Errorf("estimate = %v, want 45m", tasks[0].RemainingEstimate)
	}
	if tasks[0].Summary != "Do thing" {
		t.Errorf("summary = %q, want %q", tasks[0].Summary, "Do thing")
	}
}
