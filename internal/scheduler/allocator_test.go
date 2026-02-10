package scheduler

import (
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/jira"
)

func date(year int, month time.Month, day, hour, min int) time.Time {
	return time.Date(year, month, day, hour, min, 0, 0, time.UTC)
}

func datePtr(year int, month time.Month, day, hour, min int) *time.Time {
	t := date(year, month, day, hour, min)
	return &t
}

func Test_ALLOC001_first_fit_highest_priority_first(t *testing.T) {
	// Two tasks with different priorities; highest priority gets earliest slot.
	tasks := []ScoredTask{
		{
			Task: jira.Task{
				Key:               "HIGH-1",
				Summary:           "High priority task",
				Priority:          1,
				RemainingEstimate: 1 * time.Hour,
				Project:           "PROJ",
			},
			Score: 90,
		},
		{
			Task: jira.Task{
				Key:               "LOW-2",
				Summary:           "Low priority task",
				Priority:          5,
				RemainingEstimate: 1 * time.Hour,
				Project:           "PROJ",
			},
			Score: 30,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 12, 0)},
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 2 {
		t.Fatalf("expected 2 allocations, got %d", len(result))
	}

	// Highest priority task gets the earliest slot
	if result[0].Task.Key != "HIGH-1" {
		t.Errorf("expected first allocation to be HIGH-1, got %s", result[0].Task.Key)
	}
	if result[0].Start != date(2025, 1, 20, 9, 0) {
		t.Errorf("expected HIGH-1 to start at 09:00, got %v", result[0].Start)
	}

	// Low priority task gets the next available time
	if result[1].Task.Key != "LOW-2" {
		t.Errorf("expected second allocation to be LOW-2, got %s", result[1].Task.Key)
	}
	if !result[1].Start.After(result[0].End) && !result[1].Start.Equal(result[0].End) {
		t.Errorf("expected LOW-2 to start after HIGH-1 ends")
	}
}

func Test_ALLOC001_three_tasks_ordered(t *testing.T) {
	tasks := []ScoredTask{
		{Task: jira.Task{Key: "A", RemainingEstimate: 30 * time.Minute, Project: "P"}, Score: 100},
		{Task: jira.Task{Key: "B", RemainingEstimate: 30 * time.Minute, Project: "P"}, Score: 80},
		{Task: jira.Task{Key: "C", RemainingEstimate: 30 * time.Minute, Project: "P"}, Score: 60},
	}

	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 12, 0)},
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 3 {
		t.Fatalf("expected 3 allocations, got %d", len(result))
	}

	for i := 1; i < len(result); i++ {
		if result[i].Start.Before(result[i-1].End) {
			t.Errorf("allocation %d starts before allocation %d ends", i, i-1)
		}
	}

	if result[0].Task.Key != "A" || result[1].Task.Key != "B" || result[2].Task.Key != "C" {
		t.Errorf("expected order A, B, C; got %s, %s, %s", result[0].Task.Key, result[1].Task.Key, result[2].Task.Key)
	}
}

func Test_ALLOC002_project_filtering(t *testing.T) {
	// ADMIN task should use ADMIN slots, general task should use default slots.
	tasks := []ScoredTask{
		{
			Task: jira.Task{
				Key:               "ADMIN-1",
				Summary:           "Admin task",
				RemainingEstimate: 30 * time.Minute,
				Project:           "ADMIN",
			},
			Score: 80,
		},
		{
			Task: jira.Task{
				Key:               "PROJ-2",
				Summary:           "General task",
				RemainingEstimate: 1 * time.Hour,
				Project:           "PROJ",
			},
			Score: 70,
		},
	}

	// ADMIN gets morning-only window, default gets full day
	slots := map[string][]calendar.Slot{
		"ADMIN": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 10, 0)},
		},
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 17, 0)},
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 2 {
		t.Fatalf("expected 2 allocations, got %d", len(result))
	}

	// ADMIN task should be in the 09:00-10:00 window
	adminAlloc := result[0]
	if adminAlloc.Task.Key != "ADMIN-1" {
		t.Errorf("expected first allocation to be ADMIN-1, got %s", adminAlloc.Task.Key)
	}
	if adminAlloc.Start.Before(date(2025, 1, 20, 9, 0)) || adminAlloc.End.After(date(2025, 1, 20, 10, 0)) {
		t.Errorf("ADMIN task should be within 09:00-10:00, got %v-%v", adminAlloc.Start, adminAlloc.End)
	}

	// General task should use default slots; the ADMIN slot time is consumed,
	// so it starts after 09:30 (ADMIN took 09:00-09:30)
	projAlloc := result[1]
	if projAlloc.Task.Key != "PROJ-2" {
		t.Errorf("expected second allocation to be PROJ-2, got %s", projAlloc.Task.Key)
	}
	if projAlloc.Start.Before(adminAlloc.End) {
		t.Errorf("PROJ task should start after ADMIN task ends; ADMIN ends %v, PROJ starts %v", adminAlloc.End, projAlloc.Start)
	}
}

