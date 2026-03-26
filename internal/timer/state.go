package timer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

// Segment represents a completed time segment within a timer stack entry.
type Segment struct {
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Comment   string    `json:"comment,omitempty"`
}

// StackEntry represents a timer on the stack — either active (top) or paused.
type StackEntry struct {
	TaskKey   string    `json:"taskKey"`
	StartTime time.Time `json:"startTime,omitempty"`
	Project   string    `json:"project,omitempty"`
	Section   string    `json:"section,omitempty"`
	Provider  string    `json:"provider,omitempty"`
	Comment   string    `json:"comment,omitempty"`
	Segments  []Segment `json:"segments,omitempty"`
}

// StackState holds the full timer stack. Index 0 is active, 1+ are paused.
type StackState struct {
	Stack []StackEntry `json:"stack"`
}

// ResumedInfo describes a timer that was auto-resumed after stop/abort.
type ResumedInfo struct {
	TaskKey string
	Project string
}

// StopResult holds the computed values when a timer is stopped.
type StopResult struct {
	TaskKey  string
	Provider string
	Project  string
	Section  string
	Segments []Segment
	Resumed  *ResumedInfo
}

// AbortResult holds the result of aborting a timer.
type AbortResult struct {
	TaskKey string
	Resumed *ResumedInfo
}

// SegmentInfo describes a completed segment in the status output.
type SegmentInfo struct {
	Duration time.Duration
	Comment  string
}

// StatusResult holds the current timer status.
type StatusResult struct {
	TaskKey      string
	Project      string
	Section      string
	Comment      string
	StartTime    time.Time     // start of current segment
	Elapsed      time.Duration // current segment elapsed
	TotalElapsed time.Duration // all segments + current segment
	Segments     []SegmentInfo // prior completed segments
	Paused       []PausedInfo
}

// PausedInfo describes a paused timer on the stack.
type PausedInfo struct {
	TaskKey      string
	Project      string
	Section      string
	SegmentCount int
}

// DefaultPath returns the default timer state file path (~/.config/fylla/timer.json).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(dir, "fylla", "timer.json"), nil
}

func loadStack(path string) (*StackState, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read timer state: %w", err)
	}

	var ss StackState
	if err := json.Unmarshal(data, &ss); err != nil {
		return nil, fmt.Errorf("parse timer state: %w", err)
	}
	if len(ss.Stack) == 0 {
		return nil, nil
	}
	return &ss, nil
}

