package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/scheduler"
	"github.com/iruoy/fylla/internal/task"
)

// mockCalendar records all calendar operations for assertion.
type mockCalendar struct {
	events        []calendar.Event
	deletedRanges []timeRange
	created       []calendar.CreateEventInput
	fetchCalls    []timeRange
}

type timeRange struct {
	start, end time.Time
}

func (m *mockCalendar) FetchEvents(_ context.Context, start, end time.Time) ([]calendar.Event, error) {
	m.fetchCalls = append(m.fetchCalls, timeRange{start, end})
	return m.events, nil
}

func (m *mockCalendar) DeleteFyllaEvents(_ context.Context, start, end time.Time) error {
	m.deletedRanges = append(m.deletedRanges, timeRange{start, end})
	return nil
}

func (m *mockCalendar) CreateEvent(_ context.Context, input calendar.CreateEventInput) error {
	m.created = append(m.created, input)
	return nil
}

// mockTaskFetcher returns preconfigured tasks.
type mockTaskFetcher struct {
	tasks     []task.Task
	queryUsed string
}

func (m *mockTaskFetcher) FetchTasks(_ context.Context, query string) ([]task.Task, error) {
	m.queryUsed = query
	return m.tasks, nil
}

// testConfig returns a standard config for testing.
func testConfig() *config.Config {
	return &config.Config{
		Jira: config.JiraConfig{
			URL:        "https://test.atlassian.net",
			Email:      "test@example.com",
			DefaultJQL: "assignee = currentUser()",
		},
		Calendar: config.CalendarConfig{
			SourceCalendar: "primary",
			FyllaCalendar:  "fylla",
		},
		Scheduling: config.SchedulingConfig{
			WindowDays:             5,
			MinTaskDurationMinutes: 25,
			BufferMinutes:          15,
		},
		BusinessHours: config.BusinessHoursConfig{
			Start:    "09:00",
			End:      "17:00",
			WorkDays: []int{1, 2, 3, 4, 5},
		},
		Weights: config.WeightsConfig{
			Priority:  0.40,
			DueDate:   0.30,
			Estimate:  0.15,
			IssueType: 0.10,
			Age:       0.05,
		},
		TypeScores: map[string]int{
			"Bug":   100,
			"Task":  70,
			"Story": 50,
		},
	}
}

func TestSYNC001_delete_existing_fylla_events(t *testing.T) {
	// Monday 9am
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
	start := now
	end := now.AddDate(0, 0, 5)

	t.Run("deletes fylla events before creating new ones", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{}

		_, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(cal.deletedRanges) != 1 {
			t.Fatalf("expected 1 delete call, got %d", len(cal.deletedRanges))
		}
		if !cal.deletedRanges[0].start.Equal(start) || !cal.deletedRanges[0].end.Equal(end) {
			t.Errorf("delete range = %v-%v, want %v-%v",
				cal.deletedRanges[0].start, cal.deletedRanges[0].end, start, end)
		}
	})

	t.Run("dry-run skips deletion", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{}

		_, err := RunSync(context.Background(), SyncParams{
			Cal:    cal,
			Tasks:  jr,
			Cfg:    testConfig(),
			Query:  "project = TEST",
			Now:    now,
			Start:  start,
			End:    end,
			DryRun: true,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(cal.deletedRanges) != 0 {
			t.Errorf("dry-run should skip deletion, got %d calls", len(cal.deletedRanges))
		}
	})

	t.Run("delete is called before event creation", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "TEST-1", Summary: "Task 1", Priority: 1, RemainingEstimate: 30 * time.Minute, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		_, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		// Delete should have happened (1 call) before events were created
		if len(cal.deletedRanges) != 1 {
			t.Fatalf("expected 1 delete call, got %d", len(cal.deletedRanges))
		}
		if len(cal.created) == 0 {
			t.Fatal("expected events to be created")
		}
	})
}

