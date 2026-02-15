package calendar

import (
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/config"
)

func defaultHours() []config.BusinessHoursConfig {
	return []config.BusinessHoursConfig{{
		Start:    "09:00",
		End:      "17:00",
		WorkDays: []int{1, 2, 3, 4, 5},
	}}
}

func dt(year, month, day, hour, min int) time.Time {
	return time.Date(year, time.Month(month), day, hour, min, 0, 0, time.UTC)
}

func Test_SLOT001_filter_slots_to_business_hours(t *testing.T) {
	// Monday 2025-01-20
	now := dt(2025, 1, 20, 7, 0) // 7:00 AM, before business hours
	rangeStart := dt(2025, 1, 20, 0, 0)
	rangeEnd := dt(2025, 1, 20, 23, 59)

	t.Run("slots fall within configured business hours", func(t *testing.T) {
		hours := []config.BusinessHoursConfig{{
			Start:    "09:00",
			End:      "17:00",
			WorkDays: []int{1, 2, 3, 4, 5},
		}}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, hours, 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) == 0 {
			t.Fatal("expected at least one slot")
		}
		for _, s := range slots {
			if s.Start.Hour() < 9 {
				t.Errorf("slot starts before business hours: %v", s.Start)
			}
			if s.End.Hour() > 17 || (s.End.Hour() == 17 && s.End.Minute() > 0) {
				t.Errorf("slot ends after business hours: %v", s.End)
			}
		}
	})

	t.Run("custom business hours 10:00-16:00", func(t *testing.T) {
		hours := []config.BusinessHoursConfig{{
			Start:    "10:00",
			End:      "16:00",
			WorkDays: []int{1, 2, 3, 4, 5},
		}}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, hours, 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) != 1 {
			t.Fatalf("expected 1 slot, got %d", len(slots))
		}
		if slots[0].Start.Hour() != 10 || slots[0].End.Hour() != 16 {
			t.Errorf("expected 10:00-16:00, got %v-%v", slots[0].Start, slots[0].End)
		}
	})

	t.Run("no tasks scheduled outside business hours", func(t *testing.T) {
		hours := []config.BusinessHoursConfig{{
			Start:    "09:00",
			End:      "17:00",
			WorkDays: []int{1, 2, 3, 4, 5},
		}}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, hours, 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		bStart := dt(2025, 1, 20, 9, 0)
		bEnd := dt(2025, 1, 20, 17, 0)
		for _, s := range slots {
			if s.Start.Before(bStart) || s.End.After(bEnd) {
				t.Errorf("slot %v-%v outside business hours", s.Start, s.End)
			}
		}
	})
}

func Test_SLOT002_skip_weekends(t *testing.T) {
	// Friday 2025-01-17 to Monday 2025-01-20
	now := dt(2025, 1, 17, 7, 0)
	rangeStart := dt(2025, 1, 17, 0, 0)
	rangeEnd := dt(2025, 1, 20, 23, 59)

	t.Run("no tasks scheduled on Saturday or Sunday", func(t *testing.T) {
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, s := range slots {
			wd := s.Start.Weekday()
			if wd == time.Saturday || wd == time.Sunday {
				t.Errorf("slot scheduled on weekend: %v (%s)", s.Start, wd)
			}
		}
		// Should have slots on Friday and Monday only
		days := make(map[int]bool)
		for _, s := range slots {
			days[s.Start.Day()] = true
		}
		if !days[17] {
			t.Error("expected slot on Friday Jan 17")
		}
		if !days[20] {
			t.Error("expected slot on Monday Jan 20")
		}
		if days[18] || days[19] {
			t.Error("did not expect slots on Saturday/Sunday")
		}
	})

	t.Run("configurable work days include weekend", func(t *testing.T) {
		hours := []config.BusinessHoursConfig{{
			Start:    "09:00",
			End:      "17:00",
			WorkDays: []int{1, 2, 3, 4, 5, 6}, // include Saturday
		}}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, hours, 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		days := make(map[int]bool)
		for _, s := range slots {
			days[s.Start.Day()] = true
		}
		if !days[18] {
			t.Error("expected slot on Saturday when configured")
		}
	})

	t.Run("custom work days Mon-Thu only", func(t *testing.T) {
		hours := []config.BusinessHoursConfig{{
			Start:    "09:00",
			End:      "17:00",
			WorkDays: []int{1, 2, 3, 4}, // no Friday
		}}
		// Wed Jan 15 to Fri Jan 17
		rangeStart := dt(2025, 1, 15, 0, 0)
		rangeEnd := dt(2025, 1, 17, 23, 59)
		now := dt(2025, 1, 15, 7, 0)
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, hours, 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, s := range slots {
			if s.Start.Weekday() == time.Friday {
				t.Errorf("slot on Friday when not in workDays: %v", s.Start)
			}
		}
	})
}

