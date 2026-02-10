package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	googlecalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// newTestClient creates a GoogleClient backed by the given httptest.Server.
func newTestClient(t *testing.T, server *httptest.Server, sourceCalendar, fyllaCalendar, jiraBaseURL string) *GoogleClient {
	t.Helper()
	svc, err := googlecalendar.NewService(context.Background(),
		option.WithHTTPClient(server.Client()),
		option.WithEndpoint(server.URL),
	)
	if err != nil {
		t.Fatalf("create test service: %v", err)
	}
	return &GoogleClient{
		Service:        svc,
		SourceCalendar: sourceCalendar,
		FyllaCalendar:  fyllaCalendar,
		JiraBaseURL:    jiraBaseURL,
	}
}

func Test_GCAL003_FetchEventsFromSourceCalendar(t *testing.T) {
	t.Run("fetches events within time range", func(t *testing.T) {
		start := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 20, 17, 0, 0, 0, time.UTC)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/calendars/primary/events") {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("singleEvents") != "true" {
				t.Error("expected singleEvents=true")
			}
			if q.Get("orderBy") != "startTime" {
				t.Error("expected orderBy=startTime")
			}
			resp := googlecalendar.Events{
				Items: []*googlecalendar.Event{
					{
						Id:      "evt1",
						Summary: "Team standup",
						Start:   &googlecalendar.EventDateTime{DateTime: "2025-01-20T09:30:00Z"},
						End:     &googlecalendar.EventDateTime{DateTime: "2025-01-20T10:00:00Z"},
					},
					{
						Id:      "evt2",
						Summary: "Sprint review",
						Start:   &googlecalendar.EventDateTime{DateTime: "2025-01-20T14:00:00Z"},
						End:     &googlecalendar.EventDateTime{DateTime: "2025-01-20T15:00:00Z"},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://company.atlassian.net")
		events, err := client.FetchEvents(context.Background(), start, end)
		if err != nil {
			t.Fatalf("FetchEvents: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}
		if events[0].Title != "Team standup" {
			t.Errorf("expected 'Team standup', got %q", events[0].Title)
		}
		if events[1].Title != "Sprint review" {
			t.Errorf("expected 'Sprint review', got %q", events[1].Title)
		}
	})

	t.Run("uses source calendar for fetching", func(t *testing.T) {
		var calendarID string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Split(r.URL.Path, "/")
			for i, p := range parts {
				if p == "calendars" && i+1 < len(parts) {
					calendarID = parts[i+1]
				}
			}
			json.NewEncoder(w).Encode(googlecalendar.Events{})
		}))
		defer server.Close()

		client := newTestClient(t, server, "source-cal-id", "fylla-cal-id", "https://co.atlassian.net")
		_, err := client.FetchEvents(context.Background(), time.Now(), time.Now().Add(24*time.Hour))
		if err != nil {
			t.Fatalf("FetchEvents: %v", err)
		}
		if calendarID != "source-cal-id" {
			t.Errorf("expected source calendar 'source-cal-id', got %q", calendarID)
		}
	})

	t.Run("returns busy times from events", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := googlecalendar.Events{
				Items: []*googlecalendar.Event{
					{
						Id:      "busy1",
						Summary: "Meeting",
						Start:   &googlecalendar.EventDateTime{DateTime: "2025-01-20T10:00:00Z"},
						End:     &googlecalendar.EventDateTime{DateTime: "2025-01-20T11:00:00Z"},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://co.atlassian.net")
		events, err := client.FetchEvents(context.Background(), time.Now(), time.Now().Add(24*time.Hour))
		if err != nil {
			t.Fatalf("FetchEvents: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		expectedStart := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
		expectedEnd := time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC)
		if !events[0].Start.Equal(expectedStart) {
			t.Errorf("expected start %v, got %v", expectedStart, events[0].Start)
		}
		if !events[0].End.Equal(expectedEnd) {
			t.Errorf("expected end %v, got %v", expectedEnd, events[0].End)
		}
	})
}

func Test_GCAL004_CreateEventsOnFyllaCalendar(t *testing.T) {
	t.Run("creates event with correct start and end times", func(t *testing.T) {
		var created googlecalendar.Event
		var usedCalendar string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Split(r.URL.Path, "/")
			for i, p := range parts {
				if p == "calendars" && i+1 < len(parts) {
					usedCalendar = parts[i+1]
				}
			}
			if r.Method == http.MethodPost {
				json.NewDecoder(r.Body).Decode(&created)
				created.Id = "new-evt"
				json.NewEncoder(w).Encode(created)
				return
			}
			json.NewEncoder(w).Encode(googlecalendar.Events{})
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla-cal", "https://company.atlassian.net")
		start := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)

		err := client.CreateEvent(context.Background(), CreateEventInput{
			TaskKey: "PROJ-123",
			Summary: "Fix login bug",
			Start:   start,
			End:     end,
		})
		if err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
		if usedCalendar != "fylla-cal" {
			t.Errorf("expected fylla calendar 'fylla-cal', got %q", usedCalendar)
		}
		if created.Start.DateTime != start.Format(time.RFC3339) {
			t.Errorf("expected start %s, got %s", start.Format(time.RFC3339), created.Start.DateTime)
		}
		if created.End.DateTime != end.Format(time.RFC3339) {
			t.Errorf("expected end %s, got %s", end.Format(time.RFC3339), created.End.DateTime)
		}
	})

	t.Run("events appear on fylla calendar", func(t *testing.T) {
		var insertPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				insertPath = r.URL.Path
				json.NewEncoder(w).Encode(googlecalendar.Event{Id: "new"})
				return
			}
			json.NewEncoder(w).Encode(googlecalendar.Events{})
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "my-fylla-cal", "https://co.atlassian.net")
		err := client.CreateEvent(context.Background(), CreateEventInput{
			TaskKey: "PROJ-1",
			Summary: "Test",
			Start:   time.Now(),
			End:     time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
		if !strings.Contains(insertPath, "my-fylla-cal") {
			t.Errorf("expected insert on fylla calendar, got path %q", insertPath)
		}
	})
}

func Test_GCAL005_DeleteFyllaPrefixedEvents(t *testing.T) {
	t.Run("deletes existing Fylla events on sync", func(t *testing.T) {
		var deletedIDs []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				resp := googlecalendar.Events{
					Items: []*googlecalendar.Event{
						{Id: "fylla-1", Summary: "[Fylla] PROJ-1: Old task"},
						{Id: "fylla-2", Summary: "[Fylla] PROJ-2: Another old task"},
						{Id: "meeting-1", Summary: "Team meeting"},
					},
				}
				json.NewEncoder(w).Encode(resp)
				return
			}
			if r.Method == http.MethodDelete {
				parts := strings.Split(r.URL.Path, "/")
				deletedIDs = append(deletedIDs, parts[len(parts)-1])
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://co.atlassian.net")
		err := client.DeleteFyllaEvents(context.Background(), time.Now(), time.Now().Add(7*24*time.Hour))
		if err != nil {
			t.Fatalf("DeleteFyllaEvents: %v", err)
		}
		if len(deletedIDs) != 2 {
			t.Fatalf("expected 2 deletions, got %d", len(deletedIDs))
		}
		if deletedIDs[0] != "fylla-1" || deletedIDs[1] != "fylla-2" {
			t.Errorf("expected fylla-1,fylla-2 deleted, got %v", deletedIDs)
		}
	})

	t.Run("does not delete non-Fylla events", func(t *testing.T) {
		var deletedIDs []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				resp := googlecalendar.Events{
					Items: []*googlecalendar.Event{
						{Id: "meeting-1", Summary: "Team standup"},
						{Id: "meeting-2", Summary: "Sprint review"},
					},
				}
				json.NewEncoder(w).Encode(resp)
				return
			}
			if r.Method == http.MethodDelete {
				parts := strings.Split(r.URL.Path, "/")
				deletedIDs = append(deletedIDs, parts[len(parts)-1])
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://co.atlassian.net")
		err := client.DeleteFyllaEvents(context.Background(), time.Now(), time.Now().Add(7*24*time.Hour))
		if err != nil {
			t.Fatalf("DeleteFyllaEvents: %v", err)
		}
		if len(deletedIDs) != 0 {
			t.Errorf("expected 0 deletions, got %d: %v", len(deletedIDs), deletedIDs)
		}
	})

	t.Run("also deletes LATE prefixed Fylla events", func(t *testing.T) {
		var deletedIDs []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				resp := googlecalendar.Events{
					Items: []*googlecalendar.Event{
						{Id: "late-1", Summary: "[LATE] [Fylla] PROJ-1: Overdue task"},
					},
				}
				json.NewEncoder(w).Encode(resp)
				return
			}
			if r.Method == http.MethodDelete {
				parts := strings.Split(r.URL.Path, "/")
				deletedIDs = append(deletedIDs, parts[len(parts)-1])
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://co.atlassian.net")
		err := client.DeleteFyllaEvents(context.Background(), time.Now(), time.Now().Add(7*24*time.Hour))
		if err != nil {
			t.Fatalf("DeleteFyllaEvents: %v", err)
		}
		if len(deletedIDs) != 1 {
			t.Fatalf("expected 1 deletion, got %d", len(deletedIDs))
		}
	})
}

func Test_GCAL006_EventTitleFormat(t *testing.T) {
	t.Run("normal event has Fylla prefix with key and summary", func(t *testing.T) {
		var created googlecalendar.Event
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				json.NewDecoder(r.Body).Decode(&created)
				created.Id = "new"
				json.NewEncoder(w).Encode(created)
				return
			}
			json.NewEncoder(w).Encode(googlecalendar.Events{})
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://co.atlassian.net")
		err := client.CreateEvent(context.Background(), CreateEventInput{
			TaskKey: "PROJ-123",
			Summary: "Fix login bug",
			Start:   time.Now(),
			End:     time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
		expected := "[Fylla] PROJ-123: Fix login bug"
		if created.Summary != expected {
			t.Errorf("expected title %q, got %q", expected, created.Summary)
		}
	})

	t.Run("at-risk event has LATE prefix", func(t *testing.T) {
		var created googlecalendar.Event
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				json.NewDecoder(r.Body).Decode(&created)
				created.Id = "new"
				json.NewEncoder(w).Encode(created)
				return
			}
			json.NewEncoder(w).Encode(googlecalendar.Events{})
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://co.atlassian.net")
		err := client.CreateEvent(context.Background(), CreateEventInput{
			TaskKey: "PROJ-456",
			Summary: "Overdue task",
			Start:   time.Now(),
			End:     time.Now().Add(time.Hour),
			AtRisk:  true,
		})
		if err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
		expected := "[LATE] [Fylla] PROJ-456: Overdue task"
		if created.Summary != expected {
			t.Errorf("expected title %q, got %q", expected, created.Summary)
		}
	})
}

func Test_GCAL007_EventDescriptionIncludesJiraLink(t *testing.T) {
	t.Run("description contains Jira issue URL", func(t *testing.T) {
		var created googlecalendar.Event
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				json.NewDecoder(r.Body).Decode(&created)
				created.Id = "new"
				json.NewEncoder(w).Encode(created)
				return
			}
			json.NewEncoder(w).Encode(googlecalendar.Events{})
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://company.atlassian.net")
		err := client.CreateEvent(context.Background(), CreateEventInput{
			TaskKey: "PROJ-123",
			Summary: "Fix bug",
			Start:   time.Now(),
			End:     time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
		expectedURL := "https://company.atlassian.net/browse/PROJ-123"
		if !strings.Contains(created.Description, expectedURL) {
			t.Errorf("expected description to contain %q, got %q", expectedURL, created.Description)
		}
	})

	t.Run("Jira URL uses configured base URL", func(t *testing.T) {
		var created googlecalendar.Event
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				json.NewDecoder(r.Body).Decode(&created)
				created.Id = "new"
				json.NewEncoder(w).Encode(created)
				return
			}
			json.NewEncoder(w).Encode(googlecalendar.Events{})
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://other.atlassian.net")
		err := client.CreateEvent(context.Background(), CreateEventInput{
			TaskKey: "ABC-99",
			Summary: "Test",
			Start:   time.Now(),
			End:     time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
		if !strings.Contains(created.Description, "https://other.atlassian.net/browse/ABC-99") {
			t.Errorf("expected other.atlassian.net URL, got %q", created.Description)
		}
	})
}

func Test_GCAL008_DetectOOOByEventType(t *testing.T) {
	t.Run("outOfOffice eventType detected as OOO", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := googlecalendar.Events{
				Items: []*googlecalendar.Event{
					{
						Id:        "ooo-1",
						Summary:   "Out of office",
						EventType: "outOfOffice",
						Start:     &googlecalendar.EventDateTime{DateTime: "2025-01-20T00:00:00Z"},
						End:       &googlecalendar.EventDateTime{DateTime: "2025-01-21T00:00:00Z"},
					},
					{
						Id:        "meeting-1",
						Summary:   "Sprint review",
						EventType: "default",
						Start:     &googlecalendar.EventDateTime{DateTime: "2025-01-22T14:00:00Z"},
						End:       &googlecalendar.EventDateTime{DateTime: "2025-01-22T15:00:00Z"},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://co.atlassian.net")
		events, err := client.FetchEvents(context.Background(), time.Now(), time.Now().Add(7*24*time.Hour))
		if err != nil {
			t.Fatalf("FetchEvents: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}
		if !events[0].IsOOO() {
			t.Error("expected first event (outOfOffice) to be OOO")
		}
		if events[1].IsOOO() {
			t.Error("expected second event (default) to NOT be OOO")
		}
	})

	t.Run("no tasks scheduled during OOO period", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := googlecalendar.Events{
				Items: []*googlecalendar.Event{
					{
						Id:        "ooo-1",
						Summary:   "Doctor appointment",
						EventType: "outOfOffice",
						Start:     &googlecalendar.EventDateTime{DateTime: "2025-01-20T09:00:00Z"},
						End:       &googlecalendar.EventDateTime{DateTime: "2025-01-20T17:00:00Z"},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://co.atlassian.net")
		events, err := client.FetchEvents(context.Background(), time.Now(), time.Now().Add(7*24*time.Hour))
		if err != nil {
			t.Fatalf("FetchEvents: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if !events[0].IsOOO() {
			t.Error("expected OOO event")
		}
		// Verify the OOO blocks the 09:00-17:00 range
		expectedStart := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
		expectedEnd := time.Date(2025, 1, 20, 17, 0, 0, 0, time.UTC)
		if !events[0].Start.Equal(expectedStart) || !events[0].End.Equal(expectedEnd) {
			t.Errorf("expected OOO range %v-%v, got %v-%v", expectedStart, expectedEnd, events[0].Start, events[0].End)
		}
	})
}

func Test_GCAL009_DetectOOOByTitlePattern(t *testing.T) {
	tests := []struct {
		title    string
		expected bool
	}{
		{"OOO", true},
		{"OOO - John", true},
		{"Out of Office", true},
		{"Out of office all day", true},
		{"PTO", true},
		{"PTO - Vacation", true},
		{"Vacation", true},
		{"Summer Vacation", true},
		{"Team standup", false},
		{"Sprint review", false},
		{"Lunch with Bob", false},
		{"Project OOOrganization", true}, // contains "ooo"
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("title=%q", tc.title), func(t *testing.T) {
			e := Event{Title: tc.title, EventType: "default"}
			if got := e.IsOOO(); got != tc.expected {
				t.Errorf("IsOOO() for %q = %v, want %v", tc.title, got, tc.expected)
			}
		})
	}

	t.Run("all-day PTO event detected via fetch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := googlecalendar.Events{
				Items: []*googlecalendar.Event{
					{
						Id:      "pto-1",
						Summary: "PTO",
						Start:   &googlecalendar.EventDateTime{Date: "2025-01-20"},
						End:     &googlecalendar.EventDateTime{Date: "2025-01-21"},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://co.atlassian.net")
		events, err := client.FetchEvents(context.Background(), time.Now(), time.Now().Add(7*24*time.Hour))
		if err != nil {
			t.Fatalf("FetchEvents: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if !events[0].IsOOO() {
			t.Error("expected PTO event to be detected as OOO")
		}
		if !events[0].AllDay {
			t.Error("expected PTO event to be all-day")
		}
	})

	t.Run("vacation title detected as OOO", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := googlecalendar.Events{
				Items: []*googlecalendar.Event{
					{
						Id:      "vac-1",
						Summary: "Vacation - week off",
						Start:   &googlecalendar.EventDateTime{Date: "2025-01-20"},
						End:     &googlecalendar.EventDateTime{Date: "2025-01-24"},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := newTestClient(t, server, "primary", "fylla", "https://co.atlassian.net")
		events, err := client.FetchEvents(context.Background(), time.Now(), time.Now().Add(14*24*time.Hour))
		if err != nil {
			t.Fatalf("FetchEvents: %v", err)
		}
		if !events[0].IsOOO() {
			t.Error("expected vacation event to be detected as OOO")
		}
	})
}
