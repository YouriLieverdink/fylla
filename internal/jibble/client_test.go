package jibble

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

// testServer wires a Jibble client to an httptest server with canned responses.
type testServer struct {
	srv       *httptest.Server
	tokenHits int32
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *testServer) {
	t.Helper()
	ts := &testServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&ts.tokenHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"jwt-abc","expires_in":3600}`))
	})
	mux.HandleFunc("/", handler)
	ts.srv = httptest.NewServer(mux)
	t.Cleanup(ts.srv.Close)

	c := NewClient("key", "secret")
	c.identityURL = ts.srv.URL + "/token"
	c.workspaceBaseURL = ts.srv.URL
	c.timeBaseURL = ts.srv.URL
	return c, ts
}

func TestToken_CachedAndBearerAttached(t *testing.T) {
	var sawAuth string
	c, ts := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		writeJSON(w, map[string]any{"value": []any{}})
	})

	// Two GETs should share one token.
	if _, err := c.FetchWorklogs(context.Background(), today(), today(), task.WorklogFilter{}); err != nil {
		// FetchWorklogs also loads projects/clients; ignore decode specifics here.
	}
	_, _ = c.ListProjects(context.Background())

	if sawAuth != "Bearer jwt-abc" {
		t.Errorf("Authorization = %q, want Bearer jwt-abc", sawAuth)
	}
	if got := atomic.LoadInt32(&ts.tokenHits); got != 1 {
		t.Errorf("token endpoint hit %d times, want 1 (cached)", got)
	}
}

func TestToken_RefreshOn401(t *testing.T) {
	var calls int32
	c, ts := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/Projects") {
			if atomic.AddInt32(&calls, 1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			writeJSON(w, map[string]any{"value": []map[string]string{{"id": "p1", "name": "ICie", "clientId": "c1"}}})
			return
		}
		writeJSON(w, map[string]any{"value": []map[string]string{{"id": "c1", "name": "Tjas"}}})
	})

	if _, err := c.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	// One initial token + one refresh after the 401.
	if got := atomic.LoadInt32(&ts.tokenHits); got != 2 {
		t.Errorf("token hits = %d, want 2 (initial + refresh)", got)
	}
}

func TestListProjects_ClientProjectLabels(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/Projects"):
			writeJSON(w, map[string]any{"value": []map[string]string{
				{"id": "p1", "name": "KasCie", "clientId": "c1"},
				{"id": "p2", "name": "ICie", "clientId": "c1"},
				{"id": "p3", "name": "Solo", "clientId": ""},
			}})
		case strings.HasPrefix(r.URL.Path, "/Clients"):
			writeJSON(w, map[string]any{"value": []map[string]string{{"id": "c1", "name": "Tjas"}}})
		default:
			http.NotFound(w, r)
		}
	})

	got, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	want := []string{"Solo", "Tjas / ICie", "Tjas / KasCie"} // sorted
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("label[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPostWorklog_HourEntry(t *testing.T) {
	var posts []hourEntryPost
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/Projects"):
			writeJSON(w, map[string]any{"value": []map[string]string{{"id": "p2", "name": "ICie", "clientId": "c1"}}})
		case strings.HasPrefix(r.URL.Path, "/Clients"):
			writeJSON(w, map[string]any{"value": []map[string]string{{"id": "c1", "name": "Tjas"}}})
		case strings.HasPrefix(r.URL.Path, "/People"):
			writeJSON(w, map[string]any{"value": []map[string]string{{"id": "person-1"}}})
		case r.URL.Path == "/HourEntries" && r.Method == http.MethodPost:
			var p hourEntryPost
			_ = json.NewDecoder(r.Body).Decode(&p)
			posts = append(posts, p)
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	})

	start := time.Date(2026, 6, 7, 9, 0, 0, 0, time.Local)
	err := c.PostWorklog(context.Background(), "Tjas / ICie", 90*time.Minute, "Build site", start)
	if err != nil {
		t.Fatalf("PostWorklog: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected 1 HourEntry post, got %d", len(posts))
	}
	p := posts[0]
	if p.ProjectID != "p2" {
		t.Errorf("ProjectID = %q, want p2 (resolved from label)", p.ProjectID)
	}
	if p.PersonID != "person-1" {
		t.Errorf("PersonID = %q, want person-1", p.PersonID)
	}
	if p.Note != "Build site" {
		t.Errorf("Note = %q, want Build site", p.Note)
	}
	if p.Date != "2026-06-07" {
		t.Errorf("Date = %q, want 2026-06-07", p.Date)
	}
	if p.Duration != "PT1H30M" {
		t.Errorf("Duration = %q, want PT1H30M", p.Duration)
	}
	if p.ClientType != "Web" {
		t.Errorf("ClientType = %q, want Web", p.ClientType)
	}
	if p.Platform.ClientVersion == "" {
		t.Error("Platform.ClientVersion should be set")
	}
}

func TestPostWorklog_UnknownProject(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/Projects"):
			writeJSON(w, map[string]any{"value": []map[string]string{}})
		case strings.HasPrefix(r.URL.Path, "/Clients"):
			writeJSON(w, map[string]any{"value": []map[string]string{}})
		default:
			http.NotFound(w, r)
		}
	})
	err := c.PostWorklog(context.Background(), "Nope", time.Hour, "x", time.Now())
	if err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestFetchWorklogs_HourEntries(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/Projects"):
			writeJSON(w, map[string]any{"value": []map[string]string{
				{"id": "p2", "name": "ICie", "clientId": "c1"},
				{"id": "p9", "name": "Other", "clientId": "c1"},
			}})
		case strings.HasPrefix(r.URL.Path, "/Clients"):
			writeJSON(w, map[string]any{"value": []map[string]string{{"id": "c1", "name": "Tjas"}}})
		case strings.HasPrefix(r.URL.Path, "/HourEntries"):
			writeJSON(w, map[string]any{"value": []map[string]string{
				{"id": "h1", "date": "2026-06-07", "duration": "PT1H", "projectId": "p2", "note": "Site"},
				{"id": "h2", "date": "2026-06-07", "duration": "PT30M", "projectId": "p9"},
			}})
		default:
			http.NotFound(w, r)
		}
	})

	day := time.Date(2026, 6, 7, 0, 0, 0, 0, time.Local)

	all, err := c.FetchWorklogs(context.Background(), day, day, task.WorklogFilter{})
	if err != nil {
		t.Fatalf("FetchWorklogs: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
	if all[0].Project != "ICie" || all[0].TimeSpent != time.Hour {
		t.Errorf("entry0 = %+v, want ICie/1h", all[0])
	}
	if all[0].Description != "Site" || all[0].Provider != "jibble" {
		t.Errorf("entry0 meta = %+v", all[0])
	}

	// Filter by bare project name (targets key on this).
	filtered, err := c.FetchWorklogs(context.Background(), day, day, task.WorklogFilter{Project: "ICie"})
	if err != nil {
		t.Fatalf("FetchWorklogs filtered: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Project != "ICie" {
		t.Fatalf("filtered = %+v, want one ICie entry", filtered)
	}
}

func TestUpdateAndDeleteWorklog(t *testing.T) {
	var patchPath, patched, deletePath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/Projects"):
			writeJSON(w, map[string]any{"value": []map[string]string{{"id": "p2", "name": "ICie", "clientId": "c1"}}})
		case strings.HasPrefix(r.URL.Path, "/Clients"):
			writeJSON(w, map[string]any{"value": []map[string]string{{"id": "c1", "name": "Tjas"}}})
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/HourEntries("):
			patchPath = r.URL.Path
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			patched, _ = body["duration"].(string)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/HourEntries("):
			deletePath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	if err := c.UpdateWorklog(context.Background(), "Tjas / ICie", "h1", 2*time.Hour, "edited", time.Date(2026, 6, 7, 0, 0, 0, 0, time.Local)); err != nil {
		t.Fatalf("UpdateWorklog: %v", err)
	}
	if patchPath != "/HourEntries(h1)" {
		t.Errorf("PATCH path = %q, want /HourEntries(h1)", patchPath)
	}
	if patched != "PT2H" {
		t.Errorf("patched duration = %q, want PT2H", patched)
	}

	if err := c.DeleteWorklog(context.Background(), "", "h1"); err != nil {
		t.Fatalf("DeleteWorklog: %v", err)
	}
	if deletePath != "/HourEntries(h1)" {
		t.Errorf("DELETE path = %q, want /HourEntries(h1)", deletePath)
	}
}

func TestISO8601DurationRoundTrip(t *testing.T) {
	cases := []struct {
		d   time.Duration
		iso string
	}{
		{90 * time.Minute, "PT1H30M"},
		{time.Hour, "PT1H"},
		{45 * time.Minute, "PT45M"},
		{2*time.Hour + 15*time.Minute, "PT2H15M"},
	}
	for _, tc := range cases {
		if got := iso8601Duration(tc.d); got != tc.iso {
			t.Errorf("iso8601Duration(%v) = %q, want %q", tc.d, got, tc.iso)
		}
		if got := iso8601Parse(tc.iso); got != tc.d {
			t.Errorf("iso8601Parse(%q) = %v, want %v", tc.iso, got, tc.d)
		}
	}
	if got := iso8601Parse("P0DT1H30M0S"); got != 90*time.Minute {
		t.Errorf("iso8601Parse(P0DT1H30M0S) = %v, want 1h30m", got)
	}
}

func TestStubsUnsupported(t *testing.T) {
	c := NewClient("k", "s")
	if _, err := c.FetchTasks(context.Background(), ""); err != nil {
		t.Errorf("FetchTasks should return (nil,nil), got err %v", err)
	}
	if _, err := c.CreateTask(context.Background(), task.CreateInput{}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("CreateTask err = %v, want ErrUnsupported", err)
	}
	if err := c.CompleteTask(context.Background(), "x"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("CompleteTask err = %v, want ErrUnsupported", err)
	}
	if _, err := c.GetSummary(context.Background(), "x"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("GetSummary err = %v, want ErrUnsupported", err)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func today() time.Time { return time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC) }
