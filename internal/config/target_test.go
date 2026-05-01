package config

import (
	"testing"
	"time"
)

func TestTargetConfig_ResolvePeriod(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, loc)

	tests := []struct {
		name      string
		target    TargetConfig
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name:      "default month",
			target:    TargetConfig{Project: "P", Hours: 24},
			wantStart: time.Date(2026, 5, 1, 0, 0, 0, 0, loc),
			wantEnd:   time.Date(2026, 5, 31, 0, 0, 0, 0, loc),
		},
		{
			name: "explicit dates",
			target: TargetConfig{
				Project:   "P",
				Hours:     10,
				StartDate: "2026-04-10",
				EndDate:   "2026-04-20",
			},
			wantStart: time.Date(2026, 4, 10, 0, 0, 0, 0, loc),
			wantEnd:   time.Date(2026, 4, 20, 0, 0, 0, 0, loc),
		},
		{
			name: "invalid start date",
			target: TargetConfig{
				Project:   "P",
				Hours:     10,
				StartDate: "not-a-date",
				EndDate:   "2026-04-20",
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, e, err := tc.target.ResolvePeriod(now)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolvePeriod: %v", err)
			}
			if !s.Equal(tc.wantStart) {
				t.Errorf("start = %v, want %v", s, tc.wantStart)
			}
			if !e.Equal(tc.wantEnd) {
				t.Errorf("end = %v, want %v", e, tc.wantEnd)
			}
		})
	}
}

func TestTargetConfig_ResolvePeriod_Recurring(t *testing.T) {
	loc := time.UTC
	mkDay := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, loc)
	}

	tests := []struct {
		name      string
		target    TargetConfig
		now       time.Time
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name:      "weekly default mon (now=fri)",
			target:    TargetConfig{Project: "P", Hours: 10, Period: "weekly"},
			now:       mkDay(2026, 5, 1), // Friday
			wantStart: mkDay(2026, 4, 27), // Monday
			wantEnd:   mkDay(2026, 5, 3),  // Sunday
		},
		{
			name:      "weekly default mon (now=mon)",
			target:    TargetConfig{Project: "P", Hours: 10, Period: "weekly"},
			now:       mkDay(2026, 4, 27),
			wantStart: mkDay(2026, 4, 27),
			wantEnd:   mkDay(2026, 5, 3),
		},
		{
			name:      "weekly anchor wed (now=fri)",
			target:    TargetConfig{Project: "P", Hours: 10, Period: "weekly", Anchor: "2026-04-29"},
			now:       mkDay(2026, 5, 1),
			wantStart: mkDay(2026, 4, 29),
			wantEnd:   mkDay(2026, 5, 5),
		},
		{
			name:      "biweekly anchor day=now",
			target:    TargetConfig{Project: "P", Hours: 60, Period: "biweekly", Anchor: "2026-04-27"},
			now:       mkDay(2026, 4, 27),
			wantStart: mkDay(2026, 4, 27),
			wantEnd:   mkDay(2026, 5, 10),
		},
		{
			name:      "biweekly inside first cycle",
			target:    TargetConfig{Project: "P", Hours: 60, Period: "biweekly", Anchor: "2026-04-27"},
			now:       mkDay(2026, 5, 4),
			wantStart: mkDay(2026, 4, 27),
			wantEnd:   mkDay(2026, 5, 10),
		},
		{
			name:      "biweekly second cycle boundary",
			target:    TargetConfig{Project: "P", Hours: 60, Period: "biweekly", Anchor: "2026-04-27"},
			now:       mkDay(2026, 5, 11),
			wantStart: mkDay(2026, 5, 11),
			wantEnd:   mkDay(2026, 5, 24),
		},
		{
			name:      "biweekly before anchor",
			target:    TargetConfig{Project: "P", Hours: 60, Period: "biweekly", Anchor: "2026-04-27"},
			now:       mkDay(2026, 4, 20),
			wantStart: mkDay(2026, 4, 13),
			wantEnd:   mkDay(2026, 4, 26),
		},
		{
			name:    "biweekly without anchor errors",
			target:  TargetConfig{Project: "P", Hours: 60, Period: "biweekly"},
			now:     mkDay(2026, 5, 1),
			wantErr: true,
		},
		{
			name:    "unknown period errors",
			target:  TargetConfig{Project: "P", Hours: 1, Period: "yearly"},
			now:     mkDay(2026, 5, 1),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, e, err := tc.target.ResolvePeriod(tc.now)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolvePeriod: %v", err)
			}
			if !s.Equal(tc.wantStart) {
				t.Errorf("start = %v, want %v", s, tc.wantStart)
			}
			if !e.Equal(tc.wantEnd) {
				t.Errorf("end = %v, want %v", e, tc.wantEnd)
			}
		})
	}
}