func Test_ALLOC002_project_falls_back_to_default(t *testing.T) {
	tasks := []ScoredTask{
		{
			Task:  jira.Task{Key: "UNKNOWN-1", RemainingEstimate: 30 * time.Minute, Project: "UNKNOWN"},
			Score: 80,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 17, 0)},
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(result))
	}
	if result[0].Start != date(2025, 1, 20, 9, 0) {
		t.Errorf("expected task to use default slots, start at 09:00, got %v", result[0].Start)
	}
}

func Test_ALLOC003_default_estimate_one_hour(t *testing.T) {
	// Task without an estimate should default to 1 hour.
	tasks := []ScoredTask{
		{
			Task: jira.Task{
				Key:               "PROJ-1",
				Summary:           "No estimate task",
				RemainingEstimate: 0, // no estimate
				Project:           "PROJ",
			},
			Score: 80,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 17, 0)},
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(result))
	}

	duration := result[0].End.Sub(result[0].Start)
	if duration != 1*time.Hour {
		t.Errorf("expected 1 hour duration for task without estimate, got %v", duration)
	}
}

func Test_ALLOC003_negative_estimate_defaults(t *testing.T) {
	tasks := []ScoredTask{
		{
			Task:  jira.Task{Key: "X-1", RemainingEstimate: -5 * time.Minute, Project: "P"},
			Score: 50,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 17, 0)}},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(result))
	}
	if dur := result[0].End.Sub(result[0].Start); dur != time.Hour {
		t.Errorf("expected 1h default, got %v", dur)
	}
}

func Test_ALLOC004_minimum_duration_skips_tiny_slots(t *testing.T) {
	// 20-minute slot should be skipped when minTaskDurationMinutes is 25.
	tasks := []ScoredTask{
		{
			Task: jira.Task{
				Key:               "PROJ-1",
				Summary:           "Task",
				RemainingEstimate: 15 * time.Minute,
				Project:           "PROJ",
			},
			Score: 80,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {
			// Only a 20-minute slot available
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 9, 20)},
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 0 {
		t.Fatalf("expected 0 allocations (slot too small), got %d", len(result))
	}
}

func Test_ALLOC004_slot_at_exactly_minimum(t *testing.T) {
	tasks := []ScoredTask{
		{
			Task:  jira.Task{Key: "P-1", RemainingEstimate: 25 * time.Minute, Project: "P"},
			Score: 80,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 9, 25)},
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 1 {
		t.Fatalf("expected 1 allocation (slot equals minimum), got %d", len(result))
	}
}

func Test_ALLOC005_splitting_remainder_below_minimum(t *testing.T) {
	// 60-minute task, 45-minute slot, 25-minute minimum.
	// Remainder would be 15 min < 25 min minimum → task moves to next slot.
	tasks := []ScoredTask{
		{
			Task: jira.Task{
				Key:               "PROJ-1",
				Summary:           "Big task",
				RemainingEstimate: 60 * time.Minute,
				Project:           "PROJ",
			},
			Score: 80,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 9, 45)},  // 45 min
			{Start: date(2025, 1, 20, 10, 0), End: date(2025, 1, 20, 12, 0)}, // 2 hours
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(result))
	}

	// Task should skip the 45-min slot and go to the 2-hour slot
	if result[0].Start != date(2025, 1, 20, 10, 0) {
		t.Errorf("expected task to start at 10:00 (second slot), got %v", result[0].Start)
	}
	if result[0].End != date(2025, 1, 20, 11, 0) {
		t.Errorf("expected task to end at 11:00, got %v", result[0].End)
	}
}

