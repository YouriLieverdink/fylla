package kendo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func projectsResponse() []byte {
	data, _ := json.Marshal([]project{
		{ID: 1, Name: "Iruoy", Code: "IRUOY"},
		{ID: 2, Name: "Admin", Code: "ADMIN"},
	})
	return data
}

func TestFetchTasks(t *testing.T) {
	t.Run("parses issues and sets Provider field", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer test-token" {
				t.Errorf("Authorization = %q, want Bearer test-token", auth)
			}

			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			case "/api/projects/1/issues":
				est := 120
				json.NewEncoder(w).Encode([]issueJSON{
					{
						ID:               1,
						Key:              "IRUOY-0001",
						Title:            "Fix login bug",
						Priority:         1,
						EstimatedMinutes: &est,
						CreatedAt:        "2025-01-20T09:00:00Z",
						ProjectID:        1,
					},
				})
			default:
				json.NewEncoder(w).Encode([]issueJSON{})
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		tasks, err := client.FetchTasks(context.Background(), "IRUOY")
		if err != nil {
			t.Fatalf("FetchTasks: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("got %d tasks, want 1", len(tasks))
		}
		if tasks[0].Key != "IRUOY-0001" {
			t.Errorf("Key = %q, want IRUOY-0001", tasks[0].Key)
		}
		if tasks[0].Provider != "kendo" {
			t.Errorf("Provider = %q, want kendo", tasks[0].Provider)
		}
		if tasks[0].Summary != "Fix login bug" {
			t.Errorf("Summary = %q, want Fix login bug", tasks[0].Summary)
		}
		if tasks[0].RemainingEstimate != 2*time.Hour {
			t.Errorf("RemainingEstimate = %v, want 2h", tasks[0].RemainingEstimate)
		}
		// Priority 1 (High in Kendo) -> 2 in Fylla
		if tasks[0].Priority != 2 {
			t.Errorf("Priority = %d, want 2", tasks[0].Priority)
		}
	})

	t.Run("excludes issues in done lane", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			case "/api/projects/1/lanes":
				json.NewEncoder(w).Encode([]laneJSON{
					{ID: 10, Title: "To Do"},
					{ID: 11, Title: "In Progress"},
					{ID: 12, Title: "Done"},
				})
			case "/api/projects/1/issues":
				est := 60
				json.NewEncoder(w).Encode([]issueJSON{
					{ID: 1, Key: "IRUOY-0001", Title: "Active task", LaneID: 10, EstimatedMinutes: &est, CreatedAt: "2025-01-20T09:00:00Z", ProjectID: 1},
					{ID: 2, Key: "IRUOY-0002", Title: "Done task", LaneID: 12, EstimatedMinutes: &est, CreatedAt: "2025-01-20T09:00:00Z", ProjectID: 1},
					{ID: 3, Key: "IRUOY-0003", Title: "Another active", LaneID: 11, EstimatedMinutes: &est, CreatedAt: "2025-01-20T09:00:00Z", ProjectID: 1},
				})
			default:
				json.NewEncoder(w).Encode([]issueJSON{})
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		tasks, err := client.FetchTasks(context.Background(), "IRUOY")
		if err != nil {
			t.Fatalf("FetchTasks: %v", err)
		}
		if len(tasks) != 2 {
			t.Fatalf("got %d tasks, want 2 (done task should be excluded)", len(tasks))
		}
		for _, tk := range tasks {
			if tk.Key == "IRUOY-0002" {
				t.Errorf("done task IRUOY-0002 should have been excluded")
			}
		}
	})

	t.Run("error on non-200 response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/projects" {
				w.Write(projectsResponse())
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		_, err := client.FetchTasks(context.Background(), "IRUOY")
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
	})
}

func TestParseIssueSummary(t *testing.T) {
	titles := []struct {
		title   string
		wantNon string // non-empty summary expected
	}{
		{"Intern development", "Intern development"},
		{"Code reviews, refactoring, spikes", "Code reviews, refactoring, spikes"},
		{"Fix login bug", "Fix login bug"},
		{"Inventaris up-to-date maken.", "Inventaris up-to-date maken."},
		{"On-premise hardware documenteren", "On-premise hardware documenteren"},
		{"Github organisatie naar de self-hosted runner", "Github organisatie naar de self-hosted runner"},
		{"Klantdossier aanmaken in Elements", "Klantdossier aanmaken in Elements"},
		{"Sentry opnieuw instellen", "Sentry opnieuw instellen"},
		{"Overstappen naar Kendo.dev", "Overstappen naar Kendo.dev"},
	}
	for _, tc := range titles {
		t.Run(tc.title, func(t *testing.T) {
			issue := issueJSON{
				ID:    1,
				Key:   "TEST-0001",
				Title: tc.title,
			}
			task := parseIssue(issue, "TEST", "To Do", nil)
			if task.Summary != tc.wantNon {
				t.Errorf("parseIssue(%q).Summary = %q, want %q", tc.title, task.Summary, tc.wantNon)
			}
		})
	}
}

