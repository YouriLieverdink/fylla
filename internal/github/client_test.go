package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iruoy/fylla/internal/task"
)

func TestFetchTasks(t *testing.T) {
	t.Run("returns PRs as tasks with correct key format", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"total_count": 1,
				"items": []map[string]interface{}{
					{
						"number":         42,
						"title":          "Fix login bug",
						"created_at":     "2025-01-15T10:00:00Z",
						"repository_url": "https://api.github.com/repos/iruoy/fylla",
						"pull_request": map[string]interface{}{
							"url": "https://api.github.com/repos/iruoy/fylla/pulls/42",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		})
		mux.HandleFunc("/repos/iruoy/fylla/pulls/42", func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"additions": 30,
				"deletions": 10,
			}
			json.NewEncoder(w).Encode(resp)
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient("test-token")
		client.SetHTTPClient(server.Client())
		client.SetBaseURL(server.URL + "/")

		tasks, err := client.FetchTasks(context.Background(), "is:pr state:open review-requested:@me")
		if err != nil {
			t.Fatalf("FetchTasks: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}

		task := tasks[0]
		if task.Key != "fylla#42" {
			t.Errorf("key = %q, want fylla#42", task.Key)
		}
		if task.Summary != "Fix login bug" {
			t.Errorf("summary = %q, want 'Fix login bug'", task.Summary)
		}
		if task.Priority != 2 {
			t.Errorf("priority = %d, want 2", task.Priority)
		}
		if task.IssueType != "Pull Request" {
			t.Errorf("issueType = %q, want 'Pull Request'", task.IssueType)
		}
		if task.Project != "iruoy/fylla" {
			t.Errorf("project = %q, want 'iruoy/fylla'", task.Project)
		}
		// 30+10=40 lines → 15m estimate
		if task.RemainingEstimate.Minutes() != 15 {
			t.Errorf("estimate = %v, want 15m", task.RemainingEstimate)
		}
	})

	t.Run("large PR gets longer estimate", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"total_count": 1,
				"items": []map[string]interface{}{
					{
						"number":         10,
						"title":          "Big refactor",
						"created_at":     "2025-01-10T08:00:00Z",
						"repository_url": "https://api.github.com/repos/org/repo",
						"pull_request": map[string]interface{}{
							"url": "https://api.github.com/repos/org/repo/pulls/10",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		})
		mux.HandleFunc("/repos/org/repo/pulls/10", func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"additions": 600,
				"deletions": 200,
			}
			json.NewEncoder(w).Encode(resp)
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient("test-token")
		client.SetHTTPClient(server.Client())
		client.SetBaseURL(server.URL + "/")

		tasks, err := client.FetchTasks(context.Background(), "is:pr state:open review-requested:@me")
		if err != nil {
			t.Fatalf("FetchTasks: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		// 600+200=800 lines → 1h estimate
		if tasks[0].RemainingEstimate.Minutes() != 60 {
			t.Errorf("estimate = %v, want 1h", tasks[0].RemainingEstimate)
		}
	})

	t.Run("skips non-PR issues", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"total_count": 1,
				"items": []map[string]interface{}{
					{
						"number":         1,
						"title":          "Not a PR",
						"created_at":     "2025-01-15T10:00:00Z",
						"repository_url": "https://api.github.com/repos/org/repo",
						// no pull_request field
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient("test-token")
		client.SetHTTPClient(server.Client())
		client.SetBaseURL(server.URL + "/")

		tasks, err := client.FetchTasks(context.Background(), "is:pr state:open review-requested:@me")
		if err != nil {
			t.Fatalf("FetchTasks: %v", err)
		}
		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks, got %d", len(tasks))
		}
	})
}

func TestParsePRKey(t *testing.T) {
	repos := []string{"iruoy/fylla", "org/backend"}

	tests := []struct {
		key       string
		wantOwner string
		wantRepo  string
		wantNum   int
		wantErr   bool
	}{
		{"fylla#42", "iruoy", "fylla", 42, false},
		{"backend#7", "org", "backend", 7, false},
		{"unknown#1", "", "", 0, true},
		{"nohash", "", "", 0, true},
		{"repo#abc", "", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			owner, repo, num, err := parsePRKey(tt.key, repos)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for key %q", tt.key)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner || repo != tt.wantRepo || num != tt.wantNum {
				t.Errorf("got (%q, %q, %d), want (%q, %q, %d)",
					owner, repo, num, tt.wantOwner, tt.wantRepo, tt.wantNum)
			}
		})
	}
}

func TestResolveJiraKey(t *testing.T) {
	t.Run("extracts key from branch name", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/repos/iruoy/fylla/pulls/42", func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"head": map[string]interface{}{
					"ref": "feature/PROJ-123-fix-login",
				},
				"body": "Some description",
			}
			json.NewEncoder(w).Encode(resp)
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient("test-token")
		client.SetHTTPClient(server.Client())
		client.SetBaseURL(server.URL + "/")
		client.Repos = []string{"iruoy/fylla"}

		key, err := client.ResolveJiraKey(context.Background(), "fylla#42")
		if err != nil {
			t.Fatalf("ResolveJiraKey: %v", err)
		}
		if key != "PROJ-123" {
			t.Errorf("key = %q, want PROJ-123", key)
		}
	})

	t.Run("extracts key from body when not in branch", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/repos/iruoy/fylla/pulls/10", func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"head": map[string]interface{}{
					"ref": "fix/something",
				},
				"body": "Fixes TEAM-456 login issue",
			}
			json.NewEncoder(w).Encode(resp)
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient("test-token")
		client.SetHTTPClient(server.Client())
		client.SetBaseURL(server.URL + "/")
		client.Repos = []string{"iruoy/fylla"}

		key, err := client.ResolveJiraKey(context.Background(), "fylla#10")
		if err != nil {
			t.Fatalf("ResolveJiraKey: %v", err)
		}
		if key != "TEAM-456" {
			t.Errorf("key = %q, want TEAM-456", key)
		}
	})

	t.Run("returns empty when no key found", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/repos/iruoy/fylla/pulls/5", func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"head": map[string]interface{}{
					"ref": "fix/no-jira-key",
				},
				"body": "Just a fix, no ticket reference",
			}
			json.NewEncoder(w).Encode(resp)
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient("test-token")
		client.SetHTTPClient(server.Client())
		client.SetBaseURL(server.URL + "/")
		client.Repos = []string{"iruoy/fylla"}

		key, err := client.ResolveJiraKey(context.Background(), "fylla#5")
		if err != nil {
			t.Fatalf("ResolveJiraKey: %v", err)
		}
		if key != "" {
			t.Errorf("key = %q, want empty", key)
		}
	})
}

func TestUnsupportedOperations(t *testing.T) {
	client := NewClient("token")
	ctx := context.Background()

	if _, err := client.CreateTask(ctx, task.CreateInput{}); err == nil {
		t.Error("CreateTask should return error")
	}
	if err := client.CompleteTask(ctx, "r#1"); err == nil {
		t.Error("CompleteTask should return error")
	}
	if err := client.DeleteTask(ctx, "r#1"); err == nil {
		t.Error("DeleteTask should return error")
	}
	if err := client.UpdateEstimate(ctx, "r#1", 0); err == nil {
		t.Error("UpdateEstimate should return error")
	}
	if err := client.UpdatePriority(ctx, "r#1", 1); err == nil {
		t.Error("UpdatePriority should return error")
	}
	if err := client.UpdateSummary(ctx, "r#1", "x"); err == nil {
		t.Error("UpdateSummary should return error")
	}
}
