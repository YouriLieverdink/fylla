package commands

import (
	"testing"
	"time"
)

func TestParseSnoozeTarget(t *testing.T) {
	now := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		raw     string
		want    time.Time
		wantErr bool
	}{
		{
			name: "3 days",
			raw:  "3d",
			want: time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC),
		},
		{
			name: "1 week",
			raw:  "1w",
			want: time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC),
		},
		{
			name: "2 hours",
			raw:  "2h",
			want: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			name: "1 month",
			raw:  "1m",
			want: time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSnoozeTarget(tt.raw, now)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
