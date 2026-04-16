package todoist

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

// todoistHandler builds a reusable httptest handler for Todoist API tests.
type todoistHandlerConfig struct {
	projects []todoistProject
	sections []todoistSection
	tasks    []todoistTask
	onPost   func(http.ResponseWriter, *http.Request)
}

func todoistHandler(t *testing.T, opts ...func(*todoistHandlerConfig)) http.HandlerFunc {
	t.Helper()
	cfg := &todoistHandlerConfig{
		projects: []todoistProject{
			{ID: "111", Name: "Work"},
			{ID: "222", Name: "Personal"},
		},
		sections: []todoistSection{
			{ID: "s1", ProjectID: "111", Name: "Sprint 1"},
			{ID: "s2", ProjectID: "111", Name: "Sprint 2"},
		},
		tasks: []todoistTask{},
	}
	for _, o := range opts {
		o(cfg)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case r.URL.Path == "/projects" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(paginatedResults[todoistProject]{Results: cfg.projects})
		case r.URL.Path == "/sections" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(paginatedResults[todoistSection]{Results: cfg.sections})
		case r.URL.Path == "/tasks" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(paginatedResults[todoistTask]{Results: cfg.tasks})
		case r.URL.Path == "/tasks/filter" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(paginatedResults[todoistTask]{Results: cfg.tasks})
		case strings.HasPrefix(r.URL.Path, "/tasks/") && r.Method == http.MethodGet:
			// Single task fetch
			taskID := strings.TrimPrefix(r.URL.Path, "/tasks/")
			for _, tk := range cfg.tasks {
				if tk.ID == taskID {
					json.NewEncoder(w).Encode(tk)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/tasks" && r.Method == http.MethodPost:
			if cfg.onPost != nil {
				cfg.onPost(w, r)
				return
			}
			json.NewEncoder(w).Encode(todoistTask{ID: "9999"})
		case strings.HasSuffix(r.URL.Path, "/close") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/move") && r.Method == http.MethodPost:
			if cfg.onPost != nil {
				cfg.onPost(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/tasks/") && r.Method == http.MethodPost:
			if cfg.onPost != nil {
				cfg.onPost(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/tasks/") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/comments" && r.Method == http.MethodPost:
			if cfg.onPost != nil {
				cfg.onPost(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}
}

func TestDeleteTask(t *testing.T) {
	t.Run("sends DELETE request", func(t *testing.T) {
		var gotMethod string
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		err := client.DeleteTask(context.Background(), "1001")
		if err != nil {
			t.Fatalf("DeleteTask: %v", err)
		}
		if gotMethod != http.MethodDelete {
			t.Errorf("Method = %s, want DELETE", gotMethod)
		}
		if gotPath != "/tasks/1001" {
			t.Errorf("Path = %q, want /tasks/1001", gotPath)
		}
	})

	t.Run("error on non-204 response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		err := client.DeleteTask(context.Background(), "1001")
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
	})
}

func TestGetDueDate(t *testing.T) {
	t.Run("returns native due date", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(todoistTask{
				ID:      "1001",
				Content: "Task",
				Due:     &todoistDue{Date: "2025-06-15"},
			})
		}))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		due, err := client.GetDueDate(context.Background(), "1001")
		if err != nil {
			t.Fatalf("GetDueDate: %v", err)
		}
		if due == nil {
			t.Fatal("expected non-nil due date")
		}
		expected := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
		if !due.Equal(expected) {
			t.Errorf("due = %v, want %v", due, expected)
		}
	})

	t.Run("returns nil when no due date", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(todoistTask{
				ID:      "1001",
				Content: "Task without due",
			})
		}))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		due, err := client.GetDueDate(context.Background(), "1001")
		if err != nil {
			t.Fatalf("GetDueDate: %v", err)
		}
		if due != nil {
			t.Errorf("expected nil due date, got %v", due)
		}
	})

	t.Run("error on fetch failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		}))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		_, err := client.GetDueDate(context.Background(), "9999")
		if err == nil {
			t.Fatal("expected error for 404 response")
		}
	})
}

func TestUpdateDueDate(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	dueDate := time.Date(2025, 8, 20, 0, 0, 0, 0, time.UTC)
	err := client.UpdateDueDate(context.Background(), "1001", dueDate)
	if err != nil {
		t.Fatalf("UpdateDueDate: %v", err)
	}
	if received["due_date"] != "2025-08-20" {
		t.Errorf("due_date = %v, want '2025-08-20'", received["due_date"])
	}
}

