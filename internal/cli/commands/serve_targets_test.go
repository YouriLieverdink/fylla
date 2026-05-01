package commands

import (
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/config"
)

func TestShiftNowForCycle(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, loc)

	tests := []struct {
		name   string
		target config.TargetConfig
		offset int
		want   time.Time
	}{
		{
			name:   "zero offset returns now",
			target: config.TargetConfig{Period: "monthly"},
			offset: 0,
			want:   now,
		},
		{
			name:   "monthly offset -1",
			target: config.TargetConfig{Period: "monthly"},
			offset: -1,
			want:   time.Date(2026, 4, 15, 10, 0, 0, 0, loc),
		},
		{
			name:   "monthly offset -3",
			target: config.TargetConfig{Period: "monthly"},
			offset: -3,
			want:   time.Date(2026, 2, 15, 10, 0, 0, 0, loc),
		},
		{
			name:   "monthly default (empty period)",
			target: config.TargetConfig{},
			offset: -1,
			want:   time.Date(2026, 4, 15, 10, 0, 0, 0, loc),
		},
		{
			name:   "weekly offset -2",
			target: config.TargetConfig{Period: "weekly"},
			offset: -2,
			want:   now.AddDate(0, 0, -14),
		},
		{
			name:   "biweekly offset +1",
			target: config.TargetConfig{Period: "biweekly", Anchor: "2026-04-27"},
			offset: 1,
			want:   now.AddDate(0, 0, 14),
		},
		{
			name: "fixed range ignores offset",
			target: config.TargetConfig{
				StartDate: "2026-04-01", EndDate: "2026-04-30",
			},
			offset: -3,
			want:   now,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shiftNowForCycle(now, tc.target, tc.offset)
			if !got.Equal(tc.want) {
				t.Errorf("shiftNowForCycle = %v, want %v", got, tc.want)
			}
		})
	}
}
