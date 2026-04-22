package github

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
		// Flat 30m estimate for all PRs
		if task.RemainingEstimate.Minutes() != 30 {
			t.Errorf("estimate = %v, want 30m", task.RemainingEstimate)
		}
	})

	t.Run("parses inline clauses from title", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"total_count": 1,
				"items": []map[string]interface{}{
					{
						"number":         7,
						"title":          "Important thing [2h] {2026-05-01} (priority:p1)",
						"created_at":     "2025-01-15T10:00:00Z",
						"repository_url": "https://api.github.com/repos/iruoy/fylla",
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

		tasks, err := client.FetchTasks(context.Background(), "is:issue")
		if err != nil {
			t.Fatalf("FetchTasks: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		got := tasks[0]
		if got.Summary != "Important thing" {
			t.Errorf("summary = %q, want 'Important thing'", got.Summary)
		}
		if got.Priority != 1 {
			t.Errorf("priority = %d, want 1", got.Priority)
		}
		if got.RemainingEstimate != 2*time.Hour {
			t.Errorf("estimate = %v, want 2h", got.RemainingEstimate)
		}
		if got.DueDate == nil || got.DueDate.Format("2006-01-02") != "2026-05-01" {
			t.Errorf("due = %v, want 2026-05-01", got.DueDate)
		}
	})

	t.Run("includes non-PR issues as Issue type", func(t *testing.T) {
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

		tasks, err := client.FetchTasks(context.Background(), "is:issue state:open assignee:@me")
		if err != nil {
			t.Fatalf("FetchTasks: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].IssueType != "Issue" {
			t.Errorf("issueType = %q, want Issue", tasks[0].IssueType)
		}
		if tasks[0].Key != "repo#1" {
			t.Errorf("key = %q, want repo#1", tasks[0].Key)
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
		{"someone/anyrepo#9", "someone", "anyrepo", 9, false},
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

func TestResolveIssueKey(t *testing.T) {
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

		key, err := client.ResolveIssueKey(context.Background(), "fylla#42")
		if err != nil {
			t.Fatalf("ResolveIssueKey: %v", err)
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

		key, err := client.ResolveIssueKey(context.Background(), "fylla#10")
		if err != nil {
			t.Fatalf("ResolveIssueKey: %v", err)
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
					"ref": "fix/no-issue-key",
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

		key, err := client.ResolveIssueKey(context.Background(), "fylla#5")
		if err != nil {
			t.Fatalf("ResolveIssueKey: %v", err)
		}
		if key != "" {
			t.Errorf("key = %q, want empty", key)
		}
	})
}

func TestCreateTask(t *testing.T) {
	t.Run("creates issue and returns key", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/repos/iruoy/fylla/issues", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "want POST", http.StatusMethodNotAllowed)
				return
			}
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)
			resp := map[string]interface{}{
				"number": 99,
				"title":  req["title"],
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(resp)
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient("test-token")
		client.SetHTTPClient(server.Client())
		client.SetBaseURL(server.URL + "/")
		client.Repos = []string{"iruoy/fylla"}

		key, err := client.CreateTask(context.Background(), task.CreateInput{
			Project: "fylla",
			Summary: "New issue",
		})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if key != "fylla#99" {
			t.Errorf("key = %q, want fylla#99", key)
		}
	})

	t.Run("unknown repo returns error", func(t *testing.T) {
		client := NewClient("test-token")
		client.Repos = []string{"iruoy/fylla"}

		_, err := client.CreateTask(context.Background(), task.CreateInput{
			Project: "unknown",
			Summary: "test",
		})
		if err == nil {
			t.Fatal("expected error for unknown repo")
		}
	})
}

func TestListProjects(t *testing.T) {
	t.Run("returns short repo names", func(t *testing.T) {
		client := NewClient("test-token")
		client.Repos = []string{"iruoy/fylla", "org/backend"}

		projects, err := client.ListProjects(context.Background())
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		if len(projects) != 2 {
			t.Fatalf("expected 2 projects, got %d", len(projects))
		}
		if projects[0] != "fylla" {
			t.Errorf("projects[0] = %q, want fylla", projects[0])
		}
		if projects[1] != "backend" {
			t.Errorf("projects[1] = %q, want backend", projects[1])
		}
	})

	t.Run("no repos returns empty list", func(t *testing.T) {
		client := NewClient("test-token")

		projects, err := client.ListProjects(context.Background())
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		if len(projects) != 0 {
			t.Errorf("expected 0 projects, got %d", len(projects))
		}
	})
}

func TestUnsupportedOperations(t *testing.T) {
	client := NewClient("token")
	ctx := context.Background()

	if err := client.UpdateEstimate(ctx, "r#1", 0); err == nil {
		t.Error("UpdateEstimate should return error")
	}
	if err := client.UpdatePriority(ctx, "r#1", 1); err == nil {
		t.Error("UpdatePriority should return error")
	}
}

func TestCloseOperations(t *testing.T) {
	mux := http.NewServeMux()
	var lastReq map[string]interface{}
	currentTitle := "original title"
	mux.HandleFunc("/repos/iruoy/fylla/issues/5", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{"number": 5, "title": currentTitle})
		case http.MethodPatch:
			lastReq = map[string]interface{}{}
			json.NewDecoder(r.Body).Decode(&lastReq)
			if t, ok := lastReq["title"].(string); ok {
				currentTitle = t
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"number": 5, "state": "closed"})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient("test-token")
	client.SetHTTPClient(server.Client())
	client.SetBaseURL(server.URL + "/")
	client.Repos = []string{"iruoy/fylla"}

	t.Run("CompleteTask closes with completed reason", func(t *testing.T) {
		if err := client.CompleteTask(context.Background(), "fylla#5"); err != nil {
			t.Fatalf("CompleteTask: %v", err)
		}
		if lastReq["state"] != "closed" || lastReq["state_reason"] != "completed" {
			t.Errorf("req = %v, want state=closed reason=completed", lastReq)
		}
	})

	t.Run("DeleteTask closes with not_planned reason", func(t *testing.T) {
		if err := client.DeleteTask(context.Background(), "fylla#5"); err != nil {
			t.Fatalf("DeleteTask: %v", err)
		}
		if lastReq["state"] != "closed" || lastReq["state_reason"] != "not_planned" {
			t.Errorf("req = %v, want state=closed reason=not_planned", lastReq)
		}
	})

	t.Run("UpdateSummary patches title", func(t *testing.T) {
		if err := client.UpdateSummary(context.Background(), "fylla#5", "new title"); err != nil {
			t.Fatalf("UpdateSummary: %v", err)
		}
		if lastReq["title"] != "new title" {
			t.Errorf("req = %v, want title=new title", lastReq)
		}
	})

	t.Run("accepts owner/repo#N key form", func(t *testing.T) {
		c := NewClient("test-token")
		c.SetHTTPClient(server.Client())
		c.SetBaseURL(server.URL + "/")
		if err := c.CompleteTask(context.Background(), "iruoy/fylla#5"); err != nil {
			t.Fatalf("CompleteTask with owner/repo: %v", err)
		}
	})
}

func TestMetadataOperations(t *testing.T) {
	mux := http.NewServeMux()
	title := "Do thing [15m] (priority:p3)"
	mux.HandleFunc("/repos/iruoy/fylla/issues/9", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{"number": 9, "title": title})
		case http.MethodPatch:
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)
			if t, ok := req["title"].(string); ok {
				title = t
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"number": 9, "title": title})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient("test-token")
	client.SetHTTPClient(server.Client())
	client.SetBaseURL(server.URL + "/")
	client.Repos = []string{"iruoy/fylla"}

	ctx := context.Background()

	t.Run("UpdatePriority preserves estimate and summary", func(t *testing.T) {
		if err := client.UpdatePriority(ctx, "fylla#9", 1); err != nil {
			t.Fatalf("UpdatePriority: %v", err)
		}
		if title != "Do thing [15m] (priority:p1)" {
			t.Errorf("title = %q", title)
		}
	})

	t.Run("UpdateEstimate mutates in place", func(t *testing.T) {
		if err := client.UpdateEstimate(ctx, "fylla#9", time.Hour); err != nil {
			t.Fatalf("UpdateEstimate: %v", err)
		}
		if title != "Do thing [1h] (priority:p1)" {
			t.Errorf("title = %q", title)
		}
	})

	t.Run("UpdateDueDate adds due token", func(t *testing.T) {
		due := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
		if err := client.UpdateDueDate(ctx, "fylla#9", due); err != nil {
			t.Fatalf("UpdateDueDate: %v", err)
		}
		if title != "Do thing [1h] (priority:p1) {2026-05-10}" {
			t.Errorf("title = %q", title)
		}
	})

	t.Run("RemoveDueDate strips due token", func(t *testing.T) {
		if err := client.RemoveDueDate(ctx, "fylla#9"); err != nil {
			t.Fatalf("RemoveDueDate: %v", err)
		}
		if title != "Do thing [1h] (priority:p1)" {
			t.Errorf("title = %q", title)
		}
	})

	t.Run("UpdateSummary preserves metadata", func(t *testing.T) {
		if err := client.UpdateSummary(ctx, "fylla#9", "New summary"); err != nil {
			t.Fatalf("UpdateSummary: %v", err)
		}
		if title != "New summary [1h] (priority:p1)" {
			t.Errorf("title = %q", title)
		}
	})

	t.Run("GetSummary strips metadata", func(t *testing.T) {
		s, err := client.GetSummary(ctx, "fylla#9")
		if err != nil {
			t.Fatalf("GetSummary: %v", err)
		}
		if s != "New summary" {
			t.Errorf("summary = %q", s)
		}
	})

	t.Run("GetPriority reads meta", func(t *testing.T) {
		p, err := client.GetPriority(ctx, "fylla#9")
		if err != nil {
			t.Fatalf("GetPriority: %v", err)
		}
		if p != 1 {
			t.Errorf("priority = %d, want 1", p)
		}
	})

	t.Run("GetEstimate reads meta", func(t *testing.T) {
		d, err := client.GetEstimate(ctx, "fylla#9")
		if err != nil {
			t.Fatalf("GetEstimate: %v", err)
		}
		if d != time.Hour {
			t.Errorf("estimate = %v, want 1h", d)
		}
	})
}

func TestCreateTaskEmbedsMetadata(t *testing.T) {
	var lastTitle string
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/iruoy/fylla/issues", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		lastTitle, _ = req["title"].(string)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"number": 100})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient("test-token")
	client.SetHTTPClient(server.Client())
	client.SetBaseURL(server.URL + "/")
	client.Repos = []string{"iruoy/fylla"}

	due := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	_, err := client.CreateTask(context.Background(), task.CreateInput{
		Project:  "fylla",
		Summary:  "New bug",
		Priority: "Highest",
		Estimate: 90 * time.Minute,
		DueDate:  &due,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	want := "New bug [1h30m] {2026-06-01} (priority:p1)"
	if lastTitle != want {
		t.Errorf("title = %q, want %q", lastTitle, want)
	}
}