func TestPostWorklog(t *testing.T) {
	t.Run("sends correct request body", func(t *testing.T) {
		var gotBody map[string]interface{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			case "/api/projects/1/issues/IRUOY-0001/time-entries":
				if r.Method != http.MethodPost {
					t.Errorf("Method = %s, want POST", r.Method)
				}
				json.NewDecoder(r.Body).Decode(&gotBody)
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		started := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
		err := client.PostWorklog(context.Background(), "IRUOY-0001", 30*time.Minute, "test work", started)
		if err != nil {
			t.Fatalf("PostWorklog: %v", err)
		}
		if int(gotBody["minutes_spent"].(float64)) != 30 {
			t.Errorf("minutes_spent = %v, want 30", gotBody["minutes_spent"])
		}
		if gotBody["note"] != "test work" {
			t.Errorf("note = %v, want test work", gotBody["note"])
		}
	})
}

func TestCompleteTask(t *testing.T) {
	t.Run("sends lane_id update", func(t *testing.T) {
		var gotBody map[string]interface{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			case "/api/projects/1/lanes":
				json.NewEncoder(w).Encode([]laneJSON{
					{ID: 10, Title: "To Do"},
					{ID: 11, Title: "In Progress"},
					{ID: 12, Title: "Completed"},
				})
			case "/api/projects/1/issues/IRUOY-0001":
				if r.Method == http.MethodPut {
					json.NewDecoder(r.Body).Decode(&gotBody)
				}
				json.NewEncoder(w).Encode(issueJSON{})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		client.DoneLane = "Completed"
		err := client.CompleteTask(context.Background(), "IRUOY-0001")
		if err != nil {
			t.Fatalf("CompleteTask: %v", err)
		}
		if int(gotBody["lane_id"].(float64)) != 12 {
			t.Errorf("lane_id = %v, want 12", gotBody["lane_id"])
		}
	})
}

func TestFetchWorklogs(t *testing.T) {
	t.Run("returns only current user entries", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/auth/user":
				json.NewEncoder(w).Encode(map[string]interface{}{"id": 42})
			case "/api/projects":
				w.Write(projectsResponse())
			case "/api/time-entries":
				json.NewEncoder(w).Encode([]timeEntryJSON{
					{
						ID:           1,
						UserID:       42,
						MinutesSpent: 60,
						Note:         "worked on stuff",
						StartedAt:    "2025-01-20T09:00:00Z",
						IssueTitle:   "Fix bug",
						IssueKey:     "IRUOY-0001",
						ProjectID:    1,
					},
					{
						ID:           2,
						UserID:       99,
						MinutesSpent: 120,
						Note:         "colleague work",
						StartedAt:    "2025-01-20T10:00:00Z",
						IssueTitle:   "Other task",
						IssueKey:     "IRUOY-0002",
						ProjectID:    1,
					},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		since := time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)
		until := time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)
		entries, err := client.FetchWorklogs(context.Background(), since, until)
		if err != nil {
			t.Fatalf("FetchWorklogs: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1 (colleague entry should be filtered)", len(entries))
		}
		if entries[0].IssueKey != "IRUOY-0001" {
			t.Errorf("IssueKey = %q, want IRUOY-0001", entries[0].IssueKey)
		}
		if entries[0].TimeSpent != time.Hour {
			t.Errorf("TimeSpent = %v, want 1h", entries[0].TimeSpent)
		}
		if entries[0].Provider != "kendo" {
			t.Errorf("Provider = %q, want kendo", entries[0].Provider)
		}
	})
}

func TestUpdateWorklog(t *testing.T) {
	t.Run("sends PUT request", func(t *testing.T) {
		var gotMethod string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			default:
				gotMethod = r.Method
				json.NewEncoder(w).Encode(map[string]interface{}{})
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.UpdateWorklog(context.Background(), "IRUOY-0001", "5", 45*time.Minute, "updated", time.Now())
		if err != nil {
			t.Fatalf("UpdateWorklog: %v", err)
		}
		if gotMethod != http.MethodPut {
			t.Errorf("Method = %s, want PUT", gotMethod)
		}
	})
}

func TestDeleteWorklog(t *testing.T) {
	t.Run("sends DELETE request", func(t *testing.T) {
		var gotMethod string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			default:
				gotMethod = r.Method
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.DeleteWorklog(context.Background(), "IRUOY-0001", "5")
		if err != nil {
			t.Fatalf("DeleteWorklog: %v", err)
		}
		if gotMethod != http.MethodDelete {
			t.Errorf("Method = %s, want DELETE", gotMethod)
		}
	})
}

func TestSplitKey(t *testing.T) {
	tests := []struct {
		key      string
		wantCode string
		wantNum  string
	}{
		{"IRUOY-0001", "IRUOY", "0001"},
		{"PROJ-123", "PROJ", "123"},
		{"AB-1", "AB", "1"},
		{"", "", ""},
		{"NOHYPHEN", "", ""},
		{"-1", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			code, num := splitKey(tt.key)
			if code != tt.wantCode {
				t.Errorf("code = %q, want %q", code, tt.wantCode)
			}
			if num != tt.wantNum {
				t.Errorf("num = %q, want %q", num, tt.wantNum)
			}
		})
	}
}

