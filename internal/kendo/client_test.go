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
	data, _ := json.Marshal(map[string]interface{}{
		"data": []project{
			{ID: 1, Name: "Iruoy", Prefix: "IRUOY", Slug: "iruoy"},
			{ID: 2, Name: "Admin", Prefix: "ADMIN", Slug: "admin"},
		},
	})
	return data
}

func TestFetchTasks(t *testing.T) {
	t.Run("parses issues and sets Provider field", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify Bearer auth
			auth := r.Header.Get("Authorization")
			if auth != "Bearer test-token" {
				t.Errorf("Authorization = %q, want Bearer test-token", auth)
			}

			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			case "/api/projects/iruoy/issues":
				est := 120
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": []issueJSON{
						{
							ID:              1,
							Title:           "Fix login bug",
							Number:          1,
							EstimateMinutes: &est,
							CreatedAt:       "2025-01-20T09:00:00Z",
							Project:         projectRef{ID: 1, Name: "Iruoy", Prefix: "IRUOY", Slug: "iruoy"},
						},
					},
				})
			default:
				json.NewEncoder(w).Encode(map[string]interface{}{"data": []issueJSON{}})
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
		if tasks[0].Key != "IRUOY-1" {
			t.Errorf("Key = %q, want IRUOY-1", tasks[0].Key)
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

func TestPostWorklog(t *testing.T) {
	t.Run("sends correct request body", func(t *testing.T) {
		var gotBody map[string]interface{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			case "/api/projects/iruoy/issues/1/time-entries":
				if r.Method != http.MethodPost {
					t.Errorf("Method = %s, want POST", r.Method)
				}
				json.NewDecoder(r.Body).Decode(&gotBody)
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{"id": 1}})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		started := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
		err := client.PostWorklog(context.Background(), "IRUOY-1", 30*time.Minute, "test work", started)
		if err != nil {
			t.Fatalf("PostWorklog: %v", err)
		}
		if int(gotBody["minutes"].(float64)) != 30 {
			t.Errorf("minutes = %v, want 30", gotBody["minutes"])
		}
		if gotBody["description"] != "test work" {
			t.Errorf("description = %v, want test work", gotBody["description"])
		}
	})
}

func TestCompleteTask(t *testing.T) {
	t.Run("sends lane update", func(t *testing.T) {
		var gotBody map[string]interface{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			case "/api/projects/iruoy/issues/1":
				if r.Method == http.MethodPatch {
					json.NewDecoder(r.Body).Decode(&gotBody)
				}
				json.NewEncoder(w).Encode(map[string]interface{}{"data": issueJSON{}})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		client.DoneLane = "completed"
		err := client.CompleteTask(context.Background(), "IRUOY-1")
		if err != nil {
			t.Fatalf("CompleteTask: %v", err)
		}
		if gotBody["lane"] != "completed" {
			t.Errorf("lane = %v, want completed", gotBody["lane"])
		}
	})
}

func TestFetchWorklogs(t *testing.T) {
	t.Run("returns entries for user", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/auth/user":
				json.NewEncoder(w).Encode(map[string]interface{}{"id": 42})
			case "/api/projects":
				w.Write(projectsResponse())
			default:
				// Time entries endpoint
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": []timeEntryJSON{
						{
							ID:          1,
							Minutes:     60,
							Description: "worked on stuff",
							StartedAt:   "2025-01-20T09:00:00Z",
							IssueTitle:  "Fix bug",
							IssueNumber: 1,
							IssueKey:    "IRUOY-1",
						},
					},
				})
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
		if len(entries) == 0 {
			t.Fatal("expected at least 1 entry")
		}
		if entries[0].IssueKey != "IRUOY-1" {
			t.Errorf("IssueKey = %q, want IRUOY-1", entries[0].IssueKey)
		}
		if entries[0].TimeSpent != time.Hour {
			t.Errorf("TimeSpent = %v, want 1h", entries[0].TimeSpent)
		}
	})
}

func TestUpdateWorklog(t *testing.T) {
	t.Run("sends PATCH request", func(t *testing.T) {
		var gotMethod string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/projects":
				w.Write(projectsResponse())
			default:
				gotMethod = r.Method
				json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{}})
			}
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		err := client.UpdateWorklog(context.Background(), "IRUOY-1", "5", 45*time.Minute, "updated", time.Now())
		if err != nil {
			t.Fatalf("UpdateWorklog: %v", err)
		}
		if gotMethod != http.MethodPatch {
			t.Errorf("Method = %s, want PATCH", gotMethod)
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
		err := client.DeleteWorklog(context.Background(), "IRUOY-1", "5")
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
		key        string
		wantPrefix string
		wantNum    string
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
			prefix, num := splitKey(tt.key)
			if prefix != tt.wantPrefix {
				t.Errorf("prefix = %q, want %q", prefix, tt.wantPrefix)
			}
			if num != tt.wantNum {
				t.Errorf("num = %q, want %q", num, tt.wantNum)
			}
		})
	}
}
