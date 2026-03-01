package task

import (
	"testing"
	"time"
)

func TestExtractRecurrence(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantRec *Recurrence
	}{
		{
			name:    "no recurrence",
			input:   "Buy groceries",
			want:    "Buy groceries",
			wantRec: nil,
		},
		{
			name:    "every day",
			input:   "Review inbox [30m] (every day)",
			want:    "Review inbox [30m]",
			wantRec: &Recurrence{Freq: "daily"},
		},
		{
			name:    "every weekday",
			input:   "Standup [15m] (every weekday)",
			want:    "Standup [15m]",
			wantRec: &Recurrence{Freq: "weekly", Days: []int{1, 2, 3, 4, 5}},
		},
		{
			name:    "every Monday",
			input:   "Weekly review [1h] (every Monday)",
			want:    "Weekly review [1h]",
			wantRec: &Recurrence{Freq: "weekly", Days: []int{1}},
		},
		{
			name:    "multiple days",
			input:   "Gym [1h] (every Monday Wednesday Friday)",
			want:    "Gym [1h]",
			wantRec: &Recurrence{Freq: "weekly", Days: []int{1, 3, 5}},
		},
		{
			name:    "biweekly",
			input:   "Sprint retro [1h] (every 2 weeks on Friday)",
			want:    "Sprint retro [1h]",
			wantRec: &Recurrence{Freq: "biweekly", Days: []int{5}},
		},
		{
			name:    "every month",
			input:   "Monthly report [2h] (every month)",
			want:    "Monthly report [2h]",
			wantRec: &Recurrence{Freq: "monthly"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rec := ExtractRecurrence(tt.input)
			if got != tt.want {
				t.Errorf("cleaned = %q, want %q", got, tt.want)
			}
			if tt.wantRec == nil {
				if rec != nil {
					t.Errorf("expected nil recurrence, got %+v", rec)
				}
				return
			}
			if rec == nil {
				t.Fatalf("expected recurrence, got nil")
			}
			if rec.Freq != tt.wantRec.Freq {
				t.Errorf("freq = %q, want %q", rec.Freq, tt.wantRec.Freq)
			}
			if len(rec.Days) != len(tt.wantRec.Days) {
				t.Errorf("days = %v, want %v", rec.Days, tt.wantRec.Days)
			} else {
				for i := range rec.Days {
					if rec.Days[i] != tt.wantRec.Days[i] {
						t.Errorf("day[%d] = %d, want %d", i, rec.Days[i], tt.wantRec.Days[i])
					}
				}
			}
		})
	}
}

func TestExpandRecurring(t *testing.T) {
	loc := time.UTC

	t.Run("daily for 3 days", func(t *testing.T) {
		task := Task{
			Key:        "L-1",
			Summary:    "Daily standup",
			Recurrence: &Recurrence{Freq: "daily"},
		}
		start := time.Date(2026, 3, 2, 0, 0, 0, 0, loc)
		end := time.Date(2026, 3, 4, 23, 59, 0, 0, loc)

		instances := ExpandRecurring(task, start, end)
		if len(instances) != 3 {
			t.Fatalf("expected 3 instances, got %d", len(instances))
		}
		if instances[0].Key != "L-1@2026-03-02" {
			t.Errorf("key[0] = %s", instances[0].Key)
		}
		if instances[2].Key != "L-1@2026-03-04" {
			t.Errorf("key[2] = %s", instances[2].Key)
		}
		if instances[0].Recurrence != nil {
			t.Error("instance should have nil Recurrence")
		}
	})

	t.Run("weekly Monday Wednesday", func(t *testing.T) {
		task := Task{
			Key:        "L-2",
			Summary:    "Gym",
			Recurrence: &Recurrence{Freq: "weekly", Days: []int{1, 3}},
		}
		// March 2-6, 2026: Mon=2, Tue=3, Wed=4, Thu=5, Fri=6
		start := time.Date(2026, 3, 2, 0, 0, 0, 0, loc)
		end := time.Date(2026, 3, 6, 23, 59, 0, 0, loc)

		instances := ExpandRecurring(task, start, end)
		if len(instances) != 2 {
			t.Fatalf("expected 2 instances, got %d", len(instances))
		}
		if instances[0].Key != "L-2@2026-03-02" {
			t.Errorf("key[0] = %s, want L-2@2026-03-02", instances[0].Key)
		}
		if instances[1].Key != "L-2@2026-03-04" {
			t.Errorf("key[1] = %s, want L-2@2026-03-04", instances[1].Key)
		}
	})

	t.Run("weekday", func(t *testing.T) {
		task := Task{
			Key:        "L-3",
			Summary:    "Standup",
			Recurrence: &Recurrence{Freq: "weekly", Days: []int{1, 2, 3, 4, 5}},
		}
		// March 2-8, 2026: Mon-Sun
		start := time.Date(2026, 3, 2, 0, 0, 0, 0, loc)
		end := time.Date(2026, 3, 8, 23, 59, 0, 0, loc)

		instances := ExpandRecurring(task, start, end)
		if len(instances) != 5 {
			t.Fatalf("expected 5 instances (weekdays), got %d", len(instances))
		}
	})

	t.Run("non-recurring passthrough", func(t *testing.T) {
		task := Task{Key: "L-4", Summary: "One-time"}
		start := time.Date(2026, 3, 2, 0, 0, 0, 0, loc)
		end := time.Date(2026, 3, 6, 23, 59, 0, 0, loc)

		instances := ExpandRecurring(task, start, end)
		if len(instances) != 1 || instances[0].Key != "L-4" {
			t.Errorf("non-recurring should pass through, got %v", instances)
		}
	})

	t.Run("monthly", func(t *testing.T) {
		task := Task{
			Key:        "L-5",
			Summary:    "Monthly report",
			Recurrence: &Recurrence{Freq: "monthly"},
		}
		start := time.Date(2026, 1, 15, 0, 0, 0, 0, loc)
		end := time.Date(2026, 4, 15, 23, 59, 0, 0, loc)

		instances := ExpandRecurring(task, start, end)
		if len(instances) != 4 {
			t.Fatalf("expected 4 monthly instances, got %d", len(instances))
		}
	})
}

func TestExpandAll(t *testing.T) {
	loc := time.UTC
	tasks := []Task{
		{Key: "L-1", Summary: "Normal task"},
		{Key: "L-2", Summary: "Daily", Recurrence: &Recurrence{Freq: "daily"}},
	}
	start := time.Date(2026, 3, 2, 0, 0, 0, 0, loc)
	end := time.Date(2026, 3, 3, 23, 59, 0, 0, loc)

	expanded := ExpandAll(tasks, start, end)
	// 1 normal + 2 daily instances
	if len(expanded) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(expanded))
	}
}

func TestStripInstanceSuffix(t *testing.T) {
	tests := []struct {
		input    string
		wantKey  string
		wantOK   bool
	}{
		{"L-1@2026-03-02", "L-1", true},
		{"PROJ-123@2026-01-15", "PROJ-123", true},
		{"L-1", "L-1", false},
		{"short", "short", false},
	}
	for _, tt := range tests {
		got, ok := StripInstanceSuffix(tt.input)
		if got != tt.wantKey || ok != tt.wantOK {
			t.Errorf("StripInstanceSuffix(%q) = (%q, %v), want (%q, %v)",
				tt.input, got, ok, tt.wantKey, tt.wantOK)
		}
	}
}