func saveStack(ss *StackState, path string) error {
	if ss == nil || len(ss.Stack) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove timer state: %w", err)
		}
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create timer dir: %w", err)
	}
	data, err := json.MarshalIndent(ss, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal timer state: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// Start creates a new timer. Errors if a timer is already running.
func Start(taskKey, project, section, provider string, now time.Time, path string) error {
	ss, err := loadStack(path)
	if err != nil {
		return err
	}
	if ss != nil && len(ss.Stack) > 0 {
		return fmt.Errorf("timer already running, use interrupt to pause it")
	}
	ss = &StackState{
		Stack: []StackEntry{{
			TaskKey:   taskKey,
			StartTime: CeilMinute(now),
			Project:   project,
			Section:   section,
			Provider:  provider,
		}},
	}
	return saveStack(ss, path)
}

// Interrupt pauses the current timer and starts a new anonymous timer.
func Interrupt(now time.Time, path string) error {
	ss, err := loadStack(path)
	if err != nil {
		return err
	}
	if ss == nil || len(ss.Stack) == 0 {
		return fmt.Errorf("no timer running")
	}

	active := &ss.Stack[0]
	// Save current run as a segment — floor the end time to avoid overlap.
	seg := Segment{
		StartTime: active.StartTime,
		EndTime:   FloorMinute(now),
		Comment:   active.Comment,
	}
	active.Segments = append(active.Segments, seg)
	active.StartTime = time.Time{} // clear — paused
	active.Comment = ""

	// Push new anonymous entry at front — ceil the start time.
	ss.Stack = append([]StackEntry{{StartTime: CeilMinute(now)}}, ss.Stack...)
	return saveStack(ss, path)
}

// Stop stops the active timer, returns all segments, and resumes the next timer if present.
func Stop(now time.Time, path string) (*StopResult, error) {
	ss, err := loadStack(path)
	if err != nil {
		return nil, err
	}
	if ss == nil || len(ss.Stack) == 0 {
		return nil, fmt.Errorf("no timer running")
	}

	active := ss.Stack[0]

	// Create final segment from current run.
	// Floor the end time so the worklog ends on a minute boundary and does
	// not overlap with a task that starts (ceiled) in the same minute.
	finalSeg := Segment{
		StartTime: active.StartTime,
		EndTime:   FloorMinute(now),
		Comment:   active.Comment,
	}
	segments := append(active.Segments, finalSeg)

	result := &StopResult{
		TaskKey:  active.TaskKey,
		Provider: active.Provider,
		Project:  active.Project,
		Section:  active.Section,
		Segments: segments,
	}

	// Remove active entry
	ss.Stack = ss.Stack[1:]

	// Resume next if present — ceil to the next minute so it does not
	// share a minute with the segment that just ended.
	if len(ss.Stack) > 0 {
		ss.Stack[0].StartTime = CeilMinute(now)
		result.Resumed = &ResumedInfo{
			TaskKey: ss.Stack[0].TaskKey,
			Project: ss.Stack[0].Project,
		}
	}

	if err := saveStack(ss, path); err != nil {
		return nil, err
	}
	return result, nil
}

// Status returns the current timer status, or nil if no timer is running.
func Status(now time.Time, path string) (*StatusResult, error) {
	ss, err := loadStack(path)
	if err != nil {
		return nil, err
	}
	if ss == nil || len(ss.Stack) == 0 {
		return nil, nil
	}

	active := ss.Stack[0]
	elapsed := now.Sub(active.StartTime)
	if elapsed < 0 {
		elapsed = 0
	}

	// Sum prior segments for total elapsed
	var priorElapsed time.Duration
	var segments []SegmentInfo
	for _, seg := range active.Segments {
		d := seg.EndTime.Sub(seg.StartTime)
		if d < 0 {
			d = 0
		}
		priorElapsed += d
		segments = append(segments, SegmentInfo{Duration: d, Comment: seg.Comment})
	}

	result := &StatusResult{
		TaskKey:      active.TaskKey,
		Project:      active.Project,
		Section:      active.Section,
		Comment:      active.Comment,
		StartTime:    active.StartTime,
		Elapsed:      elapsed,
		TotalElapsed: priorElapsed + elapsed,
		Segments:     segments,
	}

	for _, entry := range ss.Stack[1:] {
		result.Paused = append(result.Paused, PausedInfo{
			TaskKey:      entry.TaskKey,
			Project:      entry.Project,
			Section:      entry.Section,
			SegmentCount: len(entry.Segments),
		})
	}

	return result, nil
}

// Abort discards the current timer and resumes the next if present.
func Abort(now time.Time, path string) (*AbortResult, error) {
	ss, err := loadStack(path)
	if err != nil {
		return nil, err
	}
	if ss == nil || len(ss.Stack) == 0 {
		return nil, fmt.Errorf("no timer running")
	}

	result := &AbortResult{
		TaskKey: ss.Stack[0].TaskKey,
	}

	// Remove active entry
	ss.Stack = ss.Stack[1:]

	// Resume next if present — ceil to avoid overlap.
	if len(ss.Stack) > 0 {
		ss.Stack[0].StartTime = CeilMinute(now)
		result.Resumed = &ResumedInfo{
			TaskKey: ss.Stack[0].TaskKey,
			Project: ss.Stack[0].Project,
		}
	}

	if err := saveStack(ss, path); err != nil {
		return nil, err
	}
	return result, nil
}

// SetStartTime changes the start time of the active (current segment) timer.
// The provided time is ceiled to the next minute boundary for consistency.
func SetStartTime(startTime, now time.Time, path string) error {
	ss, err := loadStack(path)
	if err != nil {
		return err
	}
	if ss == nil || len(ss.Stack) == 0 {
		return fmt.Errorf("no timer running")
	}
	rounded := CeilMinute(startTime)
	if rounded.After(CeilMinute(now)) {
		return fmt.Errorf("start time cannot be in the future")
	}
	ss.Stack[0].StartTime = rounded
	return saveStack(ss, path)
}

// SetComment sets the comment on the active timer.
func SetComment(comment, path string) error {
	ss, err := loadStack(path)
	if err != nil {
		return err
	}
	if ss == nil || len(ss.Stack) == 0 {
		return fmt.Errorf("no timer running")
	}
	ss.Stack[0].Comment = comment
	return saveStack(ss, path)
}

// FloorMinute truncates t down to the current minute boundary.
func FloorMinute(t time.Time) time.Time {
	return t.Truncate(time.Minute)
}

// CeilMinute rounds t up to the next minute boundary. If t is already on a
// minute boundary it is returned unchanged.
func CeilMinute(t time.Time) time.Time {
	if t.Second() == 0 && t.Nanosecond() == 0 {
		return t
	}
	return t.Truncate(time.Minute).Add(time.Minute)
}

// RoundDuration rounds d to the nearest roundMinutes, with a minimum of roundMinutes.
func RoundDuration(d time.Duration, roundMinutes int) time.Duration {
	if roundMinutes <= 0 {
		return d
	}
	unit := time.Duration(roundMinutes) * time.Minute
	rounded := time.Duration(math.Round(float64(d)/float64(unit))) * unit
	if rounded < unit {
		rounded = unit
	}
	return rounded
}
