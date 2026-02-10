package commands

import (
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/timer"
	"github.com/spf13/cobra"
)

// StatusParams holds inputs for the status command.
type StatusParams struct {
	TimerPath string
	Now       time.Time
}

// StatusResult holds the output of a status operation.
type StatusResult struct {
	TaskKey string
	Elapsed time.Duration
}

// RunStatus returns the current timer state, or nil if no timer is running.
func RunStatus(p StatusParams) (*StatusResult, error) {
	state, elapsed, err := timer.Status(p.Now, p.TimerPath)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	return &StatusResult{
		TaskKey: state.TaskKey,
		Elapsed: elapsed,
	}, nil
}

// PrintStatusResult writes the status to the given writer.
func PrintStatusResult(w io.Writer, result *StatusResult) {
	if result == nil {
		fmt.Fprintln(w, "No timer running.")
		return
	}
	fmt.Fprintf(w, "%s\n", result.TaskKey)
	fmt.Fprintf(w, "Running for: %s\n", formatElapsed(result.Elapsed))
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

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show currently running task and elapsed time",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := timer.DefaultPath()
			if err != nil {
				return fmt.Errorf("timer path: %w", err)
			}

			result, err := RunStatus(StatusParams{
				TimerPath: path,
				Now:       time.Now(),
			})
			if err != nil {
				return err
			}

			PrintStatusResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
}