func Test_ALLOC005_splitting_remainder_above_minimum(t *testing.T) {
	// 90-minute task, 45-minute slot, 25-minute minimum.
	// Remainder would be 45 min >= 25 min → split is allowed.
	tasks := []ScoredTask{
		{
			Task: jira.Task{
				Key:               "PROJ-1",
				Summary:           "Splittable task",
				RemainingEstimate: 90 * time.Minute,
				Project:           "PROJ",
			},
			Score: 80,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 9, 45)},   // 45 min
			{Start: date(2025, 1, 20, 10, 0), End: date(2025, 1, 20, 12, 0)},  // 2 hours
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 2 {
		t.Fatalf("expected 2 allocations (split), got %d", len(result))
	}

	// First part uses the 45-min slot entirely
	if result[0].Start != date(2025, 1, 20, 9, 0) || result[0].End != date(2025, 1, 20, 9, 45) {
		t.Errorf("expected first part at 09:00-09:45, got %v-%v", result[0].Start, result[0].End)
	}

	// Second part (45 min remaining) goes to the next slot
	if result[1].Start != date(2025, 1, 20, 10, 0) || result[1].End != date(2025, 1, 20, 10, 45) {
		t.Errorf("expected second part at 10:00-10:45, got %v-%v", result[1].Start, result[1].End)
	}
}

func Test_ALLOC006_at_risk_detection(t *testing.T) {
	// Task due tomorrow, but scheduled after due date due to higher-priority tasks.
	dueDate := date(2025, 1, 21, 17, 0) // Due Jan 21 EOD

	tasks := []ScoredTask{
		{
			Task: jira.Task{
				Key:               "URGENT-1",
				Summary:           "Fills all of today and tomorrow",
				RemainingEstimate: 16 * time.Hour,
				Project:           "PROJ",
			},
			Score: 100,
		},
		{
			Task: jira.Task{
				Key:               "LATE-1",
				Summary:           "Will be late",
				DueDate:           &dueDate,
				RemainingEstimate: 2 * time.Hour,
				Project:           "PROJ",
			},
			Score: 50,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 17, 0)}, // Jan 20
			{Start: date(2025, 1, 21, 9, 0), End: date(2025, 1, 21, 17, 0)}, // Jan 21
			{Start: date(2025, 1, 22, 9, 0), End: date(2025, 1, 22, 17, 0)}, // Jan 22
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	// Find the LATE-1 allocation
	var lateAlloc *Allocation
	for i := range result {
		if result[i].Task.Key == "LATE-1" {
			lateAlloc = &result[i]
			break
		}
	}

	if lateAlloc == nil {
		t.Fatal("LATE-1 task was not allocated")
	}

	if !lateAlloc.AtRisk {
		t.Error("expected LATE-1 to be marked as at-risk")
	}

	// It should be scheduled on Jan 22 (after due date Jan 21)
	if !lateAlloc.Start.After(dueDate) {
		t.Errorf("expected LATE-1 to be scheduled after due date %v, got start %v", dueDate, lateAlloc.Start)
	}
}

func Test_ALLOC006_not_at_risk_when_before_due(t *testing.T) {
	dueDate := date(2025, 1, 25, 17, 0)

	tasks := []ScoredTask{
		{
			Task: jira.Task{
				Key:               "OK-1",
				Summary:           "On time task",
				DueDate:           &dueDate,
				RemainingEstimate: 1 * time.Hour,
				Project:           "PROJ",
			},
			Score: 80,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 17, 0)},
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(result))
	}

	if result[0].AtRisk {
		t.Error("expected task to NOT be at-risk when scheduled before due date")
	}
}

func Test_ALLOC006_no_due_date_not_at_risk(t *testing.T) {
	tasks := []ScoredTask{
		{
			Task: jira.Task{
				Key:               "NODUE-1",
				RemainingEstimate: 1 * time.Hour,
				Project:           "PROJ",
			},
			Score: 80,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 17, 0)}},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(result))
	}
	if result[0].AtRisk {
		t.Error("task without due date should not be at-risk")
	}
}

