package commands

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/task"
)

func TestIsKendoKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"PROJ-123", true},
		{"AB-1", true},
		{"A1-99", true},
		{"MY2PROJ-42", true},
		{"123456", false},
		{"proj-123", false},
		{"PROJ", false},
		{"PROJ-", false},
		{"-123", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := isKendoKey(tt.key); got != tt.want {
				t.Errorf("isKendoKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestProviderForKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"PROJ-123", "kendo"},
		{"AB-1", "kendo"},
		{"123456", "todoist"},
		{"8765432101", "todoist"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := providerForKey(tt.key); got != tt.want {
				t.Errorf("providerForKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// mockSource records method calls for testing MultiTaskSource routing.
type mockSource struct {
	name           string
	completedKey   string
	deletedKey     string
	fetchQuery     string
	tasks          []task.Task
	summary        string
	updatedSummary string
}

func (m *mockSource) FetchTasks(_ context.Context, query string) ([]task.Task, error) {
	m.fetchQuery = query
	return m.tasks, nil
}

func (m *mockSource) CreateTask(_ context.Context, input task.CreateInput) (string, error) {
	return m.name + "-NEW", nil
}

func (m *mockSource) CompleteTask(_ context.Context, taskKey string) error {
	m.completedKey = taskKey
	return nil
}

func (m *mockSource) DeleteTask(_ context.Context, taskKey string) error {
	m.deletedKey = taskKey
	return nil
}

func (m *mockSource) PostWorklog(_ context.Context, _ string, _ time.Duration, _ string, _ time.Time) error {
	return nil
}

func (m *mockSource) GetEstimate(_ context.Context, _ string) (time.Duration, error) {
	return time.Hour, nil
}

func (m *mockSource) UpdateEstimate(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (m *mockSource) GetDueDate(_ context.Context, _ string) (*time.Time, error) {
	return nil, nil
}

func (m *mockSource) UpdateDueDate(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (m *mockSource) GetPriority(_ context.Context, _ string) (int, error) {
	return 3, nil
}

func (m *mockSource) UpdatePriority(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockSource) RemoveDueDate(_ context.Context, _ string) error {
	return nil
}

func (m *mockSource) GetSummary(_ context.Context, _ string) (string, error) {
	return m.summary, nil
}

func (m *mockSource) UpdateSummary(_ context.Context, _ string, summary string) error {
	m.updatedSummary = summary
	return nil
}

func TestMultiTaskSource_RoutesKendoKeys(t *testing.T) {
	kendoSrc := &mockSource{name: "kendo"}
	todoistSrc := &mockSource{name: "todoist"}
	ms := NewMultiTaskSource(
		map[string]TaskSource{"kendo": kendoSrc, "todoist": todoistSrc},
		[]string{"kendo", "todoist"},
	)

	t.Run("CompleteTask routes to kendo", func(t *testing.T) {
		ms.CompleteTask(context.Background(), "PROJ-123")
		if kendoSrc.completedKey != "PROJ-123" {
			t.Errorf("expected kendo to handle PROJ-123, got %q", kendoSrc.completedKey)
		}
	})

	t.Run("CompleteTask routes to todoist", func(t *testing.T) {
		ms.CompleteTask(context.Background(), "8765432101")
		if todoistSrc.completedKey != "8765432101" {
			t.Errorf("expected todoist to handle 8765432101, got %q", todoistSrc.completedKey)
		}
	})

	t.Run("DeleteTask routes to kendo", func(t *testing.T) {
		ms.DeleteTask(context.Background(), "TEST-42")
		if kendoSrc.deletedKey != "TEST-42" {
			t.Errorf("expected kendo to handle TEST-42, got %q", kendoSrc.deletedKey)
		}
	})

	t.Run("DeleteTask routes to todoist", func(t *testing.T) {
		ms.DeleteTask(context.Background(), "99999")
		if todoistSrc.deletedKey != "99999" {
			t.Errorf("expected todoist to handle 99999, got %q", todoistSrc.deletedKey)
		}
	})

	t.Run("CreateTask defaults to first provider", func(t *testing.T) {
		key, _ := ms.CreateTask(context.Background(), task.CreateInput{Summary: "test"})
		if key != "kendo-NEW" {
			t.Errorf("CreateTask returned %q, want kendo-NEW", key)
		}
	})

	t.Run("CreateTaskOn routes to specific provider", func(t *testing.T) {
		src, _ := ms.RouteToProvider("todoist")
		key, _ := src.CreateTask(context.Background(), task.CreateInput{Summary: "test"})
		if key != "todoist-NEW" {
			t.Errorf("CreateTaskOn returned %q, want todoist-NEW", key)
		}
	})
}

// failingSource always returns an error on FetchTasks.
type failingSource struct{ mockSource }

func (f *failingSource) FetchTasks(_ context.Context, _ string) ([]task.Task, error) {
	return nil, fmt.Errorf("connection refused")
}

func TestMultiFetcher_ConcurrentFetch(t *testing.T) {
	t.Run("merges results from multiple providers", func(t *testing.T) {
		kendoSrc := &mockSource{
			name:  "kendo",
			tasks: []task.Task{{Key: "PROJ-1", Summary: "Kendo task"}},
		}
		todoistSrc := &mockSource{
			name:  "todoist",
			tasks: []task.Task{{Key: "123", Summary: "Todoist task"}},
		}

		mf := &multiFetcher{
			queries: map[string]string{"kendo": "assignee=me", "todoist": "today"},
			sources: map[string]TaskSource{"kendo": kendoSrc, "todoist": todoistSrc},
		}

		tasks, err := mf.FetchTasks(context.Background(), "")
		if err != nil {
			t.Fatalf("FetchTasks: %v", err)
		}
		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(tasks))
		}

		keys := make(map[string]bool)
		for _, task := range tasks {
			keys[task.Key] = true
		}
		if !keys["PROJ-1"] || !keys["123"] {
			t.Errorf("expected tasks from both providers, got keys %v", keys)
		}
	})

	t.Run("partial failure returns error with partial tasks", func(t *testing.T) {
		kendoSrc := &mockSource{
			name:  "kendo",
			tasks: []task.Task{{Key: "PROJ-1", Summary: "Kendo task"}},
		}
		failSrc := &failingSource{}

		mf := &multiFetcher{
			queries: map[string]string{"kendo": "assignee=me", "todoist": "today"},
			sources: map[string]TaskSource{"kendo": kendoSrc, "todoist": failSrc},
		}

		tasks, err := mf.FetchTasks(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for partial failure")
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task from successful provider, got %d", len(tasks))
		}
		if tasks[0].Key != "PROJ-1" {
			t.Errorf("expected PROJ-1, got %q", tasks[0].Key)
		}
	})

	t.Run("all providers fail returns error", func(t *testing.T) {
		failSrc1 := &failingSource{}
		failSrc2 := &failingSource{}

		mf := &multiFetcher{
			queries: map[string]string{"kendo": "q1", "todoist": "q2"},
			sources: map[string]TaskSource{"kendo": failSrc1, "todoist": failSrc2},
		}

		_, err := mf.FetchTasks(context.Background(), "")
		if err == nil {
			t.Fatal("expected error when all providers fail")
		}
	})
}

func TestBuildProviderQueries(t *testing.T) {
	t.Run("single kendo provider uses filter flag", func(t *testing.T) {
		cfg := &config.Config{Providers: []string{"kendo"}, Kendo: config.KendoConfig{DefaultFilter: "default filter"}}
		queries := buildProviderQueries(cfg, "custom filter")
		if queries["kendo"] != "custom filter" {
			t.Errorf("kendo query = %q, want custom filter", queries["kendo"])
		}
	})

	t.Run("single kendo provider falls back to config default", func(t *testing.T) {
		cfg := &config.Config{Providers: []string{"kendo"}, Kendo: config.KendoConfig{DefaultFilter: "default filter"}}
		queries := buildProviderQueries(cfg, "")
		if queries["kendo"] != "default filter" {
			t.Errorf("kendo query = %q, want default filter", queries["kendo"])
		}
	})

	t.Run("multi-provider builds separate queries", func(t *testing.T) {
		cfg := &config.Config{
			Providers: []string{"kendo", "todoist"},
			Kendo:     config.KendoConfig{DefaultFilter: "assignee = me"},
			Todoist:   config.TodoistConfig{DefaultFilter: "today | overdue"},
		}
		queries := buildProviderQueries(cfg, "")
		if queries["kendo"] != "assignee = me" {
			t.Errorf("kendo query = %q", queries["kendo"])
		}
		if queries["todoist"] != "today | overdue" {
			t.Errorf("todoist query = %q", queries["todoist"])
		}
	})

	t.Run("filter flag overrides config defaults", func(t *testing.T) {
		cfg := &config.Config{
			Providers: []string{"kendo", "todoist"},
			Kendo:     config.KendoConfig{DefaultFilter: "default"},
			Todoist:   config.TodoistConfig{DefaultFilter: "default"},
		}
		queries := buildProviderQueries(cfg, "custom filter")
		if queries["kendo"] != "custom filter" {
			t.Errorf("kendo query = %q, want custom filter", queries["kendo"])
		}
		if queries["todoist"] != "custom filter" {
			t.Errorf("todoist query = %q, want custom filter", queries["todoist"])
		}
	})
}
