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

		if err := Start("PROJ-123", "", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		ss, err := loadStack(path)
		if err != nil {
			t.Fatalf("loadStack: %v", err)
		}
		if ss.Stack[0].TaskKey != "PROJ-123" {
			t.Errorf("TaskKey = %q, want PROJ-123", ss.Stack[0].TaskKey)
		}
		if !ss.Stack[0].StartTime.Equal(now) {
			t.Errorf("StartTime = %v, want %v", ss.Stack[0].StartTime, now)
		}
	})

	t.Run("confirmation data available", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)

		if err := Start("ADMIN-42", "", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		ss, err := loadStack(path)
		if err != nil {
			t.Fatalf("loadStack: %v", err)
		}
		if ss.Stack[0].TaskKey != "ADMIN-42" {
			t.Errorf("TaskKey = %q, want ADMIN-42", ss.Stack[0].TaskKey)
		}
	})
}

func TestTIMER002_TimerStatePersisted(t *testing.T) {
	t.Run("file exists at path", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if err := Start("PROJ-123", "", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("timer.json should exist: %v", err)
		}
	})

	t.Run("file contains task key and start timestamp", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if err := Start("PROJ-123", "", "", "", "", now, path); err != nil {
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
		stack, ok := raw["stack"].([]interface{})
		if !ok || len(stack) == 0 {
			t.Fatal("expected stack array with entries")
		}
		entry := stack[0].(map[string]interface{})
		if entry["taskKey"] != "PROJ-123" {
			t.Errorf("taskKey = %v, want PROJ-123", entry["taskKey"])
		}
		if _, ok := entry["startTime"]; !ok {
			t.Error("startTime field missing from timer entry")
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

		if err := Start("PROJ-123", "", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		ss, err := loadStack(path)
		if err != nil {
			t.Fatalf("loadStack: %v", err)
		}
		if ss.Stack[0].TaskKey != "PROJ-123" {
			t.Errorf("TaskKey = %q, want PROJ-123", ss.Stack[0].TaskKey)
		}
		if !ss.Stack[0].StartTime.Equal(now) {
			t.Errorf("StartTime = %v, want %v", ss.Stack[0].StartTime, now)
		}
	})

	t.Run("load returns nil when no file exists", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent.json")
		ss, err := loadStack(path)
		if err != nil {
			t.Fatalf("loadStack: %v", err)
		}
		if ss != nil {
			t.Error("expected nil state for missing file")
		}
	})
}

func TestTIMER003_StopTimerCalculatesElapsed(t *testing.T) {
	t.Run("elapsed time calculated correctly", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
		stop := start.Add(5 * time.Minute)

		if err := Start("PROJ-123", "", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		result, err := Stop(stop, path)
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		elapsed := result.Segments[0].EndTime.Sub(result.Segments[0].StartTime)
		if elapsed != 5*time.Minute {
			t.Errorf("Elapsed = %v, want 5m", elapsed)
		}
	})

	t.Run("timer file removed after stop", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if err := Start("PROJ-123", "", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := Stop(start.Add(10*time.Minute), path); err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("timer.json should be removed after stop")
		}
	})

	t.Run("stop with no timer returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "timer.json")
		_, err := Stop(time.Now(), path)
		if err == nil {
			t.Error("expected error when no timer running")
		}
	})

	t.Run("task key preserved in result", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if err := Start("PROJ-456", "", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		result, err := Stop(start.Add(30*time.Minute), path)
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if result.TaskKey != "PROJ-456" {
			t.Errorf("TaskKey = %q, want PROJ-456", result.TaskKey)
		}
	})
}

