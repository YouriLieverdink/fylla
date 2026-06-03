package config

import (
	"strings"
	"testing"
	"time"
)

func TestBuildHolidayIndex_Valid(t *testing.T) {
	idx, err := BuildHolidayIndex([]HolidayConfig{
		{Date: "2026-04-27"},
		{Date: "2026-05-05", Start: "13:00", End: "17:00"},
		{Date: "2026-05-06", Start: "09:00", End: "10:00"},
		{Date: "2026-05-06", Start: "16:00", End: "17:00"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loc := time.UTC
	if !idx.IsFullDay(time.Date(2026, 4, 27, 0, 0, 0, 0, loc)) {
		t.Error("2026-04-27 should be full day")
	}
	if idx.IsFullDay(time.Date(2026, 5, 5, 0, 0, 0, 0, loc)) {
		t.Error("2026-05-05 should not be full day")
	}

	blocks := idx.BlocksOn(time.Date(2026, 5, 6, 0, 0, 0, 0, loc))
	if len(blocks) != 2 {
		t.Fatalf("want 2 blocks on 2026-05-06, got %d", len(blocks))
	}
}

func TestBuildHolidayIndex_Errors(t *testing.T) {
	tests := []struct {
		name    string
		entries []HolidayConfig
		want    string
	}{
		{"missing date", []HolidayConfig{{}}, "date: required"},
		{"bad date", []HolidayConfig{{Date: "26-04-27"}}, "date"},
		{"start without end", []HolidayConfig{{Date: "2026-04-27", Start: "09:00"}}, "start and end"},
		{"end without start", []HolidayConfig{{Date: "2026-04-27", End: "10:00"}}, "start and end"},
		{"start equals end", []HolidayConfig{{Date: "2026-04-27", Start: "09:00", End: "09:00"}}, "start must be before end"},
		{"start after end", []HolidayConfig{{Date: "2026-04-27", Start: "10:00", End: "09:00"}}, "start must be before end"},
		{"bad time", []HolidayConfig{{Date: "2026-04-27", Start: "9:00", End: "17:00"}}, "start"},
		{"full plus partial", []HolidayConfig{
			{Date: "2026-04-27"},
			{Date: "2026-04-27", Start: "09:00", End: "10:00"},
		}, "partial entry conflicts"},
		{"partial then full", []HolidayConfig{
			{Date: "2026-04-27", Start: "09:00", End: "10:00"},
			{Date: "2026-04-27"},
		}, "full-day entry conflicts"},
		{"two full days", []HolidayConfig{
			{Date: "2026-04-27"},
			{Date: "2026-04-27"},
		}, "duplicate full-day"},
		{"overlapping partials", []HolidayConfig{
			{Date: "2026-04-27", Start: "09:00", End: "11:00"},
			{Date: "2026-04-27", Start: "10:00", End: "12:00"},
		}, "overlapping ranges"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildHolidayIndex(tt.entries)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestBuildSickDayIndex_ErrorPrefix(t *testing.T) {
	_, err := BuildSickDayIndex([]HolidayConfig{{Date: "2026-04-27", Start: "10:00", End: "09:00"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "sickDays[0]") {
		t.Errorf("error %q does not mention sickDays prefix", err.Error())
	}
}

func TestHolidayIndex_EffectiveDailyHours(t *testing.T) {
	biz := []BusinessHoursConfig{
		{Start: "09:00", End: "17:00", WorkDays: []int{1, 2, 3, 4, 5}},
	}
	loc := time.UTC

	tests := []struct {
		name    string
		entries []HolidayConfig
		date    time.Time
		nominal float64
		want    float64
	}{
		{
			name:    "no holiday returns nominal",
			date:    time.Date(2026, 5, 4, 0, 0, 0, 0, loc), // Mon
			nominal: 8,
			want:    8,
		},
		{
			name:    "full day returns 0",
			entries: []HolidayConfig{{Date: "2026-05-04"}},
			date:    time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
			nominal: 8,
			want:    0,
		},
		{
			name:    "afternoon off",
			entries: []HolidayConfig{{Date: "2026-05-04", Start: "13:00", End: "17:00"}},
			date:    time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
			nominal: 8,
			want:    4,
		},
		{
			name: "two 1h blocks",
			entries: []HolidayConfig{
				{Date: "2026-05-04", Start: "09:00", End: "10:00"},
				{Date: "2026-05-04", Start: "16:00", End: "17:00"},
			},
			date:    time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
			nominal: 8,
			want:    6,
		},
		{
			name:    "block outside business hours has no effect",
			entries: []HolidayConfig{{Date: "2026-05-04", Start: "06:00", End: "08:00"}},
			date:    time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
			nominal: 8,
			want:    8,
		},
		{
			name:    "block clipped to business hours",
			entries: []HolidayConfig{{Date: "2026-05-04", Start: "08:00", End: "10:00"}},
			date:    time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
			nominal: 8,
			want:    7, // 09:00–10:00 overlap
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := BuildHolidayIndex(tt.entries)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			got := idx.EffectiveDailyHours(tt.date, tt.nominal, biz)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
