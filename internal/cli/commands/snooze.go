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

	"github.com/spf13/cobra"
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

func newSnoozeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "snooze TASK-KEY DURATION",
		Short: "Snooze a task (set not-before date)",
		Long: `Snooze a task by setting its not-before date.

Supports duration offsets (3d, 1w, 2h, 1m) and natural language (Monday, next Friday).

Examples:
  fylla task snooze L-1 3d       # snooze 3 days from now
  fylla task snooze L-1 1w       # snooze 1 week from now
  fylla task snooze L-1 Monday   # snooze until Monday`,
		Args: cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			source, _, err := loadTaskSource()
			if err != nil {
				return err
			}

			result, err := RunSnooze(cmd.Context(), SnoozeParams{
				TaskKey: args[0],
				Target:  args[1],
				Source:  source,
			})
			if err != nil {
				return err
			}

			PrintSnoozeResult(cmd.OutOrStdout(), result)
			maybeAutoResync(cmd.Context(), cmd.ErrOrStderr())
			return nil
		},
	}
}