func TestTIMER004_StopPromptForDescription(t *testing.T) {
	t.Run("stop result provides task key for prompt context", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if err := Start("PROJ-123", "", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		result, err := Stop(start.Add(15*time.Minute), path)
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if result.TaskKey == "" {
			t.Error("TaskKey should be set so CLI can prompt with context")
		}
		if len(result.Segments) == 0 {
			t.Error("expected at least one segment")
		}
	})

	t.Run("stop result has segment with correct times", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if err := Start("PROJ-123", "", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		result, err := Stop(start.Add(7*time.Minute), path)
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		seg := result.Segments[0]
		elapsed := seg.EndTime.Sub(seg.StartTime)
		if elapsed != 7*time.Minute {
			t.Errorf("Elapsed = %v, want 7m", elapsed)
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
}

func TestSetComment(t *testing.T) {
	t.Run("set comment persists", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if err := Start("PROJ-123", "", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if err := SetComment("working on X", path); err != nil {
			t.Fatalf("SetComment: %v", err)
		}
		ss, err := loadStack(path)
		if err != nil {
			t.Fatalf("loadStack: %v", err)
		}
		if ss.Stack[0].Comment != "working on X" {
			t.Errorf("Comment = %q, want %q", ss.Stack[0].Comment, "working on X")
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

func TestSetStartTime(t *testing.T) {
	t.Run("changes start time", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if err := Start("PROJ-123", "", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		newStart := time.Date(2025, 6, 15, 9, 30, 0, 0, time.UTC)
		if err := SetStartTime(newStart, now, path); err != nil {
			t.Fatalf("SetStartTime: %v", err)
		}
		ss, err := loadStack(path)
		if err != nil {
			t.Fatalf("loadStack: %v", err)
		}
		if !ss.Stack[0].StartTime.Equal(newStart) {
			t.Errorf("StartTime = %v, want %v", ss.Stack[0].StartTime, newStart)
		}
	})

	t.Run("ceils to minute boundary", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if err := Start("PROJ-123", "", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		newStart := time.Date(2025, 6, 15, 9, 30, 15, 0, time.UTC)
		if err := SetStartTime(newStart, now, path); err != nil {
			t.Fatalf("SetStartTime: %v", err)
		}
		ss, err := loadStack(path)
		if err != nil {
			t.Fatalf("loadStack: %v", err)
		}
		want := time.Date(2025, 6, 15, 9, 31, 0, 0, time.UTC)
		if !ss.Stack[0].StartTime.Equal(want) {
			t.Errorf("StartTime = %v, want %v", ss.Stack[0].StartTime, want)
		}
	})

	t.Run("error when no timer running", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "timer.json")
		err := SetStartTime(time.Now(), time.Now(), path)
		if err == nil {
			t.Error("expected error when no timer running")
		}
	})

	t.Run("error when start time in future", func(t *testing.T) {
		path := tmpPath(t)
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if err := Start("PROJ-123", "", "", "", "", now, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		future := now.Add(time.Hour)
		err := SetStartTime(future, now, path)
		if err == nil {
			t.Error("expected error for future start time")
		}
	})
}

func TestStop_IncludesComment(t *testing.T) {
	path := tmpPath(t)
	start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	if err := Start("PROJ-123", "", "", "", "", start, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := SetComment("did stuff", path); err != nil {
		t.Fatalf("SetComment: %v", err)
	}
	result, err := Stop(start.Add(10*time.Minute), path)
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if result.Segments[0].Comment != "did stuff" {
		t.Errorf("Comment = %q, want %q", result.Segments[0].Comment, "did stuff")
	}
}

func TestLegacyFormat_ReturnsNil(t *testing.T) {
	path := tmpPath(t)
	// Legacy single-entry format (no "stack" key) is no longer supported.
	// It should be treated as "no timer running" (nil).
	data := []byte(`{"taskKey":"PROJ-1","startTime":"2025-06-15T10:00:00Z"}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	ss, err := loadStack(path)
	if err != nil {
		t.Fatalf("loadStack: %v", err)
	}
	if ss != nil {
		t.Fatalf("expected nil for legacy format, got %+v", ss)
	}
}

func TestTIMER006_StatusShowsRunningTaskAndElapsed(t *testing.T) {
	t.Run("shows task key and elapsed time", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
		now := start.Add(83 * time.Minute) // 1h 23m

		if err := Start("PROJ-123", "", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}
		sr, err := Status(now, path)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if sr == nil {
			t.Fatal("expected non-nil status")
		}
		if sr.TaskKey != "PROJ-123" {
			t.Errorf("TaskKey = %q, want PROJ-123", sr.TaskKey)
		}
		if sr.Elapsed != 83*time.Minute {
			t.Errorf("Elapsed = %v, want 1h23m", sr.Elapsed)
		}
	})

	t.Run("no timer returns nil status", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "timer.json")
		sr, err := Status(time.Now(), path)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if sr != nil {
			t.Error("expected nil status when no timer running")
		}
	})

	t.Run("elapsed updates with current time", func(t *testing.T) {
		path := tmpPath(t)
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

		if err := Start("PROJ-123", "", "", "", "", start, path); err != nil {
			t.Fatalf("Start: %v", err)
		}

		sr1, _ := Status(start.Add(5*time.Minute), path)
		sr2, _ := Status(start.Add(10*time.Minute), path)

		if sr1.Elapsed != 5*time.Minute {
			t.Errorf("elapsed1 = %v, want 5m", sr1.Elapsed)
		}
		if sr2.Elapsed != 10*time.Minute {
			t.Errorf("elapsed2 = %v, want 10m", sr2.Elapsed)
		}
	})
}

// Stack operation tests

func TestInterrupt_SavesSegmentAndStartsAnonymous(t *testing.T) {
	path := tmpPath(t)
	start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	interruptTime := start.Add(30 * time.Minute)

	if err := Start("PROJ-123", "MyProject", "", "", "", start, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := SetComment("working on feature", path); err != nil {
		t.Fatalf("SetComment: %v", err)
	}
	if err := Interrupt(interruptTime, path); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}

	ss, err := loadStack(path)
	if err != nil {
		t.Fatalf("loadStack: %v", err)
	}
	if len(ss.Stack) != 2 {
		t.Fatalf("expected 2 stack entries, got %d", len(ss.Stack))
	}

	// Active entry should be anonymous with startTime = interruptTime
	active := ss.Stack[0]
	if active.TaskKey != "" {
		t.Errorf("active TaskKey = %q, want empty (anonymous)", active.TaskKey)
	}
	if !active.StartTime.Equal(interruptTime) {
		t.Errorf("active StartTime = %v, want %v", active.StartTime, interruptTime)
	}

	// Paused entry should have segment and no startTime
	paused := ss.Stack[1]
	if paused.TaskKey != "PROJ-123" {
		t.Errorf("paused TaskKey = %q, want PROJ-123", paused.TaskKey)
	}
	if !paused.StartTime.IsZero() {
		t.Error("paused entry should have zero StartTime")
	}
	if len(paused.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(paused.Segments))
	}
	seg := paused.Segments[0]
	if !seg.StartTime.Equal(start) {
		t.Errorf("segment StartTime = %v, want %v", seg.StartTime, start)
	}
	if !seg.EndTime.Equal(interruptTime) {
		t.Errorf("segment EndTime = %v, want %v", seg.EndTime, interruptTime)
	}
	if seg.Comment != "working on feature" {
		t.Errorf("segment Comment = %q, want %q", seg.Comment, "working on feature")
	}
	if paused.Comment != "" {
		t.Error("paused entry comment should be cleared after interrupt")
	}
}

func TestStop_WithSegments_ReturnsAllSegments(t *testing.T) {
	path := tmpPath(t)
	t0 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)
	t2 := t1.Add(15 * time.Minute)

	// Start PROJ-123, interrupt, then stop anonymous
	if err := Start("PROJ-123", "", "", "", "", t0, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := Interrupt(t1, path); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}
	// Stop the anonymous timer
	result, err := Stop(t2, path)
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Anonymous timer had 1 segment (t1→t2)
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	if result.TaskKey != "" {
		t.Errorf("TaskKey = %q, want empty (anonymous)", result.TaskKey)
	}

	// Should resume PROJ-123
	if result.Resumed == nil {
		t.Fatal("expected resumed info")
	}
	if result.Resumed.TaskKey != "PROJ-123" {
		t.Errorf("Resumed.TaskKey = %q, want PROJ-123", result.Resumed.TaskKey)
	}

	// Now stop the resumed PROJ-123
	t3 := t2.Add(20 * time.Minute)
	result2, err := Stop(t3, path)
	if err != nil {
		t.Fatalf("Stop resumed: %v", err)
	}
	// PROJ-123 has 2 segments: original (t0→t1) + resumed (t2→t3)
	if len(result2.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(result2.Segments))
	}
	if result2.TaskKey != "PROJ-123" {
		t.Errorf("TaskKey = %q, want PROJ-123", result2.TaskKey)
	}
	if !result2.Segments[0].StartTime.Equal(t0) {
		t.Errorf("seg[0].Start = %v, want %v", result2.Segments[0].StartTime, t0)
	}
	if !result2.Segments[0].EndTime.Equal(t1) {
		t.Errorf("seg[0].End = %v, want %v", result2.Segments[0].EndTime, t1)
	}
	if !result2.Segments[1].StartTime.Equal(t2) {
		t.Errorf("seg[1].Start = %v, want %v", result2.Segments[1].StartTime, t2)
	}
	if !result2.Segments[1].EndTime.Equal(t3) {
		t.Errorf("seg[1].End = %v, want %v", result2.Segments[1].EndTime, t3)
	}
	if result2.Resumed != nil {
		t.Error("expected no resumed info after final stop")
	}
}

func TestStop_ResumesPausedTimerWithFreshStartTime(t *testing.T) {
	path := tmpPath(t)
	t0 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)
	t2 := t1.Add(15 * time.Minute)

	if err := Start("PROJ-123", "", "", "", "", t0, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := Interrupt(t1, path); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}
	if _, err := Stop(t2, path); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Verify resumed timer has startTime = t2
	ss, err := loadStack(path)
	if err != nil {
		t.Fatalf("loadStack: %v", err)
	}
	if len(ss.Stack) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(ss.Stack))
	}
	if !ss.Stack[0].StartTime.Equal(t2) {
		t.Errorf("resumed StartTime = %v, want %v", ss.Stack[0].StartTime, t2)
	}
}

func TestDoubleInterrupt_CreatesStackDepth3(t *testing.T) {
	path := tmpPath(t)
	t0 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)
	t2 := t1.Add(15 * time.Minute)

	if err := Start("PROJ-123", "", "", "", "", t0, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := Interrupt(t1, path); err != nil {
		t.Fatalf("Interrupt 1: %v", err)
	}
	if err := Interrupt(t2, path); err != nil {
		t.Fatalf("Interrupt 2: %v", err)
	}

	ss, err := loadStack(path)
	if err != nil {
		t.Fatalf("loadStack: %v", err)
	}
	if len(ss.Stack) != 3 {
		t.Fatalf("expected 3 stack entries, got %d", len(ss.Stack))
	}
	// Top is anonymous (from 2nd interrupt)
	if ss.Stack[0].TaskKey != "" {
		t.Errorf("stack[0] TaskKey = %q, want empty", ss.Stack[0].TaskKey)
	}
	// Middle is anonymous (from 1st interrupt, now paused)
	if ss.Stack[1].TaskKey != "" {
		t.Errorf("stack[1] TaskKey = %q, want empty", ss.Stack[1].TaskKey)
	}
	// Bottom is PROJ-123
	if ss.Stack[2].TaskKey != "PROJ-123" {
		t.Errorf("stack[2] TaskKey = %q, want PROJ-123", ss.Stack[2].TaskKey)
	}
}

func TestAbort_DiscardsCurrentResumesPrevious(t *testing.T) {
	path := tmpPath(t)
	t0 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)
	t2 := t1.Add(15 * time.Minute)

	if err := Start("PROJ-123", "MyProject", "", "", "", t0, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := Interrupt(t1, path); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}

	result, err := Abort(t2, path)
	if err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if result.TaskKey != "" {
		t.Errorf("aborted TaskKey = %q, want empty (anonymous)", result.TaskKey)
	}
	if result.Resumed == nil {
		t.Fatal("expected resumed info")
	}
	if result.Resumed.TaskKey != "PROJ-123" {
		t.Errorf("Resumed.TaskKey = %q, want PROJ-123", result.Resumed.TaskKey)
	}

	// Verify resumed timer has fresh startTime
	ss, err := loadStack(path)
	if err != nil {
		t.Fatalf("loadStack: %v", err)
	}
	if len(ss.Stack) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(ss.Stack))
	}
	if !ss.Stack[0].StartTime.Equal(t2) {
		t.Errorf("resumed StartTime = %v, want %v", ss.Stack[0].StartTime, t2)
	}
}

func TestAbort_AtStackBottom_ClearsEverything(t *testing.T) {
	path := tmpPath(t)
	t0 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)

	if err := Start("PROJ-123", "", "", "", "", t0, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	result, err := Abort(t1, path)
	if err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if result.TaskKey != "PROJ-123" {
		t.Errorf("aborted TaskKey = %q, want PROJ-123", result.TaskKey)
	}
	if result.Resumed != nil {
		t.Error("expected no resumed info")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("timer.json should be removed after abort at bottom")
	}
}

func TestStop_EmptyStack_ReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timer.json")
	_, err := Stop(time.Now(), path)
	if err == nil {
		t.Error("expected error when no timer running")
	}
}

func TestInterrupt_EmptyStack_ReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timer.json")
	err := Interrupt(time.Now(), path)
	if err == nil {
		t.Error("expected error when no timer running")
	}
}

func TestSetComment_OnStack_SetsTopEntry(t *testing.T) {
	path := tmpPath(t)
	t0 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)

	if err := Start("PROJ-123", "", "", "", "", t0, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := Interrupt(t1, path); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}
	if err := SetComment("interrupt work", path); err != nil {
		t.Fatalf("SetComment: %v", err)
	}

	ss, err := loadStack(path)
	if err != nil {
		t.Fatalf("loadStack: %v", err)
	}
	if ss.Stack[0].Comment != "interrupt work" {
		t.Errorf("active Comment = %q, want %q", ss.Stack[0].Comment, "interrupt work")
	}
}

func TestSegmentComment_PreservedThroughInterrupt(t *testing.T) {
	path := tmpPath(t)
	t0 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)

	if err := Start("PROJ-123", "", "", "", "", t0, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := SetComment("doing feature X", path); err != nil {
		t.Fatalf("SetComment: %v", err)
	}
	if err := Interrupt(t1, path); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}

	ss, err := loadStack(path)
	if err != nil {
		t.Fatalf("loadStack: %v", err)
	}
	// The paused entry's segment should have the comment
	if len(ss.Stack[1].Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(ss.Stack[1].Segments))
	}
	if ss.Stack[1].Segments[0].Comment != "doing feature X" {
		t.Errorf("segment comment = %q, want %q", ss.Stack[1].Segments[0].Comment, "doing feature X")
	}
}

func TestStatus_ReturnsPausedInfo(t *testing.T) {
	path := tmpPath(t)
	t0 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)
	t2 := t1.Add(15 * time.Minute)

	if err := Start("PROJ-123", "MyProject", "Sprint 1", "", "", t0, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := Interrupt(t1, path); err != nil {
		t.Fatalf("Interrupt: %v", err)
	}

	sr, err := Status(t2, path)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if sr.TaskKey != "" {
		t.Errorf("active TaskKey = %q, want empty (anonymous)", sr.TaskKey)
	}
	if sr.Elapsed != 15*time.Minute {
		t.Errorf("Elapsed = %v, want 15m", sr.Elapsed)
	}
	if len(sr.Paused) != 1 {
		t.Fatalf("expected 1 paused entry, got %d", len(sr.Paused))
	}
	if sr.Paused[0].TaskKey != "PROJ-123" {
		t.Errorf("paused TaskKey = %q, want PROJ-123", sr.Paused[0].TaskKey)
	}
	if sr.Paused[0].Project != "MyProject" {
		t.Errorf("paused Project = %q, want MyProject", sr.Paused[0].Project)
	}
	if sr.Paused[0].SegmentCount != 1 {
		t.Errorf("paused SegmentCount = %d, want 1", sr.Paused[0].SegmentCount)
	}
}

func TestStart_ErrorsWhenTimerAlreadyRunning(t *testing.T) {
	path := tmpPath(t)
	t0 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	if err := Start("PROJ-123", "", "", "", "", t0, path); err != nil {
		t.Fatalf("Start: %v", err)
	}
	err := Start("PROJ-456", "", "", "", "", t0.Add(5*time.Minute), path)
	if err == nil {
		t.Error("expected error when timer already running")
	}
}