func TestTargetConfig_PeriodLabel(t *testing.T) {
	now := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	if got := (TargetConfig{}).PeriodLabel(now); got != "May 2026" {
		t.Errorf("default label = %q, want %q", got, "May 2026")
	}
	t2 := TargetConfig{StartDate: "2026-04-01", EndDate: "2026-04-30"}
	if got := t2.PeriodLabel(now); got != "2026-04-01..2026-04-30" {
		t.Errorf("override label = %q", got)
	}
}

func TestValidate_Targets(t *testing.T) {
	base := func() *Config {
		return &Config{
			Providers: []string{"local"},
			Weights: WeightsConfig{
				Priority: 0.45, DueDate: 0.30, Estimate: 0.15, Age: 0.10,
			},
			BusinessHours: []BusinessHoursConfig{
				{Start: "09:00", End: "17:00", WorkDays: []int{1, 2, 3, 4, 5}},
			},
			Scheduling: SchedulingConfig{
				WindowDays:             5,
				MinTaskDurationMinutes: 25,
			},
		}
	}

	tests := []struct {
		name    string
		targets []TargetConfig
		wantErr bool
	}{
		{name: "empty ok", targets: nil},
		{name: "valid me", targets: []TargetConfig{{Project: "P", Hours: 24, Scope: "me"}}},
		{name: "valid anyone", targets: []TargetConfig{{Project: "P", Hours: 24, Scope: "anyone"}}},
		{name: "valid empty scope", targets: []TargetConfig{{Project: "P", Hours: 24}}},
		{name: "missing project", targets: []TargetConfig{{Hours: 24}}, wantErr: true},
		{name: "zero hours", targets: []TargetConfig{{Project: "P", Hours: 0}}, wantErr: true},
		{name: "negative hours", targets: []TargetConfig{{Project: "P", Hours: -1}}, wantErr: true},
		{name: "bad scope", targets: []TargetConfig{{Project: "P", Hours: 24, Scope: "us"}}, wantErr: true},
		{
			name:    "only start date",
			targets: []TargetConfig{{Project: "P", Hours: 24, StartDate: "2026-01-01"}},
			wantErr: true,
		},
		{
			name: "valid date range",
			targets: []TargetConfig{
				{Project: "P", Hours: 24, StartDate: "2026-01-01", EndDate: "2026-01-31"},
			},
		},
		{
			name: "start after end",
			targets: []TargetConfig{
				{Project: "P", Hours: 24, StartDate: "2026-02-01", EndDate: "2026-01-01"},
			},
			wantErr: true,
		},
		{
			name: "malformed date",
			targets: []TargetConfig{
				{Project: "P", Hours: 24, StartDate: "yes", EndDate: "no"},
			},
			wantErr: true,
		},
		{
			name:    "valid weekly",
			targets: []TargetConfig{{Project: "P", Hours: 10, Period: "weekly"}},
		},
		{
			name:    "valid biweekly with anchor",
			targets: []TargetConfig{{Project: "P", Hours: 60, Period: "biweekly", Anchor: "2026-04-27"}},
		},
		{
			name:    "biweekly without anchor",
			targets: []TargetConfig{{Project: "P", Hours: 60, Period: "biweekly"}},
			wantErr: true,
		},
		{
			name:    "bad period",
			targets: []TargetConfig{{Project: "P", Hours: 1, Period: "yearly"}},
			wantErr: true,
		},
		{
			name: "period + fixed mutex",
			targets: []TargetConfig{{
				Project: "P", Hours: 1, Period: "weekly",
				StartDate: "2026-04-01", EndDate: "2026-04-30",
			}},
			wantErr: true,
		},
		{
			name:    "bad anchor",
			targets: []TargetConfig{{Project: "P", Hours: 1, Period: "weekly", Anchor: "bad"}},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			cfg.Targets = tc.targets
			err := cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
