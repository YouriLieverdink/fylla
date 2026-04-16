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
func newTestClient(t *testing.T, server *httptest.Server, sourceCalendars []string, fyllaCalendar string) *GoogleClient {
	t.Helper()
	svc, err := googlecalendar.NewService(context.Background(),
		option.WithHTTPClient(server.Client()),
		option.WithEndpoint(server.URL),
	)
	if err != nil {
		t.Fatalf("create test service: %v", err)
	}
	return &GoogleClient{
		Service:         svc,
		SourceCalendars: sourceCalendars,
		FyllaCalendar:   fyllaCalendar,
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
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

		client := newTestClient(t, server, []string{"source-cal-id"}, "fylla-cal-id")
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
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

		client := newTestClient(t, server, []string{"primary"}, "fylla-cal")
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

		client := newTestClient(t, server, []string{"primary"}, "my-fylla-cal")
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

func Test_GCAL005_DeleteFyllaEvents(t *testing.T) {
	t.Run("deletes Fylla-managed events on sync", func(t *testing.T) {
		var deletedIDs []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				resp := googlecalendar.Events{
					Items: []*googlecalendar.Event{
						{Id: "fylla-1", Summary: "Old task", Description: "fylla: PROJ-1\nhttps://co.atlassian.net/browse/PROJ-1"},
						{Id: "fylla-2", Summary: "Another old task", Description: "fylla: PROJ-2\nhttps://co.atlassian.net/browse/PROJ-2"},
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
		err := client.DeleteFyllaEvents(context.Background(), time.Now(), time.Now().Add(7*24*time.Hour))
		if err != nil {
			t.Fatalf("DeleteFyllaEvents: %v", err)
		}
		if len(deletedIDs) != 0 {
			t.Errorf("expected 0 deletions, got %d: %v", len(deletedIDs), deletedIDs)
		}
	})

	t.Run("also deletes LATE Fylla events", func(t *testing.T) {
		var deletedIDs []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				resp := googlecalendar.Events{
					Items: []*googlecalendar.Event{
						{Id: "late-1", Summary: "[LATE] Overdue task", Description: "fylla: PROJ-1\nhttps://co.atlassian.net/browse/PROJ-1"},
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
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
	t.Run("normal event has summary as title", func(t *testing.T) {
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
		err := client.CreateEvent(context.Background(), CreateEventInput{
			TaskKey: "PROJ-123",
			Summary: "Fix login bug",
			Start:   time.Now(),
			End:     time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
		expected := "Fix login bug"
		if created.Summary != expected {
			t.Errorf("expected title %q, got %q", expected, created.Summary)
		}
	})

	t.Run("at-risk event has warning emoji prefix", func(t *testing.T) {
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
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
		expected := "⚠️ Overdue task"
		if created.Summary != expected {
			t.Errorf("expected title %q, got %q", expected, created.Summary)
		}
	})
}

func Test_GCAL007_EventDescriptionIncludesFyllaMarker(t *testing.T) {
	t.Run("description contains task key and fylla marker", func(t *testing.T) {
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
		err := client.CreateEvent(context.Background(), CreateEventInput{
			TaskKey: "PROJ-123",
			Summary: "Fix bug",
			Start:   time.Now(),
			End:     time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
		if !strings.Contains(created.Description, "fylla: PROJ-123") {
			t.Errorf("expected description to contain fylla marker, got %q", created.Description)
		}
	})

	t.Run("description contains fylla marker for different key", func(t *testing.T) {
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
		err := client.CreateEvent(context.Background(), CreateEventInput{
			TaskKey: "ABC-99",
			Summary: "Test",
			Start:   time.Now(),
			End:     time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
		if !strings.Contains(created.Description, "fylla: ABC-99") {
			t.Errorf("expected fylla marker with ABC-99, got %q", created.Description)
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
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

		client := newTestClient(t, server, []string{"primary"}, "fylla")
		events, err := client.FetchEvents(context.Background(), time.Now(), time.Now().Add(14*24*time.Hour))
		if err != nil {
			t.Fatalf("FetchEvents: %v", err)
		}
		if !events[0].IsOOO() {
			t.Error("expected vacation event to be detected as OOO")
		}
	})
}

func TestBuildTitle(t *testing.T) {
	tests := []struct {
		name    string
		project string
		summary string
		atRisk  bool
		want    string
	}{
		{"normal without project", "", "Fix bug", false, "Fix bug"},
		{"normal with project", "PROJ", "Fix bug", false, "[PROJ] Fix bug"},
		{"at-risk without project", "", "Overdue", true, "⚠️ Overdue"},
		{"at-risk with project", "PROJ", "Overdue", true, "⚠️ [PROJ] Overdue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildTitle(tt.project, tt.summary, tt.atRisk)
			if got != tt.want {
				t.Errorf("BuildTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTitle(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		want    ParsedTitle
	}{
		{"plain summary", "Fix bug", ParsedTitle{Summary: "Fix bug"}},
		{"with project", "[PROJ] Fix bug", ParsedTitle{Project: "PROJ", Summary: "Fix bug"}},
		{"with project and section", "[PROJ / Backlog] Fix bug", ParsedTitle{Project: "PROJ", Section: "Backlog", Summary: "Fix bug"}},
		{"at-risk without project", "⚠️ Overdue", ParsedTitle{Summary: "Overdue", AtRisk: true}},
		{"at-risk with project", "⚠️ [PROJ] Overdue", ParsedTitle{Project: "PROJ", Summary: "Overdue", AtRisk: true}},
		{"at-risk with section", "⚠️ [PROJ / Sprint 1] Overdue", ParsedTitle{Project: "PROJ", Section: "Sprint 1", Summary: "Overdue", AtRisk: true}},
		{"empty string", "", ParsedTitle{Summary: ""}},
		{"bracket without close", "[PROJ Fix bug", ParsedTitle{Summary: "[PROJ Fix bug"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTitle(tt.title)
			if got != tt.want {
				t.Errorf("ParseTitle(%q) = %+v, want %+v", tt.title, got, tt.want)
			}
		})
	}

	t.Run("round-trip with BuildTitleWithSection", func(t *testing.T) {
		cases := []struct {
			project string
			section string
			summary string
			atRisk  bool
		}{
			{"", "", "Fix bug", false},
			{"PROJ", "", "Fix bug", false},
			{"PROJ", "Backlog", "Fix bug", false},
			{"", "", "Overdue", true},
			{"PROJ", "", "Overdue", true},
			{"PROJ", "Sprint 1", "Overdue", true},
		}
		for _, c := range cases {
			title := BuildTitleWithSection(c.project, c.section, c.summary, c.atRisk)
			got := ParseTitle(title)
			if got.Project != c.project || got.Section != c.section || got.Summary != c.summary || got.AtRisk != c.atRisk {
				t.Errorf("round-trip failed: BuildTitleWithSection(%q, %q, %q, %v) = %q → ParseTitle = %+v",
					c.project, c.section, c.summary, c.atRisk, title, got)
			}
		}
	})
}

func TestParseTitleDoneMarker(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  ParsedTitle
	}{
		{
			"done with project and summary",
			"✓ [PROJ] Summary",
			ParsedTitle{Done: true, Project: "PROJ", Summary: "Summary"},
		},
		{
			"done and at-risk with project",
			"✓ ⚠️ [PROJ] Summary",
			ParsedTitle{Done: true, AtRisk: true, Project: "PROJ", Summary: "Summary"},
		},
		{
			"no done marker works as before",
			"[PROJ] Summary",
			ParsedTitle{Project: "PROJ", Summary: "Summary"},
		},
		{
			"done without project",
			"✓ Just a summary",
			ParsedTitle{Done: true, Summary: "Just a summary"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTitle(tt.title)
			if got != tt.want {
				t.Errorf("ParseTitle(%q) = %+v, want %+v", tt.title, got, tt.want)
			}
		})
	}
}

func TestBuildDoneTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"prepends marker", "[PROJ] Fix bug", "✓ [PROJ] Fix bug"},
		{"prepends to at-risk", "⚠️ [PROJ] Overdue", "✓ ⚠️ [PROJ] Overdue"},
		{"prepends to plain", "Summary", "✓ Summary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDoneTitle(tt.title)
			if got != tt.want {
				t.Errorf("BuildDoneTitle(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}

	t.Run("round-trip done marker", func(t *testing.T) {
		original := BuildTitleWithSection("PROJ", "", "Fix bug", false)
		done := BuildDoneTitle(original)
		parsed := ParseTitle(done)
		if !parsed.Done {
			t.Error("expected Done=true after BuildDoneTitle")
		}
		if parsed.Project != "PROJ" {
			t.Errorf("Project = %q, want PROJ", parsed.Project)
		}
		if parsed.Summary != "Fix bug" {
			t.Errorf("Summary = %q, want Fix bug", parsed.Summary)
		}
	})
}

func TestBuildDescription(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		project string
		want    string
	}{
		{"kendo key", "PROJ-123", "PROJ", "fylla: PROJ-123"},
		{"todoist numeric key", "123456", "", "fylla: 123456\nhttps://todoist.com/app/task/123456"},
		{"github PR key", "fylla#42", "iruoy/fylla", "fylla: fylla#42\nhttps://github.com/iruoy/fylla/pull/42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDescription(tt.key, tt.project)
			if got != tt.want {
				t.Errorf("BuildDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTaskKeyFromDescription(t *testing.T) {
	tests := []struct {
		desc string
		want string
	}{
		{"fylla: PROJ-123\nhttps://company.atlassian.net/browse/PROJ-123", "PROJ-123"},
		{"fylla: T-1\nhttps://todoist.com/browse/T-1", "T-1"},
		{"some random description", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("desc=%q", tt.desc), func(t *testing.T) {
			got := TaskKeyFromDescription(tt.desc)
			if got != tt.want {
				t.Errorf("TaskKeyFromDescription(%q) = %q, want %q", tt.desc, got, tt.want)
			}
		})
	}
}

func Test_GCAL010_FetchFyllaEvents(t *testing.T) {
	t.Run("returns only Fylla-managed events", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := googlecalendar.Events{
				Items: []*googlecalendar.Event{
					{Id: "f1", Summary: "Task one", Description: "fylla: T-1\nhttps://co.atlassian.net/browse/T-1", Start: &googlecalendar.EventDateTime{DateTime: "2025-01-20T09:00:00Z"}, End: &googlecalendar.EventDateTime{DateTime: "2025-01-20T10:00:00Z"}},
					{Id: "m1", Summary: "Meeting", Start: &googlecalendar.EventDateTime{DateTime: "2025-01-20T10:00:00Z"}, End: &googlecalendar.EventDateTime{DateTime: "2025-01-20T11:00:00Z"}},
					{Id: "f2", Summary: "[LATE] Late task", Description: "fylla: T-2\nhttps://co.atlassian.net/browse/T-2", Start: &googlecalendar.EventDateTime{DateTime: "2025-01-20T11:00:00Z"}, End: &googlecalendar.EventDateTime{DateTime: "2025-01-20T12:00:00Z"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := newTestClient(t, server, []string{"primary"}, "fylla")
		events, err := client.FetchFyllaEvents(context.Background(), time.Now(), time.Now().Add(24*time.Hour))
		if err != nil {
			t.Fatalf("FetchFyllaEvents: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("expected 2 Fylla events, got %d", len(events))
		}
		if events[0].ID != "f1" || events[1].ID != "f2" {
			t.Errorf("got IDs %s, %s, want f1, f2", events[0].ID, events[1].ID)
		}
	})

	t.Run("uses fylla calendar", func(t *testing.T) {
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

		client := newTestClient(t, server, []string{"source-cal"}, "my-fylla-cal")
		_, err := client.FetchFyllaEvents(context.Background(), time.Now(), time.Now().Add(24*time.Hour))
		if err != nil {
			t.Fatalf("FetchFyllaEvents: %v", err)
		}
		if calendarID != "my-fylla-cal" {
			t.Errorf("expected fylla calendar, got %q", calendarID)
		}
	})
}

func Test_GCAL011_UpdateEvent(t *testing.T) {
	t.Run("updates event with correct data", func(t *testing.T) {
		var updated googlecalendar.Event
		var method string
		var eventIDInPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method = r.Method
			parts := strings.Split(r.URL.Path, "/")
			eventIDInPath = parts[len(parts)-1]
			if r.Method == http.MethodPut {
				json.NewDecoder(r.Body).Decode(&updated)
				updated.Id = eventIDInPath
				json.NewEncoder(w).Encode(updated)
				return
			}
			json.NewEncoder(w).Encode(googlecalendar.Events{})
		}))
		defer server.Close()

		client := newTestClient(t, server, []string{"primary"}, "fylla")
		start := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 20, 11, 0, 0, 0, time.UTC)
		err := client.UpdateEvent(context.Background(), "evt-123", CreateEventInput{
			TaskKey: "PROJ-1",
			Summary: "Updated task",
			Start:   start,
			End:     end,
			AtRisk:  true,
		})
		if err != nil {
			t.Fatalf("UpdateEvent: %v", err)
		}
		if method != http.MethodPut {
			t.Errorf("expected PUT, got %s", method)
		}
		if eventIDInPath != "evt-123" {
			t.Errorf("expected event ID evt-123 in path, got %s", eventIDInPath)
		}
		if updated.Summary != "⚠️ Updated task" {
			t.Errorf("title = %q, want ⚠️ Updated task", updated.Summary)
		}
	})
}

func Test_GCAL012_DeleteEvent(t *testing.T) {
	t.Run("deletes single event by ID", func(t *testing.T) {
		var deletedID string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				parts := strings.Split(r.URL.Path, "/")
				deletedID = parts[len(parts)-1]
				w.WriteHeader(http.StatusNoContent)
				return
			}
			json.NewEncoder(w).Encode(googlecalendar.Events{})
		}))
		defer server.Close()

		client := newTestClient(t, server, []string{"primary"}, "fylla")
		err := client.DeleteEvent(context.Background(), "evt-to-delete")
		if err != nil {
			t.Fatalf("DeleteEvent: %v", err)
		}
		if deletedID != "evt-to-delete" {
			t.Errorf("deleted ID = %q, want evt-to-delete", deletedID)
		}
	})
}
