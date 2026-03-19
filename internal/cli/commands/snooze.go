package commands

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tj/go-naturaldate"
)

var snoozeDurationRe = regexp.MustCompile(`^(\d+)([dhwm])$`)

// SnoozeParams holds inputs for the snooze command.
type SnoozeParams struct {
	TaskKey string
	Target  string
	Source  TaskSource
}

// SnoozeResult holds the output of a snooze operation.
type SnoozeResult struct {
	TaskKey   string
	NotBefore time.Time
}

// ParseSnoozeTarget parses a snooze target into a time.Time.
// Supports: "3d", "1w", "2h", and natural language via go-naturaldate.
func ParseSnoozeTarget(raw string, now time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)

	if m := snoozeDurationRe.FindStringSubmatch(raw); m != nil {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "h":
			return now.Add(time.Duration(n) * time.Hour), nil
		case "d":
			return now.AddDate(0, 0, n), nil
		case "w":
			return now.AddDate(0, 0, n*7), nil
		case "m":
			return now.AddDate(0, n, 0), nil
		}
	}

	t, err := naturaldate.Parse(raw, now, naturaldate.WithDirection(naturaldate.Future))
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse snooze target %q: %w", raw, err)
	}
	return t, nil
}

// RunSnooze sets the not-before date on a task to snooze it.
func RunSnooze(ctx context.Context, p SnoozeParams) (*SnoozeResult, error) {
	target, err := ParseSnoozeTarget(p.Target, time.Now())
	if err != nil {
		return nil, err
	}

	notBeforeStr := target.Format("2006-01-02")
	_, err = RunEdit(ctx, EditParams{
		TaskKey:   p.TaskKey,
		NotBefore: notBeforeStr,
		Source:    p.Source,
	})
	if err != nil {
		return nil, fmt.Errorf("snooze: %w", err)
	}

	return &SnoozeResult{
		TaskKey:   p.TaskKey,
		NotBefore: target,
	}, nil
}

// PrintSnoozeResult writes the snooze confirmation to the given writer.
func PrintSnoozeResult(w io.Writer, result *SnoozeResult) {
	fmt.Fprintf(w, "Snoozed %s until %s\n", result.TaskKey, result.NotBefore.Format("Mon Jan 2"))
}
