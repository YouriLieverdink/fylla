package worklog

import (
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/config"
)

func defaultBiz() []config.BusinessHoursConfig {
	return []config.BusinessHoursConfig{
		{Start: "09:00", End: "17:00", WorkDays: []int{1, 2, 3, 4, 5}},
	}
}

func newModel(t *testing.T, date time.Time, holidays []config.HolidayConfig) Model {
	t.Helper()
	idx, err := config.BuildHolidayIndex(holidays)
	if err != nil {
		t.Fatalf("build holiday index: %v", err)
	}
	m := New(8, 40, 0.7, []int{1, 2, 3, 4, 5}, defaultBiz(), idx)
	m.Date = date
	return m
}

func TestDailyTarget(t *testing.T) {
	mon := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC) // Monday

	tests := []struct {
		name     string
		holidays []config.HolidayConfig
		want     time.Duration
	}{
		{"no holiday", nil, 8 * time.Hour},
		{"full day off", []config.HolidayConfig{{Date: "2026-05-04"}}, 0},
		{"4h afternoon", []config.HolidayConfig{{Date: "2026-05-04", Start: "13:00", End: "17:00"}}, 4 * time.Hour},
		{"two 1h blocks", []config.HolidayConfig{
			{Date: "2026-05-04", Start: "09:00", End: "10:00"},
			{Date: "2026-05-04", Start: "16:00", End: "17:00"},
		}, 6 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModel(t, mon, tt.holidays)
			if got := m.dailyTarget(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWeeklyTarget(t *testing.T) {
	wed := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC) // Wednesday in week of 2026-05-04..05-10

	tests := []struct {
		name     string
		holidays []config.HolidayConfig
		want     time.Duration
	}{
		{"clean week", nil, 40 * time.Hour},
		{"one full holiday", []config.HolidayConfig{{Date: "2026-05-04"}}, 32 * time.Hour},
		{"4h afternoon off", []config.HolidayConfig{{Date: "2026-05-05", Start: "13:00", End: "17:00"}}, 36 * time.Hour},
		{"two 1h blocks one day", []config.HolidayConfig{
			{Date: "2026-05-06", Start: "09:00", End: "10:00"},
			{Date: "2026-05-06", Start: "16:00", End: "17:00"},
		}, 38 * time.Hour},
		{"holiday on weekend ignored", []config.HolidayConfig{{Date: "2026-05-09"}}, 40 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModel(t, wed, tt.holidays)
			if got := m.weeklyTarget(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
