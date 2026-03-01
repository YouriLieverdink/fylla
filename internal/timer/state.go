package timer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

// State represents a running timer persisted to disk.
type State struct {
	TaskKey   string    `json:"taskKey"`
	StartTime time.Time `json:"startTime"`
	Project   string    `json:"project,omitempty"`
	Section   string    `json:"section,omitempty"`
}

// StopResult holds the computed values when a timer is stopped.
type StopResult struct {
	TaskKey   string
	StartTime time.Time
	Elapsed   time.Duration
	Rounded   time.Duration
}

// DefaultPath returns the default timer state file path (~/.config/fylla/timer.json).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(dir, "fylla", "timer.json"), nil
}

// Start creates a new timer state and persists it to the given path.
func Start(taskKey, project, section string, now time.Time, path string) (*State, error) {
	s := &State{TaskKey: taskKey, StartTime: now, Project: project, Section: section}
	if err := save(s, path); err != nil {
		return nil, fmt.Errorf("start timer: %w", err)
	}
	return s, nil
}

// Load reads the timer state from the given path.
// Returns nil and no error if the file does not exist.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read timer state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse timer state: %w", err)
	}
	return &s, nil
}

// Stop loads the running timer, computes elapsed time, rounds it, removes the
// state file, and returns the result. roundMinutes controls rounding granularity
// (e.g. 5 rounds to the nearest 5 minutes, minimum 5 minutes).
func Stop(now time.Time, roundMinutes int, path string) (*StopResult, error) {
	s, err := Load(path)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("no timer running")
	}

	elapsed := now.Sub(s.StartTime)
	if elapsed < 0 {
		elapsed = 0
	}

	rounded := RoundDuration(elapsed, roundMinutes)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove timer state: %w", err)
	}

	return &StopResult{
		TaskKey:   s.TaskKey,
		StartTime: s.StartTime,
		Elapsed:   elapsed,
		Rounded:   rounded,
	}, nil
}

// Status returns the current timer state and elapsed time, or nil if no timer is running.
func Status(now time.Time, path string) (*State, time.Duration, error) {
	s, err := Load(path)
	if err != nil {
		return nil, 0, err
	}
	if s == nil {
		return nil, 0, nil
	}
	elapsed := now.Sub(s.StartTime)
	if elapsed < 0 {
		elapsed = 0
	}
	return s, elapsed, nil
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

func save(s *State, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create timer dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal timer state: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}
