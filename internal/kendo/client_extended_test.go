package kendo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

func intPtr(v int) *int { return &v }

// kendoHandlerConfig configures a reusable httptest handler for Kendo API tests.
type kendoHandlerConfig struct {
	projects           []project
	lanes              []laneJSON
	epics              []epicJSON
	sprints            []sprintJSON
	issues             map[string]issueJSON
	userID             int
	onCreateIssue      func(http.ResponseWriter, *http.Request)
	onPutIssue         func(http.ResponseWriter, *http.Request)
	onDeleteIssue      func(http.ResponseWriter, *http.Request)
	onTimeEntry        func(http.ResponseWriter, *http.Request)
	onFetchTimeEntries func(http.ResponseWriter, *http.Request)
}

func kendoHandler(t *testing.T, opts ...func(*kendoHandlerConfig)) http.HandlerFunc {
	t.Helper()
	cfg := &kendoHandlerConfig{
		projects: []project{
			{ID: 1, Name: "Iruoy", Code: "IRUOY"},
			{ID: 2, Name: "Admin", Code: "ADMIN"},
		},
		lanes: []laneJSON{
			{ID: 10, Title: "To Do"},
			{ID: 11, Title: "In Progress"},
			{ID: 12, Title: "Done"},
		},
		epics: []epicJSON{
			{ID: 100, Title: "Epic One"},
			{ID: 101, Title: "Epic Two"},
		},
		sprints: []sprintJSON{
			{ID: 50, Title: "Sprint 1", Status: 1, ProjectID: 1},
			{ID: 51, Title: "Sprint 2", Status: 0, ProjectID: 1},
			{ID: 52, Title: "Sprint 3", Status: 2, ProjectID: 1},
		},
		issues: map[string]issueJSON{
			"IRUOY-0001": {
				ID: 1, Key: "IRUOY-0001", Title: "Fix login bug", Priority: 1,
				LaneID: 10, ProjectID: 1, CreatedAt: "2025-01-20T09:00:00Z",
				AssigneeID: intPtr(42),
			},
		},
		userID: 42,
	}
	for _, o := range opts {
		o(cfg)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/projects":
			json.NewEncoder(w).Encode(cfg.projects)
		case r.URL.Path == "/api/auth/user":
			json.NewEncoder(w).Encode(map[string]interface{}{"id": cfg.userID})
		case strings.HasSuffix(r.URL.Path, "/lanes"):
			json.NewEncoder(w).Encode(cfg.lanes)
		case strings.HasSuffix(r.URL.Path, "/epics"):
			json.NewEncoder(w).Encode(cfg.epics)
		case strings.HasSuffix(r.URL.Path, "/sprints"):
			json.NewEncoder(w).Encode(cfg.sprints)
		case strings.HasSuffix(r.URL.Path, "/issues") && r.Method == http.MethodGet:
			// Extract project ID from URL /api/projects/{id}/issues
			var pid int
			parts := strings.Split(r.URL.Path, "/")
			for i, p := range parts {
				if p == "projects" && i+1 < len(parts) {
					pid, _ = strconv.Atoi(parts[i+1])
				}
			}
			issues := make([]issueJSON, 0, len(cfg.issues))
			for _, iss := range cfg.issues {
				if pid == 0 || iss.ProjectID == pid {
					issues = append(issues, iss)
				}
			}
			json.NewEncoder(w).Encode(issues)
		case strings.HasSuffix(r.URL.Path, "/issues") && r.Method == http.MethodPost:
			if cfg.onCreateIssue != nil {
				cfg.onCreateIssue(w, r)
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(issueJSON{Key: "IRUOY-0099"})
		case strings.Contains(r.URL.Path, "/issues/") && strings.Contains(r.URL.Path, "/time-entries"):
			if cfg.onTimeEntry != nil {
				cfg.onTimeEntry(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "/issues/") && r.Method == http.MethodGet:
			parts := strings.Split(r.URL.Path, "/")
			key := parts[len(parts)-1]
			if iss, ok := cfg.issues[key]; ok {
				json.NewEncoder(w).Encode(iss)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case strings.Contains(r.URL.Path, "/issues/") && r.Method == http.MethodPut:
			if cfg.onPutIssue != nil {
				cfg.onPutIssue(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "/issues/") && r.Method == http.MethodDelete:
			if cfg.onDeleteIssue != nil {
				cfg.onDeleteIssue(w, r)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/time-entries":
			if cfg.onFetchTimeEntries != nil {
				cfg.onFetchTimeEntries(w, r)
				return
			}
			json.NewEncoder(w).Encode([]timeEntryJSON{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestFetchTasks_WildcardQuery(t *testing.T) {
	est := 60
	assignee := 42
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.issues = map[string]issueJSON{
			"IRUOY-0001": {ID: 1, Key: "IRUOY-0001", Title: "Task A", Priority: 0, LaneID: 10, ProjectID: 1, EstimatedMinutes: &est, AssigneeID: &assignee},
			"ADMIN-0001": {ID: 2, Key: "ADMIN-0001", Title: "Task B", Priority: 1, LaneID: 10, ProjectID: 2, EstimatedMinutes: &est, AssigneeID: &assignee},
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	tasks, err := client.FetchTasks(context.Background(), "*")
	if err != nil {
		t.Fatalf("FetchTasks(*): %v", err)
	}
	if len(tasks) < 1 {
		t.Fatalf("expected at least 1 task from wildcard query, got %d", len(tasks))
	}
}

func TestFetchTasks_EmptyQuery(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	tasks, err := client.FetchTasks(context.Background(), "")
	if err != nil {
		t.Fatalf("FetchTasks(empty): %v", err)
	}
	if tasks == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestFetchTasks_FilterQuery(t *testing.T) {
	assignee := 42
	est := 60
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.issues = map[string]issueJSON{
			"IRUOY-0001": {ID: 1, Key: "IRUOY-0001", Title: "My task", Priority: 0, LaneID: 10, ProjectID: 1, AssigneeID: &assignee, EstimatedMinutes: &est},
			"IRUOY-0002": {ID: 2, Key: "IRUOY-0002", Title: "Other task", Priority: 1, LaneID: 10, ProjectID: 1, EstimatedMinutes: &est},
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	tasks, err := client.FetchTasks(context.Background(), "assignee_id=me")
	if err != nil {
		t.Fatalf("FetchTasks(filter): %v", err)
	}
	for _, tk := range tasks {
		if tk.Key == "IRUOY-0002" {
			t.Errorf("task IRUOY-0002 should have been filtered out by assignee_id=me")
		}
	}
}

func TestFetchTasks_TextSearch(t *testing.T) {
	est := 60
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.issues = map[string]issueJSON{
			"IRUOY-0001": {ID: 1, Key: "IRUOY-0001", Title: "Fix login bug", Priority: 0, LaneID: 10, ProjectID: 1, EstimatedMinutes: &est},
			"IRUOY-0002": {ID: 2, Key: "IRUOY-0002", Title: "Add feature", Priority: 1, LaneID: 10, ProjectID: 1, EstimatedMinutes: &est},
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	tasks, err := client.FetchTasks(context.Background(), "login")
	if err != nil {
		t.Fatalf("FetchTasks(text search): %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task matching 'login', got %d", len(tasks))
	}
	if tasks[0].Key != "IRUOY-0001" {
		t.Errorf("Key = %q, want IRUOY-0001", tasks[0].Key)
	}
}

func TestFetchTasks_ProjectLoadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.FetchTasks(context.Background(), "")
	if err == nil {
		t.Fatal("expected error when projects endpoint fails")
	}
}

func TestCreateTask(t *testing.T) {
	t.Run("basic creation", func(t *testing.T) {
		var gotPayload map[string]interface{}
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onCreateIssue = func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&gotPayload)
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(issueJSON{Key: "IRUOY-0042"})
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		key, err := client.CreateTask(context.Background(), task.CreateInput{
			Project:   "IRUOY",
			Lane:      "To Do",
			Summary:   "New feature",
			IssueType: "Feature",
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if key != "IRUOY-0042" {
			t.Errorf("key = %q, want IRUOY-0042", key)
		}
		if gotPayload["title"] != "New feature" {
			t.Errorf("title = %v, want 'New feature'", gotPayload["title"])
		}
	})

	t.Run("with bug type", func(t *testing.T) {
		var gotPayload map[string]interface{}
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onCreateIssue = func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&gotPayload)
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(issueJSON{Key: "IRUOY-0043"})
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		_, err := client.CreateTask(context.Background(), task.CreateInput{
			Project:   "IRUOY",
			Lane:      "To Do",
			Summary:   "Fix crash",
			IssueType: "Bug",
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if int(gotPayload["type"].(float64)) != 1 {
			t.Errorf("type = %v, want 1 (Bug)", gotPayload["type"])
		}
	})

	t.Run("with task type", func(t *testing.T) {
		var gotPayload map[string]interface{}
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onCreateIssue = func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&gotPayload)
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(issueJSON{Key: "IRUOY-0044"})
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		_, err := client.CreateTask(context.Background(), task.CreateInput{
			Project:   "IRUOY",
			Lane:      "To Do",
			Summary:   "Routine work",
			IssueType: "Task",
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if int(gotPayload["type"].(float64)) != 2 {
			t.Errorf("type = %v, want 2 (Task)", gotPayload["type"])
		}
	})

	t.Run("with estimate and priority", func(t *testing.T) {
		var gotPayload map[string]interface{}
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onCreateIssue = func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&gotPayload)
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(issueJSON{Key: "IRUOY-0045"})
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		_, err := client.CreateTask(context.Background(), task.CreateInput{
			Project:  "IRUOY",
			Lane:     "To Do",
			Summary:  "Estimated task",
			Estimate: 90 * time.Minute,
			Priority: "high",
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if int(gotPayload["estimated_minutes"].(float64)) != 90 {
			t.Errorf("estimated_minutes = %v, want 90", gotPayload["estimated_minutes"])
		}
		// "high" -> fylla level 2 -> kendo level 1
		if int(gotPayload["priority"].(float64)) != 1 {
			t.Errorf("priority = %v, want 1", gotPayload["priority"])
		}
	})

	t.Run("with sprint and parent", func(t *testing.T) {
		var gotPayload map[string]interface{}
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onCreateIssue = func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&gotPayload)
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(issueJSON{Key: "IRUOY-0046"})
			}
		}))
		defer server.Close()

		sprintID := 50
		client := NewClient(server.URL, "test-token")
		_, err := client.CreateTask(context.Background(), task.CreateInput{
			Project:  "IRUOY",
			Lane:     "To Do",
			Summary:  "Sprint task",
			SprintID: &sprintID,
			Parent:   "100",
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if int(gotPayload["sprint_id"].(float64)) != 50 {
			t.Errorf("sprint_id = %v, want 50", gotPayload["sprint_id"])
		}
		if int(gotPayload["epic_id"].(float64)) != 100 {
			t.Errorf("epic_id = %v, want 100", gotPayload["epic_id"])
		}
	})

	t.Run("error on non-201 response", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onCreateIssue = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("bad request"))
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		_, err := client.CreateTask(context.Background(), task.CreateInput{
			Project: "IRUOY",
			Lane:    "To Do",
			Summary: "Will fail",
		})
		if err == nil {
			t.Fatal("expected error for 400 response")
		}
	})

	t.Run("error on unknown project", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		_, err := client.CreateTask(context.Background(), task.CreateInput{
			Project: "NOSUCH",
			Lane:    "To Do",
			Summary: "Will fail",
		})
		if err == nil {
			t.Fatal("expected error for unknown project")
		}
	})
}

func TestDeleteTask(t *testing.T) {
	t.Run("sends DELETE request", func(t *testing.T) {
		var gotMethod string
		var gotPath string
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onDeleteIssue = func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.DeleteTask(context.Background(), "IRUOY-0001")
		if err != nil {
			t.Fatalf("DeleteTask: %v", err)
		}
		if gotMethod != http.MethodDelete {
			t.Errorf("Method = %s, want DELETE", gotMethod)
		}
		if !strings.Contains(gotPath, "IRUOY-0001") {
			t.Errorf("Path = %q, want to contain IRUOY-0001", gotPath)
		}
	})

	t.Run("error on invalid key", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.DeleteTask(context.Background(), "BADKEY")
		if err == nil {
			t.Fatal("expected error for invalid key")
		}
	})

	t.Run("error on non-200 response", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onDeleteIssue = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("server error"))
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.DeleteTask(context.Background(), "IRUOY-0001")
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
	})
}

func TestGetEstimate(t *testing.T) {
	t.Run("returns estimate from issue", func(t *testing.T) {
		est := 90
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.issues = map[string]issueJSON{
				"IRUOY-0001": {ID: 1, Key: "IRUOY-0001", Title: "Task", Priority: 0, LaneID: 10, ProjectID: 1, EstimatedMinutes: &est},
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		d, err := client.GetEstimate(context.Background(), "IRUOY-0001")
		if err != nil {
			t.Fatalf("GetEstimate: %v", err)
		}
		if d != 90*time.Minute {
			t.Errorf("estimate = %v, want 90m", d)
		}
	})

	t.Run("returns zero when no estimate", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.issues = map[string]issueJSON{
				"IRUOY-0001": {ID: 1, Key: "IRUOY-0001", Title: "Task", Priority: 0, LaneID: 10, ProjectID: 1},
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		d, err := client.GetEstimate(context.Background(), "IRUOY-0001")
		if err != nil {
			t.Fatalf("GetEstimate: %v", err)
		}
		if d != 0 {
			t.Errorf("estimate = %v, want 0", d)
		}
	})

	t.Run("error on invalid key", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		_, err := client.GetEstimate(context.Background(), "NOPE")
		if err == nil {
			t.Fatal("expected error for invalid key")
		}
	})
}

func TestUpdateEstimate(t *testing.T) {
	var gotPayload map[string]interface{}
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.onPutIssue = func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&gotPayload)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.UpdateEstimate(context.Background(), "IRUOY-0001", 2*time.Hour)
	if err != nil {
		t.Fatalf("UpdateEstimate: %v", err)
	}
	if int(gotPayload["estimated_minutes"].(float64)) != 120 {
		t.Errorf("estimated_minutes = %v, want 120", gotPayload["estimated_minutes"])
	}
}

func TestGetPriority(t *testing.T) {
	tests := []struct {
		name      string
		kendoPri  int
		wantFylla int
	}{
		{"highest", 0, 1},
		{"high", 1, 2},
		{"medium", 2, 3},
		{"low", 3, 4},
		{"trivial", 4, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
				cfg.issues = map[string]issueJSON{
					"IRUOY-0001": {ID: 1, Key: "IRUOY-0001", Title: "Task", Priority: tt.kendoPri, LaneID: 10, ProjectID: 1},
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			pri, err := client.GetPriority(context.Background(), "IRUOY-0001")
			if err != nil {
				t.Fatalf("GetPriority: %v", err)
			}
			if pri != tt.wantFylla {
				t.Errorf("priority = %d, want %d", pri, tt.wantFylla)
			}
		})
	}
}

func TestUpdatePriority(t *testing.T) {
	var gotPayload map[string]interface{}
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.onPutIssue = func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&gotPayload)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.UpdatePriority(context.Background(), "IRUOY-0001", 3) // fylla 3 -> kendo 2
	if err != nil {
		t.Fatalf("UpdatePriority: %v", err)
	}
	if int(gotPayload["priority"].(float64)) != 2 {
		t.Errorf("priority = %v, want 2", gotPayload["priority"])
	}
}

func TestGetSummary(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.issues = map[string]issueJSON{
			"IRUOY-0001": {ID: 1, Key: "IRUOY-0001", Title: "My summary", Priority: 0, LaneID: 10, ProjectID: 1},
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	summary, err := client.GetSummary(context.Background(), "IRUOY-0001")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if summary != "My summary" {
		t.Errorf("summary = %q, want 'My summary'", summary)
	}
}

func TestUpdateSummary(t *testing.T) {
	var gotPayload map[string]interface{}
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.onPutIssue = func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&gotPayload)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.UpdateSummary(context.Background(), "IRUOY-0001", "Updated title")
	if err != nil {
		t.Fatalf("UpdateSummary: %v", err)
	}
	if gotPayload["title"] != "Updated title" {
		t.Errorf("title = %v, want 'Updated title'", gotPayload["title"])
	}
}

func TestListLanes(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	lanes, err := client.ListLanes(context.Background(), "IRUOY")
	if err != nil {
		t.Fatalf("ListLanes: %v", err)
	}
	if len(lanes) != 3 {
		t.Fatalf("expected 3 lanes, got %d", len(lanes))
	}

	found := map[string]bool{}
	for _, l := range lanes {
		found[l] = true
	}
	for _, want := range []string{"To Do", "In Progress", "Done"} {
		if !found[want] {
			t.Errorf("missing lane %q", want)
		}
	}
}

func TestListLanes_UnknownProject(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.ListLanes(context.Background(), "NOSUCH")
	if err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestListEpics(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	epics, err := client.ListEpics(context.Background(), "IRUOY")
	if err != nil {
		t.Fatalf("ListEpics: %v", err)
	}
	if len(epics) != 2 {
		t.Fatalf("expected 2 epics, got %d", len(epics))
	}
	found := map[string]bool{}
	for _, e := range epics {
		found[e.Summary] = true
	}
	if !found["Epic One"] || !found["Epic Two"] {
		t.Errorf("unexpected epics: %v", epics)
	}
}

func TestListProjects(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	codes, err := client.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("expected 2 project codes, got %d", len(codes))
	}
	found := map[string]bool{}
	for _, c := range codes {
		found[c] = true
	}
	if !found["IRUOY"] || !found["ADMIN"] {
		t.Errorf("unexpected project codes: %v", codes)
	}
}

func TestListSprints(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	sprints, err := client.ListSprints(context.Background(), "IRUOY")
	if err != nil {
		t.Fatalf("ListSprints: %v", err)
	}
	// Sprint 3 is completed (status=2), should be excluded
	if len(sprints) != 2 {
		t.Fatalf("expected 2 sprints (excluding completed), got %d", len(sprints))
	}
	// Active sprint should be first
	if !sprints[0].Active {
		t.Errorf("first sprint should be active")
	}
	if sprints[1].Active {
		t.Errorf("second sprint should not be active")
	}
	if !strings.Contains(sprints[0].Label, "(Active)") {
		t.Errorf("active sprint label should contain '(Active)', got %q", sprints[0].Label)
	}
}

func TestListSprints_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects" {
			json.NewEncoder(w).Encode([]project{{ID: 1, Name: "Iruoy", Code: "IRUOY"}})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.ListSprints(context.Background(), "IRUOY")
	if err == nil {
		t.Fatal("expected error for 500 response on sprints")
	}
}

func TestListIssueTypes(t *testing.T) {
	client := NewClient("http://unused", "unused")
	types, err := client.ListIssueTypes(context.Background(), "any")
	if err != nil {
		t.Fatalf("ListIssueTypes: %v", err)
	}
	if len(types) != 3 {
		t.Fatalf("expected 3 issue types, got %d", len(types))
	}
	expected := map[string]bool{"Feature": true, "Bug": true, "Task": true}
	for _, tp := range types {
		if !expected[tp] {
			t.Errorf("unexpected issue type: %q", tp)
		}
	}
}

func TestPostWorklog_Error(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.onTimeEntry = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("fail"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.PostWorklog(context.Background(), "IRUOY-0001", 30*time.Minute, "work", time.Now())
	if err == nil {
		t.Fatal("expected error for 500 response on post worklog")
	}
}

func TestUpdateWorklog_Error(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.onTimeEntry = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("fail"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.UpdateWorklog(context.Background(), "IRUOY-0001", "5", 30*time.Minute, "update", time.Now())
	if err == nil {
		t.Fatal("expected error for 500 response on update worklog")
	}
}

func TestDeleteWorklog_Error(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.onTimeEntry = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("fail"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.DeleteWorklog(context.Background(), "IRUOY-0001", "5")
	if err == nil {
		t.Fatal("expected error for 500 response on delete worklog")
	}
}

func TestFetchWorklogs_Errors(t *testing.T) {
	t.Run("user fetch error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("fail"))
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		_, err := client.FetchWorklogs(context.Background(), time.Now(), time.Now(), task.WorklogFilter{})
		if err == nil {
			t.Fatal("expected error when user endpoint fails")
		}
	})

	t.Run("time entries fetch error", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onFetchTimeEntries = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("fail"))
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		_, err := client.FetchWorklogs(context.Background(), time.Now(), time.Now(), task.WorklogFilter{})
		if err == nil {
			t.Fatal("expected error when time entries endpoint fails")
		}
	})

	t.Run("filters entries outside date range", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onFetchTimeEntries = func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode([]timeEntryJSON{
					{
						ID: 1, UserID: 42, MinutesSpent: 60,
						Note: "in range", StartedAt: "2025-01-20T09:00:00Z",
						IssueKey: "IRUOY-0001", ProjectID: 1,
					},
					{
						ID: 2, UserID: 42, MinutesSpent: 60,
						Note: "out of range", StartedAt: "2025-01-19T09:00:00Z",
						IssueKey: "IRUOY-0002", ProjectID: 1,
					},
				})
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		since := time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)
		until := time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)
		entries, err := client.FetchWorklogs(context.Background(), since, until, task.WorklogFilter{})
		if err != nil {
			t.Fatalf("FetchWorklogs: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry in range, got %d", len(entries))
		}
		if entries[0].IssueKey != "IRUOY-0001" {
			t.Errorf("IssueKey = %q, want IRUOY-0001", entries[0].IssueKey)
		}
	})
}

func TestGetDueDate(t *testing.T) {
	t.Run("no due date returns nil", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.issues = map[string]issueJSON{
				"IRUOY-0001": {ID: 1, Key: "IRUOY-0001", Title: "No due date", Priority: 0, LaneID: 10, ProjectID: 1},
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		due, err := client.GetDueDate(context.Background(), "IRUOY-0001")
		if err != nil {
			t.Fatalf("GetDueDate: %v", err)
		}
		if due != nil {
			t.Errorf("expected nil due date, got %v", due)
		}
	})

	t.Run("parses due date from title with braces", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.issues = map[string]issueJSON{
				"IRUOY-0001": {ID: 1, Key: "IRUOY-0001", Title: "Task {2025-03-15}", Priority: 0, LaneID: 10, ProjectID: 1},
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		due, err := client.GetDueDate(context.Background(), "IRUOY-0001")
		if err != nil {
			t.Fatalf("GetDueDate: %v", err)
		}
		if due == nil {
			t.Fatal("expected non-nil due date")
		}
		expected := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
		if !due.Equal(expected) {
			t.Errorf("due = %v, want %v", due, expected)
		}
	})
}

func TestUpdateSprint(t *testing.T) {
	t.Run("sets sprint", func(t *testing.T) {
		var gotPayload map[string]interface{}
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onPutIssue = func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&gotPayload)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		sprintID := 50
		err := client.UpdateSprint(context.Background(), "IRUOY-0001", &sprintID)
		if err != nil {
			t.Fatalf("UpdateSprint: %v", err)
		}
		if int(gotPayload["sprint_id"].(float64)) != 50 {
			t.Errorf("sprint_id = %v, want 50", gotPayload["sprint_id"])
		}
	})

	t.Run("removes sprint with nil", func(t *testing.T) {
		var gotPayload map[string]interface{}
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onPutIssue = func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&gotPayload)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.UpdateSprint(context.Background(), "IRUOY-0001", nil)
		if err != nil {
			t.Fatalf("UpdateSprint: %v", err)
		}
		if gotPayload["sprint_id"] != nil {
			t.Errorf("sprint_id = %v, want nil", gotPayload["sprint_id"])
		}
	})
}