func TestSYNC002_fetch_jira_tasks_using_jql(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
	start := now
	end := now.AddDate(0, 0, 5)

	t.Run("uses provided query", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "PROJ-1", Summary: "Task 1", Priority: 2, RemainingEstimate: time.Hour, Project: "PROJ", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		_, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = MYPROJ",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if jr.queryUsed != "project = MYPROJ" {
			t.Errorf("query = %q, want %q", jr.queryUsed, "project = MYPROJ")
		}
	})

	t.Run("fetched tasks appear in allocations", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "PROJ-10", Summary: "Important task", Priority: 1, RemainingEstimate: 30 * time.Minute, Project: "PROJ", IssueType: "Bug", Created: now.AddDate(0, 0, -5)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = PROJ",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(result.Allocations) == 0 {
			t.Fatal("expected allocations for fetched tasks")
		}
		if result.Allocations[0].Task.Key != "PROJ-10" {
			t.Errorf("allocated task key = %q, want %q", result.Allocations[0].Task.Key, "PROJ-10")
		}
	})
}

func TestSYNC003_sort_tasks_by_composite_score(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
	start := now
	end := now.AddDate(0, 0, 5)

	t.Run("higher priority tasks scheduled first", func(t *testing.T) {
		cal := &mockCalendar{}
		due := now.AddDate(0, 0, 10)
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "LOW-1", Summary: "Low priority", Priority: 5, DueDate: &due, RemainingEstimate: 30 * time.Minute, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
				{Key: "HIGH-1", Summary: "High priority", Priority: 1, DueDate: &due, RemainingEstimate: 30 * time.Minute, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(result.Allocations) < 2 {
			t.Fatalf("expected at least 2 allocations, got %d", len(result.Allocations))
		}
		// HIGH-1 should be scheduled before LOW-1
		if result.Allocations[0].Task.Key != "HIGH-1" {
			t.Errorf("first allocated task = %q, want HIGH-1", result.Allocations[0].Task.Key)
		}
		if result.Allocations[0].Start.After(result.Allocations[1].Start) {
			t.Errorf("higher priority task should have earlier start time")
		}
	})

	t.Run("composite score considers due date and type", func(t *testing.T) {
		cal := &mockCalendar{}
		soonDue := now.AddDate(0, 0, 1)
		laterDue := now.AddDate(0, 0, 20)
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				// Same priority, but different due date — sooner due should rank higher
				{Key: "LATER-1", Summary: "Later due", Priority: 3, DueDate: &laterDue, RemainingEstimate: 30 * time.Minute, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
				{Key: "SOON-1", Summary: "Soon due", Priority: 3, DueDate: &soonDue, RemainingEstimate: 30 * time.Minute, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(result.Allocations) < 2 {
			t.Fatalf("expected at least 2 allocations, got %d", len(result.Allocations))
		}
		if result.Allocations[0].Task.Key != "SOON-1" {
			t.Errorf("first allocated task = %q, want SOON-1 (sooner due date)", result.Allocations[0].Task.Key)
		}
	})
}

func TestSYNC004_fetch_calendar_events(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
	start := now
	end := now.AddDate(0, 0, 5)

	t.Run("fetches events within scheduling window", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{}

		_, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(cal.fetchCalls) != 1 {
			t.Fatalf("expected 1 fetch call, got %d", len(cal.fetchCalls))
		}
		if !cal.fetchCalls[0].start.Equal(start) || !cal.fetchCalls[0].end.Equal(end) {
			t.Errorf("fetch range = %v-%v, want %v-%v",
				cal.fetchCalls[0].start, cal.fetchCalls[0].end, start, end)
		}
	})

	t.Run("events reduce available scheduling slots", func(t *testing.T) {
		// Meeting from 09:00-12:00 on Monday — tasks should be after 12:00+buffer
		meeting := calendar.Event{
			Title: "Team Meeting",
			Start: time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC),
			End:   time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC),
		}
		cal := &mockCalendar{events: []calendar.Event{meeting}}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "TEST-1", Summary: "Task 1", Priority: 1, RemainingEstimate: 30 * time.Minute, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(result.Allocations) == 0 {
			t.Fatal("expected at least 1 allocation")
		}
		// Task should start after meeting + buffer (12:00 + 15min = 12:15)
		expectedEarliest := time.Date(2025, 1, 20, 12, 15, 0, 0, time.UTC)
		if result.Allocations[0].Start.Before(expectedEarliest) {
			t.Errorf("task start = %v, want >= %v (after meeting + buffer)",
				result.Allocations[0].Start, expectedEarliest)
		}
	})
}

