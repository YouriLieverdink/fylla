package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJIRA001_FetchTasks(t *testing.T) {
	t.Run("fetches tasks from Jira REST API", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rest/api/3/search" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			json.NewEncoder(w).Encode(searchResponse{
				Issues: []issueJSON{
					{
						Key: "PROJ-1",
						Fields: fieldsJSON{
							Summary:   "Test task",
							Priority:  &priorityJSON{Name: "High"},
							IssueType: issueTypeJSON{Name: "Task"},
							Project:   projectJSON{Key: "PROJ"},
							Created:   "2025-01-01T10:00:00.000+0000",
						},
					},
				},
			})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "token123")
		tasks, err := client.FetchTasks(context.Background(), "project = PROJ")
		if err != nil {
			t.Fatalf("FetchTasks: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].Key != "PROJ-1" {
			t.Errorf("expected key PROJ-1, got %s", tasks[0].Key)
		}
		if tasks[0].Summary != "Test task" {
			t.Errorf("expected summary 'Test task', got %s", tasks[0].Summary)
		}
	})

	t.Run("uses basic auth credentials", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok {
				t.Error("expected basic auth")
			}
			if user != "user@test.com" || pass != "token123" {
				t.Errorf("unexpected credentials: %s / %s", user, pass)
			}
			json.NewEncoder(w).Encode(searchResponse{})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "token123")
		_, err := client.FetchTasks(context.Background(), "project = PROJ")
		if err != nil {
			t.Fatalf("FetchTasks: %v", err)
		}
	})

	t.Run("handles HTTP errors gracefully", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"Unauthorized"}`))
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "bad-token")
		_, err := client.FetchTasks(context.Background(), "project = PROJ")
		if err == nil {
			t.Fatal("expected error for 401 response")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("error should mention status code: %v", err)
		}
	})
}

func TestJIRA002_CustomJQL(t *testing.T) {
	t.Run("sends custom JQL in request body", func(t *testing.T) {
		customJQL := "project = MYPROJ AND status = 'In Progress'"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)
			if payload["jql"] != customJQL {
				t.Errorf("expected JQL %q, got %q", customJQL, payload["jql"])
			}
			json.NewEncoder(w).Encode(searchResponse{})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "token123")
		_, err := client.FetchTasks(context.Background(), customJQL)
		if err != nil {
			t.Fatalf("FetchTasks: %v", err)
		}
	})

	t.Run("returns error for invalid JQL response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"errorMessages":["Error in the JQL Query"]}`))
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "token123")
		_, err := client.FetchTasks(context.Background(), "INVALID JQL !!!")
		if err == nil {
			t.Fatal("expected error for invalid JQL")
		}
		if !strings.Contains(err.Error(), "400") {
			t.Errorf("error should mention status 400: %v", err)
		}
	})
}

func TestJIRA003_PostWorklog(t *testing.T) {
	t.Run("posts worklog to correct endpoint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rest/api/3/issue/PROJ-123/worklog" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)
			if int(payload["timeSpentSeconds"].(float64)) != 7200 {
				t.Errorf("expected 7200 seconds, got %v", payload["timeSpentSeconds"])
			}
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "token123")
		err := client.PostWorklog(context.Background(), "PROJ-123", 2*time.Hour, "Worked on feature")
		if err != nil {
			t.Fatalf("PostWorklog: %v", err)
		}
	})

	t.Run("includes description in worklog comment", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "Worked on feature") {
				t.Error("worklog should contain description")
			}
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "token123")
		err := client.PostWorklog(context.Background(), "PROJ-123", 2*time.Hour, "Worked on feature")
		if err != nil {
			t.Fatalf("PostWorklog: %v", err)
		}
	})

	t.Run("records time correctly", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)
			seconds := int(payload["timeSpentSeconds"].(float64))
			if seconds != 7200 {
				t.Errorf("expected 7200 seconds (2h), got %d", seconds)
			}
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "token123")
		err := client.PostWorklog(context.Background(), "PROJ-123", 2*time.Hour, "desc")
		if err != nil {
			t.Fatalf("PostWorklog: %v", err)
		}
	})
}