func TestGetParent(t *testing.T) {
	t.Run("returns epic ID", func(t *testing.T) {
		epicID := 100
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.issues = map[string]issueJSON{
				"IRUOY-0001": {ID: 1, Key: "IRUOY-0001", Title: "Task", Priority: 0, LaneID: 10, ProjectID: 1, EpicID: &epicID},
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		parent, err := client.GetParent(context.Background(), "IRUOY-0001")
		if err != nil {
			t.Fatalf("GetParent: %v", err)
		}
		if parent != "100" {
			t.Errorf("parent = %q, want '100'", parent)
		}
	})

	t.Run("returns empty when no epic", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.issues = map[string]issueJSON{
				"IRUOY-0001": {ID: 1, Key: "IRUOY-0001", Title: "Task", Priority: 0, LaneID: 10, ProjectID: 1},
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		parent, err := client.GetParent(context.Background(), "IRUOY-0001")
		if err != nil {
			t.Fatalf("GetParent: %v", err)
		}
		if parent != "" {
			t.Errorf("parent = %q, want empty", parent)
		}
	})
}

func TestUpdateParent(t *testing.T) {
	t.Run("sets epic", func(t *testing.T) {
		var gotPayload map[string]interface{}
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onPutIssue = func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&gotPayload)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.UpdateParent(context.Background(), "IRUOY-0001", "100")
		if err != nil {
			t.Fatalf("UpdateParent: %v", err)
		}
		if int(gotPayload["epic_id"].(float64)) != 100 {
			t.Errorf("epic_id = %v, want 100", gotPayload["epic_id"])
		}
	})

	t.Run("clears epic with empty key", func(t *testing.T) {
		var gotPayload map[string]interface{}
		server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
			cfg.onPutIssue = func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&gotPayload)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.UpdateParent(context.Background(), "IRUOY-0001", "")
		if err != nil {
			t.Fatalf("UpdateParent: %v", err)
		}
		if gotPayload["epic_id"] != nil {
			t.Errorf("epic_id = %v, want nil", gotPayload["epic_id"])
		}
	})

	t.Run("error on unknown epic", func(t *testing.T) {
		server := httptest.NewServer(kendoHandler(t))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.UpdateParent(context.Background(), "IRUOY-0001", "9999")
		if err == nil {
			t.Fatal("expected error for unknown epic")
		}
	})
}