func Test_SLOT003_buffer_between_tasks(t *testing.T) {
	// Monday 2025-01-20
	now := dt(2025, 1, 20, 7, 0)
	rangeStart := dt(2025, 1, 20, 0, 0)
	rangeEnd := dt(2025, 1, 20, 23, 59)

	meeting := Event{
		Title: "Team standup",
		Start: dt(2025, 1, 20, 10, 0),
		End:   dt(2025, 1, 20, 10, 30),
	}

	t.Run("15-minute buffer after meeting", func(t *testing.T) {
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, []Event{meeting}, defaultHours(), 15, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should have gap: 09:00-10:00, then 10:45-17:00
		if len(slots) < 2 {
			t.Fatalf("expected at least 2 slots, got %d", len(slots))
		}
		// Second slot should start at 10:45 (10:30 + 15 min buffer)
		if slots[1].Start != dt(2025, 1, 20, 10, 45) {
			t.Errorf("expected second slot to start at 10:45, got %v", slots[1].Start)
		}
	})

	t.Run("30-minute buffer configured", func(t *testing.T) {
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, []Event{meeting}, defaultHours(), 30, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) < 2 {
			t.Fatalf("expected at least 2 slots, got %d", len(slots))
		}
		// Second slot should start at 11:00 (10:30 + 30 min buffer)
		if slots[1].Start != dt(2025, 1, 20, 11, 0) {
			t.Errorf("expected second slot to start at 11:00, got %v", slots[1].Start)
		}
	})

	t.Run("zero buffer means slots are adjacent", func(t *testing.T) {
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, []Event{meeting}, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) < 2 {
			t.Fatalf("expected at least 2 slots, got %d", len(slots))
		}
		if slots[1].Start != dt(2025, 1, 20, 10, 30) {
			t.Errorf("expected second slot to start at 10:30, got %v", slots[1].Start)
		}
	})
}

func Test_SLOT004_project_aware_time_windows(t *testing.T) {
	// Monday 2025-01-20
	now := dt(2025, 1, 20, 7, 0)
	rangeStart := dt(2025, 1, 20, 0, 0)
	rangeEnd := dt(2025, 1, 20, 23, 59)

	t.Run("ADMIN project only scheduled 09:00-10:00", func(t *testing.T) {
		adminHours := []config.BusinessHoursConfig{{
			Start:    "09:00",
			End:      "10:00",
			WorkDays: []int{1, 2, 3, 4, 5},
		}}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, adminHours, 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) != 1 {
			t.Fatalf("expected 1 slot, got %d", len(slots))
		}
		if slots[0].Start != dt(2025, 1, 20, 9, 0) || slots[0].End != dt(2025, 1, 20, 10, 0) {
			t.Errorf("expected 09:00-10:00, got %v-%v", slots[0].Start, slots[0].End)
		}
	})

	t.Run("default tasks use full business hours", func(t *testing.T) {
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) != 1 {
			t.Fatalf("expected 1 slot, got %d", len(slots))
		}
		if slots[0].Start != dt(2025, 1, 20, 9, 0) || slots[0].End != dt(2025, 1, 20, 17, 0) {
			t.Errorf("expected 09:00-17:00, got %v-%v", slots[0].Start, slots[0].End)
		}
	})

	t.Run("BusinessHoursFor returns project-specific or default", func(t *testing.T) {
		cfg := &config.Config{
			BusinessHours: []config.BusinessHoursConfig{{
				Start:    "09:00",
				End:      "17:00",
				WorkDays: []int{1, 2, 3, 4, 5},
			}},
			ProjectRules: map[string][]config.BusinessHoursConfig{
				"ADMIN": {{Start: "09:00", End: "10:00", WorkDays: []int{1, 2, 3, 4, 5}}},
			},
		}
		adminHours := cfg.BusinessHoursFor("ADMIN")
		if adminHours[0].End != "10:00" {
			t.Errorf("expected ADMIN end 10:00, got %s", adminHours[0].End)
		}
		defaultBH := cfg.BusinessHoursFor("OTHER")
		if defaultBH[0].End != "17:00" {
			t.Errorf("expected default end 17:00, got %s", defaultBH[0].End)
		}
	})
}

