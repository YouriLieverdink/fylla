package commands

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// PriorityGetter abstracts fetching the current priority from a task source.
type PriorityGetter interface {
	GetPriority(ctx context.Context, issueKey string) (int, error)
}

// PriorityUpdater abstracts updating the priority in a task source.
type PriorityUpdater interface {
	UpdatePriority(ctx context.Context, issueKey string, priority int) error
}

// PriorityParams holds inputs for the priority command.
type PriorityParams struct {
	TaskKey  string
	Priority string // priority name (e.g. "High") or relative adjustment ("+1", "-1")
	Updater  PriorityUpdater
	Getter   PriorityGetter
}

// PriorityResult holds the output of a priority operation.
type PriorityResult struct {
	TaskKey  string
	Priority int
	Name     string
}

// priorityNames maps priority names to numeric levels.
var priorityNames = map[string]int{
	"highest": 1,
	"high":    2,
	"medium":  3,
	"low":     4,
	"lowest":  5,
}

// priorityLevelNames maps numeric levels to display names.
var priorityLevelNames = map[int]string{
	1: "Highest",
	2: "High",
	3: "Medium",
	4: "Low",
	5: "Lowest",
}

// RunPriority sets or adjusts the priority on a task.
func RunPriority(ctx context.Context, p PriorityParams) (*PriorityResult, error) {
	raw := strings.TrimSpace(p.Priority)
	if raw == "" {
		return nil, fmt.Errorf("priority is required")
	}

	var final int

	if strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") {
		// Relative adjustment
		delta, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid relative priority %q: %w", raw, err)
		}

		current, err := p.Getter.GetPriority(ctx, p.TaskKey)
		if err != nil {
			return nil, fmt.Errorf("get current priority: %w", err)
		}

		final = current + delta
		if final < 1 {
			final = 1
		}
		if final > 5 {
			final = 5
		}
	} else {
		// Absolute value: try name first, then numeric
		if level, ok := priorityNames[strings.ToLower(raw)]; ok {
			final = level
		} else {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > 5 {
				return nil, fmt.Errorf("invalid priority %q (use Highest, High, Medium, Low, Lowest or 1-5)", raw)
			}
			final = n
		}
	}

	if err := p.Updater.UpdatePriority(ctx, p.TaskKey, final); err != nil {
		return nil, err
	}

	return &PriorityResult{
		TaskKey:  p.TaskKey,
		Priority: final,
		Name:     priorityLevelNames[final],
	}, nil
}

// PrintPriorityResult writes the priority confirmation to the given writer.
func PrintPriorityResult(w io.Writer, result *PriorityResult) {
	fmt.Fprintf(w, "Priority for %s set to %s (%d)\n", result.TaskKey, result.Name, result.Priority)
}

