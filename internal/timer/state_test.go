package timer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tmpPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "timer.json")
}

func TestTIMER001_StartTimerStoresTaskKeyAndStartTime(t *testing.T) {
	t.Run("stores task key", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		s, err := Start("PROJ-123", "", "", "", now, path)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if s.TaskKey != "PROJ-123" {
			t.Errorf("TaskKey = %q, want PROJ-123", s.TaskKey)
		}
		if !s.StartTime.Equal(now) {
			t.Errorf("StartTime = %v, want %v", s.StartTime, now)
		}
	})

	t.Run("confirmation data available", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)

		s, err := Start("ADMIN-42", "", "", "", now, path)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if s.TaskKey != "ADMIN-42" {
			t.Errorf("TaskKey = %q, want ADMIN-42", s.TaskKey)
		}
	})
}

func TestTIMER002_TimerStatePersisted(t *testing.T) {
	t.Run("file exists at path", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if _, err := Start("PROJ-123", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("timer.json should exist: %v", err)
		}
	})

	t.Run("file contains task key and start timestamp", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if _, err := Start("PROJ-123", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if raw["taskKey"] != "PROJ-123" {
			t.Errorf("taskKey = %v, want PROJ-123", raw["taskKey"])
		}
		if _, ok := raw["startTime"]; !ok {
			t.Error("startTime field missing from timer.json")
		}
	})

	t.Run("default path is under fylla config dir", func(t *testing.T) {
		p, err := DefaultPath()
		if err != nil {
			t.Fatalf("DefaultPath: %v", err)
		}
		if filepath.Base(p) != "timer.json" {
			t.Errorf("expected timer.json, got %s", filepath.Base(p))
		}
		if filepath.Base(filepath.Dir(p)) != "fylla" {
			t.Errorf("expected fylla dir, got %s", filepath.Base(filepath.Dir(p)))
		}
	})

	t.Run("round-trip save and load", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if _, err := Start("PROJ-123", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if loaded.TaskKey != "PROJ-123" {
			t.Errorf("TaskKey = %q, want PROJ-123", loaded.TaskKey)
		}
		if !loaded.StartTime.Equal(now) {
			t.Errorf("StartTime = %v, want %v", loaded.StartTime, now)
		}
	})

	t.Run("load returns nil when no file exists", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent.json")
		s, err := Load(path)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if s != nil {
			t.Error("expected nil state for missing file")
		}
	})
}

func TestTIMER003_StopTimerCalculatesElapsed(t *testing.T) {
	t.Run("elapsed time calculated correctly", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
		stop := start.Add(5 * time.Minute)

		if _, err := Start("PROJ-123", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		result, err := Stop(stop, 5, path)
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if result.Elapsed != 5*time.Minute {
			t.Errorf("Elapsed = %v, want 5m", result.Elapsed)
		}
	})

	t.Run("timer file removed after stop", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if _, err := Start("PROJ-123", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := Stop(start.Add(10*time.Minute), 5, path); err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("timer.json should be removed after stop")
		}
	})

	t.Run("stop with no timer returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "timer.json")
		_, err := Stop(time.Now(), 5, path)
		if err == nil {
			t.Error("expected error when no timer running")
		}
	})

	t.Run("task key preserved in result", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if _, err := Start("PROJ-456", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		result, err := Stop(start.Add(30*time.Minute), 5, path)
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if result.TaskKey != "PROJ-456" {
			t.Errorf("TaskKey = %q, want PROJ-456", result.TaskKey)
		}
	})
}

func TestTIMER004_StopPromptForDescription(t *testing.T) {
	// TIMER-004 requires that the CLI stop command prompts for description
	// when not provided. The timer package exposes StopResult which the CLI
	// layer uses to decide whether to prompt. We verify the result contains
	// the data needed for the CLI to act on.

	t.Run("stop result provides task key for prompt context", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if _, err := Start("PROJ-123", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		result, err := Stop(start.Add(15*time.Minute), 5, path)
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if result.TaskKey == "" {
			t.Error("TaskKey should be set so CLI can prompt with context")
		}
		if result.Rounded == 0 {
			t.Error("Rounded duration should be set for worklog submission")
		}
	})

	t.Run("stop result has both elapsed and rounded for display", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if _, err := Start("PROJ-123", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		result, err := Stop(start.Add(7*time.Minute), 5, path)
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if result.Elapsed != 7*time.Minute {
			t.Errorf("Elapsed = %v, want 7m", result.Elapsed)
		}
		if result.Rounded != 5*time.Minute {
			t.Errorf("Rounded = %v, want 5m", result.Rounded)
		}
	})
}

