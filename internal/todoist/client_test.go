package todoist

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

func TestFetchTasks(t *testing.T) {
	projects := []todoistProject{
		{ID: "111", Name: "Work"},
		{ID: "222", Name: "Personal"},
	}

	tasks := []todoistTask{
		{
			ID:        "1001",
			Content:   "Fix bug",
			Priority:  4,
			ProjectID: "111",
			Labels:    []string{"Bug"},
			AddedAt: "2025-01-15T10:00:00Z",
			Due:       &todoistDue{Date: "2025-02-01"},
			Duration:  &todoistDuration{Amount: 120, Unit: "minute"},
		},
		{
			ID:        "1002",
			Content:   "Write tests",
			Priority:  1,
			ProjectID: "222",
			Labels:    []string{"Task"},
			AddedAt: "2025-01-18T14:00:00Z",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/projects":
			json.NewEncoder(w).Encode(paginatedResults[todoistProject]{Results: projects})
		case "/tasks":
			json.NewEncoder(w).Encode(paginatedResults[todoistTask]{Results: tasks})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	result, err := client.FetchTasks(context.Background(), "")
	if err != nil {
		t.Fatalf("FetchTasks: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(result))
	}

	// Task 1: high priority (API 4 → fylla 1), with native due date and duration
	if result[0].Key != "1001" {
		t.Errorf("task[0].Key = %q, want 1001", result[0].Key)
	}
	if result[0].Summary != "Fix bug" {
		t.Errorf("task[0].Summary = %q, want 'Fix bug'", result[0].Summary)
	}
	if result[0].Priority != 1 {
		t.Errorf("task[0].Priority = %d, want 1 (API 4 → fylla 1)", result[0].Priority)
	}
	if result[0].Project != "Work" {
		t.Errorf("task[0].Project = %q, want Work", result[0].Project)
	}
	if result[0].IssueType != "Bug" {
		t.Errorf("task[0].IssueType = %q, want Bug", result[0].IssueType)
	}
	if result[0].DueDate == nil {
		t.Fatal("task[0].DueDate is nil")
	}
	expectedDue := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	if !result[0].DueDate.Equal(expectedDue) {
		t.Errorf("task[0].DueDate = %v, want %v", result[0].DueDate, expectedDue)
	}
	if result[0].OriginalEstimate != 2*time.Hour {
		t.Errorf("task[0].OriginalEstimate = %v, want 2h", result[0].OriginalEstimate)
	}

	// Task 2: normal priority (API 1 → fylla 4), no due date/duration
	if result[1].Priority != 4 {
		t.Errorf("task[1].Priority = %d, want 4 (API 1 → fylla 4)", result[1].Priority)
	}
	if result[1].Project != "Personal" {
		t.Errorf("task[1].Project = %q, want Personal", result[1].Project)
	}
}

func TestFetchTasks_BracketAndNativeDue(t *testing.T) {
	tasks := []todoistTask{
		{
			ID:        "2001",
			Content:   "Deploy app [2h] {2025-03-15}",
			Priority:  2,
			ProjectID: "111",
			AddedAt:   "2025-01-20T09:00:00Z",
			Due:       &todoistDue{Date: "2025-03-20"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			json.NewEncoder(w).Encode(paginatedResults[todoistProject]{Results: []todoistProject{{ID: "111", Name: "Ops"}}})
		case "/tasks":
			json.NewEncoder(w).Encode(paginatedResults[todoistTask]{Results: tasks})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	result, err := client.FetchTasks(context.Background(), "")
	if err != nil {
		t.Fatalf("FetchTasks: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result))
	}

	if result[0].Summary != "Deploy app" {
		t.Errorf("Summary = %q, want 'Deploy app' (brackets stripped)", result[0].Summary)
	}
	if result[0].OriginalEstimate != 2*time.Hour {
		t.Errorf("OriginalEstimate = %v, want 2h (from title brackets)", result[0].OriginalEstimate)
	}
	// Due date comes from native field, not curly braces
	if result[0].DueDate == nil {
		t.Fatal("DueDate is nil")
	}
	expectedDue := time.Date(2025, 3, 20, 0, 0, 0, 0, time.UTC)
	if !result[0].DueDate.Equal(expectedDue) {
		t.Errorf("DueDate = %v, want %v (from native due field)", result[0].DueDate, expectedDue)
	}
}

func TestCreateTask(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			json.NewEncoder(w).Encode(paginatedResults[todoistProject]{Results: []todoistProject{{ID: "111", Name: "Work"}}})
		case "/tasks":
			if r.Method == http.MethodPost {
				json.NewDecoder(r.Body).Decode(&received)
				json.NewEncoder(w).Encode(todoistTask{ID: "9999"})
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	id, err := client.CreateTask(context.Background(), task.CreateInput{
		Summary:   "New task",
		Priority:  "High",
		Estimate:  90 * time.Minute,
		Project:   "Work",
		IssueType: "Bug",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if id != "9999" {
		t.Errorf("id = %q, want 9999", id)
	}
	if received["content"] != "New task [1h30m]" {
		t.Errorf("content = %v, want 'New task [1h30m]'", received["content"])
	}
	// Priority "High" → API 3
	if pri, ok := received["priority"].(float64); !ok || int(pri) != 3 {
		t.Errorf("priority = %v, want 3", received["priority"])
	}
	if labels, ok := received["labels"].([]interface{}); !ok || len(labels) != 1 || labels[0] != "Bug" {
		t.Errorf("labels = %v, want [Bug]", received["labels"])
	}
}

func TestPostWorklog(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/comments" && r.Method == http.MethodPost {
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	err := client.PostWorklog(context.Background(), "1001", 1*time.Hour+30*time.Minute, "Fixed the bug")
	if err != nil {
		t.Fatalf("PostWorklog: %v", err)
	}

	if received["task_id"] != "1001" {
		t.Errorf("task_id = %v, want 1001", received["task_id"])
	}
	content, _ := received["content"].(string)
	if content != "[Worklog] 1h 30m — Fixed the bug" {
		t.Errorf("content = %q, want '[Worklog] 1h 30m — Fixed the bug'", content)
	}
}

func TestGetEstimate(t *testing.T) {
	t.Run("from title brackets", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(todoistTask{
				ID:      "1001",
				Content: "Task [1h30m]",
			})
		}))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		est, err := client.GetEstimate(context.Background(), "1001")
		if err != nil {
			t.Fatalf("GetEstimate: %v", err)
		}
		if est != 90*time.Minute {
			t.Errorf("estimate = %v, want 1h30m", est)
		}
	})

	t.Run("no estimate returns zero", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(todoistTask{
				ID:      "1002",
				Content: "Task without estimate",
			})
		}))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		est, err := client.GetEstimate(context.Background(), "1002")
		if err != nil {
			t.Fatalf("GetEstimate: %v", err)
		}
		if est != 0 {
			t.Errorf("estimate = %v, want 0", est)
		}
	})
}

func TestUpdateEstimate(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(todoistTask{
				ID:      "1001",
				Content: "Fix bug [30m]",
			})
			return
		}
		if r.Method == http.MethodPost {
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	err := client.UpdateEstimate(context.Background(), "1001", 45*time.Minute)
	if err != nil {
		t.Fatalf("UpdateEstimate: %v", err)
	}

	if received["content"] != "Fix bug [45m]" {
		t.Errorf("content = %v, want 'Fix bug [45m]'", received["content"])
	}
}

func TestFetchTasks_Filter(t *testing.T) {
	projects := []todoistProject{{ID: "111", Name: "Work"}}
	tasks := []todoistTask{
		{ID: "3001", Content: "Filtered task", Priority: 2, ProjectID: "111", AddedAt: "2025-01-20T09:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			json.NewEncoder(w).Encode(paginatedResults[todoistProject]{Results: projects})
		case "/tasks/filter":
			if r.URL.Query().Get("query") != "today" {
				t.Errorf("filter query = %q, want 'today'", r.URL.Query().Get("query"))
			}
			json.NewEncoder(w).Encode(paginatedResults[todoistTask]{Results: tasks})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	result, err := client.FetchTasks(context.Background(), "today")
	if err != nil {
		t.Fatalf("FetchTasks with filter: %v", err)
	}
	if len(result) != 1 || result[0].Key != "3001" {
		t.Errorf("expected 1 filtered task with key 3001, got %v", result)
	}
}

func TestAPIPriorityToLevel(t *testing.T) {
	tests := []struct {
		api  int
		want int
	}{
		{4, 1}, // urgent → Highest
		{3, 2}, // high → High
		{2, 3}, // medium → Medium
		{1, 4}, // normal → Low
	}
	for _, tt := range tests {
		got := apiPriorityToLevel(tt.api)
		if got != tt.want {
			t.Errorf("apiPriorityToLevel(%d) = %d, want %d", tt.api, got, tt.want)
		}
	}
}