func TestJIRA004_UpdateEstimate(t *testing.T) {
	t.Run("updates remaining estimate via PUT", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rest/api/3/issue/PROJ-123" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)
			fields := payload["fields"].(map[string]interface{})
			tt := fields["timetracking"].(map[string]interface{})
			if tt["remainingEstimate"] != "4h" {
				t.Errorf("expected remainingEstimate 4h, got %v", tt["remainingEstimate"])
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "token123")
		err := client.UpdateEstimate(context.Background(), "PROJ-123", 4*time.Hour)
		if err != nil {
			t.Fatalf("UpdateEstimate: %v", err)
		}
	})

	t.Run("formats mixed hours and minutes", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)
			fields := payload["fields"].(map[string]interface{})
			tt := fields["timetracking"].(map[string]interface{})
			if tt["remainingEstimate"] != "2h 30m" {
				t.Errorf("expected '2h 30m', got %v", tt["remainingEstimate"])
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "token123")
		err := client.UpdateEstimate(context.Background(), "PROJ-123", 2*time.Hour+30*time.Minute)
		if err != nil {
			t.Fatalf("UpdateEstimate: %v", err)
		}
	})
}

func TestJIRA005_CreateIssue(t *testing.T) {
	t.Run("creates issue with all fields", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rest/api/3/issue" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)
			fields := payload["fields"].(map[string]interface{})

			proj := fields["project"].(map[string]interface{})
			if proj["key"] != "PROJ" {
				t.Errorf("expected project PROJ, got %v", proj["key"])
			}

			itype := fields["issuetype"].(map[string]interface{})
			if itype["name"] != "Bug" {
				t.Errorf("expected issuetype Bug, got %v", itype["name"])
			}

			if fields["summary"] != "Fix login bug" {
				t.Errorf("expected summary 'Fix login bug', got %v", fields["summary"])
			}

			prio := fields["priority"].(map[string]interface{})
			if prio["name"] != "High" {
				t.Errorf("expected priority High, got %v", prio["name"])
			}

			tt := fields["timetracking"].(map[string]interface{})
			if tt["originalEstimate"] != "2h" {
				t.Errorf("expected estimate 2h, got %v", tt["originalEstimate"])
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(createIssueResponse{Key: "PROJ-456"})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "token123")
		key, err := client.CreateIssue(context.Background(), CreateIssueInput{
			Project:     "PROJ",
			IssueType:   "Bug",
			Summary:     "Fix login bug",
			Description: "Login is broken",
			Estimate:    2 * time.Hour,
			Priority:    "High",
		})
		if err != nil {
			t.Fatalf("CreateIssue: %v", err)
		}
		if key != "PROJ-456" {
			t.Errorf("expected key PROJ-456, got %s", key)
		}
	})

	t.Run("creates issue with minimal fields", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)
			fields := payload["fields"].(map[string]interface{})
			if _, ok := fields["description"]; ok {
				t.Error("should not include description when empty")
			}
			if _, ok := fields["timetracking"]; ok {
				t.Error("should not include timetracking when estimate is 0")
			}
			if _, ok := fields["priority"]; ok {
				t.Error("should not include priority when empty")
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(createIssueResponse{Key: "PROJ-457"})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user@test.com", "token123")
		key, err := client.CreateIssue(context.Background(), CreateIssueInput{
			Project:   "PROJ",
			IssueType: "Task",
			Summary:   "Quick bugfix",
		})
		if err != nil {
			t.Fatalf("CreateIssue: %v", err)
		}
		if key != "PROJ-457" {
			t.Errorf("expected key PROJ-457, got %s", key)
		}
	})
}

func TestJIRA006_ParsePriority(t *testing.T) {
	tests := []struct {
		name     string
		priority string
		expected int
	}{
		{"Highest maps to 1", "Highest", 1},
		{"High maps to 2", "High", 2},
		{"Medium maps to 3", "Medium", 3},
		{"Low maps to 4", "Low", 4},
		{"Lowest maps to 5", "Lowest", 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := parseIssue(issueJSON{
				Key: "TEST-1",
				Fields: fieldsJSON{
					Priority:  &priorityJSON{Name: tc.priority},
					IssueType: issueTypeJSON{Name: "Task"},
					Project:   projectJSON{Key: "TEST"},
				},
			})
			if task.Priority != tc.expected {
				t.Errorf("priority %s: expected %d, got %d", tc.priority, tc.expected, task.Priority)
			}
		})
	}

	t.Run("nil priority defaults to Medium (3)", func(t *testing.T) {
		task := parseIssue(issueJSON{
			Key: "TEST-1",
			Fields: fieldsJSON{
				Priority:  nil,
				IssueType: issueTypeJSON{Name: "Task"},
				Project:   projectJSON{Key: "TEST"},
			},
		})
		if task.Priority != 3 {
			t.Errorf("expected default priority 3, got %d", task.Priority)
		}
	})
}

