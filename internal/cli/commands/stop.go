package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/timer"
	"github.com/spf13/cobra"
)

// WorklogPoster abstracts Jira worklog posting for testing.
type WorklogPoster interface {
	PostWorklog(ctx context.Context, issueKey string, timeSpent time.Duration, description string) error
}

// StopParams holds inputs for the stop command.
type StopParams struct {
	TimerPath    string
	RoundMinutes int
	Now          time.Time
	Description  string
	Jira         WorklogPoster
}

// StopResult holds the output of a stop operation.
type StopResult struct {
	TaskKey     string
	Elapsed     time.Duration
	Rounded     time.Duration
	Description string
}

// RunStop stops the timer, posts the worklog to Jira, and returns the result.
func RunStop(ctx context.Context, p StopParams) (*StopResult, error) {
	sr, err := timer.Stop(p.Now, p.RoundMinutes, p.TimerPath)
	if err != nil {
		return nil, err
	}

	if err := p.Jira.PostWorklog(ctx, sr.TaskKey, sr.Rounded, p.Description); err != nil {
		return nil, fmt.Errorf("post worklog: %w", err)
	}

	return &StopResult{
		TaskKey:     sr.TaskKey,
		Elapsed:     sr.Elapsed,
		Rounded:     sr.Rounded,
		Description: p.Description,
	}, nil
}

// PrintStopResult writes the stop result to the given writer.
func PrintStopResult(w io.Writer, result *StopResult) {
	fmt.Fprintf(w, "Timer stopped: %s\n", formatElapsed(result.Rounded))
	fmt.Fprintf(w, "Worklog added to %s\n", result.TaskKey)
}

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop timer and log work to Jira",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().StringP("description", "d", "", "Work description (skips interactive prompt)")

	return cmd
}