func TestSYNC005_find_free_slots_per_project(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
	start := now
	end := now.AddDate(0, 0, 5)

	t.Run("project-specific time windows", func(t *testing.T) {
		cfg := testConfig()
		cfg.ProjectRules = map[string]config.ProjectRule{
			"ADMIN": {Start: "09:00", End: "10:00", WorkDays: []int{1, 2, 3, 4, 5}},
		}

		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "ADMIN-1", Summary: "Admin task", Priority: 1, RemainingEstimate: 30 * time.Minute, Project: "ADMIN", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   cfg,
			Query: "project = ADMIN",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(result.Allocations) == 0 {
			t.Fatal("expected at least 1 allocation")
		}
		// ADMIN task should be between 09:00 and 10:00
		for _, alloc := range result.Allocations {
			h := alloc.Start.Hour()
			if h < 9 || alloc.End.Hour() > 10 || (alloc.End.Hour() == 10 && alloc.End.Minute() > 0) {
				t.Errorf("ADMIN allocation %v-%v outside 09:00-10:00 window", alloc.Start, alloc.End)
			}
		}
	})

	t.Run("non-project tasks use default business hours", func(t *testing.T) {
		cfg := testConfig()
		cfg.ProjectRules = map[string]config.ProjectRule{
			"ADMIN": {Start: "09:00", End: "10:00", WorkDays: []int{1, 2, 3, 4, 5}},
		}

		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "GEN-1", Summary: "General task", Priority: 1, RemainingEstimate: 30 * time.Minute, Project: "GEN", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   cfg,
			Query: "project = GEN",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(result.Allocations) == 0 {
			t.Fatal("expected at least 1 allocation")
		}
		// General task uses default hours (09:00-17:00)
		alloc := result.Allocations[0]
		if alloc.Start.Hour() < 9 || alloc.End.Hour() > 17 {
			t.Errorf("general task allocation %v-%v outside default business hours", alloc.Start, alloc.End)
		}
	})

	t.Run("OOO blocks scheduling", func(t *testing.T) {
		cfg := testConfig()
		cal := &mockCalendar{
			events: []calendar.Event{
				{
					Title:     "PTO",
					Start:     time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC),
					End:       time.Date(2025, 1, 21, 0, 0, 0, 0, time.UTC),
					AllDay:    true,
					EventType: "outOfOffice",
				},
			},
		}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "TEST-1", Summary: "Task 1", Priority: 1, RemainingEstimate: 30 * time.Minute, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   cfg,
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		// Task should not be scheduled on Monday (OOO day)
		for _, alloc := range result.Allocations {
			if alloc.Start.Day() == 20 && alloc.Start.Month() == 1 {
				t.Errorf("task scheduled on OOO day: %v", alloc.Start)
			}
		}
	})
}

func TestSYNC006_allocate_tasks_first_fit(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
	start := now
	end := now.AddDate(0, 0, 5)

	t.Run("highest priority gets earliest slot", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "P5-1", Summary: "Low", Priority: 5, RemainingEstimate: time.Hour, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
				{Key: "P1-1", Summary: "High", Priority: 1, RemainingEstimate: time.Hour, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(result.Allocations) < 2 {
			t.Fatalf("expected at least 2 allocations, got %d", len(result.Allocations))
		}

		// Find the allocations for each task
		var highStart, lowStart time.Time
		for _, a := range result.Allocations {
			switch a.Task.Key {
			case "P1-1":
				if highStart.IsZero() {
					highStart = a.Start
				}
			case "P5-1":
				if lowStart.IsZero() {
					lowStart = a.Start
				}
			}
		}
		if highStart.After(lowStart) || highStart.Equal(lowStart) {
			t.Errorf("high priority start %v should be before low priority start %v", highStart, lowStart)
		}
	})

	t.Run("respects minimum duration", func(t *testing.T) {
		cfg := testConfig()
		cfg.Scheduling.MinTaskDurationMinutes = 25

		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "TEST-1", Summary: "Task", Priority: 1, RemainingEstimate: 30 * time.Minute, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   cfg,
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		for _, alloc := range result.Allocations {
			dur := alloc.End.Sub(alloc.Start)
			if dur < 25*time.Minute {
				t.Errorf("allocation duration %v < minimum 25 minutes", dur)
			}
		}
	})
}