func Test_SLOT005_block_OOO_time_ranges(t *testing.T) {
	// Monday 2025-01-20
	now := dt(2025, 1, 20, 7, 0)
	rangeStart := dt(2025, 1, 20, 0, 0)
	rangeEnd := dt(2025, 1, 20, 23, 59)

	t.Run("full day OOO blocks all scheduling", func(t *testing.T) {
		ooo := Event{
			Title:     "PTO",
			Start:     dt(2025, 1, 20, 0, 0),
			End:       dt(2025, 1, 21, 0, 0),
			EventType: "outOfOffice",
			AllDay:    true,
		}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, []Event{ooo}, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) != 0 {
			t.Errorf("expected no slots during OOO, got %d", len(slots))
		}
	})

	t.Run("partial OOO blocks that period only", func(t *testing.T) {
		ooo := Event{
			Title:     "Doctor appointment",
			Start:     dt(2025, 1, 20, 13, 0),
			End:       dt(2025, 1, 20, 15, 0),
			EventType: "outOfOffice",
		}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, []Event{ooo}, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should have: 09:00-13:00 and 15:00-17:00
		if len(slots) != 2 {
			t.Fatalf("expected 2 slots, got %d", len(slots))
		}
		if slots[0].End != dt(2025, 1, 20, 13, 0) {
			t.Errorf("expected first slot to end at 13:00, got %v", slots[0].End)
		}
		if slots[1].Start != dt(2025, 1, 20, 15, 0) {
			t.Errorf("expected second slot to start at 15:00, got %v", slots[1].Start)
		}
	})

	t.Run("OOO detected by title pattern", func(t *testing.T) {
		ooo := Event{
			Title:  "Vacation",
			Start:  dt(2025, 1, 20, 0, 0),
			End:    dt(2025, 1, 21, 0, 0),
			AllDay: true,
		}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, []Event{ooo}, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) != 0 {
			t.Errorf("expected no slots during vacation, got %d", len(slots))
		}
	})
}

func Test_SLOT006_events_start_from_current_time(t *testing.T) {
	// Monday 2025-01-20, current time 14:00
	now := dt(2025, 1, 20, 14, 0)
	rangeStart := dt(2025, 1, 20, 0, 0)
	rangeEnd := dt(2025, 1, 20, 23, 59)

	t.Run("today slots start from now, not start of day", func(t *testing.T) {
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) == 0 {
			t.Fatal("expected at least one slot")
		}
		// First slot should start at 14:00 (now), not 09:00
		if slots[0].Start.Before(now) {
			t.Errorf("expected slots to start at or after %v, got %v", now, slots[0].Start)
		}
	})

	t.Run("no tasks scheduled in past hours", func(t *testing.T) {
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, s := range slots {
			if s.Start.Before(now) {
				t.Errorf("slot starts in the past: %v", s.Start)
			}
		}
	})

	t.Run("future days start from business hours start", func(t *testing.T) {
		// Range covers Monday and Tuesday
		rangeEnd := dt(2025, 1, 21, 23, 59)
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Find Tuesday's first slot
		for _, s := range slots {
			if s.Start.Day() == 21 {
				if s.Start.Hour() != 9 || s.Start.Minute() != 0 {
					t.Errorf("expected Tuesday to start at 09:00, got %v", s.Start)
				}
				break
			}
		}
	})

	t.Run("now with buffer pushes start forward", func(t *testing.T) {
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, defaultHours(), 15, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) == 0 {
			t.Fatal("expected at least one slot")
		}
		expected := dt(2025, 1, 20, 14, 15) // now + 15 min buffer
		if slots[0].Start != expected {
			t.Errorf("expected first slot at %v, got %v", expected, slots[0].Start)
		}
	})
}