func TestJIRA007_ParseDueDate(t *testing.T) {
	t.Run("parses due date correctly", func(t *testing.T) {
		task := parseIssue(issueJSON{
			Key: "TEST-1",
			Fields: fieldsJSON{
				DueDate:   "2025-06-15",
				IssueType: issueTypeJSON{Name: "Task"},
				Project:   projectJSON{Key: "TEST"},
			},
		})
		if task.DueDate == nil {
			t.Fatal("expected due date to be parsed")
		}
		expected := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
		if !task.DueDate.Equal(expected) {
			t.Errorf("expected %v, got %v", expected, *task.DueDate)
		}
	})

	t.Run("handles missing due date gracefully", func(t *testing.T) {
		task := parseIssue(issueJSON{
			Key: "TEST-1",
			Fields: fieldsJSON{
				DueDate:   "",
				IssueType: issueTypeJSON{Name: "Task"},
				Project:   projectJSON{Key: "TEST"},
			},
		})
		if task.DueDate != nil {
			t.Error("expected nil due date for empty string")
		}
	})
}

func TestJIRA008_ParseEstimates(t *testing.T) {
	t.Run("parses original estimate (4h)", func(t *testing.T) {
		task := parseIssue(issueJSON{
			Key: "TEST-1",
			Fields: fieldsJSON{
				IssueType: issueTypeJSON{Name: "Task"},
				Project:   projectJSON{Key: "TEST"},
				TimeTracking: &timeTrackingJSON{
					OriginalEstimateSeconds:  14400, // 4h
					RemainingEstimateSeconds: 7200,  // 2h
				},
			},
		})
		if task.OriginalEstimate != 4*time.Hour {
			t.Errorf("expected original estimate 4h, got %v", task.OriginalEstimate)
		}
	})

	t.Run("parses remaining estimate (2h)", func(t *testing.T) {
		task := parseIssue(issueJSON{
			Key: "TEST-1",
			Fields: fieldsJSON{
				IssueType: issueTypeJSON{Name: "Task"},
				Project:   projectJSON{Key: "TEST"},
				TimeTracking: &timeTrackingJSON{
					OriginalEstimateSeconds:  14400,
					RemainingEstimateSeconds: 7200,
				},
			},
		})
		if task.RemainingEstimate != 2*time.Hour {
			t.Errorf("expected remaining estimate 2h, got %v", task.RemainingEstimate)
		}
	})

	t.Run("handles nil time tracking", func(t *testing.T) {
		task := parseIssue(issueJSON{
			Key: "TEST-1",
			Fields: fieldsJSON{
				IssueType:    issueTypeJSON{Name: "Task"},
				Project:      projectJSON{Key: "TEST"},
				TimeTracking: nil,
			},
		})
		if task.OriginalEstimate != 0 {
			t.Errorf("expected zero original estimate, got %v", task.OriginalEstimate)
		}
		if task.RemainingEstimate != 0 {
			t.Errorf("expected zero remaining estimate, got %v", task.RemainingEstimate)
		}
	})
}

func TestJIRA009_ParseIssueType(t *testing.T) {
	types := []string{"Bug", "Task", "Story"}
	for _, issueType := range types {
		t.Run("parses "+issueType+" type", func(t *testing.T) {
			task := parseIssue(issueJSON{
				Key: "TEST-1",
				Fields: fieldsJSON{
					IssueType: issueTypeJSON{Name: issueType},
					Project:   projectJSON{Key: "TEST"},
				},
			})
			if task.IssueType != issueType {
				t.Errorf("expected type %s, got %s", issueType, task.IssueType)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{4 * time.Hour, "4h"},
		{30 * time.Minute, "30m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{0, "0m"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := formatDuration(tc.d)
			if got != tc.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}