func TestUpdateDueDate_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	err := client.UpdateDueDate(context.Background(), "1001", time.Now())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestRemoveDueDate(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	err := client.RemoveDueDate(context.Background(), "1001")
	if err != nil {
		t.Fatalf("RemoveDueDate: %v", err)
	}
	if received["due_string"] != "no date" {
		t.Errorf("due_string = %v, want 'no date'", received["due_string"])
	}
}

func TestRemoveDueDate_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	err := client.RemoveDueDate(context.Background(), "1001")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestGetPriority(t *testing.T) {
	tests := []struct {
		name     string
		apiPri   int
		wantPri  int
	}{
		{"urgent", 4, 1},
		{"high", 3, 2},
		{"medium", 2, 3},
		{"normal", 1, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(todoistTask{
					ID:       "1001",
					Content:  "Task",
					Priority: tt.apiPri,
				})
			}))
			defer srv.Close()

			client := NewClient("test-token")
			client.BaseURL = srv.URL

			pri, err := client.GetPriority(context.Background(), "1001")
			if err != nil {
				t.Fatalf("GetPriority: %v", err)
			}
			if pri != tt.wantPri {
				t.Errorf("priority = %d, want %d", pri, tt.wantPri)
			}
		})
	}
}

func TestGetPriority_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	_, err := client.GetPriority(context.Background(), "9999")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestUpdatePriority(t *testing.T) {
	tests := []struct {
		name      string
		fyllaPri  int
		wantAPI   int
	}{
		{"highest to urgent", 1, 4},
		{"high to high", 2, 3},
		{"medium to medium", 3, 2},
		{"low to normal", 4, 1},
		{"lowest to normal", 5, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var received map[string]interface{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&received)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			client := NewClient("test-token")
			client.BaseURL = srv.URL

			err := client.UpdatePriority(context.Background(), "1001", tt.fyllaPri)
			if err != nil {
				t.Fatalf("UpdatePriority: %v", err)
			}
			if int(received["priority"].(float64)) != tt.wantAPI {
				t.Errorf("priority = %v, want %d", received["priority"], tt.wantAPI)
			}
		})
	}
}

func TestUpdatePriority_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	err := client.UpdatePriority(context.Background(), "1001", 2)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestGetSummary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(todoistTask{
			ID:      "1001",
			Content: "My task title",
		})
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	summary, err := client.GetSummary(context.Background(), "1001")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if summary != "My task title" {
		t.Errorf("summary = %q, want 'My task title'", summary)
	}
}

func TestGetSummary_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	_, err := client.GetSummary(context.Background(), "9999")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestUpdateSummary(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	err := client.UpdateSummary(context.Background(), "1001", "Updated title")
	if err != nil {
		t.Fatalf("UpdateSummary: %v", err)
	}
	if received["content"] != "Updated title" {
		t.Errorf("content = %v, want 'Updated title'", received["content"])
	}
}

func TestUpdateSummary_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	err := client.UpdateSummary(context.Background(), "1001", "Updated title")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestUpdateEstimate_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(todoistTask{ID: "1001", Content: "Task [30m]"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	err := client.UpdateEstimate(context.Background(), "1001", 45*time.Minute)
	if err == nil {
		t.Fatal("expected error for 500 response on update")
	}
}

func TestGetEstimate_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	_, err := client.GetEstimate(context.Background(), "9999")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFetchTasks_Error(t *testing.T) {
	t.Run("projects error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("fail"))
		}))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		_, err := client.FetchTasks(context.Background(), "")
		if err == nil {
			t.Fatal("expected error when projects endpoint fails")
		}
	})

	t.Run("tasks error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/projects":
				json.NewEncoder(w).Encode(paginatedResults[todoistProject]{Results: nil})
			case "/sections":
				json.NewEncoder(w).Encode(paginatedResults[todoistSection]{Results: nil})
			default:
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("fail"))
			}
		}))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		_, err := client.FetchTasks(context.Background(), "")
		if err == nil {
			t.Fatal("expected error when tasks endpoint fails")
		}
	})
}