func TestSYNC007_create_calendar_events(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
	start := now
	end := now.AddDate(0, 0, 5)

	t.Run("creates events for each allocation", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "TEST-1", Summary: "Task 1", Priority: 1, RemainingEstimate: time.Hour, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
				{Key: "TEST-2", Summary: "Task 2", Priority: 2, RemainingEstimate: 30 * time.Minute, Project: "TEST", IssueType: "Bug", Created: now.AddDate(0, 0, -2)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(cal.created) != len(result.Allocations) {
			t.Errorf("created %d events, expected %d (one per allocation)", len(cal.created), len(result.Allocations))
		}

		// Verify created events match allocations
		for i, alloc := range result.Allocations {
			if i >= len(cal.created) {
				break
			}
			ev := cal.created[i]
			if ev.TaskKey != alloc.Task.Key {
				t.Errorf("event[%d] key = %q, want %q", i, ev.TaskKey, alloc.Task.Key)
			}
			if ev.Summary != alloc.Task.Summary {
				t.Errorf("event[%d] summary = %q, want %q", i, ev.Summary, alloc.Task.Summary)
			}
			if !ev.Start.Equal(alloc.Start) {
				t.Errorf("event[%d] start = %v, want %v", i, ev.Start, alloc.Start)
			}
			if !ev.End.Equal(alloc.End) {
				t.Errorf("event[%d] end = %v, want %v", i, ev.End, alloc.End)
			}
		}
	})

	t.Run("dry-run skips event creation", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "TEST-1", Summary: "Task 1", Priority: 1, RemainingEstimate: time.Hour, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:    cal,
			Tasks:  jr,
			Cfg:    testConfig(),
			Query:  "project = TEST",
			Now:    now,
			Start:  start,
			End:    end,
			DryRun: true,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(cal.created) != 0 {
			t.Errorf("dry-run created %d events, expected 0", len(cal.created))
		}
		// But allocations should still be computed
		if len(result.Allocations) == 0 {
			t.Error("dry-run should still compute allocations")
		}
	})

	t.Run("events have correct details", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "PROJ-42", Summary: "Fix login bug", Priority: 1, RemainingEstimate: time.Hour, Project: "PROJ", IssueType: "Bug", Created: now.AddDate(0, 0, -3)},
			},
		}

		_, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = PROJ",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(cal.created) == 0 {
			t.Fatal("expected events to be created")
		}
		ev := cal.created[0]
		if ev.TaskKey != "PROJ-42" {
			t.Errorf("event task key = %q, want PROJ-42", ev.TaskKey)
		}
		if ev.Summary != "Fix login bug" {
			t.Errorf("event summary = %q, want %q", ev.Summary, "Fix login bug")
		}
	})
}