func Test_SLOT007_multi_day_OOO_handled(t *testing.T) {
	// Monday 2025-01-20 through Friday 2025-01-24
	now := dt(2025, 1, 20, 7, 0)
	rangeStart := dt(2025, 1, 20, 0, 0)
	rangeEnd := dt(2025, 1, 24, 23, 59)

	t.Run("week-long vacation blocks all days", func(t *testing.T) {
		vacation := Event{
			Title:     "Vacation",
			Start:     dt(2025, 1, 20, 0, 0),
			End:       dt(2025, 1, 25, 0, 0),
			EventType: "outOfOffice",
			AllDay:    true,
		}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, []Event{vacation}, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) != 0 {
			t.Errorf("expected no slots during vacation week, got %d", len(slots))
		}
	})

	t.Run("partial week OOO blocks only covered days", func(t *testing.T) {
		ooo := Event{
			Title:     "Conference",
			Start:     dt(2025, 1, 20, 0, 0),
			End:       dt(2025, 1, 22, 0, 0), // Mon-Tue blocked
			EventType: "outOfOffice",
			AllDay:    true,
		}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, []Event{ooo}, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should have slots on Wed(22), Thu(23), Fri(24) only
		days := make(map[int]bool)
		for _, s := range slots {
			days[s.Start.Day()] = true
		}
		if days[20] || days[21] {
			t.Error("expected no slots on Mon/Tue during OOO")
		}
		if !days[22] || !days[23] || !days[24] {
			t.Errorf("expected slots on Wed/Thu/Fri, got days: %v", days)
		}
	})

	t.Run("multiple OOO events in window", func(t *testing.T) {
		ooo1 := Event{
			Title:     "PTO",
			Start:     dt(2025, 1, 20, 0, 0),
			End:       dt(2025, 1, 21, 0, 0),
			EventType: "outOfOffice",
			AllDay:    true,
		}
		ooo2 := Event{
			Title:     "PTO",
			Start:     dt(2025, 1, 23, 0, 0),
			End:       dt(2025, 1, 24, 0, 0),
			EventType: "outOfOffice",
			AllDay:    true,
		}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, []Event{ooo1, ooo2}, defaultHours(), 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		days := make(map[int]bool)
		for _, s := range slots {
			days[s.Start.Day()] = true
		}
		if days[20] || days[23] {
			t.Error("expected no slots on OOO days")
		}
		if !days[21] || !days[22] || !days[24] {
			t.Errorf("expected slots on non-OOO days, got: %v", days)
		}
	})
}

func Test_SLOT008_multiple_business_hour_windows(t *testing.T) {
	// Monday 2025-01-20
	now := dt(2025, 1, 20, 7, 0)
	rangeStart := dt(2025, 1, 20, 0, 0)
	rangeEnd := dt(2025, 1, 20, 23, 59)

	t.Run("two windows produce two slots with no events", func(t *testing.T) {
		hours := []config.BusinessHoursConfig{
			{Start: "09:00", End: "12:00", WorkDays: []int{1, 2, 3, 4, 5}},
			{Start: "13:00", End: "17:00", WorkDays: []int{1, 2, 3, 4, 5}},
		}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, nil, hours, 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(slots) != 2 {
			t.Fatalf("expected 2 slots, got %d", len(slots))
		}
		if slots[0].Start != dt(2025, 1, 20, 9, 0) || slots[0].End != dt(2025, 1, 20, 12, 0) {
			t.Errorf("first slot = %v-%v, want 09:00-12:00", slots[0].Start, slots[0].End)
		}
		if slots[1].Start != dt(2025, 1, 20, 13, 0) || slots[1].End != dt(2025, 1, 20, 17, 0) {
			t.Errorf("second slot = %v-%v, want 13:00-17:00", slots[1].Start, slots[1].End)
		}
	})

	t.Run("event spanning gap only affects relevant window", func(t *testing.T) {
		hours := []config.BusinessHoursConfig{
			{Start: "09:00", End: "12:00", WorkDays: []int{1, 2, 3, 4, 5}},
			{Start: "13:00", End: "17:00", WorkDays: []int{1, 2, 3, 4, 5}},
		}
		// Meeting from 11:00-14:00 spans the lunch gap
		meeting := Event{
			Title: "Long meeting",
			Start: dt(2025, 1, 20, 11, 0),
			End:   dt(2025, 1, 20, 14, 0),
		}
		slots, err := FindFreeSlots(now, rangeStart, rangeEnd, []Event{meeting}, hours, 0, 1, nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// First window: 09:00-11:00 (before meeting)
		// Second window: 14:00-17:00 (after meeting)
		if len(slots) != 2 {
			t.Fatalf("expected 2 slots, got %d", len(slots))
		}
		if slots[0].Start != dt(2025, 1, 20, 9, 0) || slots[0].End != dt(2025, 1, 20, 11, 0) {
			t.Errorf("first slot = %v-%v, want 09:00-11:00", slots[0].Start, slots[0].End)
		}
		if slots[1].Start != dt(2025, 1, 20, 14, 0) || slots[1].End != dt(2025, 1, 20, 17, 0) {
			t.Errorf("second slot = %v-%v, want 14:00-17:00", slots[1].Start, slots[1].End)
		}
	})
}