func Test_ALLOC007_at_risk_late_prefix(t *testing.T) {
	// At-risk tasks should have AtRisk=true, which the calendar layer uses for [LATE] prefix.
	dueDate := date(2025, 1, 20, 12, 0)

	tasks := []ScoredTask{
		{
			Task: jira.Task{
				Key:               "BLOCKER-1",
				Summary:           "Takes all morning",
				RemainingEstimate: 4 * time.Hour,
				Project:           "PROJ",
			},
			Score: 100,
		},
		{
			Task: jira.Task{
				Key:               "ATRISK-1",
				Summary:           "Due at noon but scheduled after",
				DueDate:           &dueDate,
				RemainingEstimate: 1 * time.Hour,
				Project:           "PROJ",
			},
			Score: 50,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 17, 0)},
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	var atRiskAlloc *Allocation
	for i := range result {
		if result[i].Task.Key == "ATRISK-1" {
			atRiskAlloc = &result[i]
			break
		}
	}

	if atRiskAlloc == nil {
		t.Fatal("ATRISK-1 task was not allocated")
	}

	if !atRiskAlloc.AtRisk {
		t.Error("expected ATRISK-1 to have AtRisk=true for [LATE] prefix")
	}

	// Verify the calendar layer would use this: CreateEventInput.AtRisk → [LATE] prefix
	// This is verified by the AtRisk field being true, which CreateEvent already handles.
	if atRiskAlloc.Start.Before(date(2025, 1, 20, 13, 0)) {
		t.Errorf("expected ATRISK-1 to start after BLOCKER-1 ends at 13:00, got %v", atRiskAlloc.Start)
	}
}

func Test_ALLOC007_split_task_all_parts_at_risk(t *testing.T) {
	// When a split task is at-risk, all parts should be marked.
	dueDate := date(2025, 1, 20, 10, 0) // Due at 10:00

	tasks := []ScoredTask{
		{
			Task: jira.Task{
				Key:               "SPLIT-1",
				Summary:           "Large task that splits and is late",
				DueDate:           &dueDate,
				RemainingEstimate: 3 * time.Hour,
				Project:           "PROJ",
			},
			Score: 80,
		},
	}

	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 14, 0), End: date(2025, 1, 20, 15, 30)}, // 1.5 hours
			{Start: date(2025, 1, 20, 16, 0), End: date(2025, 1, 20, 17, 30)}, // 1.5 hours
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 2 {
		t.Fatalf("expected 2 allocations (split), got %d", len(result))
	}

	for i, alloc := range result {
		if !alloc.AtRisk {
			t.Errorf("allocation %d should be at-risk", i)
		}
	}
}

func Test_ALLOC_consumed_time_shared_across_projects(t *testing.T) {
	// When a default-project task consumes a time range, that range should also
	// be unavailable for project-specific slots.
	tasks := []ScoredTask{
		{
			Task:  jira.Task{Key: "DEF-1", RemainingEstimate: 1 * time.Hour, Project: "DEF"},
			Score: 100,
		},
		{
			Task:  jira.Task{Key: "ADMIN-1", RemainingEstimate: 30 * time.Minute, Project: "ADMIN"},
			Score: 80,
		},
	}

	// Both project slot lists cover 09:00-10:00
	slots := map[string][]calendar.Slot{
		"": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 12, 0)},
		},
		"ADMIN": {
			{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 10, 0)},
		},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 1 {
		// DEF-1 takes 09:00-10:00, so ADMIN-1 has no room in its 09:00-10:00 window
		t.Fatalf("expected 1 allocation (ADMIN has no room), got %d", len(result))
	}

	if result[0].Task.Key != "DEF-1" {
		t.Errorf("expected DEF-1 to be allocated, got %s", result[0].Task.Key)
	}
}

func Test_ALLOC_no_slots_no_allocations(t *testing.T) {
	tasks := []ScoredTask{
		{Task: jira.Task{Key: "X-1", RemainingEstimate: time.Hour, Project: "P"}, Score: 80},
	}

	slots := map[string][]calendar.Slot{
		"": {},
	}

	result := Allocate(tasks, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 0 {
		t.Errorf("expected 0 allocations with no slots, got %d", len(result))
	}
}

func Test_ALLOC_empty_tasks(t *testing.T) {
	slots := map[string][]calendar.Slot{
		"": {{Start: date(2025, 1, 20, 9, 0), End: date(2025, 1, 20, 17, 0)}},
	}

	result := Allocate(nil, slots, AllocateConfig{MinTaskDurationMinutes: 25})

	if len(result) != 0 {
		t.Errorf("expected 0 allocations with no tasks, got %d", len(result))
	}
}