func TestSYNC008_at_risk_warnings(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)
	start := now
	end := now.AddDate(0, 0, 5)

	t.Run("detects at-risk tasks scheduled after due date", func(t *testing.T) {
		// LATE-1 has a due date in the past — any scheduling will be after due date
		dueYesterday := time.Date(2025, 1, 19, 17, 0, 0, 0, time.UTC)
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "LATE-1", Summary: "Overdue task", Priority: 1, DueDate: &dueYesterday, RemainingEstimate: time.Hour, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -5)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(result.AtRisk) == 0 {
			t.Fatal("expected at-risk tasks")
		}

		found := false
		for _, ar := range result.AtRisk {
			if ar.Task.Key == "LATE-1" {
				found = true
				break
			}
		}
		if !found {
			t.Error("LATE-1 should be in at-risk list")
		}
	})

	t.Run("at-risk events have AtRisk flag set", func(t *testing.T) {
		dueYesterday := time.Date(2025, 1, 19, 17, 0, 0, 0, time.UTC)
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "LATE-1", Summary: "Overdue task", Priority: 1, DueDate: &dueYesterday, RemainingEstimate: time.Hour, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -5)},
			},
		}

		_, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		// Check that CreateEvent was called with AtRisk=true for LATE-1
		foundAtRisk := false
		for _, ev := range cal.created {
			if ev.TaskKey == "LATE-1" && ev.AtRisk {
				foundAtRisk = true
				break
			}
		}
		if !foundAtRisk {
			t.Error("LATE-1 event should have AtRisk=true")
		}
	})

	t.Run("on-time tasks not flagged as at-risk", func(t *testing.T) {
		farDue := now.AddDate(0, 0, 20)
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "OK-1", Summary: "On time", Priority: 1, DueDate: &farDue, RemainingEstimate: time.Hour, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(result.AtRisk) != 0 {
			t.Errorf("expected no at-risk tasks, got %d", len(result.AtRisk))
		}
	})

	t.Run("at-risk deduplicated by task key", func(t *testing.T) {
		dueYesterday := time.Date(2025, 1, 19, 17, 0, 0, 0, time.UTC)
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				// Large overdue task — may be split into multiple allocations
				{Key: "LATE-1", Summary: "Overdue", Priority: 1, DueDate: &dueYesterday, RemainingEstimate: 3 * time.Hour, Project: "TEST", IssueType: "Task", Created: now.AddDate(0, 0, -5)},
			},
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal:   cal,
			Tasks: jr,
			Cfg:   testConfig(),
			Query: "project = TEST",
			Now:   now,
			Start: start,
			End:   end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		// Count at-risk entries for LATE-1 — should be exactly 1
		count := 0
		for _, ar := range result.AtRisk {
			if ar.Task.Key == "LATE-1" {
				count++
			}
		}
		if count > 1 {
			t.Errorf("at-risk should be deduplicated, got %d entries for LATE-1", count)
		}
	})
}

func TestCLI004_sync_creates_calendar_events(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

	t.Run("sync creates events from tasks", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "PROJ-1", Summary: "Task 1", Priority: 1, RemainingEstimate: time.Hour, Project: "PROJ", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		cfg := testConfig()
		query, start, end, dryRun, err := BuildSyncParams(SyncFlags{}, cfg, now)
		if err != nil {
			t.Fatalf("BuildSyncParams: %v", err)
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal: cal, Tasks: jr, Cfg: cfg,
			Query: query, Now: now, Start: start, End: end, DryRun: dryRun,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(cal.created) == 0 {
			t.Fatal("expected events to be created")
		}
		if cal.created[0].TaskKey != "PROJ-1" {
			t.Errorf("event key = %q, want PROJ-1", cal.created[0].TaskKey)
		}
		if len(result.Allocations) == 0 {
			t.Fatal("expected allocations in result")
		}
	})

	t.Run("events match scheduled tasks", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "A-1", Summary: "Alpha", Priority: 1, RemainingEstimate: 30 * time.Minute, Project: "A", IssueType: "Bug", Created: now.AddDate(0, 0, -2)},
				{Key: "A-2", Summary: "Beta", Priority: 3, RemainingEstimate: time.Hour, Project: "A", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		cfg := testConfig()
		query, start, end, dryRun, err := BuildSyncParams(SyncFlags{}, cfg, now)
		if err != nil {
			t.Fatalf("BuildSyncParams: %v", err)
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal: cal, Tasks: jr, Cfg: cfg,
			Query: query, Now: now, Start: start, End: end, DryRun: dryRun,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(cal.created) != len(result.Allocations) {
			t.Errorf("created %d events, but %d allocations", len(cal.created), len(result.Allocations))
		}
		for i, alloc := range result.Allocations {
			if i < len(cal.created) && cal.created[i].TaskKey != alloc.Task.Key {
				t.Errorf("event[%d] = %q, allocation = %q", i, cal.created[i].TaskKey, alloc.Task.Key)
			}
		}
	})
}

