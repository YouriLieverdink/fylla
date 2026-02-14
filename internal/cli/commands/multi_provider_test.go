package commands

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/task"
)

func TestIsJiraKey(t *testing.T) {
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
			if got := isJiraKey(tt.key); got != tt.want {
				t.Errorf("isJiraKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestProviderForKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"PROJ-123", "jira"},
		{"AB-1", "jira"},
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
	name         string
	completedKey string
	deletedKey   string
	fetchQuery   string
	tasks        []task.Task
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

func (m *mockSource) PostWorklog(_ context.Context, _ string, _ time.Duration, _ string) error {
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

func TestMultiTaskSource_RoutesJiraKeys(t *testing.T) {
	jiraSrc := &mockSource{name: "jira"}
	todoistSrc := &mockSource{name: "todoist"}
	ms := NewMultiTaskSource(
		map[string]TaskSource{"jira": jiraSrc, "todoist": todoistSrc},
		[]string{"jira", "todoist"},
	)

	t.Run("CompleteTask routes to jira", func(t *testing.T) {
		ms.CompleteTask(context.Background(), "PROJ-123")
		if jiraSrc.completedKey != "PROJ-123" {
			t.Errorf("expected jira to handle PROJ-123, got %q", jiraSrc.completedKey)
		}
	})

	t.Run("CompleteTask routes to todoist", func(t *testing.T) {
		ms.CompleteTask(context.Background(), "8765432101")
		if todoistSrc.completedKey != "8765432101" {
			t.Errorf("expected todoist to handle 8765432101, got %q", todoistSrc.completedKey)
		}
	})

	t.Run("DeleteTask routes to jira", func(t *testing.T) {
		ms.DeleteTask(context.Background(), "TEST-42")
		if jiraSrc.deletedKey != "TEST-42" {
			t.Errorf("expected jira to handle TEST-42, got %q", jiraSrc.deletedKey)
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
		if key != "jira-NEW" {
			t.Errorf("CreateTask returned %q, want jira-NEW", key)
		}
	})

	t.Run("CreateTaskOn routes to specific provider", func(t *testing.T) {
		key, _ := ms.CreateTaskOn(context.Background(), "todoist", task.CreateInput{Summary: "test"})
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
		jiraSrc := &mockSource{
			name:  "jira",
			tasks: []task.Task{{Key: "PROJ-1", Summary: "Jira task"}},
		}
		todoistSrc := &mockSource{
			name:  "todoist",
			tasks: []task.Task{{Key: "123", Summary: "Todoist task"}},
		}

		mf := &multiFetcher{
			queries: map[string]string{"jira": "assignee=me", "todoist": "today"},
			sources: map[string]TaskSource{"jira": jiraSrc, "todoist": todoistSrc},
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

	t.Run("partial success continues with warnings", func(t *testing.T) {
		jiraSrc := &mockSource{
			name:  "jira",
			tasks: []task.Task{{Key: "PROJ-1", Summary: "Jira task"}},
		}
		failSrc := &failingSource{}

		mf := &multiFetcher{
			queries: map[string]string{"jira": "assignee=me", "todoist": "today"},
			sources: map[string]TaskSource{"jira": jiraSrc, "todoist": failSrc},
		}

		tasks, err := mf.FetchTasks(context.Background(), "")
		if err != nil {
			t.Fatalf("expected partial success, got error: %v", err)
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
			queries: map[string]string{"jira": "q1", "todoist": "q2"},
			sources: map[string]TaskSource{"jira": failSrc1, "todoist": failSrc2},
		}

		_, err := mf.FetchTasks(context.Background(), "")
		if err == nil {
			t.Fatal("expected error when all providers fail")
		}
	})
}

func TestBuildProviderQueries(t *testing.T) {
	t.Run("single jira provider uses JQL flag", func(t *testing.T) {
		cfg := &config.Config{Providers: []string{"jira"}, Jira: config.JiraConfig{DefaultJQL: "default jql"}}
		queries := buildProviderQueries(cfg, "custom jql", "")
		if queries["jira"] != "custom jql" {
			t.Errorf("jira query = %q, want custom jql", queries["jira"])
		}
	})

	t.Run("single jira provider falls back to config default", func(t *testing.T) {
		cfg := &config.Config{Providers: []string{"jira"}, Jira: config.JiraConfig{DefaultJQL: "default jql"}}
		queries := buildProviderQueries(cfg, "", "")
		if queries["jira"] != "default jql" {
			t.Errorf("jira query = %q, want default jql", queries["jira"])
		}
	})

	t.Run("multi-provider builds separate queries", func(t *testing.T) {
		cfg := &config.Config{
			Providers: []string{"jira", "todoist"},
			Jira:      config.JiraConfig{DefaultJQL: "assignee = me"},
			Todoist:   config.TodoistConfig{DefaultFilter: "today | overdue"},
		}
		queries := buildProviderQueries(cfg, "", "")
		if queries["jira"] != "assignee = me" {
			t.Errorf("jira query = %q", queries["jira"])
		}
		if queries["todoist"] != "today | overdue" {
			t.Errorf("todoist query = %q", queries["todoist"])
		}
	})

	t.Run("flags override config defaults", func(t *testing.T) {
		cfg := &config.Config{
			Providers: []string{"jira", "todoist"},
			Jira:      config.JiraConfig{DefaultJQL: "default"},
			Todoist:   config.TodoistConfig{DefaultFilter: "default"},
		}
		queries := buildProviderQueries(cfg, "custom jql", "custom filter")
		if queries["jira"] != "custom jql" {
			t.Errorf("jira query = %q, want custom jql", queries["jira"])
		}
		if queries["todoist"] != "custom filter" {
			t.Errorf("todoist query = %q, want custom filter", queries["todoist"])
		}
	})
}