func TestCompleteTask_NoDoneLane(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.lanes = []laneJSON{
			{ID: 10, Title: "To Do"},
			{ID: 11, Title: "In Progress"},
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.CompleteTask(context.Background(), "IRUOY-0001")
	if err == nil {
		t.Fatal("expected error when no done lane exists")
	}
}

func TestPriorityConversions(t *testing.T) {
	tests := []struct {
		kendo int
		fylla int
	}{
		{0, 1},
		{1, 2},
		{2, 3},
		{3, 4},
		{4, 5},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("kendo_%d_to_fylla_%d", tt.kendo, tt.fylla), func(t *testing.T) {
			got := kendoPriorityToFylla(tt.kendo)
			if got != tt.fylla {
				t.Errorf("kendoPriorityToFylla(%d) = %d, want %d", tt.kendo, got, tt.fylla)
			}
		})
		t.Run(fmt.Sprintf("fylla_%d_to_kendo_%d", tt.fylla, tt.kendo), func(t *testing.T) {
			got := fyllaPriorityToKendo(tt.fylla)
			if got != tt.kendo {
				t.Errorf("fyllaPriorityToKendo(%d) = %d, want %d", tt.fylla, got, tt.kendo)
			}
		})
	}
}

func TestIssueUpdatePayload(t *testing.T) {
	assignee := 42
	sprint := 10
	epic := 100
	est := 60
	current := issueJSON{
		Title:            "Original",
		Description:      "Desc",
		LaneID:           10,
		Priority:         2,
		Type:             0,
		Order:            1,
		AssigneeID:       &assignee,
		SprintID:         &sprint,
		EpicID:           &epic,
		EstimatedMinutes: &est,
		BlockedByIDs:     []int{1, 2},
		BlocksIDs:        []int{3},
	}
	overrides := map[string]interface{}{
		"title":    "Updated",
		"priority": 0,
	}

	payload := issueUpdatePayload(current, overrides)

	if payload["title"] != "Updated" {
		t.Errorf("title = %v, want 'Updated'", payload["title"])
	}
	if payload["priority"] != 0 {
		t.Errorf("priority = %v, want 0", payload["priority"])
	}
	if payload["description"] != "Desc" {
		t.Errorf("description = %v, want 'Desc'", payload["description"])
	}
	if payload["assignee_id"] != 42 {
		t.Errorf("assignee_id = %v, want 42", payload["assignee_id"])
	}
	if payload["sprint_id"] != 10 {
		t.Errorf("sprint_id = %v, want 10", payload["sprint_id"])
	}
	if payload["epic_id"] != 100 {
		t.Errorf("epic_id = %v, want 100", payload["epic_id"])
	}
	if payload["estimated_minutes"] != 60 {
		t.Errorf("estimated_minutes = %v, want 60", payload["estimated_minutes"])
	}
}