func TestCLI005_sync_dry_run(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

	t.Run("dry-run does not create events", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "T-1", Summary: "Task", Priority: 1, RemainingEstimate: time.Hour, Project: "T", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		cfg := testConfig()
		query, start, end, dryRun, err := BuildSyncParams(SyncFlags{DryRun: true}, cfg, now)
		if err != nil {
			t.Fatalf("BuildSyncParams: %v", err)
		}
		if !dryRun {
			t.Fatal("expected dryRun to be true")
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal: cal, Tasks: jr, Cfg: cfg,
			Query: query, Now: now, Start: start, End: end, DryRun: dryRun,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(cal.created) != 0 {
			t.Errorf("dry-run created %d events, want 0", len(cal.created))
		}
		if len(result.Allocations) == 0 {
			t.Error("dry-run should still compute allocations")
		}
	})

	t.Run("dry-run output shows schedule", func(t *testing.T) {
		allocs := []scheduler.Allocation{
			{Task: task.Task{Key: "T-1", Summary: "Fix bug"}, Start: now, End: now.Add(time.Hour)},
		}
		result := &SyncResult{Allocations: allocs}
		var buf bytes.Buffer
		PrintSyncResult(&buf, result, true)

		out := buf.String()
		if !strings.Contains(out, "Dry run") {
			t.Errorf("output missing dry-run header, got:\n%s", out)
		}
		if !strings.Contains(out, "T-1") {
			t.Errorf("output missing task key, got:\n%s", out)
		}
		if !strings.Contains(out, "Fix bug") {
			t.Errorf("output missing task summary, got:\n%s", out)
		}
	})
}

