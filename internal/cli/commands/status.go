package commands

import (
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/timer"
)

// StatusParams holds inputs for the status command.
type StatusParams struct {
	TimerPath string
	Now       time.Time
}

// PausedStatus describes a paused timer in the status output.
type PausedStatus struct {
	TaskKey      string
	Project      string
	Section      string
	SegmentCount int
}

// SegmentStatus describes a completed segment in the status output.
type SegmentStatus struct {
	Duration time.Duration
	Comment  string
}

// StatusResult holds the output of a status operation.
type StatusResult struct {
	TaskKey      string
	Project      string
	Section      string
	Elapsed      time.Duration
	TotalElapsed time.Duration
	Segments     []SegmentStatus
	Comment      string
	Paused       []PausedStatus
}

// RunStatus returns the current timer state, or nil if no timer is running.
func RunStatus(p StatusParams) (*StatusResult, error) {
	sr, err := timer.Status(p.Now, p.TimerPath)
	if err != nil {
		return nil, err
	}
	if sr == nil {
		return nil, nil
	}
	result := &StatusResult{
		TaskKey:      sr.TaskKey,
		Project:      sr.Project,
		Section:      sr.Section,
		Elapsed:      sr.Elapsed,
		TotalElapsed: sr.TotalElapsed,
		Comment:      sr.Comment,
	}
	for _, s := range sr.Segments {
		result.Segments = append(result.Segments, SegmentStatus{Duration: s.Duration, Comment: s.Comment})
	}
	for _, p := range sr.Paused {
		result.Paused = append(result.Paused, PausedStatus{
			TaskKey:      p.TaskKey,
			Project:      p.Project,
			Section:      p.Section,
			SegmentCount: p.SegmentCount,
		})
	}
	return result, nil
}

// PrintStatusResult writes the status to the given writer.
func PrintStatusResult(w io.Writer, result *StatusResult) {
	if result == nil {
		fmt.Fprintln(w, "No timer running.")
		return
	}
	fmt.Fprintf(w, "%s\n", result.TaskKey)
	fmt.Fprintf(w, "Running for: %s\n", formatElapsed(result.Elapsed))
	for _, p := range result.Paused {
		label := p.TaskKey
		if label == "" {
			label = "(anonymous)"
		}
		fmt.Fprintf(w, "Paused: %s (%d segments)\n", label, p.SegmentCount)
	}
}

func formatElapsed(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}