func TestIssueUpdatePayload_NilOptionals(t *testing.T) {
	current := issueJSON{
		Title:    "Task",
		Priority: 1,
	}
	payload := issueUpdatePayload(current, nil)

	if _, ok := payload["assignee_id"]; ok {
		t.Error("assignee_id should not be in payload when nil")
	}
	if _, ok := payload["sprint_id"]; ok {
		t.Error("sprint_id should not be in payload when nil")
	}
	if _, ok := payload["epic_id"]; ok {
		t.Error("epic_id should not be in payload when nil")
	}
	if _, ok := payload["estimated_minutes"]; ok {
		t.Error("estimated_minutes should not be in payload when nil")
	}
}

func TestIsFilterQuery(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"assignee_id=me", true},
		{"project_id=1&lane_id=2", true},
		{"search text", false},
		{"*", false},
		{"", false},
		{"IRUOY", false},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := isFilterQuery(tt.query)
			if got != tt.want {
				t.Errorf("isFilterQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestParseFilter(t *testing.T) {
	client := &Client{}

	t.Run("assignee_id=me", func(t *testing.T) {
		f := client.parseFilter("assignee_id=me", 42)
		if f.assigneeID == nil || *f.assigneeID != 42 {
			t.Errorf("assigneeID = %v, want 42", f.assigneeID)
		}
	})

	t.Run("assignee_id=numeric", func(t *testing.T) {
		f := client.parseFilter("assignee_id=99", 42)
		if f.assigneeID == nil || *f.assigneeID != 99 {
			t.Errorf("assigneeID = %v, want 99", f.assigneeID)
		}
	})

	t.Run("project_id", func(t *testing.T) {
		f := client.parseFilter("project_id=5", 42)
		if f.projectID == nil || *f.projectID != 5 {
			t.Errorf("projectID = %v, want 5", f.projectID)
		}
	})

	t.Run("lane_id", func(t *testing.T) {
		f := client.parseFilter("lane_id=10", 42)
		if f.laneID == nil || *f.laneID != 10 {
			t.Errorf("laneID = %v, want 10", f.laneID)
		}
	})

	t.Run("priority", func(t *testing.T) {
		f := client.parseFilter("priority=2", 42)
		if f.priority == nil || *f.priority != 2 {
			t.Errorf("priority = %v, want 2", f.priority)
		}
	})

	t.Run("multiple filters", func(t *testing.T) {
		f := client.parseFilter("assignee_id=me&priority=1", 42)
		if f.assigneeID == nil || *f.assigneeID != 42 {
			t.Errorf("assigneeID = %v, want 42", f.assigneeID)
		}
		if f.priority == nil || *f.priority != 1 {
			t.Errorf("priority = %v, want 1", f.priority)
		}
	})

	t.Run("invalid filter parts are skipped", func(t *testing.T) {
		f := client.parseFilter("invalid&assignee_id=me", 42)
		if f.assigneeID == nil || *f.assigneeID != 42 {
			t.Errorf("assigneeID = %v, want 42", f.assigneeID)
		}
	})
}

func TestIssueFilter_Matches(t *testing.T) {
	assignee := 42
	issue := issueJSON{
		ID:         1,
		AssigneeID: &assignee,
		ProjectID:  5,
		LaneID:     10,
		Priority:   2,
	}

	t.Run("empty filter matches all", func(t *testing.T) {
		f := issueFilter{}
		if !f.matches(issue) {
			t.Error("empty filter should match all issues")
		}
	})

	t.Run("assignee match", func(t *testing.T) {
		f := issueFilter{assigneeID: &assignee}
		if !f.matches(issue) {
			t.Error("should match assignee")
		}
	})

	t.Run("assignee mismatch", func(t *testing.T) {
		other := 99
		f := issueFilter{assigneeID: &other}
		if f.matches(issue) {
			t.Error("should not match different assignee")
		}
	})

	t.Run("nil assignee does not match filter", func(t *testing.T) {
		noAssignee := issueJSON{ID: 2, ProjectID: 5, LaneID: 10, Priority: 2}
		f := issueFilter{assigneeID: &assignee}
		if f.matches(noAssignee) {
			t.Error("should not match issue with nil assignee")
		}
	})

	t.Run("project match", func(t *testing.T) {
		pid := 5
		f := issueFilter{projectID: &pid}
		if !f.matches(issue) {
			t.Error("should match project")
		}
	})

	t.Run("project mismatch", func(t *testing.T) {
		pid := 99
		f := issueFilter{projectID: &pid}
		if f.matches(issue) {
			t.Error("should not match different project")
		}
	})

	t.Run("lane match", func(t *testing.T) {
		lid := 10
		f := issueFilter{laneID: &lid}
		if !f.matches(issue) {
			t.Error("should match lane")
		}
	})

	t.Run("priority match", func(t *testing.T) {
		p := 2
		f := issueFilter{priority: &p}
		if !f.matches(issue) {
			t.Error("should match priority")
		}
	})
}

func TestIssueMatchesText(t *testing.T) {
	issue := issueJSON{
		Key:   "IRUOY-0001",
		Title: "Fix login bug",
	}

	tests := []struct {
		search string
		want   bool
	}{
		{"login", true},
		{"LOGIN", true},
		{"IRUOY", true},
		{"0001", true},
		{"nonexistent", false},
		{"Fix", true},
	}

	for _, tt := range tests {
		t.Run(tt.search, func(t *testing.T) {
			got := issueMatchesText(issue, tt.search)
			if got != tt.want {
				t.Errorf("issueMatchesText(%q) = %v, want %v", tt.search, got, tt.want)
			}
		})
	}
}

func TestIsTextSearch(t *testing.T) {
	client := &Client{
		projects: []project{
			{ID: 1, Name: "Iruoy", Code: "IRUOY"},
		},
	}

	tests := []struct {
		query string
		want  bool
	}{
		{"", false},
		{"*", false},
		{"assignee_id=me", false},
		{"IRUOY", false},
		{"Iruoy", false},
		{"42", false},
		{"login bug", true},
		{"search term", true},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := client.isTextSearch(tt.query)
			if got != tt.want {
				t.Errorf("isTextSearch(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestParseIssueWithEpic(t *testing.T) {
	epicID := 100
	issue := issueJSON{
		ID: 1, Key: "IRUOY-0001", Title: "Task with epic",
		Priority: 2, ProjectID: 1, EpicID: &epicID,
	}
	epicMap := map[int]string{100: "My Epic"}
	tk := parseIssue(issue, "IRUOY", "To Do", epicMap)
	if tk.Section != "My Epic" {
		t.Errorf("Section = %q, want 'My Epic'", tk.Section)
	}
}

func TestParseIssueWithSprintID(t *testing.T) {
	sprintID := 50
	issue := issueJSON{
		ID: 1, Key: "IRUOY-0001", Title: "Sprint task",
		Priority: 2, ProjectID: 1, SprintID: &sprintID,
	}
	tk := parseIssue(issue, "IRUOY", "To Do", nil)
	if tk.SprintID == nil || *tk.SprintID != 50 {
		t.Errorf("SprintID = %v, want 50", tk.SprintID)
	}
}

func TestLanesCache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/projects":
			json.NewEncoder(w).Encode([]project{{ID: 1, Name: "Iruoy", Code: "IRUOY"}})
		case "/api/projects/1/lanes":
			callCount++
			json.NewEncoder(w).Encode([]laneJSON{{ID: 10, Title: "To Do"}})
		default:
			json.NewEncoder(w).Encode([]interface{}{})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	ctx := context.Background()

	_, err := client.fetchLaneMap(ctx, 1)
	if err != nil {
		t.Fatalf("fetchLaneMap: %v", err)
	}
	_, err = client.fetchLaneMap(ctx, 1)
	if err != nil {
		t.Fatalf("fetchLaneMap (second): %v", err)
	}
	if callCount != 1 {
		t.Errorf("lanes endpoint called %d times, want 1 (cached)", callCount)
	}
}

func TestEpicsCache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/projects":
			json.NewEncoder(w).Encode([]project{{ID: 1, Name: "Iruoy", Code: "IRUOY"}})
		case "/api/projects/1/epics":
			callCount++
			json.NewEncoder(w).Encode([]epicJSON{{ID: 100, Title: "Epic"}})
		default:
			json.NewEncoder(w).Encode([]interface{}{})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	ctx := context.Background()

	_, err := client.fetchEpicMap(ctx, 1)
	if err != nil {
		t.Fatalf("fetchEpicMap: %v", err)
	}
	_, err = client.fetchEpicMap(ctx, 1)
	if err != nil {
		t.Fatalf("fetchEpicMap (second): %v", err)
	}
	if callCount != 1 {
		t.Errorf("epics endpoint called %d times, want 1 (cached)", callCount)
	}
}

func TestProjectsOnce(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects" {
			callCount++
			json.NewEncoder(w).Encode([]project{{ID: 1, Name: "Iruoy", Code: "IRUOY"}})
			return
		}
		json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	ctx := context.Background()

	_ = client.loadProjects(ctx)
	_ = client.loadProjects(ctx)
	if callCount != 1 {
		t.Errorf("projects endpoint called %d times, want 1 (sync.Once)", callCount)
	}
}

func TestProjectIDForKey(t *testing.T) {
	server := httptest.NewServer(kendoHandler(t))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	ctx := context.Background()

	t.Run("valid key", func(t *testing.T) {
		pid, err := client.ProjectIDForKey(ctx, "IRUOY-0001")
		if err != nil {
			t.Fatalf("ProjectIDForKey: %v", err)
		}
		if pid != 1 {
			t.Errorf("pid = %d, want 1", pid)
		}
	})

	t.Run("invalid key format", func(t *testing.T) {
		_, err := client.ProjectIDForKey(ctx, "BADKEY")
		if err == nil {
			t.Fatal("expected error for invalid key")
		}
	})

	t.Run("unknown project code", func(t *testing.T) {
		_, err := client.ProjectIDForKey(ctx, "UNKNOWN-0001")
		if err == nil {
			t.Fatal("expected error for unknown project code")
		}
	})
}

func TestNewClient(t *testing.T) {
	client := NewClient("http://example.com/", "my-token")
	if client.BaseURL != "http://example.com" {
		t.Errorf("BaseURL = %q, want trailing slash trimmed", client.BaseURL)
	}
	if client.Token != "my-token" {
		t.Errorf("Token = %q, want 'my-token'", client.Token)
	}
	if client.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
}

func TestUpdateWorklog_Payload(t *testing.T) {
	var gotPayload map[string]interface{}
	var gotPath string
	server := httptest.NewServer(kendoHandler(t, func(cfg *kendoHandlerConfig) {
		cfg.onTimeEntry = func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			json.NewDecoder(r.Body).Decode(&gotPayload)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	started := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	err := client.UpdateWorklog(context.Background(), "IRUOY-0001", "7", 45*time.Minute, "updated note", started)
	if err != nil {
		t.Fatalf("UpdateWorklog: %v", err)
	}
	if !strings.Contains(gotPath, "/time-entries/7") {
		t.Errorf("path = %q, want to contain /time-entries/7", gotPath)
	}
	if int(gotPayload["minutes_spent"].(float64)) != 45 {
		t.Errorf("minutes_spent = %v, want 45", gotPayload["minutes_spent"])
	}
	if gotPayload["note"] != "updated note" {
		t.Errorf("note = %v, want 'updated note'", gotPayload["note"])
	}
}

func TestFetchUserID(t *testing.T) {
	t.Run("fetches user ID from API", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/auth/user" {
				json.NewEncoder(w).Encode(map[string]interface{}{"id": 42})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.fetchUserID(context.Background())
		if err != nil {
			t.Fatalf("fetchUserID: %v", err)
		}
		if client.UserID != 42 {
			t.Errorf("UserID = %d, want 42", client.UserID)
		}
	})

	t.Run("skips fetch when UserID already set", func(t *testing.T) {
		called := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		client.UserID = 99
		err := client.fetchUserID(context.Background())
		if err != nil {
			t.Fatalf("fetchUserID: %v", err)
		}
		if called {
			t.Error("should not call API when UserID is already set")
		}
		if client.UserID != 99 {
			t.Errorf("UserID = %d, want 99 (unchanged)", client.UserID)
		}
	})

	t.Run("error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("unauthorized"))
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.fetchUserID(context.Background())
		if err == nil {
			t.Fatal("expected error for 401 response")
		}
	})
}

func TestDoneLaneIDFromMap(t *testing.T) {
	client := &Client{}

	t.Run("finds default done lane", func(t *testing.T) {
		m := map[int]string{10: "To Do", 11: "In Progress", 12: "Done"}
		id := client.doneLaneIDFromMap(m)
		if id != 12 {
			t.Errorf("doneLaneIDFromMap = %d, want 12", id)
		}
	})

	t.Run("finds custom done lane", func(t *testing.T) {
		client.DoneLane = "Completed"
		m := map[int]string{10: "To Do", 11: "Completed"}
		id := client.doneLaneIDFromMap(m)
		if id != 11 {
			t.Errorf("doneLaneIDFromMap = %d, want 11", id)
		}
		client.DoneLane = ""
	})

	t.Run("returns -1 when not found", func(t *testing.T) {
		m := map[int]string{10: "To Do", 11: "In Progress"}
		id := client.doneLaneIDFromMap(m)
		if id != -1 {
			t.Errorf("doneLaneIDFromMap = %d, want -1", id)
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		m := map[int]string{10: "DONE"}
		id := client.doneLaneIDFromMap(m)
		if id != 10 {
			t.Errorf("doneLaneIDFromMap = %d, want 10", id)
		}
	})
}

func TestProjectCodeByID(t *testing.T) {
	client := &Client{
		projects: []project{
			{ID: 1, Name: "Iruoy", Code: "IRUOY"},
			{ID: 2, Name: "Admin", Code: "ADMIN"},
		},
	}

	if code := client.projectCodeByID(1); code != "IRUOY" {
		t.Errorf("projectCodeByID(1) = %q, want IRUOY", code)
	}
	if code := client.projectCodeByID(2); code != "ADMIN" {
		t.Errorf("projectCodeByID(2) = %q, want ADMIN", code)
	}
	if code := client.projectCodeByID(99); code != "" {
		t.Errorf("projectCodeByID(99) = %q, want empty", code)
	}
}