func TestCreateTask_WithDueDate(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(todoistHandler(t, func(cfg *todoistHandlerConfig) {
		cfg.onPost = func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&received)
			json.NewEncoder(w).Encode(todoistTask{ID: "9999"})
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	dueDate := time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)
	id, err := client.CreateTask(context.Background(), task.CreateInput{
		Summary: "Task with due",
		DueDate: &dueDate,
		Project: "Work",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if id != "9999" {
		t.Errorf("id = %q, want 9999", id)
	}
	if received["due_date"] != "2025-06-30" {
		t.Errorf("due_date = %v, want '2025-06-30'", received["due_date"])
	}
}

func TestCreateTask_WithDescription(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(todoistHandler(t, func(cfg *todoistHandlerConfig) {
		cfg.onPost = func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&received)
			json.NewEncoder(w).Encode(todoistTask{ID: "9999"})
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	_, err := client.CreateTask(context.Background(), task.CreateInput{
		Summary:     "Task with desc",
		Description: "Detailed description",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if received["description"] != "Detailed description" {
		t.Errorf("description = %v, want 'Detailed description'", received["description"])
	}
}

func TestCreateTask_WithSection(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(todoistHandler(t, func(cfg *todoistHandlerConfig) {
		cfg.onPost = func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&received)
			json.NewEncoder(w).Encode(todoistTask{ID: "9999"})
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	_, err := client.CreateTask(context.Background(), task.CreateInput{
		Summary: "Task with section",
		Section: "Sprint 1",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if received["section_id"] != "s1" {
		t.Errorf("section_id = %v, want 's1'", received["section_id"])
	}
}

func TestCreateTask_Error(t *testing.T) {
	srv := httptest.NewServer(todoistHandler(t, func(cfg *todoistHandlerConfig) {
		cfg.onPost = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request"))
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	_, err := client.CreateTask(context.Background(), task.CreateInput{
		Summary: "Will fail",
	})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestPostWorklog_TimeFormats(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		wantTime string
	}{
		{"hours and minutes", 1*time.Hour + 30*time.Minute, "1h 30m"},
		{"hours only", 2 * time.Hour, "2h"},
		{"minutes only", 45 * time.Minute, "45m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var received map[string]interface{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&received)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			client := NewClient("test-token")
			client.BaseURL = srv.URL

			err := client.PostWorklog(context.Background(), "1001", tt.duration, "work", time.Now())
			if err != nil {
				t.Fatalf("PostWorklog: %v", err)
			}
			content := received["content"].(string)
			if !strings.Contains(content, tt.wantTime) {
				t.Errorf("content = %q, want to contain %q", content, tt.wantTime)
			}
		})
	}
}

func TestPostWorklog_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	err := client.PostWorklog(context.Background(), "1001", 30*time.Minute, "work", time.Now())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestListProjects(t *testing.T) {
	srv := httptest.NewServer(todoistHandler(t))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	names, err := client.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(names))
	}
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["Work"] || !found["Personal"] {
		t.Errorf("unexpected projects: %v", names)
	}
}

func TestListProjects_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	_, err := client.ListProjects(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestListSections(t *testing.T) {
	t.Run("all sections", func(t *testing.T) {
		srv := httptest.NewServer(todoistHandler(t))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		sections, err := client.ListSections(context.Background(), "")
		if err != nil {
			t.Fatalf("ListSections: %v", err)
		}
		if len(sections) != 2 {
			t.Fatalf("expected 2 sections, got %d", len(sections))
		}
	})

	t.Run("filtered by project", func(t *testing.T) {
		srv := httptest.NewServer(todoistHandler(t))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		sections, err := client.ListSections(context.Background(), "Work")
		if err != nil {
			t.Fatalf("ListSections: %v", err)
		}
		if len(sections) != 2 {
			t.Fatalf("expected 2 sections for Work, got %d", len(sections))
		}
	})

	t.Run("unknown project returns nil", func(t *testing.T) {
		srv := httptest.NewServer(todoistHandler(t))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		sections, err := client.ListSections(context.Background(), "Unknown")
		if err != nil {
			t.Fatalf("ListSections: %v", err)
		}
		if sections != nil {
			t.Errorf("expected nil sections for unknown project, got %v", sections)
		}
	})
}

func TestListSections_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	_, err := client.ListSections(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestLevelToAPIPriority(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{1, 4},
		{2, 3},
		{3, 2},
		{4, 1},
		{5, 1},
		{0, 1},
	}
	for _, tt := range tests {
		got := levelToAPIPriority(tt.level)
		if got != tt.want {
			t.Errorf("levelToAPIPriority(%d) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestParseTask_Recurrence(t *testing.T) {
	srv := httptest.NewServer(todoistHandler(t, func(cfg *todoistHandlerConfig) {
		cfg.tasks = []todoistTask{
			{
				ID:        "1001",
				Content:   "Daily standup",
				Priority:  1,
				ProjectID: "111",
				AddedAt:   "2025-01-20T09:00:00Z",
				Due: &todoistDue{
					Date:        "2025-01-21",
					IsRecurring: true,
					String:      "every day",
				},
			},
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	tasks, err := client.FetchTasks(context.Background(), "")
	if err != nil {
		t.Fatalf("FetchTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Recurrence == nil {
		t.Error("expected non-nil Recurrence for recurring task")
	}
}

func TestParseTask_DayDuration(t *testing.T) {
	srv := httptest.NewServer(todoistHandler(t, func(cfg *todoistHandlerConfig) {
		cfg.tasks = []todoistTask{
			{
				ID:        "1001",
				Content:   "Multi-day task",
				Priority:  1,
				ProjectID: "111",
				AddedAt:   "2025-01-20T09:00:00Z",
				Duration:  &todoistDuration{Amount: 2, Unit: "day"},
			},
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	tasks, err := client.FetchTasks(context.Background(), "")
	if err != nil {
		t.Fatalf("FetchTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	// 2 days * 8 hours = 16 hours
	if tasks[0].OriginalEstimate != 16*time.Hour {
		t.Errorf("OriginalEstimate = %v, want 16h", tasks[0].OriginalEstimate)
	}
}

func TestParseTask_TitleEstimateTakesPrecedenceOverDuration(t *testing.T) {
	srv := httptest.NewServer(todoistHandler(t, func(cfg *todoistHandlerConfig) {
		cfg.tasks = []todoistTask{
			{
				ID:        "1001",
				Content:   "Task [2h]",
				Priority:  1,
				ProjectID: "111",
				AddedAt:   "2025-01-20T09:00:00Z",
				Duration:  &todoistDuration{Amount: 120, Unit: "minute"},
			},
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	tasks, err := client.FetchTasks(context.Background(), "")
	if err != nil {
		t.Fatalf("FetchTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	// Title bracket estimate should take precedence
	if tasks[0].OriginalEstimate != 2*time.Hour {
		t.Errorf("OriginalEstimate = %v, want 2h (from title brackets)", tasks[0].OriginalEstimate)
	}
	if tasks[0].Summary != "Task" {
		t.Errorf("Summary = %q, want 'Task' (brackets stripped)", tasks[0].Summary)
	}
}

func TestParseTask_Provider(t *testing.T) {
	srv := httptest.NewServer(todoistHandler(t, func(cfg *todoistHandlerConfig) {
		cfg.tasks = []todoistTask{
			{
				ID:        "1001",
				Content:   "Task",
				Priority:  1,
				ProjectID: "111",
				AddedAt:   "2025-01-20T09:00:00Z",
			},
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	tasks, err := client.FetchTasks(context.Background(), "")
	if err != nil {
		t.Fatalf("FetchTasks: %v", err)
	}
	if tasks[0].Provider != "todoist" {
		t.Errorf("Provider = %q, want 'todoist'", tasks[0].Provider)
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("my-token")
	if client.BaseURL != defaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", client.BaseURL, defaultBaseURL)
	}
	if client.Token != "my-token" {
		t.Errorf("Token = %q, want 'my-token'", client.Token)
	}
	if client.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
}

func TestUpdateSection(t *testing.T) {
	t.Run("moves to section", func(t *testing.T) {
		var received map[string]interface{}
		var gotPath string
		srv := httptest.NewServer(todoistHandler(t, func(cfg *todoistHandlerConfig) {
			cfg.onPost = func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				json.NewDecoder(r.Body).Decode(&received)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		err := client.UpdateSection(context.Background(), "1001", "Sprint 1")
		if err != nil {
			t.Fatalf("UpdateSection: %v", err)
		}
		if !strings.HasSuffix(gotPath, "/move") {
			t.Errorf("path = %q, want to end with /move", gotPath)
		}
		if received["section_id"] != "s1" {
			t.Errorf("section_id = %v, want 's1'", received["section_id"])
		}
	})

	t.Run("error for unknown section", func(t *testing.T) {
		srv := httptest.NewServer(todoistHandler(t))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		err := client.UpdateSection(context.Background(), "1001", "NonexistentSection")
		if err == nil {
			t.Fatal("expected error for unknown section")
		}
	})

	t.Run("removes section by moving to project", func(t *testing.T) {
		var received map[string]interface{}
		srv := httptest.NewServer(todoistHandler(t, func(cfg *todoistHandlerConfig) {
			cfg.tasks = []todoistTask{
				{ID: "1001", Content: "Task", ProjectID: "111"},
			}
			cfg.onPost = func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&received)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer srv.Close()

		client := NewClient("test-token")
		client.BaseURL = srv.URL

		err := client.UpdateSection(context.Background(), "1001", "")
		if err != nil {
			t.Fatalf("UpdateSection: %v", err)
		}
		if received["project_id"] != "111" {
			t.Errorf("project_id = %v, want '111'", received["project_id"])
		}
	})
}

func TestUpdateSection_Error(t *testing.T) {
	srv := httptest.NewServer(todoistHandler(t, func(cfg *todoistHandlerConfig) {
		cfg.onPost = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("fail"))
		}
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.BaseURL = srv.URL

	err := client.UpdateSection(context.Background(), "1001", "Sprint 1")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestParseTodoistRecurrence(t *testing.T) {
	tests := []struct {
		input string
		isNil bool
	}{
		{"every day", false},
		{"every! weekday", false},
		{"every mon, wed", false},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseTodoistRecurrence(tt.input)
			if tt.isNil && result != nil {
				t.Errorf("expected nil for %q, got %v", tt.input, result)
			}
			if !tt.isNil && result == nil {
				t.Errorf("expected non-nil for %q", tt.input)
			}
		})
	}
}

func TestSectionName(t *testing.T) {
	client := &Client{
		sections: map[string]string{
			"s1": "Sprint 1",
			"s2": "Sprint 2",
		},
	}

	if name := client.sectionName("s1"); name != "Sprint 1" {
		t.Errorf("sectionName(s1) = %q, want 'Sprint 1'", name)
	}
	if name := client.sectionName("unknown"); name != "" {
		t.Errorf("sectionName(unknown) = %q, want empty", name)
	}
	if name := client.sectionName(""); name != "" {
		t.Errorf("sectionName(empty) = %q, want empty", name)
	}
}

func TestSectionName_NilMap(t *testing.T) {
	client := &Client{}
	if name := client.sectionName("s1"); name != "" {
		t.Errorf("sectionName with nil map = %q, want empty", name)
	}
}

func TestProjectName(t *testing.T) {
	client := &Client{
		projects: map[string]string{
			"111": "Work",
		},
	}

	if name := client.projectName("111"); name != "Work" {
		t.Errorf("projectName(111) = %q, want 'Work'", name)
	}
	if name := client.projectName("999"); name != "999" {
		t.Errorf("projectName(999) = %q, want '999' (fallback to id)", name)
	}
}

func TestProjectName_NilMap(t *testing.T) {
	client := &Client{}
	if name := client.projectName("111"); name != "111" {
		t.Errorf("projectName with nil map = %q, want '111'", name)
	}
}

func TestPriorityNameToAPI(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"Highest", 4},
		{"High", 3},
		{"Medium", 2},
		{"Low", 1},
		{"Lowest", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := priorityNameToAPI[tt.name]
			if !ok {
				t.Fatalf("priority name %q not found in map", tt.name)
			}
			if got != tt.want {
				t.Errorf("priorityNameToAPI[%q] = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestFetchTasks_AuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/projects":
			json.NewEncoder(w).Encode(paginatedResults[todoistProject]{Results: nil})
		case "/sections":
			json.NewEncoder(w).Encode(paginatedResults[todoistSection]{Results: nil})
		case "/tasks":
			json.NewEncoder(w).Encode(paginatedResults[todoistTask]{Results: nil})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient("my-secret-token")
	client.BaseURL = srv.URL

	_, err := client.FetchTasks(context.Background(), "")
	if err != nil {
		t.Fatalf("FetchTasks: %v", err)
	}
	if gotAuth != "Bearer my-secret-token" {
		t.Errorf("Authorization = %q, want 'Bearer my-secret-token'", gotAuth)
	}
}