func TestTIMER005_TimeRoundedToNearest5Minutes(t *testing.T) {
	tests := []struct {
		name     string
		elapsed  time.Duration
		round    int
		expected time.Duration
	}{
		{"7 min rounds to 5 min", 7 * time.Minute, 5, 5 * time.Minute},
		{"8 min rounds to 10 min", 8 * time.Minute, 5, 10 * time.Minute},
		{"exactly 10 min stays 10 min", 10 * time.Minute, 5, 10 * time.Minute},
		{"2 min rounds up to minimum 5 min", 2 * time.Minute, 5, 5 * time.Minute},
		{"0 min rounds to minimum 5 min", 0, 5, 5 * time.Minute},
		{"12 min with 10-min rounding = 10 min", 12 * time.Minute, 10, 10 * time.Minute},
		{"18 min with 10-min rounding = 20 min", 18 * time.Minute, 10, 20 * time.Minute},
		{"3 min with 1-min rounding = 3 min", 3 * time.Minute, 1, 3 * time.Minute},
		{"30 sec with 1-min rounding = 1 min", 30 * time.Second, 1, 1 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RoundDuration(tt.elapsed, tt.round)
			if got != tt.expected {
				t.Errorf("RoundDuration(%v, %d) = %v, want %v", tt.elapsed, tt.round, got, tt.expected)
			}
		})
	}

	t.Run("rounding configurable via roundMinutes parameter", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if _, err := Start("PROJ-123", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		result, err := Stop(start.Add(7*time.Minute), 10, path)
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if result.Rounded != 10*time.Minute {
			t.Errorf("Rounded = %v, want 10m (10-min rounding)", result.Rounded)
		}
	})
}

func TestSetComment(t *testing.T) {
	t.Run("set comment persists", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if _, err := Start("PROJ-123", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if err := SetComment("working on X", path); err != nil {
			t.Fatalf("SetComment: %v", err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if loaded.Comment != "working on X" {
			t.Errorf("Comment = %q, want %q", loaded.Comment, "working on X")
		}
	})

	t.Run("error when no timer running", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "timer.json")
		err := SetComment("some comment", path)
		if err == nil {
			t.Error("expected error when no timer running")
		}
	})
}

func TestStop_IncludesComment(t *testing.T) {
	path := tmpPath(t)
	start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	if _, err := Start("PROJ-123", "", "", "", start, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := SetComment("did stuff", path); err != nil {
		t.Fatalf("SetComment: %v", err)
	}
	result, err := Stop(start.Add(10*time.Minute), 5, path)
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if result.Comment != "did stuff" {
		t.Errorf("Comment = %q, want %q", result.Comment, "did stuff")
	}
}

func TestBackwardCompat_NoCommentField(t *testing.T) {
	path := tmpPath(t)
	data := []byte(`{"taskKey":"PROJ-1","startTime":"2025-06-15T10:00:00Z"}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Comment != "" {
		t.Errorf("Comment = %q, want empty", loaded.Comment)
	}
}

func TestTIMER006_StatusShowsRunningTaskAndElapsed(t *testing.T) {
	t.Run("shows task key and elapsed time", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
		now := start.Add(83 * time.Minute) // 1h 23m

		if _, err := Start("PROJ-123", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		state, elapsed, err := Status(now, path)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if state == nil {
			t.Fatal("expected non-nil state")
		}
		if state.TaskKey != "PROJ-123" {
			t.Errorf("TaskKey = %q, want PROJ-123", state.TaskKey)
		}
		if elapsed != 83*time.Minute {
			t.Errorf("Elapsed = %v, want 1h23m", elapsed)
		}
	})

	t.Run("no timer returns nil state", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "timer.json")
		state, elapsed, err := Status(time.Now(), path)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if state != nil {
			t.Error("expected nil state when no timer running")
		}
		if elapsed != 0 {
			t.Error("expected zero elapsed when no timer running")
		}
	})

	t.Run("elapsed updates with current time", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if _, err := Start("PROJ-123", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}

		_, elapsed1, _ := Status(start.Add(5*time.Minute), path)
		_, elapsed2, _ := Status(start.Add(10*time.Minute), path)

		if elapsed1 != 5*time.Minute {
			t.Errorf("elapsed1 = %v, want 5m", elapsed1)
		}
		if elapsed2 != 10*time.Minute {
			t.Errorf("elapsed2 = %v, want 10m", elapsed2)
		}
	})
}
