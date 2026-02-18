package prutil

import (
	"testing"
	"time"
)

func TestEstimateFromLines(t *testing.T) {
	tests := []struct {
		name    string
		added   int
		removed int
		want    time.Duration
	}{
		{"zero lines", 0, 0, 15 * time.Minute},
		{"trivial change", 10, 5, 15 * time.Minute},
		{"just under 50", 30, 19, 15 * time.Minute},
		{"exactly 50", 30, 20, 30 * time.Minute},
		{"small PR", 100, 50, 30 * time.Minute},
		{"just under 200", 100, 99, 30 * time.Minute},
		{"exactly 200", 100, 100, 45 * time.Minute},
		{"medium PR", 300, 100, 45 * time.Minute},
		{"just under 500", 300, 199, 45 * time.Minute},
		{"exactly 500", 300, 200, 1 * time.Hour},
		{"large PR", 600, 200, 1 * time.Hour},
		{"just under 1000", 500, 499, 1 * time.Hour},
		{"exactly 1000", 500, 500, 90 * time.Minute},
		{"very large PR", 2000, 500, 90 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateFromLines(tt.added, tt.removed)
			if got != tt.want {
				t.Errorf("EstimateFromLines(%d, %d) = %v, want %v", tt.added, tt.removed, got, tt.want)
			}
		})
	}
}