func TestListTransitions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/projects":
			w.Write(projectsResponse())
		case "/api/projects/1/lanes":
			json.NewEncoder(w).Encode([]laneJSON{
				{ID: 10, Title: "To Do"},
				{ID: 11, Title: "In Progress"},
				{ID: 12, Title: "Done"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	names, err := client.ListTransitions(context.Background(), "IRUOY-0001")
	if err != nil {
		t.Fatalf("ListTransitions: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 lanes, got %d", len(names))
	}
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	for _, want := range []string{"To Do", "In Progress", "Done"} {
		if !found[want] {
			t.Errorf("missing lane %q in %v", want, names)
		}
	}
}

func TestTransitionTask(t *testing.T) {
	t.Run("moves issue to target lane", func(t *testing.T) {
		var updatedLaneID int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			case "/api/projects/1/lanes":
				json.NewEncoder(w).Encode([]laneJSON{
					{ID: 10, Title: "To Do"},
					{ID: 11, Title: "In Progress"},
					{ID: 12, Title: "Done"},
				})
			case "/api/projects/1/issues/IRUOY-0001":
				if r.Method == http.MethodGet {
					json.NewEncoder(w).Encode(issueJSON{
						ID: 1, Key: "IRUOY-0001", Title: "Test", Priority: 2, LaneID: 10, ProjectID: 1,
					})
					return
				}
				if r.Method == http.MethodPut {
					var payload map[string]interface{}
					json.NewDecoder(r.Body).Decode(&payload)
					if lid, ok := payload["lane_id"].(float64); ok {
						updatedLaneID = int(lid)
					}
					w.WriteHeader(http.StatusOK)
					return
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.TransitionTask(context.Background(), "IRUOY-0001", "In Progress")
		if err != nil {
			t.Fatalf("TransitionTask: %v", err)
		}
		if updatedLaneID != 11 {
			t.Errorf("updated lane_id = %d, want 11", updatedLaneID)
		}
	})

	t.Run("returns error for unknown lane", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			case "/api/projects/1/lanes":
				json.NewEncoder(w).Encode([]laneJSON{
					{ID: 10, Title: "To Do"},
				})
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.TransitionTask(context.Background(), "IRUOY-0001", "Nonexistent")
		if err == nil {
			t.Fatal("expected error for unknown lane")
		}
	})
}

func TestFetchTasksPopulatesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/projects":
			w.Write(projectsResponse())
		case "/api/projects/1/lanes":
			json.NewEncoder(w).Encode([]laneJSON{
				{ID: 10, Title: "To Do"},
				{ID: 11, Title: "In Progress"},
			})
		case "/api/projects/1/issues":
			json.NewEncoder(w).Encode([]issueJSON{
				{ID: 1, Key: "IRUOY-0001", Title: "Task A", Priority: 2, LaneID: 11, ProjectID: 1},
			})
		default:
			json.NewEncoder(w).Encode([]interface{}{})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	tasks, err := client.FetchTasks(context.Background(), "IRUOY")
	if err != nil {
		t.Fatalf("FetchTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Status != "In Progress" {
		t.Errorf("Status = %q, want In Progress", tasks[0].Status)
	}
}