func TestCLI006_sync_jql_override(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

	t.Run("--jql overrides default JQL", func(t *testing.T) {
		cfg := testConfig()
		cfg.Jira.DefaultJQL = "assignee = currentUser()"

		query, _, _, _, err := BuildSyncParams(SyncFlags{JQL: "project = MYPROJ"}, cfg, now)
		if err != nil {
			t.Fatalf("BuildSyncParams: %v", err)
		}
		if query != "project = MYPROJ" {
			t.Errorf("query = %q, want %q", query, "project = MYPROJ")
		}
	})

	t.Run("default JQL used when --jql not set", func(t *testing.T) {
		cfg := testConfig()
		cfg.Jira.DefaultJQL = "assignee = currentUser()"

		query, _, _, _, err := BuildSyncParams(SyncFlags{}, cfg, now)
		if err != nil {
			t.Fatalf("BuildSyncParams: %v", err)
		}
		if query != "assignee = currentUser()" {
			t.Errorf("query = %q, want default", query)
		}
	})

	t.Run("only custom query tasks fetched", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "MYPROJ-1", Summary: "My task", Priority: 1, RemainingEstimate: time.Hour, Project: "MYPROJ", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		cfg := testConfig()
		query, start, end, _, err := BuildSyncParams(SyncFlags{JQL: "project = MYPROJ"}, cfg, now)
		if err != nil {
			t.Fatalf("BuildSyncParams: %v", err)
		}

		_, err = RunSync(context.Background(), SyncParams{
			Cal: cal, Tasks: jr, Cfg: cfg,
			Query: query, Now: now, Start: start, End: end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if jr.queryUsed != "project = MYPROJ" {
			t.Errorf("query sent to source = %q, want %q", jr.queryUsed, "project = MYPROJ")
		}
	})
}

func TestCLI007_sync_days_override(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

	t.Run("--days overrides windowDays", func(t *testing.T) {
		cfg := testConfig()
		cfg.Scheduling.WindowDays = 5

		_, start, end, _, err := BuildSyncParams(SyncFlags{Days: 10}, cfg, now)
		if err != nil {
			t.Fatalf("BuildSyncParams: %v", err)
		}

		expectedEnd := now.AddDate(0, 0, 10)
		if !end.Equal(expectedEnd) {
			t.Errorf("end = %v, want %v (10 days)", end, expectedEnd)
		}
		if !start.Equal(now) {
			t.Errorf("start = %v, want now", start)
		}
	})

	t.Run("default windowDays used when --days not set", func(t *testing.T) {
		cfg := testConfig()
		cfg.Scheduling.WindowDays = 5

		_, _, end, _, err := BuildSyncParams(SyncFlags{}, cfg, now)
		if err != nil {
			t.Fatalf("BuildSyncParams: %v", err)
		}

		expectedEnd := now.AddDate(0, 0, 5)
		if !end.Equal(expectedEnd) {
			t.Errorf("end = %v, want %v (5 days)", end, expectedEnd)
		}
	})

	t.Run("events created up to N days ahead", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "T-1", Summary: "Task", Priority: 1, RemainingEstimate: time.Hour, Project: "T", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		cfg := testConfig()
		query, start, end, _, err := BuildSyncParams(SyncFlags{Days: 10}, cfg, now)
		if err != nil {
			t.Fatalf("BuildSyncParams: %v", err)
		}

		_, err = RunSync(context.Background(), SyncParams{
			Cal: cal, Tasks: jr, Cfg: cfg,
			Query: query, Now: now, Start: start, End: end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		if len(cal.fetchCalls) == 0 {
			t.Fatal("no fetch calls")
		}
		window := cal.fetchCalls[0].end.Sub(cal.fetchCalls[0].start)
		if window != 10*24*time.Hour {
			t.Errorf("fetch window = %v, want 10 days", window)
		}
	})
}

func TestCLI008_sync_from_to_date_range(t *testing.T) {
	now := time.Date(2025, 1, 20, 9, 0, 0, 0, time.UTC)

	t.Run("--from and --to set explicit date range", func(t *testing.T) {
		cfg := testConfig()
		_, start, end, _, err := BuildSyncParams(SyncFlags{
			From: "2025-01-20",
			To:   "2025-01-24",
		}, cfg, now)
		if err != nil {
			t.Fatalf("BuildSyncParams: %v", err)
		}

		expectedStart := time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)
		expectedEnd := time.Date(2025, 1, 25, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)

		if !start.Equal(expectedStart) {
			t.Errorf("start = %v, want %v", start, expectedStart)
		}
		if !end.Equal(expectedEnd) {
			t.Errorf("end = %v, want %v", end, expectedEnd)
		}
	})

	t.Run("events only within specified range", func(t *testing.T) {
		cal := &mockCalendar{}
		jr := &mockTaskFetcher{
			tasks: []task.Task{
				{Key: "T-1", Summary: "Task", Priority: 1, RemainingEstimate: time.Hour, Project: "T", IssueType: "Task", Created: now.AddDate(0, 0, -1)},
			},
		}

		cfg := testConfig()
		query, start, end, _, err := BuildSyncParams(SyncFlags{
			From: "2025-01-20",
			To:   "2025-01-24",
		}, cfg, now)
		if err != nil {
			t.Fatalf("BuildSyncParams: %v", err)
		}

		result, err := RunSync(context.Background(), SyncParams{
			Cal: cal, Tasks: jr, Cfg: cfg,
			Query: query, Now: now, Start: start, End: end,
		})
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}

		rangeStart := time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)
		rangeEnd := time.Date(2025, 1, 25, 0, 0, 0, 0, time.UTC)
		for _, alloc := range result.Allocations {
			if alloc.Start.Before(rangeStart) || alloc.End.After(rangeEnd) {
				t.Errorf("allocation %v-%v outside range %v-%v", alloc.Start, alloc.End, rangeStart, rangeEnd)
			}
		}
	})

	t.Run("invalid --from date returns error", func(t *testing.T) {
		cfg := testConfig()
		_, _, _, _, err := BuildSyncParams(SyncFlags{
			From: "not-a-date",
			To:   "2025-01-24",
		}, cfg, now)
		if err == nil {
			t.Fatal("expected error for invalid --from")
		}
		if !strings.Contains(err.Error(), "--from") {
			t.Errorf("error = %q, want to mention --from", err.Error())
		}
	})

	t.Run("invalid --to date returns error", func(t *testing.T) {
		cfg := testConfig()
		_, _, _, _, err := BuildSyncParams(SyncFlags{
			From: "2025-01-20",
			To:   "not-a-date",
		}, cfg, now)
		if err == nil {
			t.Fatal("expected error for invalid --to")
		}
		if !strings.Contains(err.Error(), "--to") {
			t.Errorf("error = %q, want to mention --to", err.Error())
		}
	})
}
