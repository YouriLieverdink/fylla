package commands

import (
	"fmt"
	"io"

	"github.com/iruoy/fylla/internal/timer"
	"github.com/spf13/cobra"
)

// AbortParams holds inputs for the abort command.
type AbortParams struct {
	TimerPath string
}

// AbortResult holds the output of an abort operation.
type AbortResult struct {
	TaskKey string
}

// RunAbort aborts the running timer without logging work.
func RunAbort(p AbortParams) (*AbortResult, error) {
	s, err := timer.Abort(p.TimerPath)
	if err != nil {
		return nil, err
	}
	return &AbortResult{TaskKey: s.TaskKey}, nil
}

// PrintAbortResult writes the abort result to the given writer.
func PrintAbortResult(w io.Writer, result *AbortResult) {
	fmt.Fprintf(w, "Timer aborted for %s\n", result.TaskKey)
}

func newAbortCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "abort",
		Short: "Abort timer without logging work",
		RunE: func(cmd *cobra.Command, args []string) error {
			timerPath, err := timer.DefaultPath()
			if err != nil {
				return fmt.Errorf("timer path: %w", err)
			}

			result, err := RunAbort(AbortParams{TimerPath: timerPath})
			if err != nil {
				return err
			}

			PrintAbortResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
}
